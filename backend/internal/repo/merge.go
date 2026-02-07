package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
	"github.com/fastcrm/backend/internal/util"
)

// MergeRepo handles database operations for merge snapshots
type MergeRepo struct {
	db db.DBConn
}

// NewMergeRepo creates a new MergeRepo
func NewMergeRepo(conn db.DBConn) *MergeRepo {
	return &MergeRepo{db: conn}
}

// WithDB returns a new repo with a different DB connection (for multi-tenant)
func (r *MergeRepo) WithDB(conn db.DBConn) *MergeRepo {
	return &MergeRepo{db: conn}
}

// EnsureTableExists creates the merge_snapshots table if it doesn't exist
// This is called defensively to handle cases where migrations haven't run
func (r *MergeRepo) EnsureTableExists(ctx context.Context) error {
	// Check if table exists
	var tableExists int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='merge_snapshots'",
	).Scan(&tableExists); err != nil {
		return fmt.Errorf("failed to check merge_snapshots table: %w", err)
	}

	if tableExists > 0 {
		return nil // Table already exists
	}

	// Create the table
	createSQL := `
		CREATE TABLE IF NOT EXISTS merge_snapshots (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			survivor_id TEXT NOT NULL,
			survivor_before TEXT NOT NULL,
			duplicate_ids TEXT NOT NULL,
			duplicate_snapshots TEXT NOT NULL,
			related_record_fks TEXT NOT NULL,
			merged_by_id TEXT NOT NULL,
			consumed_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at TEXT NOT NULL
		)
	`
	if _, err := r.db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("failed to create merge_snapshots table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_merge_snapshots_org_survivor ON merge_snapshots(org_id, survivor_id)",
		"CREATE INDEX IF NOT EXISTS idx_merge_snapshots_org_created ON merge_snapshots(org_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_merge_snapshots_expires ON merge_snapshots(org_id, expires_at)",
	}
	for _, idx := range indexes {
		if _, err := r.db.ExecContext(ctx, idx); err != nil {
			// Log but don't fail - indexes are optional for basic functionality
			fmt.Printf("Warning: failed to create merge_snapshots index: %v\n", err)
		}
	}

	return nil
}

// Save inserts a new merge snapshot
func (r *MergeRepo) Save(ctx context.Context, snapshot *entity.MergeSnapshot) error {
	// Generate ID if not set
	if snapshot.ID == "" {
		snapshot.ID = sfid.NewMergeSnapshot()
	}

	// Set created_at if not set
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}

	// Set expires_at if not set (30 days from now)
	if snapshot.ExpiresAt.IsZero() {
		snapshot.ExpiresAt = snapshot.CreatedAt.Add(30 * 24 * time.Hour)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO merge_snapshots (
			id, org_id, entity_type, survivor_id, survivor_before,
			duplicate_ids, duplicate_snapshots, related_record_fks,
			merged_by_id, consumed_at, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.ID,
		snapshot.OrgID,
		snapshot.EntityType,
		snapshot.SurvivorID,
		snapshot.SurvivorBefore,
		snapshot.DuplicateIDs,
		snapshot.DuplicateSnapshots,
		snapshot.RelatedRecordFKs,
		snapshot.MergedByID,
		snapshot.ConsumedAt,
		snapshot.CreatedAt.Format(time.RFC3339),
		snapshot.ExpiresAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to save merge snapshot: %w", err)
	}

	return nil
}

// GetByID retrieves a merge snapshot by ID
func (r *MergeRepo) GetByID(ctx context.Context, orgID, snapshotID string) (*entity.MergeSnapshot, error) {
	var snapshot entity.MergeSnapshot
	var createdAtStr, expiresAtStr string
	var consumedAtStr sql.NullString

	err := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, entity_type, survivor_id, survivor_before,
		        duplicate_ids, duplicate_snapshots, related_record_fks,
		        merged_by_id, consumed_at, created_at, expires_at
		 FROM merge_snapshots
		 WHERE org_id = ? AND id = ?`,
		orgID, snapshotID,
	).Scan(
		&snapshot.ID,
		&snapshot.OrgID,
		&snapshot.EntityType,
		&snapshot.SurvivorID,
		&snapshot.SurvivorBefore,
		&snapshot.DuplicateIDs,
		&snapshot.DuplicateSnapshots,
		&snapshot.RelatedRecordFKs,
		&snapshot.MergedByID,
		&consumedAtStr,
		&createdAtStr,
		&expiresAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get merge snapshot: %w", err)
	}

	// Parse timestamps
	snapshot.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	snapshot.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAtStr)

	if consumedAtStr.Valid {
		snapshot.ConsumedAt = &consumedAtStr.String
	}

	return &snapshot, nil
}

// GetBySurvivor retrieves all merge snapshots for a survivor record
func (r *MergeRepo) GetBySurvivor(ctx context.Context, orgID, survivorID string) ([]entity.MergeSnapshot, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, entity_type, survivor_id, survivor_before,
		        duplicate_ids, duplicate_snapshots, related_record_fks,
		        merged_by_id, consumed_at, created_at, expires_at
		 FROM merge_snapshots
		 WHERE org_id = ? AND survivor_id = ?
		 ORDER BY created_at DESC`,
		orgID, survivorID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query merge snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []entity.MergeSnapshot
	for rows.Next() {
		var snapshot entity.MergeSnapshot
		var createdAtStr, expiresAtStr string
		var consumedAtStr sql.NullString

		err := rows.Scan(
			&snapshot.ID,
			&snapshot.OrgID,
			&snapshot.EntityType,
			&snapshot.SurvivorID,
			&snapshot.SurvivorBefore,
			&snapshot.DuplicateIDs,
			&snapshot.DuplicateSnapshots,
			&snapshot.RelatedRecordFKs,
			&snapshot.MergedByID,
			&consumedAtStr,
			&createdAtStr,
			&expiresAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan merge snapshot: %w", err)
		}

		// Parse timestamps
		snapshot.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		snapshot.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAtStr)

		if consumedAtStr.Valid {
			snapshot.ConsumedAt = &consumedAtStr.String
		}

		snapshots = append(snapshots, snapshot)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if snapshots == nil {
		snapshots = []entity.MergeSnapshot{}
	}

	return snapshots, nil
}

// ListByOrg retrieves merge snapshots for an organization with pagination
func (r *MergeRepo) ListByOrg(ctx context.Context, orgID string, page, pageSize int) ([]entity.MergeSnapshot, int, error) {
	// Set defaults
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	// Get total count
	var total int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM merge_snapshots WHERE org_id = ?",
		orgID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count merge snapshots: %w", err)
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, entity_type, survivor_id, survivor_before,
		        duplicate_ids, duplicate_snapshots, related_record_fks,
		        merged_by_id, consumed_at, created_at, expires_at
		 FROM merge_snapshots
		 WHERE org_id = ?
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		orgID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query merge snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []entity.MergeSnapshot
	for rows.Next() {
		var snapshot entity.MergeSnapshot
		var createdAtStr, expiresAtStr string
		var consumedAtStr sql.NullString

		err := rows.Scan(
			&snapshot.ID,
			&snapshot.OrgID,
			&snapshot.EntityType,
			&snapshot.SurvivorID,
			&snapshot.SurvivorBefore,
			&snapshot.DuplicateIDs,
			&snapshot.DuplicateSnapshots,
			&snapshot.RelatedRecordFKs,
			&snapshot.MergedByID,
			&consumedAtStr,
			&createdAtStr,
			&expiresAtStr,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan merge snapshot: %w", err)
		}

		// Parse timestamps
		snapshot.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		snapshot.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAtStr)

		if consumedAtStr.Valid {
			snapshot.ConsumedAt = &consumedAtStr.String
		}

		snapshots = append(snapshots, snapshot)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	if snapshots == nil {
		snapshots = []entity.MergeSnapshot{}
	}

	return snapshots, total, nil
}

// MarkConsumed marks a snapshot as consumed (undo was performed)
func (r *MergeRepo) MarkConsumed(ctx context.Context, orgID, snapshotID string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := r.db.ExecContext(ctx,
		`UPDATE merge_snapshots
		 SET consumed_at = ?
		 WHERE org_id = ? AND id = ? AND consumed_at IS NULL`,
		now, orgID, snapshotID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark snapshot as consumed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("snapshot not found or already consumed")
	}

	return nil
}

// CleanupExpired deletes expired snapshots that are past their expires_at date
func (r *MergeRepo) CleanupExpired(ctx context.Context, orgID string) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := r.db.ExecContext(ctx,
		`DELETE FROM merge_snapshots
		 WHERE org_id = ? AND expires_at < ?`,
		orgID, now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired snapshots: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to check rows affected: %w", err)
	}

	return int(rows), nil
}

// EnsureArchiveColumns ensures the archived_at column exists on a table
// This is a static utility that checks PRAGMA table_info and adds missing columns
func (r *MergeRepo) EnsureArchiveColumns(ctx context.Context, dbConn db.DBConn, tableName string) error {
	// Check if archived_at column exists
	rows, err := dbConn.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	hasArchivedAt := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == "archived_at" {
			hasArchivedAt = true
			break
		}
	}

	// Add archived_at if missing
	if !hasArchivedAt {
		alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN archived_at TEXT", util.QuoteIdentifier(tableName))
		if _, err := dbConn.ExecContext(ctx, alterSQL); err != nil {
			return fmt.Errorf("failed to add archived_at column: %w", err)
		}
	}

	return nil
}
