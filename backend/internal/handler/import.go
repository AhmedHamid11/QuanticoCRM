package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// ImportHandler handles CSV import operations
type ImportHandler struct {
	db                *sql.DB
	metadataRepo      *repo.MetadataRepo
	csvParser         *service.CSVParser
	csvValidator      *service.CSVValidatorService
	tripwireService   TripwireServiceInterface
	validationService ValidationServiceInterface
}

// NewImportHandler creates a new ImportHandler
func NewImportHandler(
	db *sql.DB,
	metadataRepo *repo.MetadataRepo,
	tripwireService TripwireServiceInterface,
	validationService ValidationServiceInterface,
) *ImportHandler {
	return &ImportHandler{
		db:                db,
		metadataRepo:      metadataRepo,
		csvParser:         service.NewCSVParser(),
		csvValidator:      service.NewCSVValidatorService(),
		tripwireService:   tripwireService,
		validationService: validationService,
	}
}

// getMetadataRepo returns a metadata repo using the tenant database from context
func (h *ImportHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// getDB returns the tenant database from context, falling back to default db
func (h *ImportHandler) getDB(c *fiber.Ctx) *sql.DB {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.db
}

// ImportMode defines the operation mode for CSV import
type ImportMode string

const (
	ImportModeCreate ImportMode = "create" // Insert new records (default)
	ImportModeUpdate ImportMode = "update" // Update existing records by ID
	ImportModeUpsert ImportMode = "upsert" // Create or update based on match field
	ImportModeDelete ImportMode = "delete" // Delete records by ID or match field
)

// ImportCSVRequest represents optional parameters for CSV import
type ImportCSVRequest struct {
	Mode          ImportMode        `json:"mode"`          // create, update, upsert, delete (default: create)
	MatchField    string            `json:"matchField"`    // Field to match on for upsert/update/delete (e.g., "email")
	ColumnMapping map[string]string `json:"columnMapping"` // CSV header -> field name
	SkipErrors    bool              `json:"skipErrors"`
	FireTripwires bool              `json:"fireTripwires"`
	ValidateOnly  bool              `json:"validateOnly"`
}

// ImportCSVResponse represents the response from a CSV import
type ImportCSVResponse struct {
	Mode          ImportMode               `json:"mode,omitempty"`
	Created       int                      `json:"created,omitempty"`
	Updated       int                      `json:"updated,omitempty"`
	Deleted       int                      `json:"deleted,omitempty"`
	Skipped       int                      `json:"skipped,omitempty"` // Records skipped (not found for update/delete)
	Failed        int                      `json:"failed"`
	Validated     int                      `json:"validated,omitempty"`
	TotalRows     int                      `json:"totalRows"`
	Headers       []string                 `json:"headers,omitempty"`
	MappedHeaders []string                 `json:"mappedHeaders,omitempty"`
	Errors        []BulkError              `json:"errors,omitempty"`
	IDs           []string                 `json:"ids,omitempty"`
}

// PreviewCSVResponse represents a preview of CSV data before import
type PreviewCSVResponse struct {
	Headers       []string                   `json:"headers"`
	MappedHeaders []string                   `json:"mappedHeaders"`
	SampleRows    []map[string]interface{}   `json:"sampleRows"`
	TotalRows     int                        `json:"totalRows"`
	UnmappedCols  []string                   `json:"unmappedColumns"`
	Fields        []FieldMapping             `json:"fields"`
}

// FieldMapping shows how a CSV column maps to an entity field
type FieldMapping struct {
	CSVHeader  string `json:"csvHeader"`
	FieldName  string `json:"fieldName"`
	FieldLabel string `json:"fieldLabel"`
	FieldType  string `json:"fieldType"`
	Mapped     bool   `json:"mapped"`
}

// getTableName converts entity name to table name
func (h *ImportHandler) getTableName(entityName string) string {
	return util.GetTableName(entityName)
}

// ImportCSV handles POST /api/v1/entities/:entity/import/csv
func (h *ImportHandler) ImportCSV(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	entityName := c.Params("entity")

	// Verify entity exists
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Get the uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded. Use 'file' field in multipart form."})
	}

	// Validate file size (max 50MB)
	if fileHeader.Size > 50*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File too large. Maximum size is 50MB."})
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid file type. Only CSV files are accepted."})
	}

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	// Read file content into buffer (for potential re-reading)
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	// Parse options from form data
	var options ImportCSVRequest
	if optionsStr := c.FormValue("options"); optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid options JSON"})
		}
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse CSV
	var parseResult *service.CSVParseResult
	if len(options.ColumnMapping) > 0 {
		parseResult, err = h.csvParser.ParseWithMapping(bytes.NewReader(fileContent), options.ColumnMapping)
	} else {
		parseResult, err = h.csvParser.Parse(bytes.NewReader(fileContent), fields)
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Check for parse errors
	if len(parseResult.Errors) > 0 && !options.SkipErrors {
		var bulkErrors []BulkError
		for _, e := range parseResult.Errors {
			bulkErrors = append(bulkErrors, BulkError{
				Index: e.Row,
				Error: e.Message,
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(ImportCSVResponse{
			Failed:    len(parseResult.Errors),
			TotalRows: parseResult.RowCount + len(parseResult.Errors),
			Errors:    bulkErrors,
		})
	}

	if len(parseResult.Records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No valid records found in CSV"})
	}

	// Default mode is create
	mode := options.Mode
	if mode == "" {
		mode = ImportModeCreate
	}

	// Validate mode-specific requirements
	if (mode == ImportModeUpsert || mode == ImportModeUpdate || mode == ImportModeDelete) && options.MatchField == "" {
		// Check if 'id' column is present
		hasID := false
		for _, h := range parseResult.MappedHeaders {
			if h == "id" {
				hasID = true
				break
			}
		}
		if !hasID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Mode '%s' requires either 'matchField' option or 'id' column in CSV", mode),
			})
		}
		options.MatchField = "id"
	}

	// Validate matchField exists
	if options.MatchField != "" && options.MatchField != "id" {
		found := false
		for _, f := range fields {
			if f.Name == options.MatchField {
				found = true
				break
			}
		}
		if !found {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Match field '%s' does not exist on entity", options.MatchField),
			})
		}
	}

	// Ensure table exists
	if err := h.ensureTableExists(c.Context(), h.getDB(c), orgID, entityName, fields); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	tableName := h.getTableName(entityName)
	now := time.Now().UTC().Format(time.RFC3339)

	var response ImportCSVResponse
	response.Mode = mode
	response.Headers = parseResult.Headers
	response.MappedHeaders = parseResult.MappedHeaders
	response.TotalRows = parseResult.RowCount

	// Determine validation operation based on mode
	validationOp := "CREATE"
	if mode == ImportModeUpdate || mode == ImportModeUpsert {
		validationOp = "UPDATE"
	}

	var errors []BulkError
	failedIndices := make(map[int]bool)

	// Validation pass (skip for delete mode)
	if h.validationService != nil && mode != ImportModeDelete {
		for i, record := range parseResult.Records {
			result, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, "", validationOp, nil, record)
			if err != nil {
				errors = append(errors, BulkError{
					Index: i,
					Error: err.Error(),
				})
				failedIndices[i] = true
				continue
			}
			if !result.Valid {
				errors = append(errors, BulkError{
					Index:       i,
					Error:       result.Message,
					FieldErrors: result.FieldErrors,
				})
				failedIndices[i] = true
			}
		}

		if len(errors) > 0 && !options.SkipErrors {
			response.Failed = len(errors)
			response.Errors = errors
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}
	}

	// Validate only mode
	if options.ValidateOnly {
		response.Validated = len(parseResult.Records) - len(errors)
		response.Failed = len(errors)
		response.Errors = errors
		return c.JSON(response)
	}

	// Process based on mode
	switch mode {
	case ImportModeCreate:
		return h.processCreateMode(c, orgID, userID, entityName, tableName, fields, parseResult.Records, failedIndices, now, options, &response, errors)
	case ImportModeUpdate:
		return h.processUpdateMode(c, orgID, userID, entityName, tableName, fields, parseResult.Records, failedIndices, now, options, &response, errors)
	case ImportModeUpsert:
		return h.processUpsertMode(c, orgID, userID, entityName, tableName, fields, parseResult.Records, failedIndices, now, options, &response, errors)
	case ImportModeDelete:
		return h.processDeleteMode(c, orgID, entityName, tableName, fields, parseResult.Records, options, &response)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid mode: %s", mode)})
	}
}

// processCreateMode handles create (insert) operations
func (h *ImportHandler) processCreateMode(
	c *fiber.Ctx,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	failedIndices map[int]bool,
	now string,
	options ImportCSVRequest,
	response *ImportCSVResponse,
	errors []BulkError,
) error {
	var createdIDs []string

	batchSize := DefaultBatchSize
	for batchStart := 0; batchStart < len(records); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(records) {
			batchEnd = len(records)
		}

		batch := records[batchStart:batchEnd]
		batchIDs, batchErrors := h.processBatch(c.Context(), h.getDB(c), orgID, userID, tableName, fields, batch, batchStart, failedIndices, now, options.SkipErrors)

		createdIDs = append(createdIDs, batchIDs...)
		errors = append(errors, batchErrors...)

		// Fire tripwires
		if options.FireTripwires && h.tripwireService != nil {
			for i, id := range batchIDs {
				recordIdx := batchStart + i
				if failedIndices[recordIdx] {
					continue
				}
				go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "CREATE", nil, records[recordIdx])
			}
		}
	}

	response.Created = len(createdIDs)
	response.Failed = len(errors)
	response.IDs = createdIDs
	if len(errors) > 0 {
		response.Errors = errors
	}

	if response.Failed > 0 && !options.SkipErrors {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// processUpdateMode handles update operations
func (h *ImportHandler) processUpdateMode(
	c *fiber.Ctx,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	failedIndices map[int]bool,
	now string,
	options ImportCSVRequest,
	response *ImportCSVResponse,
	errors []BulkError,
) error {
	var updatedIDs []string
	var skipped int

	for i, record := range records {
		if failedIndices[i] {
			continue
		}

		// Find existing record
		existingID, oldRecord, err := h.findExistingRecord(c.Context(), h.getDB(c), orgID, tableName, options.MatchField, record, fields)
		if err != nil {
			if options.SkipErrors {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				continue
			}
			response.Failed = len(errors) + 1
			response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}

		if existingID == "" {
			// Record not found - skip
			skipped++
			continue
		}

		// Update the record
		if err := h.updateRecord(c.Context(), h.getDB(c), orgID, userID, tableName, existingID, fields, record, now); err != nil {
			if options.SkipErrors {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				continue
			}
			response.Failed = len(errors) + 1
			response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}

		updatedIDs = append(updatedIDs, existingID)

		// Fire tripwires
		if options.FireTripwires && h.tripwireService != nil {
			go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, existingID, "UPDATE", oldRecord, record)
		}
	}

	response.Updated = len(updatedIDs)
	response.Skipped = skipped
	response.Failed = len(errors)
	response.IDs = updatedIDs
	if len(errors) > 0 {
		response.Errors = errors
	}

	return c.JSON(response)
}

// processUpsertMode handles upsert (create or update) operations
func (h *ImportHandler) processUpsertMode(
	c *fiber.Ctx,
	orgID, userID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	failedIndices map[int]bool,
	now string,
	options ImportCSVRequest,
	response *ImportCSVResponse,
	errors []BulkError,
) error {
	var createdIDs []string
	var updatedIDs []string

	for i, record := range records {
		if failedIndices[i] {
			continue
		}

		// Find existing record
		existingID, oldRecord, err := h.findExistingRecord(c.Context(), h.getDB(c), orgID, tableName, options.MatchField, record, fields)
		if err != nil {
			if options.SkipErrors {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				continue
			}
			response.Failed = len(errors) + 1
			response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}

		if existingID != "" {
			// Update existing record
			if err := h.updateRecord(c.Context(), h.getDB(c), orgID, userID, tableName, existingID, fields, record, now); err != nil {
				if options.SkipErrors {
					errors = append(errors, BulkError{Index: i, Error: err.Error()})
					continue
				}
				response.Failed = len(errors) + 1
				response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
				return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
			}
			updatedIDs = append(updatedIDs, existingID)

			// Fire tripwires for update
			if options.FireTripwires && h.tripwireService != nil {
				go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, existingID, "UPDATE", oldRecord, record)
			}
		} else {
			// Create new record
			tx, err := h.getDB(c).BeginTx(c.Context(), nil)
			if err != nil {
				if options.SkipErrors {
					errors = append(errors, BulkError{Index: i, Error: err.Error()})
					continue
				}
				response.Failed = len(errors) + 1
				response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
				return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
			}

			id, err := h.insertRecord(c.Context(), tx, orgID, userID, tableName, fields, record, now)
			if err != nil {
				tx.Rollback()
				if options.SkipErrors {
					errors = append(errors, BulkError{Index: i, Error: err.Error()})
					continue
				}
				response.Failed = len(errors) + 1
				response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
				return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
			}

			if err := tx.Commit(); err != nil {
				if options.SkipErrors {
					errors = append(errors, BulkError{Index: i, Error: err.Error()})
					continue
				}
				response.Failed = len(errors) + 1
				response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
				return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
			}

			createdIDs = append(createdIDs, id)

			// Fire tripwires for create
			if options.FireTripwires && h.tripwireService != nil {
				go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "CREATE", nil, record)
			}
		}
	}

	response.Created = len(createdIDs)
	response.Updated = len(updatedIDs)
	response.Failed = len(errors)
	response.IDs = append(createdIDs, updatedIDs...)
	if len(errors) > 0 {
		response.Errors = errors
	}

	status := fiber.StatusOK
	if len(createdIDs) > 0 && len(updatedIDs) == 0 {
		status = fiber.StatusCreated
	}

	return c.Status(status).JSON(response)
}

// processDeleteMode handles delete operations
func (h *ImportHandler) processDeleteMode(
	c *fiber.Ctx,
	orgID, entityName, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	options ImportCSVRequest,
	response *ImportCSVResponse,
) error {
	var deletedIDs []string
	var errors []BulkError
	var skipped int

	for i, record := range records {
		// Find existing record
		existingID, _, err := h.findExistingRecord(c.Context(), h.getDB(c), orgID, tableName, options.MatchField, record, fields)
		if err != nil {
			if options.SkipErrors {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				continue
			}
			response.Failed = len(errors) + 1
			response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}

		if existingID == "" {
			// Record not found - skip
			skipped++
			continue
		}

		// Delete the record
		query := fmt.Sprintf("DELETE FROM %s WHERE id = ? AND org_id = ?", tableName)
		_, err = h.getDB(c).ExecContext(c.Context(), query, existingID, orgID)
		if err != nil {
			if options.SkipErrors {
				errors = append(errors, BulkError{Index: i, Error: err.Error()})
				continue
			}
			response.Failed = len(errors) + 1
			response.Errors = append(errors, BulkError{Index: i, Error: err.Error()})
			return c.Status(fiber.StatusUnprocessableEntity).JSON(response)
		}

		deletedIDs = append(deletedIDs, existingID)

		// Fire tripwires for delete
		if options.FireTripwires && h.tripwireService != nil {
			go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, existingID, "DELETE", record, nil)
		}
	}

	response.Deleted = len(deletedIDs)
	response.Skipped = skipped
	response.Failed = len(errors)
	response.IDs = deletedIDs
	if len(errors) > 0 {
		response.Errors = errors
	}

	return c.JSON(response)
}

// findExistingRecord finds an existing record by match field
func (h *ImportHandler) findExistingRecord(
	ctx context.Context,
	db *sql.DB,
	orgID, tableName, matchField string,
	record map[string]interface{},
	fields []entity.FieldDef,
) (string, map[string]interface{}, error) {
	// Get the match value from the record
	matchValue, ok := record[matchField]
	if !ok {
		return "", nil, fmt.Errorf("match field '%s' not found in record", matchField)
	}

	// Build the column name
	colName := matchField
	if matchField != "id" {
		colName = util.CamelToSnake(matchField)
	}

	// Query for existing record
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ? AND org_id = ? LIMIT 1",
		tableName, quoteIdentifier(colName))

	rows, err := db.QueryContext(ctx, query, matchValue, orgID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return "", nil, nil // Not found
	}

	// Scan into map
	columns, err := rows.Columns()
	if err != nil {
		return "", nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return "", nil, err
	}

	oldRecord := make(map[string]interface{})
	var existingID string
	for i, col := range columns {
		if col == "id" {
			if v, ok := values[i].(string); ok {
				existingID = v
			}
		}
		oldRecord[col] = values[i]
	}

	return existingID, oldRecord, nil
}

// updateRecord updates an existing record
func (h *ImportHandler) updateRecord(
	ctx context.Context,
	db *sql.DB,
	orgID, userID, tableName, id string,
	fields []entity.FieldDef,
	record map[string]interface{},
	now string,
) error {
	var setClauses []string
	var values []interface{}

	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Handle lookup fields
		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := record[idKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_id")))
				values = append(values, idVal)
			}
			if nameVal, ok := record[nameKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_name")))
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := record[idsKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_ids")))
				values = append(values, idsVal)
			}
			if namesVal, ok := record[namesKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_names")))
				values = append(values, namesVal)
			}
			continue
		}

		// Regular fields
		if val, ok := record[field.Name]; ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(util.CamelToSnake(field.Name))))
			values = append(values, val)
		}
	}

	if len(setClauses) == 0 {
		return nil // Nothing to update
	}

	// Add modified timestamps
	setClauses = append(setClauses, "modified_at = ?", "modified_by_id = ?")
	values = append(values, now, userID)

	// Add WHERE clause values
	values = append(values, id, orgID)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND org_id = ?",
		tableName, strings.Join(setClauses, ", "))

	_, err := db.ExecContext(ctx, query, values...)
	return err
}

// AnalyzeCSV handles POST /api/v1/entities/:entity/import/csv/analyze
func (h *ImportHandler) AnalyzeCSV(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Verify entity exists
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Get the uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded. Use 'file' field in multipart form."})
	}

	// Validate file size (max 50MB)
	if fileHeader.Size > 50*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File too large. Maximum size is 50MB."})
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid file type. Only CSV files are accepted."})
	}

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	// Read file content into buffer
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	// Parse options from form data (for column mapping)
	var options ImportCSVRequest
	if optionsStr := c.FormValue("options"); optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid options JSON"})
		}
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse CSV
	var parseResult *service.CSVParseResult
	if len(options.ColumnMapping) > 0 {
		parseResult, err = h.csvParser.ParseWithMapping(bytes.NewReader(fileContent), options.ColumnMapping)
	} else {
		parseResult, err = h.csvParser.Parse(bytes.NewReader(fileContent), fields)
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Check for parse errors
	if len(parseResult.Errors) > 0 {
		var bulkErrors []BulkError
		for _, e := range parseResult.Errors {
			bulkErrors = append(bulkErrors, BulkError{
				Index: e.Row,
				Error: e.Message,
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  "CSV parsing errors",
			"errors": bulkErrors,
		})
	}

	if len(parseResult.Records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No valid records found in CSV"})
	}

	// Validate records
	analyzeResult := h.csvValidator.Validate(parseResult.Records, fields)

	return c.JSON(analyzeResult)
}

// PreviewCSV handles POST /api/v1/entities/:entity/import/csv/preview
func (h *ImportHandler) PreviewCSV(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Verify entity exists
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Get the uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	// Read file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Get sample rows for preview
	headers, sampleRows, err := h.csvParser.GetSampleRows(bytes.NewReader(fileContent), 5)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse full file to get total count and mapping
	parseResult, err := h.csvParser.Parse(bytes.NewReader(fileContent), fields)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Build field mapping info
	var fieldMappings []FieldMapping
	var unmappedCols []string

	for i, header := range headers {
		var mappedField string
		if i < len(parseResult.MappedHeaders) {
			mappedField = parseResult.MappedHeaders[i]
		}

		mapping := FieldMapping{
			CSVHeader: header,
			FieldName: mappedField,
			Mapped:    mappedField != "",
		}

		// Find field details
		if mappedField != "" {
			for _, field := range fields {
				if field.Name == mappedField ||
				   field.Name+"Id" == mappedField ||
				   field.Name+"Name" == mappedField ||
				   field.Name+"Ids" == mappedField ||
				   field.Name+"Names" == mappedField {
					mapping.FieldLabel = field.Label
					mapping.FieldType = string(field.Type)
					break
				}
			}
		} else {
			unmappedCols = append(unmappedCols, header)
		}

		fieldMappings = append(fieldMappings, mapping)
	}

	// Convert sample rows to maps
	var sampleMaps []map[string]interface{}
	for _, row := range sampleRows {
		record := make(map[string]interface{})
		for i, val := range row {
			if i < len(headers) {
				record[headers[i]] = val
			}
		}
		sampleMaps = append(sampleMaps, record)
	}

	return c.JSON(PreviewCSVResponse{
		Headers:       headers,
		MappedHeaders: parseResult.MappedHeaders,
		SampleRows:    sampleMaps,
		TotalRows:     parseResult.RowCount,
		UnmappedCols:  unmappedCols,
		Fields:        fieldMappings,
	})
}

// processBatch processes a batch of records (reused from bulk handler pattern)
func (h *ImportHandler) processBatch(
	ctx context.Context,
	db *sql.DB,
	orgID, userID, tableName string,
	fields []entity.FieldDef,
	records []map[string]interface{},
	batchOffset int,
	failedIndices map[int]bool,
	now string,
	skipErrors bool,
) ([]string, []BulkError) {
	var createdIDs []string
	var errors []BulkError

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		for i := range records {
			errors = append(errors, BulkError{
				Index: batchOffset + i,
				Error: "Failed to start transaction: " + err.Error(),
			})
		}
		return nil, errors
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for i, record := range records {
		globalIndex := batchOffset + i

		if failedIndices[globalIndex] {
			continue
		}

		id, err := h.insertRecord(ctx, tx, orgID, userID, tableName, fields, record, now)
		if err != nil {
			if skipErrors {
				errors = append(errors, BulkError{
					Index: globalIndex,
					Error: err.Error(),
				})
				continue
			}
			tx.Rollback()
			return nil, []BulkError{{
				Index: globalIndex,
				Error: err.Error(),
			}}
		}
		createdIDs = append(createdIDs, id)
	}

	if err := tx.Commit(); err != nil {
		return nil, []BulkError{{
			Index: batchOffset,
			Error: "Failed to commit transaction: " + err.Error(),
		}}
	}

	return createdIDs, errors
}

// insertRecord inserts a single record within a transaction
func (h *ImportHandler) insertRecord(
	ctx context.Context,
	tx *sql.Tx,
	orgID, userID, tableName string,
	fields []entity.FieldDef,
	record map[string]interface{},
	now string,
) (string, error) {
	var columns []string
	var placeholders []string
	var values []interface{}

	id := sfid.New("Rec")
	columns = append(columns, "id")
	placeholders = append(placeholders, "?")
	values = append(values, id)

	columns = append(columns, "org_id")
	placeholders = append(placeholders, "?")
	values = append(values, orgID)

	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Handle lookup fields
		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := record[idKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_id"))
				placeholders = append(placeholders, "?")
				values = append(values, idVal)
			}
			if nameVal, ok := record[nameKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_name"))
				placeholders = append(placeholders, "?")
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := record[idsKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_ids"))
				placeholders = append(placeholders, "?")
				values = append(values, idsVal)
			}
			if namesVal, ok := record[namesKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_names"))
				placeholders = append(placeholders, "?")
				values = append(values, namesVal)
			}
			continue
		}

		// Regular fields
		if val, ok := record[field.Name]; ok {
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, val)
		} else if field.DefaultValue != nil && *field.DefaultValue != "" {
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, *field.DefaultValue)
		}
	}

	columns = append(columns, "created_at", "modified_at", "created_by_id", "modified_by_id")
	placeholders = append(placeholders, "?", "?", "?", "?")
	values = append(values, now, now, userID, userID)

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err := tx.ExecContext(ctx, query, values...)
	if err != nil {
		return "", err
	}

	return id, nil
}

// ensureTableExists creates the table if it doesn't exist
func (h *ImportHandler) ensureTableExists(ctx context.Context, db *sql.DB, orgID, entityName string, fields []entity.FieldDef) error {
	tableName := h.getTableName(entityName)

	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	var columns []string
	columns = append(columns, "id TEXT PRIMARY KEY")
	columns = append(columns, "org_id TEXT NOT NULL")

	for _, field := range fields {
		if field.Name == "id" || field.Name == "created_at" || field.Name == "modified_at" {
			continue
		}

		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			columns = append(columns, fmt.Sprintf("%s TEXT", quoteIdentifier(snakeName+"_id")))
			columns = append(columns, fmt.Sprintf("%s TEXT DEFAULT ''", quoteIdentifier(snakeName+"_name")))
			continue
		}

		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			columns = append(columns, fmt.Sprintf("%s TEXT DEFAULT '[]'", quoteIdentifier(snakeName+"_ids")))
			columns = append(columns, fmt.Sprintf("%s TEXT DEFAULT '[]'", quoteIdentifier(snakeName+"_names")))
			continue
		}

		colType := "TEXT"
		switch field.Type {
		case "int":
			colType = "INTEGER"
		case "float", "currency":
			colType = "REAL"
		case "bool":
			colType = "INTEGER"
		}

		colDef := fmt.Sprintf("%s %s", quoteIdentifier(util.CamelToSnake(field.Name)), colType)
		if field.IsRequired {
			colDef += " NOT NULL"
		}
		columns = append(columns, colDef)
	}

	columns = append(columns, "created_at TEXT DEFAULT CURRENT_TIMESTAMP")
	columns = append(columns, "modified_at TEXT DEFAULT CURRENT_TIMESTAMP")
	columns = append(columns, "created_by_id TEXT")
	columns = append(columns, "modified_by_id TEXT")

	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(columns, ", "))
	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		return err
	}

	indexSQL := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_org_id ON %s(org_id)", tableName, tableName)
	_, err = db.ExecContext(ctx, indexSQL)
	return err
}

// RegisterRoutes registers import routes
func (h *ImportHandler) RegisterRoutes(app fiber.Router) {
	app.Post("/entities/:entity/import/csv", h.ImportCSV)
	app.Post("/entities/:entity/import/csv/preview", h.PreviewCSV)
	app.Post("/entities/:entity/import/csv/analyze", h.AnalyzeCSV)
}
