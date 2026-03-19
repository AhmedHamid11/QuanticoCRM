package handler

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/gofiber/fiber/v2"
)

// MirrorHandler handles mirror management endpoints
type MirrorHandler struct {
	repo          *repo.MirrorRepo
	jobRepo       *repo.IngestJobRepo
	provisioning  *service.ProvisioningService
	ingestService *service.IngestService
	metadataRepo  *repo.MetadataRepo
	sequenceRepo  *repo.SequenceRepo
}

// NewMirrorHandler creates a new MirrorHandler
func NewMirrorHandler(repo *repo.MirrorRepo, jobRepo *repo.IngestJobRepo, provisioning *service.ProvisioningService) *MirrorHandler {
	return &MirrorHandler{repo: repo, jobRepo: jobRepo, provisioning: provisioning}
}

// SetIngestService sets the ingest service for CSV import processing
func (h *MirrorHandler) SetIngestService(svc *service.IngestService) {
	h.ingestService = svc
}

// SetMetadataRepo sets the metadata repo for loading entity field definitions
func (h *MirrorHandler) SetMetadataRepo(r *repo.MetadataRepo) {
	h.metadataRepo = r
}

// SetSequenceRepo wires in the SequenceRepo for watermark queries (Phase 36-02).
func (h *MirrorHandler) SetSequenceRepo(r *repo.SequenceRepo) {
	h.sequenceRepo = r
}

// getTenantDBConn extracts the tenant DB connection from context
func (h *MirrorHandler) getTenantDBConn(c *fiber.Ctx) db.DBConn {
	return middleware.GetTenantDBConn(c)
}

// tryEnsureIngestTables creates ingest tables on the tenant DB using provisioning service
func (h *MirrorHandler) tryEnsureIngestTables(c *fiber.Ctx) error {
	tenantDB := h.getTenantDBConn(c)
	ps := service.NewProvisioningService(tenantDB)
	return ps.EnsureIngestTables(c.Context())
}

// Create creates a new mirror
// POST /mirrors
func (h *MirrorHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	tenantDB := h.getTenantDBConn(c)

	var input entity.MirrorCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON payload"})
	}

	// Validate required fields
	if input.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	if input.TargetEntity == "" {
		return c.Status(400).JSON(fiber.Map{"error": "targetEntity is required"})
	}
	if input.UniqueKeyField == "" {
		return c.Status(400).JSON(fiber.Map{"error": "uniqueKeyField is required"})
	}

	// Validate unmapped field mode if provided
	if input.UnmappedFieldMode != "" && !entity.ValidateUnmappedFieldMode(input.UnmappedFieldMode) {
		return c.Status(400).JSON(fiber.Map{
			"error":          "invalid unmappedFieldMode",
			"validValues":    []string{entity.UnmappedFieldModeStrict, entity.UnmappedFieldModeFlexible},
			"receivedValue":  input.UnmappedFieldMode,
		})
	}

	// Validate source field types if provided
	for i, field := range input.SourceFields {
		if field.FieldName == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "sourceFields[" + string(rune(i)) + "].fieldName is required",
			})
		}
		if field.FieldType != "" && !entity.ValidateFieldType(field.FieldType) {
			return c.Status(400).JSON(fiber.Map{
				"error":         "invalid field type",
				"field":         field.FieldName,
				"validTypes":    entity.ValidFieldTypes,
				"receivedType":  field.FieldType,
			})
		}
	}

	// Create mirror (with auto-recovery on missing tables)
	mirror, err := h.repo.Create(c.Context(), tenantDB, orgID, input)
	if err != nil && isNoSuchTableError(err) {
		log.Printf("[MirrorHandler] 'no such table' error, attempting to create ingest tables for org %s", orgID)
		if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
			log.Printf("[MirrorHandler] Failed to create ingest tables: %v", ensureErr)
			return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized. Please contact admin."})
		}
		// Retry the operation
		mirror, err = h.repo.Create(c.Context(), tenantDB, orgID, input)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(mirror)
}

// List retrieves all mirrors for the organization
// GET /mirrors
func (h *MirrorHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	tenantDB := h.getTenantDBConn(c)

	mirrors, err := h.repo.ListByOrg(c.Context(), tenantDB, orgID)
	if err != nil && isNoSuchTableError(err) {
		log.Printf("[MirrorHandler] 'no such table' error on List, attempting to create ingest tables for org %s", orgID)
		if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
			log.Printf("[MirrorHandler] Failed to create ingest tables: %v", ensureErr)
			return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized. Please contact admin."})
		}
		mirrors, err = h.repo.ListByOrg(c.Context(), tenantDB, orgID)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(mirrors)
}

// Get retrieves a single mirror by ID
// GET /mirrors/:id
func (h *MirrorHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	mirrorID := c.Params("id")
	tenantDB := h.getTenantDBConn(c)

	mirror, err := h.repo.GetByID(c.Context(), tenantDB, orgID, mirrorID)
	if err != nil && isNoSuchTableError(err) {
		log.Printf("[MirrorHandler] 'no such table' error on Get, attempting to create ingest tables for org %s", orgID)
		if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
			log.Printf("[MirrorHandler] Failed to create ingest tables: %v", ensureErr)
			return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized. Please contact admin."})
		}
		mirror, err = h.repo.GetByID(c.Context(), tenantDB, orgID, mirrorID)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if mirror == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Mirror not found"})
	}

	return c.JSON(mirror)
}

// Update updates a mirror
// PUT /mirrors/:id
func (h *MirrorHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	mirrorID := c.Params("id")
	tenantDB := h.getTenantDBConn(c)

	var input entity.MirrorUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON payload"})
	}

	// Validate unmapped field mode if provided
	if input.UnmappedFieldMode != nil && !entity.ValidateUnmappedFieldMode(*input.UnmappedFieldMode) {
		return c.Status(400).JSON(fiber.Map{
			"error":          "invalid unmappedFieldMode",
			"validValues":    []string{entity.UnmappedFieldModeStrict, entity.UnmappedFieldModeFlexible},
			"receivedValue":  *input.UnmappedFieldMode,
		})
	}

	// Validate source field types if provided
	if input.SourceFields != nil {
		for i, field := range *input.SourceFields {
			if field.FieldName == "" {
				return c.Status(400).JSON(fiber.Map{
					"error": "sourceFields[" + string(rune(i)) + "].fieldName is required",
				})
			}
			if field.FieldType != "" && !entity.ValidateFieldType(field.FieldType) {
				return c.Status(400).JSON(fiber.Map{
					"error":         "invalid field type",
					"field":         field.FieldName,
					"validTypes":    entity.ValidFieldTypes,
					"receivedType":  field.FieldType,
				})
			}
		}
	}

	// Update mirror (with auto-recovery on missing tables)
	mirror, err := h.repo.Update(c.Context(), tenantDB, orgID, mirrorID, input)
	if err != nil && isNoSuchTableError(err) {
		log.Printf("[MirrorHandler] 'no such table' error on Update, attempting to create ingest tables for org %s", orgID)
		if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
			log.Printf("[MirrorHandler] Failed to create ingest tables: %v", ensureErr)
			return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized. Please contact admin."})
		}
		mirror, err = h.repo.Update(c.Context(), tenantDB, orgID, mirrorID, input)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if mirror == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Mirror not found"})
	}

	return c.JSON(mirror)
}

// Delete deletes a mirror
// DELETE /mirrors/:id
func (h *MirrorHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	mirrorID := c.Params("id")
	tenantDB := h.getTenantDBConn(c)

	err := h.repo.Delete(c.Context(), tenantDB, orgID, mirrorID)
	if err != nil && isNoSuchTableError(err) {
		log.Printf("[MirrorHandler] 'no such table' error on Delete, attempting to create ingest tables for org %s", orgID)
		if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
			log.Printf("[MirrorHandler] Failed to create ingest tables: %v", ensureErr)
			return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized. Please contact admin."})
		}
		err = h.repo.Delete(c.Context(), tenantDB, orgID, mirrorID)
	}
	if err != nil {
		if err.Error() == "mirror not found" {
			return c.Status(404).JSON(fiber.Map{"error": "Mirror not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Mirror deleted successfully"})
}

// ListJobs retrieves ingest jobs for a specific mirror
// GET /mirrors/:id/jobs
func (h *MirrorHandler) ListJobs(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	mirrorID := c.Params("id")
	tenantDB := h.getTenantDBConn(c)

	// Validate mirror exists and belongs to org
	mirror, err := h.repo.GetByID(c.Context(), tenantDB, orgID, mirrorID)
	if err != nil && isNoSuchTableError(err) {
		log.Printf("[MirrorHandler] 'no such table' error on ListJobs, attempting to create ingest tables for org %s", orgID)
		if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
			log.Printf("[MirrorHandler] Failed to create ingest tables: %v", ensureErr)
			return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized. Please contact admin."})
		}
		mirror, err = h.repo.GetByID(c.Context(), tenantDB, orgID, mirrorID)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if mirror == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Mirror not found"})
	}

	// Parse limit query param (default 50, max 200)
	limit := c.QueryInt("limit", 50)
	if limit > 200 {
		limit = 200
	}
	if limit < 1 {
		limit = 1
	}

	jobs, err := h.jobRepo.ListByMirror(c.Context(), tenantDB, orgID, mirrorID, limit)
	if err != nil && isNoSuchTableError(err) {
		log.Printf("[MirrorHandler] 'no such table' error on ListJobs (jobs query), attempting to create ingest tables for org %s", orgID)
		if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
			log.Printf("[MirrorHandler] Failed to create ingest tables: %v", ensureErr)
			return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized. Please contact admin."})
		}
		jobs, err = h.jobRepo.ListByMirror(c.Context(), tenantDB, orgID, mirrorID, limit)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

// ImportCSV handles POST /admin/mirrors/:id/import-csv
// Accepts a CSV file via multipart/form-data and feeds it through the ingest pipeline
// Uses JWT auth (admin UI) instead of X-API-Key
func (h *MirrorHandler) ImportCSV(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	mirrorID := c.Params("id")
	tenantDB := h.getTenantDBConn(c)

	if h.ingestService == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ingest service not configured",
		})
	}

	// Load mirror config with source fields
	mirror, err := h.repo.GetActiveByID(c.Context(), tenantDB, orgID, mirrorID)
	if err != nil && isNoSuchTableError(err) {
		if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized"})
		}
		mirror, err = h.repo.GetActiveByID(c.Context(), tenantDB, orgID, mirrorID)
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load mirror"})
	}
	if mirror == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Mirror not found or inactive"})
	}
	if mirror.TargetEntity == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Mirror has no target entity configured"})
	}

	// Parse multipart form and extract CSV file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing or invalid 'file' field"})
	}
	if fileHeader.Size > 50*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File exceeds 50MB limit"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read uploaded file"})
	}
	defer file.Close()

	// Parse CSV with entity field definitions for smart column matching
	csvParser := service.NewCSVParser()
	var fields []entity.FieldDef
	if h.metadataRepo != nil {
		fields, err = h.metadataRepo.ListFields(c.Context(), orgID, mirror.TargetEntity)
		if err != nil {
			log.Printf("[MirrorHandler.ImportCSV] Warning: failed to load fields for %s: %v", mirror.TargetEntity, err)
			fields = nil
		}
	}

	var parseResult *service.CSVParseResult
	if len(fields) > 0 {
		parseResult, err = csvParser.Parse(file, fields)
	} else {
		parseResult, err = csvParser.Parse(file, []entity.FieldDef{})
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse CSV: " + err.Error()})
	}
	if parseResult.RowCount == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "CSV file contains no data rows"})
	}

	// Build source field lookup structures
	reverseMap := make(map[string]string)
	sourceFieldNames := make(map[string]bool)
	sourceFieldNamesLower := make(map[string]string)
	for _, sf := range mirror.SourceFields {
		sourceFieldNames[sf.FieldName] = true
		sourceFieldNamesLower[strings.ToLower(sf.FieldName)] = sf.FieldName
		if sf.MapField != nil && *sf.MapField != "" {
			reverseMap[*sf.MapField] = sf.FieldName
		}
	}

	// Transform records: remap keys to mirror source field names
	columnMapping := make(map[string]string)
	unmappedColumnsSet := make(map[string]bool)
	transformedRecords := make([]map[string]interface{}, 0, len(parseResult.Records))

	for _, record := range parseResult.Records {
		transformed := make(map[string]interface{}, len(record))
		for key, value := range record {
			if sourceFieldName, ok := reverseMap[key]; ok {
				transformed[sourceFieldName] = value
				columnMapping[key] = sourceFieldName
			} else if sourceFieldNames[key] {
				transformed[key] = value
				columnMapping[key] = key
			} else if original, ok := sourceFieldNamesLower[strings.ToLower(key)]; ok {
				transformed[original] = value
				columnMapping[key] = original
			} else {
				transformed[key] = value
				unmappedColumnsSet[key] = true
			}
		}
		if len(transformed) > 0 {
			transformedRecords = append(transformedRecords, transformed)
		}
	}

	unmappedColumns := make([]string, 0, len(unmappedColumnsSet))
	for col := range unmappedColumnsSet {
		unmappedColumns = append(unmappedColumns, col)
	}

	if len(transformedRecords) == 0 {
		sourceNames := make([]string, len(mirror.SourceFields))
		for i, sf := range mirror.SourceFields {
			sourceNames[i] = sf.FieldName
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":         "No CSV columns matched any mirror source fields",
			"csv_headers":   parseResult.Headers,
			"source_fields": sourceNames,
		})
	}

	// Create ingest job
	now := time.Now().UTC()
	job := &entity.IngestJob{
		ID:              sfid.NewIngestJob(),
		OrgID:           orgID,
		MirrorID:        mirrorID,
		KeyID:           "csv-import",
		Status:          entity.IngestJobStatusAccepted,
		RecordsReceived: len(transformedRecords),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.jobRepo.Create(c.Context(), tenantDB, job); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create ingest job"})
	}

	// Launch async processing
	go h.ingestService.ProcessJob(context.Background(), tenantDB, orgID, job, transformedRecords)

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id":           job.ID,
		"status":           "accepted",
		"records_received": len(transformedRecords),
		"mirror_id":        mirrorID,
		"column_mapping":   columnMapping,
		"unmapped_columns": unmappedColumns,
		"message":          "CSV ingest request accepted for processing",
	})
}

// GetWatermark returns the last ingest timestamp and record count for a Mirror.
// GET /admin/mirrors/:id/watermark
//
// Returns 200 with {"lastIngestAt": null, "lastIngestCount": 0} if no watermark exists yet
// (i.e., the mirror has never successfully ingested records). Returns the watermark data
// if it has been recorded by a previous ingest run.
func (h *MirrorHandler) GetWatermark(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	mirrorID := c.Params("id")

	if h.sequenceRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "watermark service not configured",
		})
	}

	tenantDB := h.getTenantDBConn(c)
	tenantRepo := h.sequenceRepo.WithDB(tenantDB)

	watermark, err := tenantRepo.GetWatermark(c.Context(), orgID, mirrorID)
	if err != nil {
		log.Printf("[MirrorHandler] GetWatermark failed for org=%s mirror=%s: %v", orgID, mirrorID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load watermark"})
	}

	// Not found: mirror has never ingested — return null defaults (not 404)
	if watermark == nil {
		return c.JSON(fiber.Map{
			"lastIngestAt":    nil,
			"lastIngestCount": 0,
		})
	}

	return c.JSON(watermark)
}

// RegisterRoutes registers all mirror routes
func (h *MirrorHandler) RegisterRoutes(router fiber.Router) {
	mirrors := router.Group("/admin/mirrors")
	mirrors.Post("/", h.Create)
	mirrors.Get("/", h.List)
	mirrors.Get("/:id", h.Get)
	mirrors.Put("/:id", h.Update)
	mirrors.Delete("/:id", h.Delete)
	mirrors.Get("/:id/jobs", h.ListJobs)
	mirrors.Post("/:id/import-csv", h.ImportCSV)
	mirrors.Get("/:id/watermark", h.GetWatermark)
}
