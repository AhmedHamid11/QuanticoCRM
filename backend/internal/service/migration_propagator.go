package service

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/migrations"
	"github.com/fastcrm/backend/internal/repo"
)

// MigrationPropagator handles propagating migrations to all org databases
type MigrationPropagator struct {
	masterDB       db.DBConn
	dbManager      *db.Manager
	versionRepo    *repo.VersionRepo
	migrationRepo  *repo.MigrationRepo
	versionService *VersionService
}

// NewMigrationPropagator creates a new MigrationPropagator
func NewMigrationPropagator(
	masterDB db.DBConn,
	dbManager *db.Manager,
	versionRepo *repo.VersionRepo,
	migrationRepo *repo.MigrationRepo,
	versionService *VersionService,
) *MigrationPropagator {
	return &MigrationPropagator{
		masterDB:       masterDB,
		dbManager:      dbManager,
		versionRepo:    versionRepo,
		migrationRepo:  migrationRepo,
		versionService: versionService,
	}
}

// PropagateAll runs migrations on all orgs that have pending migration files.
// This uses the per-file _migrations tracking table in each tenant DB for accuracy,
// rather than relying solely on version comparison which can miss migrations when
// version bumps are forgotten.
func (p *MigrationPropagator) PropagateAll(ctx context.Context) *entity.PropagationResult {
	result := &entity.PropagationResult{
		StartedAt: time.Now(),
		Runs:      []entity.MigrationRun{},
	}

	// Get current platform version (for version stamping after success)
	platformVersion, err := p.versionRepo.GetPlatformVersion(ctx)
	if err != nil {
		log.Printf("[MIGRATION] ERROR: Failed to get platform version: %v", err)
		result.CompletedAt = time.Now()
		return result
	}
	targetVersion := platformVersion.Version
	log.Printf("[MIGRATION] Target platform version: %s", targetVersion)

	// Get the list of all migration files we expect to be applied
	migrationFiles, err := p.getMigrationFiles()
	if err != nil {
		log.Printf("[MIGRATION] ERROR: Failed to list migration files: %v", err)
		result.CompletedAt = time.Now()
		return result
	}
	log.Printf("[MIGRATION] Total migration files available: %d", len(migrationFiles))

	// Get ALL active orgs with tenant databases (not version-gated)
	orgs, err := p.getAllActiveOrgs(ctx)
	if err != nil {
		log.Printf("[MIGRATION] ERROR: Failed to get organizations: %v", err)
		result.CompletedAt = time.Now()
		return result
	}

	if len(orgs) == 0 {
		log.Printf("[MIGRATION] No active organizations found")
		result.CompletedAt = time.Now()
		return result
	}

	log.Printf("[MIGRATION] Checking %d active organizations for pending migrations", len(orgs))

	// Process each org sequentially
	for _, org := range orgs {
		run := p.migrateOrg(ctx, &org, targetVersion)
		result.Runs = append(result.Runs, run)

		switch run.Status {
		case "success":
			result.SuccessCount++
		case "failed":
			result.FailedCount++
		case "skipped":
			result.SkippedCount++
		}
	}

	result.CompletedAt = time.Now()
	duration := result.CompletedAt.Sub(result.StartedAt)
	log.Printf("[MIGRATION] Propagation complete in %v: %d success, %d failed, %d skipped",
		duration, result.SuccessCount, result.FailedCount, result.SkippedCount)

	return result
}

// migrateOrg migrates a single organization
func (p *MigrationPropagator) migrateOrg(ctx context.Context, org *orgInfo, targetVersion string) entity.MigrationRun {
	run := entity.MigrationRun{
		OrgID:       org.ID,
		OrgName:     org.Name,
		FromVersion: org.CurrentVersion,
		ToVersion:   targetVersion,
		Status:      "running",
		StartedAt:   time.Now(),
	}

	// Create initial run record
	if err := p.migrationRepo.CreateRun(ctx, &run); err != nil {
		log.Printf("[MIGRATION] Org=%s ERROR creating run record: %v", org.ID, err)
	}

	// Per-org timeout (2 minutes)
	orgCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// In local mode, skip tenant migrations (shared DB, master migrations handle it)
	if p.dbManager.IsLocalMode() {
		log.Printf("[MIGRATION] Org=%s SKIPPED (local mode - shared database)", org.ID)
		run.Status = "skipped"
		now := time.Now()
		run.CompletedAt = &now
		p.migrationRepo.UpdateRunStatus(ctx, run.ID, run.Status, "")
		return run
	}

	// Skip orgs without a dedicated database URL (they use master DB)
	if org.DatabaseURL == "" {
		log.Printf("[MIGRATION] Org=%s SKIPPED (no dedicated database URL)", org.ID)
		run.Status = "skipped"
		now := time.Now()
		run.CompletedAt = &now
		p.migrationRepo.UpdateRunStatus(ctx, run.ID, run.Status, "")
		return run
	}

	// Get tenant database connection
	tenantDB, err := p.dbManager.GetTenantDBConn(orgCtx, org.ID, org.DatabaseURL, org.DatabaseToken)
	if err != nil {
		run.Status = "failed"
		run.ErrorMessage = fmt.Sprintf("failed to connect: %v", err)
		now := time.Now()
		run.CompletedAt = &now
		log.Printf("[MIGRATION] Org=%s FAILED to connect: %v", org.ID, err)
		p.migrationRepo.UpdateRunStatus(ctx, run.ID, run.Status, run.ErrorMessage)
		return run
	}

	// Apply migrations to tenant database
	appliedCount, err := p.applyMigrations(orgCtx, tenantDB, org)
	if err != nil {
		run.Status = "failed"
		run.ErrorMessage = err.Error()
		now := time.Now()
		run.CompletedAt = &now
		log.Printf("[MIGRATION] Org=%s FAILED: %v", org.ID, err)
		p.migrationRepo.UpdateRunStatus(ctx, run.ID, run.Status, run.ErrorMessage)
		return run
	}

	// If no migrations were applied, mark as skipped (already up to date)
	if appliedCount == 0 {
		run.Status = "skipped"
		now := time.Now()
		run.CompletedAt = &now
		// Still update version in case it's stale
		p.updateOrgVersion(ctx, org.ID, targetVersion)
		p.migrationRepo.UpdateRunStatus(ctx, run.ID, run.Status, "")
		return run
	}

	// Update org version in master DB
	if err := p.updateOrgVersion(ctx, org.ID, targetVersion); err != nil {
		run.Status = "failed"
		run.ErrorMessage = fmt.Sprintf("failed to update org version: %v", err)
		now := time.Now()
		run.CompletedAt = &now
		log.Printf("[MIGRATION] Org=%s FAILED to update version: %v", org.ID, err)
		p.migrationRepo.UpdateRunStatus(ctx, run.ID, run.Status, run.ErrorMessage)
		return run
	}

	// Success
	run.Status = "success"
	now := time.Now()
	run.CompletedAt = &now
	log.Printf("[MIGRATION] Org=%s SUCCESS: applied %d migrations (%s -> %s)", org.ID, appliedCount, org.CurrentVersion, targetVersion)
	p.migrationRepo.UpdateRunStatus(ctx, run.ID, run.Status, "")
	return run
}

// orgInfo holds minimal org data for migration
type orgInfo struct {
	ID             string
	Name           string
	CurrentVersion string
	DatabaseURL    string
	DatabaseToken  string
}

// getAllActiveOrgs returns all active organizations (no version filtering).
// The _migrations table in each tenant DB is the source of truth for what's been applied.
func (p *MigrationPropagator) getAllActiveOrgs(ctx context.Context) ([]orgInfo, error) {
	query := `
		SELECT id, name, COALESCE(current_version, 'v0.1.0'),
		       COALESCE(database_url, ''), COALESCE(database_token, '')
		FROM organizations
		WHERE is_active = 1
		ORDER BY created_at ASC
	`

	rows, err := p.masterDB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query organizations: %w", err)
	}
	defer rows.Close()

	var orgs []orgInfo
	for rows.Next() {
		var org orgInfo
		if err := rows.Scan(&org.ID, &org.Name, &org.CurrentVersion, &org.DatabaseURL, &org.DatabaseToken); err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		org.CurrentVersion = p.versionService.Normalize(org.CurrentVersion)
		orgs = append(orgs, org)
	}

	return orgs, rows.Err()
}

// getOrgsNeedingUpdate returns orgs with version < platform version (kept for RetryOrg compatibility)
func (p *MigrationPropagator) getOrgsNeedingUpdate(ctx context.Context, platformVersion string) ([]orgInfo, error) {
	orgs, err := p.getAllActiveOrgs(ctx)
	if err != nil {
		return nil, err
	}

	var needUpdate []orgInfo
	for _, org := range orgs {
		if p.versionService.NeedsUpdate(org.CurrentVersion, platformVersion) {
			needUpdate = append(needUpdate, org)
		}
	}
	return needUpdate, nil
}

// applyMigrations applies pending migrations to a tenant database.
// Returns the number of migrations applied and any error.
func (p *MigrationPropagator) applyMigrations(ctx context.Context, tenantDB db.DBConn, org *orgInfo) (int, error) {
	// Create _migrations table if not exists
	// Retry on connection errors as they may be transient
	maxRetries := 3
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err = tenantDB.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS _migrations (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT UNIQUE NOT NULL,
				applied_at TEXT DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err == nil {
			break
		}
		if isConnectionError(err) {
			if attempt < maxRetries-1 {
				log.Printf("[MIGRATION] Org=%s Connection error creating migrations table (attempt %d), retrying...", org.ID, attempt+1)
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
				continue
			}
		}
		return 0, fmt.Errorf("failed to create migrations table: %w", err)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to create migrations table after retries: %w", err)
	}

	// Get applied migrations
	applied := make(map[string]bool)
	rows, err := tenantDB.QueryContext(ctx, "SELECT name FROM _migrations")
	if err != nil {
		return 0, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return 0, fmt.Errorf("failed to scan migration name: %w", err)
		}
		applied[name] = true
	}
	rows.Close()

	// Get migration files
	files, err := p.getMigrationFiles()
	if err != nil {
		return 0, err
	}

	// Apply pending migrations
	appliedCount := 0
	for _, file := range files {
		if applied[file] {
			continue
		}

		// Read migration file from embedded FS
		content, err := fs.ReadFile(migrations.Files, file)
		if err != nil {
			return appliedCount, fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		// Begin transaction for this migration with retry logic
		var tx interface {
			ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
			Rollback() error
			Commit() error
		}
		maxRetries := 3
		var beginErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			tx, beginErr = tenantDB.BeginTx(ctx, nil)
			if beginErr == nil {
				break
			}
			if isConnectionError(beginErr) && attempt < maxRetries-1 {
				log.Printf("[MIGRATION] Org=%s Connection error beginning transaction for %s (attempt %d), retrying...", org.ID, file, attempt+1)
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
				continue
			}
			return appliedCount, fmt.Errorf("failed to begin transaction for %s: %w", file, beginErr)
		}
		if beginErr != nil {
			return appliedCount, fmt.Errorf("failed to begin transaction for %s after retries: %w", file, beginErr)
		}

		// Execute statements
		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			// Strip leading SQL comments (-- lines) before checking content
			// This is critical: comments before SQL must not cause the statement to be skipped
			stmt = stripLeadingComments(stmt)
			if stmt == "" {
				continue
			}

			// Skip master-only tables on tenant databases
			if isMasterOnlyStatement(stmt) {
				continue
			}

			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				// Handle idempotent errors (safe to skip)
				if isAddColumnError(err) || isAlreadyExistsError(err) || isForeignKeyError(err) {
					log.Printf("[MIGRATION] Org=%s Skipping idempotent error in %s: %v", org.ID, file, err)
					continue
				}
				// Handle missing table/column errors for master-only tables that we intentionally skip
				if isMissingObjectError(err) {
					log.Printf("[MIGRATION] Org=%s Skipping missing object error in %s: %v", org.ID, file, err)
					continue
				}
				// Check for connection errors - these are transient
				if isConnectionError(err) {
					log.Printf("[MIGRATION] Org=%s WARNING: Connection error in %s, rolling back: %v", org.ID, file, err)
					tx.Rollback()
					return appliedCount, fmt.Errorf("failed to execute %s (connection error): %w", file, err)
				}
				tx.Rollback()
				return appliedCount, fmt.Errorf("failed to execute %s: %w\nStatement: %s", file, err, truncateStmt(stmt))
			}
		}

		// Record migration
		if _, err := tx.ExecContext(ctx, "INSERT INTO _migrations (name) VALUES (?)", file); err != nil {
			tx.Rollback()
			return appliedCount, fmt.Errorf("failed to record migration %s: %w", file, err)
		}

		if err := tx.Commit(); err != nil {
			return appliedCount, fmt.Errorf("failed to commit migration %s: %w", file, err)
		}

		appliedCount++
		log.Printf("[MIGRATION] Org=%s Applied: %s", org.ID, file)
	}

	if appliedCount > 0 {
		log.Printf("[MIGRATION] Org=%s Applied %d migrations", org.ID, appliedCount)
	}

	return appliedCount, nil
}

// getMigrationFiles returns sorted list of migration files from embedded FS
func (p *MigrationPropagator) getMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to list migrations: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

// updateOrgVersion updates the org's current_version in master DB
func (p *MigrationPropagator) updateOrgVersion(ctx context.Context, orgID, version string) error {
	query := `UPDATE organizations SET current_version = ?, modified_at = ? WHERE id = ?`
	_, err := p.masterDB.ExecContext(ctx, query, version, time.Now().UTC().Format(time.RFC3339), orgID)
	return err
}

// RetryOrg retries migration for a specific failed org
func (p *MigrationPropagator) RetryOrg(ctx context.Context, orgID string) (*entity.MigrationRun, error) {
	// Get platform version
	platformVersion, err := p.versionRepo.GetPlatformVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get platform version: %w", err)
	}

	// Get org info
	query := `
		SELECT id, name, COALESCE(current_version, 'v0.1.0'),
		       COALESCE(database_url, ''), COALESCE(database_token, '')
		FROM organizations
		WHERE id = ? AND is_active = 1
	`
	var org orgInfo
	err = p.masterDB.QueryRowContext(ctx, query, orgID).Scan(
		&org.ID, &org.Name, &org.CurrentVersion, &org.DatabaseURL, &org.DatabaseToken,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("organization not found: %s", orgID)
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	org.CurrentVersion = p.versionService.Normalize(org.CurrentVersion)

	// Run migration (always attempt - _migrations table is source of truth)
	run := p.migrateOrg(ctx, &org, platformVersion.Version)
	return &run, nil
}

// stripLeadingComments removes SQL comment lines (-- ...) from the beginning of a statement.
// This ensures that comments preceding CREATE TABLE, INSERT, etc. don't cause the statement
// to be incorrectly skipped or misclassified.
func stripLeadingComments(stmt string) string {
	lines := strings.Split(stmt, "\n")
	var result []string
	foundSQL := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !foundSQL && (strings.HasPrefix(trimmed, "--") || trimmed == "") {
			continue
		}
		foundSQL = true
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

// isAddColumnError checks if error is about a column already existing
func isAddColumnError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate column") ||
		strings.Contains(errStr, "column already exists") ||
		strings.Contains(errStr, "already has a column named")
}

// isAlreadyExistsError checks if error is about an object already existing (table, index)
func isAlreadyExistsError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "table already exists") ||
		strings.Contains(errStr, "index already exists")
}

// isForeignKeyError checks if error is a foreign key constraint mismatch
// These are safe to skip because FK constraints in migrations reference tables
// that may not have the expected schema in tenant DBs (e.g., relationship_defs
// referencing entity_defs.name which may not be unique). The INSERT OR IGNORE
// handles duplicates, and FK enforcement is cosmetic.
func isForeignKeyError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "foreign key mismatch") ||
		strings.Contains(errStr, "foreign key constraint failed")
}

// isMissingObjectError checks if error is about a missing table or column.
// This handles cases where a migration references a master-only table that
// doesn't exist on tenant databases (e.g., INSERT ... SELECT FROM organizations).
func isMissingObjectError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "table does not exist") ||
		strings.Contains(errStr, "table not found") ||
		strings.Contains(errStr, "no such column") ||
		strings.Contains(errStr, "column does not exist")
}

// isConnectionError checks if error is a transient connection error
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "stream is closed") ||
		strings.Contains(errStr, "database is closed") ||
		strings.Contains(errStr, "bad connection") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset")
}

// masterOnlyTableNames is the definitive list of tables that only exist on the master database.
// Tenant databases should never have these tables created or populated.
var masterOnlyTableNames = map[string]bool{
	"organizations":            true,
	"users":                    true,
	"user_org_memberships":     true,
	"sessions":                 true,
	"org_invitations":          true,
	"custom_pages":             true,
	"api_tokens":               true,
	"audit_logs":               true,
	"platform_versions":        true,
	"migration_runs":           true,
	"password_reset_tokens":    true,
	"salesforce_connections":   true,
	"salesforce_field_mappings": true,
	"ingest_api_keys":          true,
}

// tableNamePattern extracts the table name from CREATE TABLE, ALTER TABLE, INSERT INTO,
// DROP TABLE, and CREATE INDEX statements.
var tableNamePattern = regexp.MustCompile(
	`(?i)(?:` +
		`CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + // CREATE TABLE [IF NOT EXISTS]
		`|ALTER\s+TABLE\s+` + // ALTER TABLE
		`|INSERT\s+(?:OR\s+\w+\s+)?INTO\s+` + // INSERT [OR REPLACE|IGNORE] INTO
		`|DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?` + // DROP TABLE [IF EXISTS]
		`|CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?\S+\s+ON\s+` + // CREATE [UNIQUE] INDEX ... ON
		`|DELETE\s+FROM\s+` + // DELETE FROM
		`|UPDATE\s+` + // UPDATE
		`)(\w+)`, // capture the table name
)

// isMasterOnlyStatement checks if a SQL statement operates on a master-only table.
// Uses precise table name extraction from the SQL statement rather than substring matching,
// which prevents false positives from comments, column values, and foreign key references.
func isMasterOnlyStatement(stmt string) bool {
	match := tableNamePattern.FindStringSubmatch(stmt)
	if match == nil {
		return false
	}
	tableName := strings.ToLower(match[1])
	return masterOnlyTableNames[tableName]
}

// truncateStmt truncates a SQL statement for logging (avoids flooding logs with large statements)
func truncateStmt(stmt string) string {
	if len(stmt) > 200 {
		return stmt[:200] + "..."
	}
	return stmt
}
