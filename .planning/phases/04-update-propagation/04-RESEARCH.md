# Phase 4: Update Propagation - Research

**Researched:** 2026-02-01
**Domain:** Multi-tenant database migration, Go startup jobs, version tracking
**Confidence:** HIGH

## Summary

This phase implements automatic database migration propagation to all organization databases on application startup. The research focused on three areas: (1) Go Fiber startup patterns for blocking initialization, (2) multi-tenant migration strategies, and (3) migration status tracking schemas.

The codebase already has a robust foundation for this work: existing migration infrastructure (`cmd/migrate/main.go`), multi-tenant database management (`db/manager.go`), version tracking (`organizations.current_version`), and the provisioning service pattern (`provisioning.go`). The implementation follows the established patterns - a new service that iterates through organizations, applies migrations, and updates version numbers.

**Primary recommendation:** Create a MigrationPropagator service that runs before `app.Listen()`, iterates organizations sequentially, applies migrations within transactions, and stores results in a new `migration_runs` table for admin visibility.

## Standard Stack

The established libraries/tools for this domain:

### Core (Already in Use)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `database/sql` | stdlib | Database operations | Go standard library |
| `golang.org/x/mod/semver` | latest | Version comparison | Already used for version tracking (Phase 1 decision) |
| `github.com/gofiber/fiber/v2` | 2.x | Web framework | Already in use |
| `github.com/tursodatabase/libsql-client-go` | latest | Turso/SQLite driver | Already in use |

### Supporting (New for This Phase)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `time` | stdlib | Timestamps, durations | Migration timing |
| `context` | stdlib | Timeout control | Per-org migration timeouts |
| `log` | stdlib | Structured logging | Migration progress tracking |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom migration runner | golang-migrate | Overkill - our migrations are simple SQL files, already have working pattern |
| Concurrent org processing | Sequential (chosen) | User explicitly chose sequential for safety and predictability |
| Background job queue | Blocking startup (chosen) | User explicitly chose blocking for simplicity |

**No new dependencies needed.** Everything required is already in use or in stdlib.

## Architecture Patterns

### Recommended Project Structure

Extend existing structure:
```
backend/
├── internal/
│   ├── service/
│   │   └── migration_propagator.go  # NEW: Propagation service
│   ├── repo/
│   │   └── migration.go             # NEW: Migration status repo
│   ├── handler/
│   │   └── version.go               # EXTEND: Add migration status endpoint
│   └── entity/
│       └── migration.go             # NEW: Migration run entity
└── cmd/
    └── api/
        └── main.go                  # MODIFY: Call propagator before Listen()
```

### Pattern 1: Blocking Startup Initialization

**What:** Run migration propagation BEFORE `app.Listen()` is called
**When to use:** When all orgs must be migrated before accepting any requests
**Why:** User explicitly chose this - app waits for all org migrations before accepting requests

**Example:**
```go
// Source: Existing main.go pattern + Fiber docs
func main() {
    // ... existing setup (repos, services, handlers, middleware) ...

    // Initialize migration propagator
    migrationPropagator := service.NewMigrationPropagator(
        masterDBConn,
        dbManager,
        versionRepo,
        versionService,
    )

    // Run migrations BEFORE app.Listen() - this blocks until complete
    log.Println("Running migration propagation for all organizations...")
    result := migrationPropagator.PropagateAll(context.Background())
    log.Printf("Migration propagation complete: %d success, %d failed",
        result.SuccessCount, result.FailedCount)

    // ... route setup ...

    // NOW start accepting requests
    app.Listen(":" + port)
}
```

### Pattern 2: Per-Org Transactional Migration

**What:** Each org's migration runs in a transaction; failure rolls back that org only
**When to use:** Always - ensures atomic migration per org
**Why:** User chose "per-org transaction - each org's migration is atomic"

**Example:**
```go
// Source: Existing db/turso.go BeginTx pattern
func (p *MigrationPropagator) migrateOrg(ctx context.Context, org *entity.Organization) error {
    // Get tenant database connection
    tenantDB, err := p.dbManager.GetTenantDBConn(ctx, org.ID, org.DatabaseURL, org.DatabaseToken)
    if err != nil {
        return fmt.Errorf("failed to connect to org %s: %w", org.ID, err)
    }

    // Begin transaction
    tx, err := tenantDB.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback() // No-op if committed

    // Apply pending migrations within transaction
    if err := p.applyPendingMigrations(ctx, tx, org); err != nil {
        return fmt.Errorf("migration failed: %w", err)
    }

    // Update org version in master DB
    if err := p.updateOrgVersion(ctx, org.ID, p.targetVersion); err != nil {
        return fmt.Errorf("failed to update org version: %w", err)
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit: %w", err)
    }

    return nil
}
```

### Pattern 3: Skip-and-Continue with Status Tracking

**What:** On org failure, log full context and continue to next org
**When to use:** Always - ensures one bad org doesn't block all others
**Why:** User chose "skip and continue - if an org fails, log it and move to next org"

**Example:**
```go
// Source: Designed for this phase
func (p *MigrationPropagator) PropagateAll(ctx context.Context) PropagationResult {
    result := PropagationResult{
        StartedAt: time.Now(),
    }

    orgs, _ := p.getOrgsNeedingUpdate(ctx)

    for _, org := range orgs {
        runResult := MigrationRun{
            OrgID:          org.ID,
            OrgName:        org.Name,
            FromVersion:    org.CurrentVersion,
            ToVersion:      p.targetVersion,
            StartedAt:      time.Now(),
        }

        err := p.migrateOrg(ctx, &org)

        runResult.CompletedAt = time.Now()
        if err != nil {
            runResult.Status = "failed"
            runResult.ErrorMessage = err.Error()
            result.FailedCount++
            log.Printf("[MIGRATION] Org=%s Name=%s FAILED: %v", org.ID, org.Name, err)
        } else {
            runResult.Status = "success"
            result.SuccessCount++
            log.Printf("[MIGRATION] Org=%s Name=%s SUCCESS", org.ID, org.Name)
        }

        // Always save run result (success or failure)
        p.saveMigrationRun(ctx, &runResult)
        result.Runs = append(result.Runs, runResult)
    }

    result.CompletedAt = time.Now()
    return result
}
```

### Pattern 4: Version-Aware Migration Selection

**What:** Only apply migrations newer than the org's current version
**When to use:** To avoid re-running migrations
**Why:** User discretion - version-aware is cleaner than idempotent re-run given existing patterns

**Example:**
```go
// Source: Existing versioning.go + changelog/entries.go pattern
func (p *MigrationPropagator) getPendingMigrations(orgVersion string) []Migration {
    var pending []Migration

    // Get all migrations sorted by version
    for version, migration := range p.migrations {
        // Only include migrations newer than org's current version
        if p.versionService.Compare(version, orgVersion) > 0 {
            pending = append(pending, migration)
        }
    }

    // Sort by version ascending (oldest first)
    sort.Slice(pending, func(i, j int) bool {
        return p.versionService.Compare(pending[i].Version, pending[j].Version) < 0
    })

    return pending
}
```

### Anti-Patterns to Avoid

- **Running propagation after Listen():** App would accept requests with inconsistent schema versions
- **Concurrent org processing:** Harder to debug, resource contention, user explicitly chose sequential
- **Shared transaction across orgs:** One failure would roll back all orgs
- **Silent failures:** Always log full context (org name, error, timestamp, version)
- **Storing credentials in migration_runs:** Only store org_id, never database tokens

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Version comparison | String comparison | `golang.org/x/mod/semver` | Already in use, handles v prefix, prerelease |
| Version normalization | Manual prefix handling | `VersionService.Normalize()` | Already implemented in Phase 1 |
| Tenant DB connections | Direct sql.Open | `dbManager.GetTenantDBConn()` | Handles reconnection, connection pooling |
| Transaction retry | Custom retry loop | `TenantDB.BeginTx()` | Built-in retry logic in turso.go |
| Org listing | Custom query | `authRepo.ListOrganizations()` | Already implemented with pagination |

**Key insight:** The codebase already has well-tested patterns for everything this phase needs. Follow existing conventions rather than inventing new approaches.

## Common Pitfalls

### Pitfall 1: Connection Exhaustion During Propagation

**What goes wrong:** Opening connections to all org databases simultaneously exhausts connection limits
**Why it happens:** Eager connection pooling, not closing idle connections
**How to avoid:** Process orgs sequentially, use single connection per org, close when done
**Warning signs:** "too many open files" or connection timeout errors

### Pitfall 2: Long Migration Blocking Startup Indefinitely

**What goes wrong:** A large org or slow network causes startup to take 10+ minutes
**Why it happens:** No per-org timeout, one slow org blocks all
**How to avoid:** Set per-org migration timeout (e.g., 2 minutes), fail fast on timeout
**Warning signs:** Startup takes longer than expected, health checks failing

```go
// Prevention: Per-org context with timeout
ctx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
defer cancel()
err := p.migrateOrg(ctx, &org)
```

### Pitfall 3: Version Mismatch After Partial Migration

**What goes wrong:** Migration applies some statements, fails, but version is already updated
**Why it happens:** Updating version outside the transaction
**How to avoid:** Update org version in master DB ONLY after tenant migration transaction commits
**Warning signs:** Org shows latest version but has old schema

### Pitfall 4: Migration Status Not Persisted

**What goes wrong:** Failed orgs aren't tracked, admin can't see what failed
**Why it happens:** Only logging to stdout, not saving to database
**How to avoid:** Save migration run record BEFORE and AFTER each org migration
**Warning signs:** Can't determine which orgs failed after restart

### Pitfall 5: Retry Endpoint Without Version Guard

**What goes wrong:** Admin can retry an org that's already at latest version
**Why it happens:** Missing version check in retry handler
**How to avoid:** Only allow retry for orgs where `current_version < platform_version`
**Warning signs:** Migrations run twice, duplicate records or constraint violations

## Code Examples

Verified patterns from existing codebase:

### Get All Organizations Needing Update
```go
// Source: Pattern from authRepo + versionRepo
func (p *MigrationPropagator) getOrgsNeedingUpdate(ctx context.Context) ([]entity.Organization, error) {
    // Get platform version (target)
    platformVersion, err := p.versionRepo.GetPlatformVersion(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get platform version: %w", err)
    }

    // Query orgs with version < platform version
    query := `
        SELECT id, name, slug, current_version, database_url, database_token
        FROM organizations
        WHERE current_version IS NULL
           OR current_version = ''
           OR current_version < ?
        ORDER BY created_at ASC
    `

    rows, err := p.masterDB.QueryContext(ctx, query, platformVersion.Version)
    // ... scan rows ...
}
```

### Migration Run Entity
```go
// Source: Pattern from existing entities like session.go
type MigrationRun struct {
    ID           string    `json:"id"`
    OrgID        string    `json:"orgId"`
    OrgName      string    `json:"orgName"`
    FromVersion  string    `json:"fromVersion"`
    ToVersion    string    `json:"toVersion"`
    Status       string    `json:"status"` // "running", "success", "failed"
    ErrorMessage string    `json:"errorMessage,omitempty"`
    StartedAt    time.Time `json:"startedAt"`
    CompletedAt  time.Time `json:"completedAt,omitempty"`
}
```

### Migration Status API Response
```go
// Source: Pattern from version handler
type MigrationStatusResponse struct {
    PlatformVersion  string         `json:"platformVersion"`
    TotalOrgs        int            `json:"totalOrgs"`
    UpToDateCount    int            `json:"upToDateCount"`
    FailedCount      int            `json:"failedCount"`
    FailedOrgs       []FailedOrg    `json:"failedOrgs"`
    LastRunAt        *time.Time     `json:"lastRunAt,omitempty"`
}

type FailedOrg struct {
    OrgID        string    `json:"orgId"`
    OrgName      string    `json:"orgName"`
    ErrorMessage string    `json:"errorMessage"`
    FailedAt     time.Time `json:"failedAt"`
    AttemptedVersion string `json:"attemptedVersion"`
}
```

### Update Org Version (Master DB)
```go
// Source: Pattern to add to authRepo or versionRepo
func (r *VersionRepo) UpdateOrgVersion(ctx context.Context, orgID, version string) error {
    query := `UPDATE organizations SET current_version = ?, modified_at = ? WHERE id = ?`
    _, err := r.db.ExecContext(ctx, query, version, time.Now().UTC(), orgID)
    return err
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual migration via CLI | Automatic on startup | This phase | Zero-touch deployment |
| Version stored but not enforced | Version-gated migrations | This phase | Consistent platform version |
| No failure visibility | Admin panel status | This phase | Operational visibility |

**Deprecated/outdated:**
- Manual `go run cmd/migrate/main.go` for each org: Replaced by automatic propagation

## Open Questions

Things that couldn't be fully resolved:

1. **Migration file versioning scheme**
   - What we know: Existing migrations use sequential numbers (001_, 002_, etc.)
   - What's unclear: How to map SQL files to platform versions
   - Recommendation: Create a Go map of `version -> []migration_files` or embed version in migration filename (e.g., `043_v0.2.0_add_column.sql`)

2. **Version-specific migrations vs cumulative schema**
   - What we know: Current migrations are cumulative (apply all in order)
   - What's unclear: Should v0.2.0 migrations apply to all orgs regardless of their starting version?
   - Recommendation: Apply all migrations with version > org_version in order; existing migration tracking table (`_migrations`) prevents duplicates

3. **Retry mechanism for failed orgs**
   - What we know: User wants manual retry from admin panel
   - What's unclear: Should retry be per-org or batch all failed orgs?
   - Recommendation: Implement both - individual retry button per org, plus "Retry All Failed" button

## Sources

### Primary (HIGH confidence)
- Existing codebase: `cmd/migrate/main.go` - Migration runner pattern
- Existing codebase: `internal/db/manager.go` - Tenant connection management
- Existing codebase: `internal/db/turso.go` - DBConn interface, transaction support
- Existing codebase: `internal/service/provisioning.go` - Per-org operations pattern
- Existing codebase: `internal/repo/version.go` - Version queries
- Existing codebase: `internal/service/versioning.go` - Version comparison
- [Fiber API Documentation](https://docs.gofiber.io/api/fiber/) - Startup patterns

### Secondary (MEDIUM confidence)
- [Go Database Migrations with golang-migrate](https://oneuptime.com/blog/post/2026-01-07-go-database-migrations/view) - Migration status tracking patterns
- [Atlas Multi-Tenant Go Apps](https://atlasgo.io/blog/2025/05/26/gophercon-scalable-multi-tenant-apps-in-go) - Multi-tenant migration challenges
- [Database migrations in Go](https://betterstack.com/community/guides/scaling-go/golang-migrate/) - Migration versioning

### Tertiary (LOW confidence)
- [Multi-tenant Database Migration Patterns](https://fenixara.com/database-migration-and-version-control-for-multiple-schema-multi-tenant-systems/) - General multi-tenant patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using existing codebase libraries only
- Architecture: HIGH - Follows established patterns (provisioning service, version service)
- Pitfalls: MEDIUM - Based on common patterns, some extrapolation from multi-tenant literature

**Research date:** 2026-02-01
**Valid until:** 2026-03-01 (30 days - stable patterns, no external dependencies)

## Implementation Notes for Planner

### Database Schema Addition

```sql
-- New table for migration run tracking (in master DB)
CREATE TABLE IF NOT EXISTS migration_runs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    org_name TEXT NOT NULL,
    from_version TEXT NOT NULL,
    to_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'success', 'failed')),
    error_message TEXT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX idx_migration_runs_org ON migration_runs(org_id);
CREATE INDEX idx_migration_runs_status ON migration_runs(status);
CREATE INDEX idx_migration_runs_started ON migration_runs(started_at DESC);
```

### Frontend UI Placement

Per user decision: Add migration status section to existing changelog page (`/admin/changelog`).

Suggested UI structure:
1. **Migration Status Card** at top of page (before version list)
   - Show: "X of Y organizations up to date"
   - If failures: Warning banner with count and "View Details" link
2. **Failed Orgs Section** (collapsible, only if failures exist)
   - Table: Org Name | Error | Failed At | Retry Button
   - "Retry All Failed" button at bottom
3. **Existing Changelog** below (unchanged)

### Processing Order Recommendation

Per user discretion: Use **creation order** (oldest first, `ORDER BY created_at ASC`).

Rationale:
- Oldest orgs are most likely to be actively used
- Consistent, deterministic order for debugging
- Matches existing `ListOrganizations` pattern
