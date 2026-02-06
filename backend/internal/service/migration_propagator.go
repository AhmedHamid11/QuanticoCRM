package service

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
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

// PropagateAll runs migrations on all orgs that need updates
// This blocks until all orgs are processed (success or failure)
// Failed orgs are logged and skipped - they don't block other orgs
func (p *MigrationPropagator) PropagateAll(ctx context.Context) *entity.PropagationResult {
	result := &entity.PropagationResult{
		StartedAt: time.Now(),
		Runs:      []entity.MigrationRun{},
	}

	// Get current platform version (target)
	platformVersion, err := p.versionRepo.GetPlatformVersion(ctx)
	if err != nil {
		log.Printf("[MIGRATION] ERROR: Failed to get platform version: %v", err)
		result.CompletedAt = time.Now()
		return result
	}
	targetVersion := platformVersion.Version
	log.Printf("[MIGRATION] Target platform version: %s", targetVersion)

	// Get all orgs that need updating
	orgs, err := p.getOrgsNeedingUpdate(ctx, targetVersion)
	if err != nil {
		log.Printf("[MIGRATION] ERROR: Failed to get orgs needing update: %v", err)
		result.CompletedAt = time.Now()
		return result
	}

	if len(orgs) == 0 {
		log.Printf("[MIGRATION] All organizations already at version %s", targetVersion)
		result.CompletedAt = time.Now()
		return result
	}

	log.Printf("[MIGRATION] Found %d organizations needing update to %s", len(orgs), targetVersion)

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
	if err := p.applyMigrations(orgCtx, tenantDB, org); err != nil {
		run.Status = "failed"
		run.ErrorMessage = err.Error()
		now := time.Now()
		run.CompletedAt = &now
		log.Printf("[MIGRATION] Org=%s FAILED: %v", org.ID, err)
		p.migrationRepo.UpdateRunStatus(ctx, run.ID, run.Status, run.ErrorMessage)
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
	log.Printf("[MIGRATION] Org=%s SUCCESS: %s -> %s", org.ID, org.CurrentVersion, targetVersion)
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

// getOrgsNeedingUpdate returns orgs with version < platform version
func (p *MigrationPropagator) getOrgsNeedingUpdate(ctx context.Context, platformVersion string) ([]orgInfo, error) {
	// Query orgs with their database credentials
	// Order by created_at ASC (oldest first) per user decision
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

		// Normalize version for comparison
		org.CurrentVersion = p.versionService.Normalize(org.CurrentVersion)

		// Only include orgs that need update
		if p.versionService.NeedsUpdate(org.CurrentVersion, platformVersion) {
			orgs = append(orgs, org)
		}
	}

	return orgs, rows.Err()
}

// applyMigrations applies pending migrations to a tenant database
func (p *MigrationPropagator) applyMigrations(ctx context.Context, tenantDB db.DBConn, org *orgInfo) error {
	// Create _migrations table if not exists
	_, err := tenantDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			applied_at TEXT DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied := make(map[string]bool)
	rows, err := tenantDB.QueryContext(ctx, "SELECT name FROM _migrations")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	for rows.Next() {
		var name string
		rows.Scan(&name)
		applied[name] = true
	}
	rows.Close()

	// Get migration files
	files, err := p.getMigrationFiles()
	if err != nil {
		return err
	}

	// Apply pending migrations
	appliedCount := 0
	for _, file := range files {
		// file is now just the filename (e.g., "001_create_users.sql") from embedded FS
		if applied[file] {
			continue
		}

		// Read migration file from embedded FS
		content, err := fs.ReadFile(migrations.Files, file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		// Begin transaction for this migration
		tx, err := tenantDB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", file, err)
		}

		// Execute statements
		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}

			// Skip master-only tables on tenant databases
			if isMasterOnlyStatement(stmt) {
				log.Printf("[MIGRATION] Org=%s Skipping master-only statement in %s", org.ID, file)
				continue
			}

			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				// Handle idempotent errors (safe to skip)
				if isAddColumnError(err) || isTableNotExistsError(err) {
					log.Printf("[MIGRATION] Org=%s Skipping safe error in %s: %v", org.ID, file, err)
					continue
				}
				tx.Rollback()
				return fmt.Errorf("failed to execute %s: %w\nStatement: %s", file, err, stmt)
			}
		}

		// Record migration
		if _, err := tx.ExecContext(ctx, "INSERT INTO _migrations (name) VALUES (?)", file); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", file, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file, err)
		}

		appliedCount++
		log.Printf("[MIGRATION] Org=%s Applied: %s", org.ID, file)
	}

	if appliedCount > 0 {
		log.Printf("[MIGRATION] Org=%s Applied %d migrations", org.ID, appliedCount)
	}

	return nil
}

// getMigrationFiles returns sorted list of migration files from embedded FS
func (p *MigrationPropagator) getMigrationFiles() ([]string, error) {
	// Use embedded migrations instead of reading from filesystem
	// This works in Docker containers where the filesystem paths don't exist
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

	// Normalize version
	org.CurrentVersion = p.versionService.Normalize(org.CurrentVersion)

	// Check if update is needed
	if !p.versionService.NeedsUpdate(org.CurrentVersion, platformVersion.Version) {
		return nil, fmt.Errorf("organization %s is already at version %s", org.Name, org.CurrentVersion)
	}

	// Run migration
	run := p.migrateOrg(ctx, &org, platformVersion.Version)
	return &run, nil
}

// isAddColumnError checks if error is about a column already existing
func isAddColumnError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate column") ||
		strings.Contains(errStr, "column already exists") ||
		strings.Contains(errStr, "already has a column named")
}

// isTableNotExistsError checks if error is about a missing table
func isTableNotExistsError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "table does not exist") ||
		strings.Contains(errStr, "table not found")
}

// isMasterOnlyStatement checks if a statement operates on master-only tables
// These should not be applied to tenant databases
func isMasterOnlyStatement(stmt string) bool {
	masterOnlyTables := []string{
		"organizations",
		"users",
		"user_org_memberships",
		"sessions",
		"org_invitations",
		"custom_pages",
		"api_tokens",
		"audit_log",
		"_migrations",
	}

	upper := strings.ToUpper(stmt)
	for _, table := range masterOnlyTables {
		if strings.Contains(upper, strings.ToUpper(table)) {
			return true
		}
	}
	return false
}
