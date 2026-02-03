package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/flow"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/google/uuid"
)

// FlowEntityService adapts the generic entity handler for use by the flow engine
type FlowEntityService struct {
	db           *sql.DB
	metadataRepo *repo.MetadataRepo
}

// NewFlowEntityService creates a new FlowEntityService
func NewFlowEntityService(db *sql.DB, metadataRepo *repo.MetadataRepo) *FlowEntityService {
	return &FlowEntityService{
		db:           db,
		metadataRepo: metadataRepo,
	}
}

// Ensure FlowEntityService implements flow.EntityService
var _ flow.EntityService = (*FlowEntityService)(nil)

// Create creates a new record in the specified entity
func (s *FlowEntityService) Create(ctx context.Context, entityType, orgID string, data map[string]interface{}) (map[string]interface{}, error) {
	tableName := s.getTableName(entityType)

	// Get entity fields from metadata
	fields, err := s.metadataRepo.ListFields(ctx, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity fields: %w", err)
	}

	// Build field name map for validation
	validFields := make(map[string]bool)
	for _, f := range fields {
		validFields[f.Name] = true
	}

	// Always valid fields (system fields)
	systemFields := []string{"id", "org_id", "created_at", "modified_at", "created_by", "modified_by", "is_deleted"}
	for _, f := range systemFields {
		validFields[f] = true
	}

	// Generate ID and set system fields
	id := uuid.NewString()
	now := time.Now().Format(time.RFC3339)

	// Build INSERT statement
	var columns []string
	var placeholders []string
	var values []interface{}

	columns = append(columns, "id", "org_id", "created_at", "modified_at")
	placeholders = append(placeholders, "?", "?", "?", "?")
	values = append(values, id, orgID, now, now)

	// Add data fields
	for key, value := range data {
		// Convert snake_case key if needed
		dbKey := toSnakeCase(key)
		if !validFields[dbKey] && !validFields[key] {
			continue // Skip unknown fields
		}

		columns = append(columns, dbKey)
		placeholders = append(placeholders, "?")
		values = append(values, value)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err = s.db.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	// Return the created record
	data["id"] = id
	data["org_id"] = orgID
	data["created_at"] = now
	data["modified_at"] = now

	return data, nil
}

// Update updates an existing record
func (s *FlowEntityService) Update(ctx context.Context, entityType, orgID, recordID string, data map[string]interface{}) (map[string]interface{}, error) {
	tableName := s.getTableName(entityType)

	// Get entity fields from metadata
	fields, err := s.metadataRepo.ListFields(ctx, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity fields: %w", err)
	}

	// Build field name map for validation
	validFields := make(map[string]bool)
	for _, f := range fields {
		validFields[f.Name] = true
	}

	// Debug logging
	fmt.Printf("[FlowEntityService.Update] entity=%s, orgID=%s, recordID=%s\n", entityType, orgID, recordID)
	fmt.Printf("[FlowEntityService.Update] data=%+v\n", data)
	fmt.Printf("[FlowEntityService.Update] validFields=%+v\n", validFields)

	now := time.Now().Format(time.RFC3339)

	// Build UPDATE statement
	var setClauses []string
	var values []interface{}

	setClauses = append(setClauses, "modified_at = ?")
	values = append(values, now)

	for key, value := range data {
		dbKey := toSnakeCase(key)
		if !validFields[dbKey] && !validFields[key] {
			fmt.Printf("[FlowEntityService.Update] SKIPPING field: key=%s, dbKey=%s\n", key, dbKey)
			continue
		}

		fmt.Printf("[FlowEntityService.Update] ADDING field: key=%s, dbKey=%s, value=%v\n", key, dbKey, value)
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", dbKey))
		values = append(values, value)
	}

	values = append(values, recordID, orgID)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE id = ? AND org_id = ?",
		tableName,
		strings.Join(setClauses, ", "),
	)

	fmt.Printf("[FlowEntityService.Update] query=%s\n", query)
	fmt.Printf("[FlowEntityService.Update] values=%+v\n", values)

	result, err := s.db.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("failed to update record: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("record not found: %s", recordID)
	}

	data["id"] = recordID
	data["modified_at"] = now

	return data, nil
}

// Get retrieves a single record by ID
func (s *FlowEntityService) Get(ctx context.Context, entityType, orgID, recordID string) (map[string]interface{}, error) {
	tableName := s.getTableName(entityType)

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = ? AND org_id = ?", tableName)

	rows, err := s.db.QueryContext(ctx, query, recordID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query record: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	if !rows.Next() {
		return nil, fmt.Errorf("record not found: %s", recordID)
	}

	// Create interface slice for scanning
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("failed to scan record: %w", err)
	}

	// Build result map
	result := make(map[string]interface{})
	for i, col := range columns {
		result[col] = values[i]
	}

	return result, nil
}

// Delete soft-deletes a record
func (s *FlowEntityService) Delete(ctx context.Context, entityType, orgID, recordID string) error {
	tableName := s.getTableName(entityType)

	query := fmt.Sprintf(
		"UPDATE %s SET is_deleted = 1, modified_at = ? WHERE id = ? AND org_id = ?",
		tableName,
	)

	now := time.Now().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, query, now, recordID, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("record not found: %s", recordID)
	}

	return nil
}

// getTableName returns the database table name for an entity type
func (s *FlowEntityService) getTableName(entityType string) string {
	// Convert entity type to table name (lowercase, pluralized)
	name := strings.ToLower(entityType)
	if !strings.HasSuffix(name, "s") {
		name += "s"
	}
	return name
}

// toSnakeCase converts a camelCase string to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
