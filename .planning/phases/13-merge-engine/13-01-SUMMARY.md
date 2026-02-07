---
phase: 13-merge-engine
plan: 01
subsystem: database
tags: [migrations, go-entities, sfid, merge-snapshots, archive-columns]

# Dependency graph
requires:
  - phase: 12-real-time-detection
    provides: duplicate detection and alert system
provides:
  - merge_snapshots table for undo capability
  - Archive column schema contract for entity tables
  - MergeSnapshot, MergeRequest, MergeResult, MergePreview Go types
  - SFID prefix 0Ms for merge snapshot IDs
affects: [13-02-merge-service, 13-03-merge-api, 13-04-merge-ui]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Archive columns added dynamically to entity tables at merge time"
    - "Merge snapshots expire after 30 days (undo window)"

key-files:
  created:
    - backend/internal/migrations/055_create_merge_snapshots.sql
    - backend/internal/migrations/056_add_archive_columns.sql
    - backend/internal/entity/merge.go
  modified:
    - backend/internal/sfid/sfid.go

key-decisions:
  - "Archive columns added dynamically via ALTER TABLE at merge time (not pre-provisioned)"
  - "Merge snapshots use JSON fields for flexibility (survivor_before, duplicate_snapshots, related_record_fks)"
  - "SFID prefix 0Ms for MergeSnapshot IDs"

patterns-established:
  - "Snapshot-based undo: Store full record state before merge for 30-day rollback window"
  - "consumed_at field prevents double-undo attacks"

# Metrics
duration: 1.6min
completed: 2026-02-07
---

# Phase 13 Plan 01: Merge Foundation Summary

**Database schema and Go entity types for snapshot-based merge with 30-day undo capability**

## Performance

- **Duration:** 1 min 35 sec
- **Started:** 2026-02-07T22:51:44Z
- **Completed:** 2026-02-07T22:53:19Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- merge_snapshots table created with full snapshot storage and FK change tracking
- Archive column contract documented (archived_at, archived_reason, survivor_id)
- Complete Go entity types for merge operations (MergeSnapshot, MergeRequest, MergeResult, MergePreview)
- SFID prefix 0Ms added for merge snapshot ID generation

## Task Commits

Each task was committed atomically:

1. **Task 1: Create merge database migrations** - `1200342` (feat)
2. **Task 2: Create Go entity types and SFID prefix** - `a4fa49a` (feat)

## Files Created/Modified
- `backend/internal/migrations/055_create_merge_snapshots.sql` - merge_snapshots table with snapshot storage and expiration
- `backend/internal/migrations/056_add_archive_columns.sql` - Documents archive column contract for dynamic ALTER TABLE
- `backend/internal/entity/merge.go` - Complete merge entity types including MergeSnapshot, MergeRequest, MergeResult, MergePreview, RelatedRecordGroup, FKChange
- `backend/internal/sfid/sfid.go` - Added PrefixMergeSnapshot = "0Ms" and NewMergeSnapshot() helper

## Decisions Made

**Archive columns added dynamically (not pre-provisioned)**
- Migration 056 documents the schema contract but doesn't ALTER TABLE
- Merge service will add archive columns at merge time if missing
- Rationale: Entity tables are dynamically created per-org, custom entities exist, and not all entities will be merged

**JSON fields for snapshot storage**
- survivor_before: Full JSON snapshot of survivor record before merge
- duplicate_snapshots: JSON array of full duplicate record snapshots
- related_record_fks: JSON map of FK changes for undo FK reversal
- Rationale: Flexible schema that supports any entity structure without type coupling

**30-day undo window with expiration**
- expires_at field set to created_at + 30 days
- consumed_at prevents double-undo
- Rationale: Balance between data retention and undo capability window

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

Ready for 13-02 (Merge Service Implementation):
- Database schema in place
- Go entity types compiled and ready
- SFID prefix registered for snapshot ID generation
- Archive column pattern documented

All merge plans (13-02 through 13-04) can now build on this foundation.

---

## Self-Check: PASSED

---
*Phase: 13-merge-engine*
*Completed: 2026-02-07*
