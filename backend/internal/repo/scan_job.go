package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

type ScanJobRepo struct {
	db db.DBConn
}

func NewScanJobRepo(conn db.DBConn) *ScanJobRepo {
	return &ScanJobRepo{db: conn}
}

func (r *ScanJobRepo) WithDB(conn db.DBConn) *ScanJobRepo {
	return &ScanJobRepo{db: conn}
}

// ========== Schedule Operations (Master DB) ==========

// GetSchedule retrieves a schedule for a specific entity type
func (r *ScanJobRepo) GetSchedule(ctx context.Context, orgID, entityType string) (*entity.ScanSchedule, error) {
	query := `
		SELECT id, org_id, entity_type, frequency, day_of_week, day_of_month,
		       hour, minute, is_enabled, last_run_at, next_run_at, created_at, updated_at
		FROM scan_schedules
		WHERE org_id = ? AND entity_type = ?
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No schedule found
	}

	var schedule entity.ScanSchedule
	var isEnabledInt int
	var lastRunAt, nextRunAt, createdAt, updatedAt *string

	err = rows.Scan(
		&schedule.ID, &schedule.OrgID, &schedule.EntityType, &schedule.Frequency,
		&schedule.DayOfWeek, &schedule.DayOfMonth, &schedule.Hour, &schedule.Minute,
		&isEnabledInt, &lastRunAt, &nextRunAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	schedule.IsEnabled = isEnabledInt == 1

	// Parse timestamps
	if lastRunAt != nil && *lastRunAt != "" {
		t, _ := time.Parse(time.RFC3339, *lastRunAt)
		schedule.LastRunAt = &t
	}
	if nextRunAt != nil && *nextRunAt != "" {
		t, _ := time.Parse(time.RFC3339, *nextRunAt)
		schedule.NextRunAt = &t
	}
	if createdAt != nil && *createdAt != "" {
		schedule.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
	}
	if updatedAt != nil && *updatedAt != "" {
		schedule.UpdatedAt, _ = time.Parse(time.RFC3339, *updatedAt)
	}

	return &schedule, nil
}

// ListSchedules retrieves all schedules for an organization
func (r *ScanJobRepo) ListSchedules(ctx context.Context, orgID string) ([]entity.ScanSchedule, error) {
	query := `
		SELECT id, org_id, entity_type, frequency, day_of_week, day_of_month,
		       hour, minute, is_enabled, last_run_at, next_run_at, created_at, updated_at
		FROM scan_schedules
		WHERE org_id = ?
		ORDER BY entity_type
	`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []entity.ScanSchedule
	for rows.Next() {
		var schedule entity.ScanSchedule
		var isEnabledInt int
		var lastRunAt, nextRunAt, createdAt, updatedAt *string

		err = rows.Scan(
			&schedule.ID, &schedule.OrgID, &schedule.EntityType, &schedule.Frequency,
			&schedule.DayOfWeek, &schedule.DayOfMonth, &schedule.Hour, &schedule.Minute,
			&isEnabledInt, &lastRunAt, &nextRunAt, &createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		schedule.IsEnabled = isEnabledInt == 1

		// Parse timestamps
		if lastRunAt != nil && *lastRunAt != "" {
			t, _ := time.Parse(time.RFC3339, *lastRunAt)
			schedule.LastRunAt = &t
		}
		if nextRunAt != nil && *nextRunAt != "" {
			t, _ := time.Parse(time.RFC3339, *nextRunAt)
			schedule.NextRunAt = &t
		}
		if createdAt != nil && *createdAt != "" {
			schedule.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
		}
		if updatedAt != nil && *updatedAt != "" {
			schedule.UpdatedAt, _ = time.Parse(time.RFC3339, *updatedAt)
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// ListAllEnabledSchedules retrieves all enabled schedules across all orgs (for scheduler startup)
func (r *ScanJobRepo) ListAllEnabledSchedules(ctx context.Context) ([]entity.ScanSchedule, error) {
	query := `
		SELECT id, org_id, entity_type, frequency, day_of_week, day_of_month,
		       hour, minute, is_enabled, last_run_at, next_run_at, created_at, updated_at
		FROM scan_schedules
		WHERE is_enabled = 1
		ORDER BY org_id, entity_type
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []entity.ScanSchedule
	for rows.Next() {
		var schedule entity.ScanSchedule
		var isEnabledInt int
		var lastRunAt, nextRunAt, createdAt, updatedAt *string

		err = rows.Scan(
			&schedule.ID, &schedule.OrgID, &schedule.EntityType, &schedule.Frequency,
			&schedule.DayOfWeek, &schedule.DayOfMonth, &schedule.Hour, &schedule.Minute,
			&isEnabledInt, &lastRunAt, &nextRunAt, &createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		schedule.IsEnabled = isEnabledInt == 1

		// Parse timestamps
		if lastRunAt != nil && *lastRunAt != "" {
			t, _ := time.Parse(time.RFC3339, *lastRunAt)
			schedule.LastRunAt = &t
		}
		if nextRunAt != nil && *nextRunAt != "" {
			t, _ := time.Parse(time.RFC3339, *nextRunAt)
			schedule.NextRunAt = &t
		}
		if createdAt != nil && *createdAt != "" {
			schedule.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
		}
		if updatedAt != nil && *updatedAt != "" {
			schedule.UpdatedAt, _ = time.Parse(time.RFC3339, *updatedAt)
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// UpsertSchedule creates or updates a schedule
func (r *ScanJobRepo) UpsertSchedule(ctx context.Context, schedule *entity.ScanSchedule) error {
	isEnabledInt := 0
	if schedule.IsEnabled {
		isEnabledInt = 1
	}

	var lastRunAt, nextRunAt *string
	if schedule.LastRunAt != nil {
		t := schedule.LastRunAt.Format(time.RFC3339)
		lastRunAt = &t
	}
	if schedule.NextRunAt != nil {
		t := schedule.NextRunAt.Format(time.RFC3339)
		nextRunAt = &t
	}

	query := `
		INSERT OR REPLACE INTO scan_schedules
		(id, org_id, entity_type, frequency, day_of_week, day_of_month, hour, minute,
		 is_enabled, last_run_at, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, query,
		schedule.ID, schedule.OrgID, schedule.EntityType, schedule.Frequency,
		schedule.DayOfWeek, schedule.DayOfMonth, schedule.Hour, schedule.Minute,
		isEnabledInt, lastRunAt, nextRunAt, now, now)

	return err
}

// DeleteSchedule deletes a schedule
func (r *ScanJobRepo) DeleteSchedule(ctx context.Context, orgID, entityType string) error {
	query := `DELETE FROM scan_schedules WHERE org_id = ? AND entity_type = ?`
	_, err := r.db.ExecContext(ctx, query, orgID, entityType)
	return err
}

// UpdateNextRunAt updates the next_run_at timestamp for a schedule
func (r *ScanJobRepo) UpdateNextRunAt(ctx context.Context, scheduleID string, nextRun time.Time) error {
	query := `UPDATE scan_schedules SET next_run_at = ?, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	nextRunStr := nextRun.Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, nextRunStr, now, scheduleID)
	return err
}

// ========== Job Operations (Tenant DB) ==========

// CreateJob creates a new scan job
func (r *ScanJobRepo) CreateJob(ctx context.Context, job *entity.ScanJob) error {
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
		INSERT INTO scan_jobs
		(id, org_id, entity_type, schedule_id, status, trigger_type, total_records,
		 processed_records, duplicates_found, error_message, status_text, started_at, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, query,
		job.ID, job.OrgID, job.EntityType, job.ScheduleID, job.Status, job.TriggerType,
		job.TotalRecords, job.ProcessedRecords, job.DuplicatesFound, job.ErrorMessage, job.StatusText,
		startedAt, completedAt, now, now)

	return err
}

// GetJob retrieves a job by ID
func (r *ScanJobRepo) GetJob(ctx context.Context, jobID string) (*entity.ScanJob, error) {
	query := `
		SELECT id, org_id, entity_type, schedule_id, status, trigger_type, total_records,
		       processed_records, duplicates_found, error_message, status_text, started_at, completed_at,
		       created_at, updated_at
		FROM scan_jobs
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

	var job entity.ScanJob
	var startedAt, completedAt, createdAt, updatedAt *string

	err = rows.Scan(
		&job.ID, &job.OrgID, &job.EntityType, &job.ScheduleID, &job.Status, &job.TriggerType,
		&job.TotalRecords, &job.ProcessedRecords, &job.DuplicatesFound, &job.ErrorMessage, &job.StatusText,
		&startedAt, &completedAt, &createdAt, &updatedAt,
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

// ListJobs retrieves jobs for an org with pagination
func (r *ScanJobRepo) ListJobs(ctx context.Context, orgID string, limit, offset int) ([]entity.ScanJob, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM scan_jobs WHERE org_id = ?`
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, orgID).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated jobs
	query := `
		SELECT id, org_id, entity_type, schedule_id, status, trigger_type, total_records,
		       processed_records, duplicates_found, error_message, status_text, started_at, completed_at,
		       created_at, updated_at
		FROM scan_jobs
		WHERE org_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []entity.ScanJob
	for rows.Next() {
		var job entity.ScanJob
		var startedAt, completedAt, createdAt, updatedAt *string

		err = rows.Scan(
			&job.ID, &job.OrgID, &job.EntityType, &job.ScheduleID, &job.Status, &job.TriggerType,
			&job.TotalRecords, &job.ProcessedRecords, &job.DuplicatesFound, &job.ErrorMessage, &job.StatusText,
			&startedAt, &completedAt, &createdAt, &updatedAt,
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

// ListJobsByEntity retrieves jobs for a specific entity type with pagination
func (r *ScanJobRepo) ListJobsByEntity(ctx context.Context, orgID, entityType string, limit, offset int) ([]entity.ScanJob, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM scan_jobs WHERE org_id = ? AND entity_type = ?`
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, orgID, entityType).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated jobs
	query := `
		SELECT id, org_id, entity_type, schedule_id, status, trigger_type, total_records,
		       processed_records, duplicates_found, error_message, status_text, started_at, completed_at,
		       created_at, updated_at
		FROM scan_jobs
		WHERE org_id = ? AND entity_type = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityType, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []entity.ScanJob
	for rows.Next() {
		var job entity.ScanJob
		var startedAt, completedAt, createdAt, updatedAt *string

		err = rows.Scan(
			&job.ID, &job.OrgID, &job.EntityType, &job.ScheduleID, &job.Status, &job.TriggerType,
			&job.TotalRecords, &job.ProcessedRecords, &job.DuplicatesFound, &job.ErrorMessage, &job.StatusText,
			&startedAt, &completedAt, &createdAt, &updatedAt,
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

// GetRunningJobForEntity checks if a job is currently running for an entity
func (r *ScanJobRepo) GetRunningJobForEntity(ctx context.Context, orgID, entityType string) (*entity.ScanJob, error) {
	query := `
		SELECT id, org_id, entity_type, schedule_id, status, trigger_type, total_records,
		       processed_records, duplicates_found, error_message, status_text, started_at, completed_at,
		       created_at, updated_at
		FROM scan_jobs
		WHERE org_id = ? AND entity_type = ? AND status = ?
		LIMIT 1
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, entityType, entity.ScanStatusRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No running job
	}

	var job entity.ScanJob
	var startedAt, completedAt, createdAt, updatedAt *string

	err = rows.Scan(
		&job.ID, &job.OrgID, &job.EntityType, &job.ScheduleID, &job.Status, &job.TriggerType,
		&job.TotalRecords, &job.ProcessedRecords, &job.DuplicatesFound, &job.ErrorMessage, &job.StatusText,
		&startedAt, &completedAt, &createdAt, &updatedAt,
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

// UpdateJobStatus updates a job's status
func (r *ScanJobRepo) UpdateJobStatus(ctx context.Context, jobID, status string) error {
	query := `UPDATE scan_jobs SET status = ?, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, status, now, jobID)
	return err
}

// UpdateJobStatusText updates the status_text field (for backfill progress messages)
func (r *ScanJobRepo) UpdateJobStatusText(ctx context.Context, jobID string, statusText *string) error {
	query := `UPDATE scan_jobs SET status_text = ?, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, statusText, now, jobID)
	return err
}

// UpdateJobProgress updates processed records and duplicates found
func (r *ScanJobRepo) UpdateJobProgress(ctx context.Context, jobID string, processed, duplicates int) error {
	query := `
		UPDATE scan_jobs
		SET processed_records = ?, duplicates_found = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, processed, duplicates, now, jobID)
	return err
}

// UpdateJobCompletion marks a job as completed with final stats
func (r *ScanJobRepo) UpdateJobCompletion(ctx context.Context, jobID, status string, totalRecords, processed, duplicates int) error {
	query := `
		UPDATE scan_jobs
		SET status = ?, total_records = ?, processed_records = ?,
		    duplicates_found = ?, status_text = NULL, completed_at = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, status, totalRecords, processed, duplicates, now, now, jobID)
	return err
}

// CountRunningJobsForOrg counts currently running jobs for an org (for rate limiting)
func (r *ScanJobRepo) CountRunningJobsForOrg(ctx context.Context, orgID string) (int, error) {
	query := `SELECT COUNT(*) FROM scan_jobs WHERE org_id = ? AND status = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, orgID, entity.ScanStatusRunning).Scan(&count)
	return count, err
}

// DeleteJob deletes a scan job and its associated checkpoint
func (r *ScanJobRepo) DeleteJob(ctx context.Context, jobID string) error {
	// Delete checkpoint first (FK-like cleanup)
	_, _ = r.db.ExecContext(ctx, "DELETE FROM scan_checkpoints WHERE job_id = ?", jobID)
	// Delete the job itself
	_, err := r.db.ExecContext(ctx, "DELETE FROM scan_jobs WHERE id = ?", jobID)
	return err
}

// DeleteNonRunningJobs deletes all completed, failed, and cancelled jobs for an org
func (r *ScanJobRepo) DeleteNonRunningJobs(ctx context.Context, orgID string) (int64, error) {
	// Delete associated checkpoints first
	_, _ = r.db.ExecContext(ctx, `
		DELETE FROM scan_checkpoints WHERE job_id IN (
			SELECT id FROM scan_jobs WHERE org_id = ? AND status IN ('completed', 'failed', 'cancelled')
		)
	`, orgID)
	// Delete the jobs
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM scan_jobs WHERE org_id = ? AND status IN ('completed', 'failed', 'cancelled')
	`, orgID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ========== Checkpoint Operations (Tenant DB) ==========

// SaveCheckpoint saves or updates a checkpoint
func (r *ScanJobRepo) SaveCheckpoint(ctx context.Context, checkpoint *entity.ScanCheckpoint) error {
	query := `
		INSERT OR REPLACE INTO scan_checkpoints
		(id, job_id, last_offset, last_processed_id, retry_count, chunk_size, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, query,
		checkpoint.ID, checkpoint.JobID, checkpoint.LastOffset, checkpoint.LastProcessedID,
		checkpoint.RetryCount, checkpoint.ChunkSize, now, now)

	return err
}

// GetCheckpoint retrieves a checkpoint for a job
func (r *ScanJobRepo) GetCheckpoint(ctx context.Context, jobID string) (*entity.ScanCheckpoint, error) {
	query := `
		SELECT id, job_id, last_offset, last_processed_id, retry_count, chunk_size, created_at, updated_at
		FROM scan_checkpoints
		WHERE job_id = ?
	`

	rows, err := r.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No checkpoint found
	}

	var checkpoint entity.ScanCheckpoint
	var createdAt, updatedAt *string

	err = rows.Scan(
		&checkpoint.ID, &checkpoint.JobID, &checkpoint.LastOffset, &checkpoint.LastProcessedID,
		&checkpoint.RetryCount, &checkpoint.ChunkSize, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse timestamps
	if createdAt != nil && *createdAt != "" {
		checkpoint.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
	}
	if updatedAt != nil && *updatedAt != "" {
		checkpoint.UpdatedAt, _ = time.Parse(time.RFC3339, *updatedAt)
	}

	return &checkpoint, nil
}

// IncrementRetryCount increments the retry count for a checkpoint
func (r *ScanJobRepo) IncrementRetryCount(ctx context.Context, jobID string) error {
	query := `
		UPDATE scan_checkpoints
		SET retry_count = retry_count + 1, updated_at = ?
		WHERE job_id = ?
	`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, now, jobID)
	return err
}

// DeleteCheckpoint deletes a checkpoint
func (r *ScanJobRepo) DeleteCheckpoint(ctx context.Context, jobID string) error {
	query := `DELETE FROM scan_checkpoints WHERE job_id = ?`
	_, err := r.db.ExecContext(ctx, query, jobID)
	return err
}
