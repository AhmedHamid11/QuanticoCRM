package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// MatchingRuleRepo handles database operations for matching rules
type MatchingRuleRepo struct {
	db db.DBConn
}

// NewMatchingRuleRepo creates a new MatchingRuleRepo
func NewMatchingRuleRepo(dbConn db.DBConn) *MatchingRuleRepo {
	return &MatchingRuleRepo{db: dbConn}
}

// WithDB returns a new MatchingRuleRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *MatchingRuleRepo) WithDB(dbConn db.DBConn) *MatchingRuleRepo {
	if dbConn == nil {
		return r
	}
	return &MatchingRuleRepo{db: dbConn}
}

// WithRawDB returns a new MatchingRuleRepo using a raw *sql.DB connection
// This is used for multi-tenant database routing with tenant databases
func (r *MatchingRuleRepo) WithRawDB(rawDB *sql.DB) *MatchingRuleRepo {
	if rawDB == nil {
		return r
	}
	return &MatchingRuleRepo{db: rawDB}
}

// ListRules returns all matching rules for an organization
func (r *MatchingRuleRepo) ListRules(ctx context.Context, orgID string) ([]entity.MatchingRule, error) {
	query := `SELECT id, org_id, name, description, entity_type, target_entity_type,
	          is_enabled, priority, threshold, high_confidence_threshold,
	          medium_confidence_threshold, blocking_strategy, field_configs,
	          created_at, modified_at
	          FROM matching_rules WHERE org_id = ? ORDER BY priority, name`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list matching rules: %w", err)
	}
	defer rows.Close()

	var rules []entity.MatchingRule
	for rows.Next() {
		rule, err := scanMatchingRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}

	if rules == nil {
		rules = []entity.MatchingRule{}
	}

	return rules, nil
}

// ListEnabledRules returns enabled matching rules for an entity type, ordered by priority
func (r *MatchingRuleRepo) ListEnabledRules(ctx context.Context, orgID, entityType string) ([]entity.MatchingRule, error) {
	query := `SELECT id, org_id, name, description, entity_type, target_entity_type,
	          is_enabled, priority, threshold, high_confidence_threshold,
	          medium_confidence_threshold, blocking_strategy, field_configs,
	          created_at, modified_at
	          FROM matching_rules
	          WHERE org_id = ? AND entity_type = ? AND is_enabled = 1
	          ORDER BY priority, name`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled matching rules: %w", err)
	}
	defer rows.Close()

	var rules []entity.MatchingRule
	for rows.Next() {
		rule, err := scanMatchingRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}

	if rules == nil {
		rules = []entity.MatchingRule{}
	}

	return rules, nil
}

// GetRule returns a matching rule by ID
func (r *MatchingRuleRepo) GetRule(ctx context.Context, orgID, ruleID string) (*entity.MatchingRule, error) {
	query := `SELECT id, org_id, name, description, entity_type, target_entity_type,
	          is_enabled, priority, threshold, high_confidence_threshold,
	          medium_confidence_threshold, blocking_strategy, field_configs,
	          created_at, modified_at
	          FROM matching_rules WHERE org_id = ? AND id = ?`

	row := r.db.QueryRowContext(ctx, query, orgID, ruleID)
	return scanMatchingRule(row)
}

// CreateRule creates a new matching rule
func (r *MatchingRuleRepo) CreateRule(ctx context.Context, orgID string, input entity.MatchingRuleCreateInput) (*entity.MatchingRule, error) {
	// Marshal field configs to JSON
	fieldConfigsJSON, err := json.Marshal(input.FieldConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal field configs: %w", err)
	}

	id := sfid.New("mrule")
	now := time.Now().Format(time.RFC3339)

	query := `INSERT INTO matching_rules (
	              id, org_id, name, description, entity_type, target_entity_type,
	              is_enabled, priority, threshold, high_confidence_threshold,
	              medium_confidence_threshold, blocking_strategy, field_configs,
	              created_at, modified_at
	          ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.db.ExecContext(ctx, query,
		id, orgID, input.Name, input.Description, input.EntityType, input.TargetEntityType,
		input.IsEnabled, input.Priority, input.Threshold, input.HighConfidenceThreshold,
		input.MediumConfidenceThreshold, input.BlockingStrategy, string(fieldConfigsJSON),
		now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create matching rule: %w", err)
	}

	return r.GetRule(ctx, orgID, id)
}

// UpdateRule updates an existing matching rule
func (r *MatchingRuleRepo) UpdateRule(ctx context.Context, orgID, ruleID string, input entity.MatchingRuleUpdateInput) (*entity.MatchingRule, error) {
	// Build dynamic UPDATE query based on provided fields
	updates := []string{}
	args := []interface{}{}

	if input.Name != nil {
		updates = append(updates, "name = ?")
		args = append(args, *input.Name)
	}
	if input.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *input.Description)
	}
	if input.IsEnabled != nil {
		updates = append(updates, "is_enabled = ?")
		args = append(args, *input.IsEnabled)
	}
	if input.Priority != nil {
		updates = append(updates, "priority = ?")
		args = append(args, *input.Priority)
	}
	if input.Threshold != nil {
		updates = append(updates, "threshold = ?")
		args = append(args, *input.Threshold)
	}
	if input.HighConfidenceThreshold != nil {
		updates = append(updates, "high_confidence_threshold = ?")
		args = append(args, *input.HighConfidenceThreshold)
	}
	if input.MediumConfidenceThreshold != nil {
		updates = append(updates, "medium_confidence_threshold = ?")
		args = append(args, *input.MediumConfidenceThreshold)
	}
	if input.BlockingStrategy != nil {
		updates = append(updates, "blocking_strategy = ?")
		args = append(args, *input.BlockingStrategy)
	}
	if input.FieldConfigs != nil {
		fieldConfigsJSON, err := json.Marshal(input.FieldConfigs)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal field configs: %w", err)
		}
		updates = append(updates, "field_configs = ?")
		args = append(args, string(fieldConfigsJSON))
	}

	if len(updates) == 0 {
		return r.GetRule(ctx, orgID, ruleID)
	}

	updates = append(updates, "modified_at = ?")
	args = append(args, time.Now().Format(time.RFC3339))

	// Add WHERE clause args
	args = append(args, orgID, ruleID)

	query := fmt.Sprintf("UPDATE matching_rules SET %s WHERE org_id = ? AND id = ?",
		joinUpdates(updates))

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update matching rule: %w", err)
	}

	return r.GetRule(ctx, orgID, ruleID)
}

// DeleteRule deletes a matching rule
func (r *MatchingRuleRepo) DeleteRule(ctx context.Context, orgID, ruleID string) error {
	query := `DELETE FROM matching_rules WHERE org_id = ? AND id = ?`

	result, err := r.db.ExecContext(ctx, query, orgID, ruleID)
	if err != nil {
		return fmt.Errorf("failed to delete matching rule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("matching rule not found")
	}

	return nil
}

// scanMatchingRule scans a row into a MatchingRule entity
func scanMatchingRule(row interface {
	Scan(dest ...interface{}) error
}) (*entity.MatchingRule, error) {
	var rule entity.MatchingRule
	var createdAt, modifiedAt string
	var description, targetEntityType sql.NullString

	err := row.Scan(
		&rule.ID, &rule.OrgID, &rule.Name, &description, &rule.EntityType, &targetEntityType,
		&rule.IsEnabled, &rule.Priority, &rule.Threshold, &rule.HighConfidenceThreshold,
		&rule.MediumConfidenceThreshold, &rule.BlockingStrategy, &rule.FieldConfigsJSON,
		&createdAt, &modifiedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan matching rule: %w", err)
	}

	// Handle nullable fields
	if description.Valid {
		rule.Description = description.String
	}
	if targetEntityType.Valid {
		rule.TargetEntityType = &targetEntityType.String
	}

	// Parse timestamps
	rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rule.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	// Unmarshal field configs from JSON
	if rule.FieldConfigsJSON != "" {
		if err := json.Unmarshal([]byte(rule.FieldConfigsJSON), &rule.FieldConfigs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal field configs: %w", err)
		}
	}

	return &rule, nil
}

// joinUpdates joins update clauses with commas
func joinUpdates(updates []string) string {
	result := ""
	for i, update := range updates {
		if i > 0 {
			result += ", "
		}
		result += update
	}
	return result
}
