package handler

import (
	"log"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// OrgFeaturesHandler handles platform-admin feature provisioning endpoints.
// These operate on a specific org's TENANT database, not the master DB.
type OrgFeaturesHandler struct {
	repo      *repo.OrgFeaturesRepo
	dbManager *db.Manager
	authRepo  *repo.AuthRepo
}

// NewOrgFeaturesHandler creates a new OrgFeaturesHandler
func NewOrgFeaturesHandler(repo *repo.OrgFeaturesRepo, dbManager *db.Manager, authRepo *repo.AuthRepo) *OrgFeaturesHandler {
	return &OrgFeaturesHandler{
		repo:      repo,
		dbManager: dbManager,
		authRepo:  authRepo,
	}
}

// resolveTenantDB resolves the tenant DB for a given orgID.
// In local mode, returns the master DB (shared). In production, looks up org and connects.
func (h *OrgFeaturesHandler) resolveTenantDB(c *fiber.Ctx, orgID string) (db.DBConn, error) {
	if h.dbManager.IsLocalMode() {
		return h.dbManager.GetMasterDB(), nil
	}

	org, err := h.authRepo.GetOrganizationByID(c.Context(), orgID)
	if err != nil {
		return nil, err
	}

	if org.DatabaseURL == "" {
		return h.dbManager.GetMasterDB(), nil
	}

	return h.dbManager.GetTenantDBConn(c.Context(), orgID, org.DatabaseURL, org.DatabaseToken)
}

// ListFeatures returns all known features with enabled status for an org
// GET /platform/orgs/:orgId/features
func (h *OrgFeaturesHandler) ListFeatures(c *fiber.Ctx) error {
	orgID := c.Params("orgId")
	if orgID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Organization ID is required",
		})
	}

	// Verify org exists
	if _, err := h.authRepo.GetOrganizationByID(c.Context(), orgID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Organization not found",
		})
	}

	tenantDB, err := h.resolveTenantDB(c, orgID)
	if err != nil {
		log.Printf("[OrgFeatures] Failed to resolve tenant DB for org %s: %v", orgID, err)
		return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ErrCategoryDatabase)
	}

	tenantRepo := h.repo.WithDB(tenantDB)
	features, err := tenantRepo.ListFeatures(c.Context(), orgID)
	if err != nil {
		log.Printf("[OrgFeatures] Failed to list features for org %s: %v", orgID, err)
		return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ClassifyError(err))
	}

	return c.JSON(fiber.Map{
		"orgId":    orgID,
		"features": features,
	})
}

// SetFeature toggles a feature for an org
// PUT /platform/orgs/:orgId/features/:featureKey
func (h *OrgFeaturesHandler) SetFeature(c *fiber.Ctx) error {
	orgID := c.Params("orgId")
	featureKey := c.Params("featureKey")

	if orgID == "" || featureKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Organization ID and feature key are required",
		})
	}

	// Validate feature key is known
	if !isKnownFeature(featureKey) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Unknown feature key",
			"code":  "UNKNOWN_FEATURE",
			"key":   featureKey,
		})
	}

	// Verify org exists
	if _, err := h.authRepo.GetOrganizationByID(c.Context(), orgID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Organization not found",
		})
	}

	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	tenantDB, err := h.resolveTenantDB(c, orgID)
	if err != nil {
		log.Printf("[OrgFeatures] Failed to resolve tenant DB for org %s: %v", orgID, err)
		return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ErrCategoryDatabase)
	}

	enabledBy := ""
	if email, ok := c.Locals("email").(string); ok {
		enabledBy = email
	}

	tenantRepo := h.repo.WithDB(tenantDB)
	if err := tenantRepo.SetFeature(c.Context(), featureKey, input.Enabled, enabledBy); err != nil {
		log.Printf("[OrgFeatures] Failed to set feature %s for org %s: %v", featureKey, orgID, err)
		return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ClassifyError(err))
	}

	// Return updated feature list
	features, err := tenantRepo.ListFeatures(c.Context(), orgID)
	if err != nil {
		log.Printf("[OrgFeatures] Failed to list features after update for org %s: %v", orgID, err)
		return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ClassifyError(err))
	}

	return c.JSON(fiber.Map{
		"orgId":    orgID,
		"features": features,
	})
}

// RegisterPlatformRoutes registers platform admin feature routes
func (h *OrgFeaturesHandler) RegisterPlatformRoutes(app fiber.Router) {
	app.Get("/orgs/:orgId/features", h.ListFeatures)
	app.Put("/orgs/:orgId/features/:featureKey", h.SetFeature)
}

// isKnownFeature checks if a feature key is in the KnownFeatures registry
func isKnownFeature(key string) bool {
	for _, f := range entity.KnownFeatures {
		if f.Key == key {
			return true
		}
	}
	return false
}
