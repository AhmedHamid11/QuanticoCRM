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

// TripwireRepo handles database operations for tripwires
type TripwireRepo struct {
	db db.DBConn
}

// NewTripwireRepo creates a new TripwireRepo
func NewTripwireRepo(conn db.DBConn) *TripwireRepo {
	return &TripwireRepo{db: conn}
}

// WithDB returns a new TripwireRepo using the specified database connection
// This is used for multi-tenant database routing
func (r *TripwireRepo) WithDB(conn db.DBConn) *TripwireRepo {
	if conn == nil {
		return r
	}
	return &TripwireRepo{db: conn}
}

// DB returns the current database connection
func (r *TripwireRepo) DB() db.DBConn {
	return r.db
}

// Create inserts a new tripwire into the database
func (r *TripwireRepo) Create(ctx context.Context, orgID string, input entity.TripwireCreateInput, userID string) (*entity.Tripwire, error) {
	// Serialize conditions to JSON
	conditionsJSON, err := json.Marshal(input.Conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	conditionLogic := "AND"
	if input.ConditionLogic != "" {
		conditionLogic = input.ConditionLogic
	}

	tripwire := &entity.Tripwire{
		ID:             sfid.NewTripwire(),
		OrgID:          orgID,
		Name:           input.Name,
		Description:    input.Description,
		EntityType:     input.EntityType,
		EndpointURL:    input.EndpointURL,
		Enabled:        enabled,
		ConditionLogic: conditionLogic,
		Conditions:     input.Conditions,
		ConditionsJSON: string(conditionsJSON),
		CreatedAt:      time.Now().UTC(),
		ModifiedAt:     time.Now().UTC(),
		CreatedBy:      &userID,
		ModifiedBy:     &userID,
	}

	query := `
		INSERT INTO tripwires (
			id, org_id, name, description, entity_type, endpoint_url,
			enabled, condition_logic, conditions,
			created_at, modified_at, created_by, modified_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		tripwire.ID, tripwire.OrgID, tripwire.Name, tripwire.Description,
		tripwire.EntityType, tripwire.EndpointURL,
		tripwire.Enabled, tripwire.ConditionLogic, tripwire.ConditionsJSON,
		tripwire.CreatedAt.Format(time.RFC3339), tripwire.ModifiedAt.Format(time.RFC3339),
		tripwire.CreatedBy, tripwire.ModifiedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tripwire: %w", err)
	}

	return tripwire, nil
}

// GetByID retrieves a tripwire by its ID
func (r *TripwireRepo) GetByID(ctx context.Context, orgID, id string) (*entity.Tripwire, error) {
	query := `
		SELECT id, org_id, name, description, entity_type, endpoint_url,
			enabled, condition_logic, conditions,
			created_at, modified_at, created_by, modified_by
		FROM tripwires
		WHERE id = ? AND org_id = ?
	`

	var tripwire entity.Tripwire
	var createdAt, modifiedAt, conditionsJSON string
	var description, createdBy, modifiedBy sql.NullString

	err := r.db.QueryRowContext(ctx, query, id, orgID).Scan(
		&tripwire.ID, &tripwire.OrgID, &tripwire.Name, &description,
		&tripwire.EntityType, &tripwire.EndpointURL,
		&tripwire.Enabled, &tripwire.ConditionLogic, &conditionsJSON,
		&createdAt, &modifiedAt, &createdBy, &modifiedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tripwire: %w", err)
	}

	tripwire.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	tripwire.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	if description.Valid {
		tripwire.Description = &description.String
	}
	if createdBy.Valid {
		tripwire.CreatedBy = &createdBy.String
	}
	if modifiedBy.Valid {
		tripwire.ModifiedBy = &modifiedBy.String
	}

	// Parse conditions JSON
	if err := json.Unmarshal([]byte(conditionsJSON), &tripwire.Conditions); err != nil {
		tripwire.Conditions = []entity.TripwireCondition{}
	}

	return &tripwire, nil
}

// ListByOrg retrieves all tripwires for an organization with pagination
func (r *TripwireRepo) ListByOrg(ctx context.Context, orgID string, params entity.TripwireListParams) (*entity.TripwireListResponse, error) {
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
		"created_at": true, "modified_at": true, "name": true,
		"entity_type": true, "enabled": true,
	}
	if !validSortCols[params.SortBy] {
		params.SortBy = "created_at"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		params.SortDir = "desc"
	}

	// Build query with filters
	baseQuery := `FROM tripwires WHERE org_id = ?`
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
		return nil, fmt.Errorf("failed to count tripwires: %w", err)
	}

	// Query with pagination
	offset := (params.Page - 1) * params.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT id, org_id, name, description, entity_type, endpoint_url,
			enabled, condition_logic, conditions,
			created_at, modified_at, created_by, modified_by
		%s ORDER BY %s %s LIMIT ? OFFSET ?
	`, baseQuery, params.SortBy, strings.ToUpper(params.SortDir))

	args = append(args, params.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tripwires: %w", err)
	}
	defer rows.Close()

	var tripwires []entity.Tripwire
	for rows.Next() {
		var tripwire entity.Tripwire
		var createdAt, modifiedAt, conditionsJSON string
		var description, createdBy, modifiedBy sql.NullString

		if err := rows.Scan(
			&tripwire.ID, &tripwire.OrgID, &tripwire.Name, &description,
			&tripwire.EntityType, &tripwire.EndpointURL,
			&tripwire.Enabled, &tripwire.ConditionLogic, &conditionsJSON,
			&createdAt, &modifiedAt, &createdBy, &modifiedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tripwire: %w", err)
		}

		tripwire.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		tripwire.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

		if description.Valid {
			tripwire.Description = &description.String
		}
		if createdBy.Valid {
			tripwire.CreatedBy = &createdBy.String
		}
		if modifiedBy.Valid {
			tripwire.ModifiedBy = &modifiedBy.String
		}

		// Parse conditions JSON
		if err := json.Unmarshal([]byte(conditionsJSON), &tripwire.Conditions); err != nil {
			tripwire.Conditions = []entity.TripwireCondition{}
		}

		tripwires = append(tripwires, tripwire)
	}

	if tripwires == nil {
		tripwires = []entity.Tripwire{}
	}

	totalPages := total / params.PageSize
	if total%params.PageSize > 0 {
		totalPages++
	}

	return &entity.TripwireListResponse{
		Data:       tripwires,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ListByEntityType retrieves tripwires for a specific entity type (critical for evaluation)
func (r *TripwireRepo) ListByEntityType(ctx context.Context, orgID, entityType string, enabledOnly bool) ([]entity.Tripwire, error) {
	query := `
		SELECT id, org_id, name, description, entity_type, endpoint_url,
			enabled, condition_logic, conditions,
			created_at, modified_at, created_by, modified_by
		FROM tripwires
		WHERE org_id = ? AND entity_type = ?
	`
	args := []any{orgID, entityType}

	if enabledOnly {
		query += ` AND enabled = 1`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tripwires by entity type: %w", err)
	}
	defer rows.Close()

	var tripwires []entity.Tripwire
	for rows.Next() {
		var tripwire entity.Tripwire
		var createdAt, modifiedAt, conditionsJSON string
		var description, createdBy, modifiedBy sql.NullString

		if err := rows.Scan(
			&tripwire.ID, &tripwire.OrgID, &tripwire.Name, &description,
			&tripwire.EntityType, &tripwire.EndpointURL,
			&tripwire.Enabled, &tripwire.ConditionLogic, &conditionsJSON,
			&createdAt, &modifiedAt, &createdBy, &modifiedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tripwire: %w", err)
		}

		tripwire.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		tripwire.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

		if description.Valid {
			tripwire.Description = &description.String
		}
		if createdBy.Valid {
			tripwire.CreatedBy = &createdBy.String
		}
		if modifiedBy.Valid {
			tripwire.ModifiedBy = &modifiedBy.String
		}

		// Parse conditions JSON
		if err := json.Unmarshal([]byte(conditionsJSON), &tripwire.Conditions); err != nil {
			tripwire.Conditions = []entity.TripwireCondition{}
		}

		tripwires = append(tripwires, tripwire)
	}

	if tripwires == nil {
		tripwires = []entity.Tripwire{}
	}

	return tripwires, nil
}

// Update updates an existing tripwire
func (r *TripwireRepo) Update(ctx context.Context, orgID, id string, input entity.TripwireUpdateInput, userID string) (*entity.Tripwire, error) {
	// First get the existing tripwire
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
	if input.EndpointURL != nil {
		existing.EndpointURL = *input.EndpointURL
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.ConditionLogic != nil {
		existing.ConditionLogic = *input.ConditionLogic
	}
	if input.Conditions != nil {
		existing.Conditions = input.Conditions
	}

	// Serialize conditions to JSON
	conditionsJSON, err := json.Marshal(existing.Conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}
	existing.ConditionsJSON = string(conditionsJSON)

	existing.ModifiedBy = &userID
	existing.ModifiedAt = time.Now().UTC()

	query := `
		UPDATE tripwires SET
			name = ?, description = ?, entity_type = ?, endpoint_url = ?,
			enabled = ?, condition_logic = ?, conditions = ?,
			modified_at = ?, modified_by = ?
		WHERE id = ? AND org_id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.Name, existing.Description, existing.EntityType, existing.EndpointURL,
		existing.Enabled, existing.ConditionLogic, existing.ConditionsJSON,
		existing.ModifiedAt.Format(time.RFC3339), existing.ModifiedBy,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update tripwire: %w", err)
	}

	return existing, nil
}

// Delete deletes a tripwire
func (r *TripwireRepo) Delete(ctx context.Context, orgID, id string) error {
	query := `DELETE FROM tripwires WHERE id = ? AND org_id = ?`

	result, err := r.db.ExecContext(ctx, query, id, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete tripwire: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Toggle toggles the enabled state of a tripwire
func (r *TripwireRepo) Toggle(ctx context.Context, orgID, id string, userID string) (*entity.Tripwire, error) {
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
		UPDATE tripwires SET enabled = ?, modified_at = ?, modified_by = ?
		WHERE id = ? AND org_id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.Enabled, existing.ModifiedAt.Format(time.RFC3339), existing.ModifiedBy,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to toggle tripwire: %w", err)
	}

	return existing, nil
}

// GetWebhookSettings retrieves the webhook settings for an organization
func (r *TripwireRepo) GetWebhookSettings(ctx context.Context, orgID string) (*entity.OrgWebhookSettings, error) {
	query := `
		SELECT id, org_id, auth_type, api_key, bearer_token,
			custom_header_name, custom_header_value, timeout_ms,
			created_at, modified_at
		FROM org_webhook_settings
		WHERE org_id = ?
	`

	var settings entity.OrgWebhookSettings
	var createdAt, modifiedAt string
	var apiKey, bearerToken, customHeaderName, customHeaderValue sql.NullString

	err := r.db.QueryRowContext(ctx, query, orgID).Scan(
		&settings.ID, &settings.OrgID, &settings.AuthType,
		&apiKey, &bearerToken, &customHeaderName, &customHeaderValue,
		&settings.TimeoutMs, &createdAt, &modifiedAt,
	)
	if err == sql.ErrNoRows {
		// Return default settings if none exist
		return &entity.OrgWebhookSettings{
			OrgID:     orgID,
			AuthType:  entity.WebhookAuthNone,
			TimeoutMs: 5000,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook settings: %w", err)
	}

	settings.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	settings.ModifiedAt, _ = time.Parse(time.RFC3339, modifiedAt)

	if apiKey.Valid {
		settings.APIKey = &apiKey.String
	}
	if bearerToken.Valid {
		settings.BearerToken = &bearerToken.String
	}
	if customHeaderName.Valid {
		settings.CustomHeaderName = &customHeaderName.String
	}
	if customHeaderValue.Valid {
		settings.CustomHeaderValue = &customHeaderValue.String
	}

	return &settings, nil
}

// SaveWebhookSettings saves the webhook settings for an organization
func (r *TripwireRepo) SaveWebhookSettings(ctx context.Context, orgID string, input entity.OrgWebhookSettingsInput) (*entity.OrgWebhookSettings, error) {
	// Check if settings already exist
	existing, err := r.GetWebhookSettings(ctx, orgID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	timeoutMs := 5000
	if input.TimeoutMs != nil {
		timeoutMs = *input.TimeoutMs
	}

	if existing.ID == "" {
		// Create new settings
		settings := &entity.OrgWebhookSettings{
			ID:                sfid.NewWebhookSettings(),
			OrgID:             orgID,
			AuthType:          input.AuthType,
			APIKey:            input.APIKey,
			BearerToken:       input.BearerToken,
			CustomHeaderName:  input.CustomHeaderName,
			CustomHeaderValue: input.CustomHeaderValue,
			TimeoutMs:         timeoutMs,
			CreatedAt:         now,
			ModifiedAt:        now,
		}

		query := `
			INSERT INTO org_webhook_settings (
				id, org_id, auth_type, api_key, bearer_token,
				custom_header_name, custom_header_value, timeout_ms,
				created_at, modified_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		_, err = r.db.ExecContext(ctx, query,
			settings.ID, settings.OrgID, settings.AuthType,
			settings.APIKey, settings.BearerToken,
			settings.CustomHeaderName, settings.CustomHeaderValue,
			settings.TimeoutMs, settings.CreatedAt.Format(time.RFC3339),
			settings.ModifiedAt.Format(time.RFC3339),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create webhook settings: %w", err)
		}

		return settings, nil
	}

	// Update existing settings
	existing.AuthType = input.AuthType
	existing.APIKey = input.APIKey
	existing.BearerToken = input.BearerToken
	existing.CustomHeaderName = input.CustomHeaderName
	existing.CustomHeaderValue = input.CustomHeaderValue
	existing.TimeoutMs = timeoutMs
	existing.ModifiedAt = now

	query := `
		UPDATE org_webhook_settings SET
			auth_type = ?, api_key = ?, bearer_token = ?,
			custom_header_name = ?, custom_header_value = ?,
			timeout_ms = ?, modified_at = ?
		WHERE org_id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		existing.AuthType, existing.APIKey, existing.BearerToken,
		existing.CustomHeaderName, existing.CustomHeaderValue,
		existing.TimeoutMs, existing.ModifiedAt.Format(time.RFC3339),
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update webhook settings: %w", err)
	}

	return existing, nil
}

// CreateLog creates a new tripwire execution log entry
func (r *TripwireRepo) CreateLog(ctx context.Context, log *entity.TripwireLog) error {
	query := `
		INSERT INTO tripwire_logs (
			id, tripwire_id, tripwire_name, org_id, record_id, entity_type,
			event_type, status, response_code, error_message, duration_ms, executed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		log.ID, log.TripwireID, log.TripwireName, log.OrgID, log.RecordID, log.EntityType,
		log.EventType, log.Status, log.ResponseCode, log.ErrorMessage,
		log.DurationMs, log.ExecutedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to create tripwire log: %w", err)
	}

	return nil
}

// ListLogs retrieves tripwire execution logs
func (r *TripwireRepo) ListLogs(ctx context.Context, orgID string, params entity.TripwireLogListParams) (*entity.TripwireLogListResponse, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}
	if params.SortBy == "" {
		params.SortBy = "executed_at"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	// Validate sort column
	validSortCols := map[string]bool{
		"executed_at": true, "status": true, "event_type": true,
	}
	if !validSortCols[params.SortBy] {
		params.SortBy = "executed_at"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		params.SortDir = "desc"
	}

	// Build query with filters
	baseQuery := `FROM tripwire_logs WHERE org_id = ?`
	args := []any{orgID}

	if params.TripwireID != "" {
		baseQuery += ` AND tripwire_id = ?`
		args = append(args, params.TripwireID)
	}

	if params.Status != "" {
		baseQuery += ` AND status = ?`
		args = append(args, params.Status)
	}

	if params.EventType != "" {
		baseQuery += ` AND event_type = ?`
		args = append(args, params.EventType)
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count tripwire logs: %w", err)
	}

	// Query with pagination
	offset := (params.Page - 1) * params.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT id, tripwire_id, tripwire_name, org_id, record_id, entity_type,
			event_type, status, response_code, error_message, duration_ms, executed_at
		%s ORDER BY %s %s LIMIT ? OFFSET ?
	`, baseQuery, params.SortBy, strings.ToUpper(params.SortDir))

	args = append(args, params.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tripwire logs: %w", err)
	}
	defer rows.Close()

	var logs []entity.TripwireLog
	for rows.Next() {
		var log entity.TripwireLog
		var executedAt string
		var tripwireName, errorMessage sql.NullString
		var responseCode, durationMs sql.NullInt64

		if err := rows.Scan(
			&log.ID, &log.TripwireID, &tripwireName, &log.OrgID, &log.RecordID, &log.EntityType,
			&log.EventType, &log.Status, &responseCode, &errorMessage, &durationMs, &executedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tripwire log: %w", err)
		}

		log.ExecutedAt, _ = time.Parse(time.RFC3339, executedAt)

		if tripwireName.Valid {
			log.TripwireName = &tripwireName.String
		}
		if errorMessage.Valid {
			log.ErrorMessage = &errorMessage.String
		}
		if responseCode.Valid {
			code := int(responseCode.Int64)
			log.ResponseCode = &code
		}
		if durationMs.Valid {
			dur := int(durationMs.Int64)
			log.DurationMs = &dur
		}

		logs = append(logs, log)
	}

	if logs == nil {
		logs = []entity.TripwireLog{}
	}

	totalPages := total / params.PageSize
	if total%params.PageSize > 0 {
		totalPages++
	}

	return &entity.TripwireLogListResponse{
		Data:       logs,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}
