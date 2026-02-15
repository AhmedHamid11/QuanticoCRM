package handler

import (
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// DedupResultsHandler exposes import dedup decisions via the ingest API (X-API-Key auth)
type DedupResultsHandler struct {
	importJobRepo *repo.ImportJobRepo
}

// NewDedupResultsHandler creates a new DedupResultsHandler
func NewDedupResultsHandler(importJobRepo *repo.ImportJobRepo) *DedupResultsHandler {
	return &DedupResultsHandler{importJobRepo: importJobRepo}
}

// ListImports handles GET /api/v1/ingest/imports
// Returns a paginated list of import jobs for the authenticated org
func (h *DedupResultsHandler) ListImports(c *fiber.Ctx) error {
	orgID, ok := c.Locals("ingestOrgID").(string)
	if !ok || orgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing organization context"})
	}

	tenantDB := middleware.GetTenantDB(c)
	if tenantDB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database connection error"})
	}

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	entityType := c.Query("entityType")
	since := c.Query("since")

	jobs, total, err := h.importJobRepo.ListJobs(c.Context(), tenantDB, orgID, entityType, since, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"imports": jobs,
		"total":   total,
		"page":    page,
		"pageSize": pageSize,
	})
}

// GetDedupResults handles GET /api/v1/ingest/imports/:id/dedup-results
// Returns the import job metadata + all dedup decisions
func (h *DedupResultsHandler) GetDedupResults(c *fiber.Ctx) error {
	orgID, ok := c.Locals("ingestOrgID").(string)
	if !ok || orgID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing organization context"})
	}

	tenantDB := middleware.GetTenantDB(c)
	if tenantDB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database connection error"})
	}

	jobID := c.Params("id")
	if jobID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Import ID is required"})
	}

	job, err := h.importJobRepo.GetJob(c.Context(), tenantDB, orgID, jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if job == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Import job not found"})
	}

	decisions, err := h.importJobRepo.GetDecisions(c.Context(), tenantDB, orgID, jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"import_id":   job.ID,
		"entity_type": job.EntityType,
		"total_rows":  job.TotalRows,
		"created":     job.CreatedCount,
		"updated":     job.UpdatedCount,
		"skipped":     job.SkippedCount,
		"merged":      job.MergedCount,
		"failed":      job.FailedCount,
		"created_at":  job.CreatedAt,
		"decisions":   decisions,
	})
}
