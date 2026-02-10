package handler

import (
	"context"
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
}

// NewIngestHandler creates a new IngestHandler
func NewIngestHandler(ingestService *service.IngestService, mirrorRepo *repo.MirrorRepo, jobRepo *repo.IngestJobRepo) *IngestHandler {
	return &IngestHandler{
		ingestService: ingestService,
		mirrorRepo:    mirrorRepo,
		jobRepo:       jobRepo,
	}
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

// RegisterRoutes registers ingest routes
func (h *IngestHandler) RegisterRoutes(router fiber.Router) {
	router.Post("", h.Ingest)
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
