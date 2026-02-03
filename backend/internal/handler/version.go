package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/fastcrm/backend/internal/changelog"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
)

// VersionHandler handles version-related API requests
type VersionHandler struct {
	repo                *repo.VersionRepo
	service             *service.VersionService
	migrationRepo       *repo.MigrationRepo
	migrationPropagator *service.MigrationPropagator
	authRepo            *repo.AuthRepo
}

// NewVersionHandler creates a new VersionHandler
func NewVersionHandler(
	versionRepo *repo.VersionRepo,
	versionService *service.VersionService,
	migrationRepo *repo.MigrationRepo,
	authRepo *repo.AuthRepo,
) *VersionHandler {
	return &VersionHandler{
		repo:          versionRepo,
		service:       versionService,
		migrationRepo: migrationRepo,
		authRepo:      authRepo,
	}
}

// SetMigrationPropagator sets the migration propagator (called after handler construction)
func (h *VersionHandler) SetMigrationPropagator(mp *service.MigrationPropagator) {
	h.migrationPropagator = mp
}

// RegisterRoutes registers version routes (accessible to all authenticated users)
func (h *VersionHandler) RegisterRoutes(router fiber.Router) {
	version := router.Group("/version")
	version.Get("/platform", h.GetPlatformVersion)
	version.Get("/current", h.GetCurrentVersion)
	version.Get("/history", h.GetVersionHistory)
	version.Get("/changelog", h.GetChangelog)
	version.Get("/changelog/since", h.GetChangelogSince)
	version.Get("/migration-status", h.GetMigrationStatus)
}

// RegisterAdminRoutes registers admin-only version routes
func (h *VersionHandler) RegisterAdminRoutes(router fiber.Router) {
	version := router.Group("/version")
	version.Post("/migration-retry/:orgId", h.RetryMigration)
	version.Post("/migration-retry-all", h.RetryAllFailed)
}

// GetPlatformVersion returns the current platform version
// GET /api/v1/version/platform
func (h *VersionHandler) GetPlatformVersion(c *fiber.Ctx) error {
	pv, err := h.repo.GetPlatformVersion(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get platform version: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"version":     pv.Version,
		"description": pv.Description,
		"releasedAt":  pv.ReleasedAt,
	})
}

// GetCurrentVersion returns the org's current version and update status
// GET /api/v1/version/current
func (h *VersionHandler) GetCurrentVersion(c *fiber.Ctx) error {
	orgID := c.Locals("orgID")
	if orgID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization not found in context",
		})
	}

	// Get platform version
	pv, err := h.repo.GetPlatformVersion(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get platform version: " + err.Error(),
		})
	}

	// Get org version
	orgVersion, err := h.repo.GetOrgVersion(c.Context(), orgID.(string))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get org version: " + err.Error(),
		})
	}

	// Check if update is needed
	needsUpdate := h.service.NeedsUpdate(orgVersion, pv.Version)

	return c.JSON(fiber.Map{
		"orgVersion":      orgVersion,
		"platformVersion": pv.Version,
		"needsUpdate":     needsUpdate,
		"releasedAt":      pv.ReleasedAt,
	})
}

// GetVersionHistory returns version history
// GET /api/v1/version/history?limit=10
func (h *VersionHandler) GetVersionHistory(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)

	versions, err := h.repo.GetVersionHistory(c.Context(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get version history: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"versions": versions,
	})
}

// GetChangelog returns changelog entries for a specific version
// GET /api/v1/version/changelog?version=v0.1.0
// If version not provided, defaults to current platform version
func (h *VersionHandler) GetChangelog(c *fiber.Ctx) error {
	version := c.Query("version")

	// If no version specified, use current platform version
	if version == "" {
		pv, err := h.repo.GetPlatformVersion(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get platform version: " + err.Error(),
			})
		}
		version = pv.Version
	}

	// Normalize version to ensure v prefix and canonical form
	version = h.service.Normalize(version)

	// Get entries for this version
	entries, _ := changelog.GetEntriesForVersion(version)

	// Return empty array if no entries (not an error)
	if entries == nil {
		entries = []changelog.Entry{}
	}

	return c.JSON(fiber.Map{
		"version": version,
		"entries": entries,
	})
}

// GetChangelogSince returns all changelog entries between a version and current platform version
// GET /api/v1/version/changelog/since?from=v0.1.0
// Returns changelogs for versions in range (from, current] - exclusive of 'from', inclusive of current
func (h *VersionHandler) GetChangelogSince(c *fiber.Ctx) error {
	fromVersion := c.Query("from")

	// 'from' is required
	if fromVersion == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing 'from' query parameter",
		})
	}

	// Normalize fromVersion
	fromVersion = h.service.Normalize(fromVersion)

	// Get current platform version as toVersion
	pv, err := h.repo.GetPlatformVersion(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get platform version: " + err.Error(),
		})
	}
	toVersion := pv.Version

	// Get changelogs between versions
	changelogs := changelog.GetEntriesBetweenVersions(fromVersion, toVersion)

	return c.JSON(fiber.Map{
		"fromVersion": fromVersion,
		"toVersion":   toVersion,
		"changelogs":  changelogs,
	})
}

// GetMigrationStatus returns current migration status for admin visibility
// GET /api/v1/version/migration-status
func (h *VersionHandler) GetMigrationStatus(c *fiber.Ctx) error {
	ctx := c.Context()

	// Get platform version
	pv, err := h.repo.GetPlatformVersion(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get platform version: " + err.Error(),
		})
	}

	// Get total org count
	orgList, err := h.authRepo.ListOrganizations(ctx, 1, 1)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get organizations: " + err.Error(),
		})
	}
	totalOrgs := orgList.Total

	// Get failed runs
	failedRuns, err := h.migrationRepo.GetFailedRuns(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get migration status: " + err.Error(),
		})
	}

	// Get last run time
	lastRunAt, _ := h.migrationRepo.GetLastRunTime(ctx)

	// Build failed orgs response
	var failedOrgs []entity.FailedOrg
	for _, run := range failedRuns {
		failedOrgs = append(failedOrgs, entity.FailedOrg{
			OrgID:            run.OrgID,
			OrgName:          run.OrgName,
			ErrorMessage:     run.ErrorMessage,
			FailedAt:         run.StartedAt,
			AttemptedVersion: run.ToVersion,
		})
	}

	upToDate := totalOrgs - len(failedOrgs)

	return c.JSON(entity.MigrationStatusResponse{
		PlatformVersion: pv.Version,
		TotalOrgs:       totalOrgs,
		UpToDateCount:   upToDate,
		FailedCount:     len(failedOrgs),
		FailedOrgs:      failedOrgs,
		LastRunAt:       lastRunAt,
	})
}

// RetryMigration retries migration for a specific failed org
// POST /api/v1/version/migration-retry/:orgId
func (h *VersionHandler) RetryMigration(c *fiber.Ctx) error {
	orgID := c.Params("orgId")
	if orgID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Organization ID is required",
		})
	}

	if h.migrationPropagator == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Migration propagator not initialized",
		})
	}

	run, err := h.migrationPropagator.RetryOrg(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": run.Status == "success",
		"run":     run,
	})
}

// RetryAllFailed retries migration for all failed orgs
// POST /api/v1/version/migration-retry-all
func (h *VersionHandler) RetryAllFailed(c *fiber.Ctx) error {
	if h.migrationPropagator == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Migration propagator not initialized",
		})
	}

	// Get failed runs
	failedRuns, err := h.migrationRepo.GetFailedRuns(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get failed runs: " + err.Error(),
		})
	}

	if len(failedRuns) == 0 {
		return c.JSON(fiber.Map{
			"message":      "No failed migrations to retry",
			"successCount": 0,
			"failedCount":  0,
		})
	}

	var successCount, failedCount int
	var results []entity.MigrationRun

	for _, failedRun := range failedRuns {
		run, err := h.migrationPropagator.RetryOrg(c.Context(), failedRun.OrgID)
		if err != nil {
			failedCount++
			continue
		}
		results = append(results, *run)
		if run.Status == "success" {
			successCount++
		} else {
			failedCount++
		}
	}

	return c.JSON(fiber.Map{
		"successCount": successCount,
		"failedCount":  failedCount,
		"runs":         results,
	})
}
