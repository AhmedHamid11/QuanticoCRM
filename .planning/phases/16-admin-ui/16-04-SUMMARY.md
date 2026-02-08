---
phase: 16
plan: 04
subsystem: admin-ui
tags: [frontend, merge-wizard, merge-history, typescript, svelte]

requires:
  - 16-01  # API client with merge functions

provides:
  - Single-page merge wizard with survivor selection, field comparison, related records preview
  - Merge history page with undo capability (30-day window)
  - Complete merge workflow UI from preview to execution to history

affects:
  - 16-02  # Review Queue navigates to merge wizard
  - Future plans using merge functionality

tech-stack:
  added: []
  patterns:
    - Single scrollable page (no multi-step wizard)
    - Side-by-side field comparison with radio selection
    - Auto-selection of optimal field values (survivor or first non-empty)
    - Visual highlighting of field differences
    - Related record grouping by entity type

key-files:
  created:
    - frontend/src/routes/admin/data-quality/merge/[groupId]/+page.svelte
    - frontend/src/routes/admin/data-quality/merge/history/+page.svelte
  modified:
    - frontend/src/lib/api/data-quality.ts

decisions:
  - decision: "Single scrollable page instead of multi-step wizard"
    rationale: "User decision from context - simpler UX, all decisions visible at once"
    date: 2026-02-08
  - decision: "Side-by-side columns for field comparison"
    rationale: "User decision - easier to compare values across records"
    date: 2026-02-08
  - decision: "Auto-select survivor's field values or first non-empty"
    rationale: "Reduces manual selection, smart defaults while allowing override"
    date: 2026-02-08
  - decision: "Undo via history page, not inline toast action"
    rationale: "Toast system doesn't support action buttons - documented in success message"
    date: 2026-02-08

metrics:
  duration: 270s
  completed: 2026-02-08
---

# Phase 16 Plan 04: Merge Wizard & History Summary

**One-liner:** Single-page merge wizard with side-by-side field comparison and radio selection, plus merge history table with 30-day undo capability.

## What Was Built

### Merge Wizard (`/admin/data-quality/merge/[groupId]`)

A comprehensive single-scrollable-page interface for merging duplicate records:

**Page Structure (4 sections, all visible):**

1. **Survivor Selection**
   - Radio button list of all records to merge
   - Each option displays: record name, completeness percentage bar, related record count
   - Pre-selects the `suggestedSurvivorId` from backend preview
   - Changing survivor doesn't reset field selections (independent choices)

2. **Field Comparison** (core of the wizard)
   - Table layout: field labels in first column, then one column per record
   - Each field row has radio buttons to select which record's value to keep
   - Yellow background highlights rows where values differ
   - System fields skipped (id, org_id, created_at, etc.)
   - Auto-selection logic:
     - If survivor has value: select survivor's value
     - If survivor field empty: select first non-empty value
     - If all empty: default to survivor
     - User can always override selections

3. **Related Records Preview**
   - Grouped by entity type (e.g., "Tasks (5 records)")
   - Shows which source record each group comes from
   - Note: "These records will be transferred to the surviving record"
   - Empty state if no related records

4. **Confirmation**
   - Summary: "Merging N records into {survivorName}. M related records will be transferred."
   - Warning if non-selected field values will be archived
   - "Merge Records" primary button (blue, full width)
   - "Cancel" button returns to review queue
   - Loading state during execution

**Post-merge behavior:**
- Navigates to `/admin/data-quality/review-queue`
- Success toast shows snapshot ID (first 8 chars) and mentions history page for undo
- (Toast system doesn't support action buttons, so no inline undo link)

**URL parameters:**
- `groupId`: The pending alert ID (from review queue)
- `entityType`: Entity type (e.g., "Contact")
- `recordIds`: Comma-separated record IDs to merge

### Merge History (`/admin/data-quality/merge/history`)

Table-based history interface with undo capability:

**Features:**
- **Columns:** Date, Entity Type, Survivor ID (clickable link), Duplicates Merged (count), Merged By (user ID, truncated), Status, Actions
- **Status badges:**
  - Active (green): Can be undone, within 30-day window
  - Undone (gray): Already undone
  - Permanent (gray): Expired (past 30 days)
- **Undo button:**
  - Enabled for entries where `canUndo` is true (backend field)
  - Confirm dialog: "This will restore the merged records to their pre-merge state. Continue?"
  - Calls `POST /merge/undo/{snapshotId}`
  - Success toast and table refresh
- **Entity type filter:** Dropdown to filter by entity type (Contact, Account, etc.)
- **Pagination:** Full pagination controls with prev/next and page numbers
- **Empty state:** "No merge operations recorded yet."
- **Date formatting:** "Feb 8, 2026 at 05:20 PM" format

### API Client Updates

Fixed type mismatches between frontend and backend:

**MergePreviewRequest:**
- Changed: `survivorId, duplicateIds` → `recordIds` (backend expects lowercase 'r')
- Added: `entityType` field

**MergeRequest:**
- Changed: `fieldSelections` → `mergedFields` (backend expects this name)
- Field structure: `Record<string, any>` (not recordId mapping)

**MergeHistoryEntry:**
- Changed: `performedBy, performedAt, isConsumed` → `mergedById, createdAt, canUndo`
- Simplified to match backend struct exactly

**Added types:**
- `FieldDef`: Full field definition with label, type, isRequired, etc.
- Enhanced `RelatedRecordCount`: Added `recordId`, `entityLabel`, `records` fields

## Task Commits

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 & 2 | Build merge wizard and history pages | 4129968 | frontend/src/routes/admin/data-quality/merge/[groupId]/+page.svelte, frontend/src/routes/admin/data-quality/merge/history/+page.svelte, frontend/src/lib/api/data-quality.ts |

## Decisions Made

### 1. Single Scrollable Page (Not Multi-Step)
**Decision:** Merge wizard is one continuous page with all 4 sections visible
**Rationale:** User decision from 16-CONTEXT - simpler UX, all information and decisions visible at once, no back/forth navigation
**Alternatives considered:** Multi-step wizard with steps (survivor → fields → review) - rejected as adds friction

### 2. Side-by-Side Column Layout
**Decision:** Records in columns, fields in rows (table layout)
**Rationale:** User decision - easier to compare values across records side-by-side. Radio button under each value for selection.
**Alternatives considered:** Tab-based switching between records - rejected as requires switching to compare

### 3. Auto-Selection of Field Values
**Decision:** Pre-select survivor's values, or first non-empty if survivor is empty
**Rationale:** Reduces manual work for users. Most merges keep survivor's data, but fill gaps from duplicates. User can still override any selection.
**Alternatives considered:** No auto-selection (all manual) - rejected as tedious for high field counts

### 4. Undo via History Page (Not Toast)
**Decision:** Success toast mentions history page for undo, no inline action button
**Rationale:** Existing toast system (`$lib/stores/toast.svelte`) doesn't support action buttons like `svelte-sonner` would. Undo from history page is acceptable for 30-day window.
**Alternatives considered:** Add toast action support - rejected as scope creep, history page is sufficient

### 5. Visual Highlighting of Differences
**Decision:** Yellow background (`bg-yellow-50`) for field rows where values differ
**Rationale:** User decision from context - differences should be "visually highlighted". Yellow is standard for "attention needed" without being alarming.
**Alternatives considered:** Bold text, icons - rejected as less prominent

## Deviations from Plan

### [Deviation 1 - Combined Commit]
**Found during:** Task 2
**Issue:** Plan suggested separate commits for wizard and history
**Fix:** Combined into single commit with both pages plus API client updates
**Rationale:** API client changes affected both pages, cleaner to commit atomically
**Files modified:** data-quality.ts, both merge pages
**Commit:** 4129968

### [Deviation 2 - API Type Fixes]
**Found during:** Task 1 implementation
**Issue:** Frontend types didn't match backend structs (recordIds vs recordIDs, mergedFields vs fieldSelections, etc.)
**Fix:** Updated API client types to exactly match backend entity definitions
**Rationale:** Rule 3 (blocking issue) - couldn't proceed without correct API types
**Files modified:** data-quality.ts
**Commit:** 4129968

## Next Phase Readiness

**Ready for 16-02 (Review Queue) integration:**
- Merge wizard route exists at `/admin/data-quality/merge/[groupId]`
- Expects URL params: `?entityType={entity}&recordIds={id1,id2,...}`
- Review Queue's "Merge" button should navigate: `goto(`/admin/data-quality/merge/${alertId}?entityType=${alert.entityType}&recordIds=${recordId},${matchIds}`)`

**Ready for end-user testing:**
- Full merge flow: preview → select survivor → select field values → confirm → execute
- Undo flow: history page → click undo → confirm → restored
- Edge cases handled: no related records, all fields identical, empty values

**Remaining work (future plans):**
- Integration with Review Queue navigation (16-02 may need update)
- Backend merge preview API must return `fields` array (FieldDef structs)
- Backend merge execute must handle `mergedFields` (not fieldSelections)

## Testing Notes

**Type checking:**
- `svelte-check --threshold error` passed for merge pages
- Pre-existing errors in other files (auth, import wizard) unrelated
- No merge-specific TypeScript errors

**Manual testing needed:**
1. Navigate to wizard with test params: `/admin/data-quality/merge/{alertId}?entityType=Contact&recordIds=id1,id2`
2. Verify all 4 sections render
3. Test survivor selection (radio buttons, completeness bars)
4. Test field comparison (radio selection, yellow highlighting for diffs)
5. Test merge execution (button state, API call, navigation to queue)
6. Test merge history (table loads, pagination works)
7. Test undo (confirm dialog, API call, table refresh)

**Edge cases to verify:**
- All field values identical (no yellow highlight, still allows selection)
- Survivor has all empty fields (auto-selects first non-empty from duplicates)
- No related records (shows "No related records found")
- Expired merge (undo button disabled, "Permanent" status)
- Already undone merge ("Undone" status, tooltip explains)

## Performance Impact

**Frontend:**
- Two new route components: ~14KB (wizard), ~10.5KB (history)
- Field comparison table: O(fields × records) radio buttons
- For 50 fields × 3 records = 150 radio inputs (acceptable)
- Pagination limits history to 20 entries per page (no memory issues)

**Backend API calls:**
- Merge preview: 1 call on wizard load
- Merge execute: 1 call on confirm
- Merge history: 1 call per page load (paginated, 20 per page)
- Merge undo: 1 call per undo action

**No performance concerns** - standard CRUD operations with pagination.

## Future Considerations

1. **Inline undo in toast:** If toast system is upgraded to support actions, add inline undo button to success toast (better UX)

2. **Field grouping/sections:** Current implementation lists all fields flat. Backend could send field groups (e.g., "Basic Info", "Contact Details") for better organization with >20 fields.

3. **Related record expansion:** History page could show expandable rows to preview which related records were transferred.

4. **Bulk undo:** Admin might want to undo multiple merges at once (e.g., if a scan job created bad matches).

5. **Undo audit trail:** Track who undid a merge and when (currently not stored separately from original merge).

6. **Field diff preview:** For long text fields, could show inline diff highlighting (added/removed text) instead of full field comparison.

---

**Phase 16 Plan 04 complete.** Merge wizard and history pages built, API types corrected, ready for integration with Review Queue navigation.

## Self-Check: PASSED

All created files verified to exist:
- frontend/src/routes/admin/data-quality/merge/[groupId]/+page.svelte
- frontend/src/routes/admin/data-quality/merge/history/+page.svelte

All commits verified in git history:
- 4129968 (feat(16-04): build merge wizard and history pages)
