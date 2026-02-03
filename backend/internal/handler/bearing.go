package handler

import (
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// BearingHandler handles HTTP requests for bearing configurations
type BearingHandler struct {
	bearingRepo *repo.BearingRepo
}

// NewBearingHandler creates a new BearingHandler
func NewBearingHandler(bearingRepo *repo.BearingRepo) *BearingHandler {
	return &BearingHandler{
		bearingRepo: bearingRepo,
	}
}

// getRepo returns the bearing repo using the tenant database from context
// Uses GetTenantDBConn for retry-enabled connections
func (h *BearingHandler) getRepo(c *fiber.Ctx) *repo.BearingRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.bearingRepo.WithDB(tenantDB)
	}
	return h.bearingRepo
}

// ListConfigs returns all bearing configs for an entity
// GET /api/v1/entities/:entity/bearing-configs
func (h *BearingHandler) ListConfigs(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	configs, err := h.getRepo(c).ListByEntity(c.Context(), orgID, entityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(configs)
}

// ListActiveWithStages returns active bearing configs with resolved stages for display
// GET /api/v1/entities/:entity/bearings
func (h *BearingHandler) ListActiveWithStages(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	bearings, err := h.getRepo(c).ListActiveWithStages(c.Context(), orgID, entityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(bearings)
}

// GetPicklistFields returns all enum fields for an entity (fields that can be used as bearing source)
// GET /api/v1/entities/:entity/picklist-fields
func (h *BearingHandler) GetPicklistFields(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	fields, err := h.getRepo(c).GetPicklistFields(c.Context(), orgID, entityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fields)
}

// GetConfig returns a single bearing config by ID
// GET /api/v1/entities/:entity/bearing-configs/:id
func (h *BearingHandler) GetConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	configID := c.Params("id")

	config, err := h.getRepo(c).GetByID(c.Context(), orgID, configID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if config == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bearing config not found",
		})
	}

	return c.JSON(config)
}

// CreateConfig creates a new bearing config
// POST /api/v1/entities/:entity/bearing-configs
func (h *BearingHandler) CreateConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	var input entity.BearingConfigCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name is required",
		})
	}
	if input.SourcePicklist == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "sourcePicklist is required",
		})
	}

	config, err := h.getRepo(c).Create(c.Context(), orgID, entityType, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(config)
}

// UpdateConfig updates a bearing config
// PUT /api/v1/entities/:entity/bearing-configs/:id
func (h *BearingHandler) UpdateConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	configID := c.Params("id")

	var input entity.BearingConfigUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	config, err := h.getRepo(c).Update(c.Context(), orgID, configID, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if config == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bearing config not found",
		})
	}

	return c.JSON(config)
}

// DeleteConfig deletes a bearing config
// DELETE /api/v1/entities/:entity/bearing-configs/:id
func (h *BearingHandler) DeleteConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	configID := c.Params("id")

	err := h.getRepo(c).Delete(c.Context(), orgID, configID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bearing config not found",
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// RegisterPublicRoutes registers read-only bearing routes for all authenticated users
// These are needed for displaying stage indicators on detail pages
func (h *BearingHandler) RegisterPublicRoutes(app fiber.Router) {
	entities := app.Group("/entities")
	// Read-only: get active bearings with their stages for display on detail pages
	entities.Get("/:entity/bearings", h.ListActiveWithStages)
}

// RegisterRoutes registers admin bearing routes on the Fiber app
func (h *BearingHandler) RegisterRoutes(app fiber.Router) {
	// Bearing config endpoints (admin/entity manager)
	entities := app.Group("/entities")
	entities.Get("/:entity/bearing-configs", h.ListConfigs)
	entities.Get("/:entity/picklist-fields", h.GetPicklistFields)
	entities.Get("/:entity/bearing-configs/:id", h.GetConfig)
	entities.Post("/:entity/bearing-configs", h.CreateConfig)
	entities.Put("/:entity/bearing-configs/:id", h.UpdateConfig)
	entities.Patch("/:entity/bearing-configs/:id", h.UpdateConfig)
	entities.Delete("/:entity/bearing-configs/:id", h.DeleteConfig)
}
