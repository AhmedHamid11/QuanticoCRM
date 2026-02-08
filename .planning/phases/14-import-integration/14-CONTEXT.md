# Phase 14: Import Integration - Context

**Gathered:** 2026-02-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend the existing CSV import flow to detect and handle duplicates during import. This includes detecting matches against existing database records, detecting matches between rows within the import file, and providing a resolution workflow before import proceeds. The merge wizard (Phase 13) and matching rules engine (Phase 11) already exist — this phase integrates them into the import pipeline.

</domain>

<decisions>
## Implementation Decisions

### Duplicate Presentation
- Separate review step after field mapping/analyze — not inline flags on rows
- Side-by-side table comparison: import row on the left, matched existing record(s) on the right, fields aligned
- Review step shows ONLY flagged rows (not all import rows) — counter shows "X of Y rows need review"
- Each flagged row shows confidence score AND highlights which fields matched (name, email, phone) so user sees WHY it's a match
- Clean (non-duplicate) rows import automatically without user intervention

### Resolution Actions
- Four options: Skip, Update, Import Anyway, Merge
- "Update" means overwrite all fields on the existing record with the import row's values
- "Merge" links to the full Phase 13 merge wizard in a modal/new tab, pre-loaded with import row + matched record
- When a row has multiple potential matches: show top-confidence match by default, expandable list to see/switch to other matches
- Default resolution based on confidence: high confidence (>=95%) defaults to "Skip", medium defaults to "Import anyway" — user can override any default
- Bulk actions available: "Skip All Remaining" or "Import All Remaining" to speed through unresolved rows

### Within-File Duplicates
- Rows that duplicate each other within the CSV are grouped together in the review step
- Uses the same org matching rules as database detection (Phase 11 rules) — consistent behavior
- Resolution: user picks which row from the group to keep, others are skipped
- If a row matches both another file row AND an existing DB record, show both matches — user must resolve both

### Import Flow
- Duplicate detection runs as a separate step AFTER the analyze/validation step — not combined
- If zero duplicates detected, still show the step briefly with green "all clear" message — builds confidence
- Strict blocking: all flagged rows must have a resolution before import proceeds, BUT bulk resolve actions available to speed through
- Post-import summary shows counts by action (Imported: X, Skipped: X, Updated: X, Sent to merge: X) PLUS downloadable CSV report showing which rows were skipped/updated and why

### Claude's Discretion
- Exact layout/styling of the side-by-side comparison table
- How the "Check Duplicates" loading/progress is shown
- Error handling if detection fails mid-analysis
- How merge wizard modal integrates (modal vs new tab)
- Confidence tier color coding (can reuse Phase 12 red/yellow/blue tiers)

</decisions>

<specifics>
## Specific Ideas

- Detection step is separate from analyze — two distinct API calls, two distinct UI steps
- The "all clear" message when no duplicates are found is important — user should feel the system checked, not that it skipped
- Downloadable CSV report after import is an audit trail — shows exactly what happened to each flagged row
- Bulk resolve is the escape valve for strict blocking — prevents user frustration on large imports with many duplicates

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 14-import-integration*
*Context gathered: 2026-02-07*
