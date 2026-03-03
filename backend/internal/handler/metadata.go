package handler

import (
	"github.com/fastcrm/backend/internal/cache"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// MetadataHandler provides read-only access to entity metadata
// This is accessible to all authenticated users for rendering layouts
type MetadataHandler struct {
	metadataRepo  *repo.MetadataRepo
	metadataCache *cache.MetadataCache // optional cache for read-only routes
	layoutService *service.LayoutService  // for V3 up-conversion
}

// NewMetadataHandler creates a new MetadataHandler
func NewMetadataHandler(metadataRepo *repo.MetadataRepo) *MetadataHandler {
	return &MetadataHandler{
		metadataRepo:  metadataRepo,
		layoutService: service.NewLayoutService(),
	}
}

// SetMetadataCache injects the shared metadata cache for read-only routes.
func (h *MetadataHandler) SetMetadataCache(mc *cache.MetadataCache) {
	h.metadataCache = mc
}

// getMetadataRepo returns a metadata repo using the tenant database from context
func (h *MetadataHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// getMetadataCache returns the MetadataCache scoped to the tenant DB for this request.
func (h *MetadataHandler) getMetadataCache(c *fiber.Ctx) *cache.MetadataCache {
	if h.metadataCache == nil {
		return cache.NewMetadataCache(h.getMetadataRepo(c), cache.DefaultMetadataTTL)
	}
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataCache.WithDB(tenantDB)
	}
	return h.metadataCache
}

// GetEntityFields returns field definitions for an entity (read-only)
func (h *MetadataHandler) GetEntityFields(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	fields, err := h.getMetadataCache(c).ListFields(c.Context(), orgID, entityName)
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

	layout, err := h.getMetadataCache(c).GetLayout(c.Context(), orgID, entityName, layoutType)
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

// GetEntityLayoutV3 returns a layout in v3 format for rendering (read-only)
// Accessible to all authenticated users, not just admins.
func (h *MetadataHandler) GetEntityLayoutV3(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	layoutType := c.Params("type")

	layout, err := h.getMetadataCache(c).GetLayout(c.Context(), orgID, entityName, layoutType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var layoutJSON string
	if layout == nil {
		layoutJSON = "[]"
	} else {
		layoutJSON = layout.LayoutData
	}

	layoutV3, err := h.layoutService.GetLayoutAsV3(layoutJSON)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"entityName": entityName,
		"layoutType": layoutType,
		"layout":     layoutV3,
		"exists":     layout != nil,
	})
}

// GetEntity returns a single entity definition (read-only)
func (h *MetadataHandler) GetEntity(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	name := c.Params("entity")

	entity, err := h.getMetadataCache(c).GetEntity(c.Context(), orgID, name)
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
	metadata.Get("/entities/:entity/layouts/:type/v3", h.GetEntityLayoutV3)
}
