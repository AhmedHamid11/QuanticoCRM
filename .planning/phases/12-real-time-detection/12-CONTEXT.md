# Phase 12: Real-Time Detection - Context

**Gathered:** 2026-02-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Detect potential duplicates during record creation/edit and surface matches to users. Users can choose to proceed, merge, or investigate. Actual merge execution is Phase 13; background scanning is Phase 15.

</domain>

<decisions>
## Implementation Decisions

### Warning Presentation
- Modal dialog for duplicate warnings — interrupts flow, forces decision
- Show all matching fields highlighted with their values
- Color-coded confidence badges: High=red (95%+), Medium=yellow (85%+), Low=blue (70%+)
- Expandable preview within modal to see more fields without leaving

### Decision Flow
- Actions available: Create anyway, Merge, View existing, Cancel
- Warn vs block mode configurable per entity in matching rules
- Block mode override: user must type "DUPLICATE" to proceed
- API/bulk imports bypass real-time detection — handled by Phase 14 import deduplication
- Quick merge inline when user chooses merge (simple survivor selection in modal)

### Match Display
- Show top 3 matches by confidence
- "X more found" indicator if additional matches exist
- Sorted by confidence score, highest first
- Minimum threshold configurable per matching rule

### Permissions
- Deduplication requires special permission assignment
- Users without permission see warning but merge/queue options hidden
- They can only "Create anyway" (with override) or cancel

### Trigger Timing
- **Optimistic save with async detection** — record saves immediately, check runs async
- If duplicates found, notification appears when viewing record (banner on detail page)
- Applies to new records AND edits when key fields change
- Silent success when no duplicates found

### Claude's Discretion
- Exact modal layout and field arrangement
- "X more found" behavior (expand vs link to queue)
- Loading/checking indicator during async detection
- Banner styling on detail page for pending duplicates

</decisions>

<specifics>
## Specific Ideas

- "Our goal is always to not interrupt a flow but we also do not want to slow people down until there is a reason to" — hence optimistic save with async notification
- Block mode override with "DUPLICATE" text entry creates friction to ensure intentional override

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-real-time-detection*
*Context gathered: 2026-02-06*
