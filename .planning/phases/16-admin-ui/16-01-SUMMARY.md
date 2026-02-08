---
phase: 16
plan: 01
subsystem: admin-ui
tags: [api, typescript, layout, backend, frontend, data-quality]

requires:
  - 15-03  # Background scanning API endpoints needed for scan job types

provides:
  - ListAllPending backend endpoint for review queue org-wide alerts
  - Comprehensive TypeScript API client for all dedup/merge/scan endpoints
  - Admin dashboard navigation to Data Quality section
  - Data Quality layout with tab navigation structure

affects:
  - 16-02  # Review Queue page will import listPendingAlerts from data-quality.ts
  - 16-03  # Merge Wizard will import merge functions from data-quality.ts
  - 16-04  # Merge History will import mergeHistory from data-quality.ts
  - 16-05  # Scan Jobs Dashboard will import scan job functions from data-quality.ts

tech-stack:
  added:
    - None (used existing Go/Fiber backend, SvelteKit frontend)
  patterns:
    - Paginated backend endpoints with query param filtering
    - Shared TypeScript API client with comprehensive type exports
    - Tab navigation layout with Svelte 5 runes
    - Admin dashboard card-based navigation

key-files:
  created:
    - backend/internal/repo/pending_alert.go (added ListAllPending method)
    - backend/internal/handler/dedup.go (added ListPendingAlerts handler)
    - frontend/src/lib/api/data-quality.ts
    - frontend/src/routes/admin/data-quality/+layout.svelte
  modified:
    - frontend/src/routes/admin/+page.svelte (added Data Quality section)

decisions:
  - decision: "ListAllPending sorts by highest_confidence DESC, detected_at DESC"
    rationale: "Highest confidence alerts first per user decisions from Phase 12 context"
    date: 2026-02-08
  - decision: "data-quality.ts re-exports utilities from dedup.ts"
    rationale: "Avoid duplication of getBannerClass, formatConfidence functions"
    date: 2026-02-08
  - decision: "PaginatedResponse generic type for all paginated endpoints"
    rationale: "Consistent pagination interface across rules, alerts, merge history, scan jobs"
    date: 2026-02-08

metrics:
  duration: 195s
  completed: 2026-02-08
---

# Phase 16 Plan 01: Foundation & API Client Summary

**One-liner:** ListAllPending backend endpoint with pagination, comprehensive TypeScript API client covering 16 endpoints, and Data Quality admin hub wiring with tab navigation.

## What Was Built

### Backend Foundation
Added the missing `ListAllPending` method to `PendingAlertRepo`:
- Queries `pending_duplicate_alerts` table with `org_id` and `status = 'pending'`
- Optional `entityType` filter for reviewing specific entity duplicates
- Orders by `highest_confidence DESC, detected_at DESC` (highest confidence first)
- Returns paginated results with total count for pagination metadata
- Parses JSON matches column into `[]entity.PendingDuplicateAlert`

Added `ListPendingAlerts` HTTP handler to `DedupHandler`:
- Parses query params: `entityType` (optional), `page` (default 1), `pageSize` (default 20, max 100)
- Returns JSON: `{ data: [...], total: N, page: N, pageSize: N }`
- Registered at `GET /dedup/pending-alerts` on admin route group

### Frontend API Client
Created comprehensive `data-quality.ts` TypeScript client:

**Types defined (17 types):**
- `MatchingRule`, `DedupFieldConfig`, `MatchingRuleCreateInput`, `MatchingRuleUpdateInput`
- `PaginatedResponse<T>` (generic for all paginated endpoints)
- `MergePreviewRequest`, `MergePreview`, `MergeRequest`, `MergeResult`, `MergeHistoryEntry`, `RelatedRecordCount`
- `ScanSchedule`, `ScheduleInput`, `ScanJob`, `ScanCheckpoint`, `ProgressEvent`
- Re-exported `PendingAlert`, `DuplicateMatch`, `MatchResult` from dedup.ts

**API functions (16 endpoints):**
- Rules: `listRules`, `getRule`, `createRule`, `updateRule`, `deleteRule`, `checkDuplicates`
- Pending Alerts: `listPendingAlerts` (with pagination and entityType filter)
- Merge: `mergePreview`, `mergeExecute`, `mergeUndo`, `mergeHistory`
- Scan Jobs: `listSchedules`, `upsertSchedule`, `deleteSchedule`, `listJobs`, `triggerManualScan`, `retryJob`

**Utilities re-exported:**
- `getBannerClass`, `getConfidenceBadgeClass`, `formatConfidence` from dedup.ts

### Admin Hub Wiring
Created Data Quality section navigation:
- New layout at `/admin/data-quality/+layout.svelte` with tab bar
- Four tabs: Duplicate Rules, Review Queue, Merge History, Scan Jobs
- Active tab detection using Svelte 5 `$derived` and `$page` store
- Tailwind-styled tabs with blue underline for active tab

Updated admin dashboard:
- Added "Data Quality" section header between Customization and Automation
- Card link to `/admin/data-quality` with shield-check icon
- Emerald border (`border-emerald-500`) per plan specifications
- Description: "Manage duplicate detection rules, review queue, and merge operations"

## Task Commits

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Add ListAllPending backend endpoint | 90ab9a5 | backend/internal/repo/pending_alert.go, backend/internal/handler/dedup.go |
| 2 | Create data-quality API client and admin hub wiring | b5c385b | frontend/src/lib/api/data-quality.ts, frontend/src/routes/admin/data-quality/+layout.svelte, frontend/src/routes/admin/+page.svelte |

## Decisions Made

### 1. Pagination Strategy
**Decision:** Default page size 20, max 100, with page/pageSize query params
**Rationale:** Consistent with existing pagination patterns in merge history and scan jobs. Max 100 prevents excessive memory/rendering overhead
**Alternatives considered:** Cursor-based pagination (more complex, not needed for admin UI)

### 2. API Client Organization
**Decision:** Single comprehensive data-quality.ts file for all Phase 16 endpoints
**Rationale:** Shared types across plans (e.g., MergePreview used by both review queue and merge wizard). Avoids duplication and import complexity
**Alternatives considered:** Separate files per subdomain (rules.ts, merge.ts, scan-jobs.ts) — would duplicate types like PaginatedResponse

### 3. Type Re-exports
**Decision:** Re-export PendingAlert, DuplicateMatch from dedup.ts instead of duplicating
**Rationale:** Single source of truth for alert types. dedup.ts already used by Phase 12 (real-time detection), ensures consistency
**Alternatives considered:** Duplicate types in data-quality.ts (violates DRY, causes drift)

## Deviations from Plan

None — plan executed exactly as written.

## Next Phase Readiness

**Ready to proceed to 16-02 (Review Queue page):**
- `listPendingAlerts` endpoint available with pagination and filtering
- `PendingAlert` type exported from data-quality.ts
- Admin hub navigation link exists at `/admin/data-quality`
- Tab navigation layout ready to receive child route components

**Ready to proceed to 16-03 (Merge Wizard):**
- `mergePreview`, `mergeExecute`, `mergeUndo` functions exported
- `MergePreview`, `MergeRequest`, `MergeResult` types defined
- Related record types (`RelatedRecordCount`) available

**Ready to proceed to 16-04 (Merge History):**
- `mergeHistory` function with pagination
- `MergeHistoryEntry` type with undo metadata

**Ready to proceed to 16-05 (Scan Jobs Dashboard):**
- `listSchedules`, `listJobs`, `triggerManualScan`, `retryJob` functions
- `ScanSchedule`, `ScanJob`, `ProgressEvent` types for SSE

## Testing Notes

**Backend verification:**
- `go build ./...` passed without errors
- ListAllPending method compiles with proper pagination and filtering logic
- Route registered at GET /dedup/pending-alerts

**Frontend verification:**
- data-quality.ts created with 371 lines of types and functions
- Layout file created with tab navigation (44 lines)
- Admin dashboard updated with Data Quality section
- TypeScript types compile (pre-existing auth.svelte.ts errors unrelated to this plan)

**Manual testing needed (16-02+):**
- Verify listPendingAlerts returns correct data format
- Test pagination params (page, pageSize, entityType filter)
- Verify tab navigation active state detection
- Confirm admin dashboard card link navigates correctly

## Performance Impact

**Database:**
- New query pattern: SELECT with ORDER BY highest_confidence DESC, detected_at DESC
- Uses existing pending_duplicate_alerts table index on (org_id, status)
- LIMIT/OFFSET pagination prevents full table scan

**Frontend:**
- Single 15KB data-quality.ts file (tree-shakable, only imports used endpoints)
- No runtime overhead (pure TypeScript types compile away)
- Tab navigation uses reactive Svelte 5 $derived, no polling

## Future Considerations

1. **Index optimization:** If review queue becomes slow with large orgs, consider composite index on (org_id, status, highest_confidence, detected_at)

2. **Cursor pagination:** Current offset-based pagination fine for admin UI (low traffic), but cursor-based could improve efficiency for very large result sets

3. **SSE types:** ProgressEvent type defined but SSE connection logic left for 16-05 (Scan Jobs Dashboard)

4. **Bulk operations:** Review queue will need bulk actions (Merge All, Dismiss All) — types ready, endpoints TBD in 16-02

---

**Phase 16 Plan 01 complete.** Foundation layer in place for all subsequent admin UI plans. Backend endpoint, comprehensive API client, and navigation wiring ready for Review Queue, Merge Wizard, Merge History, and Scan Jobs pages.

## Self-Check: PASSED

All created files verified to exist:
- backend/internal/repo/pending_alert.go
- backend/internal/handler/dedup.go
- frontend/src/lib/api/data-quality.ts
- frontend/src/routes/admin/data-quality/+layout.svelte

All commits verified in git history:
- 90ab9a5 (Task 1: ListAllPending backend endpoint)
- b5c385b (Task 2: data-quality API client and admin hub wiring)
