---
phase: quick
plan: 003
subsystem: api
tags: [go, fiber, sqlite, schema-sync, custom-entities]

# Dependency graph
requires:
  - phase: none
    provides: n/a
provides:
  - Schema sync errors properly surfaced to API clients
  - Text fields now save correctly on custom entities
affects: [custom-entities, entity-manager, field-definitions]

# Tech tracking
tech-stack:
  added: []
  patterns: [error-blocking pattern for schema operations]

key-files:
  created: []
  modified: [FastCRM/fastcrm/backend/internal/handler/generic_entity.go]

key-decisions:
  - "Schema sync errors must block execution to prevent silent data loss"
  - "Applied fix to all entity mutation handlers (Create, Update, Upsert)"

patterns-established:
  - "Schema sync errors return HTTP 500 with descriptive message instead of logging warnings"

# Metrics
duration: 3min
completed: 2026-02-02
---

# Quick Task 003: Fix Text Field Saving on Custom Entities

**Schema sync failures now block execution and return HTTP 500, preventing silent data loss on custom entities**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-02T18:59:37Z
- **Completed:** 2026-02-02T19:01:53Z
- **Tasks:** 3
- **Files modified:** 1

## Accomplishments
- Fixed silent schema sync failures that caused text fields not to persist
- Schema sync errors now properly return HTTP 500 to API clients
- Applied fix to all entity mutation handlers (Create, Update, Upsert)
- **Added EnsureTableExists call before SyncFieldColumns in Create/Update handlers**
  - This creates the table automatically if it doesn't exist (retroactive fix)
  - Taxrise and any other org with missing tables will auto-heal on first save

## Task Commits

Each task was committed atomically:

1. **Task 1-3: Fix SyncFieldColumns error handling** - `09fc2a3` (fix)
2. **Task 4: Add EnsureTableExists to Create/Update** - `b5e4952` (fix, retroactive)

## Files Created/Modified
- `FastCRM/fastcrm/backend/internal/handler/generic_entity.go` - Changed SyncFieldColumns error handling from WARNING log to ERROR + HTTP 500 response in Create, Update, and Upsert handlers

## Decisions Made
- **Schema sync must block execution:** When `SyncFieldColumns` fails, we must return an error to the client rather than continuing with the INSERT/UPDATE. Continuing would result in SQL errors or silent data loss if columns don't exist.
- **Consistent error handling:** Applied the same fix pattern to all three handlers (Create, Update, Upsert) to ensure consistent behavior across all entity mutation operations.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added SyncFieldColumns to Upsert handler**
- **Found during:** Task 1 (code review)
- **Issue:** Upsert handler was missing SyncFieldColumns call entirely, only calling ensureTableExists. This meant upsert operations could fail silently if fields existed in metadata but columns were missing.
- **Fix:** Added the same SyncFieldColumns call with error blocking to the Upsert handler
- **Files modified:** FastCRM/fastcrm/backend/internal/handler/generic_entity.go (line ~1215)
- **Verification:** Build passes
- **Committed in:** 09fc2a3 (combined with main fix)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Auto-fix essential for consistency. Upsert handler had same bug as Create/Update but was not mentioned in plan.

## Issues Encountered
None - fix was straightforward error handling change.

## Root Cause Analysis

The bug occurred because of two issues:

**Issue 1: CreateEntity doesn't create the data table**
- When admin creates a custom entity, only metadata is stored (`entity_defs`, `field_defs`)
- The actual data table is NOT created at entity creation time
- On first record save, `SyncFieldColumns` fails because "table does not exist"

**Issue 2: SyncFieldColumns failures were silently ignored**
- When `SyncFieldColumns` failed, the code logged a WARNING and continued
- INSERT/UPDATE would attempt to write to a non-existent table, causing silent data loss

**Two-part fix:**
1. **Call EnsureTableExists before SyncFieldColumns** in Create/Update handlers
   - Creates the table automatically if missing (retroactive fix for all orgs)
2. **Block execution when SyncFieldColumns fails**
   - Return HTTP 500 instead of continuing with broken schema

**Code before:**
```go
if columnsAdded, syncErr := util.SyncFieldColumns(...); syncErr != nil {
    log.Printf("WARNING: Failed to sync columns for %s: %v", entityName, syncErr)
} else if columnsAdded > 0 {
    log.Printf("INFO: Added %d missing columns to %s table", columnsAdded, tableName)
}
// Continue with INSERT/UPDATE...
```

**Code after:**
```go
if columnsAdded, syncErr := util.SyncFieldColumns(...); syncErr != nil {
    log.Printf("ERROR: Failed to sync columns for %s: %v", entityName, syncErr)
    return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
        "error": fmt.Sprintf("Failed to prepare table schema: %v", syncErr),
    })
}
// Only continue if sync succeeded
```

## Next Phase Readiness
- Text fields on custom entities now save correctly
- Schema drift issues are properly surfaced to users
- No blockers

---
*Phase: quick*
*Completed: 2026-02-02*
