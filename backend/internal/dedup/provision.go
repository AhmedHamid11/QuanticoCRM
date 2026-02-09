package dedup

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/fastcrm/backend/internal/db"
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
			blocking_strategy TEXT NOT NULL DEFAULT 'multi',
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

	// Upgrade existing rules to multi blocking strategy if they have an invalid or narrow strategy
	// soundex-only misses email/phone-based duplicates; empty/none are invalid
	_, _ = conn.ExecContext(ctx, `UPDATE matching_rules SET blocking_strategy = 'multi' WHERE blocking_strategy NOT IN ('multi', 'prefix', 'exact')`)

	log.Printf("[DEDUP] Auto-provisioned dedup tables and blocking key columns on tenant DB")
	return nil
}

// BackfillBlockingKeys populates blocking key columns for all contacts that have NULL keys.
// This ensures existing records (created before the dedup feature) are findable as candidates.
// Safe to call multiple times — caches by connection pointer and only processes NULL records.
func BackfillBlockingKeys(ctx context.Context, conn db.DBConn, blocker *Blocker) error {
	connKey := fmt.Sprintf("%p", conn)
	if _, ok := backfilledDBs.Load(connKey); ok {
		return nil
	}

	// Query contacts that haven't been backfilled yet (dedup_email_domain IS NULL as marker)
	// Use a single query to fetch id + the fields needed for blocking key generation
	rows, err := conn.QueryContext(ctx, `SELECT id, last_name, email_address, phone_number
		FROM contacts WHERE dedup_email_domain IS NULL LIMIT 5000`)
	if err != nil {
		if IsSchemaError(err) {
			// Table or columns don't exist yet — nothing to backfill
			backfilledDBs.Store(connKey, true)
			return nil
		}
		return fmt.Errorf("failed to query contacts for backfill: %w", err)
	}
	defer rows.Close()

	type backfillRecord struct {
		id        string
		lastName  string
		email     string
		phone     string
	}

	var records []backfillRecord
	for rows.Next() {
		var r backfillRecord
		var lastName, email, phone *string
		if err := rows.Scan(&r.id, &lastName, &email, &phone); err != nil {
			return fmt.Errorf("failed to scan contact for backfill: %w", err)
		}
		if lastName != nil {
			r.lastName = *lastName
		}
		if email != nil {
			r.email = *email
		}
		if phone != nil {
			r.phone = *phone
		}
		records = append(records, r)
	}

	if len(records) == 0 {
		backfilledDBs.Store(connKey, true)
		return nil
	}

	log.Printf("[DEDUP] Backfilling blocking keys for %d contacts", len(records))

	updated := 0
	for _, r := range records {
		// Build a record map matching the format GenerateBlockingKeys expects (camelCase)
		recordMap := map[string]interface{}{
			"lastName":     r.lastName,
			"emailAddress": r.email,
			"phoneNumber":  r.phone,
		}

		keys := blocker.GenerateBlockingKeys(recordMap)

		_, err := conn.ExecContext(ctx, `UPDATE contacts SET
			dedup_last_name_soundex = ?,
			dedup_last_name_prefix = ?,
			dedup_email_domain = ?,
			dedup_phone_e164 = ?
			WHERE id = ?`,
			keys.LastNameSoundex, keys.LastNamePrefix,
			keys.EmailDomain, keys.PhoneE164, r.id)
		if err != nil {
			log.Printf("[DEDUP] Failed to backfill blocking keys for %s: %v", r.id, err)
			continue
		}
		updated++
	}

	log.Printf("[DEDUP] Backfilled blocking keys for %d/%d contacts", updated, len(records))
	backfilledDBs.Store(connKey, true)
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
