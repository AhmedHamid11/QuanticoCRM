package handler

import (
	"fmt"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

// RelatedHandler handles related record operations
type RelatedHandler struct {
	defaultDB db.DBConn
}

// NewRelatedHandler creates a new RelatedHandler
func NewRelatedHandler(conn db.DBConn) *RelatedHandler {
	return &RelatedHandler{defaultDB: conn}
}

// getDB returns the tenant database from context, falling back to default db
func (h *RelatedHandler) getDB(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// relationshipConfig holds relationship-specific configuration
type relationshipConfig struct {
	relatedTable    string
	foreignKey      string
	displayFields   []string
	displayTemplate string // SQL expression for display name
}

// getRelationshipConfig returns configuration for known relationships
func getRelationshipConfig(fromEntity, relationship string) (*relationshipConfig, error) {
	// Key format: "FromEntity.relationship"
	key := fromEntity + "." + relationship

	configs := map[string]*relationshipConfig{
		"Account.contacts": {
			relatedTable:    "contacts",
			foreignKey:      "account_id",
			displayFields:   []string{"id", "first_name", "last_name", "email_address"},
			displayTemplate: "first_name || ' ' || last_name",
		},
		// Add more relationships as needed
		// "Account.opportunities": {...},
		// "Contact.cases": {...},
	}

	config, ok := configs[key]
	if !ok {
		return nil, fmt.Errorf("unknown relationship: %s.%s", fromEntity, relationship)
	}
	return config, nil
}

// RelatedRecord represents a related record with key fields
type RelatedRecord struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
}

// RelatedRecordsResponse represents the response for related records
type RelatedRecordsResponse struct {
	Records    []RelatedRecord `json:"records"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	TotalPages int             `json:"totalPages"`
}

// List returns related records for a given entity
// GET /api/v1/accounts/:id/related/:relationship
// Example: GET /api/v1/accounts/001xxx/related/contacts
func (h *RelatedHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityID := c.Params("id")
	relationship := c.Params("relationship")

	// Extract entity name from path (e.g., "/api/v1/accounts/..." -> "accounts")
	path := c.Path()
	entityName := extractEntityFromPath(path)

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	if pageSize > 100 {
		pageSize = 100
	}

	config, err := getRelationshipConfig(capitalizeFirst(entityName), relationship)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s
		WHERE org_id = ? AND %s = ? AND deleted = 0
	`, config.relatedTable, config.foreignKey)

	var total int
	if err := h.getDB(c).QueryRowContext(c.Context(), countQuery, orgID, entityID).Scan(&total); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to count related records",
		})
	}

	// Get records
	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`
		SELECT id, %s as name
		FROM %s
		WHERE org_id = ? AND %s = ? AND deleted = 0
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, config.displayTemplate, config.relatedTable, config.foreignKey)

	rows, err := h.getDB(c).QueryContext(c.Context(), query, orgID, entityID, pageSize, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get related records: %v", err),
		})
	}
	defer rows.Close()

	var records []RelatedRecord
	for rows.Next() {
		var record RelatedRecord
		if err := rows.Scan(&record.ID, &record.Name); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to scan record",
			})
		}
		records = append(records, record)
	}

	if records == nil {
		records = []RelatedRecord{}
	}

	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}

	return c.JSON(RelatedRecordsResponse{
		Records:    records,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// extractEntityFromPath extracts the entity name from a path like "/api/v1/accounts/..."
func extractEntityFromPath(path string) string {
	parts := strings.Split(path, "/")
	// Path format: /api/v1/{entity}/{id}/related/{relationship}
	// parts[0] = "", parts[1] = "api", parts[2] = "v1", parts[3] = entity
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	// Handle common plural -> singular mappings for entity names
	entityMap := map[string]string{
		"accounts": "Account",
		"contacts": "Contact",
	}
	if mapped, ok := entityMap[s]; ok {
		return mapped
	}
	// Default: capitalize first letter
	return string(s[0]-32) + s[1:]
}

// RegisterRoutes registers related routes on the Fiber app
// NOTE: These routes are disabled in favor of the new RelatedListHandler
// which provides dynamic discovery and configuration via the admin UI
func (h *RelatedHandler) RegisterRelatedRoutes(app fiber.Router) {
	// Disabled - using RelatedListHandler instead
	// app.Get("/accounts/:id/related/:relationship", h.List)
	// app.Get("/contacts/:id/related/:relationship", h.List)
}
