package handler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/gofiber/fiber/v2"
)

// IngestHandler handles ingest API endpoints
type IngestHandler struct {
	ingestService *service.IngestService
	mirrorRepo    *repo.MirrorRepo
	jobRepo       *repo.IngestJobRepo
	deltaKeyRepo  *repo.DeltaKeyRepo
	rateLimiter   *service.IngestRateLimiter
	metadataRepo  *repo.MetadataRepo
}

// NewIngestHandler creates a new IngestHandler
func NewIngestHandler(
	ingestService *service.IngestService,
	mirrorRepo *repo.MirrorRepo,
	jobRepo *repo.IngestJobRepo,
	deltaKeyRepo *repo.DeltaKeyRepo,
	rateLimiter *service.IngestRateLimiter,
) *IngestHandler {
	return &IngestHandler{
		ingestService: ingestService,
		mirrorRepo:    mirrorRepo,
		jobRepo:       jobRepo,
		deltaKeyRepo:  deltaKeyRepo,
		rateLimiter:   rateLimiter,
	}
}

// SetMetadataRepo sets the metadata repo for loading entity field definitions during CSV import
func (h *IngestHandler) SetMetadataRepo(r *repo.MetadataRepo) {
	h.metadataRepo = r
}

// Ingest handles POST /api/ingest
// Accepts batched JSON records from external systems
func (h *IngestHandler) Ingest(c *fiber.Ctx) error {
	// Get org ID from context (set by ingest auth middleware)
	ingestOrgID, ok := c.Locals("ingestOrgID").(string)
	if !ok || ingestOrgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing organization context",
		})
	}

	// Get ingest key ID from context
	ingestKeyID, _ := c.Locals("ingestKeyID").(string)

	// Parse request body
	var req entity.IngestRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON payload",
		})
	}

	// Validate required fields
	if req.OrgID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "org_id is required",
		})
	}

	if req.MirrorID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mirror_id is required",
		})
	}

	if len(req.Records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "records array is required and must not be empty",
		})
	}

	// Belt-and-suspenders org_id check
	// The API key is tied to a specific org - payload org_id must match
	if req.OrgID != ingestOrgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":    "org_id does not match API key tenant",
			"expected": "<masked>",
			"received": req.OrgID,
		})
	}

	// Get tenant DB connection from context
	tenantDB := middleware.GetTenantDBConn(c)
	if tenantDB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database connection error",
		})
	}

	// Validate mirror exists and is active (synchronous)
	mirror, err := h.mirrorRepo.GetActiveByID(c.Context(), tenantDB, ingestOrgID, req.MirrorID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to validate mirror",
		})
	}
	if mirror == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":     "Mirror not found or inactive",
			"mirror_id": req.MirrorID,
		})
	}

	// Validate mirror has a target entity
	if mirror.TargetEntity == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":     "Mirror has no target entity configured",
			"mirror_id": req.MirrorID,
		})
	}

	// Check per-mirror rate limit
	allowed, retryAfter := h.rateLimiter.Allow(mirror.ID, mirror.RateLimit)
	if !allowed {
		c.Set("Retry-After", strconv.Itoa(retryAfter))
		return c.Status(429).JSON(fiber.Map{
			"error":       "Rate limit exceeded for mirror",
			"mirror_id":   req.MirrorID,
			"retry_after": retryAfter,
			"limit":       mirror.RateLimit,
			"message":     fmt.Sprintf("Mirror rate limit of %d requests/minute exceeded. Retry after %d seconds.", mirror.RateLimit, retryAfter),
		})
	}

	// Create ingest job (synchronous)
	now := time.Now().UTC()
	job := &entity.IngestJob{
		ID:              sfid.NewIngestJob(),
		OrgID:           ingestOrgID,
		MirrorID:        req.MirrorID,
		KeyID:           ingestKeyID,
		Status:          entity.IngestJobStatusAccepted,
		RecordsReceived: len(req.Records),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.jobRepo.Create(c.Context(), tenantDB, job); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create ingest job",
		})
	}

	// Launch async processing using context.Background() (request context will be canceled when 202 is sent)
	go h.ingestService.ProcessJob(context.Background(), tenantDB, ingestOrgID, job, req.Records)

	// Return 202 Accepted with real job ID
	response := entity.IngestResponse{
		JobID:           job.ID,
		Status:          entity.IngestJobStatusAccepted,
		RecordsReceived: len(req.Records),
		MirrorID:        req.MirrorID,
		Message:         "Ingest request accepted for processing",
	}

	return c.Status(fiber.StatusAccepted).JSON(response)
}

// GetJobStatus handles GET /api/v1/ingest/jobs/:id
func (h *IngestHandler) GetJobStatus(c *fiber.Ctx) error {
	// Get org ID from context (set by ingest auth middleware)
	ingestOrgID, ok := c.Locals("ingestOrgID").(string)
	if !ok || ingestOrgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing organization context",
		})
	}

	// Get job ID from URL params
	jobID := c.Params("id")
	if jobID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Job ID is required",
		})
	}

	// Get tenant DB connection from context
	tenantDB := middleware.GetTenantDBConn(c)
	if tenantDB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database connection error",
		})
	}

	// Get job by ID
	job, err := h.jobRepo.GetByID(c.Context(), tenantDB, ingestOrgID, jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve job",
		})
	}

	if job == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":  "Job not found",
			"job_id": jobID,
		})
	}

	return c.JSON(job)
}

// ListDeltaKeys handles GET /api/v1/ingest/mirrors/:mirror_id/keys
func (h *IngestHandler) ListDeltaKeys(c *fiber.Ctx) error {
	// Get org ID from context (set by ingest auth middleware)
	ingestOrgID, ok := c.Locals("ingestOrgID").(string)
	if !ok || ingestOrgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing organization context",
		})
	}

	// Get mirror ID from URL params
	mirrorID := c.Params("mirror_id")
	if mirrorID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Mirror ID is required",
		})
	}

	// Get tenant DB connection from context
	tenantDB := middleware.GetTenantDBConn(c)
	if tenantDB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database connection error",
		})
	}

	// Validate mirror exists and belongs to this org
	mirror, err := h.mirrorRepo.GetByID(c.Context(), tenantDB, ingestOrgID, mirrorID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to validate mirror",
		})
	}
	if mirror == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":     "Mirror not found",
			"mirror_id": mirrorID,
		})
	}

	// Parse query params
	cursor := c.Query("cursor", "")
	limit := c.QueryInt("limit", 100)

	// List delta keys with pagination
	page, err := h.deltaKeyRepo.ListByMirror(c.Context(), tenantDB, mirrorID, cursor, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve delta keys",
		})
	}

	return c.JSON(page)
}

// IngestCSV handles POST /api/v1/ingest/mirrors/:mirror_id/csv
// Accepts a CSV file via multipart/form-data and feeds it through the ingest pipeline
func (h *IngestHandler) IngestCSV(c *fiber.Ctx) error {
	// Get org ID from context (set by ingest auth middleware via X-API-Key)
	ingestOrgID, ok := c.Locals("ingestOrgID").(string)
	if !ok || ingestOrgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing organization context",
		})
	}
	ingestKeyID, _ := c.Locals("ingestKeyID").(string)

	mirrorID := c.Params("mirror_id")
	if mirrorID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Mirror ID is required",
		})
	}

	// Get tenant DB connection from context
	tenantDB := middleware.GetTenantDBConn(c)
	if tenantDB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database connection error",
		})
	}

	// 1. Load mirror config with source fields
	mirror, err := h.mirrorRepo.GetActiveByID(c.Context(), tenantDB, ingestOrgID, mirrorID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to validate mirror",
		})
	}
	if mirror == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":     "Mirror not found or inactive",
			"mirror_id": mirrorID,
		})
	}
	if mirror.TargetEntity == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":     "Mirror has no target entity configured",
			"mirror_id": mirrorID,
		})
	}

	// Check per-mirror rate limit
	allowed, retryAfter := h.rateLimiter.Allow(mirror.ID, mirror.RateLimit)
	if !allowed {
		c.Set("Retry-After", strconv.Itoa(retryAfter))
		return c.Status(429).JSON(fiber.Map{
			"error":       "Rate limit exceeded for mirror",
			"mirror_id":   mirrorID,
			"retry_after": retryAfter,
			"limit":       mirror.RateLimit,
			"message":     fmt.Sprintf("Mirror rate limit of %d requests/minute exceeded. Retry after %d seconds.", mirror.RateLimit, retryAfter),
		})
	}

	// 2. Parse multipart form and extract CSV file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing or invalid 'file' field in form data",
		})
	}

	// Check file size (50MB max)
	if fileHeader.Size > 50*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File exceeds 50MB limit",
		})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read uploaded file",
		})
	}
	defer file.Close()

	// 3. Parse CSV - try to use entity field definitions for smarter column matching
	csvParser := service.NewCSVParser()
	var fields []entity.FieldDef
	if h.metadataRepo != nil {
		fields, err = h.metadataRepo.ListFields(c.Context(), ingestOrgID, mirror.TargetEntity)
		if err != nil {
			log.Printf("[IngestHandler.IngestCSV] Warning: failed to load fields for %s: %v (using raw parse)", mirror.TargetEntity, err)
			fields = nil
		}
	}

	var parseResult *service.CSVParseResult
	if len(fields) > 0 {
		parseResult, err = csvParser.Parse(file, fields)
	} else {
		// No field definitions - parse with empty field list (raw string values keyed by header name)
		parseResult, err = csvParser.Parse(file, []entity.FieldDef{})
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse CSV: " + err.Error(),
		})
	}

	if parseResult.RowCount == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "CSV file contains no data rows",
		})
	}

	// 4. Build source field lookup structures
	// reverseMap: entityFieldName -> sourceFieldName (for records that went through CSVParser field mapping)
	// sourceFieldNames: set of all source field names (for direct matching)
	reverseMap := make(map[string]string)
	sourceFieldNames := make(map[string]bool)
	sourceFieldNamesLower := make(map[string]string) // lowercase -> original
	for _, sf := range mirror.SourceFields {
		sourceFieldNames[sf.FieldName] = true
		sourceFieldNamesLower[strings.ToLower(sf.FieldName)] = sf.FieldName
		if sf.MapField != nil && *sf.MapField != "" {
			reverseMap[*sf.MapField] = sf.FieldName
		}
	}

	// 5. Transform records: remap keys to mirror source field names
	// The ingest pipeline expects records keyed by source field names
	columnMapping := make(map[string]string) // csvHeader -> sourceFieldName (for response)
	unmappedColumnsSet := make(map[string]bool)
	transformedRecords := make([]map[string]interface{}, 0, len(parseResult.Records))

	for _, record := range parseResult.Records {
		transformed := make(map[string]interface{}, len(record))
		for key, value := range record {
			if sourceFieldName, ok := reverseMap[key]; ok {
				// Entity field name -> source field name via reverse map
				transformed[sourceFieldName] = value
				columnMapping[key] = sourceFieldName
			} else if sourceFieldNames[key] {
				// Direct match: key already matches a source field name exactly
				transformed[key] = value
				columnMapping[key] = key
			} else if original, ok := sourceFieldNamesLower[strings.ToLower(key)]; ok {
				// Case-insensitive match to source field name
				transformed[original] = value
				columnMapping[key] = original
			} else {
				// No mapping found - pass through (ingest pipeline handles validation)
				transformed[key] = value
				unmappedColumnsSet[key] = true
			}
		}
		if len(transformed) > 0 {
			transformedRecords = append(transformedRecords, transformed)
		}
	}

	// Also check raw CSV headers for unmapped columns (headers that the CSVParser couldn't map to entity fields)
	for i, header := range parseResult.Headers {
		mappedHeader := ""
		if i < len(parseResult.MappedHeaders) {
			mappedHeader = parseResult.MappedHeaders[i]
		}
		if mappedHeader == "" {
			// CSVParser couldn't map this header to an entity field
			// Check if it matches a source field name directly
			if !sourceFieldNames[header] {
				if _, ok := sourceFieldNamesLower[strings.ToLower(header)]; !ok {
					unmappedColumnsSet[header] = true
				}
			}
		}
	}

	unmappedColumns := make([]string, 0, len(unmappedColumnsSet))
	for col := range unmappedColumnsSet {
		unmappedColumns = append(unmappedColumns, col)
	}

	if len(transformedRecords) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":            "No CSV columns matched any mirror source fields",
			"csv_headers":      parseResult.Headers,
			"source_fields":    getSourceFieldNames(mirror),
			"unmapped_columns": unmappedColumns,
		})
	}

	// 6. Create ingest job
	now := time.Now().UTC()
	job := &entity.IngestJob{
		ID:              sfid.NewIngestJob(),
		OrgID:           ingestOrgID,
		MirrorID:        mirrorID,
		KeyID:           ingestKeyID,
		Status:          entity.IngestJobStatusAccepted,
		RecordsReceived: len(transformedRecords),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.jobRepo.Create(c.Context(), tenantDB, job); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create ingest job",
		})
	}

	// 7. Launch async processing (same pattern as JSON ingest)
	go h.ingestService.ProcessJob(context.Background(), tenantDB, ingestOrgID, job, transformedRecords)

	// 8. Return 202 Accepted with mapping info
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id":            job.ID,
		"status":            "accepted",
		"records_received":  len(transformedRecords),
		"mirror_id":         mirrorID,
		"column_mapping":    columnMapping,
		"unmapped_columns":  unmappedColumns,
		"message":           "CSV ingest request accepted for processing",
	})
}

// getSourceFieldNames extracts source field names from a mirror for error reporting
func getSourceFieldNames(mirror *entity.Mirror) []string {
	names := make([]string, len(mirror.SourceFields))
	for i, sf := range mirror.SourceFields {
		names[i] = sf.FieldName
	}
	return names
}

// RegisterRoutes registers ingest routes
func (h *IngestHandler) RegisterRoutes(router fiber.Router) {
	router.Post("", h.Ingest)
	router.Get("/jobs/:id", h.GetJobStatus)
	router.Get("/mirrors/:mirror_id/keys", h.ListDeltaKeys)
	router.Post("/mirrors/:mirror_id/csv", h.IngestCSV)
}

// IngestAPIKeyHandler handles ingest API key management endpoints (admin only)
type IngestAPIKeyHandler struct {
	service *service.IngestAPIKeyService
}

// NewIngestAPIKeyHandler creates a new IngestAPIKeyHandler
func NewIngestAPIKeyHandler(service *service.IngestAPIKeyService) *IngestAPIKeyHandler {
	return &IngestAPIKeyHandler{service: service}
}

// Create handles POST /api/v1/ingest-keys
func (h *IngestAPIKeyHandler) Create(c *fiber.Ctx) error {
	orgID, ok := c.Locals("orgID").(string)
	if !ok || orgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing organization context",
		})
	}

	userID, ok := c.Locals("userID").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing user context",
		})
	}

	var input entity.IngestAPIKeyCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	response, err := h.service.Create(c.Context(), orgID, userID, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// List handles GET /api/v1/ingest-keys
func (h *IngestAPIKeyHandler) List(c *fiber.Ctx) error {
	orgID, ok := c.Locals("orgID").(string)
	if !ok || orgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing organization context",
		})
	}

	keys, err := h.service.List(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(keys)
}

// Deactivate handles POST /api/v1/ingest-keys/:id/deactivate
func (h *IngestAPIKeyHandler) Deactivate(c *fiber.Ctx) error {
	orgID, ok := c.Locals("orgID").(string)
	if !ok || orgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing organization context",
		})
	}

	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Key ID is required",
		})
	}

	if err := h.service.Deactivate(c.Context(), id, orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Key deactivated successfully",
	})
}

// Delete handles DELETE /api/v1/ingest-keys/:id
func (h *IngestAPIKeyHandler) Delete(c *fiber.Ctx) error {
	orgID, ok := c.Locals("orgID").(string)
	if !ok || orgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing organization context",
		})
	}

	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Key ID is required",
		})
	}

	if err := h.service.Delete(c.Context(), id, orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Key deleted successfully",
	})
}
