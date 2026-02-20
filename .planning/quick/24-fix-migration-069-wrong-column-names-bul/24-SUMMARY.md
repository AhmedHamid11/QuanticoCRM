---
phase: quick-24
plan: "01"
subsystem: database
tags: [sqlite, migration, indexes, collate-nocase, pending-alerts, deduplication]

# Dependency graph
requires:
  - phase: quick-23
    provides: deduplication review queue and pending alerts infrastructure
provides:
  - Corrected COLLATE NOCASE indexes on accounts.name, contacts.last_name, contacts.email_address
  - UNIQUE constraint-safe BulkResolve with delete-before-update pattern
affects: [deduplication, bulk-dismiss, bulk-resolve, pending-alerts]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - delete-before-update pattern for UNIQUE constraint safety in bulk operations

key-files:
  created: []
  modified:
    - backend/internal/migrations/069_add_lookup_indexes.sql
    - backend/internal/repo/pending_alert.go

key-decisions:
  - "Used delete-before-update in BulkResolve to match pattern established in single-record Resolve method"
  - "Scoped delete to org_id + target status + optional entity_type to match the scope of the UPDATE"
  - "Removed all leads table indexes — no leads table exists in the schema"

patterns-established:
  - "delete-before-update: When updating a record's status can collide with an existing row under the same UNIQUE constraint, delete the colliding row first"

requirements-completed: [QUICK-24]

# Metrics
duration: 0min
completed: 2026-02-20
---

# Quick Task 24: Fix Migration 069 Wrong Column Names + BulkResolve UNIQUE Constraint Summary

**Fixed migration 069 referencing non-existent columns (contacts.name, contacts.email, leads table) and added delete-before-update to BulkResolve to prevent UNIQUE constraint violations when bulk-dismissing previously dismissed alerts.**

## Performance

- **Duration:** 0 min (pre-committed)
- **Completed:** 2026-02-20
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Migration 069 now creates exactly 3 valid COLLATE NOCASE indexes on columns that actually exist: `accounts(org_id, name COLLATE NOCASE)`, `contacts(org_id, last_name COLLATE NOCASE)`, `contacts(org_id, email_address COLLATE NOCASE)`
- BulkResolve in `pending_alert.go` now deletes previously resolved/dismissed alerts with the same target status before updating, preventing UNIQUE constraint violations on `(org_id, entity_type, record_id, status)`
- Bulk dismiss/resolve operations can now handle records that were previously dismissed, re-pended, and dismissed again without erroring

## Task Commits

Both tasks committed atomically:

1. **Task 1: Fix migration 069 column names and remove leads indexes** - `56511b2` (fix)
2. **Task 2: Fix BulkResolve UNIQUE constraint violation** - `56511b2` (fix)

**Plan metadata:** `c211eae` (docs: plan quick task 24)

## Files Created/Modified
- `backend/internal/migrations/069_add_lookup_indexes.sql` - Corrected COLLATE NOCASE indexes: `contacts.name` → `contacts.last_name`, `contacts.email` → `contacts.email_address`, removed all `leads` table indexes
- `backend/internal/repo/pending_alert.go` - Added delete-before-update block in `BulkResolve` before the UPDATE statement, scoped to `org_id + status + optional entity_type`

## Decisions Made
- Used delete-before-update pattern in BulkResolve to match the pattern already established in the single-record `Resolve` method (lines 145-151)
- Scoped the delete in BulkResolve to `org_id + status` (+ optional `entity_type`) to precisely match the scope of the UPDATE it precedes — no wider deletion than necessary

## Deviations from Plan

None - fixes were pre-committed exactly as specified in the plan.

## Issues Encountered
None - both bugs were straightforward schema and constraint issues with clear root causes.

## Next Phase Readiness
- Bulk dismiss/resolve is now safe for repeated use on the same records
- COLLATE NOCASE indexes are correctly scoped to columns that exist, so migration 069 will apply cleanly to all tenant DBs on next startup

---
*Phase: quick-24*
*Completed: 2026-02-20*
