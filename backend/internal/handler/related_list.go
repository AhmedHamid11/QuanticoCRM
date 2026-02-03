package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// RelatedListHandler handles HTTP requests for related list configurations
type RelatedListHandler struct {
	relatedListRepo *repo.RelatedListRepo
	metadataRepo    *repo.MetadataRepo
	defaultDB       *sql.DB
}

// NewRelatedListHandler creates a new RelatedListHandler
func NewRelatedListHandler(relatedListRepo *repo.RelatedListRepo, metadataRepo *repo.MetadataRepo, defaultDB *sql.DB) *RelatedListHandler {
	return &RelatedListHandler{
		relatedListRepo: relatedListRepo,
		metadataRepo:    metadataRepo,
		defaultDB:       defaultDB,
	}
}

// getDBConn returns the retry-enabled tenant database connection from context
// This is the preferred method for new code
func (h *RelatedListHandler) getDBConn(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getDB returns the raw tenant database from context, falling back to default db
// DEPRECATED: Use getDBConn for retry-enabled connections
func (h *RelatedListHandler) getDB(c *fiber.Ctx) *sql.DB {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getRelatedListRepo returns the repo using the tenant database from context
// It also ensures the schema is up to date for the tenant database
func (h *RelatedListHandler) getRelatedListRepo(c *fiber.Ctx) *repo.RelatedListRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		r := h.relatedListRepo.WithDB(tenantDB)
		// Ensure schema is up to date (handles missing columns in older tenant DBs)
		r.EnsureSchema(c.Context())
		return r
	}
	return h.relatedListRepo
}

// getMetadataRepo returns the metadata repo using the tenant database from context
// It also ensures the schema is up to date for the tenant database
func (h *RelatedListHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		r := h.metadataRepo.WithDB(tenantDB)
		// Ensure schema is up to date (handles missing columns in older tenant DBs)
		r.EnsureSchema(c.Context())
		return r
	}
	return h.metadataRepo
}

// isDBConnectionError checks if the error indicates a closed/stale database connection
func isDBConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "database is closed") ||
		strings.Contains(errStr, "bad connection") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "stream is closed")
}

// queryWithRetry executes a query with automatic retry on connection errors
// Works with both *sql.DB and db.DBConn interfaces
func queryWithRetry(ctx context.Context, conn db.DBConn, query string, args ...interface{}) (*sql.Rows, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		rows, err := conn.QueryContext(ctx, query, args...)
		if err == nil {
			return rows, nil
		}
		lastErr = err
		if isDBConnectionError(err) {
			log.Printf("[TENANT-DB] Query failed (attempt %d/3): %v", attempt+1, err)
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			continue
		}
		// Non-connection error, return immediately
		return nil, err
	}
	return nil, fmt.Errorf("query failed after 3 attempts: %w", lastErr)
}

// scanIntWithRetry executes a query that returns a single int value with retry logic
// This wraps QueryRow + Scan to enable proper retry on connection errors
// Works with both *sql.DB and db.DBConn interfaces
func scanIntWithRetry(ctx context.Context, conn db.DBConn, query string, args ...interface{}) (int, error) {
	var result int
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		row := conn.QueryRowContext(ctx, query, args...)
		if row == nil {
			lastErr = fmt.Errorf("QueryRowContext returned nil")
			log.Printf("[TENANT-DB] ScanInt failed (attempt %d/3): %v", attempt+1, lastErr)
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			continue
		}
		err := row.Scan(&result)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if isDBConnectionError(err) {
			log.Printf("[TENANT-DB] ScanInt failed (attempt %d/3): %v", attempt+1, err)
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			continue
		}
		// Non-connection error, return immediately
		return 0, err
	}
	return 0, fmt.Errorf("scan failed after 3 attempts: %w", lastErr)
}

// execWithRetry executes a statement with automatic retry on connection errors
// Works with both *sql.DB and db.DBConn interfaces
func execWithRetry(ctx context.Context, conn db.DBConn, query string, args ...interface{}) (sql.Result, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		result, err := conn.ExecContext(ctx, query, args...)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if isDBConnectionError(err) {
			log.Printf("[TENANT-DB] Exec failed (attempt %d/3): %v", attempt+1, err)
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			continue
		}
		// Non-connection error, return immediately
		return nil, err
	}
	return nil, fmt.Errorf("exec failed after 3 attempts: %w", lastErr)
}

// DiscoverRelatedListOptions returns all possible related lists for an entity
// GET /api/v1/entities/:entity/related-list-options
func (h *RelatedListHandler) DiscoverRelatedListOptions(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	options, err := h.getRelatedListRepo(c).DiscoverRelatedLists(c.Context(), orgID, entityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(options)
}

// ListConfigs returns configured related lists for an entity
// GET /api/v1/entities/:entity/related-list-configs
func (h *RelatedListHandler) ListConfigs(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	configs, err := h.getRelatedListRepo(c).ListByEntity(c.Context(), orgID, entityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(configs)
}

// SaveConfigs saves all related list configs for an entity (bulk update)
// PUT /api/v1/entities/:entity/related-list-configs
func (h *RelatedListHandler) SaveConfigs(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	var input entity.RelatedListConfigBulkInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	configs, err := h.getRelatedListRepo(c).BulkSave(c.Context(), orgID, entityType, input.Configs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(configs)
}

// CreateConfig creates a new related list config
// POST /api/v1/entities/:entity/related-list-configs
func (h *RelatedListHandler) CreateConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entity")

	var input entity.RelatedListConfigCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if input.RelatedEntity == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "relatedEntity is required",
		})
	}
	if input.LookupField == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "lookupField is required",
		})
	}
	if input.Label == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "label is required",
		})
	}

	config, err := h.getRelatedListRepo(c).Create(c.Context(), orgID, entityType, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(config)
}

// UpdateConfig updates a related list config
// PUT /api/v1/entities/:entity/related-list-configs/:id
func (h *RelatedListHandler) UpdateConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	configID := c.Params("id")

	var input entity.RelatedListConfigUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	config, err := h.getRelatedListRepo(c).Update(c.Context(), orgID, configID, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if config == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Related list config not found",
		})
	}

	return c.JSON(config)
}

// DeleteConfig deletes a related list config
// DELETE /api/v1/entities/:entity/related-list-configs/:id
func (h *RelatedListHandler) DeleteConfig(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	configID := c.Params("id")

	err := h.getRelatedListRepo(c).Delete(c.Context(), orgID, configID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Related list config not found",
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// GetRelatedRecords fetches actual related records for display
// GET /api/v1/:entity/:id/related/:relatedEntity
func (h *RelatedListHandler) GetRelatedRecords(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityParam := c.Params("entity") // e.g., "accounts" or "farmcustomers"
	recordID := c.Params("id")
	relatedEntity := c.Params("relatedEntity") // e.g., "Contact"

	// Convert URL entity to actual entity name
	// First try to look it up in the database by matching lowercase name
	entityType, err := h.getMetadataRepo(c).GetEntityByLowercaseName(c.Context(), orgID, entityParam)
	if err != nil || entityType == "" {
		// Fall back to simple conversion (for standard entities like "accounts" -> "Account")
		entityType = urlEntityToPascalCase(entityParam)
		log.Printf("[RELATED-LIST] Entity lookup fallback: urlParam=%s resolved=%s (lookup err=%v)", entityParam, entityType, err)
	}

	// Parse query parameters
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 5)
	sortField := c.Query("sort", "created_at")
	sortDir := c.Query("dir", "desc")

	if pageSize > 100 {
		pageSize = 100
	}
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc"
	}

	// Get the related list config to find the lookup field
	configs, err := h.getRelatedListRepo(c).ListByEntity(c.Context(), orgID, entityType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Find the config for this related entity
	var config *entity.RelatedListConfig
	for i := range configs {
		if configs[i].RelatedEntity == relatedEntity {
			config = &configs[i]
			break
		}
	}

	// If no config found, try to discover the relationship from metadata
	var lookupField string
	var isMultiLookup bool
	if config != nil {
		lookupField = config.LookupField
		isMultiLookup = config.IsMultiLookup
	} else {
		// Fall back to discovering from field definitions
		fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, relatedEntity)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		for _, field := range fields {
			if (field.Type == entity.FieldTypeLink || field.Type == entity.FieldTypeLinkMultiple) &&
			   field.LinkEntity != nil && *field.LinkEntity == entityType {
				lookupField = field.Name
				isMultiLookup = field.Type == entity.FieldTypeLinkMultiple
				break
			}
		}

		if lookupField == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("No relationship found between %s and %s", entityType, relatedEntity),
			})
		}
	}

	// Get the table name by converting PascalCase to snake_case and pluralizing
	// e.g., "Contact" -> "contacts", "ClientContact" -> "client_contacts"
	// Use the canonical GetTableName to ensure consistency with table creation
	tableName := util.GetTableName(relatedEntity)

	// Convert lookup field from camelCase to snake_case for SQL
	snakeCaseLookupField := camelToSnake(lookupField)

	// Fix legacy lookup field names that were incorrectly stored
	// "client_name" was used instead of "clientId" for Submittal -> Client relationship
	if relatedEntity == "Submittal" && (lookupField == "client_name" || lookupField == "clientName") {
		lookupField = "clientId"
		snakeCaseLookupField = "client_id"
		// Ensure submittals table has client_id column in tenant database
		ensureSubmittalsClientIDColumn(h.getDBConn(c))
	}

	// Check if the related entity is a custom entity
	// Custom entities have different column naming conventions but ALWAYS have org_id for multi-tenant isolation
	// SECURITY: We check if the table has org_id column - if it does, we MUST filter by it
	isCustomEntity := false
	relatedEntityDef, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, relatedEntity)
	if err == nil && relatedEntityDef != nil {
		isCustomEntity = relatedEntityDef.IsCustom
	}

	// SECURITY CRITICAL: Check if table has org_id column - if it does, we MUST filter by it
	// This ensures multi-tenant data isolation even for custom entities
	dbConn := h.getDBConn(c)

	// First check if the table exists at all
	tableExists := tableExistsWithDB(dbConn, tableName)
	if !tableExists {
		log.Printf("[RELATED-LIST] Table does not exist: %s for entity %s", tableName, relatedEntity)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("Related entity table '%s' not found. The entity may not have been fully provisioned.", relatedEntity),
		})
	}

	tableHasOrgID := tableHasColumnWithDB(dbConn, tableName, "org_id")
	tableHasDeleted := tableHasColumnWithDB(dbConn, tableName, "deleted")

	log.Printf("[RELATED-LIST] Table=%s hasOrgID=%v hasDeleted=%v isCustom=%v lookupField=%s snakeLookup=%s",
		tableName, tableHasOrgID, tableHasDeleted, isCustomEntity, lookupField, snakeCaseLookupField)

	// Validate sort field to prevent SQL injection
	validSortFields := map[string]bool{
		"created_at": true, "modified_at": true, "name": true, "id": true,
		"first_name": true, "last_name": true, "email_address": true,
		"subject": true, "due_date": true, "status": true, "priority": true, "type": true,
	}
	snakeCaseSortField := camelToSnake(sortField)
	if !validSortFields[snakeCaseSortField] {
		snakeCaseSortField = "created_at"
	}

	// Special handling for Task entity (polymorphic relationship)
	// Tasks use parent_type + parent_id instead of a simple foreign key
	var countQuery string
	var queryArgs []interface{}
	if relatedEntity == "Task" {
		countQuery = `
			SELECT COUNT(*) FROM tasks
			WHERE org_id = ? AND parent_type = ? AND parent_id = ? AND deleted = 0
		`
		queryArgs = []interface{}{orgID, entityType, recordID}
	} else if isMultiLookup {
		// For multi-lookup fields, the IDs are stored in a JSON array column (e.g., companies_test_ids)
		// We need to check if the parent recordID is contained in the JSON array
		idsColumn := snakeCaseLookupField + "_ids"
		// SECURITY: Always filter by org_id when the table has it (multi-tenant isolation)
		if tableHasOrgID {
			if tableHasDeleted {
				countQuery = fmt.Sprintf(`
					SELECT COUNT(*) FROM %s
					WHERE org_id = ? AND deleted = 0
					AND EXISTS (
						SELECT 1 FROM json_each(%s) WHERE json_each.value = ?
					)
				`, tableName, idsColumn)
			} else {
				countQuery = fmt.Sprintf(`
					SELECT COUNT(*) FROM %s
					WHERE org_id = ?
					AND EXISTS (
						SELECT 1 FROM json_each(%s) WHERE json_each.value = ?
					)
				`, tableName, idsColumn)
			}
			queryArgs = []interface{}{orgID, recordID}
		} else {
			// Legacy table without org_id - log warning and skip (should not happen after migration)
			countQuery = fmt.Sprintf(`
				SELECT COUNT(*) FROM %s
				WHERE EXISTS (
					SELECT 1 FROM json_each(%s) WHERE json_each.value = ?
				)
			`, tableName, idsColumn)
			queryArgs = []interface{}{recordID}
		}
	} else {
		// For single lookup fields, determine the column name using schema introspection
		// This handles all cases correctly:
		// - Standard entities: field "accountId" -> column "account_id"
		// - Custom entities: field "form433D" -> column "form433_d_id"
		lookupColumn := snakeCaseLookupField
		customColumn := snakeCaseLookupField + "_id"

		// Try columns in order of precedence - use schema introspection to find actual column
		if tableHasColumnWithDB(dbConn, tableName, customColumn) {
			// Table has {field_name}_id column (standard custom entity pattern)
			lookupColumn = customColumn
		} else if tableHasColumnWithDB(dbConn, tableName, snakeCaseLookupField) {
			// Table has {field_name} column directly (some legacy patterns)
			lookupColumn = snakeCaseLookupField
		} else if !strings.HasSuffix(lookupField, "Id") {
			// Try with "_id" appended for standard entities
			lookupColumn = snakeCaseLookupField + "_id"
		}

		// Validate the lookup column exists before building query
		if !tableHasColumnWithDB(dbConn, tableName, lookupColumn) {
			log.Printf("[RELATED-LIST] Lookup column not found: table=%s column=%s (tried: %s, %s)",
				tableName, lookupColumn, customColumn, snakeCaseLookupField)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Lookup column '%s' not found in table '%s'. The relationship may not be configured correctly.", lookupColumn, tableName),
			})
		}

		// Quote the column name to handle identifiers starting with numbers (e.g., "433d_form_id")
		quotedLookupColumn := util.QuoteIdentifier(lookupColumn)

		// SECURITY: Always filter by org_id when the table has it (multi-tenant isolation)
		if tableHasOrgID {
			if tableHasDeleted {
				countQuery = fmt.Sprintf(`
					SELECT COUNT(*) FROM %s
					WHERE org_id = ? AND %s = ? AND deleted = 0
				`, tableName, quotedLookupColumn)
			} else {
				countQuery = fmt.Sprintf(`
					SELECT COUNT(*) FROM %s
					WHERE org_id = ? AND %s = ?
				`, tableName, quotedLookupColumn)
			}
			queryArgs = []interface{}{orgID, recordID}
		} else {
			// Legacy table without org_id - should not happen after migration
			countQuery = fmt.Sprintf(`
				SELECT COUNT(*) FROM %s
				WHERE %s = ?
			`, tableName, quotedLookupColumn)
			queryArgs = []interface{}{recordID}
		}
	}

	// Debug logging for related records query
	log.Printf("[RELATED-LIST] Entity=%s Related=%s Table=%s Query=%s Args=%v",
		entityType, relatedEntity, tableName, countQuery, queryArgs)

	total, err := scanIntWithRetry(c.Context(), dbConn, countQuery, queryArgs...)
	if err != nil {
		log.Printf("[RELATED-LIST] Count query failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to count related records: %v", err),
		})
	}

	// Build the query to fetch records
	offset := (page - 1) * pageSize
	var query string
	var selectArgs []interface{}
	if relatedEntity == "Task" {
		query = fmt.Sprintf(`
			SELECT * FROM tasks
			WHERE org_id = ? AND parent_type = ? AND parent_id = ? AND deleted = 0
			ORDER BY %s %s
			LIMIT ? OFFSET ?
		`, snakeCaseSortField, strings.ToUpper(sortDir))
		selectArgs = []interface{}{orgID, entityType, recordID, pageSize, offset}
	} else if isMultiLookup {
		// For multi-lookup fields, use JSON array contains query
		idsColumn := snakeCaseLookupField + "_ids"
		// SECURITY: Always filter by org_id when the table has it (multi-tenant isolation)
		if tableHasOrgID {
			if tableHasDeleted {
				query = fmt.Sprintf(`
					SELECT * FROM %s
					WHERE org_id = ? AND deleted = 0
					AND EXISTS (
						SELECT 1 FROM json_each(%s) WHERE json_each.value = ?
					)
					ORDER BY %s %s
					LIMIT ? OFFSET ?
				`, tableName, idsColumn, snakeCaseSortField, strings.ToUpper(sortDir))
			} else {
				query = fmt.Sprintf(`
					SELECT * FROM %s
					WHERE org_id = ?
					AND EXISTS (
						SELECT 1 FROM json_each(%s) WHERE json_each.value = ?
					)
					ORDER BY %s %s
					LIMIT ? OFFSET ?
				`, tableName, idsColumn, snakeCaseSortField, strings.ToUpper(sortDir))
			}
			selectArgs = []interface{}{orgID, recordID, pageSize, offset}
		} else {
			// Legacy table without org_id
			query = fmt.Sprintf(`
				SELECT * FROM %s
				WHERE EXISTS (
					SELECT 1 FROM json_each(%s) WHERE json_each.value = ?
				)
				ORDER BY %s %s
				LIMIT ? OFFSET ?
			`, tableName, idsColumn, snakeCaseSortField, strings.ToUpper(sortDir))
			selectArgs = []interface{}{recordID, pageSize, offset}
		}
	} else {
		// For single lookup fields, use same column discovery as count query
		// Column already validated in count query section above
		lookupColumn := snakeCaseLookupField
		customColumn := snakeCaseLookupField + "_id"

		// Match column discovery logic from count query
		if tableHasColumnWithDB(dbConn, tableName, customColumn) {
			lookupColumn = customColumn
		} else if tableHasColumnWithDB(dbConn, tableName, snakeCaseLookupField) {
			lookupColumn = snakeCaseLookupField
		} else if !strings.HasSuffix(lookupField, "Id") {
			lookupColumn = snakeCaseLookupField + "_id"
		}

		// Quote the column name to handle identifiers starting with numbers (e.g., "433d_form_id")
		quotedLookupColumn := util.QuoteIdentifier(lookupColumn)

		// SECURITY: Always filter by org_id when the table has it (multi-tenant isolation)
		if tableHasOrgID {
			if tableHasDeleted {
				query = fmt.Sprintf(`
					SELECT * FROM %s
					WHERE org_id = ? AND %s = ? AND deleted = 0
					ORDER BY %s %s
					LIMIT ? OFFSET ?
				`, tableName, quotedLookupColumn, snakeCaseSortField, strings.ToUpper(sortDir))
			} else {
				query = fmt.Sprintf(`
					SELECT * FROM %s
					WHERE org_id = ? AND %s = ?
					ORDER BY %s %s
					LIMIT ? OFFSET ?
				`, tableName, quotedLookupColumn, snakeCaseSortField, strings.ToUpper(sortDir))
			}
			selectArgs = []interface{}{orgID, recordID, pageSize, offset}
		} else {
			// Legacy table without org_id
			query = fmt.Sprintf(`
				SELECT * FROM %s
				WHERE %s = ?
				ORDER BY %s %s
				LIMIT ? OFFSET ?
			`, tableName, quotedLookupColumn, snakeCaseSortField, strings.ToUpper(sortDir))
			selectArgs = []interface{}{recordID, pageSize, offset}
		}
	}

	rows, err := queryWithRetry(c.Context(), dbConn, query, selectArgs...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get related records: %v", err),
		})
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to get columns",
		})
	}

	// Prepare to scan rows into maps
	var records []map[string]interface{}
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to scan record",
			})
		}

		// Build the record map with camelCase keys
		record := make(map[string]interface{})
		for i, col := range columns {
			camelCol := snakeToCamel(col)
			val := values[i]
			// Handle SQL NULL values
			if val == nil {
				record[camelCol] = nil
			} else if b, ok := val.([]byte); ok {
				// Handle byte arrays (strings in SQLite)
				record[camelCol] = string(b)
			} else {
				record[camelCol] = val
			}
		}

		// Parse custom fields if present
		if customFieldsRaw, ok := record["customFields"].(string); ok && customFieldsRaw != "" {
			var customFields map[string]interface{}
			if err := json.Unmarshal([]byte(customFieldsRaw), &customFields); err == nil {
				record["customFields"] = customFields
			}
		}

		records = append(records, record)
	}

	if records == nil {
		records = []map[string]interface{}{}
	}

	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}

	return c.JSON(entity.RelatedRecordsResponse{
		Records:    records,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// camelToSnake converts a camelCase string to snake_case
func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteByte(byte(r + 32)) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// snakeToCamel converts a snake_case string to camelCase
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// urlEntityToPascalCase converts a URL entity path (e.g., "accounts") to PascalCase entity name (e.g., "Account")
func urlEntityToPascalCase(urlEntity string) string {
	// Remove trailing 's' if present (e.g., "accounts" -> "account")
	name := strings.TrimSuffix(urlEntity, "s")
	// Capitalize first letter (e.g., "account" -> "Account")
	if len(name) > 0 {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}

// tableExistsWithDB checks if a table exists in the database
// Uses retry logic to handle connection errors
func tableExistsWithDB(conn db.DBConn, tableName string) bool {
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, err := scanIntWithRetry(ctx, conn, query, tableName)
	if err != nil {
		return false
	}
	return count > 0
}

// tableHasColumnWithDB checks if a table has a specific column using provided db connection
// Uses retry logic to handle connection errors
// Works with both *sql.DB and db.DBConn interfaces
func tableHasColumnWithDB(conn db.DBConn, tableName, columnName string) bool {
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", tableName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, err := scanIntWithRetry(ctx, conn, query, columnName)
	if err != nil {
		return false
	}
	return count > 0
}

// ensureSubmittalsClientIDColumn ensures the submittals table has client_id column
// This is a migration that runs on first access to handle tenant databases
// Uses retry logic to handle connection errors
// Works with both *sql.DB and db.DBConn interfaces
func ensureSubmittalsClientIDColumn(conn db.DBConn) {
	if conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if column already exists
	if tableHasColumnWithDB(conn, "submittals", "client_id") {
		return
	}
	// Add the client_id column
	_, err := execWithRetry(ctx, conn, "ALTER TABLE submittals ADD COLUMN client_id TEXT")
	if err != nil {
		log.Printf("[TENANT-DB] Failed to add client_id column: %v", err)
		return // Column might already exist or table doesn't exist
	}
	log.Println("[TENANT-DB] Added client_id column to submittals table")
	// Create index for efficient queries
	_, _ = execWithRetry(ctx, conn, "CREATE INDEX IF NOT EXISTS idx_submittals_client ON submittals(client_id)")
	// Backfill client_id from job_openings relationship
	result, err := execWithRetry(ctx, conn, `
		UPDATE submittals
		SET client_id = (
			SELECT j.client_id
			FROM job_openings j
			WHERE j.id = submittals.job_opening_id
		)
		WHERE client_id IS NULL AND job_opening_id IS NOT NULL
	`)
	if err == nil {
		rows, _ := result.RowsAffected()
		log.Printf("[TENANT-DB] Backfilled client_id for %d submittals", rows)
	}
}

// RegisterPublicRoutes registers read-only related list routes for all authenticated users
// These are needed for displaying related lists on detail pages
func (h *RelatedListHandler) RegisterPublicRoutes(app fiber.Router) {
	entities := app.Group("/entities")
	// Read-only: get related list configs for display on detail pages
	entities.Get("/:entity/related-list-configs", h.ListConfigs)

	// Related records endpoints (for detail page display)
	// These work for any entity type dynamically
	app.Get("/:entity/:id/related/:relatedEntity", h.GetRelatedRecords)
}

// RegisterRoutes registers admin related list routes on the Fiber app
func (h *RelatedListHandler) RegisterRoutes(app fiber.Router) {
	// Related list config endpoints (admin/layout editor)
	entities := app.Group("/entities")
	entities.Get("/:entity/related-list-options", h.DiscoverRelatedListOptions)
	entities.Put("/:entity/related-list-configs", h.SaveConfigs)
	entities.Post("/:entity/related-list-configs", h.CreateConfig)
	entities.Put("/:entity/related-list-configs/:id", h.UpdateConfig)
	entities.Patch("/:entity/related-list-configs/:id", h.UpdateConfig)
	entities.Delete("/:entity/related-list-configs/:id", h.DeleteConfig)
}
