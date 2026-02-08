# Phase 16: Admin UI - Context

**Gathered:** 2026-02-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Complete admin interface for the deduplication system: matching rule management, duplicate review queue, merge wizard, bulk merge operations, merge history with undo, and scan job management. All backend APIs exist from Phases 11-15 — this phase builds the frontend UI that consumes them.

</domain>

<decisions>
## Implementation Decisions

### Review Queue Layout
- Card-based groups — each duplicate group displayed as a card showing matched records with confidence score
- Default sort by confidence score (highest first), with entity type filter
- Inline actions on each card: Dismiss (not duplicates) and Quick Merge (auto-pick best fields). Full merge opens the merge wizard
- Checkbox selection on each card with floating bulk action bar (Merge All, Dismiss All) when items selected

### Merge Wizard Flow
- Single scrollable page — no multi-step wizard. All sections visible: survivor selection, field comparison, related records, confirm
- Side-by-side columns for field comparison — records in columns, fields in rows. Click/radio to select which value to keep. Differences highlighted
- Related records (tasks, notes, activities) shown as full list always visible, grouped by type with source record indicated
- After merge completes: return to review queue with merged group removed. Success toast with Undo link (30-day window)

### Rule Management UX
- List with inline editing — table of rules per entity type, click to expand inline for editing
- Field configuration via dropdown: select field, choose matching type (exact, fuzzy, phonetic), set weight
- "Test Rule" button runs the rule against existing data and shows sample matches with scores for threshold tuning
- Confidence thresholds configured via numeric inputs with color-coded tier labels (High/Medium/Low)

### Scan Job Dashboard
- Simple status table: columns for entity type, schedule, last run, status, next run. Sorted by next run time
- Running scans show inline progress bar directly in the table row with percentage and records processed
- Schedule configuration via preset dropdown options: Daily, Weekly (pick day), Monthly (pick date)
- Failed scans trigger in-app notification. Admin goes to dashboard to see details and manually trigger retry from last checkpoint

### Claude's Discretion
- Exact card component styling and spacing
- Empty state designs for review queue (no duplicates found)
- Loading states and skeleton screens
- Table pagination approach for large result sets
- Responsive behavior for smaller screens
- Toast/notification component implementation details
- Error handling patterns across all UI components

</decisions>

<specifics>
## Specific Ideas

- Review queue cards should show enough info to make quick dismiss/merge decisions without opening the wizard
- Quick merge on cards is for obvious duplicates — full wizard for anything that needs careful field selection
- Test rule feature is important for admin confidence when tuning thresholds
- Failure handling for scans is notification-driven, not dashboard-polling — admin gets notified and comes to dashboard

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 16-admin-ui*
*Context gathered: 2026-02-08*
