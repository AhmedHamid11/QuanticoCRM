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
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// TripwireServiceInterface defines the interface for tripwire service to avoid import cycle
type TripwireServiceInterface interface {
	EvaluateAndFire(ctx context.Context, orgID, entityType, recordID string, eventType string, oldRecord map[string]interface{}, newRecord map[string]interface{})
}

// ValidationServiceInterface defines the interface for validation service to avoid import cycle
type ValidationServiceInterface interface {
	ValidateOperation(ctx context.Context, orgID, entityType, recordID string, operation string, oldRecord, newRecord map[string]interface{}) (*entity.ValidationResult, error)
}

// RealtimeCheckerInterface defines the interface for realtime duplicate checking
type RealtimeCheckerInterface interface {
	CheckAsyncWithMap(conn db.DBConn, orgID, userID, entityType, recordID, recordName string, recordData map[string]interface{})
	HasRulesForEntity(ctx context.Context, conn db.DBConn, orgID, entityType string) bool
}

// StructToMap is an alias for util.StructToMap for backward compatibility
var StructToMap = util.StructToMap

// GenericEntityHandler handles HTTP requests for dynamic entities
type GenericEntityHandler struct {
	defaultDB           db.DBConn
	metadataRepo        *repo.MetadataRepo
	authRepo            *repo.AuthRepo // NEW: for user name lookups
	tripwireService     TripwireServiceInterface
	validationService   ValidationServiceInterface
	realtimeChecker     RealtimeCheckerInterface
	provisioningService ProvisioningServiceInterface
}

// NewGenericEntityHandler creates a new GenericEntityHandler
func NewGenericEntityHandler(conn db.DBConn, metadataRepo *repo.MetadataRepo, authRepo *repo.AuthRepo, tripwireService TripwireServiceInterface, validationService ValidationServiceInterface, realtimeChecker RealtimeCheckerInterface) *GenericEntityHandler {
	return &GenericEntityHandler{
		defaultDB:         conn,
		metadataRepo:      metadataRepo,
		authRepo:          authRepo,
		tripwireService:   tripwireService,
		validationService: validationService,
		realtimeChecker:   realtimeChecker,
	}
}

// getDB returns the tenant database from context, falling back to default db
func (h *GenericEntityHandler) getDB(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// getRawDB returns the raw *sql.DB for legacy functions that require it
func (h *GenericEntityHandler) getRawDB(c *fiber.Ctx) *sql.DB {
	return db.GetRawDB(h.getDB(c))
}

// getMetadataRepo returns a metadata repo using the tenant database from context
// Metadata (entity_defs, field_defs, layout_defs) is stored per-tenant for multi-tenant isolation
func (h *GenericEntityHandler) getMetadataRepo(c *fiber.Ctx) *repo.MetadataRepo {
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		return h.metadataRepo.WithDB(tenantDB)
	}
	return h.metadataRepo
}

// SetProvisioningService injects the provisioning service for auto-provisioning
func (h *GenericEntityHandler) SetProvisioningService(svc ProvisioningServiceInterface) {
	h.provisioningService = svc
}

// autoProvisionIfEmpty checks whether metadata is completely missing for an org
// and auto-provisions it. Returns true if provisioning was triggered.
func (h *GenericEntityHandler) autoProvisionIfEmpty(c *fiber.Ctx, orgID string) bool {
	if h.provisioningService == nil {
		return false
	}

	metaRepo := h.getMetadataRepo(c)
	entities, err := metaRepo.ListEntities(c.Context(), orgID)
	if err != nil || len(entities) > 0 {
		return false // has metadata already, or error checking
	}

	// No entities at all — metadata was never provisioned to this tenant DB
	log.Printf("[AutoProvision] No metadata found for org %s, auto-provisioning...", orgID)

	// Point provisioning at the tenant DB
	if tenantDB := middleware.GetTenantDB(c); tenantDB != nil {
		h.provisioningService.SetDB(tenantDB)
	}

	if err := h.provisioningService.ProvisionDefaultMetadata(c.Context(), orgID); err != nil {
		log.Printf("[AutoProvision] Failed for org %s: %v", orgID, err)
		return false
	}

	log.Printf("[AutoProvision] Successfully provisioned metadata for org %s", orgID)
	return true
}

// getTableName converts entity name to table name (e.g., "Candidate" -> "candidates")
func (h *GenericEntityHandler) getTableName(entityName string) string {
	return util.GetTableName(entityName)
}

// resolveUserNames adds createdByName and modifiedByName to records
func (h *GenericEntityHandler) resolveUserNames(ctx context.Context, records []map[string]interface{}) {
	if h.authRepo == nil || len(records) == 0 {
		return
	}

	// Helper to extract user ID from interface{} (handles both string and *string)
	extractUserID := func(val interface{}) string {
		if val == nil {
			return ""
		}
		if s, ok := val.(string); ok {
			return s
		}
		if sp, ok := val.(*string); ok && sp != nil {
			return *sp
		}
		return ""
	}

	// Collect unique user IDs
	userIDSet := make(map[string]bool)
	for _, record := range records {
		if id := extractUserID(record["createdById"]); id != "" {
			userIDSet[id] = true
		}
		if id := extractUserID(record["modifiedById"]); id != "" {
			userIDSet[id] = true
		}
	}

	if len(userIDSet) == 0 {
		return
	}

	// Convert to slice
	userIDs := make([]string, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}

	// Lookup names
	userNames, err := h.authRepo.GetUserNamesByIDs(ctx, userIDs)
	if err != nil {
		log.Printf("WARNING: Failed to lookup user names: %v", err)
		return
	}

	// Apply names to records
	for _, record := range records {
		if id := extractUserID(record["createdById"]); id != "" {
			record["createdByName"] = userNames[id]
		}
		if id := extractUserID(record["modifiedById"]); id != "" {
			record["modifiedByName"] = userNames[id]
		}
	}
}

// fetchRecordAsMap fetches a record by ID and returns it as a map (for tripwire evaluation)
// CRITICAL: Always filters by org_id to ensure multi-tenant data isolation
func (h *GenericEntityHandler) fetchRecordAsMap(c *fiber.Ctx, tableName, id, orgID string) (map[string]interface{}, error) {
	return util.FetchRecordAsMap(c.Context(), h.getRawDB(c), tableName, id, orgID)
}

// quoteIdentifier is an alias for util.QuoteIdentifier for backward compatibility
var quoteIdentifier = util.QuoteIdentifier

// tableHasColumn checks whether a table contains a given column name.
// Used to conditionally apply filters (e.g. deleted = 0) only when the column exists.
// Accepts db.DBConn interface for retry-enabled connections.
func tableHasColumn(ctx context.Context, conn db.DBConn, tableName, columnName string) bool {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == columnName {
			return true
		}
	}
	return false
}

// ensureTableExists creates the table for a custom entity if it doesn't exist
func (h *GenericEntityHandler) ensureTableExists(ctx *fiber.Ctx, orgID, entityName string) error {
	// Get field definitions for this entity
	fields, err := h.getMetadataRepo(ctx).ListFields(ctx.Context(), orgID, entityName)
	if err != nil {
		return err
	}
	return util.EnsureTableExists(ctx.Context(), h.getDB(ctx), entityName, fields)
}

// List returns all records for a dynamic entity
func (h *GenericEntityHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Ensure metadata schema has all required columns (handles schema drift in tenant DBs)
	if err := h.getMetadataRepo(c).EnsureSchema(c.Context()); err != nil {
		log.Printf("WARNING: Failed to ensure metadata schema: %v", err)
	}

	// Try to resolve entity name case-insensitively (e.g., "jobs" -> "Job")
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

	// Ensure table exists
	if err := h.ensureTableExists(c, orgID, entityName); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Get field definitions to identify lookup fields
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Build maps for lookup and multi-lookup fields for easy access
	lookupFields, multiLookupFields := util.BuildFieldMaps(fields)

	tableName := h.getTableName(entityName)

	// Get query params
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	knownTotal := c.QueryInt("knownTotal", 0)
	includeRollups := c.Query("includeRollups", "true") != "false"
	sortBy := c.Query("sortBy", "created_at")
	sortDir := c.Query("sortDir", "desc")
	search := c.Query("search", "")
	filter := c.Query("filter", "")

	// Cap pageSize at 100 to prevent excessive row reads
	if pageSize > 100 {
		pageSize = 100
	}

	// Validate sort direction
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc"
	}

	// Convert sortBy from camelCase to snake_case for database column matching
	sortByColumn := util.CamelToSnake(sortBy)

	// Build valid sort columns from field definitions to prevent SQL injection
	validSortColumns, linkFieldNames := util.BuildValidSortColumns(fields)

	// Validate sortByColumn against allowed columns, default to created_at if invalid
	if !validSortColumns[sortByColumn] {
		sortByColumn = "created_at"
	}

	// Translate link field sorting to use the _name column (sort by display name)
	if linkFieldNames[sortByColumn] {
		sortByColumn = sortByColumn + "_name"
	}

	offset := (page - 1) * pageSize

	// Parse filter if provided
	var filterResult *util.FilterResult
	if filter != "" {
		// Validate filter syntax first
		if err := util.ValidateFilterSyntax(filter); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid filter: " + err.Error()})
		}
		// Parse the filter
		filterResult, err = util.ParseFilter(filter, fields)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid filter: " + err.Error()})
		}
	}

	// Check if table has a 'deleted' column (custom entities may not)
	hasDeletedCol := tableHasColumn(c.Context(), h.getDB(c), tableName, "deleted")

	// Build common WHERE clause fragments
	var whereParts []string
	var whereArgs []interface{}

	if hasDeletedCol {
		whereParts = append(whereParts, "t.deleted = 0")
	}
	if search != "" {
		// Use entity's configured search_fields if available, otherwise skip search
		searchFields := []string{}
		if ent.SearchFields != "" {
			if err := json.Unmarshal([]byte(ent.SearchFields), &searchFields); err != nil {
				// Log the error but continue without search
				searchFields = []string{}
			}
		}
		// Fallback: if no search fields configured, try common patterns
		if len(searchFields) == 0 {
			// Check if table has a 'name' column
			if tableHasColumn(c.Context(), h.getDB(c), tableName, "name") {
				searchFields = []string{"name"}
			}
		}
		// Build OR clause for search across all configured fields
		if len(searchFields) > 0 {
			var searchParts []string
			for _, sf := range searchFields {
				// Convert camelCase field name to snake_case column name
				colName := util.CamelToSnake(sf)
				// Verify the column exists to prevent SQL errors
				if tableHasColumn(c.Context(), h.getDB(c), tableName, colName) {
					searchParts = append(searchParts, fmt.Sprintf("t.%s LIKE ?", colName))
					whereArgs = append(whereArgs, "%"+search+"%")
				}
			}
			if len(searchParts) > 0 {
				whereParts = append(whereParts, "("+strings.Join(searchParts, " OR ")+")")
			}
		}
	}
	if filterResult != nil && filterResult.WhereClause != "" {
		whereParts = append(whereParts, filterResult.WhereClause)
		whereArgs = append(whereArgs, filterResult.Args...)
	}

	extraWhere := ""
	if len(whereParts) > 0 {
		extraWhere = " AND " + strings.Join(whereParts, " AND ")
	}

	// Skip COUNT(*) if the frontend already knows the total (knownTotal param)
	var total int
	if knownTotal > 0 {
		total = knownTotal
	} else {
		// Count total - CRITICAL: Always filter by org_id to prevent cross-org data access
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s t WHERE t.org_id = ?%s", tableName, extraWhere)
		countArgs := append([]interface{}{orgID}, whereArgs...)
		// Use db.QueryRowScan for retry-enabled count query
		if err := db.QueryRowScan(c.Context(), h.getDB(c), []interface{}{&total}, countQuery, countArgs...); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to count records: " + err.Error()})
		}
	}

	// Fetch records - note: no user JOINs since tenant DBs don't have users table
	// User names will be resolved after fetching via resolveUserNames()
	query := fmt.Sprintf(`SELECT t.*
		FROM %s t
		WHERE t.org_id = ?%s`, tableName, extraWhere)

	args := append([]interface{}{orgID}, whereArgs...)

	query += fmt.Sprintf(" ORDER BY t.%s %s LIMIT ? OFFSET ?", sortByColumn, sortDir)
	args = append(args, pageSize, offset)

	rows, err := h.getDB(c).QueryContext(c.Context(), query, args...)
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

		// Add derived fields for lookups: {fieldName}Id, {fieldName}Name, {fieldName}Link
		for fieldName, fieldDef := range lookupFields {
			snakeName := util.CamelToSnake(fieldName)
			idCol := snakeName + "_id"
			nameCol := snakeName + "_name"

			// Get the ID and Name values from the record
			idVal := record[idCol]
			nameVal := record[nameCol]

			// Set the derived fields in camelCase
			camelRecord[fieldName+"Id"] = idVal
			camelRecord[fieldName+"Name"] = nameVal

			// Generate the link URL if we have an ID and linked entity
			if idVal != nil && idVal != "" && fieldDef.LinkEntity != nil {
				linkedEntityPlural := strings.ToLower(*fieldDef.LinkEntity) + "s"
				camelRecord[fieldName+"Link"] = "/" + linkedEntityPlural + "/" + fmt.Sprintf("%v", idVal)
			} else {
				camelRecord[fieldName+"Link"] = nil
			}
		}

		// Add derived fields for multi-lookups: {fieldName}Ids, {fieldName}Names, {fieldName}Links
		for fieldName, fieldDef := range multiLookupFields {
			snakeName := util.CamelToSnake(fieldName)
			idsCol := snakeName + "_ids"
			namesCol := snakeName + "_names"

			// Get the IDs and Names values from the record
			idsVal := record[idsCol]
			namesVal := record[namesCol]

			// Set the derived fields in camelCase
			camelRecord[fieldName+"Ids"] = idsVal
			camelRecord[fieldName+"Names"] = namesVal

			// Generate the links array if we have IDs and linked entity
			if idsVal != nil && idsVal != "" && idsVal != "[]" && fieldDef.LinkEntity != nil {
				linkedEntityPlural := strings.ToLower(*fieldDef.LinkEntity) + "s"
				// Parse the IDs JSON array and create links
				var ids []string
				if idsStr, ok := idsVal.(string); ok {
					json.Unmarshal([]byte(idsStr), &ids)
				}
				var links []string
				for _, idItem := range ids {
					links = append(links, "/"+linkedEntityPlural+"/"+idItem)
				}
				linksJSON, _ := json.Marshal(links)
				camelRecord[fieldName+"Links"] = string(linksJSON)
			} else {
				camelRecord[fieldName+"Links"] = "[]"
			}
		}

		records = append(records, camelRecord)
	}

	if records == nil {
		records = []map[string]interface{}{}
	}

	// Resolve user names for created_by and modified_by
	h.resolveUserNames(c.Context(), records)

	// Execute rollup fields for each record (in parallel), unless caller opted out
	var rollupFields []*entity.FieldDef
	if includeRollups {
		for i := range fields {
			if fields[i].Type == entity.FieldTypeRollup {
				rollupFields = append(rollupFields, &fields[i])
			}
		}
	}

	if len(rollupFields) > 0 && len(records) > 0 {
		rollupSvc := service.NewRollupService(h.getRawDB(c))

		// Collect all record IDs for batch execution
		recordIDs := make([]string, 0, len(records))
		recordMap := make(map[string]map[string]interface{})
		for _, record := range records {
			if id, ok := record["id"].(string); ok && id != "" {
				recordIDs = append(recordIDs, id)
				recordMap[id] = record
			}
		}

		// Execute each rollup field as a batch query (1 query per field instead of N queries)
		for _, fieldDef := range rollupFields {
			if fieldDef.RollupQuery == nil || fieldDef.RollupResultType == nil {
				continue
			}

			results, err := rollupSvc.ExecuteRollupBatch(c.Context(), *fieldDef.RollupQuery, recordIDs, orgID, *fieldDef.RollupResultType)
			if err != nil {
				// On batch error, set error for all records
				for _, record := range records {
					record[fieldDef.Name] = nil
					record[fieldDef.Name+"Error"] = err.Error()
				}
				continue
			}

			// Map results back to records
			for recordID, result := range results {
				if record, exists := recordMap[recordID]; exists {
					record[fieldDef.Name] = result
				}
			}
		}
	}

	totalPages := (total + pageSize - 1) / pageSize

	return c.JSON(fiber.Map{
		"data":       records,
		"total":      total,
		"totalPages": totalPages,
		"page":       page,
		"pageSize":   pageSize,
	})
}

// Get returns a single record by ID
func (h *GenericEntityHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	id := c.Params("id")

	// Try to resolve entity name case-insensitively (e.g., "jobs" -> "Job")
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

	// Get field definitions to identify lookup fields
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Build maps for lookup and multi-lookup fields for easy access
	lookupFields := make(map[string]*entity.FieldDef)
	multiLookupFields := make(map[string]*entity.FieldDef)
	for i := range fields {
		if fields[i].Type == entity.FieldTypeLink {
			lookupFields[fields[i].Name] = &fields[i]
		}
		if fields[i].Type == entity.FieldTypeLinkMultiple {
			multiLookupFields[fields[i].Name] = &fields[i]
		}
	}

	tableName := h.getTableName(entityName)

	// CRITICAL: Always filter by org_id to prevent cross-org data access
	// Note: no user JOINs since tenant DBs don't have users table
	// User names will be resolved after fetching via resolveUserNames()
	query := fmt.Sprintf(`SELECT t.*
		FROM %s t
		WHERE t.id = ? AND t.org_id = ?`, tableName)
	rows, err := h.getDB(c).QueryContext(c.Context(), query, id, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	columns, _ := rows.Columns()

	if !rows.Next() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Record not found"})
	}

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
		if b, ok := val.([]byte); ok {
			record[col] = string(b)
		} else {
			record[col] = val
		}
	}

	// Convert snake_case columns to camelCase and add derived lookup fields
	camelRecord := make(map[string]interface{})
	for col, val := range record {
		camelCol := util.SnakeToCamel(col)
		camelRecord[camelCol] = val
	}

	// Add derived fields for lookups: {fieldName}Id, {fieldName}Name, {fieldName}Link
	for fieldName, fieldDef := range lookupFields {
		snakeName := util.CamelToSnake(fieldName)
		idCol := snakeName + "_id"
		nameCol := snakeName + "_name"

		// Get the ID and Name values from the record
		idVal := record[idCol]
		nameVal := record[nameCol]

		// Set the derived fields in camelCase
		camelRecord[fieldName+"Id"] = idVal
		camelRecord[fieldName+"Name"] = nameVal

		// Generate the link URL if we have an ID and linked entity
		if idVal != nil && idVal != "" && fieldDef.LinkEntity != nil {
			linkedEntityPlural := strings.ToLower(*fieldDef.LinkEntity) + "s"
			camelRecord[fieldName+"Link"] = "/" + linkedEntityPlural + "/" + fmt.Sprintf("%v", idVal)
		} else {
			camelRecord[fieldName+"Link"] = nil
		}
	}

	// Add derived fields for multi-lookups: {fieldName}Ids, {fieldName}Names, {fieldName}Links
	for fieldName, fieldDef := range multiLookupFields {
		snakeName := util.CamelToSnake(fieldName)
		idsCol := snakeName + "_ids"
		namesCol := snakeName + "_names"

		// Get the IDs and Names values from the record
		idsVal := record[idsCol]
		namesVal := record[namesCol]

		// Set the derived fields in camelCase
		camelRecord[fieldName+"Ids"] = idsVal
		camelRecord[fieldName+"Names"] = namesVal

		// Generate the links array if we have IDs and linked entity
		if idsVal != nil && idsVal != "" && idsVal != "[]" && fieldDef.LinkEntity != nil {
			linkedEntityPlural := strings.ToLower(*fieldDef.LinkEntity) + "s"
			// Parse the IDs JSON array and create links
			var ids []string
			if idsStr, ok := idsVal.(string); ok {
				json.Unmarshal([]byte(idsStr), &ids)
			}
			var links []string
			for _, idItem := range ids {
				links = append(links, "/"+linkedEntityPlural+"/"+idItem)
			}
			linksJSON, _ := json.Marshal(links)
			camelRecord[fieldName+"Links"] = string(linksJSON)
		} else {
			camelRecord[fieldName+"Links"] = "[]"
		}
	}

	// Resolve user names for this single record
	h.resolveUserNames(c.Context(), []map[string]interface{}{camelRecord})

	// Execute rollup fields
	rollupSvc := service.NewRollupService(h.getRawDB(c))
	for i := range fields {
		if fields[i].Type == entity.FieldTypeRollup {
			fieldDef := &fields[i]
			if fieldDef.RollupQuery != nil && fieldDef.RollupResultType != nil {
				result, err := rollupSvc.ExecuteRollup(c.Context(), *fieldDef.RollupQuery, id, orgID, *fieldDef.RollupResultType)
				if err != nil {
					camelRecord[fieldDef.Name] = nil
					camelRecord[fieldDef.Name+"Error"] = err.Error()
				} else {
					camelRecord[fieldDef.Name] = result
				}
			}
		}
	}

	return c.JSON(camelRecord)
}

// Create creates a new record
func (h *GenericEntityHandler) Create(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Try to resolve entity name case-insensitively (e.g., "jobs" -> "Job")
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

	// Ensure table exists
	if err := h.ensureTableExists(c, orgID, entityName); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse body
	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// DEBUG: Log received body for troubleshooting field saving issues

	tableName := h.getTableName(entityName)

	// Get field definitions to know what columns exist
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Ensure table exists for custom entities (creates table if missing)
	// This handles cases where entity was created but table wasn't provisioned
	if err := util.EnsureTableExists(c.Context(), h.getDB(c), entityName, fields); err != nil {
		log.Printf("ERROR: Failed to ensure table exists for %s: %v", entityName, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create table: %v", err),
		})
	}

	// Sync missing columns - ensures all field definitions have corresponding table columns
	// This fixes schema drift where fields exist in metadata but columns are missing
	if columnsAdded, syncErr := util.SyncFieldColumns(c.Context(), h.getDB(c), entityName, fields); syncErr != nil {
		log.Printf("ERROR: Failed to sync columns for %s: %v", entityName, syncErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to prepare table schema: %v", syncErr),
		})
	} else if columnsAdded > 0 {
		log.Printf("INFO: Added %d missing columns to %s table", columnsAdded, tableName)
	}

	// Build INSERT statement
	var columns []string
	var placeholders []string
	var values []interface{}

	// Add ID
	id := sfid.New("Rec")
	columns = append(columns, "id")
	placeholders = append(placeholders, "?")
	values = append(values, id)

	// CRITICAL: Add org_id for multi-tenant isolation
	columns = append(columns, "org_id")
	placeholders = append(placeholders, "?")
	values = append(values, orgID)

	// Add provided fields (and apply defaults for missing fields)
	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Skip textBlock fields - they don't store data (display-only)
		if field.Type == entity.FieldTypeTextBlock {
			continue
		}

		// Handle lookup fields specially - they have {fieldName}Id and {fieldName}Name in body
		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := body[idKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_id"))
				placeholders = append(placeholders, "?")
				values = append(values, idVal)
			}
			if nameVal, ok := body[nameKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_name"))
				placeholders = append(placeholders, "?")
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields - they have {fieldName}Ids and {fieldName}Names (JSON arrays) in body
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := body[idsKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_ids"))
				placeholders = append(placeholders, "?")
				values = append(values, idsVal)
			}
			if namesVal, ok := body[namesKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_names"))
				placeholders = append(placeholders, "?")
				values = append(values, namesVal)
			}
			continue
		}

		// Handle stream fields - they have {fieldName} (entry) and {fieldName}Log (history)
		if field.Type == entity.FieldTypeStream {
			snakeName := util.CamelToSnake(field.Name)
			entryKey := field.Name

			// Get the entry value
			entryVal, hasEntry := body[entryKey]
			entryStr := ""
			if hasEntry && entryVal != nil {
				if s, ok := entryVal.(string); ok {
					entryStr = strings.TrimSpace(s)
				}
			}

			// If entry has content, create initial log with timestamp
			if entryStr != "" {
				timestamp := time.Now().UTC().Format("2006-01-02 15:04")
				newLog := fmt.Sprintf("%s - %s", timestamp, entryStr)

				// Add log column
				columns = append(columns, quoteIdentifier(snakeName+"_log"))
				placeholders = append(placeholders, "?")
				values = append(values, newLog)

				// Entry field stays empty after processing
				columns = append(columns, quoteIdentifier(snakeName))
				placeholders = append(placeholders, "?")
				values = append(values, "")
			} else {
				// No entry, initialize both columns as empty
				columns = append(columns, quoteIdentifier(snakeName))
				placeholders = append(placeholders, "?")
				values = append(values, "")

				columns = append(columns, quoteIdentifier(snakeName+"_log"))
				placeholders = append(placeholders, "?")
				values = append(values, "")
			}
			continue
		}

		// Regular fields - check if value provided or use default
		if val, ok := body[field.Name]; ok {
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, val)
		} else if field.DefaultValue != nil && *field.DefaultValue != "" {
			// Apply default value if not provided
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, *field.DefaultValue)
			// Also add to body so it's returned in the response
			body[field.Name] = *field.DefaultValue
		}
	}

	// Add timestamps and audit fields
	now := time.Now().UTC().Format(time.RFC3339)
	userID := c.Locals("userID").(string)
	columns = append(columns, "created_at", "modified_at", "created_by_id", "modified_by_id")
	placeholders = append(placeholders, "?", "?", "?", "?")
	values = append(values, now, now, userID, userID)

	// Validate before save
	if h.validationService != nil {
		result, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, "", "CREATE", nil, body)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !result.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       result.Message,
				"fieldErrors": result.FieldErrors,
			})
		}
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	// DEBUG: Log INSERT details for troubleshooting

	_, err = h.getDB(c).ExecContext(c.Context(), query, values...)
	if err != nil {
		log.Printf("ERROR Create [%s]: INSERT failed: %v", entityName, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Fire tripwire webhooks for CREATE event
	if h.tripwireService != nil {
		orgID := c.Locals("orgID").(string)
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "CREATE", nil, body)
	}

	// IMPORTANT: Spawn async duplicate detection AFTER record is successfully saved
	// This implements the "optimistic save with async detection" pattern:
	// - Record saves immediately (HTTP response prepared below)
	// - Detection runs in background goroutine
	// - Results surface as alerts when user views the record
	// The record MUST exist in the database before this point.
	if h.realtimeChecker != nil {
		// Fetch the full record from DB so dedup has all fields for blocking keys and scoring.
		// Using just `body` would miss fields the user didn't include in the request.
		recordData, fetchErr := h.fetchRecordAsMap(c, tableName, id, orgID)
		if fetchErr != nil {
			log.Printf("[DEDUP] Failed to fetch full record for dedup check on %s/%s: %v", entityName, id, fetchErr)
		}
		if recordData == nil {
			recordData = body // Fallback to request body if fetch fails
		}

		recordName := ""
		if name, ok := recordData["name"].(string); ok {
			recordName = name
		} else if firstName, ok := recordData["firstName"].(string); ok {
			recordName = firstName
			if lastName, ok := recordData["lastName"].(string); ok {
				recordName += " " + lastName
			}
		}

		h.realtimeChecker.CheckAsyncWithMap(h.getDB(c), orgID, userID, entityName, id, recordName, recordData)
	}

	// Return created record
	body["id"] = id
	body["createdAt"] = now
	body["modifiedAt"] = now
	body["modifiedBy"] = userID

	return c.Status(fiber.StatusCreated).JSON(body)
}

// Update updates an existing record
func (h *GenericEntityHandler) Update(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	id := c.Params("id")

	// Try to resolve entity name case-insensitively (e.g., "jobs" -> "Job")
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

	// Parse body
	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// DEBUG: Log received body for troubleshooting field saving issues

	tableName := h.getTableName(entityName)

	// Fetch old record for tripwire and validation evaluation (before update)
	// CRITICAL: Pass orgID to ensure we only fetch records belonging to this org
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldRecord, _ = h.fetchRecordAsMap(c, tableName, id, orgID)
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Ensure table exists for custom entities (creates table if missing)
	// This handles cases where entity was created but table wasn't provisioned
	if err := util.EnsureTableExists(c.Context(), h.getDB(c), entityName, fields); err != nil {
		log.Printf("ERROR: Failed to ensure table exists for %s: %v", entityName, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create table: %v", err),
		})
	}

	// Sync missing columns - ensures all field definitions have corresponding table columns
	// This fixes schema drift where fields exist in metadata but columns are missing
	if columnsAdded, syncErr := util.SyncFieldColumns(c.Context(), h.getDB(c), entityName, fields); syncErr != nil {
		log.Printf("ERROR: Failed to sync columns for %s: %v", entityName, syncErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to prepare table schema: %v", syncErr),
		})
	} else if columnsAdded > 0 {
		log.Printf("INFO: Added %d missing columns to %s table", columnsAdded, tableName)
	}

	// Build UPDATE statement
	var setClauses []string
	var values []interface{}

	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Skip textBlock fields - they don't store data (display-only)
		if field.Type == entity.FieldTypeTextBlock {
			continue
		}

		// Handle lookup fields specially - they have {fieldName}Id and {fieldName}Name in body
		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := body[idKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_id")))
				values = append(values, idVal)
			}
			if nameVal, ok := body[nameKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_name")))
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields - they have {fieldName}Ids and {fieldName}Names (JSON arrays) in body
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := body[idsKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_ids")))
				values = append(values, idsVal)
			}
			if namesVal, ok := body[namesKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_names")))
				values = append(values, namesVal)
			}
			continue
		}

		// Handle stream fields - they have {fieldName} (entry) and {fieldName}Log (history)
		if field.Type == entity.FieldTypeStream {
			snakeName := util.CamelToSnake(field.Name)
			entryKey := field.Name
			// For oldRecord lookup, use camelCase: activity_log -> activityLog, then + "Log" -> activityLogLog
			camelKey := util.SnakeToCamel(field.Name)
			logKey := camelKey + "Log"

			// Get the current entry value
			entryVal, hasEntry := body[entryKey]
			entryStr := ""
			if hasEntry && entryVal != nil {
				if s, ok := entryVal.(string); ok {
					entryStr = strings.TrimSpace(s)
				}
			}

			// If entry has content, append it to the log with timestamp
			if entryStr != "" {
				// Get existing log value - try both camelCase and snake_case keys
				existingLog := ""
				if logVal, hasLog := body[logKey]; hasLog && logVal != nil {
					if s, ok := logVal.(string); ok {
						existingLog = s
					}
				}

				// If we don't have the log in the body, fetch it from the database
				if existingLog == "" && oldRecord != nil {
					// Try camelCase key first (what the API returns)
					if oldLogVal, ok := oldRecord[logKey]; ok && oldLogVal != nil {
						if s, ok := oldLogVal.(string); ok {
							existingLog = s
						}
					}
				}

				// Create new log entry with timestamp
				timestamp := time.Now().UTC().Format("2006-01-02 15:04")
				newEntry := fmt.Sprintf("%s - %s", timestamp, entryStr)

				// Prepend new entry to log (newest first)
				var newLog string
				if existingLog != "" {
					newLog = newEntry + "\n" + existingLog
				} else {
					newLog = newEntry
				}

				// Update the log column
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_log")))
				values = append(values, newLog)

				// Clear the entry field after appending to log
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName)))
				values = append(values, "")
			}
			continue
		}

		// Regular fields
		if val, ok := body[field.Name]; ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(util.CamelToSnake(field.Name))))
			values = append(values, val)
		}
	}

	// DEBUG: Log UPDATE details

	// Add modified_at and modified_by_id
	now := time.Now().UTC().Format(time.RFC3339)
	userID := c.Locals("userID").(string)
	setClauses = append(setClauses, "modified_at = ?", "modified_by_id = ?")
	values = append(values, now, userID)

	// Validate before save
	if h.validationService != nil {
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, id, "UPDATE", oldRecord, body)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	// Add WHERE clause values - CRITICAL: Include org_id to prevent cross-org updates
	values = append(values, id, orgID)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND org_id = ?", tableName, strings.Join(setClauses, ", "))

	result, err := h.getDB(c).ExecContext(c.Context(), query, values...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Record not found"})
	}

	// Fire tripwire webhooks for UPDATE event
	if h.tripwireService != nil && oldRecord != nil {
		orgID := c.Locals("orgID").(string)
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "UPDATE", oldRecord, body)
	}

	// IMPORTANT: Spawn async duplicate detection AFTER record is successfully updated
	// This implements the "optimistic save with async detection" pattern:
	// - Record updates immediately (database UPDATE already executed above)
	// - Detection runs in background goroutine
	// - Results surface as alerts when user views the record
	// The updated record MUST exist in the database before this point.
	if h.realtimeChecker != nil {
		// Fetch the full updated record from DB so dedup has all fields for blocking keys and scoring.
		// Using just `body` would only have the changed fields, missing lastName/email/etc.
		recordData, fetchErr := h.fetchRecordAsMap(c, tableName, id, orgID)
		if fetchErr != nil {
			log.Printf("[DEDUP] Failed to fetch full record for dedup check on %s/%s: %v", entityName, id, fetchErr)
		}
		if recordData == nil {
			recordData = body // Fallback to request body if fetch fails
		}

		recordName := ""
		if name, ok := recordData["name"].(string); ok {
			recordName = name
		} else if firstName, ok := recordData["firstName"].(string); ok {
			recordName = firstName
			if lastName, ok := recordData["lastName"].(string); ok {
				recordName += " " + lastName
			}
		}

		h.realtimeChecker.CheckAsyncWithMap(h.getDB(c), orgID, userID, entityName, id, recordName, recordData)
	}

	body["id"] = id
	body["modifiedAt"] = now
	body["modifiedBy"] = userID

	return c.JSON(body)
}

// Delete deletes a record
func (h *GenericEntityHandler) Delete(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	id := c.Params("id")

	// Try to resolve entity name case-insensitively (e.g., "jobs" -> "Job")
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

	tableName := h.getTableName(entityName)

	// Fetch record before deletion for tripwire and validation evaluation
	// CRITICAL: Pass orgID to ensure we only fetch records belonging to this org
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldRecord, _ = h.fetchRecordAsMap(c, tableName, id, orgID)
	}

	// Validate before delete
	if h.validationService != nil && oldRecord != nil {
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, id, "DELETE", oldRecord, nil)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	// CRITICAL: Always filter by org_id to prevent cross-org deletes
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ? AND org_id = ?", tableName)
	result, err := h.getDB(c).ExecContext(c.Context(), query, id, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Record not found"})
	}

	// Fire tripwire webhooks for DELETE event
	if h.tripwireService != nil && oldRecord != nil {
		orgID := c.Locals("orgID").(string)
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "DELETE", oldRecord, nil)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListFields returns field definitions for an entity (public endpoint for display purposes)
func (h *GenericEntityHandler) ListFields(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Ensure metadata schema has all required columns (handles schema drift in tenant DBs)
	if err := h.getMetadataRepo(c).EnsureSchema(c.Context()); err != nil {
		log.Printf("WARNING: Failed to ensure metadata schema: %v", err)
	}

	// Try to resolve entity name case-insensitively (e.g., "jobs" -> "Job")
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
		// Auto-provision if no metadata exists at all, then retry
		if h.autoProvisionIfEmpty(c, orgID) {
			resolvedName, _ := h.getMetadataRepo(c).GetEntityByLowercaseName(c.Context(), orgID, entityName)
			if resolvedName != "" {
				entityName = resolvedName
			}
			ent, _ = h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
		}
		if ent == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
		}
	}

	// Get field definitions
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fields)
}

// GetEntityDef returns the entity definition (public endpoint for display purposes)
func (h *GenericEntityHandler) GetEntityDef(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")

	// Try to resolve entity name case-insensitively (e.g., "jobs" -> "Job")
	resolvedName, err := h.getMetadataRepo(c).GetEntityByLowercaseName(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if resolvedName != "" {
		entityName = resolvedName
	}

	// Get entity definition
	ent, err := h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ent == nil {
		if h.autoProvisionIfEmpty(c, orgID) {
			resolvedName, _ := h.getMetadataRepo(c).GetEntityByLowercaseName(c.Context(), orgID, entityName)
			if resolvedName != "" {
				entityName = resolvedName
			}
			ent, _ = h.getMetadataRepo(c).GetEntity(c.Context(), orgID, entityName)
		}
		if ent == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Entity not found"})
		}
	}

	return c.JSON(ent)
}

// GetLayout returns a layout definition (public endpoint for display purposes)
func (h *GenericEntityHandler) GetLayout(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	layoutType := c.Params("type")

	// Try to resolve entity name case-insensitively
	resolvedName, err := h.getMetadataRepo(c).GetEntityByLowercaseName(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if resolvedName != "" {
		entityName = resolvedName
	}

	// Get layout
	layout, err := h.getMetadataRepo(c).GetLayout(c.Context(), orgID, entityName, layoutType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if layout == nil {
		// Auto-provision if no metadata exists, then retry
		if h.autoProvisionIfEmpty(c, orgID) {
			resolvedName, _ := h.getMetadataRepo(c).GetEntityByLowercaseName(c.Context(), orgID, entityName)
			if resolvedName != "" {
				entityName = resolvedName
			}
			layout, _ = h.getMetadataRepo(c).GetLayout(c.Context(), orgID, entityName, layoutType)
		}
		if layout == nil {
			return c.JSON(fiber.Map{
				"entityName": entityName,
				"layoutType": layoutType,
				"layoutData": "[]",
				"exists":     false,
			})
		}
	}

	return c.JSON(fiber.Map{
		"id":         layout.ID,
		"entityName": layout.EntityName,
		"layoutType": layout.LayoutType,
		"layoutData": layout.LayoutData,
		"createdAt":  layout.CreatedAt,
		"modifiedAt": layout.ModifiedAt,
		"exists":     true,
	})
}

// Upsert creates or updates a record based on a match field
// Query params:
//   - matchField: field name to match on (e.g., "email", "externalId") - required
func (h *GenericEntityHandler) Upsert(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	matchField := c.Query("matchField", "")

	if matchField == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "matchField query parameter is required (e.g., ?matchField=email)",
		})
	}

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

	// Parse body
	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Get the match value from the body
	matchValue, ok := body[matchField]
	if !ok || matchValue == nil || matchValue == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("matchField '%s' must be provided in request body with a non-empty value", matchField),
		})
	}

	// Get field definitions to validate matchField exists
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Validate matchField exists in entity definition
	matchFieldExists := false
	for _, field := range fields {
		if field.Name == matchField {
			matchFieldExists = true
			break
		}
	}
	if !matchFieldExists {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("matchField '%s' does not exist on entity '%s'", matchField, entityName),
		})
	}

	// Ensure table exists
	if err := h.ensureTableExists(c, orgID, entityName); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	tableName := h.getTableName(entityName)

	// Sync missing columns - ensures all field definitions have corresponding table columns
	// This fixes schema drift where fields exist in metadata but columns are missing
	if columnsAdded, syncErr := util.SyncFieldColumns(c.Context(), h.getDB(c), entityName, fields); syncErr != nil {
		log.Printf("ERROR: Failed to sync columns for %s: %v", entityName, syncErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to prepare table schema: %v", syncErr),
		})
	} else if columnsAdded > 0 {
		log.Printf("INFO: Added %d missing columns to %s table", columnsAdded, tableName)
	}

	matchColumn := util.CamelToSnake(matchField)

	// Check if record exists with this match value
	// CRITICAL: Always filter by org_id for multi-tenant isolation
	checkQuery := fmt.Sprintf("SELECT id FROM %s WHERE %s = ? AND org_id = ? LIMIT 1",
		tableName, quoteIdentifier(matchColumn))

	var existingID string
	// Use db.QueryRowScan for retry-enabled query
	err = db.QueryRowScan(c.Context(), h.getDB(c), []interface{}{&existingID}, checkQuery, matchValue, orgID)

	if err == sql.ErrNoRows {
		// Record doesn't exist - CREATE
		return h.upsertCreate(c, orgID, entityName, tableName, fields, body)
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Record exists - UPDATE
	return h.upsertUpdate(c, orgID, entityName, tableName, existingID, fields, body)
}

// upsertCreate handles the create path of upsert
func (h *GenericEntityHandler) upsertCreate(c *fiber.Ctx, orgID, entityName, tableName string, fields []entity.FieldDef, body map[string]interface{}) error {
	var columns []string
	var placeholders []string
	var values []interface{}

	// Add ID
	id := sfid.New("Rec")
	columns = append(columns, "id")
	placeholders = append(placeholders, "?")
	values = append(values, id)

	// Add org_id
	columns = append(columns, "org_id")
	placeholders = append(placeholders, "?")
	values = append(values, orgID)

	// Add provided fields
	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Skip textBlock fields - they don't store data (display-only)
		if field.Type == entity.FieldTypeTextBlock {
			continue
		}

		// Handle lookup fields
		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := body[idKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_id"))
				placeholders = append(placeholders, "?")
				values = append(values, idVal)
			}
			if nameVal, ok := body[nameKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_name"))
				placeholders = append(placeholders, "?")
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := body[idsKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_ids"))
				placeholders = append(placeholders, "?")
				values = append(values, idsVal)
			}
			if namesVal, ok := body[namesKey]; ok {
				columns = append(columns, quoteIdentifier(snakeName+"_names"))
				placeholders = append(placeholders, "?")
				values = append(values, namesVal)
			}
			continue
		}

		// Regular fields
		if val, ok := body[field.Name]; ok {
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, val)
		} else if field.DefaultValue != nil && *field.DefaultValue != "" {
			columns = append(columns, quoteIdentifier(util.CamelToSnake(field.Name)))
			placeholders = append(placeholders, "?")
			values = append(values, *field.DefaultValue)
			body[field.Name] = *field.DefaultValue
		}
	}

	// Add timestamps and audit fields
	now := time.Now().UTC().Format(time.RFC3339)
	userID := c.Locals("userID").(string)
	columns = append(columns, "created_at", "modified_at", "created_by_id", "modified_by_id")
	placeholders = append(placeholders, "?", "?", "?", "?")
	values = append(values, now, now, userID, userID)

	// Validate before save
	if h.validationService != nil {
		result, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, "", "CREATE", nil, body)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !result.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       result.Message,
				"fieldErrors": result.FieldErrors,
			})
		}
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err := h.getDB(c).ExecContext(c.Context(), query, values...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Fire tripwire webhooks for CREATE event
	if h.tripwireService != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "CREATE", nil, body)
	}

	body["id"] = id
	body["createdAt"] = now
	body["modifiedAt"] = now
	body["_upsertAction"] = "created"

	return c.Status(fiber.StatusCreated).JSON(body)
}

// upsertUpdate handles the update path of upsert
func (h *GenericEntityHandler) upsertUpdate(c *fiber.Ctx, orgID, entityName, tableName, id string, fields []entity.FieldDef, body map[string]interface{}) error {
	// Fetch old record for tripwire and validation
	var oldRecord map[string]interface{}
	if h.tripwireService != nil || h.validationService != nil {
		oldRecord, _ = h.fetchRecordAsMap(c, tableName, id, orgID)
	}

	var setClauses []string
	var values []interface{}

	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Skip textBlock fields - they don't store data (display-only)
		if field.Type == entity.FieldTypeTextBlock {
			continue
		}

		// Handle lookup fields
		if field.Type == entity.FieldTypeLink {
			snakeName := util.CamelToSnake(field.Name)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := body[idKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_id")))
				values = append(values, idVal)
			}
			if nameVal, ok := body[nameKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_name")))
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := util.CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := body[idsKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_ids")))
				values = append(values, idsVal)
			}
			if namesVal, ok := body[namesKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(snakeName+"_names")))
				values = append(values, namesVal)
			}
			continue
		}

		// Regular fields
		if val, ok := body[field.Name]; ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(util.CamelToSnake(field.Name))))
			values = append(values, val)
		}
	}

	// DEBUG: Log UPDATE details

	// Add modified_at and modified_by_id
	now := time.Now().UTC().Format(time.RFC3339)
	userID := c.Locals("userID").(string)
	setClauses = append(setClauses, "modified_at = ?", "modified_by_id = ?")
	values = append(values, now, userID)

	// Validate before save
	if h.validationService != nil {
		validationResult, err := h.validationService.ValidateOperation(c.Context(), orgID, entityName, id, "UPDATE", oldRecord, body)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !validationResult.Valid {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":       validationResult.Message,
				"fieldErrors": validationResult.FieldErrors,
			})
		}
	}

	values = append(values, id, orgID)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND org_id = ?", tableName, strings.Join(setClauses, ", "))

	result, err := h.getDB(c).ExecContext(c.Context(), query, values...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Record not found"})
	}

	// Fire tripwire webhooks for UPDATE event
	if h.tripwireService != nil && oldRecord != nil {
		go h.tripwireService.EvaluateAndFire(context.Background(), orgID, entityName, id, "UPDATE", oldRecord, body)
	}

	body["id"] = id
	body["modifiedAt"] = now
	body["_upsertAction"] = "updated"

	return c.JSON(body)
}

// DeleteStreamEntry deletes a specific entry from a stream field's log by index
func (h *GenericEntityHandler) DeleteStreamEntry(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityName := c.Params("entity")
	recordID := c.Params("id")
	fieldName := c.Params("fieldName")
	entryIndex := c.QueryInt("index", -1)

	if entryIndex < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing or invalid index parameter"})
	}

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

	// Get field definitions to verify field exists and is a stream type
	fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var streamField *entity.FieldDef
	for _, field := range fields {
		if field.Name == fieldName && field.Type == entity.FieldTypeStream {
			streamField = &field
			break
		}
	}

	if streamField == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Stream field not found"})
	}

	tableName := h.getTableName(entityName)
	snakeFieldName := util.CamelToSnake(fieldName)
	logColumn := snakeFieldName + "_log"

	// Fetch current log value
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = ? AND org_id = ?",
		quoteIdentifier(logColumn), tableName)
	var currentLog sql.NullString
	err = h.getDB(c).QueryRowContext(c.Context(), query, recordID, orgID).Scan(&currentLog)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Record not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if !currentLog.Valid || currentLog.String == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No log entries to delete"})
	}

	// Parse log entries (split by newline)
	entries := strings.Split(currentLog.String, "\n")
	var filteredEntries []string
	for _, e := range entries {
		if strings.TrimSpace(e) != "" {
			filteredEntries = append(filteredEntries, e)
		}
	}

	if entryIndex >= len(filteredEntries) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Entry index out of range"})
	}

	// Remove the entry at the specified index
	newEntries := make([]string, 0, len(filteredEntries)-1)
	for i, entry := range filteredEntries {
		if i != entryIndex {
			newEntries = append(newEntries, entry)
		}
	}

	// Join entries back together
	newLog := strings.Join(newEntries, "\n")

	// Update the log column
	now := time.Now().UTC().Format(time.RFC3339)
	userID := c.Locals("userID").(string)
	updateQuery := fmt.Sprintf("UPDATE %s SET %s = ?, modified_at = ?, modified_by_id = ? WHERE id = ? AND org_id = ?",
		tableName, quoteIdentifier(logColumn))
	_, err = h.getDB(c).ExecContext(c.Context(), updateQuery, newLog, now, userID, recordID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"logEntries": len(newEntries),
	})
}

// RegisterRoutes registers generic entity routes
func (h *GenericEntityHandler) RegisterRoutes(app fiber.Router) {
	// Public metadata endpoints (read-only, for display purposes)
	app.Get("/entities/:entity/fields", h.ListFields)
	app.Get("/entities/:entity/def", h.GetEntityDef)
	app.Get("/entities/:entity/layouts/:type", h.GetLayout)

	// Dynamic entity routes - these should be registered last to avoid conflicts
	app.Get("/entities/:entity/records", h.List)
	app.Get("/entities/:entity/records/:id", h.Get)
	app.Post("/entities/:entity/records", h.Create)
	app.Post("/entities/:entity/records/upsert", h.Upsert)
	app.Put("/entities/:entity/records/:id", h.Update)
	app.Patch("/entities/:entity/records/:id", h.Update)
	app.Delete("/entities/:entity/records/:id", h.Delete)

	// Stream field operations
	app.Delete("/entities/:entity/records/:id/stream/:fieldName", h.DeleteStreamEntry)
}
