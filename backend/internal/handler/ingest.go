package handler

import (
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/gofiber/fiber/v2"
)

// IngestHandler handles ingest API endpoints
type IngestHandler struct {
	// No dependencies needed for Phase 20 - mirror validation comes in Phase 21
}

// NewIngestHandler creates a new IngestHandler
func NewIngestHandler() *IngestHandler {
	return &IngestHandler{}
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

	// Phase 20: Mirror validation is a placeholder
	// In Phase 21, we'll validate mirror_id exists and is active
	// For now, just check it's non-empty (already done above)

	// Generate job ID
	jobID := sfid.NewIngestJob()

	// Return 202 Accepted
	response := entity.IngestResponse{
		JobID:           jobID,
		Status:          "accepted",
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
