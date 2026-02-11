---
phase: quick-19
plan: 01
subsystem: database, api
tags: [sqlite, provisioning, self-healing, mirrors, ingest]

# Dependency graph
requires:
  - phase: 21-ingest-pipeline
    provides: mirrors, ingest_jobs, ingest_delta_keys, mirror_source_fields tables
provides:
  - ensureIngestTables() safety net in provisioning service
  - Self-healing MirrorHandler that auto-creates missing tables
  - EnsureAllTables() for reprovision coverage
affects: [mirrors, ingest, provisioning, tenant-db]

# Tech tracking
tech-stack:
  added: []
  patterns: [self-healing-handler, create-if-not-exists-safety-net]

key-files:
  created: []
  modified:
    - backend/internal/service/provisioning.go
    - backend/internal/handler/mirror.go
    - backend/cmd/api/main.go

key-decisions:
  - "Non-fatal ingest table creation in provisionMetadata (log warning, don't block core provisioning)"
  - "Reused existing isNoSuchTableError from dedup.go instead of duplicating"
  - "Create temporary ProvisioningService with tenant DB for handler-level self-healing"

patterns-established:
  - "Self-healing handler: catch 'no such table', create tables, retry once"

# Metrics
duration: 3min
completed: 2026-02-11
---

# Quick Task 019: Mirror Table Safety Net Summary

**Self-healing MirrorHandler with provisioning safety net for all 4 ingest pipeline tables (mirrors, mirror_source_fields, ingest_jobs, ingest_delta_keys)**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-11T03:22:54Z
- **Completed:** 2026-02-11T03:25:41Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added `ensureIngestTables()` to provisioning service creating all 4 ingest tables with CREATE TABLE IF NOT EXISTS and full index coverage
- MirrorHandler now self-heals on "no such table" errors across all 6 endpoints (Create, List, Get, Update, Delete, ListJobs)
- `provisionMetadata()` calls `ensureIngestTables()` as a non-fatal safety net alongside existing `ensureMetadataTables()`
- Wired provisioning service into MirrorHandler constructor in main.go

## Task Commits

Each task was committed atomically:

1. **Task 1: Add ensureIngestTables() and EnsureAllTables() to provisioning.go** - `4b0e644` (feat)
2. **Task 2: Add auto-recovery to MirrorHandler and wire provisioning service** - `b87d33d` (feat)

## Files Created/Modified
- `backend/internal/service/provisioning.go` - Added ensureIngestTables(), EnsureIngestTables(), EnsureAllTables(), and safety net call in provisionMetadata()
- `backend/internal/handler/mirror.go` - Added tryEnsureIngestTables() helper and retry-on-missing-table logic to all 6 handler methods
- `backend/cmd/api/main.go` - Updated NewMirrorHandler call to pass provisioningService

## Decisions Made
- Made ingest table creation non-fatal in provisionMetadata() since ingest tables are optional for core org provisioning
- Reused existing `isNoSuchTableError` from dedup.go (same handler package) instead of declaring a duplicate
- Created temporary ProvisioningService with tenant DB connection for handler-level self-healing (follows existing pattern)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed duplicate isNoSuchTableError declaration**
- **Found during:** Task 2 (mirror handler auto-recovery)
- **Issue:** Plan specified adding `isNoSuchTableError` to mirror.go, but it already existed in dedup.go (same package)
- **Fix:** Removed duplicate declaration and unused `strings` import, reusing the existing function
- **Files modified:** backend/internal/handler/mirror.go
- **Verification:** `go build ./...` and `go vet ./...` pass cleanly
- **Committed in:** b87d33d (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary fix to avoid compilation error. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Mirror endpoints now self-heal on tenant DBs where migrations 063/064 failed to propagate
- Reprovision endpoint automatically covers ingest tables via provisionMetadata flow
- No blockers

---
*Quick Task: 019-add-mirror-table-safety-net-to-provision*
*Completed: 2026-02-11*
