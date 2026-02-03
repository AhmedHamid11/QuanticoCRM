package handler

import (
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// OrgSettingsHandler handles HTTP requests for org settings
type OrgSettingsHandler struct {
	repo *repo.OrgSettingsRepo
}

// NewOrgSettingsHandler creates a new OrgSettingsHandler
func NewOrgSettingsHandler(repo *repo.OrgSettingsRepo) *OrgSettingsHandler {
	return &OrgSettingsHandler{repo: repo}
}

// getRepo returns the repo with the correct tenant DB from context
// Uses GetTenantDBConn for retry-enabled connections
func (h *OrgSettingsHandler) getRepo(c *fiber.Ctx) *repo.OrgSettingsRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.repo.WithDB(tenantDB)
	}
	return h.repo
}

// Get returns org settings
// GET /settings
func (h *OrgSettingsHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	settings, err := h.getRepo(c).Get(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(settings)
}

// UpdateHomePage updates the homepage setting
// PUT /admin/settings/homepage
func (h *OrgSettingsHandler) UpdateHomePage(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input entity.OrgSettingsUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.HomePage == nil || *input.HomePage == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "homePage is required",
		})
	}

	settings, err := h.getRepo(c).UpdateHomePage(c.Context(), orgID, *input.HomePage)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(settings)
}

// RegisterPublicRoutes registers settings routes accessible to all authenticated users
func (h *OrgSettingsHandler) RegisterPublicRoutes(app fiber.Router) {
	app.Get("/settings", h.Get)
}

// RegisterAdminRoutes registers admin-only settings management routes
func (h *OrgSettingsHandler) RegisterAdminRoutes(app fiber.Router) {
	admin := app.Group("/admin/settings")
	admin.Put("/homepage", h.UpdateHomePage)
}
