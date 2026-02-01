---
phase: 04-update-propagation
plan: 02
subsystem: backend/migration
tags: [migration, propagation, multi-tenant, startup, go]

requires:
  - 04-01 # Migration tracking infrastructure
provides:
  - MigrationPropagator service with PropagateAll and RetryOrg methods
  - Blocking startup migration propagation
  - Per-org migration application with skip-and-continue on failures
affects:
  - 04-03 # Admin propagation UI will use RetryOrg method

tech-stack:
  added: []
  patterns:
    - Blocking startup initialization
    - Sequential org processing with per-org timeout
    - Transaction-per-migration atomicity
    - Skip-and-continue failure handling

key-files:
  created:
    - backend/internal/service/migration_propagator.go
  modified:
    - backend/cmd/api/main.go

decisions:
  - Sequential org processing (oldest first via created_at ASC)
  - Per-org timeout of 2 minutes to prevent infinite hangs
  - Local mode skips tenant migrations (shared DB)
  - Transaction per migration file for atomicity
  - Skip-and-continue on failures (don't block other orgs)

metrics:
  duration: ~3min
  completed: 2026-02-01
---

# Phase 04 Plan 02: Migration Propagation Service Summary

**One-liner:** Blocking startup migration propagator that sequentially updates all org databases with 2-minute per-org timeout and skip-on-failure behavior.

## What Was Built

### 1. MigrationPropagator Service (`migration_propagator.go`)

**Core functionality:**
- `PropagateAll(ctx)` - Blocks on startup until all orgs processed
- `RetryOrg(ctx, orgID)` - Manual retry for failed orgs (admin UI)

**Processing logic:**
- Query orgs needing update (current_version < platform_version)
- Process sequentially (ORDER BY created_at ASC - oldest first)
- Per-org timeout: 2 minutes (context.WithTimeout)
- Skip local mode orgs (shared DB - master migrations handle it)
- Transaction per migration file for atomicity
- Continue on failures (log and skip to next org)

**Migration application:**
- Create `_migrations` tracking table if not exists
- Query applied migrations from table
- Read migration files from MIGRATIONS_DIR (env var or ../../migrations)
- Apply pending migrations in transaction
- Record each migration in `_migrations` table
- Update org's `current_version` on success

**Key implementation details:**
- Uses `db.DBConn` interface (supports both *sql.DB and *TursoDB)
- Uses `dbManager.GetTenantDBConn()` for per-org database connections
- Uses `dbManager.IsLocalMode()` to skip shared DB orgs
- Uses `versionService.NeedsUpdate()` for version comparison
- Creates migration run records via `migrationRepo.CreateRun()`
- Updates run status via `migrationRepo.UpdateRunStatus()`

### 2. Startup Integration (`main.go`)

**Initialization order:**
1. Repos (including `migrationRepo`)
2. Services (including `versionService`)
3. DB Manager (`dbManager`)
4. **MigrationPropagator** (after dbManager)
5. **PropagateAll call** (blocking - before middleware)
6. Middleware and handlers
7. Route setup
8. app.Listen()

**Placement rationale:**
- AFTER dbManager: Needs database connections
- AFTER repos/services: Needs versionRepo, migrationRepo, versionService
- BEFORE middleware: Must run before accepting requests
- BEFORE app.Listen(): Blocking startup requirement

## Deviations from Plan

None - plan executed exactly as written.

## Technical Notes

### Version Comparison
Uses existing `VersionService.NeedsUpdate(orgVersion, platformVersion)` which:
- Normalizes versions (adds "v" prefix if missing)
- Uses `golang.org/x/mod/semver.Compare()`
- Returns true if orgVersion < platformVersion

### Local Mode Behavior
In local development (no TURSO_API_TOKEN):
- All orgs share same database (masterDB)
- Master migrations already applied to shared DB
- Tenant migrations would be redundant and cause errors
- PropagateAll skips all orgs, logs "skipped (local mode)"

### Error Handling
**Connection failures:**
- Log error, mark run as "failed", continue to next org
- Error message: "failed to connect: [error]"

**Migration failures:**
- Transaction rollback
- Log error with failing migration name and SQL statement
- Mark run as "failed", continue to next org

**Version update failures:**
- Even if migrations applied, if updating org version fails, mark as "failed"
- Ensures consistency (org version matches applied state)

### Migration File Format
Expects SQL files in migrations directory:
- Semicolon-separated statements
- Comment lines starting with `--` are skipped
- Each file applied atomically in transaction
- Recorded in `_migrations` table by filename

## Next Phase Readiness

**Blocks nothing.** Plan 04-03 can proceed with:
- `migrationPropagator` available for admin handler injection
- `RetryOrg(ctx, orgID)` method ready for manual retry endpoint

**For 04-03 (Admin UI):**
- MigrationPropagator instance exists in main.go
- Will need to be passed to admin handler (or create new instance)
- RetryOrg method signature: `(ctx, orgID) -> (*entity.MigrationRun, error)`

## Verification Results

✅ All files compile: `go build ./...`
✅ MigrationPropagator exports PropagateAll
✅ MigrationPropagator exports RetryOrg
✅ main.go calls PropagateAll before app.Listen()
✅ PropagateAll positioned after dbManager, before middleware

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 9282260 | Create MigrationPropagator service |
| 2 | c83ce79 | Integrate propagator into startup |

## Decisions Made

| Decision | Rationale | Impact |
|----------|-----------|--------|
| Sequential org processing (oldest first) | User discretion - predictable order, easier debugging | Orgs migrate in creation order |
| Per-org 2-minute timeout | Research pitfall guidance - prevent infinite hangs | Long-running migrations may fail |
| Skip-and-continue on failures | User decision - failed orgs don't block others | System starts even with failed migrations |
| Local mode skips tenant migrations | Shared DB already has master migrations | Prevents redundant migration attempts |
| Transaction per migration file | Atomicity requirement | All-or-nothing migration application |
| Processing order by created_at ASC | User decision - oldest orgs first | Predictable, stable ordering |

## Related Files

**Service:**
- `backend/internal/service/migration_propagator.go` (380 lines)

**Main:**
- `backend/cmd/api/main.go` (+17 lines)

**Dependencies:**
- `backend/internal/entity/migration.go` (MigrationRun, PropagationResult)
- `backend/internal/repo/migration.go` (CreateRun, UpdateRunStatus)
- `backend/internal/repo/version.go` (GetPlatformVersion)
- `backend/internal/service/versioning.go` (NeedsUpdate, Normalize)
- `backend/internal/db/manager.go` (GetTenantDBConn, IsLocalMode)
