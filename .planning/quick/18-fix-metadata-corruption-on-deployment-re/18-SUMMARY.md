---
phase: quick-018
plan: "01"
subsystem: database
tags: [sqlite, provisioning, metadata, deployment-safety, pragma]

# Dependency graph
requires:
  - phase: quick-014
    provides: "navigation_tabs schema fix with PRAGMA table_info pattern"
provides:
  - "Non-destructive ensureMetadataTables using PRAGMA table_info"
  - "INSERT OR IGNORE for all default data (preserves user customizations)"
  - "Safe re-provisioning that never drops existing metadata"
affects: [provisioning, deployment, metadata]

# Tech tracking
tech-stack:
  added: []
  patterns: ["PRAGMA table_info for column existence checks", "INSERT OR IGNORE for idempotent default data"]

key-files:
  created: []
  modified:
    - "backend/internal/service/provisioning.go"

key-decisions:
  - "Use PRAGMA table_info instead of sql LIKE for schema validation - reliable across all SQLite versions"
  - "INSERT OR IGNORE over INSERT OR REPLACE - preserves user customizations, new orgs still get defaults"
  - "Retain dropAndRecreateMetadataTables as dead code with DEPRECATED comment for emergency manual use"

patterns-established:
  - "PRAGMA table_info: Always use PRAGMA table_info for column existence checks, never sql LIKE patterns"
  - "INSERT OR IGNORE: Default data insertion must use INSERT OR IGNORE to preserve customizations"

# Metrics
duration: 2min
completed: 2026-02-11
---

# Quick Task 018: Fix Metadata Corruption on Deployment Summary

**Non-destructive provisioning using PRAGMA table_info and INSERT OR IGNORE to prevent metadata wipe on deployment**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-11T03:09:56Z
- **Completed:** 2026-02-11T03:11:28Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- Replaced fragile `sql LIKE '%org_id%name%'` schema check with reliable `PRAGMA table_info(entity_defs)` column existence check
- Removed destructive `dropAndRecreateMetadataTables()` call from `ensureMetadataTables()` -- deployments can no longer wipe all org metadata
- Changed all 3 `INSERT OR REPLACE` statements to `INSERT OR IGNORE` for navigation_tabs, layout_defs, and related_list_configs -- user customizations now survive re-provisioning

## Task Commits

Each task was committed atomically:

1. **Task 1: Replace fragile schema check with PRAGMA table_info** - `2ba7cf9` (fix)
2. **Task 2: Change INSERT OR REPLACE to INSERT OR IGNORE** - `62515b1` (fix)

## Files Created/Modified
- `backend/internal/service/provisioning.go` - Safe schema validation and non-destructive default data insertion

## Decisions Made
- Used PRAGMA table_info (same pattern as ensureNavigationTabsTable) for consistency and reliability
- Chose INSERT OR IGNORE over INSERT OR REPLACE: silently skips existing rows, preserving any user modifications while still inserting defaults for new orgs
- Kept dropAndRecreateMetadataTables as dead code with DEPRECATED comment rather than deleting it, for emergency manual use if ever needed

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Deployments are now safe and will not corrupt metadata
- Re-provisioning (via admin panel or API) preserves user customizations
- New org provisioning continues to work correctly

---
*Quick Task: 018-fix-metadata-corruption-on-deployment-re*
*Completed: 2026-02-11*
