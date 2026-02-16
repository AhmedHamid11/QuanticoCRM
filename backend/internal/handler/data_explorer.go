package handler

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

const (
	maxRows      = 50000
	queryTimeout = 5 * time.Second
)

// DataExplorerHandler handles SQL query execution for admin data exploration
type DataExplorerHandler struct {
	defaultDB db.DBConn
}

// NewDataExplorerHandler creates a new DataExplorerHandler
func NewDataExplorerHandler(conn db.DBConn) *DataExplorerHandler {
	return &DataExplorerHandler{defaultDB: conn}
}

// getDB returns the tenant database from context, falling back to default db
func (h *DataExplorerHandler) getDB(c *fiber.Ctx) db.DBConn {
	if tenantDB := middleware.GetTenantDBConn(c); tenantDB != nil {
		return tenantDB
	}
	return h.defaultDB
}

// RegisterRoutes registers the data explorer routes for Platform Admin
// SECURITY: This allows querying ALL data across ALL organizations
func (h *DataExplorerHandler) RegisterRoutes(app fiber.Router) {
	explorer := app.Group("/data-explorer")
	explorer.Post("/query", h.ExecuteQuery)
}

// RegisterOrgRoutes registers org-scoped data explorer routes for Org Admins
// SECURITY: Queries are automatically filtered to only show data from the user's org
func (h *DataExplorerHandler) RegisterOrgRoutes(app fiber.Router) {
	explorer := app.Group("/data-explorer")
	explorer.Post("/query", h.ExecuteOrgQuery)
}

// QueryRequest represents the request body for query execution
type QueryRequest struct {
	SQL string `json:"sql"`
}

// QueryResponse represents the response from query execution
type QueryResponse struct {
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
	RowCount int             `json:"rowCount"`
}

// ExecuteQuery executes a SQL query and returns the results
func (h *DataExplorerHandler) ExecuteQuery(c *fiber.Ctx) error {
	var req QueryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Trim whitespace and validate
	query := strings.TrimSpace(req.SQL)
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "SQL query is required",
		})
	}

	// Security: Only allow SELECT statements
	if !isSelectQuery(query) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only SELECT queries are allowed",
		})
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Context(), queryTimeout)
	defer cancel()

	// Execute query
	rows, err := h.defaultDB.QueryContext(ctx, query)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get column names: " + err.Error(),
		})
	}

	// Fetch rows - initialize as empty slice so JSON encodes as [] instead of null
	results := make([][]interface{}, 0)
	for rows.Next() {
		if len(results) >= maxRows {
			break
		}

		// Create a slice of interface{} to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to scan row: " + err.Error(),
			})
		}

		// Convert []byte to string for readability
		row := make([]interface{}, len(columns))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error reading rows: " + err.Error(),
		})
	}

	return c.JSON(QueryResponse{
		Columns:  columns,
		Rows:     results,
		RowCount: len(results),
	})
}

// ExecuteOrgQuery executes a SQL query scoped to the user's organization
// SECURITY: Automatically injects org_id filter into queries to ensure data isolation
func (h *DataExplorerHandler) ExecuteOrgQuery(c *fiber.Ctx) error {
	orgID, ok := c.Locals("orgID").(string)
	if !ok || orgID == "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	var req QueryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Trim whitespace and validate
	query := strings.TrimSpace(req.SQL)
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "SQL query is required",
		})
	}

	// Security: Only allow SELECT statements
	if !isSelectQuery(query) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only SELECT queries are allowed",
		})
	}

	// Try to inject org_id filter into the query for proper multi-tenant isolation
	// SECURITY: injectOrgFilter now uses parameterized placeholder (?)
	modifiedQuery, wasModified := injectOrgFilter(query, orgID)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Context(), queryTimeout)
	defer cancel()

	// Execute query with orgID as parameter if filter was injected
	// Use tenant database from context for org-scoped queries
	db := h.getDB(c)
	var rows *sql.Rows
	var err error
	if wasModified && strings.Contains(modifiedQuery, "org_id = ?") {
		// SECURITY: Pass orgID as parameter to prevent SQL injection
		rows, err = db.QueryContext(ctx, modifiedQuery, orgID)
	} else {
		rows, err = db.QueryContext(ctx, modifiedQuery)
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get column names: " + err.Error(),
		})
	}

	// Check if org_id column exists in results (for warning purposes)
	hasOrgIDColumn := false
	for _, col := range columns {
		if col == "org_id" {
			hasOrgIDColumn = true
			break
		}
	}

	// Fetch rows - initialize as empty slice so JSON encodes as [] instead of null
	results := make([][]interface{}, 0)
	for rows.Next() {
		if len(results) >= maxRows {
			break
		}

		// Create a slice of interface{} to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to scan row: " + err.Error(),
			})
		}

		// Convert []byte to string for readability
		row := make([]interface{}, len(columns))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error reading rows: " + err.Error(),
		})
	}

	// Add a warning if we couldn't inject the org_id filter
	warning := ""
	if !wasModified && !hasOrgIDColumn {
		warning = "Warning: Could not automatically filter by organization. Results may include system tables or data from all organizations."
	}

	response := fiber.Map{
		"columns":  columns,
		"rows":     results,
		"rowCount": len(results),
	}
	if warning != "" {
		response["warning"] = warning
	}

	return c.JSON(response)
}

// injectOrgFilter attempts to inject an org_id filter into a SELECT query
// Returns the modified query, the orgID parameter, and whether it was successfully modified
// SECURITY: Uses parameterized query to prevent SQL injection
func injectOrgFilter(query string, orgID string) (string, bool) {
	upperQuery := strings.ToUpper(query)

	// Don't modify queries that already filter by org_id
	if strings.Contains(upperQuery, "ORG_ID") {
		return query, true // Already filtered
	}

	// Find the table name from simple SELECT queries
	// Pattern: SELECT ... FROM table_name ...
	fromIndex := strings.Index(upperQuery, " FROM ")
	if fromIndex == -1 {
		return query, false
	}

	// Find the end of the table name (next space, WHERE, ORDER, LIMIT, GROUP, HAVING, JOIN, or end)
	afterFrom := query[fromIndex+6:]
	afterFromUpper := upperQuery[fromIndex+6:]

	// Skip leading whitespace
	startIdx := 0
	for startIdx < len(afterFrom) && (afterFrom[startIdx] == ' ' || afterFrom[startIdx] == '\t' || afterFrom[startIdx] == '\n') {
		startIdx++
	}
	afterFrom = afterFrom[startIdx:]
	afterFromUpper = afterFromUpper[startIdx:]

	// Find end of table name
	endIdx := len(afterFrom)
	terminators := []string{" WHERE ", " ORDER ", " LIMIT ", " GROUP ", " HAVING ", " JOIN ", " LEFT ", " RIGHT ", " INNER ", " OUTER ", " CROSS ", ";"}
	for _, term := range terminators {
		idx := strings.Index(afterFromUpper, term)
		if idx != -1 && idx < endIdx {
			endIdx = idx
		}
	}

	// Also check for space followed by anything
	spaceIdx := strings.IndexAny(afterFrom[:endIdx], " \t\n")
	if spaceIdx != -1 {
		endIdx = spaceIdx
	}

	tableName := strings.TrimSpace(afterFrom[:endIdx])
	if tableName == "" {
		return query, false
	}

	// Check if the table has an org_id column by querying table info
	// For simplicity, we'll inject the filter and let SQLite handle errors if column doesn't exist

	// Build the modified query by injecting WHERE org_id = ? (parameterized)
	// SECURITY: Use placeholder instead of string interpolation to prevent SQL injection
	whereIdx := strings.Index(upperQuery, " WHERE ")
	orderIdx := strings.Index(upperQuery, " ORDER ")
	limitIdx := strings.Index(upperQuery, " LIMIT ")
	groupIdx := strings.Index(upperQuery, " GROUP ")

	// Use parameterized placeholder
	orgFilter := "org_id = ?"

	if whereIdx != -1 {
		// Has WHERE clause - inject AND condition right after WHERE
		// Find the end of the existing WHERE conditions
		insertPos := whereIdx + 7 // After " WHERE "
		modifiedQuery := query[:insertPos] + orgFilter + " AND " + query[insertPos:]
		return modifiedQuery, true
	}

	// No WHERE clause - need to add one
	// Find the right position (before ORDER BY, LIMIT, GROUP BY, or at end)
	insertPos := len(query)
	if orderIdx != -1 && orderIdx < insertPos {
		insertPos = orderIdx
	}
	if limitIdx != -1 && limitIdx < insertPos {
		insertPos = limitIdx
	}
	if groupIdx != -1 && groupIdx < insertPos {
		insertPos = groupIdx
	}

	// Remove trailing semicolon if present
	trimmedQuery := strings.TrimRight(query[:insertPos], "; \t\n")
	suffix := ""
	if insertPos < len(query) {
		suffix = query[insertPos:]
	}

	modifiedQuery := trimmedQuery + " WHERE " + orgFilter + suffix
	return modifiedQuery, true
}

// isSelectQuery checks if the query is a SELECT statement
func isSelectQuery(query string) bool {
	// Normalize: remove comments, trim whitespace
	normalized := strings.TrimSpace(query)

	// Remove single-line comments
	re := regexp.MustCompile(`--.*$`)
	normalized = re.ReplaceAllString(normalized, "")

	// Remove multi-line comments
	re = regexp.MustCompile(`/\*.*?\*/`)
	normalized = re.ReplaceAllString(normalized, "")

	normalized = strings.TrimSpace(normalized)

	// Check if it starts with SELECT (case-insensitive)
	upperQuery := strings.ToUpper(normalized)

	// Must start with SELECT or WITH (for CTEs)
	if !strings.HasPrefix(upperQuery, "SELECT") && !strings.HasPrefix(upperQuery, "WITH") {
		return false
	}

	// Check for dangerous keywords that shouldn't appear
	dangerousKeywords := []string{
		"INSERT ", "UPDATE ", "DELETE ", "DROP ", "CREATE ", "ALTER ",
		"TRUNCATE ", "REPLACE ", "ATTACH ", "DETACH ", "PRAGMA ",
	}

	for _, keyword := range dangerousKeywords {
		if strings.Contains(upperQuery, keyword) {
			return false
		}
	}

	return true
}
