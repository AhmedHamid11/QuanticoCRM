package handler

import (
	"regexp"
	"strings"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// CustomPageHandler handles HTTP requests for custom pages
type CustomPageHandler struct {
	repo *repo.CustomPageRepo
}

// NewCustomPageHandler creates a new CustomPageHandler
func NewCustomPageHandler(repo *repo.CustomPageRepo) *CustomPageHandler {
	return &CustomPageHandler{repo: repo}
}

// getRepo returns the repo with the correct tenant DB from context
func (h *CustomPageHandler) getRepo(c *fiber.Ctx) *repo.CustomPageRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// List returns all custom pages for admin
func (h *CustomPageHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	pages, err := h.getRepo(c).List(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(pages)
}

// ListEnabled returns only enabled pages (for all authenticated users)
func (h *CustomPageHandler) ListEnabled(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	role := c.Locals("role").(string)

	// Admins and owners can see admin-only pages
	includeAdminOnly := role == "admin" || role == "owner"

	pages, err := h.getRepo(c).ListEnabled(c.Context(), orgID, includeAdminOnly)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(pages)
}

// Get returns a single custom page by ID
func (h *CustomPageHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	page, err := h.getRepo(c).GetByID(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if page == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Page not found",
		})
	}

	return c.JSON(page)
}

// GetBySlug returns a custom page by slug (for rendering)
func (h *CustomPageHandler) GetBySlug(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	role := c.Locals("role").(string)
	slug := c.Params("slug")

	page, err := h.getRepo(c).GetBySlug(c.Context(), orgID, slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if page == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Page not found",
		})
	}

	// Check if page is enabled
	if !page.IsEnabled {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Page not found",
		})
	}

	// Check if user has access (non-public pages require admin/owner)
	if !page.IsPublic && role != "admin" && role != "owner" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	return c.JSON(page)
}

// Create creates a new custom page
func (h *CustomPageHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var input entity.CustomPageCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if input.Slug == "" || input.Title == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "slug and title are required",
		})
	}

	// Validate slug format
	if !isValidSlug(input.Slug) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "slug must be lowercase alphanumeric with hyphens only",
		})
	}

	// Check if slug is reserved
	if isReservedSlug(input.Slug) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "this slug is reserved",
		})
	}

	// Check if slug already exists
	exists, err := h.getRepo(c).SlugExists(c.Context(), orgID, input.Slug, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if exists {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "a page with this slug already exists",
		})
	}

	page, err := h.getRepo(c).Create(c.Context(), orgID, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(page)
}

// Update updates an existing custom page
func (h *CustomPageHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	var input entity.CustomPageUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate slug if being updated
	if input.Slug != nil {
		if !isValidSlug(*input.Slug) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "slug must be lowercase alphanumeric with hyphens only",
			})
		}

		if isReservedSlug(*input.Slug) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "this slug is reserved",
			})
		}

		// Check if slug already exists
		exists, err := h.getRepo(c).SlugExists(c.Context(), orgID, *input.Slug, id)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		if exists {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "a page with this slug already exists",
			})
		}
	}

	page, err := h.getRepo(c).Update(c.Context(), orgID, id, input, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if page == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Page not found",
		})
	}

	return c.JSON(page)
}

// Delete deletes a custom page
func (h *CustomPageHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	id := c.Params("id")

	err := h.getRepo(c).Delete(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Page not found",
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// Reorder updates the order of custom pages
func (h *CustomPageHandler) Reorder(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input struct {
		PageIDs []string `json:"pageIds"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(input.PageIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "pageIds is required",
		})
	}

	err := h.getRepo(c).Reorder(c.Context(), orgID, input.PageIDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Return updated list
	pages, err := h.getRepo(c).List(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(pages)
}

// RegisterPublicRoutes registers routes accessible to all authenticated users
func (h *CustomPageHandler) RegisterPublicRoutes(app fiber.Router) {
	app.Get("/pages", h.ListEnabled)
	app.Get("/pages/by-slug/:slug", h.GetBySlug)
}

// RegisterAdminRoutes registers admin-only custom page management routes
func (h *CustomPageHandler) RegisterAdminRoutes(app fiber.Router) {
	admin := app.Group("/admin/pages")
	admin.Get("/", h.List)
	admin.Get("/:id", h.Get)
	admin.Post("/", h.Create)
	admin.Put("/:id", h.Update)
	admin.Delete("/:id", h.Delete)
	admin.Post("/reorder", h.Reorder)
}

// isValidSlug checks if the slug is valid (lowercase alphanumeric with hyphens)
func isValidSlug(slug string) bool {
	// Must be 1-50 characters, lowercase letters, numbers, and hyphens
	matched, _ := regexp.MatchString(`^[a-z0-9][a-z0-9-]{0,48}[a-z0-9]$|^[a-z0-9]$`, slug)
	return matched
}

// isReservedSlug checks if the slug conflicts with existing routes
func isReservedSlug(slug string) bool {
	reserved := []string{
		"admin", "api", "auth", "login", "register", "logout",
		"contacts", "accounts", "tasks", "leads", "opportunities",
		"settings", "profile", "services", "new", "edit",
	}
	lower := strings.ToLower(slug)
	for _, r := range reserved {
		if lower == r {
			return true
		}
	}
	return false
}
