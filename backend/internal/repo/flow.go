package repo

import (
	"github.com/fastcrm/backend/internal/db"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/flow"
	"github.com/google/uuid"
)

// FlowRepo handles database operations for flows
type FlowRepo struct {
	db db.DBConn
}

// NewFlowRepo creates a new FlowRepo
func NewFlowRepo(conn db.DBConn) *FlowRepo {
	return &FlowRepo{db: conn}
}

// WithDB returns a new FlowRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *FlowRepo) WithDB(conn db.DBConn) *FlowRepo {
	if conn == nil {
		return r
	}
	return &FlowRepo{db: conn}
}

// DB returns the current database connection
func (r *FlowRepo) DB() db.DBConn {
	return r.db
}

// Ensure FlowRepo implements flow.FlowRepository
var _ flow.FlowRepository = (*FlowRepo)(nil)

// =============================================================================
// Flow Definition Operations
// =============================================================================

// GetFlow retrieves a flow definition by ID
func (r *FlowRepo) GetFlow(ctx context.Context, flowID, orgID string) (*flow.FlowDefinitionDB, error) {
	query := `
		SELECT id, org_id, name, description, version, definition, is_active,
		       created_by, created_at, modified_at, modified_by
		FROM flow_definitions
		WHERE id = ? AND org_id = ?
	`

	var f flow.FlowDefinitionDB
	var description, modifiedBy sql.NullString

	err := r.db.QueryRowContext(ctx, query, flowID, orgID).Scan(
		&f.ID, &f.OrgID, &f.Name, &description, &f.Version,
		&f.Definition, &f.IsActive, &f.CreatedBy, &f.CreatedAt,
		&f.ModifiedAt, &modifiedBy,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("flow not found: %s", flowID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get flow: %w", err)
	}

	if description.Valid {
		f.Description = &description.String
	}
	if modifiedBy.Valid {
		f.ModifiedBy = &modifiedBy.String
	}

	return &f, nil
}

// ListFlows retrieves flows for an organization with filtering
func (r *FlowRepo) ListFlows(ctx context.Context, orgID string, params flow.FlowListParams) (*flow.FlowListResponse, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}
	if params.SortBy == "" {
		params.SortBy = "modified_at"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	// Build query
	var conditions []string
	var args []interface{}
	args = append(args, orgID)
	conditions = append(conditions, "org_id = ?")

	if params.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		search := "%" + params.Search + "%"
		args = append(args, search, search)
	}

	if params.EntityType != "" {
		conditions = append(conditions, "json_extract(definition, '$.trigger.entityType') = ?")
		args = append(args, params.EntityType)
	}

	if params.IsActive != nil {
		conditions = append(conditions, "is_active = ?")
		if *params.IsActive {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	where := strings.Join(conditions, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM flow_definitions WHERE %s", where)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count flows: %w", err)
	}

	// Validate sort column
	validSortColumns := map[string]bool{
		"name": true, "created_at": true, "modified_at": true, "version": true,
	}
	if !validSortColumns[params.SortBy] {
		params.SortBy = "modified_at"
	}
	sortDir := "DESC"
	if strings.ToLower(params.SortDir) == "asc" {
		sortDir = "ASC"
	}

	// Fetch page
	offset := (params.Page - 1) * params.PageSize
	query := fmt.Sprintf(`
		SELECT id, org_id, name, description, version, definition, is_active,
		       created_by, created_at, modified_at, modified_by
		FROM flow_definitions
		WHERE %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, where, params.SortBy, sortDir)

	args = append(args, params.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list flows: %w", err)
	}
	defer rows.Close()

	var flows []flow.FlowDefinitionDB
	for rows.Next() {
		var f flow.FlowDefinitionDB
		var description, modifiedBy sql.NullString

		if err := rows.Scan(
			&f.ID, &f.OrgID, &f.Name, &description, &f.Version,
			&f.Definition, &f.IsActive, &f.CreatedBy, &f.CreatedAt,
			&f.ModifiedAt, &modifiedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan flow: %w", err)
		}

		if description.Valid {
			f.Description = &description.String
		}
		if modifiedBy.Valid {
			f.ModifiedBy = &modifiedBy.String
		}

		flows = append(flows, f)
	}

	totalPages := (total + params.PageSize - 1) / params.PageSize

	return &flow.FlowListResponse{
		Data:       flows,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// CreateFlow creates a new flow definition
func (r *FlowRepo) CreateFlow(ctx context.Context, f *flow.FlowDefinitionDB) error {
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	if f.Version == 0 {
		f.Version = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	f.CreatedAt = now
	f.ModifiedAt = now

	query := `
		INSERT INTO flow_definitions (id, org_id, name, description, version, definition, is_active, created_by, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		f.ID, f.OrgID, f.Name, f.Description, f.Version,
		f.Definition, f.IsActive, f.CreatedBy, f.CreatedAt, f.ModifiedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create flow: %w", err)
	}

	return nil
}

// UpdateFlow updates an existing flow definition
func (r *FlowRepo) UpdateFlow(ctx context.Context, f *flow.FlowDefinitionDB) error {
	f.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
	f.Version++

	query := `
		UPDATE flow_definitions
		SET name = ?, description = ?, version = ?, definition = ?, is_active = ?, modified_at = ?, modified_by = ?
		WHERE id = ? AND org_id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		f.Name, f.Description, f.Version, f.Definition, f.IsActive, f.ModifiedAt, f.ModifiedBy,
		f.ID, f.OrgID,
	)
	if err != nil {
		return fmt.Errorf("failed to update flow: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("flow not found: %s", f.ID)
	}

	return nil
}

// DeleteFlow deletes a flow definition
func (r *FlowRepo) DeleteFlow(ctx context.Context, flowID, orgID string) error {
	query := "DELETE FROM flow_definitions WHERE id = ? AND org_id = ?"
	result, err := r.db.ExecContext(ctx, query, flowID, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete flow: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("flow not found: %s", flowID)
	}

	return nil
}

// =============================================================================
// Flow Execution Operations
// =============================================================================

// GetExecution retrieves a flow execution by ID
func (r *FlowRepo) GetExecution(ctx context.Context, executionID string) (*flow.FlowExecutionDB, error) {
	query := `
		SELECT id, org_id, flow_id, user_id, status, current_step, variables, screen_data,
		       source_entity, source_record_id, error, started_at, completed_at
		FROM flow_executions
		WHERE id = ?
	`

	var e flow.FlowExecutionDB
	var sourceEntity, sourceRecordID, errorMsg, completedAt sql.NullString

	err := r.db.QueryRowContext(ctx, query, executionID).Scan(
		&e.ID, &e.OrgID, &e.FlowID, &e.UserID, &e.Status, &e.CurrentStep,
		&e.Variables, &e.ScreenData, &sourceEntity, &sourceRecordID,
		&errorMsg, &e.StartedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	if sourceEntity.Valid {
		e.SourceEntity = &sourceEntity.String
	}
	if sourceRecordID.Valid {
		e.SourceRecordID = &sourceRecordID.String
	}
	if errorMsg.Valid {
		e.Error = &errorMsg.String
	}
	if completedAt.Valid {
		e.CompletedAt = &completedAt.String
	}

	return &e, nil
}

// SaveExecution creates or updates a flow execution
func (r *FlowRepo) SaveExecution(ctx context.Context, e *flow.FlowExecutionDB) error {
	query := `
		INSERT INTO flow_executions (id, org_id, flow_id, user_id, status, current_step, variables, screen_data, source_entity, source_record_id, error, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			current_step = excluded.current_step,
			variables = excluded.variables,
			screen_data = excluded.screen_data,
			error = excluded.error,
			completed_at = excluded.completed_at
	`

	_, err := r.db.ExecContext(ctx, query,
		e.ID, e.OrgID, e.FlowID, e.UserID, e.Status, e.CurrentStep,
		e.Variables, e.ScreenData, e.SourceEntity, e.SourceRecordID,
		e.Error, e.StartedAt, e.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save execution: %w", err)
	}

	return nil
}

// ListExecutions retrieves executions with optional filters
func (r *FlowRepo) ListExecutions(ctx context.Context, orgID string, flowID *string, status *string, limit int) ([]flow.FlowExecutionDB, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var conditions []string
	var args []interface{}

	conditions = append(conditions, "org_id = ?")
	args = append(args, orgID)

	if flowID != nil && *flowID != "" {
		conditions = append(conditions, "flow_id = ?")
		args = append(args, *flowID)
	}

	if status != nil && *status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, *status)
	}

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT id, org_id, flow_id, user_id, status, current_step, variables, screen_data,
		       source_entity, source_record_id, error, started_at, completed_at
		FROM flow_executions
		WHERE %s
		ORDER BY started_at DESC
		LIMIT ?
	`, where)

	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}
	defer rows.Close()

	var executions []flow.FlowExecutionDB
	for rows.Next() {
		var e flow.FlowExecutionDB
		var sourceEntity, sourceRecordID, errorMsg, completedAt sql.NullString

		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.FlowID, &e.UserID, &e.Status, &e.CurrentStep,
			&e.Variables, &e.ScreenData, &sourceEntity, &sourceRecordID,
			&errorMsg, &e.StartedAt, &completedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}

		if sourceEntity.Valid {
			e.SourceEntity = &sourceEntity.String
		}
		if sourceRecordID.Valid {
			e.SourceRecordID = &sourceRecordID.String
		}
		if errorMsg.Valid {
			e.Error = &errorMsg.String
		}
		if completedAt.Valid {
			e.CompletedAt = &completedAt.String
		}

		executions = append(executions, e)
	}

	return executions, nil
}

// =============================================================================
// Helper: Convert FlowExecution to/from DB format
// =============================================================================

// ToDBExecution converts a FlowExecution to FlowExecutionDB
func ToDBExecution(exec *flow.FlowExecution, sourceEntity, sourceRecordID string) (*flow.FlowExecutionDB, error) {
	variables, err := json.Marshal(exec.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal variables: %w", err)
	}

	screenData, err := json.Marshal(exec.ScreenData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal screen data: %w", err)
	}

	db := &flow.FlowExecutionDB{
		ID:          exec.ID,
		OrgID:       exec.OrgID,
		FlowID:      exec.FlowID,
		UserID:      exec.UserID,
		Status:      string(exec.Status),
		CurrentStep: exec.CurrentStep,
		Variables:   string(variables),
		ScreenData:  string(screenData),
		StartedAt:   exec.StartedAt.UTC().Format(time.RFC3339),
	}

	if exec.CompletedAt != nil {
		completedAt := exec.CompletedAt.UTC().Format(time.RFC3339)
		db.CompletedAt = &completedAt
	}

	if sourceEntity != "" {
		db.SourceEntity = &sourceEntity
	}
	if sourceRecordID != "" {
		db.SourceRecordID = &sourceRecordID
	}
	if exec.Error != "" {
		db.Error = &exec.Error
	}

	return db, nil
}
