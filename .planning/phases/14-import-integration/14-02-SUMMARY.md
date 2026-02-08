---
phase: 14-import-integration
plan: 02
subsystem: ui
tags: [svelte, typescript, import, duplicate-detection, ui-workflow]

# Dependency graph
requires:
  - phase: 14-01
    provides: Import duplicate detection service and check-duplicates API endpoint
provides:
  - ImportWizard step 2.75 duplicate review UI with side-by-side comparison
  - Resolution state management (skip, update, import, merge actions)
  - Bulk actions (Skip All, Import All)
  - Within-file duplicate group selection
  - "All clear" toast for zero-duplicate case
affects: [14-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Side-by-side comparison table for duplicate review"
    - "Color-coded confidence tiers (red=high, yellow=medium, blue=low)"
    - "Default resolutions based on confidence score"

key-files:
  created: []
  modified:
    - frontend/src/lib/components/ImportWizard.svelte

key-decisions:
  - "High confidence (>=95%) defaults to Skip action, medium defaults to Import Anyway"
  - "Bulk actions only affect unresolved rows (don't override user decisions)"
  - "Within-file selections default to first row in each group (group.keepIndex)"
  - "All clear message auto-proceeds to import after 2-second delay"
  - "Merge button opens merge wizard in new tab"

patterns-established:
  - "Step 2.75 inserted between validation (2) and import (3)"
  - "Resolutions stored in Map<number, ImportResolution> for reactivity"
  - "Proceed button disabled until allResolved() returns true"

# Metrics
duration: 3min
completed: 2026-02-08
---

# Phase 14 Plan 02: Import Duplicate Review UI Summary

**ImportWizard step 2.75 with side-by-side duplicate comparison, four resolution actions, bulk actions, and within-file group selection**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-08T02:07:50Z
- **Completed:** 2026-02-08T02:11:03Z
- **Tasks:** 3
- **Files modified:** 1

## Accomplishments

- Added duplicate review step (2.75) to ImportWizard between validation and import
- Side-by-side comparison table showing import row vs existing record with matched fields highlighted
- Four resolution actions per row: Skip, Update Existing, Import Anyway, Merge
- Bulk actions for mass resolution: Skip All Remaining, Import All Remaining
- Within-file duplicate groups with radio selection for which row to keep
- Default resolutions based on confidence score (>=95% = skip, else import)
- "All clear" green toast when no duplicates detected, auto-proceeds after 2 seconds
- Error handling with retry/skip options

## Task Commits

Each task was committed atomically:

1. **Task 1: Add TypeScript interfaces and state for duplicate review** - `be0df96` (feat)
2. **Task 2: Add duplicate check and resolution logic functions** - `07f7985` (feat)
3. **Task 3: Add duplicate review step UI template** - `97c00f8` (feat)

## Files Created/Modified

- `frontend/src/lib/components/ImportWizard.svelte` - Added step 2.75 duplicate review UI with:
  - TypeScript interfaces: ImportMatchCandidate, ImportDuplicateMatch, ImportDuplicateGroup, DuplicateCheckResult, ImportResolution
  - State: duplicateResult, resolutions, withinFileSelections, checkingDuplicates, duplicateCheckError, showAllClear
  - Functions: checkDuplicates(), setResolution(), setWithinFileSelection(), bulkResolve(), allResolved(), proceedToImport(), getConfidenceColor()
  - Step 2.75 template: Header with progress, database matches in side-by-side tables, within-file groups with radio buttons, navigation with disabled proceed until resolved
  - Step 2 modified: "Check for Duplicates" button instead of direct import, "Skip Duplicate Check" option
  - Loading/error states: spinner during check, error with retry/skip, green "all clear" toast

## Decisions Made

1. **High confidence defaults to Skip:** Rows with >=95% confidence score default to Skip action to prevent accidental duplicates
2. **Medium confidence defaults to Import Anyway:** Rows with <95% confidence default to Import Anyway to preserve data
3. **Bulk actions don't override user decisions:** Skip All and Import All only affect unresolved rows, respecting manual selections
4. **Within-file selections use group.keepIndex:** Default selection is the first row in each duplicate group
5. **All clear auto-proceeds:** When zero duplicates detected, green toast shows for 2 seconds then auto-advances to import step
6. **Merge opens new tab:** Merge button opens merge wizard in new tab to preserve import wizard state

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for Plan 14-03 (import execution with resolutions). The following state is available for the import handler:
- `resolutions: Map<number, ImportResolution>` - user decisions for each flagged database match
- `withinFileSelections: Map<string, number>` - selected row index for each within-file duplicate group

Blocker: Plan 14-03 must implement import execution that:
- Skips rows where resolution.action === 'skip'
- Updates existing record where resolution.action === 'update' with resolution.selectedMatchId
- Imports as new record where resolution.action === 'import'
- Marks for merge where resolution.action === 'merge'
- Filters within-file rows to only keep selected indices from withinFileSelections

## Self-Check: PASSED

All files and commits verified.

---
*Phase: 14-import-integration*
*Completed: 2026-02-08*
