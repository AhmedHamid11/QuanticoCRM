package handler

import (
	"log"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// NavigationHandler handles HTTP requests for navigation configuration
type NavigationHandler struct {
	repo *repo.NavigationRepo
}

// NewNavigationHandler creates a new NavigationHandler
func NewNavigationHandler(repo *repo.NavigationRepo) *NavigationHandler {
	return &NavigationHandler{repo: repo}
}

// getRepo returns the repo with the correct tenant DB from context
func (h *NavigationHandler) getRepo(c *fiber.Ctx) *repo.NavigationRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// List returns all navigation tabs (for admin)
func (h *NavigationHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	tabs, err := h.getRepo(c).List(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(tabs)
}

// ListVisible returns only visible navigation tabs (for frontend)
func (h *NavigationHandler) ListVisible(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	tabs, err := h.getRepo(c).ListVisible(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if len(tabs) == 0 {
		log.Printf("[Navigation] WARNING: No visible navigation tabs for org %s - provisioning may have failed. Use POST /admin/reprovision to fix.", orgID)
	}

	return c.JSON(tabs)
}

// Get returns a single navigation tab by ID
func (h *NavigationHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	tab, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if tab == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Navigation tab not found",
		})
	}

	return c.JSON(tab)
}

// Create creates a new navigation tab
func (h *NavigationHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input entity.NavigationTabCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Label == "" || input.Href == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "label and href are required",
		})
	}

	tab, err := h.getRepo(c).Create(c.Context(), orgID, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(tab)
}

// Update updates an existing navigation tab
func (h *NavigationHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	var input entity.NavigationTabUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	tab, err := h.getRepo(c).Update(c.Context(), orgID, id, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if tab == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Navigation tab not found",
		})
	}

	return c.JSON(tab)
}

// Delete deletes a navigation tab
func (h *NavigationHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	err := h.getRepo(c).Delete(c.Context(), orgID, id)
	if err != nil {
		if err.Error() == "cannot delete system navigation tab" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Navigation tab not found",
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// Reorder updates the order of navigation tabs
func (h *NavigationHandler) Reorder(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input entity.NavigationReorderInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(input.TabIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "tabIds is required",
		})
	}

	err := h.getRepo(c).Reorder(c.Context(), orgID, input.TabIDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Return updated list
	tabs, err := h.getRepo(c).List(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(tabs)
}

// RegisterPublicRoutes registers navigation routes accessible to all authenticated users
func (h *NavigationHandler) RegisterPublicRoutes(app fiber.Router) {
	app.Get("/navigation", h.ListVisible)
}

// RegisterAdminRoutes registers admin-only navigation management routes
func (h *NavigationHandler) RegisterAdminRoutes(app fiber.Router) {
	admin := app.Group("/admin/navigation")
	admin.Get("/", h.List)
	admin.Get("/:id", h.Get)
	admin.Post("/", h.Create)
	admin.Put("/:id", h.Update)
	admin.Delete("/:id", h.Delete)
	admin.Post("/reorder", h.Reorder)
}
