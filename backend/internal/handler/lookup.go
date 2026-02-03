package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// LookupHandler handles lookup field operations
type LookupHandler struct {
	defaultDB    db.DBConn
	metadataRepo *repo.MetadataRepo
}

// NewLookupHandler creates a new LookupHandler
func NewLookupHandler(conn db.DBConn, metadataRepo *repo.MetadataRepo) *LookupHandler {
	return &LookupHandler{defaultDB: conn, metadataRepo: metadataRepo}
}

// getDB returns the tenant database from context, falling back to default db
func (h *LookupHandler) getDB(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getMetadataRepo returns metadata repo with tenant DB if available
func (h *LookupHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// entityConfig holds entity-specific configuration for lookups
type entityConfig struct {
	tableName    string
	displayField string
	searchFields []string
}

// getEntityConfig returns configuration for an entity by querying entity_defs
func (h *LookupHandler) getEntityConfig(c *fiber.Ctx, entityName string) (*entityConfig, error) {
	orgID := c.Locals("orgID").(string)

	// Query entity_defs for this entity
	entityDef, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity config: %w", err)
	}
	if entityDef == nil {
		return nil, fmt.Errorf("unknown entity: %s", entityName)
	}

	// Parse search fields from JSON
	var searchFields []string
	if err := json.Unmarshal([]byte(entityDef.SearchFields), &searchFields); err != nil {
		// Fallback to name if JSON parsing fails
		searchFields = []string{"name"}
	}

	// Handle empty search fields
	if len(searchFields) == 0 {
		searchFields = []string{"name"}
	}

	// Get display field with fallback
	displayField := entityDef.DisplayField
	if displayField == "" {
		displayField = "name"
	}

	return &entityConfig{
		tableName:    util.GetTableName(entityName),
		displayField: displayField,
		searchFields: searchFields,
	}, nil
}

// Search searches for records in an entity for lookup autocomplete
// GET /api/v1/lookup/:entity?search=term&limit=10
func (h *LookupHandler) Search(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	search := c.Query("search", "")
	limit := c.QueryInt("limit", 10)

	if limit > 50 {
		limit = 50
	}

	config, err := h.getEntityConfig(c, entityName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Build search query
	var query string
	var args []any

	if search == "" {
		// Return recent records
		query = fmt.Sprintf(`
			SELECT id, %s as name
			FROM %s
			WHERE org_id = ? AND deleted = 0
			ORDER BY created_at DESC
			LIMIT ?
		`, config.displayField, config.tableName)
		args = []any{orgID, limit}
	} else {
		// Search by fields
		searchConditions := make([]string, len(config.searchFields))
		searchTerm := "%" + search + "%"
		for i, field := range config.searchFields {
			searchConditions[i] = field + " LIKE ?"
			args = append(args, searchTerm)
		}
		whereClause := "(" + strings.Join(searchConditions, " OR ") + ")"

		query = fmt.Sprintf(`
			SELECT id, %s as name
			FROM %s
			WHERE org_id = ? AND deleted = 0 AND %s
			ORDER BY created_at DESC
			LIMIT ?
		`, config.displayField, config.tableName, whereClause)

		// Prepend orgID and append limit
		args = append([]any{orgID}, args...)
		args = append(args, limit)
	}

	rows, err := h.getDB(c).QueryContext(c.Context(), query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to search %s: %v", entityName, err),
		})
	}
	defer rows.Close()

	var records []entity.LookupRecord
	for rows.Next() {
		var record entity.LookupRecord
		if err := rows.Scan(&record.ID, &record.Name); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to scan record",
			})
		}
		records = append(records, record)
	}

	if records == nil {
		records = []entity.LookupRecord{}
	}

	return c.JSON(entity.LookupSearchResult{
		Records: records,
		Total:   len(records),
	})
}

// Get retrieves a single record for lookup display
// GET /api/v1/lookup/:entity/:id
func (h *LookupHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	id := c.Params("id")

	config, err := h.getEntityConfig(c, entityName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	query := fmt.Sprintf(`
		SELECT id, %s as name
		FROM %s
		WHERE id = ? AND org_id = ? AND deleted = 0
	`, config.displayField, config.tableName)

	var record entity.LookupRecord
	err = h.getDB(c).QueryRowContext(c.Context(), query, id, orgID).Scan(&record.ID, &record.Name)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("%s not found", entityName),
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get %s: %v", entityName, err),
		})
	}

	return c.JSON(record)
}

// GetMultiple retrieves multiple records for lookup display (batch)
// POST /api/v1/lookup/:entity/batch with body {"ids": ["id1", "id2"]}
func (h *LookupHandler) GetMultiple(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	var input struct {
		IDs []string `json:"ids"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(input.IDs) == 0 {
		return c.JSON(entity.LookupSearchResult{
			Records: []entity.LookupRecord{},
			Total:   0,
		})
	}

	config, err := h.getEntityConfig(c, entityName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Build query with placeholders
	placeholders := make([]string, len(input.IDs))
	args := make([]any, len(input.IDs)+1)
	args[0] = orgID
	for i, id := range input.IDs {
		placeholders[i] = "?"
		args[i+1] = id
	}

	query := fmt.Sprintf(`
		SELECT id, %s as name
		FROM %s
		WHERE org_id = ? AND id IN (%s) AND deleted = 0
	`, config.displayField, config.tableName, strings.Join(placeholders, ","))

	rows, err := h.getDB(c).QueryContext(c.Context(), query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get %s records: %v", entityName, err),
		})
	}
	defer rows.Close()

	var records []entity.LookupRecord
	for rows.Next() {
		var record entity.LookupRecord
		if err := rows.Scan(&record.ID, &record.Name); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to scan record",
			})
		}
		records = append(records, record)
	}

	if records == nil {
		records = []entity.LookupRecord{}
	}

	return c.JSON(entity.LookupSearchResult{
		Records: records,
		Total:   len(records),
	})
}

// RegisterRoutes registers lookup routes on the Fiber app
func (h *LookupHandler) RegisterRoutes(app fiber.Router) {
	lookup := app.Group("/lookup")
	lookup.Get("/:entity", h.Search)
	lookup.Get("/:entity/:id", h.Get)
	lookup.Post("/:entity/batch", h.GetMultiple)
}
