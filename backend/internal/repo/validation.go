package repo

import (
	"github.com/fastcrm/backend/internal/db"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// ValidationRepo handles database operations for validation rules
type ValidationRepo struct {
	db db.DBConn
}

// NewValidationRepo creates a new ValidationRepo
func NewValidationRepo(conn db.DBConn) *ValidationRepo {
	return &ValidationRepo{db: conn}
}

// WithDB returns a new ValidationRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *ValidationRepo) WithDB(conn db.DBConn) *ValidationRepo {
	if conn == nil {
		return r
	}
	return &ValidationRepo{db: conn}
}

// DB returns the current database connection
func (r *ValidationRepo) DB() db.DBConn {
	return r.db
}

// Create inserts a new validation rule into the database
func (r *ValidationRepo) Create(ctx context.Context, orgID string, input entity.ValidationRuleCreateInput, userID string) (*entity.ValidationRule, error) {
	// Serialize conditions to JSON
	conditionsJSON, err := json.Marshal(input.Conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}

	// Serialize actions to JSON
	actionsJSON, err := json.Marshal(input.Actions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal actions: %w", err)
	}

	// Set defaults
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	triggerOnCreate := false
	if input.TriggerOnCreate != nil {
		triggerOnCreate = *input.TriggerOnCreate
	}

	triggerOnUpdate := true
	if input.TriggerOnUpdate != nil {
		triggerOnUpdate = *input.TriggerOnUpdate
	}

	triggerOnDelete := false
	if input.TriggerOnDelete != nil {
		triggerOnDelete = *input.TriggerOnDelete
	}

	conditionLogic := "AND"
	if input.ConditionLogic != "" {
		conditionLogic = strings.ToUpper(input.ConditionLogic)
	}

	priority := 100
	if input.Priority != nil {
		priority = *input.Priority
	}

	rule := &entity.ValidationRule{
		ID:              sfid.NewValidationRule(),
		OrgID:           orgID,
		Name:            input.Name,
		Description:     input.Description,
		EntityType:      input.EntityType,
		Enabled:         enabled,
		TriggerOnCreate: triggerOnCreate,
		TriggerOnUpdate: triggerOnUpdate,
		TriggerOnDelete: triggerOnDelete,
		ConditionLogic:  conditionLogic,
		Conditions:      input.Conditions,
		ConditionsJSON:  string(conditionsJSON),
		Actions:         input.Actions,
		ActionsJSON:     string(actionsJSON),
		ErrorMessage:    input.ErrorMessage,
		Priority:        priority,
		CreatedAt:       time.Now().UTC(),
		ModifiedAt:      time.Now().UTC(),
		CreatedBy:       &userID,
		ModifiedBy:      &userID,
	}

	query := `
		INSERT INTO validation_rules (
			id, org_id, name, description, entity_type,
			enabled, trigger_on_create, trigger_on_update, trigger_on_delete,
			condition_logic, conditions, actions, error_message, priority,
			created_at, modified_at, created_by, modified_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		rule.ID, rule.OrgID, rule.Name, rule.Description, rule.EntityType,
		rule.Enabled, rule.TriggerOnCreate, rule.TriggerOnUpdate, rule.TriggerOnDelete,
		rule.ConditionLogic, rule.ConditionsJSON, rule.ActionsJSON, rule.ErrorMessage, rule.Priority,
		rule.CreatedAt.Format(time.RFC3339), rule.ModifiedAt.Format(time.RFC3339),
		rule.CreatedBy, rule.ModifiedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create validation rule: %w", err)
	}

	return rule, nil
}

// GetByID retrieves a validation rule by its ID
func (r *ValidationRepo) GetByID(ctx context.Context, orgID, id string) (*entity.ValidationRule, error) {
	query := `
		SELECT id, org_id, name, description, entity_type,
			enabled, trigger_on_create, trigger_on_update, trigger_on_delete,
			condition_logic, conditions, actions, error_message, priority,
			created_at, modified_at, created_by, modified_by
		FROM validation_rules
		WHERE id = ? AND org_id = ?
	`

	var rule entity.ValidationRule
	var createdAt, modifiedAt, conditionsJSON, actionsJSON string
	var description, errorMessage, createdBy, modifiedBy sql.NullString

	err := r.db.QueryRowContext(ctx, query, id, orgID).Scan(
		&rule.ID, &rule.OrgID, &rule.Name, &description, &rule.EntityType,
		&rule.Enabled, &rule.TriggerOnCreate, &rule.TriggerOnUpdate, &rule.TriggerOnDelete,
		&rule.ConditionLogic, &conditionsJSON, &actionsJSON, &errorMessage, &rule.Priority,
		&createdAt, &modifiedAt, &createdBy, &modifiedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get validation rule: %w", err)
	}

	rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rule.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	if description.Valid {
		rule.Description = &description.String
	}
	if errorMessage.Valid {
		rule.ErrorMessage = &errorMessage.String
	}
	if createdBy.Valid {
		rule.CreatedBy = &createdBy.String
	}
	if modifiedBy.Valid {
		rule.ModifiedBy = &modifiedBy.String
	}

	// Parse conditions JSON
	if err := json.Unmarshal([]byte(conditionsJSON), &rule.Conditions); err != nil {
		rule.Conditions = []entity.ValidationCondition{}
	}

	// Parse actions JSON
	if err := json.Unmarshal([]byte(actionsJSON), &rule.Actions); err != nil {
		rule.Actions = []entity.ValidationAction{}
	}

	return &rule, nil
}

// ListByOrg retrieves all validation rules for an organization with pagination
func (r *ValidationRepo) ListByOrg(ctx context.Context, orgID string, params entity.ValidationRuleListParams) (*entity.ValidationRuleListResponse, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}
	if params.SortBy == "" {
		params.SortBy = "priority"
	}
	if params.SortDir == "" {
		params.SortDir = "asc"
	}

	// Validate sort column
	validSortCols := map[string]bool{
		"created_at": true, "modified_at": true, "name": true,
		"entity_type": true, "enabled": true, "priority": true,
	}
	if !validSortCols[params.SortBy] {
		params.SortBy = "priority"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		params.SortDir = "asc"
	}

	// Build query with filters
	baseQuery := `FROM validation_rules WHERE org_id = ?`
	args := []any{orgID}

	// Search filter
	if params.Search != "" {
		baseQuery += ` AND (name LIKE ? OR description LIKE ?)`
		searchTerm := "%" + params.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	// Entity type filter
	if params.EntityType != "" {
		baseQuery += ` AND entity_type = ?`
		args = append(args, params.EntityType)
	}

	// Enabled filter
	if params.Enabled != nil {
		baseQuery += ` AND enabled = ?`
		args = append(args, *params.Enabled)
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count validation rules: %w", err)
	}

	// Query with pagination
	offset := (params.Page - 1) * params.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT id, org_id, name, description, entity_type,
			enabled, trigger_on_create, trigger_on_update, trigger_on_delete,
			condition_logic, conditions, actions, error_message, priority,
			created_at, modified_at, created_by, modified_by
		%s ORDER BY %s %s LIMIT ? OFFSET ?
	`, baseQuery, params.SortBy, strings.ToUpper(params.SortDir))

	args = append(args, params.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list validation rules: %w", err)
	}
	defer rows.Close()

	var rules []entity.ValidationRule
	for rows.Next() {
		var rule entity.ValidationRule
		var createdAt, modifiedAt, conditionsJSON, actionsJSON string
		var description, errorMessage, createdBy, modifiedBy sql.NullString

		if err := rows.Scan(
			&rule.ID, &rule.OrgID, &rule.Name, &description, &rule.EntityType,
			&rule.Enabled, &rule.TriggerOnCreate, &rule.TriggerOnUpdate, &rule.TriggerOnDelete,
			&rule.ConditionLogic, &conditionsJSON, &actionsJSON, &errorMessage, &rule.Priority,
			&createdAt, &modifiedAt, &createdBy, &modifiedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan validation rule: %w", err)
		}

		rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rule.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

		if description.Valid {
			rule.Description = &description.String
		}
		if errorMessage.Valid {
			rule.ErrorMessage = &errorMessage.String
		}
		if createdBy.Valid {
			rule.CreatedBy = &createdBy.String
		}
		if modifiedBy.Valid {
			rule.ModifiedBy = &modifiedBy.String
		}

		// Parse conditions JSON
		if err := json.Unmarshal([]byte(conditionsJSON), &rule.Conditions); err != nil {
			rule.Conditions = []entity.ValidationCondition{}
		}

		// Parse actions JSON
		if err := json.Unmarshal([]byte(actionsJSON), &rule.Actions); err != nil {
			rule.Actions = []entity.ValidationAction{}
		}

		rules = append(rules, rule)
	}

	if rules == nil {
		rules = []entity.ValidationRule{}
	}

	totalPages := total / params.PageSize
	if total%params.PageSize > 0 {
		totalPages++
	}

	return &entity.ValidationRuleListResponse{
		Data:       rules,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ListByEntityType retrieves validation rules for a specific entity type (for rule evaluation)
func (r *ValidationRepo) ListByEntityType(ctx context.Context, orgID, entityType string, enabledOnly bool) ([]entity.ValidationRule, error) {
	query := `
		SELECT id, org_id, name, description, entity_type,
			enabled, trigger_on_create, trigger_on_update, trigger_on_delete,
			condition_logic, conditions, actions, error_message, priority,
			created_at, modified_at, created_by, modified_by
		FROM validation_rules
		WHERE org_id = ? AND entity_type = ?
	`
	args := []any{orgID, entityType}

	if enabledOnly {
		query += ` AND enabled = 1`
	}

	// Always order by priority
	query += ` ORDER BY priority ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list validation rules by entity type: %w", err)
	}
	defer rows.Close()

	var rules []entity.ValidationRule
	for rows.Next() {
		var rule entity.ValidationRule
		var createdAt, modifiedAt, conditionsJSON, actionsJSON string
		var description, errorMessage, createdBy, modifiedBy sql.NullString

		if err := rows.Scan(
			&rule.ID, &rule.OrgID, &rule.Name, &description, &rule.EntityType,
			&rule.Enabled, &rule.TriggerOnCreate, &rule.TriggerOnUpdate, &rule.TriggerOnDelete,
			&rule.ConditionLogic, &conditionsJSON, &actionsJSON, &errorMessage, &rule.Priority,
			&createdAt, &modifiedAt, &createdBy, &modifiedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan validation rule: %w", err)
		}

		rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rule.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

		if description.Valid {
			rule.Description = &description.String
		}
		if errorMessage.Valid {
			rule.ErrorMessage = &errorMessage.String
		}
		if createdBy.Valid {
			rule.CreatedBy = &createdBy.String
		}
		if modifiedBy.Valid {
			rule.ModifiedBy = &modifiedBy.String
		}

		// Parse conditions JSON
		if err := json.Unmarshal([]byte(conditionsJSON), &rule.Conditions); err != nil {
			rule.Conditions = []entity.ValidationCondition{}
		}

		// Parse actions JSON
		if err := json.Unmarshal([]byte(actionsJSON), &rule.Actions); err != nil {
			rule.Actions = []entity.ValidationAction{}
		}

		rules = append(rules, rule)
	}

	if rules == nil {
		rules = []entity.ValidationRule{}
	}

	return rules, nil
}

// Update updates an existing validation rule
func (r *ValidationRepo) Update(ctx context.Context, orgID, id string, input entity.ValidationRuleUpdateInput, userID string) (*entity.ValidationRule, error) {
	// First get the existing rule
	existing, err := r.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Apply updates
	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.Description != nil {
		existing.Description = input.Description
	}
	if input.EntityType != nil {
		existing.EntityType = *input.EntityType
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.TriggerOnCreate != nil {
		existing.TriggerOnCreate = *input.TriggerOnCreate
	}
	if input.TriggerOnUpdate != nil {
		existing.TriggerOnUpdate = *input.TriggerOnUpdate
	}
	if input.TriggerOnDelete != nil {
		existing.TriggerOnDelete = *input.TriggerOnDelete
	}
	if input.ConditionLogic != nil {
		existing.ConditionLogic = strings.ToUpper(*input.ConditionLogic)
	}
	if input.Conditions != nil {
		existing.Conditions = input.Conditions
	}
	if input.Actions != nil {
		existing.Actions = input.Actions
	}
	if input.ErrorMessage != nil {
		existing.ErrorMessage = input.ErrorMessage
	}
	if input.Priority != nil {
		existing.Priority = *input.Priority
	}

	// Serialize conditions to JSON
	conditionsJSON, err := json.Marshal(existing.Conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}
	existing.ConditionsJSON = string(conditionsJSON)

	// Serialize actions to JSON
	actionsJSON, err := json.Marshal(existing.Actions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal actions: %w", err)
	}
	existing.ActionsJSON = string(actionsJSON)

	existing.ModifiedBy = &userID
	existing.ModifiedAt = time.Now().UTC()

	query := `
		UPDATE validation_rules SET
			name = ?, description = ?, entity_type = ?,
			enabled = ?, trigger_on_create = ?, trigger_on_update = ?, trigger_on_delete = ?,
			condition_logic = ?, conditions = ?, actions = ?, error_message = ?, priority = ?,
			modified_at = ?, modified_by = ?
		WHERE id = ? AND org_id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.Name, existing.Description, existing.EntityType,
		existing.Enabled, existing.TriggerOnCreate, existing.TriggerOnUpdate, existing.TriggerOnDelete,
		existing.ConditionLogic, existing.ConditionsJSON, existing.ActionsJSON, existing.ErrorMessage, existing.Priority,
		existing.ModifiedAt.Format(time.RFC3339), existing.ModifiedBy,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update validation rule: %w", err)
	}

	return existing, nil
}

// Delete deletes a validation rule
func (r *ValidationRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `DELETE FROM validation_rules WHERE id = ? AND org_id = ?`

	result, err := r.db.ExecContext(ctx, query, id, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete validation rule: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Toggle toggles the enabled state of a validation rule
func (r *ValidationRepo) Toggle(ctx context.Context, orgID, id string, userID string) (*entity.ValidationRule, error) {
	existing, err := r.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	existing.Enabled = !existing.Enabled
	existing.ModifiedBy = &userID
	existing.ModifiedAt = time.Now().UTC()

	query := `
		UPDATE validation_rules SET enabled = ?, modified_at = ?, modified_by = ?
		WHERE id = ? AND org_id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.Enabled, existing.ModifiedAt.Format(time.RFC3339), existing.ModifiedBy,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to toggle validation rule: %w", err)
	}

	return existing, nil
}
