---
phase: 13-merge-engine
plan: 03
subsystem: deduplication
tags: [merge, atomic-transactions, undo, snapshots, audit, sqlite]

# Dependency graph
requires:
  - phase: 13-01
    provides: Merge entity types and SFID prefix
  - phase: 13-02
    provides: Merge snapshot repository and discovery service
provides:
  - Atomic merge execution with transaction safety
  - Snapshot-based undo with 30-day window
  - FK transfer automation across all related records
  - Audit trail for merge and undo operations
affects: [13-04-merge-api, 14-merge-ui]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Atomic transactions with deferred rollback (from bulk.go)
    - Snapshot-based undo with JSON field storage
    - Fire-and-forget audit logging via goroutines

key-files:
  created:
    - backend/internal/service/merge.go
  modified:
    - backend/internal/entity/audit.go
    - backend/internal/service/audit.go

key-decisions:
  - "FK transfer happens BEFORE archiving to preserve referential integrity"
  - "Snapshots use JSON fields for flexibility across any entity structure"
  - "Undo validates 30-day window and consumed state to prevent double-undo"
  - "Audit events moved to audit.go for consistency with other event types"

patterns-established:
  - "Merge service uses deferred rollback pattern for transaction safety"
  - "Helper functions fetchRecordAsMapTx and restoreRecordFromSnapshot operate within transactions"
  - "toJSON/fromJSON helpers handle marshal/unmarshal with safe defaults on error"

# Metrics
duration: 3min
completed: 2026-02-07
---

# Phase 13 Plan 03: Merge Execution Summary

**Atomic merge execution with transaction safety, snapshot-based undo, and audit trail**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-07T23:00:27Z
- **Completed:** 2026-02-07T23:03:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- ExecuteMerge performs atomic merge within single SQLite transaction
- UndoMerge reverses merge with snapshot validation and consumed state tracking
- Audit events capture who/when/what for merge and undo operations
- FK transfer automation across all related records discovered via metadata

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement merge execution service** - `1021d49` (feat)
2. **Task 2: Add merge audit event types and logging helpers** - `b59d309` (feat)

## Files Created/Modified
- `backend/internal/service/merge.go` - MergeService with ExecuteMerge and UndoMerge
- `backend/internal/entity/audit.go` - Added RECORD_MERGE and MERGE_UNDO event types
- `backend/internal/service/audit.go` - Added LogRecordMerge and LogMergeUndo methods

## Decisions Made

**1. FK transfer order: Transfer BEFORE archiving**
- Rationale: Per research Pitfall #2, archived records may cause FK constraint errors in some configurations
- Implementation: All related record FKs transferred in step 8, duplicates archived in step 10

**2. Snapshot JSON structure**
- Rationale: Supports any entity structure without type coupling, enables custom entities
- Implementation: survivorBefore, duplicateSnapshots, relatedRecordFKs all use JSON fields with toJSON/fromJSON helpers

**3. Undo validation rules**
- Rationale: Balance between data retention and undo capability, prevent double-undo corruption
- Implementation: UndoMerge validates ExpiresAt > now and ConsumedAt is NULL before proceeding

**4. Audit event location**
- Rationale: Consistency with existing pattern where ALL audit events live in entity/audit.go
- Implementation: Moved AuditEventRecordMerge and AuditEventMergeUndo from entity/merge.go to entity/audit.go

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

Ready for 13-04 (Merge API). The merge service provides:
- ExecuteMerge for API handler to call
- UndoMerge for undo endpoint
- Clean error handling with transaction rollback
- Audit logging built in

Blockers: None
Concerns: None

## Self-Check: PASSED

All files exist, all commits verified.

---
*Phase: 13-merge-engine*
*Completed: 2026-02-07*
