# Phase 1: Platform Versioning - Research

**Researched:** 2026-01-31
**Domain:** Database schema versioning, semver handling, multi-tenant updates
**Confidence:** HIGH

## Summary

Platform versioning in a multi-tenant, database-per-tenant architecture requires coordinating version information across a central database and multiple tenant databases. The user has decided to use semver format (X.Y.Z), store version info in database tables replicated to each org, and automatically propagate updates via background jobs after deploy.

The standard approach involves:
- Official Go semver library (`golang.org/x/mod/semver`) for version parsing and comparison
- Database tables for platform version history and org current version tracking
- Background job scheduler (robfig/cron v3) for async org database updates
- Transaction-based migrations with deferred rollback for error safety
- Fix-forward strategy for failed migrations instead of rollback

**Primary recommendation:** Use `golang.org/x/mod/semver` for version handling, `robfig/cron/v3` for background job scheduling, and implement idempotent version update jobs with comprehensive error tracking per org.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/mod/semver | Latest (2026) | Semantic version parsing and comparison | Official Go implementation, zero dependencies, requires "v" prefix which enforces convention |
| github.com/robfig/cron/v3 | v3.x | Background job scheduling | Lightweight, goroutine-based, supports standard cron syntax and intervals |
| database/sql | stdlib | Database transactions | Built-in Go standard library, provides transaction safety |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/tursodatabase/libsql-client-go | v0.0.0-20240902231107-85af5b9d094d | Turso database driver | Already in use, supports multi-DB connections |
| github.com/gofiber/fiber/v2 | v2.52.0 | HTTP framework | Already in use, cron runs alongside Fiber app |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| golang.org/x/mod/semver | Masterminds/semver/v3 | More features (range constraints, wildcards) but heavier; only needed for complex version constraints which user explicitly doesn't need |
| robfig/cron/v3 | Custom time.Ticker | More control but must hand-roll scheduling, error handling, timezone support |
| Database version table | File-based versioning | Simpler but doesn't work for database-per-tenant where each org needs version tracking |

**Installation:**
```bash
go get golang.org/x/mod/semver
go get github.com/robfig/cron/v3
```

## Architecture Patterns

### Recommended Project Structure
```
backend/
├── internal/
│   ├── db/
│   │   └── turso.go           # Existing TenantDB wrapper
│   ├── service/
│   │   └── versioning.go      # Version comparison, update logic
│   ├── scheduler/
│   │   └── jobs.go            # Background job definitions
│   └── handler/
│       └── version.go         # API endpoints for version info
├── cmd/
│   ├── api/
│   │   └── main.go            # Fiber app + cron scheduler init
│   └── migrate/
│       └── main.go            # Existing migration runner
└── migrations/
    └── 042_create_version_tables.sql
```

### Pattern 1: Version Table Schema
**What:** Two tables - platform version history and org current version tracking
**When to use:** All multi-tenant SaaS platforms with schema evolution
**Example:**
```sql
-- Platform version history (replicated to all org DBs)
CREATE TABLE IF NOT EXISTS platform_versions (
    version TEXT PRIMARY KEY,      -- e.g., "v0.1.0"
    description TEXT NOT NULL,     -- What changed
    released_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Org current version (in each org DB + central DB)
ALTER TABLE organizations ADD COLUMN current_version TEXT DEFAULT 'v0.1.0';

CREATE INDEX IF NOT EXISTS idx_organizations_version
ON organizations(current_version);
```
**Source:** Based on [Bytebase database version control best practices](https://www.bytebase.com/blog/database-version-control-best-practice/) and [Devart database versioning](https://blog.devart.com/database-versioning-with-examples.html)

### Pattern 2: Semver Comparison
**What:** Use `golang.org/x/mod/semver` with "v" prefix convention
**When to use:** Comparing platform version to org version to detect updates needed
**Example:**
```go
// Source: https://pkg.go.dev/golang.org/x/mod/semver
import "golang.org/x/mod/semver"

func needsUpdate(orgVersion, platformVersion string) bool {
    // Both must have "v" prefix: "v0.1.0"
    if !semver.IsValid(orgVersion) || !semver.IsValid(platformVersion) {
        return false
    }
    return semver.Compare(orgVersion, platformVersion) < 0
}

// Get major version: semver.Major("v2.1.0") == "v2"
// Get canonical: semver.Canonical("v1.2") == "v1.2.0"
```

### Pattern 3: Background Job with Cron
**What:** Cron scheduler runs alongside Fiber app, updates org databases asynchronously
**When to use:** Any recurring background work that shouldn't block HTTP requests
**Example:**
```go
// Source: https://pkg.go.dev/github.com/robfig/cron/v3
import "github.com/robfig/cron/v3"

func initScheduler() *cron.Cron {
    c := cron.New()

    // Run version sync every 10 minutes
    c.AddFunc("@every 10m", func() {
        if err := syncOrgVersions(); err != nil {
            log.Printf("Version sync failed: %v", err)
        }
    })

    c.Start()
    return c
}

// In main.go:
// scheduler := initScheduler()
// defer scheduler.Stop()
```

### Pattern 4: Idempotent Version Update
**What:** Update function that can safely run multiple times without duplicating work
**When to use:** All background jobs that might retry or run concurrently
**Example:**
```go
func updateOrgVersion(orgDB *db.TenantDB, targetVersion string) error {
    ctx := context.Background()

    // 1. Check current version
    var currentVersion string
    err := db.QueryRowScan(ctx, orgDB, []interface{}{&currentVersion},
        "SELECT version FROM platform_versions ORDER BY released_at DESC LIMIT 1")
    if err != nil && err != sql.ErrNoRows {
        return err
    }

    // 2. Skip if already at target (idempotent)
    if currentVersion == targetVersion {
        return nil
    }

    // 3. Insert new version record (INSERT OR REPLACE for safety)
    _, err = orgDB.ExecContext(ctx, `
        INSERT OR REPLACE INTO platform_versions (version, description, released_at)
        VALUES (?, ?, CURRENT_TIMESTAMP)
    `, targetVersion, "Auto-updated from platform")

    return err
}
```

### Pattern 5: Transaction with Deferred Rollback
**What:** Standard Go pattern for safe transaction handling
**When to use:** Any multi-statement database operation that must be atomic
**Example:**
```go
// Source: https://go.dev/doc/database/execute-transactions
func updateVersionTransactionally(db *sql.DB, version string) error {
    tx, err := db.BeginTx(context.Background(), nil)
    if err != nil {
        return err
    }

    // Defer rollback - safe even after commit
    defer tx.Rollback()

    // Multiple operations
    _, err = tx.Exec("INSERT INTO platform_versions (...) VALUES (?)", version)
    if err != nil {
        return err // Rollback happens via defer
    }

    _, err = tx.Exec("UPDATE organizations SET current_version = ?", version)
    if err != nil {
        return err // Rollback happens via defer
    }

    // Commit - rollback becomes no-op if this succeeds
    return tx.Commit()
}
```

### Anti-Patterns to Avoid
- **String comparison for versions:** "v10.0.0" < "v2.0.0" in string comparison but > in semver
- **Forgetting "v" prefix:** `golang.org/x/mod/semver` requires "v" prefix, will fail silently
- **Blocking HTTP requests:** Don't run org updates in API handlers, use background jobs
- **No idempotency:** Background jobs must handle being run multiple times
- **Manual BEGIN/COMMIT:** Use `BeginTx()` not raw SQL statements

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Semantic version parsing | Regex version parsing, string splits | `golang.org/x/mod/semver` | Handles edge cases: pre-release tags, build metadata, shorthand versions (v1 = v1.0.0) |
| Version comparison | String comparison, split-and-compare | `semver.Compare()` | String comparison breaks for v10 vs v2, doesn't handle pre-release precedence |
| Background job scheduling | Custom time.Ticker loops | `robfig/cron/v3` | Handles timezone, standard cron syntax, multiple jobs, error recovery |
| Distributed locking for jobs | Custom file locks or flags | Let jobs be idempotent instead | Simpler - idempotent jobs don't need locks |
| Migration rollback | Custom rollback scripts | Fix-forward with new migration | Rollback can fail on partially applied migrations |

**Key insight:** Semver has complex rules (pre-release ordering, build metadata, canonical forms) that are easy to get wrong. Database transactions and cron scheduling have concurrency edge cases. Use battle-tested libraries.

## Common Pitfalls

### Pitfall 1: Case-Sensitive Pre-Release Comparison
**What goes wrong:** `semver.Compare("v1.0.0-alpha.1", "v1.0.0-Beta.2")` treats "alpha" > "Beta" due to ASCII ordering (lowercase > uppercase)
**Why it happens:** Semver spec uses ASCII sort order for pre-release identifiers
**How to avoid:** User decided no pre-release labels - good! If you ever need them, lowercase consistently
**Warning signs:** Unexpected version ordering in tests
**Source:** [Common SemVer Problems](https://codeengineered.com/blog/2024/common-semver-issues/)

### Pitfall 2: Partial Migration Failure
**What goes wrong:** Migration applies 5 of 10 statements, then fails. Database in unknown state. Rollback fails because some tables don't exist.
**Why it happens:** SQLite/Turso don't support transactional DDL for all operations
**How to avoid:**
- Make each migration small and atomic
- Use `INSERT OR REPLACE` for idempotency
- Implement fix-forward: new migration to handle partial state
- Track failed org updates separately for manual intervention
**Warning signs:** Migration error logs showing "table already exists" or "column already exists"
**Source:** [Database Rollbacks in CI/CD](https://medium.com/@jasminfluri/database-rollbacks-in-ci-cd-strategies-and-pitfalls-f0ffd4d4741a)

### Pitfall 3: Background Job Runs During Deployment
**What goes wrong:** Deployment pushes new version. Old container's cron job runs, updates some orgs to new version. New container starts, tries to update same orgs, conflicts.
**Why it happens:** Zero-downtime deployments overlap old and new containers
**How to avoid:**
- Make version updates idempotent (check current version first)
- Use INSERT OR REPLACE for version records
- Railway deployments may have brief overlap - design for it
**Warning signs:** Duplicate version records, race condition errors in logs
**Source:** Background job deployment patterns

### Pitfall 4: Org Database Connection Closed
**What goes wrong:** Background job loops through 100 orgs, org 50's database connection dies mid-update, job crashes, orgs 51-100 never updated
**Why it happens:** Turso connections can timeout or close during long-running jobs
**How to avoid:**
- Wrap each org update in try/catch, log error, continue to next org
- Use existing `TenantDB` retry logic (already handles reconnection)
- Store update status per org (success/failure/pending) in central DB
**Warning signs:** Job stops processing after certain org, no errors for remaining orgs
**Source:** Existing codebase `turso.go` connection handling

### Pitfall 5: Forgetting "v" Prefix
**What goes wrong:** Store version as "0.1.0" in database, `semver.IsValid("0.1.0")` returns false, all comparisons fail silently
**Why it happens:** `golang.org/x/mod/semver` requires "v" prefix by design
**How to avoid:**
- Always store versions with "v" prefix: "v0.1.0"
- Database constraint or app validation to enforce prefix
- Use `semver.Canonical()` to normalize input
**Warning signs:** Versions stored without "v", comparison logic always returns false
**Source:** [golang.org/x/mod/semver documentation](https://pkg.go.dev/golang.org/x/mod/semver)

### Pitfall 6: Race Between Deploy and Migration
**What goes wrong:** Railway deploys new code, migration hasn't run yet, code assumes new schema exists, crashes
**Why it happens:** User wants version bump on deploy, but if migration runs separately, timing is uncertain
**How to avoid:**
- Option A: Run migrations in Railway build command before starting app
- Option B: Version bump happens AFTER migration confirmed (conservative)
- Option C: Code handles both old and new schema gracefully (complex)
**Warning signs:** App crashes immediately after deploy with "column not found" errors
**Recommendation:** Run migrations in Railway's build process, version bump after migration success

## Code Examples

Verified patterns from official sources:

### Version Comparison Service
```go
// internal/service/versioning.go
package service

import (
    "golang.org/x/mod/semver"
)

type VersionService struct{}

func NewVersionService() *VersionService {
    return &VersionService{}
}

// NeedsUpdate returns true if orgVersion is older than platformVersion
func (s *VersionService) NeedsUpdate(orgVersion, platformVersion string) bool {
    if !semver.IsValid(orgVersion) || !semver.IsValid(platformVersion) {
        return false
    }
    return semver.Compare(orgVersion, platformVersion) < 0
}

// Normalize ensures version has "v" prefix and canonical form
func (s *VersionService) Normalize(version string) string {
    if version == "" {
        return "v0.1.0"
    }
    if !strings.HasPrefix(version, "v") {
        version = "v" + version
    }
    return semver.Canonical(version)
}
```

### Background Job Scheduler
```go
// internal/scheduler/jobs.go
package scheduler

import (
    "log"
    "github.com/robfig/cron/v3"
)

func InitScheduler(svc *service.VersionService) *cron.Cron {
    c := cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.New(os.Stdout, "cron: ", log.LstdFlags))))

    // Sync org versions every 10 minutes
    c.AddFunc("@every 10m", func() {
        log.Println("Starting version sync job")
        if err := syncAllOrgVersions(svc); err != nil {
            log.Printf("Version sync failed: %v", err)
        }
    })

    c.Start()
    log.Println("Scheduler started")
    return c
}

func syncAllOrgVersions(svc *service.VersionService) error {
    // Get platform version
    platformVersion := getCurrentPlatformVersion()

    // Get all orgs
    orgs := getAllOrganizations()

    for _, org := range orgs {
        if err := updateOrgVersion(org, platformVersion, svc); err != nil {
            // Log error but continue to next org
            log.Printf("Failed to update org %s: %v", org.ID, err)
            continue
        }
    }

    return nil
}
```

### Main App with Scheduler
```go
// cmd/api/main.go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/robfig/cron/v3"
    "yourapp/internal/scheduler"
)

func main() {
    app := fiber.New()

    // Init services
    versionSvc := service.NewVersionService()

    // Init background scheduler
    cronScheduler := scheduler.InitScheduler(versionSvc)
    defer cronScheduler.Stop()

    // Routes
    app.Get("/api/version", handlers.GetVersion)

    // Start server
    app.Listen(":3000")
}
```

### Idempotent Version Update
```go
// internal/service/versioning.go (continued)
func (s *VersionService) UpdateOrgVersion(orgDB *db.TenantDB, orgID, targetVersion string) error {
    ctx := context.Background()

    // Get current version from org DB
    var currentVersion string
    err := db.QueryRowScan(ctx, orgDB, []interface{}{&currentVersion},
        "SELECT version FROM platform_versions ORDER BY released_at DESC LIMIT 1")

    // First version - table might not have records yet
    if err == sql.ErrNoRows {
        currentVersion = "v0.0.0"
    } else if err != nil {
        return fmt.Errorf("failed to get current version: %w", err)
    }

    // Already up to date
    if currentVersion == targetVersion {
        log.Printf("Org %s already at version %s", orgID, targetVersion)
        return nil
    }

    // Insert new version record
    _, err = orgDB.ExecContext(ctx, `
        INSERT OR REPLACE INTO platform_versions (version, description, released_at)
        VALUES (?, ?, CURRENT_TIMESTAMP)
    `, targetVersion, fmt.Sprintf("Updated from %s", currentVersion))

    if err != nil {
        return fmt.Errorf("failed to insert version: %w", err)
    }

    log.Printf("Updated org %s from %s to %s", orgID, currentVersion, targetVersion)
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual version tracking in code | Database version tables | Established 2020s | Enables per-tenant version tracking, migration history |
| Masterminds/semver | golang.org/x/mod/semver | 2024+ | Lighter, official Go library, but requires "v" prefix |
| gocraft/work (Redis queue) | robfig/cron + idempotent jobs | Ongoing | Simpler for scheduled jobs, no Redis dependency |
| Database rollback scripts | Fix-forward migrations | 2020s+ | More reliable - rollback can fail on partial migrations |

**Deprecated/outdated:**
- Rolling back failed migrations - modern practice is fix-forward with new migration
- Complex multi-version support per org - increases complexity, modern SaaS keeps all users on same version
- File-based version tracking - doesn't work for database-per-tenant architecture

## Open Questions

Things that couldn't be fully resolved:

1. **Railway Deploy Hooks**
   - What we know: Railway supports webhooks and environment variables, can run commands on deploy
   - What's unclear: Exact mechanism to bump version number on deploy (env var? script? manual?)
   - Recommendation:
     - Option A: Environment variable `PLATFORM_VERSION` set manually in Railway, updated on schema changes
     - Option B: Read version from migration file names (042_create_version_tables.sql -> v0.42.0)
     - Option C: Hardcode in code, update when schema changes (simple but manual)
     - **Suggest Option A for user:** Manual env var update when migrations change

2. **Changelog Notification Timing**
   - What we know: User wants changelog section in admin panel showing version changes
   - What's unclear: When/how changelog content is populated (migration files? manual entry? API?)
   - Recommendation: Store changelog in `platform_versions.description` field, populated by migration or manual admin action

3. **Failed Update Recovery**
   - What we know: Background job might fail to update some orgs
   - What's unclear: Should there be retry logic? Manual intervention UI? Alert system?
   - Recommendation:
     - Track update status per org in central DB (pending/success/failed)
     - Retry failed orgs on next cron run (with max retry count)
     - Admin UI to view failed updates and trigger manual retry (future phase)

4. **Version Comparison Edge Cases**
   - What we know: User uses simple X.Y.Z format, starting at v0.1.0
   - What's unclear: How to handle v0.1.0 vs v0.1 (shorthand) if accidentally entered
   - Recommendation: Use `semver.Canonical()` to normalize all versions to X.Y.Z format before storage

## Sources

### Primary (HIGH confidence)
- [golang.org/x/mod/semver documentation](https://pkg.go.dev/golang.org/x/mod/semver) - Official Go semver library
- [robfig/cron v3 documentation](https://pkg.go.dev/github.com/robfig/cron/v3) - Cron scheduler library
- [Go database transactions guide](https://go.dev/doc/database/execute-transactions) - Official Go docs
- [Database Version Control Best Practice](https://www.bytebase.com/blog/database-version-control-best-practice/) - Version table design patterns
- Existing codebase: `/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend/internal/db/turso.go` - Multi-tenant DB patterns already implemented

### Secondary (MEDIUM confidence)
- [Common SemVer Problems](https://codeengineered.com/blog/2024/common-semver-issues/) - Verified with semver spec
- [Database Rollbacks in CI/CD](https://medium.com/@jasminfluri/database-rollbacks-in-ci-cd-strategies-and-pitfalls-f0ffd4d4741a) - Recent article (Oct 2025)
- [Turso Multi-Tenant Schema Management](https://turso.tech/blog/database-per-tenant-architectures-get-production-friendly-improvements) - Schema replication patterns
- [Railway Webhooks Documentation](https://docs.railway.com/guides/webhooks) - Deploy hook capabilities
- [Go Database Transactions Best Practices](https://threedots.tech/post/database-transactions-in-go/) - Three Dots Labs pattern guide

### Tertiary (LOW confidence)
- Various WebSearch results about background job patterns - community patterns, not authoritative
- Stack Overflow discussions about migration failures - anecdotal but matches authoritative sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official libraries with stable APIs, widely adopted
- Architecture: HIGH - Patterns verified in official docs and existing codebase
- Pitfalls: MEDIUM - Based on documented issues and community experience, but some are inferred
- CI/CD integration: MEDIUM - Railway docs exist but exact version-bump mechanism unclear
- Background job error handling: MEDIUM - Standard patterns but org-specific recovery strategy needs design

**Research date:** 2026-01-31
**Valid until:** 2026-03-31 (60 days - libraries are stable, but Railway/Turso features may evolve)

**Next steps for planner:**
- Design database schema for version tables
- Plan background job implementation with error tracking
- Decide version bump mechanism (env var vs file-based vs hardcoded)
- Design org update status tracking for failed updates
