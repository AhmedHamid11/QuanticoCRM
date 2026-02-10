package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

type SalesforceRepo struct {
	db db.DBConn
}

func NewSalesforceRepo(conn db.DBConn) *SalesforceRepo {
	return &SalesforceRepo{db: conn}
}

func (r *SalesforceRepo) WithDB(conn db.DBConn) *SalesforceRepo {
	return &SalesforceRepo{db: conn}
}

// ========== Connection Operations (Master DB) ==========

// GetConnection retrieves a Salesforce connection by org ID
func (r *SalesforceRepo) GetConnection(ctx context.Context, orgID string) (*entity.SalesforceConnection, error) {
	query := `
		SELECT id, org_id, client_id, client_secret_encrypted, redirect_url, instance_url,
		       access_token_encrypted, refresh_token_encrypted, token_type, expires_at,
		       is_enabled, connected_at, created_at, updated_at
		FROM salesforce_connections
		WHERE org_id = ?
	`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}

	var conn entity.SalesforceConnection
	var isEnabledInt int
	var expiresAt, connectedAt, createdAt, updatedAt *string

	err = rows.Scan(
		&conn.ID, &conn.OrgID, &conn.ClientID, &conn.ClientSecretEncrypted, &conn.RedirectURL,
		&conn.InstanceURL, &conn.AccessTokenEncrypted, &conn.RefreshTokenEncrypted, &conn.TokenType,
		&expiresAt, &isEnabledInt, &connectedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	conn.IsEnabled = isEnabledInt == 1

	// Parse timestamps
	if expiresAt != nil && *expiresAt != "" {
		t, _ := time.Parse(time.RFC3339, *expiresAt)
		conn.ExpiresAt = &t
	}
	if connectedAt != nil && *connectedAt != "" {
		t, _ := time.Parse(time.RFC3339, *connectedAt)
		conn.ConnectedAt = &t
	}
	if createdAt != nil && *createdAt != "" {
		conn.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
	}
	if updatedAt != nil && *updatedAt != "" {
		conn.UpdatedAt, _ = time.Parse(time.RFC3339, *updatedAt)
	}

	return &conn, nil
}

// UpsertConnection creates or updates a Salesforce connection
func (r *SalesforceRepo) UpsertConnection(ctx context.Context, conn *entity.SalesforceConnection) error {
	isEnabledInt := 0
	if conn.IsEnabled {
		isEnabledInt = 1
	}

	var expiresAt, connectedAt *string
	if conn.ExpiresAt != nil {
		t := conn.ExpiresAt.Format(time.RFC3339)
		expiresAt = &t
	}
	if conn.ConnectedAt != nil {
		t := conn.ConnectedAt.Format(time.RFC3339)
		connectedAt = &t
	}

	query := `
		INSERT OR REPLACE INTO salesforce_connections
		(id, org_id, client_id, client_secret_encrypted, redirect_url, instance_url,
		 access_token_encrypted, refresh_token_encrypted, token_type, expires_at,
		 is_enabled, connected_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, query,
		conn.ID, conn.OrgID, conn.ClientID, conn.ClientSecretEncrypted, conn.RedirectURL,
		conn.InstanceURL, conn.AccessTokenEncrypted, conn.RefreshTokenEncrypted, conn.TokenType,
		expiresAt, isEnabledInt, connectedAt, now, now)

	return err
}

// UpdateTokens updates only the access and refresh tokens
func (r *SalesforceRepo) UpdateTokens(ctx context.Context, orgID string, accessTokenEncrypted, refreshTokenEncrypted []byte, expiresAt time.Time) error {
	query := `
		UPDATE salesforce_connections
		SET access_token_encrypted = ?, refresh_token_encrypted = ?, expires_at = ?, updated_at = ?
		WHERE org_id = ?
	`
	now := time.Now().UTC().Format(time.RFC3339)
	expiresAtStr := expiresAt.Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, accessTokenEncrypted, refreshTokenEncrypted, expiresAtStr, now, orgID)
	return err
}

// DeleteConnection deletes a Salesforce connection
func (r *SalesforceRepo) DeleteConnection(ctx context.Context, orgID string) error {
	query := `DELETE FROM salesforce_connections WHERE org_id = ?`
	_, err := r.db.ExecContext(ctx, query, orgID)
	return err
}

// SetEnabled enables or disables a Salesforce connection
func (r *SalesforceRepo) SetEnabled(ctx context.Context, orgID string, enabled bool) error {
	isEnabledInt := 0
	if enabled {
		isEnabledInt = 1
	}
	query := `UPDATE salesforce_connections SET is_enabled = ?, updated_at = ? WHERE org_id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, isEnabledInt, now, orgID)
	return err
}

// ========== Sync Job Operations (Tenant DB via WithDB) ==========

// CreateSyncJob creates a new sync job
func (r *SalesforceRepo) CreateSyncJob(ctx context.Context, job *entity.SyncJob) error {
	var startedAt, completedAt *string
	if job.StartedAt != nil {
		t := job.StartedAt.Format(time.RFC3339)
		startedAt = &t
	}
	if job.CompletedAt != nil {
		t := job.CompletedAt.Format(time.RFC3339)
		completedAt = &t
	}

	query := `
		INSERT INTO sync_jobs
		(id, org_id, batch_id, entity_type, status, total_instructions, delivered_instructions,
		 failed_instructions, batch_payload, error_message, retry_count, idempotency_key,
		 trigger_type, started_at, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, query,
		job.ID, job.OrgID, job.BatchID, job.EntityType, job.Status, job.TotalInstructions,
		job.DeliveredInstructions, job.FailedInstructions, job.BatchPayload, job.ErrorMessage,
		job.RetryCount, job.IdempotencyKey, job.TriggerType, startedAt, completedAt, now, now)

	return err
}

// GetSyncJob retrieves a sync job by ID
func (r *SalesforceRepo) GetSyncJob(ctx context.Context, jobID string) (*entity.SyncJob, error) {
	query := `
		SELECT id, org_id, batch_id, entity_type, status, total_instructions, delivered_instructions,
		       failed_instructions, batch_payload, error_message, retry_count, idempotency_key,
		       trigger_type, started_at, completed_at, created_at, updated_at
		FROM sync_jobs
		WHERE id = ?
	`

	rows, err := r.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}

	var job entity.SyncJob
	var startedAt, completedAt, createdAt, updatedAt *string

	err = rows.Scan(
		&job.ID, &job.OrgID, &job.BatchID, &job.EntityType, &job.Status, &job.TotalInstructions,
		&job.DeliveredInstructions, &job.FailedInstructions, &job.BatchPayload, &job.ErrorMessage,
		&job.RetryCount, &job.IdempotencyKey, &job.TriggerType, &startedAt, &completedAt,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse timestamps
	if startedAt != nil && *startedAt != "" {
		t, _ := time.Parse(time.RFC3339, *startedAt)
		job.StartedAt = &t
	}
	if completedAt != nil && *completedAt != "" {
		t, _ := time.Parse(time.RFC3339, *completedAt)
		job.CompletedAt = &t
	}
	if createdAt != nil && *createdAt != "" {
		job.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
	}
	if updatedAt != nil && *updatedAt != "" {
		job.UpdatedAt, _ = time.Parse(time.RFC3339, *updatedAt)
	}

	return &job, nil
}

// ListSyncJobs retrieves sync jobs for an org with pagination
func (r *SalesforceRepo) ListSyncJobs(ctx context.Context, orgID string, limit, offset int) ([]entity.SyncJob, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM sync_jobs WHERE org_id = ?`
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, orgID).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated jobs
	query := `
		SELECT id, org_id, batch_id, entity_type, status, total_instructions, delivered_instructions,
		       failed_instructions, batch_payload, error_message, retry_count, idempotency_key,
		       trigger_type, started_at, completed_at, created_at, updated_at
		FROM sync_jobs
		WHERE org_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []entity.SyncJob
	for rows.Next() {
		var job entity.SyncJob
		var startedAt, completedAt, createdAt, updatedAt *string

		err = rows.Scan(
			&job.ID, &job.OrgID, &job.BatchID, &job.EntityType, &job.Status, &job.TotalInstructions,
			&job.DeliveredInstructions, &job.FailedInstructions, &job.BatchPayload, &job.ErrorMessage,
			&job.RetryCount, &job.IdempotencyKey, &job.TriggerType, &startedAt, &completedAt,
			&createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		// Parse timestamps
		if startedAt != nil && *startedAt != "" {
			t, _ := time.Parse(time.RFC3339, *startedAt)
			job.StartedAt = &t
		}
		if completedAt != nil && *completedAt != "" {
			t, _ := time.Parse(time.RFC3339, *completedAt)
			job.CompletedAt = &t
		}
		if createdAt != nil && *createdAt != "" {
			job.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
		}
		if updatedAt != nil && *updatedAt != "" {
			job.UpdatedAt, _ = time.Parse(time.RFC3339, *updatedAt)
		}

		jobs = append(jobs, job)
	}

	return jobs, totalCount, nil
}

// UpdateSyncJobStatus updates a sync job's status
func (r *SalesforceRepo) UpdateSyncJobStatus(ctx context.Context, jobID, status string) error {
	query := `UPDATE sync_jobs SET status = ?, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, status, now, jobID)
	return err
}

// UpdateSyncJobProgress updates delivered and failed instruction counts
func (r *SalesforceRepo) UpdateSyncJobProgress(ctx context.Context, jobID string, delivered, failed int) error {
	query := `
		UPDATE sync_jobs
		SET delivered_instructions = ?, failed_instructions = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, delivered, failed, now, jobID)
	return err
}

// UpdateSyncJobCompletion marks a sync job as completed with final stats
func (r *SalesforceRepo) UpdateSyncJobCompletion(ctx context.Context, jobID, status string, delivered, failed int, errorMsg *string) error {
	query := `
		UPDATE sync_jobs
		SET status = ?, delivered_instructions = ?, failed_instructions = ?,
		    error_message = ?, completed_at = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, status, delivered, failed, errorMsg, now, now, jobID)
	return err
}

// ========== Field Mapping Operations (Master DB) ==========

// ListFieldMappings retrieves field mappings for an org and entity type
func (r *SalesforceRepo) ListFieldMappings(ctx context.Context, orgID, entityType string) ([]entity.SalesforceFieldMapping, error) {
	query := `
		SELECT id, org_id, entity_type, quantico_field, salesforce_object, salesforce_field,
		       created_at, updated_at
		FROM salesforce_field_mappings
		WHERE org_id = ? AND entity_type = ?
		ORDER BY quantico_field
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []entity.SalesforceFieldMapping
	for rows.Next() {
		var mapping entity.SalesforceFieldMapping
		var createdAt, updatedAt *string

		err = rows.Scan(
			&mapping.ID, &mapping.OrgID, &mapping.EntityType, &mapping.QuanticoField,
			&mapping.SalesforceObject, &mapping.SalesforceField, &createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		// Parse timestamps
		if createdAt != nil && *createdAt != "" {
			mapping.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
		}
		if updatedAt != nil && *updatedAt != "" {
			mapping.UpdatedAt, _ = time.Parse(time.RFC3339, *updatedAt)
		}

		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

// UpsertFieldMapping creates or updates a field mapping
func (r *SalesforceRepo) UpsertFieldMapping(ctx context.Context, mapping *entity.SalesforceFieldMapping) error {
	query := `
		INSERT OR REPLACE INTO salesforce_field_mappings
		(id, org_id, entity_type, quantico_field, salesforce_object, salesforce_field, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, query,
		mapping.ID, mapping.OrgID, mapping.EntityType, mapping.QuanticoField,
		mapping.SalesforceObject, mapping.SalesforceField, now, now)

	return err
}

// DeleteFieldMapping deletes a field mapping
func (r *SalesforceRepo) DeleteFieldMapping(ctx context.Context, mappingID string) error {
	query := `DELETE FROM salesforce_field_mappings WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, mappingID)
	return err
}
