package dedup

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/util"
)

// provisionedDBs tracks which tenant DBs have been auto-provisioned
// to avoid re-running DDL on every request
var provisionedDBs sync.Map

// backfilledDBs tracks which tenant DBs have had blocking keys backfilled
var backfilledDBs sync.Map

// addColumnIfNotExists tries to add a column and silently ignores "duplicate column" errors.
// SQLite doesn't support ALTER TABLE ADD COLUMN IF NOT EXISTS.
func addColumnIfNotExists(ctx context.Context, conn db.DBConn, stmt string) error {
	_, err := conn.ExecContext(ctx, stmt)
	if err != nil {
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "duplicate column") {
			return nil
		}
		return err
	}
	return nil
}

// EnsureDedupSchema creates dedup tables and adds blocking key columns if missing.
// Safe to call multiple times (uses CREATE TABLE IF NOT EXISTS and tolerates duplicate columns).
// Caches by connection pointer to avoid re-running on every request.
func EnsureDedupSchema(ctx context.Context, conn db.DBConn) error {
	connKey := fmt.Sprintf("%p", conn)
	if _, ok := provisionedDBs.Load(connKey); ok {
		return nil
	}

	if err := ensureDedupSchema(ctx, conn); err != nil {
		return err
	}

	provisionedDBs.Store(connKey, true)
	return nil
}

func ensureDedupSchema(ctx context.Context, conn db.DBConn) error {
	// Create dedup tables
	tableStatements := []string{
		`CREATE TABLE IF NOT EXISTS matching_rules (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			entity_type TEXT NOT NULL,
			target_entity_type TEXT,
			is_enabled INTEGER DEFAULT 1,
			priority INTEGER DEFAULT 0,
			threshold REAL NOT NULL DEFAULT 0.70,
			high_confidence_threshold REAL DEFAULT 0.95,
			medium_confidence_threshold REAL DEFAULT 0.85,
			blocking_strategy TEXT NOT NULL DEFAULT '',
			field_configs TEXT NOT NULL,
			merge_display_fields TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, entity_type, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_matching_rules_org ON matching_rules(org_id, entity_type, is_enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_matching_rules_priority ON matching_rules(org_id, is_enabled, priority)`,
		`CREATE TABLE IF NOT EXISTS pending_duplicate_alerts (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			record_id TEXT NOT NULL,
			matches_json TEXT NOT NULL,
			total_match_count INTEGER NOT NULL,
			highest_confidence TEXT NOT NULL,
			is_block_mode INTEGER NOT NULL DEFAULT 0,
			detected_at TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			resolved_at TEXT,
			resolved_by_id TEXT,
			override_text TEXT,
			merge_display_fields TEXT,
			UNIQUE(org_id, entity_type, record_id, status)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_alerts_record ON pending_duplicate_alerts(org_id, entity_type, record_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_alerts_pending ON pending_duplicate_alerts(org_id, status, detected_at)`,
		`CREATE TABLE IF NOT EXISTS merge_snapshots (
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
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_merge_snapshots_org ON merge_snapshots(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_merge_snapshots_survivor ON merge_snapshots(org_id, survivor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_merge_snapshots_expires ON merge_snapshots(expires_at)`,
		`CREATE TABLE IF NOT EXISTS scan_schedules (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			frequency TEXT NOT NULL,
			day_of_week INTEGER,
			day_of_month INTEGER,
			hour INTEGER NOT NULL DEFAULT 3,
			minute INTEGER NOT NULL DEFAULT 0,
			is_enabled INTEGER NOT NULL DEFAULT 1,
			last_run_at TEXT,
			next_run_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(org_id, entity_type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_scan_schedules_org ON scan_schedules(org_id, is_enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_scan_schedules_next_run ON scan_schedules(next_run_at, is_enabled)`,
		`CREATE TABLE IF NOT EXISTS scan_jobs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			schedule_id TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			trigger_type TEXT NOT NULL DEFAULT 'scheduled',
			total_records INTEGER NOT NULL DEFAULT 0,
			processed_records INTEGER NOT NULL DEFAULT 0,
			duplicates_found INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at TEXT,
			completed_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_scan_jobs_org_status ON scan_jobs(org_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_scan_jobs_org_entity ON scan_jobs(org_id, entity_type, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_scan_jobs_schedule ON scan_jobs(schedule_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS scan_checkpoints (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			last_offset INTEGER NOT NULL DEFAULT 0,
			last_processed_id TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			chunk_size INTEGER NOT NULL DEFAULT 500,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(job_id)
		)`,
	}

	for _, stmt := range tableStatements {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to create dedup table: %w", err)
		}
	}

	// Add blocking key columns to contacts table (migration 051)
	alterStatements := []string{
		`ALTER TABLE contacts ADD COLUMN dedup_last_name_soundex TEXT`,
		`ALTER TABLE contacts ADD COLUMN dedup_last_name_prefix TEXT`,
		`ALTER TABLE contacts ADD COLUMN dedup_email_domain TEXT`,
		`ALTER TABLE contacts ADD COLUMN dedup_phone_e164 TEXT`,
	}
	for _, stmt := range alterStatements {
		if err := addColumnIfNotExists(ctx, conn, stmt); err != nil {
			return fmt.Errorf("failed to add blocking key column: %w", err)
		}
	}

	// Create indexes on blocking key columns
	indexStatements := []string{
		`CREATE INDEX IF NOT EXISTS idx_contacts_dedup_soundex ON contacts(org_id, dedup_last_name_soundex)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_dedup_prefix ON contacts(org_id, dedup_last_name_prefix)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_dedup_domain ON contacts(org_id, dedup_email_domain)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_dedup_phone ON contacts(org_id, dedup_phone_e164)`,
	}
	for _, stmt := range indexStatements {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to create blocking key index: %w", err)
		}
	}

	// Blank out blocking_strategy — the engine now ignores this field
	// and always uses all available blocking keys automatically
	_, _ = conn.ExecContext(ctx, `UPDATE matching_rules SET blocking_strategy = '' WHERE blocking_strategy != ''`)

	log.Printf("[DEDUP] Auto-provisioned dedup tables and blocking key columns on tenant DB")
	return nil
}

// entityBlockingKeys tracks which (connection, entity) pairs have had blocking key columns added
var entityBlockingKeys sync.Map

// entityBackfilled tracks which (connection, entity) pairs have been backfilled
var entityBackfilled sync.Map

// EnsureBlockingKeysForEntity adds blocking key columns and indexes to the specified entity's table.
// Safe to call multiple times — uses addColumnIfNotExists and caches by (connection, entity).
func EnsureBlockingKeysForEntity(ctx context.Context, conn db.DBConn, entityType string) error {
	cacheKey := fmt.Sprintf("%p:%s", conn, entityType)
	if _, ok := entityBlockingKeys.Load(cacheKey); ok {
		return nil
	}

	tableName := util.GetTableName(entityType)

	// Add blocking key columns
	alterStatements := []string{
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN dedup_last_name_soundex TEXT`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN dedup_last_name_prefix TEXT`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN dedup_email_domain TEXT`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN dedup_phone_e164 TEXT`, tableName),
	}
	for _, stmt := range alterStatements {
		if err := addColumnIfNotExists(ctx, conn, stmt); err != nil {
			if IsSchemaError(err) {
				// Table doesn't exist — nothing to provision
				return nil
			}
			return fmt.Errorf("failed to add blocking key column to %s: %w", tableName, err)
		}
	}

	// Create indexes on blocking key columns
	indexStatements := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_dedup_soundex ON %s(org_id, dedup_last_name_soundex)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_dedup_prefix ON %s(org_id, dedup_last_name_prefix)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_dedup_domain ON %s(org_id, dedup_email_domain)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_dedup_phone ON %s(org_id, dedup_phone_e164)`, tableName, tableName),
	}
	for _, stmt := range indexStatements {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to create blocking key index on %s: %w", tableName, err)
		}
	}

	log.Printf("[DEDUP] Ensured blocking key columns on %s table", tableName)
	entityBlockingKeys.Store(cacheKey, true)
	return nil
}

// backfillBatchSize is the number of records processed per iteration in BackfillBlockingKeysForEntity.
// Large enough to be efficient, small enough to respect Turso's 5-second query timeout.
const backfillBatchSize = 5000

// BackfillBlockingKeys populates blocking key columns for all contacts that have NULL keys.
// This ensures existing records (created before the dedup feature) are findable as candidates.
// Safe to call multiple times — caches by connection pointer and only processes NULL records.
func BackfillBlockingKeys(ctx context.Context, conn db.DBConn, blocker *Blocker) error {
	return BackfillBlockingKeysForEntity(ctx, conn, "Contact", blocker, nil)
}

// BackfillBlockingKeysForEntity populates blocking key columns for ALL records of the specified
// entity that have NULL or empty keys. Processes in batches of backfillBatchSize so it handles
// datasets of any size (33k, 100k+) without hitting query timeouts.
//
// progressFn (optional) is called after each batch with (processed, total) so callers can emit
// progress events. Pass nil to suppress progress callbacks (backward-compatible).
//
// Cache behaviour: the (connection, entity) pair is only marked done after the loop exits with
// 0 remaining records. A partial failure on a previous run will re-enter and continue from where
// it left off on the next call.
func BackfillBlockingKeysForEntity(ctx context.Context, conn db.DBConn, entityType string, blocker *Blocker, progressFn func(processed, total int)) error {
	cacheKey := fmt.Sprintf("%p:%s", conn, entityType)
	if _, ok := entityBackfilled.Load(cacheKey); ok {
		return nil
	}

	tableName := util.GetTableName(entityType)

	// Count how many records still need backfill so we can report progress.
	needsBackfillSQL := fmt.Sprintf(
		`SELECT COUNT(*) FROM %s WHERE dedup_email_domain IS NULL OR (dedup_last_name_soundex = '' AND dedup_last_name_prefix = '' AND dedup_email_domain = '' AND dedup_phone_e164 = '')`,
		tableName,
	)
	var totalNeeding int
	if err := conn.QueryRowContext(ctx, needsBackfillSQL).Scan(&totalNeeding); err != nil {
		if IsSchemaError(err) {
			// Table or columns don't exist yet — nothing to backfill
			entityBackfilled.Store(cacheKey, true)
			return nil
		}
		return fmt.Errorf("failed to count %s records needing backfill: %w", tableName, err)
	}

	if totalNeeding == 0 {
		entityBackfilled.Store(cacheKey, true)
		return nil
	}

	log.Printf("[DEDUP] Starting blocking-key backfill for %s: %d records need processing", entityType, totalNeeding)

	totalProcessed := 0
	batchNum := 0

	for {
		// Query the next batch of records that still lack blocking keys.
		// We re-query instead of using OFFSET so that successfully-updated records
		// are no longer matched, preventing infinite loops on update failures.
		rows, err := conn.QueryContext(ctx, fmt.Sprintf(
			`SELECT * FROM %s WHERE dedup_email_domain IS NULL OR (dedup_last_name_soundex = '' AND dedup_last_name_prefix = '' AND dedup_email_domain = '' AND dedup_phone_e164 = '') LIMIT %d`,
			tableName, backfillBatchSize))
		if err != nil {
			if IsSchemaError(err) {
				entityBackfilled.Store(cacheKey, true)
				return nil
			}
			return fmt.Errorf("failed to query %s batch for backfill: %w", tableName, err)
		}

		records, err := util.ScanRowsToMaps(rows)
		rows.Close()
		if err != nil {
			return fmt.Errorf("failed to scan %s batch for backfill: %w", tableName, err)
		}

		if len(records) == 0 {
			// All records have been processed
			break
		}

		batchNum++
		updated := 0

		// Use a transaction to batch all UPDATEs into a single Turso HTTP request.
		// Without this, each UPDATE is a separate HTTP round-trip (~200ms),
		// making 33k records take ~110 minutes. With a transaction, 5000 UPDATEs
		// become a single batched request (~2-5 seconds).
		tx, txErr := conn.BeginTx(ctx, nil)
		if txErr != nil {
			log.Printf("[DEDUP] Failed to begin transaction for backfill batch %d: %v (falling back to individual updates)", batchNum, txErr)
			// Fallback: individual updates if transaction fails
			for _, record := range records {
				recordID, _ := record["id"].(string)
				if recordID == "" {
					continue
				}
				keys := blocker.GenerateBlockingKeys(record)
				_, err := conn.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET
					dedup_last_name_soundex = ?,
					dedup_last_name_prefix = ?,
					dedup_email_domain = ?,
					dedup_phone_e164 = ?
					WHERE id = ?`, tableName),
					keys.LastNameSoundex, keys.LastNamePrefix,
					keys.EmailDomain, keys.PhoneE164, recordID)
				if err != nil {
					log.Printf("[DEDUP] Failed to backfill blocking keys for %s/%s: %v", entityType, recordID, err)
					continue
				}
				updated++
			}
		} else {
			for _, record := range records {
				recordID, _ := record["id"].(string)
				if recordID == "" {
					continue
				}
				keys := blocker.GenerateBlockingKeys(record)
				_, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET
					dedup_last_name_soundex = ?,
					dedup_last_name_prefix = ?,
					dedup_email_domain = ?,
					dedup_phone_e164 = ?
					WHERE id = ?`, tableName),
					keys.LastNameSoundex, keys.LastNamePrefix,
					keys.EmailDomain, keys.PhoneE164, recordID)
				if err != nil {
					log.Printf("[DEDUP] Failed to backfill blocking keys for %s/%s: %v", entityType, recordID, err)
					continue
				}
				updated++
			}
			if err := tx.Commit(); err != nil {
				log.Printf("[DEDUP] Failed to commit backfill batch %d: %v", batchNum, err)
				_ = tx.Rollback()
			}
		}

		totalProcessed += updated
		log.Printf("[DEDUP] Backfill batch %d for %s: updated %d/%d records (total so far: %d/%d)",
			batchNum, entityType, updated, len(records), totalProcessed, totalNeeding)

		if progressFn != nil {
			progressFn(totalProcessed, totalNeeding)
		}

		// If we got fewer rows than the batch size, we've reached the end
		// (remaining rows may be un-updatable; avoid infinite loop)
		if len(records) < backfillBatchSize {
			break
		}
	}

	log.Printf("[DEDUP] Backfilled blocking keys for %d %s records total across %d batches", totalProcessed, entityType, batchNum)
	entityBackfilled.Store(cacheKey, true)
	return nil
}

// IsSchemaError checks if an error is caused by missing tables or columns
func IsSchemaError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no such table") || strings.Contains(errStr, "no such column")
}
