package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/dedup"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// DedupHandler handles duplicate detection API endpoints
type DedupHandler struct {
	defaultDB db.DBConn
	ruleRepo  *repo.MatchingRuleRepo
	alertRepo *repo.PendingAlertRepo
	detector  *dedup.Detector
}

// NewDedupHandler creates a new dedup handler
func NewDedupHandler(conn db.DBConn, ruleRepo *repo.MatchingRuleRepo, alertRepo *repo.PendingAlertRepo) *DedupHandler {
	return &DedupHandler{
		defaultDB: conn,
		ruleRepo:  ruleRepo,
		alertRepo: alertRepo,
		detector:  dedup.NewDetector(ruleRepo, "US"),
	}
}

// getDB returns tenant DB from context
func (h *DedupHandler) getDB(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getRuleRepo returns rule repo with tenant DB
func (h *DedupHandler) getRuleRepo(c *fiber.Ctx) *repo.MatchingRuleRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.ruleRepo.WithDB(tenantDB)
	}
	return h.ruleRepo
}

// getAlertRepo returns alert repo with tenant DB
func (h *DedupHandler) getAlertRepo(c *fiber.Ctx) *repo.PendingAlertRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.alertRepo.WithDB(tenantDB)
	}
	return h.alertRepo
}

// --- Matching Rules CRUD ---

// ListRules returns all matching rules for the org
func (h *DedupHandler) ListRules(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Query("entityType", "")

	rules, err := h.getRuleRepo(c).ListRules(c.Context(), orgID, entityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": rules})
}

// GetRule returns a single matching rule
func (h *DedupHandler) GetRule(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	ruleID := c.Params("id")

	rule, err := h.getRuleRepo(c).GetRule(c.Context(), orgID, ruleID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rule == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Rule not found"})
	}

	return c.JSON(rule)
}

// CreateRule creates a new matching rule
func (h *DedupHandler) CreateRule(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var input entity.MatchingRuleCreateInput
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate required fields
	if input.Name == "" || input.EntityType == "" || len(input.FieldConfigs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name, entityType, and fieldConfigs are required",
		})
	}

	if input.Threshold <= 0 || input.Threshold > 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "threshold must be between 0 and 1",
		})
	}

	rule, err := h.getRuleRepo(c).CreateRule(c.Context(), orgID, input)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "A rule with this name already exists for this entity type"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(rule)
}

// UpdateRule updates an existing matching rule
func (h *DedupHandler) UpdateRule(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	ruleID := c.Params("id")

	var input entity.MatchingRuleUpdateInput
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	rule, err := h.getRuleRepo(c).UpdateRule(c.Context(), orgID, ruleID, input)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "A rule with this name already exists for this entity type"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rule == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Rule not found"})
	}

	return c.JSON(rule)
}

// DeleteRule deletes a matching rule
func (h *DedupHandler) DeleteRule(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	ruleID := c.Params("id")

	err := h.getRuleRepo(c).DeleteRule(c.Context(), orgID, ruleID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- Duplicate Detection ---

// CheckDuplicates checks for duplicates of a given record
func (h *DedupHandler) CheckDuplicates(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Optional: exclude specific record ID (for update checking)
	excludeID := c.Query("excludeId", "")

	matches, err := h.detector.CheckForDuplicates(c.Context(), h.getDB(c), orgID, entityType, body, excludeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"duplicates": matches,
		"count":      len(matches),
	})
}

// --- Pending Alert Management ---

// GetPendingAlert returns the pending alert for a specific record
// It re-runs duplicate detection to verify duplicates still exist and auto-dismisses if none remain
func (h *DedupHandler) GetPendingAlert(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")
	recordID := c.Params("id")

	alert, err := h.getAlertRepo(c).GetPendingByRecord(c.Context(), orgID, entityType, recordID)
	if err != nil {
		// Tolerate missing dedup tables in orgs that haven't had dedup migrations run
		if strings.Contains(err.Error(), "no such table") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No pending alert"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if alert == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No pending alert"})
	}

	// Re-run duplicate detection to verify duplicates still exist
	// This handles: deleted records, merged records, changed data
	conn := h.getDB(c)

	// Fetch current record data
	record, err := util.FetchRecordAsMap(c.Context(), db.GetRawDB(conn), util.GetTableName(entityType), recordID, orgID)
	if err != nil {
		// Record itself was deleted
		log.Printf("[DEDUP] Auto-dismissing alert for deleted record %s/%s", entityType, recordID)
		_ = h.getAlertRepo(c).Resolve(c.Context(), orgID, entityType, recordID, entity.AlertStatusDismissed, "system", "auto-dismissed: record deleted")
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No pending alert"})
	}

	// Re-check for duplicates
	matches, err := h.detector.CheckForDuplicates(c.Context(), conn, orgID, entityType, record, recordID)
	if err != nil {
		log.Printf("[DEDUP] Error re-checking duplicates for %s/%s: %v", entityType, recordID, err)
		// Fall back to returning cached alert on error
		return c.JSON(alert)
	}

	// If no duplicates found anymore, auto-dismiss
	if len(matches) == 0 {
		log.Printf("[DEDUP] Auto-dismissing stale alert for %s/%s (no duplicates found on re-check)", entityType, recordID)
		_ = h.getAlertRepo(c).Resolve(c.Context(), orgID, entityType, recordID, entity.AlertStatusDismissed, "system", "auto-dismissed: no duplicates found")
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No pending alert"})
	}

	// Update alert with fresh match data
	freshMatches := make([]entity.DuplicateAlertMatch, 0, len(matches))
	highestConfidence := entity.ConfidenceLow
	for _, match := range matches {
		freshMatches = append(freshMatches, entity.DuplicateAlertMatch{
			RecordID:    match.RecordID,
			MatchResult: match.MatchResult,
		})
		if match.MatchResult != nil {
			tier := match.MatchResult.ConfidenceTier
			if tier == entity.ConfidenceHigh {
				highestConfidence = entity.ConfidenceHigh
			} else if tier == entity.ConfidenceMedium && highestConfidence != entity.ConfidenceHigh {
				highestConfidence = entity.ConfidenceMedium
			}
		}
	}

	alert.Matches = freshMatches
	alert.TotalMatchCount = len(freshMatches)
	alert.HighestConfidence = highestConfidence

	return c.JSON(alert)
}

// ResolveAlert resolves a pending duplicate alert
func (h *DedupHandler) ResolveAlert(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	entityType := c.Params("entity")
	recordID := c.Params("id")

	var input struct {
		Status       string `json:"status"`
		OverrideText string `json:"overrideText"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Validate status
	validStatuses := map[string]bool{
		entity.AlertStatusDismissed:     true,
		entity.AlertStatusCreatedAnyway: true,
		entity.AlertStatusMerged:        true,
	}
	if !validStatuses[input.Status] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid status. Must be: dismissed, created_anyway, or merged",
		})
	}

	err := h.getAlertRepo(c).Resolve(c.Context(), orgID, entityType, recordID, input.Status, userID, input.OverrideText)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListPendingAlerts returns all pending alerts for an org with optional filtering and pagination
func (h *DedupHandler) ListPendingAlerts(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Query("entityType", "")

	// Parse pagination params
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}

	pageSize := c.QueryInt("pageSize", 20)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	// Fetch alerts
	alerts, total, err := h.getAlertRepo(c).ListAllPending(c.Context(), orgID, entityType, pageSize, offset)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return c.JSON(fiber.Map{"data": []any{}, "total": 0, "page": page, "pageSize": pageSize})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":     alerts,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// BackfillBlockingKeys populates blocking key columns for all existing records
// This is needed for records created before blocking keys were being populated
func (h *DedupHandler) BackfillBlockingKeys(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	if entityType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "entityType is required"})
	}

	conn := h.getDB(c)
	tableName := util.GetTableName(entityType)
	blocker := h.detector.GetBlocker()

	// Query all records for this entity
	query := fmt.Sprintf(`SELECT * FROM %s WHERE org_id = ?`, tableName)
	rows, err := conn.QueryContext(c.Context(), query, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	// Scan all records into maps
	records, err := util.ScanRowsToMaps(rows)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Update blocking keys for each record
	updated := 0
	failed := 0
	for _, record := range records {
		recordID := getStringFromMap(record, "id")
		if recordID == "" {
			continue
		}

		// Convert snake_case keys to camelCase for blocker
		camelRecord := make(map[string]interface{})
		for k, v := range record {
			camelKey := util.SnakeToCamel(k)
			camelRecord[camelKey] = v
		}

		if err := blocker.UpdateBlockingKeys(c.Context(), conn, entityType, recordID, camelRecord); err != nil {
			log.Printf("Failed to update blocking keys for %s/%s: %v", entityType, recordID, err)
			failed++
		} else {
			updated++
		}
	}

	log.Printf("[DEDUP] Backfilled blocking keys for %s: %d updated, %d failed", entityType, updated, failed)

	return c.JSON(fiber.Map{
		"entityType": entityType,
		"total":      len(records),
		"updated":    updated,
		"failed":     failed,
	})
}

// getStringFromMap extracts string value from map
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// RegisterRoutes registers dedup routes
func (h *DedupHandler) RegisterRoutes(app fiber.Router) {
	// Matching rules CRUD (admin only - apply admin middleware in main.go)
	app.Get("/dedup/rules", h.ListRules)
	app.Get("/dedup/rules/:id", h.GetRule)
	app.Post("/dedup/rules", h.CreateRule)
	app.Put("/dedup/rules/:id", h.UpdateRule)
	app.Delete("/dedup/rules/:id", h.DeleteRule)

	// Admin: Backfill blocking keys for existing records
	app.Post("/dedup/:entity/backfill-blocking-keys", h.BackfillBlockingKeys)

	// Duplicate detection
	app.Post("/dedup/:entity/check", h.CheckDuplicates)

	// Pending alert endpoints
	app.Get("/dedup/pending-alerts", h.ListPendingAlerts)
	app.Get("/dedup/:entity/:id/pending-alert", h.GetPendingAlert)
	app.Post("/dedup/:entity/:id/resolve-alert", h.ResolveAlert)
}
