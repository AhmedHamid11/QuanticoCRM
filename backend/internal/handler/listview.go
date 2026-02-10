package handler

import (
	"encoding/json"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// ListViewHandler handles list view HTTP requests
type ListViewHandler struct {
	repo *repo.ListViewRepo
}

// NewListViewHandler creates a new ListViewHandler
func NewListViewHandler(repo *repo.ListViewRepo) *ListViewHandler {
	return &ListViewHandler{repo: repo}
}

// getRepo returns the repo with the correct tenant DB from context
// Uses GetTenantDBConn for retry-enabled connections
func (h *ListViewHandler) getRepo(c *fiber.Ctx) *repo.ListViewRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// List returns all list views for an entity
func (h *ListViewHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	views, err := h.getRepo(c).List(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list views: " + err.Error(),
		})
	}

	return c.JSON(views)
}

// Get returns a single list view by ID
func (h *ListViewHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	view, err := h.getRepo(c).Get(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "List view not found",
		})
	}

	return c.JSON(view)
}

// GetDefault returns the default list view for an entity
func (h *ListViewHandler) GetDefault(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	view, err := h.getRepo(c).GetDefault(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get default view: " + err.Error(),
		})
	}

	if view == nil {
		return c.JSON(nil)
	}

	return c.JSON(view)
}

// Create creates a new list view
func (h *ListViewHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	entityName := c.Params("entity")

	// Ensure schema is up to date (handles migration from old schema)
	h.getRepo(c).EnsureSchema(c.Context())

	var input entity.ListViewInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	// Serialize columns array to JSON string
	columnsJSON := "[]"
	if len(input.Columns) > 0 {
		if b, err := json.Marshal(input.Columns); err == nil {
			columnsJSON = string(b)
		}
	}

	view := &entity.ListView{
		OrgID:       orgID,
		EntityName:  entityName,
		Name:        input.Name,
		FilterQuery: input.FilterQuery,
		Columns:     columnsJSON,
		SortBy:      input.SortBy,
		SortDir:     input.SortDir,
		IsDefault:   input.IsDefault,
		IsSystem:    false,
		CreatedByID: userID,
	}

	if err := h.getRepo(c).Create(c.Context(), view); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create view: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(view)
}

// Update updates an existing list view
func (h *ListViewHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	// Get existing view
	existing, err := h.getRepo(c).Get(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "List view not found",
		})
	}

	// Don't allow editing system views
	if existing.IsSystem {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Cannot modify system views",
		})
	}

	var input entity.ListViewInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Serialize columns array to JSON string
	columnsJSON := "[]"
	if len(input.Columns) > 0 {
		if b, err := json.Marshal(input.Columns); err == nil {
			columnsJSON = string(b)
		}
	}

	existing.Name = input.Name
	existing.FilterQuery = input.FilterQuery
	existing.Columns = columnsJSON
	existing.SortBy = input.SortBy
	existing.SortDir = input.SortDir
	existing.IsDefault = input.IsDefault

	if err := h.getRepo(c).Update(c.Context(), existing); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update view: " + err.Error(),
		})
	}

	return c.JSON(existing)
}

// Delete deletes a list view
func (h *ListViewHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	// Get existing view to check if it's a system view
	existing, err := h.getRepo(c).Get(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "List view not found",
		})
	}

	// Don't allow deleting system views
	if existing.IsSystem {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Cannot delete system views",
		})
	}

	if err := h.getRepo(c).Delete(c.Context(), orgID, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete view: " + err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
