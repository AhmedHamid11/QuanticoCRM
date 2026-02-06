package handler

import (
	"encoding/json"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/dedup"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// DedupHandler handles duplicate detection API endpoints
type DedupHandler struct {
	defaultDB db.DBConn
	ruleRepo  *repo.MatchingRuleRepo
	detector  *dedup.Detector
}

// NewDedupHandler creates a new dedup handler
func NewDedupHandler(conn db.DBConn, ruleRepo *repo.MatchingRuleRepo) *DedupHandler {
	return &DedupHandler{
		defaultDB: conn,
		ruleRepo:  ruleRepo,
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

// RegisterRoutes registers dedup routes
func (h *DedupHandler) RegisterRoutes(app fiber.Router) {
	// Matching rules CRUD (admin only - apply admin middleware in main.go)
	app.Get("/dedup/rules", h.ListRules)
	app.Get("/dedup/rules/:id", h.GetRule)
	app.Post("/dedup/rules", h.CreateRule)
	app.Put("/dedup/rules/:id", h.UpdateRule)
	app.Delete("/dedup/rules/:id", h.DeleteRule)

	// Duplicate detection
	app.Post("/dedup/:entity/check", h.CheckDuplicates)
}
