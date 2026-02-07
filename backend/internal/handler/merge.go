package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// MergeHandler handles merge API endpoints
type MergeHandler struct {
	defaultDB        *sql.DB
	mergeRepo        *repo.MergeRepo
	mergeService     *service.MergeService
	discoveryService *service.MergeDiscoveryService
	metadataRepo     *repo.MetadataRepo
}

// NewMergeHandler creates a new MergeHandler
func NewMergeHandler(
	defaultDB *sql.DB,
	mergeRepo *repo.MergeRepo,
	mergeService *service.MergeService,
	discoveryService *service.MergeDiscoveryService,
	metadataRepo *repo.MetadataRepo,
) *MergeHandler {
	return &MergeHandler{
		defaultDB:        defaultDB,
		mergeRepo:        mergeRepo,
		mergeService:     mergeService,
		discoveryService: discoveryService,
		metadataRepo:     metadataRepo,
	}
}

// getDB returns tenant DB from context, falling back to default
func (h *MergeHandler) getDB(c *fiber.Ctx) *sql.DB {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getDBConn returns tenant DBConn from context, falling back to default
func (h *MergeHandler) getDBConn(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getMetadataRepo returns metadata repo with tenant DB
func (h *MergeHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// getMergeRepo returns merge repo with tenant DB
func (h *MergeHandler) getMergeRepo(c *fiber.Ctx) *repo.MergeRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.mergeRepo.WithDB(tenantDB)
	}
	return h.mergeRepo
}

// Preview handles POST /api/v1/merge/preview
// Returns side-by-side comparison of records with completeness scores, suggested survivor, and related record counts
func (h *MergeHandler) Preview(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var req entity.MergePreviewRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate inputs
	if len(req.RecordIDs) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "At least 2 record IDs are required"})
	}
	if req.EntityType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "entityType is required"})
	}

	// Verify entity exists
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, req.EntityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, req.EntityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Fetch each record
	tableName := util.GetTableName(req.EntityType)
	var records []map[string]interface{}
	completenessScores := make(map[string]float64)

	for _, recordID := range req.RecordIDs {
		record, err := util.FetchRecordAsMap(c.Context(), h.getDB(c), tableName, recordID, orgID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to fetch record %s: %v", recordID, err),
			})
		}
		if record == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": fmt.Sprintf("Record not found: %s", recordID),
			})
		}

		records = append(records, record)
		completenessScores[recordID] = h.discoveryService.CalculateCompleteness(record)
	}

	// Suggest survivor based on completeness
	suggestedSurvivorID := h.discoveryService.SuggestSurvivor(records)

	// Count related records for each duplicate
	relatedRecordCounts, err := h.discoveryService.CountRelatedRecords(
		c.Context(),
		h.getDBConn(c),
		orgID,
		req.EntityType,
		req.RecordIDs,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to count related records: %v", err),
		})
	}

	// Return preview
	preview := entity.MergePreview{
		Records:             records,
		SuggestedSurvivorID: suggestedSurvivorID,
		CompletenessScores:  completenessScores,
		RelatedRecordCounts: relatedRecordCounts,
		Fields:              fields,
	}

	return c.JSON(preview)
}

// Execute handles POST /api/v1/merge/execute
// Performs atomic merge and returns survivor ID and snapshot ID
func (h *MergeHandler) Execute(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var req entity.MergeRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate inputs
	if req.SurvivorID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "survivorId is required"})
	}
	if len(req.DuplicateIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "At least one duplicate ID is required"})
	}
	if req.EntityType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "entityType is required"})
	}

	// Execute merge
	result, err := h.mergeService.ExecuteMerge(c.Context(), h.getDB(c), orgID, userID, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(result)
}

// Undo handles POST /api/v1/merge/undo/:snapshotId
// Reverses a merge operation within the 30-day window
func (h *MergeHandler) Undo(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	snapshotID := c.Params("snapshotId")

	if snapshotID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "snapshotId is required"})
	}

	// Undo merge
	err := h.mergeService.UndoMerge(c.Context(), h.getDB(c), orgID, userID, snapshotID)
	if err != nil {
		// Check for specific error types
		if err.Error() == fmt.Sprintf("snapshot not found: %s", snapshotID) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		if err.Error() == "snapshot has expired (undo window is 30 days)" ||
			err.Error() == "snapshot has already been consumed (undo already performed)" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// History handles GET /api/v1/merge/history
// Lists recent merges with undo eligibility
func (h *MergeHandler) History(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	// Parse pagination parameters
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	entityTypeFilter := c.Query("entityType", "")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Fetch merge history (repo doesn't support entityType filter, we'll filter in memory)
	snapshots, total, err := h.getMergeRepo(c).ListByOrg(c.Context(), orgID, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Convert to history entries with canUndo flag
	var entries []entity.MergeHistoryEntry
	for _, snapshot := range snapshots {
		// Apply entity type filter if provided
		if entityTypeFilter != "" && snapshot.EntityType != entityTypeFilter {
			continue
		}

		// Parse duplicate IDs from JSON
		var duplicateIDs []string
		if err := json.Unmarshal([]byte(snapshot.DuplicateIDs), &duplicateIDs); err != nil {
			duplicateIDs = []string{}
		}

		// Determine if undo is possible (not consumed and not expired)
		canUndo := snapshot.ConsumedAt == nil

		entries = append(entries, entity.MergeHistoryEntry{
			SnapshotID:   snapshot.ID,
			EntityType:   snapshot.EntityType,
			SurvivorID:   snapshot.SurvivorID,
			DuplicateIDs: duplicateIDs,
			MergedByID:   snapshot.MergedByID,
			CanUndo:      canUndo,
			CreatedAt:    snapshot.CreatedAt,
			ExpiresAt:    snapshot.ExpiresAt,
		})
	}

	return c.JSON(fiber.Map{
		"data":     entries,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// RegisterRoutes registers merge routes
func (h *MergeHandler) RegisterRoutes(app fiber.Router) {
	merge := app.Group("/merge")
	merge.Post("/preview", h.Preview)
	merge.Post("/execute", h.Execute)
	merge.Post("/undo/:snapshotId", h.Undo)
	merge.Get("/history", h.History)
}
