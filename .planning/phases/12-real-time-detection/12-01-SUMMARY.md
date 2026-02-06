---
phase: 12-real-time-detection
plan: 01
subsystem: database
tags: [go, sqlite, turso, deduplication, real-time, async]

# Dependency graph
requires:
  - phase: 11-detection-foundation
    provides: "Matching rules infrastructure and detection engine"
provides:
  - "Pending duplicate alerts table and entity type"
  - "Alert repository with CRUD operations"
  - "Alert API endpoints for storing and resolving async detection results"
affects: [12-02, 12-03, 12-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "INSERT OR REPLACE for upsert pattern in SQLite"
    - "PendingAlertRepo with tenant database routing via WithDB"

key-files:
  created:
    - backend/internal/migrations/052_create_pending_alerts.sql
    - backend/internal/entity/pending_alert.go
    - backend/internal/repo/pending_alert.go
  modified:
    - backend/internal/handler/dedup.go
    - backend/cmd/api/main.go

key-decisions:
  - "Use INSERT OR REPLACE for alert upsert to handle rapid edits without duplicates"
  - "Include is_block_mode field for frontend to determine warn vs block UI behavior"
  - "Alert endpoints not admin-only - regular users need to see/resolve their own alerts"
  - "Store top 3 matches with record name for rich UI display"

patterns-established:
  - "Pending alert pattern: save immediately, detect in background, store results for user review"
  - "IsBlockMode boolean in Go maps to INTEGER in SQLite with explicit conversion"

# Metrics
duration: 3min
completed: 2026-02-06
---

# Phase 12 Plan 01: Pending Alert Infrastructure Summary

**SQLite table for async duplicate detection results with is_block_mode support, repository with upsert/resolve operations, and REST endpoints for alert management**

## Performance

- **Duration:** 2m 55s
- **Started:** 2026-02-06T12:31:32Z
- **Completed:** 2026-02-06T12:34:27Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Created pending_duplicate_alerts table with indexes for efficient record/status lookups
- Implemented PendingAlertRepo with Upsert (INSERT OR REPLACE), GetPendingByRecord, Resolve, and cleanup operations
- Added GetPendingAlert and ResolveAlert endpoints to DedupHandler accessible to regular authenticated users

## Task Commits

Each task was committed atomically:

1. **Task 1: Create pending alerts migration and entity type** - `fd5f358` (feat)
2. **Task 2: Create pending alert repository** - `3d2d0e5` (feat)
3. **Task 3: Add alert API endpoints to DedupHandler** - `abaa796` (feat)

## Files Created/Modified
- `backend/internal/migrations/052_create_pending_alerts.sql` - Pending duplicate alerts table with is_block_mode column
- `backend/internal/entity/pending_alert.go` - PendingDuplicateAlert entity with IsBlockMode field and alert status constants
- `backend/internal/repo/pending_alert.go` - Repository with Upsert (INSERT OR REPLACE), GetPendingByRecord, Resolve, DeleteOldResolved methods
- `backend/internal/handler/dedup.go` - Added GetPendingAlert and ResolveAlert endpoints, integrated alertRepo
- `backend/cmd/api/main.go` - Wired pendingAlertRepo with DBConn for auto-reconnect support

## Decisions Made

**Use INSERT OR REPLACE for alert upsert:**
- Rationale: Handle rapid record edits where detection re-runs and replaces existing pending alert
- Pattern: Prevents duplicate pending alerts per record via UNIQUE constraint on (org_id, entity_type, record_id, status)

**Include is_block_mode field:**
- Rationale: Frontend needs to know whether to show "warning" banner or "blocking" modal
- Implementation: Populated from matching rule configuration when alert is created
- Storage: Boolean in Go, INTEGER in SQLite with explicit conversion

**Alert endpoints not admin-only:**
- Rationale: Regular users need to see alerts on records they view and resolve them
- Security: Multi-tenant isolation via orgID from JWT token (middleware enforced)

**Store top 3 matches with record name:**
- Rationale: UI can display "Possible duplicates: John Smith, Jane Doe, Bob Johnson"
- Storage: JSON-serialized array with RecordID, RecordName, and MatchResult

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation proceeded as designed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for 12-02 (Real-Time Detection Integration):
- Alert storage infrastructure complete
- API endpoints tested and functional
- Repository properly wired with tenant database routing

Blockers: None

Concerns: None

---
*Phase: 12-real-time-detection*
*Completed: 2026-02-06*
