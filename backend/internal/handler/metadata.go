package handler

import (
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// MetadataHandler provides read-only access to entity metadata
// This is accessible to all authenticated users for rendering layouts
type MetadataHandler struct {
	metadataRepo *repo.MetadataRepo
}

// NewMetadataHandler creates a new MetadataHandler
func NewMetadataHandler(metadataRepo *repo.MetadataRepo) *MetadataHandler {
	return &MetadataHandler{
		metadataRepo: metadataRepo,
	}
}

// getMetadataRepo returns a metadata repo using the tenant database from context
func (h *MetadataHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// GetEntityFields returns field definitions for an entity (read-only)
func (h *MetadataHandler) GetEntityFields(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fields)
}

// GetEntityLayout returns a layout definition for an entity (read-only)
func (h *MetadataHandler) GetEntityLayout(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	layoutType := c.Params("type")

	layout, err := h.getMetadataRepo(c).GetLayout(c.Context(), orgID, entityName, layoutType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if layout == nil {
		// Return empty layout if not found
		return c.JSON(fiber.Map{
			"entityName": entityName,
			"layoutType": layoutType,
			"layoutData": "[]",
			"exists":     false,
		})
	}

	return c.JSON(fiber.Map{
		"id":         layout.ID,
		"entityName": layout.EntityName,
		"layoutType": layout.LayoutType,
		"layoutData": layout.LayoutData,
		"createdAt":  layout.CreatedAt,
		"modifiedAt": layout.ModifiedAt,
		"exists":     true,
	})
}

// GetEntity returns a single entity definition (read-only)
func (h *MetadataHandler) GetEntity(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	name := c.Params("entity")

	entity, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Entity not found",
		})
	}

	return c.JSON(entity)
}

// RegisterRoutes registers read-only metadata routes
func (h *MetadataHandler) RegisterRoutes(app fiber.Router) {
	metadata := app.Group("/metadata")

	// Read-only entity metadata
	metadata.Get("/entities/:entity", h.GetEntity)
	metadata.Get("/entities/:entity/fields", h.GetEntityFields)
	metadata.Get("/entities/:entity/layouts/:type", h.GetEntityLayout)
}
