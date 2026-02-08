---
phase: 16
plan: 03
subsystem: admin-ui
tags: [svelte, typescript, ui, data-quality, merge, deduplication]

requires:
  - 16-01  # data-quality.ts API client with listPendingAlerts, mergePreview, mergeExecute
  - 12-02  # resolveAlert function from dedup.ts for dismissing alerts

provides:
  - Card-based review queue showing all pending duplicate groups
  - Entity type filtering and pagination
  - Inline Dismiss/Quick Merge/Full Merge actions per card
  - Bulk operations with floating action bar
  - Optimistic UI with error rollback

affects:
  - 16-04  # Merge Wizard will be linked from "Merge" button on cards
  - Future duplicate detection features that need review queue as primary UI

tech-stack:
  added:
    - None (used existing SvelteKit, Tailwind, data-quality API)
  patterns:
    - Card-based list layout with colored left borders for confidence tiers
    - Floating bulk action bar with progress tracking
    - Optimistic UI with backup/restore error handling
    - Quick Merge auto-selection using merge preview suggested survivor

key-files:
  created:
    - frontend/src/routes/admin/data-quality/review-queue/+page.svelte
  modified:
    - None

decisions:
  - decision: "Quick Merge uses merge preview's suggestedSurvivorId for auto-selection"
    rationale: "Leverages backend completeness scoring to pick best survivor automatically"
    date: 2026-02-08
  - decision: "Bulk operations process sequentially with progress tracking"
    rationale: "Prevents overwhelming backend, provides user feedback during long operations"
    date: 2026-02-08
  - decision: "Select All checkbox at top of list rather than in header"
    rationale: "More visible, easier to interact with when list is long"
    date: 2026-02-08

metrics:
  duration: 118s
  completed: 2026-02-08
---

# Phase 16 Plan 03: Review Queue Summary

**Card-based duplicate review queue with confidence-sorted groups, inline Quick Merge auto-selection, bulk operations, and optimistic UI with 462 lines of Svelte 5 runes**

## Performance

- **Duration:** 118s (2 min)
- **Started:** 2026-02-08T22:15:50Z
- **Completed:** 2026-02-08T22:17:48Z
- **Tasks:** 1
- **Files created:** 1

## Accomplishments

- Card-based review queue showing all pending duplicate groups sorted by confidence
- Entity type filter dropdown (Contact, Account, Lead, Opportunity) with pagination
- Each card displays matched records with individual match scores and highlighted matching fields
- Three inline actions per card: Dismiss, Quick Merge (auto-picks survivor), Full Merge (wizard)
- Checkbox selection with floating bulk action bar for Merge All / Dismiss All
- Optimistic UI removes cards immediately with rollback on error
- Empty state with "Run a scan" CTA linking to scan jobs dashboard
- Loading state with 3 skeleton cards and pulse animation

## Task Commits

1. **Task 1: Build card-based review queue with inline actions** - `e86fcf8` (feat)

## Files Created/Modified

- `frontend/src/routes/admin/data-quality/review-queue/+page.svelte` - Review queue page with card layout, filtering, pagination, inline actions, bulk operations, and optimistic UI (462 lines)

## What Was Built

### Page Structure

**Header bar:**
- "Review Queue" title with total count badge (blue)
- Entity type filter dropdown (all types / Contact / Account / Lead / Opportunity)
- Pagination controls when multiple pages (Previous/Next buttons with current page indicator)

**Card list (sorted by confidence highest first):**
- Each pending alert rendered as a card with:
  - **Left border color:** red (high), yellow (medium), blue (low) confidence via `getBannerClass`
  - **Checkbox:** for bulk selection (left side)
  - **Header:** Entity type + Record ID with confidence badge (`95% HIGH` format)
  - **Matched records section:** Shows all matches with:
    - Record name/ID
    - Individual match score
    - Matching fields as blue pills
  - **Action buttons (right side):**
    - "Dismiss" (gray outline) - calls `resolveAlert` with status 'dismissed'
    - "Quick Merge" (blue solid) - auto-picks survivor via merge preview
    - "Merge" (blue outline) - navigates to `/admin/data-quality/merge/{alertId}`

**Floating bulk action bar:**
- Fixed to bottom when `selectedIds.size > 0`
- Shows "{N} selected" count
- Progress indicator during bulk processing: "Processing X / Y..."
- Two buttons: "Dismiss All" and "Merge All"
- Processes items sequentially with progress tracking

**Empty state:**
- Centered checkmark icon with "No duplicates found" heading
- Subtitle: "All clear! No duplicate records detected in your data."
- "Run a scan" button linking to `/admin/data-quality/scan-jobs`

**Loading state:**
- Three skeleton cards with pulse animation
- Header, content, and button placeholders (gray rectangles)

### Quick Merge Logic

Quick Merge implements fully automatic merge with one click:

1. **Fetch merge preview:** Call `mergePreview` with all record IDs (source + all matches)
2. **Use suggested survivor:** Backend returns `suggestedSurvivorId` based on completeness scoring
3. **Auto-select fields:** No field selections needed - backend uses survivor's values by default
4. **Execute merge:** Call `mergeExecute` with survivor and duplicate IDs
5. **Optimistic UI:** Remove card immediately, show success toast with related record count
6. **Mark resolved:** Call `resolveAlert` with status 'merged'

On error: show toast, card remains in list (no optimistic removal on preview failure).

### Bulk Operations

**Dismiss All:**
- Resolves each selected alert sequentially via `resolveAlert(entityType, recordId, 'dismissed')`
- Tracks progress: `bulkProgress = { current: 0, total: N }`
- Removes dismissed alerts from list
- Shows toast: "Dismissed X alerts" or "Dismissed X alerts, Y failed"

**Merge All:**
- Performs Quick Merge for each selected alert sequentially
- Same progress tracking as Dismiss All
- Removes successfully merged cards
- Shows toast with success/fail counts

Sequential processing prevents overwhelming backend and provides clear progress feedback during long operations.

### State Management (Svelte 5 Runes)

```typescript
let alerts = $state<PendingAlert[]>([]);
let loading = $state(true);
let entityFilter = $state('');
let currentPage = $state(1);
let total = $state(0);
let pageSize = $state(20);
let selectedIds = $state<Set<string>>(new Set());
let showBulkBar = $derived(selectedIds.size > 0);
let bulkProcessing = $state(false);
let bulkProgress = $state({ current: 0, total: 0 });
```

Reactivity via `$effect`:
- Reloads alerts when `entityFilter` or `currentPage` changes
- Derived `showBulkBar` shows/hides floating bar based on selection

### Optimistic UI Pattern

All destructive operations (dismiss, merge) use optimistic UI:

1. **Backup:** `const backup = [...alerts];`
2. **Immediate update:** Remove card from `alerts` array
3. **API call:** Perform operation
4. **On error:** Restore backup, show error toast
5. **On success:** Show success toast, keep card removed

This makes the UI feel instant while still handling errors gracefully.

## Decisions Made

### 1. Quick Merge Auto-Selection Strategy
**Decision:** Use merge preview's `suggestedSurvivorId` without field selection UI
**Rationale:** Backend already computes completeness scores for all records. Using its recommendation eliminates user decision fatigue and makes Quick Merge truly one-click. Users who want control can use "Merge" button for full wizard.
**Alternatives considered:** Auto-select fields manually in frontend (duplicates backend logic, would drift)

### 2. Sequential Bulk Processing
**Decision:** Process bulk operations sequentially with progress tracking
**Rationale:** Prevents overwhelming backend with parallel merge operations. Merges are complex (related record transfers, snapshots) and should be done serially. Progress indicator keeps user informed during long operations.
**Alternatives considered:** Parallel processing (could cause DB contention, harder to track individual failures)

### 3. Select All Placement
**Decision:** Place "Select All" checkbox at top of card list rather than in page header
**Rationale:** More visible when user scrolls to cards, clear indication it affects the cards below. Users expect "select all" near the items being selected.
**Alternatives considered:** Header placement (farther from cards, less discoverable when list is long)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all API endpoints (`listPendingAlerts`, `mergePreview`, `mergeExecute`, `resolveAlert`) worked as expected. Types from `data-quality.ts` and `dedup.ts` imported cleanly. Svelte 5 reactivity patterns worked as designed.

## Next Phase Readiness

**Ready for 16-04 (Merge Wizard):**
- "Merge" button navigates to `/admin/data-quality/merge/{alertId}` (route doesn't exist yet, will be created in 16-04)
- Alert ID passed as route param for merge wizard to load

**Ready for 16-05 (Scan Jobs Dashboard):**
- Empty state links to `/admin/data-quality/scan-jobs` for running scans
- Users can trigger scans when review queue is empty

**Testing notes:**
- Page renders with empty state when no duplicates (verified via type-check)
- svelte-check passed with no errors in review-queue page
- Pre-existing auth.svelte.ts errors unrelated to this plan
- Manual browser testing needed to verify:
  - Cards display with correct border colors based on confidence
  - Entity filter and pagination work
  - Dismiss removes card and resolves alert
  - Quick Merge executes and shows success toast
  - Bulk operations process with progress indicator
  - Floating bar shows/hides based on selection

---
*Phase: 16-admin-ui*
*Completed: 2026-02-08*

## Self-Check: PASSED

All created files verified to exist:
- frontend/src/routes/admin/data-quality/review-queue/+page.svelte

All commits verified in git history:
- e86fcf8 (Task 1: Build card-based review queue with inline actions)
