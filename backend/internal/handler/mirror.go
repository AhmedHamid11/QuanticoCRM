package handler

import (
	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// MirrorHandler handles mirror management endpoints
type MirrorHandler struct {
	repo    *repo.MirrorRepo
	jobRepo *repo.IngestJobRepo
}

// NewMirrorHandler creates a new MirrorHandler
func NewMirrorHandler(repo *repo.MirrorRepo, jobRepo *repo.IngestJobRepo) *MirrorHandler {
	return &MirrorHandler{repo: repo, jobRepo: jobRepo}
}

// getTenantDBConn extracts the tenant DB connection from context
func (h *MirrorHandler) getTenantDBConn(c *fiber.Ctx) db.DBConn {
	return middleware.GetTenantDBConn(c)
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

	// Create mirror
	mirror, err := h.repo.Create(c.Context(), tenantDB, orgID, input)
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

	// Update mirror
	mirror, err := h.repo.Update(c.Context(), tenantDB, orgID, mirrorID, input)
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
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

// RegisterRoutes registers all mirror routes
func (h *MirrorHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/mirrors", h.Create)
	router.Get("/mirrors", h.List)
	router.Get("/mirrors/:id", h.Get)
	router.Put("/mirrors/:id", h.Update)
	router.Delete("/mirrors/:id", h.Delete)
	router.Get("/mirrors/:id/jobs", h.ListJobs)
}
