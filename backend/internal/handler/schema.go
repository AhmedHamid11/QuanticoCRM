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

// SchemaHandler handles schema discovery endpoints for the Gmail extension
type SchemaHandler struct {
	defaultDB    db.DBConn
	metadataRepo *repo.MetadataRepo
}

// NewSchemaHandler creates a new SchemaHandler
func NewSchemaHandler(conn db.DBConn, metadataRepo *repo.MetadataRepo) *SchemaHandler {
	return &SchemaHandler{
		defaultDB:    conn,
		metadataRepo: metadataRepo,
	}
}

// getDB returns the tenant database from context, falling back to default db
func (h *SchemaHandler) getDB(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getMetadataRepo returns a metadata repo using the tenant database from context
func (h *SchemaHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// EntityInfo represents a simplified entity for the schema API
type EntityInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// FieldInfo represents a simplified field for the schema API
type FieldInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// SearchFilter represents a filter in a search request
type SearchFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// SearchRequest represents a search request body
type SearchRequest struct {
	Filters []SearchFilter `json:"filters"`
}

// ListEntities returns all CRM entity types
// GET /schema/entities
func (h *SchemaHandler) ListEntities(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	entities, err := h.getMetadataRepo(c).ListEntities(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Convert to simplified response format
	data := make([]EntityInfo, 0, len(entities))
	for _, e := range entities {
		data = append(data, EntityInfo{
			Name:  e.Name,
			Label: e.Label,
		})
	}

	return c.JSON(fiber.Map{
		"data": data,
	})
}

// GetEmailFields returns all email-type fields grouped by entity
// GET /schema/entities/fields?type=email
func (h *SchemaHandler) GetEmailFields(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	fieldType := c.Query("type", "")

	// Only support email type filter for now
	if fieldType != "email" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Only type=email filter is supported",
		})
	}

	// Get all entities first
	entities, err := h.getMetadataRepo(c).ListEntities(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Build map of entity -> email fields
	result := make(map[string][]FieldInfo)

	for _, ent := range entities {
		fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, ent.Name)
		if err != nil {
			continue // Skip entities we can't read fields for
		}

		var emailFields []FieldInfo
		for _, f := range fields {
			// Include email type fields
			if f.Type == entity.FieldTypeEmail {
				emailFields = append(emailFields, FieldInfo{
					Name:  f.Name,
					Label: f.Label,
				})
			}
		}

		// Only include entities that have email fields
		if len(emailFields) > 0 {
			result[ent.Name] = emailFields
		}
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}

// Search searches for records by field value
// POST /schema/entities/:entity/search
func (h *SchemaHandler) Search(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Try to resolve entity name case-insensitively
	resolvedName, err := h.getMetadataRepo(c).GetEntityByLowercaseName(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if resolvedName != "" {
		entityName = resolvedName
	}

	// Verify entity exists for this org
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
	}

	// Parse request body
	var req SearchRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(req.Filters) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one filter is required",
		})
	}

	// Get field definitions to validate filters
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Build a map of valid field names
	validFields := make(map[string]entity.FieldDef)
	for _, f := range fields {
		validFields[f.Name] = f
	}

	tableName := util.GetTableName(entityName)

	// Build WHERE clause from filters
	var whereParts []string
	var whereArgs []interface{}

	// Always filter by org_id for multi-tenant isolation
	whereParts = append(whereParts, "org_id = ?")
	whereArgs = append(whereArgs, orgID)

	for _, filter := range req.Filters {
		// Validate field exists
		fieldDef, exists := validFields[filter.Field]
		if !exists {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Field '%s' does not exist on entity '%s'", filter.Field, entityName),
			})
		}

		// Convert field name to column name
		columnName := util.CamelToSnake(filter.Field)

		// Handle lookup fields (they store id and name separately)
		if fieldDef.Type == entity.FieldTypeLink {
			columnName = columnName + "_name" // Search by the display name
		}

		switch filter.Operator {
		case "equals":
			// Case-insensitive equals for email fields
			if fieldDef.Type == entity.FieldTypeEmail {
				whereParts = append(whereParts, fmt.Sprintf("LOWER(%s) = LOWER(?)", util.QuoteIdentifier(columnName)))
			} else {
				whereParts = append(whereParts, fmt.Sprintf("%s = ?", util.QuoteIdentifier(columnName)))
			}
			whereArgs = append(whereArgs, filter.Value)
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Unsupported operator '%s'. Supported: equals", filter.Operator),
			})
		}
	}

	// Build and execute query
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s LIMIT 10",
		tableName, strings.Join(whereParts, " AND "))

	rows, err := h.getDB(c).QueryContext(c.Context(), query, whereArgs...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Scan rows into maps
	var records []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		record := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string
			if b, ok := val.([]byte); ok {
				record[col] = string(b)
			} else {
				record[col] = val
			}
		}

		// Convert snake_case columns to camelCase
		camelRecord := make(map[string]interface{})
		for col, val := range record {
			camelCol := util.SnakeToCamel(col)
			camelRecord[camelCol] = val
		}

		// Add a "name" field if it doesn't exist (for display purposes)
		// Try to derive from entity's display field or common patterns
		if camelRecord["name"] == nil || camelRecord["name"] == "" {
			// Try common name patterns
			if firstName, ok := camelRecord["firstName"].(string); ok {
				if lastName, ok := camelRecord["lastName"].(string); ok {
					camelRecord["name"] = strings.TrimSpace(firstName + " " + lastName)
				} else {
					camelRecord["name"] = firstName
				}
			} else if title, ok := camelRecord["title"].(string); ok {
				camelRecord["name"] = title
			} else if companyName, ok := camelRecord["companyName"].(string); ok {
				camelRecord["name"] = companyName
			}
		}

		records = append(records, camelRecord)
	}

	if err := rows.Err(); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(fiber.Map{"data": []map[string]interface{}{}})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if records == nil {
		records = []map[string]interface{}{}
	}

	return c.JSON(fiber.Map{
		"data": records,
	})
}

// RegisterRoutes registers schema routes
func (h *SchemaHandler) RegisterRoutes(app fiber.Router) {
	schema := app.Group("/schema")

	// List all entities
	schema.Get("/entities", h.ListEntities)

	// Get email fields grouped by entity
	schema.Get("/entities/fields", h.GetEmailFields)

	// Search records by field value
	schema.Post("/entities/:entity/search", h.Search)
}
