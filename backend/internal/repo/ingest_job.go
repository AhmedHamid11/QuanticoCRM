package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

// IngestJobRepo handles CRUD operations for ingest jobs in tenant databases
type IngestJobRepo struct {
	db db.DBConn
}

// NewIngestJobRepo creates a new IngestJobRepo
func NewIngestJobRepo(conn db.DBConn) *IngestJobRepo {
	return &IngestJobRepo{db: conn}
}

// Create creates a new ingest job in the tenant database
func (r *IngestJobRepo) Create(ctx context.Context, tenantDB db.DBConn, job *entity.IngestJob) error {
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now

	// Default status to accepted if not set
	if job.Status == "" {
		job.Status = entity.IngestJobStatusAccepted
	}

	// Serialize errors and warnings to JSON
	errorsJSON := "[]"
	if len(job.Errors) > 0 {
		errorsBytes, err := json.Marshal(job.Errors)
		if err != nil {
			return fmt.Errorf("marshal errors: %w", err)
		}
		errorsJSON = string(errorsBytes)
	}

	warningsJSON := "[]"
	if len(job.Warnings) > 0 {
		warningsBytes, err := json.Marshal(job.Warnings)
		if err != nil {
			return fmt.Errorf("marshal warnings: %w", err)
		}
		warningsJSON = string(warningsBytes)
	}

	_, err := tenantDB.ExecContext(ctx, `
		INSERT INTO ingest_jobs (
			id, org_id, mirror_id, key_id, status,
			records_received, records_processed, records_promoted, records_skipped, records_failed,
			errors, warnings, started_at, completed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ID, job.OrgID, job.MirrorID, job.KeyID, job.Status,
		job.RecordsReceived, job.RecordsProcessed, job.RecordsPromoted, job.RecordsSkipped, job.RecordsFailed,
		errorsJSON, warningsJSON,
		timeToNullString(job.StartedAt), timeToNullString(job.CompletedAt),
		now.Format(time.RFC3339), now.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("insert ingest job: %w", err)
	}

	return nil
}

// GetByID retrieves an ingest job by ID from the tenant database
func (r *IngestJobRepo) GetByID(ctx context.Context, tenantDB db.DBConn, orgID, jobID string) (*entity.IngestJob, error) {
	var job entity.IngestJob
	var errorsJSON, warningsJSON string
	var startedAt, completedAt, createdAt, updatedAt sql.NullString

	err := tenantDB.QueryRowContext(ctx, `
		SELECT id, org_id, mirror_id, key_id, status,
			records_received, records_processed, records_promoted, records_skipped, records_failed,
			errors, warnings, started_at, completed_at, created_at, updated_at
		FROM ingest_jobs
		WHERE id = ? AND org_id = ?
	`, jobID, orgID).Scan(
		&job.ID, &job.OrgID, &job.MirrorID, &job.KeyID, &job.Status,
		&job.RecordsReceived, &job.RecordsProcessed, &job.RecordsPromoted, &job.RecordsSkipped, &job.RecordsFailed,
		&errorsJSON, &warningsJSON, &startedAt, &completedAt, &createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ingest job: %w", err)
	}

	// Parse JSON fields
	if err := json.Unmarshal([]byte(errorsJSON), &job.Errors); err != nil {
		job.Errors = []entity.RecordError{}
	}
	if err := json.Unmarshal([]byte(warningsJSON), &job.Warnings); err != nil {
		job.Warnings = []string{}
	}

	// Parse timestamps
	job.StartedAt = nullStringToTimePtr(startedAt)
	job.CompletedAt = nullStringToTimePtr(completedAt)
	if createdAt.Valid {
		t, _ := time.Parse(time.RFC3339, createdAt.String)
		job.CreatedAt = t
	}
	if updatedAt.Valid {
		t, _ := time.Parse(time.RFC3339, updatedAt.String)
		job.UpdatedAt = t
	}

	return &job, nil
}

// UpdateStatus updates the status of an ingest job
func (r *IngestJobRepo) UpdateStatus(ctx context.Context, tenantDB db.DBConn, jobID, status string) error {
	now := time.Now().UTC()

	// Build update query based on status
	var query string
	var args []interface{}

	switch status {
	case entity.IngestJobStatusProcessing:
		// Set started_at when moving to processing
		query = "UPDATE ingest_jobs SET status = ?, started_at = ?, updated_at = ? WHERE id = ?"
		args = []interface{}{status, now.Format(time.RFC3339), now.Format(time.RFC3339), jobID}

	case entity.IngestJobStatusComplete, entity.IngestJobStatusPartial, entity.IngestJobStatusFailed:
		// Set completed_at when moving to a terminal state
		query = "UPDATE ingest_jobs SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?"
		args = []interface{}{status, now.Format(time.RFC3339), now.Format(time.RFC3339), jobID}

	default:
		// Just update status and updated_at
		query = "UPDATE ingest_jobs SET status = ?, updated_at = ? WHERE id = ?"
		args = []interface{}{status, now.Format(time.RFC3339), jobID}
	}

	_, err := tenantDB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update ingest job status: %w", err)
	}

	return nil
}

// SetResult updates the final result of an ingest job
func (r *IngestJobRepo) SetResult(ctx context.Context, tenantDB db.DBConn, jobID string, result entity.IngestJobResult) error {
	now := time.Now().UTC()

	// Determine final status based on result counts
	var status string
	if result.RecordsFailed == 0 {
		status = entity.IngestJobStatusComplete
	} else if result.RecordsPromoted > 0 {
		status = entity.IngestJobStatusPartial
	} else {
		status = entity.IngestJobStatusFailed
	}

	// Serialize errors and warnings to JSON
	errorsJSON := "[]"
	if len(result.Errors) > 0 {
		errorsBytes, err := json.Marshal(result.Errors)
		if err != nil {
			return fmt.Errorf("marshal errors: %w", err)
		}
		errorsJSON = string(errorsBytes)
	}

	warningsJSON := "[]"
	if len(result.Warnings) > 0 {
		warningsBytes, err := json.Marshal(result.Warnings)
		if err != nil {
			return fmt.Errorf("marshal warnings: %w", err)
		}
		warningsJSON = string(warningsBytes)
	}

	_, err := tenantDB.ExecContext(ctx, `
		UPDATE ingest_jobs
		SET status = ?,
		    records_processed = ?,
		    records_promoted = ?,
		    records_skipped = ?,
		    records_failed = ?,
		    errors = ?,
		    warnings = ?,
		    completed_at = ?,
		    updated_at = ?
		WHERE id = ?
	`, status, result.RecordsProcessed, result.RecordsPromoted, result.RecordsSkipped, result.RecordsFailed,
		errorsJSON, warningsJSON, now.Format(time.RFC3339), now.Format(time.RFC3339), jobID)

	if err != nil {
		return fmt.Errorf("set ingest job result: %w", err)
	}

	return nil
}

// ListByMirror retrieves ingest jobs for a specific mirror
func (r *IngestJobRepo) ListByMirror(ctx context.Context, tenantDB db.DBConn, orgID, mirrorID string, limit int) ([]*entity.IngestJob, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := tenantDB.QueryContext(ctx, `
		SELECT id, org_id, mirror_id, key_id, status,
			records_received, records_processed, records_promoted, records_skipped, records_failed,
			errors, warnings, started_at, completed_at, created_at, updated_at
		FROM ingest_jobs
		WHERE org_id = ? AND mirror_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, orgID, mirrorID, limit)

	if err != nil {
		return nil, fmt.Errorf("list ingest jobs: %w", err)
	}
	defer rows.Close()

	jobs := []*entity.IngestJob{}
	for rows.Next() {
		var job entity.IngestJob
		var errorsJSON, warningsJSON string
		var startedAt, completedAt, createdAt, updatedAt sql.NullString

		err := rows.Scan(
			&job.ID, &job.OrgID, &job.MirrorID, &job.KeyID, &job.Status,
			&job.RecordsReceived, &job.RecordsProcessed, &job.RecordsPromoted, &job.RecordsSkipped, &job.RecordsFailed,
			&errorsJSON, &warningsJSON, &startedAt, &completedAt, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ingest job: %w", err)
		}

		// Parse JSON fields
		if err := json.Unmarshal([]byte(errorsJSON), &job.Errors); err != nil {
			job.Errors = []entity.RecordError{}
		}
		if err := json.Unmarshal([]byte(warningsJSON), &job.Warnings); err != nil {
			job.Warnings = []string{}
		}

		// Parse timestamps
		job.StartedAt = nullStringToTimePtr(startedAt)
		job.CompletedAt = nullStringToTimePtr(completedAt)
		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			job.CreatedAt = t
		}
		if updatedAt.Valid {
			t, _ := time.Parse(time.RFC3339, updatedAt.String)
			job.UpdatedAt = t
		}

		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// Helper functions

func timeToNullString(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func nullStringToTimePtr(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ns.String)
	if err != nil {
		return nil
	}
	return &t
}
