---
phase: 19-add-mirror-table-safety-net-to-provision
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - backend/internal/service/provisioning.go
  - backend/internal/handler/mirror.go
  - backend/cmd/api/main.go
autonomous: true
must_haves:
  truths:
    - "POST /api/v1/admin/mirrors returns 201 even when mirrors table was missing (auto-created)"
    - "GET /api/v1/admin/mirrors returns 200 even when mirrors table was missing (auto-created)"
    - "Reprovision endpoint creates mirror/ingest tables alongside metadata tables"
    - "Mirror handler self-heals on 'no such table' error by creating tables and retrying once"
  artifacts:
    - path: "backend/internal/service/provisioning.go"
      provides: "ensureIngestTables() and EnsureAllTables() methods"
      contains: "ensureIngestTables"
    - path: "backend/internal/handler/mirror.go"
      provides: "Auto-recovery on 'no such table' in all handler methods"
      contains: "ensureIngestTables"
    - path: "backend/cmd/api/main.go"
      provides: "Wiring of provisioning service into MirrorHandler"
      contains: "NewMirrorHandler"
  key_links:
    - from: "backend/internal/handler/mirror.go"
      to: "backend/internal/service/provisioning.go"
      via: "provisioning.EnsureIngestTables() call on error recovery"
      pattern: "EnsureIngestTables"
    - from: "backend/internal/service/provisioning.go"
      to: "provisionMetadata"
      via: "ensureIngestTables called after ensureMetadataTables"
      pattern: "ensureIngestTables"
---

<objective>
Add mirror/ingest table creation as a safety net in the provisioning system, and add auto-recovery to the mirror handler so it self-heals when tables are missing.

Purpose: Fix the 500 error on POST /api/v1/admin/mirrors caused by missing mirrors table on tenant DBs where migrations failed to propagate.

Output: Self-healing mirror handler + provisioning safety net for all 4 ingest tables.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@backend/internal/service/provisioning.go (existing ensureMetadataTables pattern to follow)
@backend/internal/handler/mirror.go (handler to add auto-recovery)
@backend/cmd/api/main.go (wiring to update)
@backend/internal/migrations/063_create_mirrors.sql (exact mirror+source_fields schema)
@backend/internal/migrations/064_create_ingest_pipeline_tables.sql (exact ingest_jobs+delta_keys schema + map_field ALTER)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add ensureIngestTables() and EnsureAllTables() to provisioning.go</name>
  <files>backend/internal/service/provisioning.go</files>
  <action>
Add a new method `ensureIngestTables(ctx context.Context) error` to ProvisioningService, following the exact same pattern as `ensureMetadataTables()`. This method should use `CREATE TABLE IF NOT EXISTS` for all 4 ingest tables with schemas matching migrations 063 and 064:

1. `mirrors` table — exact schema from 063_create_mirrors.sql:
   - id TEXT PRIMARY KEY, org_id TEXT NOT NULL, name TEXT NOT NULL, target_entity TEXT NOT NULL, unique_key_field TEXT NOT NULL, unmapped_field_mode TEXT NOT NULL DEFAULT 'flexible', rate_limit INTEGER DEFAULT 500, is_active INTEGER DEFAULT 1, created_at TEXT DEFAULT CURRENT_TIMESTAMP, updated_at TEXT DEFAULT CURRENT_TIMESTAMP
   - Indexes: idx_mirrors_org(org_id), idx_mirrors_active(org_id, is_active)

2. `mirror_source_fields` table — schema from 063 + map_field from 064:
   - id TEXT PRIMARY KEY, mirror_id TEXT NOT NULL, field_name TEXT NOT NULL, field_type TEXT NOT NULL DEFAULT 'text', is_required INTEGER DEFAULT 0, description TEXT DEFAULT '', sort_order INTEGER DEFAULT 0, map_field TEXT, created_at TEXT DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (mirror_id) REFERENCES mirrors(id) ON DELETE CASCADE
   - Indexes: idx_mirror_source_fields_mirror(mirror_id), idx_mirror_source_fields_unique(mirror_id, field_name) UNIQUE

3. `ingest_jobs` table — exact schema from 064:
   - id TEXT PRIMARY KEY, org_id TEXT NOT NULL, mirror_id TEXT NOT NULL, key_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'accepted', records_received/processed/promoted/skipped/failed INTEGERs, errors TEXT DEFAULT '[]', warnings TEXT DEFAULT '[]', started_at TEXT, completed_at TEXT, created_at TEXT DEFAULT CURRENT_TIMESTAMP, updated_at TEXT DEFAULT CURRENT_TIMESTAMP
   - Indexes: idx_ingest_jobs_org(org_id), idx_ingest_jobs_mirror(org_id, mirror_id), idx_ingest_jobs_status(org_id, status)

4. `ingest_delta_keys` table — exact schema from 064:
   - id TEXT PRIMARY KEY, org_id TEXT NOT NULL, mirror_id TEXT NOT NULL, unique_key TEXT NOT NULL, record_id TEXT, ingested_at TEXT DEFAULT CURRENT_TIMESTAMP, UNIQUE(mirror_id, unique_key)
   - Indexes: idx_delta_keys_mirror(mirror_id), idx_delta_keys_lookup(mirror_id, unique_key)

Log progress like ensureMetadataTables does: `[Provisioning] Creating ingest pipeline tables...`

Add a public method `EnsureIngestTables(ctx context.Context) error` that simply calls the private `ensureIngestTables`.

Add a public method `EnsureAllTables(ctx context.Context) error` that calls `ensureMetadataTables(ctx)` then `ensureIngestTables(ctx)`, returning the first error.

In `provisionMetadata()`, add a call to `ensureIngestTables(ctx)` right after the existing `ensureMetadataTables(ctx)` call (around line 493-495). If it fails, log a warning but do NOT return an error (ingest tables are optional for core provisioning).

Update the `ProvisioningServiceInterface` in admin.go to add `EnsureAllTables` so the reprovision endpoint can call it. Actually, simpler: just have ReprovisionMetadata call ProvisionDefaultMetadata which already calls provisionMetadata which will now call ensureIngestTables. So no interface change needed.
  </action>
  <verify>
Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` — must compile without errors.
  </verify>
  <done>
ensureIngestTables() creates all 4 tables with IF NOT EXISTS. EnsureIngestTables() is public. EnsureAllTables() calls both ensure methods. provisionMetadata() calls ensureIngestTables after ensureMetadataTables.
  </done>
</task>

<task type="auto">
  <name>Task 2: Add auto-recovery to MirrorHandler and wire provisioning service</name>
  <files>backend/internal/handler/mirror.go, backend/cmd/api/main.go</files>
  <action>
**In mirror.go:**

1. Add `provisioning *service.ProvisioningService` field to the MirrorHandler struct.

2. Update `NewMirrorHandler` to accept a provisioning service parameter:
   `func NewMirrorHandler(repo *repo.MirrorRepo, jobRepo *repo.IngestJobRepo, provisioning *service.ProvisioningService) *MirrorHandler`

3. Add a private helper method `tryEnsureIngestTables(c *fiber.Ctx) error` that:
   - Gets the tenant DB connection via `h.getTenantDBConn(c)`
   - Creates a temporary provisioning service pointing to the tenant DB: `ps := service.NewProvisioningService(tenantDB)` then calls `ps.EnsureIngestTables(c.Context())`
   - Returns the error (nil on success)

4. Add a private helper `isNoSuchTableError(err error) bool` that checks `strings.Contains(err.Error(), "no such table")`.

5. In the `Create` handler, wrap the repo.Create call with retry logic:
   ```
   mirror, err := h.repo.Create(c.Context(), tenantDB, orgID, input)
   if err != nil && isNoSuchTableError(err) {
       log.Printf("[MirrorHandler] 'no such table' error, attempting to create ingest tables for org %s", orgID)
       if ensureErr := h.tryEnsureIngestTables(c); ensureErr != nil {
           log.Printf("[MirrorHandler] Failed to create ingest tables: %v", ensureErr)
           return c.Status(500).JSON(fiber.Map{"error": "Database schema not initialized. Please contact admin."})
       }
       // Retry the operation
       mirror, err = h.repo.Create(c.Context(), tenantDB, orgID, input)
   }
   if err != nil {
       return c.Status(500).JSON(fiber.Map{"error": err.Error()})
   }
   ```

6. Apply the same retry pattern to `List` (wrap h.repo.ListByOrg), `Get` (wrap h.repo.GetByID), `Update` (wrap h.repo.Update), `Delete` (wrap h.repo.Delete), and `ListJobs` (wrap both h.repo.GetByID and h.jobRepo.ListByMirror).

Import "log" and "strings" and "github.com/fastcrm/backend/internal/service" if not already imported.

**In main.go:**

Update line 319 to pass provisioning service:
`mirrorHandler := handler.NewMirrorHandler(mirrorRepo, ingestJobRepo, provisioningService)`
  </action>
  <verify>
Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` — must compile without errors. Run `go vet ./...` for additional safety.
  </verify>
  <done>
MirrorHandler catches "no such table" errors, calls EnsureIngestTables to create missing tables, retries the operation once, and returns a friendly error if retry also fails. main.go passes provisioningService to NewMirrorHandler.
  </done>
</task>

</tasks>

<verification>
1. `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` compiles cleanly
2. `go vet ./...` passes
3. Grep for `ensureIngestTables` in provisioning.go confirms the method exists
4. Grep for `EnsureIngestTables` in mirror.go confirms auto-recovery is wired
5. Grep for `provisioningService` in the NewMirrorHandler call in main.go confirms wiring
</verification>

<success_criteria>
- All Go code compiles without errors
- provisioning.go has ensureIngestTables() creating 4 tables (mirrors, mirror_source_fields, ingest_jobs, ingest_delta_keys) with CREATE TABLE IF NOT EXISTS
- provisionMetadata() calls ensureIngestTables() as safety net
- EnsureIngestTables() is public for external callers
- MirrorHandler auto-recovers from "no such table" by creating tables and retrying
- main.go passes provisioningService to MirrorHandler constructor
</success_criteria>

<output>
After completion, create `.planning/quick/19-add-mirror-table-safety-net-to-provision/19-SUMMARY.md`
</output>
