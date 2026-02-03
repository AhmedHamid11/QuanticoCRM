package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// MigrationRepo handles migration run database operations
type MigrationRepo struct {
	db db.DBConn
}

// NewMigrationRepo creates a new MigrationRepo
func NewMigrationRepo(dbConn db.DBConn) *MigrationRepo {
	return &MigrationRepo{db: dbConn}
}

// CreateRun inserts a new migration run record
func (r *MigrationRepo) CreateRun(ctx context.Context, run *entity.MigrationRun) error {
	// Generate ID if not set
	if run.ID == "" {
		run.ID = sfid.New(sfid.PrefixMigrationRun)
	}

	query := `
		INSERT INTO migration_runs
		(id, org_id, org_name, from_version, to_version, status, error_message, started_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var completedAt interface{}
	if run.CompletedAt != nil {
		completedAt = run.CompletedAt.UTC().Format(time.RFC3339)
	}

	_, err := r.db.ExecContext(ctx, query,
		run.ID,
		run.OrgID,
		run.OrgName,
		run.FromVersion,
		run.ToVersion,
		run.Status,
		run.ErrorMessage,
		run.StartedAt.UTC().Format(time.RFC3339),
		completedAt,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// UpdateRunStatus updates status and completion time
func (r *MigrationRepo) UpdateRunStatus(ctx context.Context, runID, status, errorMsg string) error {
	query := `
		UPDATE migration_runs
		SET status = ?, error_message = ?, completed_at = ?
		WHERE id = ?
	`
	completedAt := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, query, status, errorMsg, completedAt, runID)
	return err
}

// GetFailedRuns returns all failed migration runs (most recent per org)
func (r *MigrationRepo) GetFailedRuns(ctx context.Context) ([]entity.MigrationRun, error) {
	query := `
		SELECT m.id, m.org_id, m.org_name, m.from_version, m.to_version,
		       m.status, m.error_message, m.started_at, m.completed_at, m.created_at
		FROM migration_runs m
		INNER JOIN (
			SELECT org_id, MAX(started_at) as max_started
			FROM migration_runs
			GROUP BY org_id
		) latest ON m.org_id = latest.org_id AND m.started_at = latest.max_started
		WHERE m.status = 'failed'
		ORDER BY m.started_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRuns(rows)
}

// GetRunsByOrg returns migration runs for a specific org
func (r *MigrationRepo) GetRunsByOrg(ctx context.Context, orgID string, limit int) ([]entity.MigrationRun, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, org_id, org_name, from_version, to_version,
		       status, error_message, started_at, completed_at, created_at
		FROM migration_runs
		WHERE org_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRuns(rows)
}

// GetLastRunTime returns the timestamp of the most recent migration run
func (r *MigrationRepo) GetLastRunTime(ctx context.Context) (*time.Time, error) {
	query := `SELECT MAX(started_at) FROM migration_runs`

	var lastRunStr sql.NullString
	err := r.db.QueryRowContext(ctx, query).Scan(&lastRunStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if !lastRunStr.Valid || lastRunStr.String == "" {
		return nil, nil
	}

	t, err := time.Parse(time.RFC3339, lastRunStr.String)
	if err != nil {
		return nil, nil
	}
	return &t, nil
}

// scanRuns scans rows into MigrationRun slice
func (r *MigrationRepo) scanRuns(rows *sql.Rows) ([]entity.MigrationRun, error) {
	var runs []entity.MigrationRun
	for rows.Next() {
		var run entity.MigrationRun
		var startedAtStr, createdAtStr string
		var completedAtStr, errorMsg sql.NullString

		err := rows.Scan(
			&run.ID, &run.OrgID, &run.OrgName, &run.FromVersion, &run.ToVersion,
			&run.Status, &errorMsg, &startedAtStr, &completedAtStr, &createdAtStr,
		)
		if err != nil {
			return nil, err
		}

		run.StartedAt, _ = time.Parse(time.RFC3339, startedAtStr)
		run.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		if completedAtStr.Valid {
			t, _ := time.Parse(time.RFC3339, completedAtStr.String)
			run.CompletedAt = &t
		}
		if errorMsg.Valid {
			run.ErrorMessage = errorMsg.String
		}

		runs = append(runs, run)
	}
	return runs, rows.Err()
}
