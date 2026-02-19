package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// TaskRepo handles database operations for tasks
type TaskRepo struct {
	conn db.DBConn
}

// NewTaskRepo creates a new TaskRepo
func NewTaskRepo(conn db.DBConn) *TaskRepo {
	return &TaskRepo{conn: conn}
}

// WithDB returns a new TaskRepo using the specified database connection
// This is used for multi-tenant database routing
// Accepts db.DBConn interface for retry-enabled connections
func (r *TaskRepo) WithDB(conn db.DBConn) *TaskRepo {
	if conn == nil {
		return r
	}
	return &TaskRepo{conn: conn}
}

// DB returns the current database connection
func (r *TaskRepo) DB() db.DBConn {
	return r.conn
}

// Create inserts a new task into the database
func (r *TaskRepo) Create(ctx context.Context, orgID string, input entity.TaskCreateInput, userID string) (*entity.Task, error) {
	task := &entity.Task{
		ID:             sfid.NewTask(),
		OrgID:          orgID,
		Subject:        input.Subject,
		Description:    input.Description,
		Status:         input.Status,
		Priority:       input.Priority,
		Type:           input.Type,
		DueDate:        input.DueDate,
		ParentID:       input.ParentID,
		ParentType:     input.ParentType,
		ParentName:     input.ParentName,
		GmailMessageID: input.GmailMessageID,
		AssignedUserID: input.AssignedUserID,
		CreatedByID:    &userID,
		ModifiedByID:   &userID,
		CreatedAt:      time.Now().UTC(),
		ModifiedAt:     time.Now().UTC(),
		Deleted:        false,
		CustomFields:   input.CustomFields,
	}

	// Set defaults
	if task.Status == "" {
		task.Status = entity.TaskStatusOpen
	}
	if task.Priority == "" {
		task.Priority = entity.TaskPriorityNormal
	}
	if task.Type == "" {
		task.Type = entity.TaskTypeTodo
	}

	// Serialize custom fields to JSON
	customFieldsJSON := "{}"
	if task.CustomFields != nil {
		if jsonBytes, err := json.Marshal(task.CustomFields); err == nil {
			customFieldsJSON = string(jsonBytes)
		}
	}

	query := `
		INSERT INTO tasks (
			id, org_id, subject, description, status, priority, type,
			due_date, parent_id, parent_type, parent_name, gmail_message_id,
			assigned_user_id, created_by_id, modified_by_id,
			created_at, modified_at, deleted, custom_fields
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.conn.ExecContext(ctx, query,
		task.ID, task.OrgID, task.Subject, task.Description, task.Status, task.Priority, task.Type,
		task.DueDate, task.ParentID, task.ParentType, task.ParentName, task.GmailMessageID,
		task.AssignedUserID, task.CreatedByID, task.ModifiedByID,
		task.CreatedAt.Format(time.RFC3339), task.ModifiedAt.Format(time.RFC3339), task.Deleted, customFieldsJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return task, nil
}

// GetByID retrieves a task by its ID
func (r *TaskRepo) GetByID(ctx context.Context, orgID, id string) (*entity.Task, error) {
	// Note: No user join - tenant DBs don't have users table
	query := `
		SELECT t.id, t.org_id, COALESCE(t.subject, ''), COALESCE(t.description, ''), COALESCE(t.status, ''), COALESCE(t.priority, ''), COALESCE(t.type, ''),
			COALESCE(t.due_date, ''), COALESCE(t.parent_id, ''), COALESCE(t.parent_type, ''), COALESCE(t.parent_name, ''),
			COALESCE(t.gmail_message_id, ''), COALESCE(t.assigned_user_id, ''), COALESCE(t.created_by_id, ''), COALESCE(t.modified_by_id, ''),
			COALESCE(t.created_at, ''), COALESCE(t.modified_at, ''), COALESCE(t.deleted, 0), COALESCE(t.custom_fields, '{}'),
			'' AS created_by_name,
			'' AS modified_by_name
		FROM tasks t
		WHERE t.id = ? AND t.org_id = ? AND t.deleted = 0
	`

	var task entity.Task
	var createdAt, modifiedAt, customFieldsJSON string

	err := r.conn.QueryRowContext(ctx, query, id, orgID).Scan(
		&task.ID, &task.OrgID, &task.Subject, &task.Description, &task.Status, &task.Priority, &task.Type,
		&task.DueDate, &task.ParentID, &task.ParentType, &task.ParentName,
		&task.GmailMessageID, &task.AssignedUserID, &task.CreatedByID, &task.ModifiedByID,
		&createdAt, &modifiedAt, &task.Deleted, &customFieldsJSON,
		&task.CreatedByName, &task.ModifiedByName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	task.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	task.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	// Parse custom fields
	task.CustomFields = make(map[string]interface{})
	json.Unmarshal([]byte(customFieldsJSON), &task.CustomFields)

	return &task, nil
}

// ListByOrg retrieves all tasks for an organization with pagination, search, and filtering
func (r *TaskRepo) ListByOrg(ctx context.Context, orgID string, params entity.TaskListParams) (*entity.TaskListResponse, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}
	if params.SortBy == "" {
		params.SortBy = "created_at"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	// Validate sort column
	validSortCols := map[string]bool{
		"created_at": true, "modified_at": true, "subject": true,
		"due_date": true, "status": true, "priority": true, "type": true,
	}
	if !validSortCols[params.SortBy] {
		params.SortBy = "created_at"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		params.SortDir = "desc"
	}

	// Build query with filters - note: we don't join with users table since tenant DBs don't have it
	baseQuery := `FROM tasks t WHERE t.org_id = ? AND t.deleted = 0`
	args := []any{orgID}

	// Apply owner filter
	if params.Owner == "unassigned" {
		baseQuery += ` AND (t.assigned_user_id IS NULL OR t.assigned_user_id = '')`
	} else if params.Owner != "" {
		baseQuery += ` AND t.assigned_user_id = ?`
		args = append(args, params.Owner)
	}

	// Search filter
	if params.Search != "" {
		baseQuery += ` AND (t.subject LIKE ? OR t.description LIKE ?)`
		searchTerm := "%" + params.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	// Status filter
	if params.Status != "" {
		baseQuery += ` AND t.status = ?`
		args = append(args, params.Status)
	}

	// Type filter
	if params.Type != "" {
		baseQuery += ` AND t.type = ?`
		args = append(args, params.Type)
	}

	// Parent filters (for showing tasks related to a specific record)
	if params.ParentType != "" {
		baseQuery += ` AND t.parent_type = ?`
		args = append(args, params.ParentType)
	}
	if params.ParentID != "" {
		baseQuery += ` AND t.parent_id = ?`
		args = append(args, params.ParentID)
	}

	// Due date filters
	if params.DueBefore != "" {
		baseQuery += ` AND t.due_date <= ?`
		args = append(args, params.DueBefore)
	}
	if params.DueAfter != "" {
		baseQuery += ` AND t.due_date >= ?`
		args = append(args, params.DueAfter)
	}

	// Gmail message ID filter
	if params.GmailMessageID != "" {
		baseQuery += ` AND t.gmail_message_id = ?`
		args = append(args, params.GmailMessageID)
	}

	// Apply SQL-style filter if provided
	if params.Filter != "" {
		validColumns := map[string]bool{
			"subject": true, "description": true, "status": true, "priority": true, "type": true,
			"due_date": true, "parent_id": true, "parent_type": true, "parent_name": true,
			"gmail_message_id": true, "assigned_user_id": true, "created_at": true, "modified_at": true,
		}
		filterResult, err := util.ParseFilterWithColumns(params.Filter, validColumns, "t")
		if err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
		if filterResult != nil && filterResult.WhereClause != "" {
			baseQuery += " AND " + filterResult.WhereClause
			args = append(args, filterResult.Args...)
		}
	}

	// Skip COUNT(*) if the frontend already knows the total (saves row reads on Turso)
	var total int
	if params.KnownTotal > 0 {
		total = params.KnownTotal
	} else {
		countQuery := "SELECT COUNT(*) " + baseQuery
		if err := r.conn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return nil, fmt.Errorf("failed to count tasks: %w", err)
		}
	}

	// Query with pagination
	// Note: CreatedByName and ModifiedByName are empty - user names should be stored when creating/updating
	offset := (params.Page - 1) * params.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT t.id, t.org_id, COALESCE(t.subject, ''), COALESCE(t.description, ''), COALESCE(t.status, ''), COALESCE(t.priority, ''), COALESCE(t.type, ''),
			COALESCE(t.due_date, ''), COALESCE(t.parent_id, ''), COALESCE(t.parent_type, ''), COALESCE(t.parent_name, ''),
			COALESCE(t.gmail_message_id, ''), COALESCE(t.assigned_user_id, ''), COALESCE(t.created_by_id, ''), COALESCE(t.modified_by_id, ''),
			COALESCE(t.created_at, ''), COALESCE(t.modified_at, ''), COALESCE(t.deleted, 0), COALESCE(t.custom_fields, '{}'),
			'' AS created_by_name,
			'' AS modified_by_name
		%s ORDER BY t.%s %s LIMIT ? OFFSET ?
	`, baseQuery, params.SortBy, strings.ToUpper(params.SortDir))

	args = append(args, params.PageSize, offset)

	rows, err := r.conn.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []entity.Task
	for rows.Next() {
		var task entity.Task
		var createdAt, modifiedAt, customFieldsJSON string

		if err := rows.Scan(
			&task.ID, &task.OrgID, &task.Subject, &task.Description, &task.Status, &task.Priority, &task.Type,
			&task.DueDate, &task.ParentID, &task.ParentType, &task.ParentName,
			&task.GmailMessageID, &task.AssignedUserID, &task.CreatedByID, &task.ModifiedByID,
			&createdAt, &modifiedAt, &task.Deleted, &customFieldsJSON,
			&task.CreatedByName, &task.ModifiedByName,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		task.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		task.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)
		task.CustomFields = make(map[string]interface{})
		json.Unmarshal([]byte(customFieldsJSON), &task.CustomFields)
		tasks = append(tasks, task)
	}

	if tasks == nil {
		tasks = []entity.Task{}
	}

	totalPages := total / params.PageSize
	if total%params.PageSize > 0 {
		totalPages++
	}

	return &entity.TaskListResponse{
		Data:       tasks,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ListByParent retrieves tasks for a specific parent record (for related lists)
func (r *TaskRepo) ListByParent(ctx context.Context, orgID, parentType, parentID string, params entity.TaskListParams) (*entity.TaskListResponse, error) {
	params.ParentType = parentType
	params.ParentID = parentID
	return r.ListByOrg(ctx, orgID, params)
}

// Update updates an existing task
func (r *TaskRepo) Update(ctx context.Context, orgID, id string, input entity.TaskUpdateInput, userID string) (*entity.Task, error) {
	// First get the existing task
	existing, err := r.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Apply updates
	if input.Subject != nil {
		existing.Subject = *input.Subject
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	if input.Status != nil {
		existing.Status = *input.Status
	}
	if input.Priority != nil {
		existing.Priority = *input.Priority
	}
	if input.Type != nil {
		existing.Type = *input.Type
	}
	if input.DueDate != nil {
		if *input.DueDate == "" {
			existing.DueDate = nil // Clear due date
		} else {
			existing.DueDate = input.DueDate
		}
	}
	if input.ParentID != nil {
		existing.ParentID = input.ParentID
	}
	if input.ParentType != nil {
		existing.ParentType = input.ParentType
	}
	if input.ParentName != nil {
		existing.ParentName = *input.ParentName
	}
	if input.GmailMessageID != nil {
		existing.GmailMessageID = input.GmailMessageID
	}
	if input.AssignedUserID != nil {
		existing.AssignedUserID = input.AssignedUserID
	}

	// Merge custom fields
	if input.CustomFields != nil {
		if existing.CustomFields == nil {
			existing.CustomFields = make(map[string]interface{})
		}
		for k, v := range input.CustomFields {
			existing.CustomFields[k] = v
		}
	}

	existing.ModifiedByID = &userID
	existing.ModifiedAt = time.Now().UTC()

	// Serialize custom fields
	customFieldsJSON := "{}"
	if existing.CustomFields != nil {
		if jsonBytes, err := json.Marshal(existing.CustomFields); err == nil {
			customFieldsJSON = string(jsonBytes)
		}
	}

	query := `
		UPDATE tasks SET
			subject = ?, description = ?, status = ?, priority = ?, type = ?,
			due_date = ?, parent_id = ?, parent_type = ?, parent_name = ?, gmail_message_id = ?,
			assigned_user_id = ?, modified_by_id = ?, modified_at = ?, custom_fields = ?
		WHERE id = ? AND org_id = ? AND deleted = 0
	`

	_, err = r.conn.ExecContext(ctx, query,
		existing.Subject, existing.Description, existing.Status, existing.Priority, existing.Type,
		existing.DueDate, existing.ParentID, existing.ParentType, existing.ParentName, existing.GmailMessageID,
		existing.AssignedUserID, existing.ModifiedByID, existing.ModifiedAt.Format(time.RFC3339), customFieldsJSON,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return existing, nil
}

// Delete soft-deletes a task
func (r *TaskRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `UPDATE tasks SET deleted = 1, modified_at = ? WHERE id = ? AND org_id = ? AND deleted = 0`

	result, err := r.conn.ExecContext(ctx, query, time.Now().UTC().Format(time.RFC3339), id, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// CountByAssignedUser returns the number of tasks assigned to a user
func (r *TaskRepo) CountByAssignedUser(ctx context.Context, orgID, userID string) (int, error) {
	var count int
	err := r.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE org_id = ? AND assigned_user_id = ? AND deleted = 0`,
		orgID, userID,
	).Scan(&count)
	return count, err
}

// BulkReassignByAssignedUser reassigns all tasks from one user to another
func (r *TaskRepo) BulkReassignByAssignedUser(ctx context.Context, orgID, fromUserID, toUserID, modifiedByID string) (int64, error) {
	result, err := r.conn.ExecContext(ctx,
		`UPDATE tasks SET assigned_user_id = ?, modified_by_id = ?, modified_at = datetime('now')
		 WHERE org_id = ? AND assigned_user_id = ? AND deleted = 0`,
		toUserID, modifiedByID, orgID, fromUserID,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
