package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/sfid"
)

// DeltaKeyRepo handles CRUD operations for delta keys in tenant databases
type DeltaKeyRepo struct {
	db db.DBConn
}

// NewDeltaKeyRepo creates a new DeltaKeyRepo
func NewDeltaKeyRepo(conn db.DBConn) *DeltaKeyRepo {
	return &DeltaKeyRepo{db: conn}
}

// DeltaKeyEntry represents a delta key entry to insert
type DeltaKeyEntry struct {
	UniqueKey string
	RecordID  string
}

// ExistsBatch checks which unique keys already exist in the delta index
func (r *DeltaKeyRepo) ExistsBatch(ctx context.Context, tenantDB db.DBConn, mirrorID string, uniqueKeys []string) (map[string]bool, error) {
	if len(uniqueKeys) == 0 {
		return map[string]bool{}, nil
	}

	// Build IN clause with placeholders
	placeholders := make([]string, len(uniqueKeys))
	args := []interface{}{mirrorID}
	for i, key := range uniqueKeys {
		placeholders[i] = "?"
		args = append(args, key)
	}

	query := fmt.Sprintf(`
		SELECT unique_key
		FROM ingest_delta_keys
		WHERE mirror_id = ? AND unique_key IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := tenantDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query existing delta keys: %w", err)
	}
	defer rows.Close()

	// Build result map
	exists := make(map[string]bool)
	for rows.Next() {
		var uniqueKey string
		if err := rows.Scan(&uniqueKey); err != nil {
			return nil, fmt.Errorf("scan unique key: %w", err)
		}
		exists[uniqueKey] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delta keys: %w", err)
	}

	return exists, nil
}

// InsertBatch inserts multiple delta key entries in a transaction
func (r *DeltaKeyRepo) InsertBatch(ctx context.Context, tenantDB db.DBConn, orgID, mirrorID string, entries []DeltaKeyEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := tenantDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)

	for _, entry := range entries {
		id := sfid.NewDeltaKey()

		// Use INSERT OR IGNORE to handle race conditions
		_, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO ingest_delta_keys (id, org_id, mirror_id, unique_key, record_id, ingested_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, id, orgID, mirrorID, entry.UniqueKey, entry.RecordID, now)

		if err != nil {
			return fmt.Errorf("insert delta key: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// DeleteByMirror deletes all delta keys for a mirror
func (r *DeltaKeyRepo) DeleteByMirror(ctx context.Context, tenantDB db.DBConn, mirrorID string) error {
	_, err := tenantDB.ExecContext(ctx, "DELETE FROM ingest_delta_keys WHERE mirror_id = ?", mirrorID)
	if err != nil {
		return fmt.Errorf("delete delta keys: %w", err)
	}
	return nil
}

// CountByMirror returns the total count of delta keys for a mirror
func (r *DeltaKeyRepo) CountByMirror(ctx context.Context, tenantDB db.DBConn, mirrorID string) (int, error) {
	var count int
	err := tenantDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM ingest_delta_keys
		WHERE mirror_id = ?
	`, mirrorID).Scan(&count)

	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("count delta keys: %w", err)
	}

	return count, nil
}
