# Phase 13: Merge Engine - Context

**Gathered:** 2026-02-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Complete merge capability for duplicate records. User selects a survivor record, picks field values via side-by-side comparison, related records transfer automatically, full audit log of who/when/what, and undo within 30 days. Covers 2-record and multi-record (3+) merge flows. Admin UI for the merge wizard and review queue are Phase 16.

</domain>

<decisions>
## Implementation Decisions

### Survivor selection
- System auto-suggests the most complete record as survivor (most filled fields)
- User can override the suggestion via radio button / explicit selection
- Completeness score should be visible so the suggestion is transparent

### Field value resolution
- All fields default to the survivor's values
- User overrides individual fields by clicking the duplicate's value
- If survivor field is empty and duplicate has data, auto-fill from duplicate
- Auto-filled fields are visually highlighted so the user sees what was auto-selected
- User can still override any auto-filled value

### Multi-record merge (3+ records)
- Pair-by-pair merging, not all-at-once
- User merges two records first, then merges the result with the next duplicate
- Simpler UI, avoids complex multi-column comparison

### Related record transfer
- All related records (Tasks, Notes, Activities, lookup references) transfer to survivor automatically
- No cherry-picking — everything transfers
- If both survivor and duplicate have the same related record linked, keep both (no deduplication of related records)

### Merge preview
- Show related record counts per entity type (e.g., "5 Tasks, 3 Notes will transfer")
- Expandable list of actual related records under each count
- Preview displays before user confirms the merge

### Duplicate record fate
- After merge, duplicate record is archived with a reference to the survivor
- Archived records are hidden from normal UI (list views, search)
- Archive state supports the 30-day undo requirement

### Claude's Discretion
- Merge confirmation UI layout and flow
- Exact visual treatment of auto-filled field highlights
- Audit log schema and storage approach
- Undo implementation mechanics (snapshot vs reverse operations)
- Data loss warning specifics and thresholds
- Atomic transaction strategy

</decisions>

<specifics>
## Specific Ideas

- Survivor suggestion based on field completeness makes the common case fast — most merges should need minimal clicks
- Pair-by-pair for 3+ records keeps the mental model simple: always comparing two things
- "Transfer all" for related records avoids decision fatigue — the user already decided these are duplicates

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 13-merge-engine*
*Context gathered: 2026-02-07*
