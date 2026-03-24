package handler

import (
	"log"
	"strings"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// OrgSettingsHandler handles HTTP requests for org settings
type OrgSettingsHandler struct {
	repo         *repo.OrgSettingsRepo
	featuresRepo *repo.OrgFeaturesRepo
	auditLogger  *service.AuditLogger
}

// NewOrgSettingsHandler creates a new OrgSettingsHandler
func NewOrgSettingsHandler(repo *repo.OrgSettingsRepo, featuresRepo *repo.OrgFeaturesRepo, auditLogger *service.AuditLogger) *OrgSettingsHandler {
	return &OrgSettingsHandler{
		repo:         repo,
		featuresRepo: featuresRepo,
		auditLogger:  auditLogger,
	}
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
	tenantRepo := h.getRepo(c)

	settings, err := tenantRepo.Get(c.Context(), orgID)
	if err != nil {
		// Auto-create table if missing (for existing orgs before this migration)
		if strings.Contains(err.Error(), "no such table") {
			if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
				// Schema must match migration 044_create_org_settings.sql
				_, createErr := tenantDB.ExecContext(c.Context(), `
					CREATE TABLE IF NOT EXISTS org_settings (
						org_id TEXT PRIMARY KEY,
						home_page TEXT DEFAULT '/',
						settings_json TEXT DEFAULT '{}',
						created_at TEXT NOT NULL DEFAULT (datetime('now')),
						modified_at TEXT NOT NULL DEFAULT (datetime('now'))
					)
				`)
				// Add timeout columns that were added later
				if createErr == nil {
					tenantDB.ExecContext(c.Context(), "ALTER TABLE org_settings ADD COLUMN idle_timeout_minutes INTEGER DEFAULT 30")
					tenantDB.ExecContext(c.Context(), "ALTER TABLE org_settings ADD COLUMN absolute_timeout_minutes INTEGER DEFAULT 1440")
					tenantDB.ExecContext(c.Context(), "ALTER TABLE org_settings ADD COLUMN accent_color TEXT DEFAULT '#1e40af'")
				}
				if createErr == nil {
					// Retry the get
					settings, err = tenantRepo.Get(c.Context(), orgID)
				}
			}
		}
		if err != nil {
			return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ClassifyError(err))
		}
	}

	// Build combined response with features
	response := fiber.Map{
		"orgId":                  settings.OrgID,
		"homePage":               settings.HomePage,
		"idleTimeoutMinutes":     settings.IdleTimeoutMinutes,
		"absoluteTimeoutMinutes": settings.AbsoluteTimeoutMinutes,
		"accentColor":            settings.AccentColor,
		"settingsJson":           settings.SettingsJSON,
	}

	// Include feature flags if repo is available
	if h.featuresRepo != nil {
		featuresRepo := h.featuresRepo
		if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
			featuresRepo = h.featuresRepo.WithDB(tenantDB)
		}
		featuresMap, err := featuresRepo.GetEnabledFeatures(c.Context())
		if err != nil {
			// Non-fatal: log and continue without features
			log.Printf("[OrgSettings] Failed to load features for org %s: %v", orgID, err)
		} else {
			response["features"] = featuresMap
		}
	}

	return c.JSON(response)
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
		return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ClassifyError(err))
	}

	// Audit log the settings change
	go h.auditLogger.LogOrgSettingsChange(
		c.Context(),
		c.Locals("userID").(string),
		c.Locals("email").(string),
		orgID,
		[]string{"homePage"},
	)

	return c.JSON(settings)
}

// Update updates organization settings (including session timeouts)
// PUT /admin/settings
func (h *OrgSettingsHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input entity.OrgSettingsUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate session timeouts if provided
	if input.IdleTimeoutMinutes != nil || input.AbsoluteTimeoutMinutes != nil {
		// Get current settings to determine which value to validate
		current, err := h.getRepo(c).Get(c.Context(), orgID)
		if err != nil {
			return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ClassifyError(err))
		}

		idle := current.IdleTimeoutMinutes
		absolute := current.AbsoluteTimeoutMinutes

		if input.IdleTimeoutMinutes != nil {
			idle = *input.IdleTimeoutMinutes
		}
		if input.AbsoluteTimeoutMinutes != nil {
			absolute = *input.AbsoluteTimeoutMinutes
		}

		if err := repo.ValidateSessionTimeouts(idle, absolute); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
				"code":  "INVALID_TIMEOUT_SETTINGS",
			})
		}
	}

	settings, err := h.getRepo(c).Update(c.Context(), orgID, &input)
	if err != nil {
		return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ClassifyError(err))
	}

	// Determine which fields changed for audit logging
	changedFields := []string{}
	if input.IdleTimeoutMinutes != nil {
		changedFields = append(changedFields, "idleTimeoutMinutes")
	}
	if input.AbsoluteTimeoutMinutes != nil {
		changedFields = append(changedFields, "absoluteTimeoutMinutes")
	}
	if input.HomePage != nil {
		changedFields = append(changedFields, "homePage")
	}
	if input.AccentColor != nil {
		changedFields = append(changedFields, "accentColor")
	}

	// Audit log the settings change
	if len(changedFields) > 0 {
		go h.auditLogger.LogOrgSettingsChange(
			c.Context(),
			c.Locals("userID").(string),
			c.Locals("email").(string),
			orgID,
			changedFields,
		)
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
	admin.Put("", h.Update) // General settings update including session timeouts
}
