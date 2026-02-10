package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// IngestService handles ingest pipeline processing
type IngestService struct {
	mirrorRepo   *repo.MirrorRepo
	jobRepo      *repo.IngestJobRepo
	deltaKeyRepo *repo.DeltaKeyRepo
	metadataRepo *repo.MetadataRepo
	defaultDB    db.DBConn
}

// NewIngestService creates a new IngestService
func NewIngestService(
	mirrorRepo *repo.MirrorRepo,
	jobRepo *repo.IngestJobRepo,
	deltaKeyRepo *repo.DeltaKeyRepo,
	metadataRepo *repo.MetadataRepo,
	defaultDB db.DBConn,
) *IngestService {
	return &IngestService{
		mirrorRepo:   mirrorRepo,
		jobRepo:      jobRepo,
		deltaKeyRepo: deltaKeyRepo,
		metadataRepo: metadataRepo,
		defaultDB:    defaultDB,
	}
}

// ProcessJob orchestrates the full ingest pipeline asynchronously
// This method runs in a goroutine and must NOT panic
func (s *IngestService) ProcessJob(ctx context.Context, tenantDB db.DBConn, orgID string, job *entity.IngestJob, records []map[string]interface{}) {
	// Wrap everything in a recover block to prevent panics from crashing the server
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[INGEST-SERVICE] PANIC in ProcessJob for job %s: %v", job.ID, r)
			// Attempt to set job to failed status
			_ = s.jobRepo.UpdateStatus(ctx, tenantDB, job.ID, entity.IngestJobStatusFailed)
		}
	}()

	log.Printf("[INGEST-SERVICE] Starting ProcessJob for job %s, org %s, mirror %s, %d records", job.ID, orgID, job.MirrorID, len(records))

	// 1. Update job status to "processing"
	if err := s.jobRepo.UpdateStatus(ctx, tenantDB, job.ID, entity.IngestJobStatusProcessing); err != nil {
		log.Printf("[INGEST-SERVICE] Failed to update job status to processing: %v", err)
		return
	}

	// 2. Load mirror config
	mirror, err := s.mirrorRepo.GetActiveByID(ctx, tenantDB, orgID, job.MirrorID)
	if err != nil {
		log.Printf("[INGEST-SERVICE] Failed to load mirror: %v", err)
		s.setJobFailed(ctx, tenantDB, job.ID, fmt.Sprintf("Failed to load mirror: %v", err))
		return
	}
	if mirror == nil {
		log.Printf("[INGEST-SERVICE] Mirror %s not found or inactive", job.MirrorID)
		s.setJobFailed(ctx, tenantDB, job.ID, "Mirror not found or inactive")
		return
	}

	log.Printf("[INGEST-SERVICE] Loaded mirror: %s -> %s, unmapped_field_mode=%s, unique_key_field=%s",
		mirror.Name, mirror.TargetEntity, mirror.UnmappedFieldMode, mirror.UniqueKeyField)

	// 3. Build source field map for O(1) lookup
	sourceFieldMap := make(map[string]*entity.MirrorSourceField)
	for i := range mirror.SourceFields {
		field := &mirror.SourceFields[i]
		sourceFieldMap[field.FieldName] = field
	}

	// 4. Build field mapping (source -> target)
	fieldMapping := make(map[string]string)
	for _, field := range mirror.SourceFields {
		if field.MapField != nil && *field.MapField != "" {
			fieldMapping[field.FieldName] = *field.MapField
		}
	}

	log.Printf("[INGEST-SERVICE] Built field mapping with %d mapped fields", len(fieldMapping))

	// 5. Validate all records
	type validatedRecord struct {
		index     int
		record    map[string]interface{}
		uniqueKey string
	}
	validRecords := []validatedRecord{}
	errors := []entity.RecordError{}
	warnings := []string{}

	for i, record := range records {
		uniqueKey, recordErrors, recordWarnings := s.validateRecord(record, sourceFieldMap, mirror.UnmappedFieldMode, mirror.UniqueKeyField, i)

		if len(recordErrors) > 0 {
			errors = append(errors, recordErrors...)
			continue // Skip this record
		}

		if len(recordWarnings) > 0 {
			warnings = append(warnings, recordWarnings...)
		}

		validRecords = append(validRecords, validatedRecord{
			index:     i,
			record:    record,
			uniqueKey: uniqueKey,
		})
	}

	log.Printf("[INGEST-SERVICE] Validated %d/%d records, %d errors, %d warnings", len(validRecords), len(records), len(errors), len(warnings))

	// 6. Delta sync - check which unique keys already exist
	uniqueKeys := make([]string, len(validRecords))
	for i, vr := range validRecords {
		uniqueKeys[i] = vr.uniqueKey
	}

	existingKeys := make(map[string]bool)
	if len(uniqueKeys) > 0 {
		existingKeys, err = s.deltaKeyRepo.ExistsBatch(ctx, tenantDB, mirror.ID, uniqueKeys)
		if err != nil {
			log.Printf("[INGEST-SERVICE] Failed to check existing delta keys: %v", err)
			s.setJobFailed(ctx, tenantDB, job.ID, fmt.Sprintf("Failed to check existing delta keys: %v", err))
			return
		}
	}

	// Filter out records with existing unique keys (skipped)
	netNewRecords := []validatedRecord{}
	skippedCount := 0
	for _, vr := range validRecords {
		if existingKeys[vr.uniqueKey] {
			skippedCount++
		} else {
			netNewRecords = append(netNewRecords, vr)
		}
	}

	log.Printf("[INGEST-SERVICE] Delta check: %d net-new, %d skipped (already ingested)", len(netNewRecords), skippedCount)

	// 7. Promote net-new records to entity tables
	tableName := util.GetTableName(mirror.TargetEntity)
	promotedCount := 0
	deltaKeyEntries := []repo.DeltaKeyEntry{}

	// Load entity field definitions for EnsureTableExists
	entityFields, err := s.metadataRepo.ListFields(ctx, orgID, mirror.TargetEntity)
	if err != nil {
		log.Printf("[INGEST-SERVICE] Warning: failed to load entity fields for %s: %v (will create minimal table)", mirror.TargetEntity, err)
		entityFields = []entity.FieldDef{} // Continue with empty field list
	}

	// Ensure table exists before first insert
	if len(netNewRecords) > 0 {
		if err := util.EnsureTableExists(ctx, tenantDB, mirror.TargetEntity, entityFields); err != nil {
			log.Printf("[INGEST-SERVICE] Failed to ensure table exists: %v", err)
			s.setJobFailed(ctx, tenantDB, job.ID, fmt.Sprintf("Failed to ensure table exists: %v", err))
			return
		}
	}

	for _, vr := range netNewRecords {
		recordID, err := s.promoteRecord(ctx, tenantDB, orgID, vr.record, fieldMapping, tableName)
		if err != nil {
			log.Printf("[INGEST-SERVICE] Failed to promote record at index %d: %v", vr.index, err)
			errors = append(errors, entity.RecordError{
				Index:     vr.index,
				UniqueKey: vr.uniqueKey,
				Field:     "",
				Message:   err.Error(),
				Code:      "PROMOTION_FAILED",
			})
			continue
		}

		// Success - track for delta key insert
		promotedCount++
		deltaKeyEntries = append(deltaKeyEntries, repo.DeltaKeyEntry{
			UniqueKey: vr.uniqueKey,
			RecordID:  recordID,
		})
	}

	log.Printf("[INGEST-SERVICE] Promoted %d/%d net-new records", promotedCount, len(netNewRecords))

	// 8. Update delta key index
	if len(deltaKeyEntries) > 0 {
		if err := s.deltaKeyRepo.InsertBatch(ctx, tenantDB, orgID, mirror.ID, deltaKeyEntries); err != nil {
			log.Printf("[INGEST-SERVICE] Warning: failed to insert delta keys: %v", err)
			// Don't fail the job - records were already promoted
		}
	}

	// 9. Set final job result
	result := entity.IngestJobResult{
		RecordsProcessed: len(records),
		RecordsPromoted:  promotedCount,
		RecordsSkipped:   skippedCount,
		RecordsFailed:    len(records) - promotedCount - skippedCount,
		Errors:           errors,
		Warnings:         warnings,
	}

	if err := s.jobRepo.SetResult(ctx, tenantDB, job.ID, result); err != nil {
		log.Printf("[INGEST-SERVICE] Failed to set job result: %v", err)
		return
	}

	log.Printf("[INGEST-SERVICE] Completed job %s: processed=%d, promoted=%d, skipped=%d, failed=%d",
		job.ID, result.RecordsProcessed, result.RecordsPromoted, result.RecordsSkipped, result.RecordsFailed)
}

// validateRecord validates a single record against mirror source fields
// Returns: (uniqueKey, errors, warnings)
func (s *IngestService) validateRecord(
	record map[string]interface{},
	sourceFields map[string]*entity.MirrorSourceField,
	unmappedFieldMode string,
	uniqueKeyField string,
	index int,
) (string, []entity.RecordError, []string) {
	errors := []entity.RecordError{}
	warnings := []string{}

	// Check required fields
	for fieldName, field := range sourceFields {
		if field.IsRequired {
			if _, exists := record[fieldName]; !exists {
				errors = append(errors, entity.RecordError{
					Index:     index,
					UniqueKey: "",
					Field:     fieldName,
					Message:   fmt.Sprintf("Required field '%s' is missing", fieldName),
					Code:      "MISSING_REQUIRED_FIELD",
				})
			}
		}
	}

	// Check for unmapped fields
	unmappedFields := []string{}
	for fieldName := range record {
		if _, exists := sourceFields[fieldName]; !exists {
			unmappedFields = append(unmappedFields, fieldName)
		}
	}

	if len(unmappedFields) > 0 {
		if unmappedFieldMode == entity.UnmappedFieldModeStrict {
			// Strict mode: reject record
			errors = append(errors, entity.RecordError{
				Index:     index,
				UniqueKey: "",
				Field:     strings.Join(unmappedFields, ", "),
				Message:   fmt.Sprintf("Unmapped fields: %s", strings.Join(unmappedFields, ", ")),
				Code:      "UNMAPPED_FIELD",
			})
		} else {
			// Flexible mode: warn but continue
			warnings = append(warnings, fmt.Sprintf("Record %d: unmapped fields: %s", index, strings.Join(unmappedFields, ", ")))
		}
	}

	// Extract unique key
	uniqueKeyVal, exists := record[uniqueKeyField]
	if !exists {
		errors = append(errors, entity.RecordError{
			Index:     index,
			UniqueKey: "",
			Field:     uniqueKeyField,
			Message:   fmt.Sprintf("Unique key field '%s' is missing", uniqueKeyField),
			Code:      "MISSING_UNIQUE_KEY",
		})
		return "", errors, warnings
	}

	uniqueKey := fmt.Sprintf("%v", uniqueKeyVal)
	if uniqueKey == "" {
		errors = append(errors, entity.RecordError{
			Index:     index,
			UniqueKey: "",
			Field:     uniqueKeyField,
			Message:   fmt.Sprintf("Unique key field '%s' is empty", uniqueKeyField),
			Code:      "MISSING_UNIQUE_KEY",
		})
		return "", errors, warnings
	}

	return uniqueKey, errors, warnings
}

// promoteRecord inserts a single record into the entity table
// Returns: (recordID, error)
func (s *IngestService) promoteRecord(
	ctx context.Context,
	tenantDB db.DBConn,
	orgID string,
	record map[string]interface{},
	fieldMapping map[string]string,
	tableName string,
) (string, error) {
	// Generate record ID
	recordID := sfid.New("Rec")

	// Build column names and values
	columns := []string{"id", "org_id", "created_at", "modified_at"}
	placeholders := []string{"?", "?", "?", "?"}
	now := time.Now().UTC().Format(time.RFC3339)
	values := []interface{}{recordID, orgID, now, now}

	// Map source fields to target columns
	for sourceField, targetField := range fieldMapping {
		if value, exists := record[sourceField]; exists {
			columnName := util.CamelToSnake(targetField)
			columns = append(columns, util.QuoteIdentifier(columnName))
			placeholders = append(placeholders, "?")
			values = append(values, value)
		}
	}

	// Build INSERT statement
	sql := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := tenantDB.ExecContext(ctx, sql, values...)
	if err != nil {
		return "", fmt.Errorf("insert into %s: %w", tableName, err)
	}

	return recordID, nil
}

// setJobFailed is a helper to set a job to failed status with an error message
func (s *IngestService) setJobFailed(ctx context.Context, tenantDB db.DBConn, jobID, errorMsg string) {
	result := entity.IngestJobResult{
		RecordsProcessed: 0,
		RecordsPromoted:  0,
		RecordsSkipped:   0,
		RecordsFailed:    0,
		Errors: []entity.RecordError{
			{
				Index:     -1,
				UniqueKey: "",
				Field:     "",
				Message:   errorMsg,
				Code:      "JOB_FAILED",
			},
		},
		Warnings: []string{},
	}
	_ = s.jobRepo.SetResult(ctx, tenantDB, jobID, result)
}
