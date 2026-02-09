---
phase: quick-17
plan: 01
subsystem: deduplication
tags: [go, duplicate-detection, review-queue, display-names]

# Dependency graph
requires:
  - phase: 15-review-queue-ui
    provides: Review Queue frontend interface
  - phase: 14-pending-alerts
    provides: Backend duplicate alert storage
provides:
  - Human-readable record names in Review Queue (replacing UUIDs)
  - RecordName field in DuplicateMatch struct
  - Record name enrichment in real-time duplicate check endpoint
  - Correct match record names in background scan alerts
affects: [review-queue, duplicate-detection, data-quality]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "util.GetRecordDisplayName for consistent name extraction"
    - "Record name enrichment before API response"

key-files:
  created: []
  modified:
    - backend/internal/dedup/detector.go
    - backend/internal/handler/dedup.go
    - backend/internal/service/scan_job.go

key-decisions:
  - "Use util.GetRecordDisplayName for consistent name formatting across contacts/accounts/leads"
  - "Enrich names at API boundary (handler level) rather than detector level for separation of concerns"
  - "Fix scan job bug where source record name was incorrectly used for match records"

patterns-established:
  - "Name enrichment pattern: fetch record via util.FetchRecordAsMap, then util.GetRecordDisplayName"
  - "Optional RecordName field with empty check before enrichment allows future optimizations"

# Metrics
duration: 2min
completed: 2026-02-09
---

# Quick Task 17: Fix Review Queue Record Name Display Summary

**Review Queue now displays human-readable record names (e.g., "John Smith") instead of cryptic UUIDs for both source records and matched duplicates**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-09T22:48:54Z
- **Completed:** 2026-02-09T22:50:54Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added RecordName field to DuplicateMatch struct enabling name display in duplicate detection responses
- Real-time duplicate check endpoint now enriches match records with human-readable names
- Fixed critical bug where background scan jobs stored source record name instead of matched record name
- Review Queue now shows proper contact/account names making duplicate review actionable without clicking into records

## Task Commits

Each task was committed atomically:

1. **Task 1: Add RecordName to DuplicateMatch struct and enrich in CheckDuplicates handler** - `b0d0b30` (feat)
2. **Task 2: Fix scan job storing wrong record name on matches** - `570b892` (fix)

**Plan metadata:** (will be committed separately)

## Files Created/Modified
- `backend/internal/dedup/detector.go` - Added RecordName field to DuplicateMatch struct with omitempty JSON tag
- `backend/internal/handler/dedup.go` - Added record name enrichment logic in CheckDuplicates endpoint before response
- `backend/internal/service/scan_job.go` - Fixed bug where alertMatches used source record name; now correctly fetches matched record and uses its name

## Decisions Made

**Name enrichment location:** Chose to enrich names at the handler level (API boundary) rather than in the detector core. This maintains separation of concerns — detector focuses on matching logic, handlers focus on presentation.

**Use util.GetRecordDisplayName:** Leveraged existing utility for consistent name formatting across entity types (firstName+lastName for contacts/leads, name field for accounts/opportunities).

**Bug fix approach:** Rather than modifying getRecordName helper (which is used for source records elsewhere), added explicit matched record fetching at the bug site to make the correction obvious and maintainable.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - straightforward implementation. The bug was clear (line 352 in scan_job.go), and the fix required importing the db package and using the existing util.FetchRecordAsMap pattern already present in the handler code.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Review Queue is now fully functional for human review of duplicate alerts. Users can meaningfully assess whether records are duplicates without clicking into each one.

Frontend already had fallback logic (`{match.recordName || match.recordId}`) so this backend fix completes the feature with zero frontend changes needed.

---
*Phase: quick-17*
*Completed: 2026-02-09*

## Self-Check: PASSED

**Files verified:**
- FOUND: backend/internal/dedup/detector.go (RecordName field present)
- FOUND: backend/internal/handler/dedup.go (enrichment logic present)
- FOUND: backend/internal/service/scan_job.go (bug fix present, db import added)

**Commits verified:**
- FOUND: b0d0b30 (Task 1: feat(quick-17): add RecordName to DuplicateMatch and enrich in CheckDuplicates)
- FOUND: 570b892 (Task 2: fix(quick-17): correct scan job to use matched record name not source)

**Build verification:**
- `go build ./...` passed with no errors
- `go vet ./...` passed with no warnings
