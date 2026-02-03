package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	rollupTimeout       = 2 * time.Second
	rollupBatchTimeout  = 5 * time.Second
	placeholderID       = "{{id}}"
	placeholderIDs      = "{{ids}}"
)

// sensitiveTablesRollup contains tables that rollup queries must NEVER access
// These contain authentication, authorization, and system data
var sensitiveTablesRollup = []string{
	"users",
	"sessions",
	"api_tokens",
	"invitations",
	"password_reset_tokens",
	"org_webhook_settings",
	"sqlite_master",
	"sqlite_schema",
	"sqlite_sequence",
	"sqlite_temp_master",
}

// RollupService handles rollup field validation and execution
type RollupService struct {
	db *sql.DB
}

// NewRollupService creates a new RollupService
func NewRollupService(db *sql.DB) *RollupService {
	return &RollupService{db: db}
}

// ValidateRollupQuery checks if the SQL is safe for rollup execution
// SECURITY: This blocks access to sensitive tables and dangerous operations
// Note: org_id filtering is automatically injected at execution time
func (s *RollupService) ValidateRollupQuery(query string) error {
	if query == "" {
		return fmt.Errorf("rollup query cannot be empty")
	}

	// Remove SQL comments before validation (prevents comment-based bypasses)
	commentRegex := regexp.MustCompile(`(?m)--.*$|/\*[\s\S]*?\*/`)
	normalized := commentRegex.ReplaceAllString(query, " ")
	normalized = strings.TrimSpace(normalized)
	upperQuery := strings.ToUpper(normalized)

	// Must be SELECT query (no CTEs for simplicity - they complicate org_id injection)
	if !strings.HasPrefix(upperQuery, "SELECT") {
		return fmt.Errorf("rollup query must be a SELECT statement")
	}

	// Block CTEs (WITH clauses) - they complicate automatic org_id injection
	if strings.Contains(upperQuery, "WITH ") && strings.Contains(upperQuery, " AS ") {
		return fmt.Errorf("rollup query cannot use WITH clauses (CTEs). Use simple SELECT statements")
	}

	// Block subqueries in FROM clause - they complicate org_id injection
	fromIdx := strings.Index(upperQuery, "FROM")
	if fromIdx > 0 {
		afterFrom := upperQuery[fromIdx:]
		// Check for SELECT in FROM clause (subquery)
		if strings.Contains(afterFrom, "(SELECT") || strings.Contains(afterFrom, "( SELECT") {
			return fmt.Errorf("rollup query cannot use subqueries in FROM clause")
		}
	}

	// Block dangerous keywords (case-insensitive, handles spaces/newlines)
	dangerous := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "CREATE",
		"ALTER", "TRUNCATE", "REPLACE", "ATTACH", "DETACH",
		"PRAGMA", "VACUUM", "REINDEX", "GRANT", "REVOKE",
	}
	// Use word boundary matching to avoid false positives
	for _, kw := range dangerous {
		pattern := regexp.MustCompile(`(?i)\b` + kw + `\b`)
		if pattern.MatchString(normalized) {
			return fmt.Errorf("rollup query contains forbidden keyword: %s", kw)
		}
	}

	// SECURITY: Block access to sensitive system tables
	for _, table := range sensitiveTablesRollup {
		// Match table name with word boundaries to catch FROM users, JOIN users, etc.
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(table) + `\b`)
		if pattern.MatchString(normalized) {
			return fmt.Errorf("rollup query cannot access system table: %s", table)
		}
	}

	// Validate we can extract at least one table name for org_id injection
	tables := extractTableNames(query)
	if len(tables) == 0 {
		return fmt.Errorf("rollup query must reference at least one table")
	}

	return nil
}

// extractTableNames extracts table names from FROM and JOIN clauses
func extractTableNames(query string) []string {
	var tables []string

	// Match FROM table_name and JOIN table_name patterns
	// Handles: FROM Contact, FROM Contact c, LEFT JOIN Task t, etc.
	tablePattern := regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	matches := tablePattern.FindAllStringSubmatch(query, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			tableName := match[1]
			// Skip SQL keywords that might be matched
			upperTable := strings.ToUpper(tableName)
			if upperTable == "SELECT" || upperTable == "WHERE" || upperTable == "AND" || upperTable == "OR" {
				continue
			}
			if !seen[tableName] {
				seen[tableName] = true
				tables = append(tables, tableName)
			}
		}
	}

	// Also check for table aliases and ensure we capture the actual table name
	// Pattern: FROM TableName alias or FROM TableName AS alias
	aliasPattern := regexp.MustCompile(`(?i)\bFROM\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+(?:AS\s+)?([a-zA-Z_][a-zA-Z0-9_]*)?\s*(?:WHERE|LEFT|RIGHT|INNER|OUTER|JOIN|$)`)
	aliasMatches := aliasPattern.FindAllStringSubmatch(query+" ", -1)
	for _, match := range aliasMatches {
		if len(match) > 1 && match[1] != "" {
			tableName := match[1]
			upperTable := strings.ToUpper(tableName)
			if upperTable != "SELECT" && upperTable != "WHERE" && !seen[tableName] {
				seen[tableName] = true
				tables = append(tables, tableName)
			}
		}
	}

	return tables
}

// injectOrgFilter automatically adds org_id filtering to a query
// This ensures multi-tenant data isolation without requiring users to add it manually
func injectOrgFilter(query string) string {
	upperQuery := strings.ToUpper(query)

	// Find the main table from the FROM clause
	tablePattern := regexp.MustCompile(`(?i)\bFROM\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	match := tablePattern.FindStringSubmatch(query)
	if len(match) < 2 {
		// Can't find table, return original (validation should have caught this)
		return query
	}
	mainTable := match[1]

	// Check if query already has a WHERE clause
	whereIdx := strings.Index(upperQuery, " WHERE ")

	if whereIdx > 0 {
		// Has WHERE clause - inject org_id condition after WHERE
		// Find the position right after "WHERE "
		insertPos := whereIdx + 7 // len(" WHERE ")
		return query[:insertPos] + mainTable + ".org_id = ? AND " + query[insertPos:]
	}

	// No WHERE clause - need to add one
	// Find where to insert (before GROUP BY, ORDER BY, LIMIT, or at end)
	insertKeywords := []string{" GROUP BY", " ORDER BY", " LIMIT", " HAVING"}
	insertPos := len(query)

	for _, kw := range insertKeywords {
		idx := strings.Index(upperQuery, kw)
		if idx > 0 && idx < insertPos {
			insertPos = idx
		}
	}

	return query[:insertPos] + " WHERE " + mainTable + ".org_id = ?" + query[insertPos:]
}

// ExecuteRollup runs the rollup query for a specific record ID within an organization
// SECURITY: Automatically injects org_id filtering to enforce multi-tenant data isolation
func (s *RollupService) ExecuteRollup(ctx context.Context, query string, recordID string, orgID string, resultType string) (interface{}, error) {
	// SECURITY: Validate orgID is provided
	if orgID == "" {
		return nil, fmt.Errorf("orgID is required for rollup execution")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, rollupTimeout)
	defer cancel()

	// SECURITY: Automatically inject org_id filter into the query
	securedQuery := injectOrgFilter(query)

	// Build the query with parameterized placeholders
	// org_id is always first parameter (injected by injectOrgFilter)
	preparedQuery := securedQuery
	params := []interface{}{orgID} // org_id is always first

	// Replace {{id}} for record-specific queries
	if strings.Contains(preparedQuery, placeholderID) {
		preparedQuery = strings.ReplaceAll(preparedQuery, placeholderID, "?")
		params = append(params, recordID)
	}

	// Execute the query with all parameters
	row := s.db.QueryRowContext(ctx, preparedQuery, params...)

	var result interface{}
	if resultType == "text" {
		var textVal sql.NullString
		if err := row.Scan(&textVal); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, fmt.Errorf("rollup query execution failed: %w", err)
		}
		if textVal.Valid {
			result = textVal.String
		}
	} else {
		// Default to numeric
		var numVal sql.NullFloat64
		if err := row.Scan(&numVal); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, fmt.Errorf("rollup query execution failed: %w", err)
		}
		if numVal.Valid {
			result = numVal.Float64
		}
	}

	return result, nil
}

// ExecuteRollupBatch executes rollup queries efficiently for multiple records
// It automatically detects whether to use batching (if {{id}} present) or single execution
// Returns a map of recordID -> result
func (s *RollupService) ExecuteRollupBatch(ctx context.Context, query string, recordIDs []string, orgID string, resultType string) (map[string]interface{}, error) {
	if orgID == "" {
		return nil, fmt.Errorf("orgID is required for rollup execution")
	}
	if len(recordIDs) == 0 {
		return make(map[string]interface{}), nil
	}

	// Check if query uses {{id}} placeholder (record-specific) or not (static)
	if strings.Contains(query, placeholderID) {
		return s.executeBatchQuery(ctx, query, recordIDs, orgID, resultType)
	}
	return s.executeStaticQuery(ctx, query, recordIDs, orgID, resultType)
}

// executeBatchQuery transforms a per-record query into a batched query
// Example: "SELECT COUNT(*) FROM Contact WHERE account_id = {{id}}"
// Becomes: "SELECT account_id, COUNT(*) FROM Contact WHERE account_id IN (?,?,?) GROUP BY account_id"
func (s *RollupService) executeBatchQuery(ctx context.Context, query string, recordIDs []string, orgID string, resultType string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, rollupBatchTimeout)
	defer cancel()

	// Extract the column name used with {{id}}
	groupCol, err := extractIDColumn(query)
	if err != nil {
		// Can't batch this query, fall back to per-record execution
		return s.fallbackPerRecord(ctx, query, recordIDs, orgID, resultType)
	}

	// Transform the query to batch format
	batchQuery, err := transformToBatchQuery(query, groupCol)
	if err != nil {
		return s.fallbackPerRecord(ctx, query, recordIDs, orgID, resultType)
	}

	// Inject org_id filter and replace {{ids}} with placeholders
	securedQuery := injectOrgFilterBatch(batchQuery, len(recordIDs))

	// Build parameter list: org_id first, then all record IDs
	params := make([]interface{}, 0, len(recordIDs)+1)
	params = append(params, orgID)
	for _, id := range recordIDs {
		params = append(params, id)
	}

	// Execute batch query
	rows, err := s.db.QueryContext(ctx, securedQuery, params...)
	if err != nil {
		return nil, fmt.Errorf("batch rollup query failed: %w", err)
	}
	defer rows.Close()

	// Map results back to record IDs
	results := make(map[string]interface{})
	for rows.Next() {
		var recordID string
		var value interface{}

		if resultType == "text" {
			var textVal sql.NullString
			if err := rows.Scan(&recordID, &textVal); err != nil {
				return nil, fmt.Errorf("failed to scan batch result: %w", err)
			}
			if textVal.Valid {
				value = textVal.String
			}
		} else {
			var numVal sql.NullFloat64
			if err := rows.Scan(&recordID, &numVal); err != nil {
				return nil, fmt.Errorf("failed to scan batch result: %w", err)
			}
			if numVal.Valid {
				value = numVal.Float64
			}
		}
		results[recordID] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("batch rollup iteration failed: %w", err)
	}

	// Records not in results get nil (no matching data)
	for _, id := range recordIDs {
		if _, exists := results[id]; !exists {
			results[id] = nil
		}
	}

	return results, nil
}

// executeStaticQuery runs a query without {{id}} once and applies result to all records
func (s *RollupService) executeStaticQuery(ctx context.Context, query string, recordIDs []string, orgID string, resultType string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, rollupTimeout)
	defer cancel()

	// Inject org_id filter
	securedQuery := injectOrgFilter(query)

	// Execute once
	row := s.db.QueryRowContext(ctx, securedQuery, orgID)

	var result interface{}
	if resultType == "text" {
		var textVal sql.NullString
		if err := row.Scan(&textVal); err != nil {
			if err == sql.ErrNoRows {
				result = nil
			} else {
				return nil, fmt.Errorf("static rollup query failed: %w", err)
			}
		} else if textVal.Valid {
			result = textVal.String
		}
	} else {
		var numVal sql.NullFloat64
		if err := row.Scan(&numVal); err != nil {
			if err == sql.ErrNoRows {
				result = nil
			} else {
				return nil, fmt.Errorf("static rollup query failed: %w", err)
			}
		} else if numVal.Valid {
			result = numVal.Float64
		}
	}

	// Apply same result to all records
	results := make(map[string]interface{})
	for _, id := range recordIDs {
		results[id] = result
	}
	return results, nil
}

// fallbackPerRecord executes queries one by one when batching isn't possible
func (s *RollupService) fallbackPerRecord(ctx context.Context, query string, recordIDs []string, orgID string, resultType string) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	for _, recordID := range recordIDs {
		result, err := s.ExecuteRollup(ctx, query, recordID, orgID, resultType)
		if err != nil {
			results[recordID] = nil
		} else {
			results[recordID] = result
		}
	}
	return results, nil
}

// extractIDColumn finds the column name used with {{id}} placeholder
// Example: "WHERE account_id = {{id}}" returns "account_id"
func extractIDColumn(query string) (string, error) {
	// Match patterns like: column_name = {{id}}, column_name={{id}}, column_name =  {{id}}
	pattern := regexp.MustCompile(`(?i)([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*\{\{id\}\}`)
	match := pattern.FindStringSubmatch(query)
	if len(match) < 2 {
		return "", fmt.Errorf("could not find column used with {{id}}")
	}
	return match[1], nil
}

// transformToBatchQuery converts a per-record query to a batch query
// Input:  "SELECT COUNT(*) FROM Contact WHERE account_id = {{id}}"
// Output: "SELECT account_id, COUNT(*) FROM Contact WHERE account_id IN (?,?,?...) GROUP BY account_id"
func transformToBatchQuery(query string, groupCol string) (string, error) {
	upperQuery := strings.ToUpper(query)

	// 1. Add groupCol to SELECT clause (right after SELECT)
	selectIdx := strings.Index(upperQuery, "SELECT")
	if selectIdx < 0 {
		return "", fmt.Errorf("no SELECT found in query")
	}
	insertPos := selectIdx + 6 // len("SELECT")

	// Skip whitespace after SELECT
	for insertPos < len(query) && (query[insertPos] == ' ' || query[insertPos] == '\t' || query[insertPos] == '\n') {
		insertPos++
	}

	// Insert "groupCol, " after SELECT
	newQuery := query[:insertPos] + groupCol + ", " + query[insertPos:]

	// 2. Replace "column = {{id}}" with "column IN ({{ids}})"
	// Use case-insensitive replacement
	idPattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(groupCol) + `\s*=\s*\{\{id\}\}`)
	newQuery = idPattern.ReplaceAllString(newQuery, groupCol+" IN ({{ids}})")

	// 3. Add GROUP BY clause if not present
	upperNew := strings.ToUpper(newQuery)
	if !strings.Contains(upperNew, "GROUP BY") {
		// Find where to insert (before ORDER BY, LIMIT, or at end)
		insertKeywords := []string{" ORDER BY", " LIMIT"}
		groupByPos := len(newQuery)

		for _, kw := range insertKeywords {
			idx := strings.Index(upperNew, kw)
			if idx > 0 && idx < groupByPos {
				groupByPos = idx
			}
		}
		newQuery = newQuery[:groupByPos] + " GROUP BY " + groupCol + newQuery[groupByPos:]
	}

	return newQuery, nil
}

// injectOrgFilterBatch is like injectOrgFilter but handles the {{ids}} placeholder
func injectOrgFilterBatch(query string, numIDs int) string {
	// First inject org_id filter using existing function
	filtered := injectOrgFilter(query)

	// Then replace {{ids}} with the right number of placeholders
	placeholders := make([]string, numIDs)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.ReplaceAll(filtered, placeholderIDs, strings.Join(placeholders, ","))
}
