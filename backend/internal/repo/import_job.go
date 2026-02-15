package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/fastcrm/backend/internal/entity"
)

// ImportJobRepo handles CRUD for import_jobs and import_dedup_decisions in tenant databases
type ImportJobRepo struct{}

// NewImportJobRepo creates a new ImportJobRepo
func NewImportJobRepo() *ImportJobRepo {
	return &ImportJobRepo{}
}

// CreateJob inserts a new import_jobs row
func (r *ImportJobRepo) CreateJob(ctx context.Context, tenantDB *sql.DB, job *entity.ImportJob) error {
	_, err := tenantDB.ExecContext(ctx, `
		INSERT INTO import_jobs (id, org_id, entity_type, external_id_field, total_rows, created_count, updated_count, skipped_count, merged_count, failed_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, job.ID, job.OrgID, job.EntityType, job.ExternalIdField,
		job.TotalRows, job.CreatedCount, job.UpdatedCount, job.SkippedCount, job.MergedCount, job.FailedCount)
	if err != nil {
		return fmt.Errorf("insert import job: %w", err)
	}
	return nil
}

// GetJob retrieves an import job by ID
func (r *ImportJobRepo) GetJob(ctx context.Context, tenantDB *sql.DB, orgID, jobID string) (*entity.ImportJob, error) {
	var job entity.ImportJob
	var externalIdField sql.NullString

	err := tenantDB.QueryRowContext(ctx, `
		SELECT id, org_id, entity_type, external_id_field, total_rows, created_count, updated_count, skipped_count, merged_count, failed_count, created_at
		FROM import_jobs
		WHERE id = ? AND org_id = ?
	`, jobID, orgID).Scan(
		&job.ID, &job.OrgID, &job.EntityType, &externalIdField,
		&job.TotalRows, &job.CreatedCount, &job.UpdatedCount, &job.SkippedCount, &job.MergedCount, &job.FailedCount,
		&job.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get import job: %w", err)
	}

	if externalIdField.Valid {
		job.ExternalIdField = externalIdField.String
	}

	return &job, nil
}

// ListJobs returns a paginated list of import jobs for an org, with optional filters
func (r *ImportJobRepo) ListJobs(ctx context.Context, tenantDB *sql.DB, orgID, entityType, since string, page, pageSize int) ([]*entity.ImportJob, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Build WHERE clause
	where := "org_id = ?"
	args := []interface{}{orgID}

	if entityType != "" {
		where += " AND entity_type = ?"
		args = append(args, entityType)
	}
	if since != "" {
		where += " AND created_at >= ?"
		args = append(args, since)
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM import_jobs WHERE %s", where)
	if err := tenantDB.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count import jobs: %w", err)
	}

	// Fetch page
	offset := (page - 1) * pageSize
	listArgs := append(args, pageSize, offset)
	listQuery := fmt.Sprintf(`
		SELECT id, org_id, entity_type, external_id_field, total_rows, created_count, updated_count, skipped_count, merged_count, failed_count, created_at
		FROM import_jobs
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, where)

	rows, err := tenantDB.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list import jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*entity.ImportJob
	for rows.Next() {
		var job entity.ImportJob
		var externalIdField sql.NullString

		if err := rows.Scan(
			&job.ID, &job.OrgID, &job.EntityType, &externalIdField,
			&job.TotalRows, &job.CreatedCount, &job.UpdatedCount, &job.SkippedCount, &job.MergedCount, &job.FailedCount,
			&job.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan import job: %w", err)
		}

		if externalIdField.Valid {
			job.ExternalIdField = externalIdField.String
		}

		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate import jobs: %w", err)
	}

	if jobs == nil {
		jobs = []*entity.ImportJob{}
	}

	return jobs, total, nil
}

// SaveDecisions bulk-inserts dedup decisions within a single transaction
func (r *ImportJobRepo) SaveDecisions(ctx context.Context, tenantDB *sql.DB, decisions []entity.ImportDedupDecision) error {
	if len(decisions) == 0 {
		return nil
	}

	tx, err := tenantDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Batch insert using multi-row VALUES
	const batchSize = 50
	for i := 0; i < len(decisions); i += batchSize {
		end := i + batchSize
		if end > len(decisions) {
			end = len(decisions)
		}
		batch := decisions[i:end]

		var placeholders []string
		var args []interface{}
		for _, d := range batch {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)")
			args = append(args, d.ID, d.OrgID, d.ImportJobID, d.DecisionType, d.Action,
				d.KeptExternalID, d.DiscardedExternalID, d.MatchField, d.MatchValue, d.MatchedRecordID)
		}

		query := fmt.Sprintf(`
			INSERT INTO import_dedup_decisions (id, org_id, import_job_id, decision_type, action, kept_external_id, discarded_external_id, match_field, match_value, matched_record_id, created_at)
			VALUES %s
		`, strings.Join(placeholders, ", "))

		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("insert dedup decisions batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dedup decisions: %w", err)
	}

	return nil
}

// GetDecisions returns all dedup decisions for a specific import job
func (r *ImportJobRepo) GetDecisions(ctx context.Context, tenantDB *sql.DB, orgID, jobID string) ([]entity.ImportDedupDecision, error) {
	rows, err := tenantDB.QueryContext(ctx, `
		SELECT id, org_id, import_job_id, decision_type, action, kept_external_id, discarded_external_id, match_field, match_value, matched_record_id, created_at
		FROM import_dedup_decisions
		WHERE import_job_id = ? AND org_id = ?
		ORDER BY created_at ASC
	`, jobID, orgID)
	if err != nil {
		return nil, fmt.Errorf("get dedup decisions: %w", err)
	}
	defer rows.Close()

	var decisions []entity.ImportDedupDecision
	for rows.Next() {
		var d entity.ImportDedupDecision
		var keptExt, discardedExt, matchField, matchValue, matchedRecID sql.NullString

		if err := rows.Scan(
			&d.ID, &d.OrgID, &d.ImportJobID, &d.DecisionType, &d.Action,
			&keptExt, &discardedExt, &matchField, &matchValue, &matchedRecID,
			&d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dedup decision: %w", err)
		}

		if keptExt.Valid {
			d.KeptExternalID = keptExt.String
		}
		if discardedExt.Valid {
			d.DiscardedExternalID = discardedExt.String
		}
		if matchField.Valid {
			d.MatchField = matchField.String
		}
		if matchValue.Valid {
			d.MatchValue = matchValue.String
		}
		if matchedRecID.Valid {
			d.MatchedRecordID = matchedRecID.String
		}

		decisions = append(decisions, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dedup decisions: %w", err)
	}

	if decisions == nil {
		decisions = []entity.ImportDedupDecision{}
	}

	return decisions, nil
}
