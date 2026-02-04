---
phase: 06-critical-fixes
plan: 05
subsystem: api
tags: [security, error-handling, go, fiber, sanitization]

# Dependency graph
requires:
  - phase: 06-03
    provides: util.NewAPIError pattern and error classification
  - phase: 06-04
    provides: core entity handlers sanitized (metadata, generic_entity)
provides:
  - Complete error sanitization across all handler files
  - Zero raw err.Error() in JSON responses
  - Phase 06 CRIT-03 verification gap closed
affects: [future-handlers, api-security-audit]

# Tech tracking
tech-stack:
  added: []
  patterns: ["util.NewAPIError() for all error responses", "util.ClassifyError() for error categorization", "request_id correlation in all errors"]

key-files:
  created: []
  modified:
    - "fastcrm/backend/internal/handler/task.go"
    - "fastcrm/backend/internal/handler/related_list.go"
    - "fastcrm/backend/internal/handler/bearing.go"
    - "fastcrm/backend/internal/handler/pdf_template.go"
    - "fastcrm/backend/internal/handler/bulk.go"
    - "fastcrm/backend/internal/handler/lookup.go"
    - "fastcrm/backend/internal/handler/org_settings.go"
    - "fastcrm/backend/internal/handler/flow.go"
    - "fastcrm/backend/internal/handler/data_explorer.go"
    - "fastcrm/backend/internal/handler/version.go"
    - "fastcrm/backend/internal/handler/related.go"
    - "fastcrm/backend/internal/handler/auth.go"
    - "fastcrm/backend/internal/handler/api_token.go"
    - "fastcrm/backend/internal/handler/listview.go"
    - "fastcrm/backend/internal/handler/import.go"
    - "fastcrm/backend/internal/handler/custom_page.go"
    - "fastcrm/backend/internal/handler/admin.go"

key-decisions:
  - "Use util.NewAPIErrorWithMessage for permission errors in auth.go where message is safe to expose"
  - "Update custom_page.go (not in original plan) to complete full handler package sanitization"

patterns-established:
  - "All handler error responses use util.NewAPIError(c, status, err, util.ClassifyError(err))"
  - "Validation errors use util.ErrCategoryValidation explicitly"
  - "Permission errors that are safe to display use util.NewAPIErrorWithMessage"

# Metrics
duration: 14min
completed: 2026-02-04
---

# Phase 06 Plan 05: Gap Closure - Error Sanitization Summary

**Complete error sanitization across 17 handler files eliminating all raw err.Error() in JSON responses**

## Performance

- **Duration:** 14 min
- **Started:** 2026-02-04T00:53:48Z
- **Completed:** 2026-02-04T01:07:43Z
- **Tasks:** 3
- **Files modified:** 17

## Accomplishments
- Sanitized 34+ error responses in medium-occurrence handlers (task, related_list, bearing, pdf_template, bulk)
- Sanitized 60+ error responses in low-occurrence handlers (lookup, org_settings, flow, data_explorer, version, related, auth, api_token, listview, import, custom_page, admin)
- Achieved zero raw err.Error() patterns in entire handler package
- Phase 06 CRIT-03 requirement fully satisfied

## Task Commits

Each task was committed atomically:

1. **Task 1: Sanitize medium-occurrence handlers** - `b1f9c50` (fix)
   - task.go: 7 replacements
   - related_list.go: 9 replacements
   - bearing.go: 6 replacements
   - pdf_template.go: 5 replacements
   - bulk.go: 7 replacements

2. **Task 2: Sanitize low-occurrence handlers** - `48898d0` (fix)
   - lookup.go: 6 replacements
   - org_settings.go: 2 replacements
   - flow.go: 2 replacements
   - data_explorer.go: 8 replacements
   - version.go: 12 replacements
   - related.go: 2 replacements
   - auth.go: 1 replacement
   - api_token.go: 1 replacement
   - listview.go: 5 replacements
   - import.go: 8 replacements
   - custom_page.go: 11 replacements (deviation)
   - admin.go: 1 replacement

3. **Task 3: Final verification and cleanup** - No code changes required (verification pass)

## Files Created/Modified
- `fastcrm/backend/internal/handler/task.go` - Task CRUD error sanitization
- `fastcrm/backend/internal/handler/related_list.go` - Related list config error sanitization
- `fastcrm/backend/internal/handler/bearing.go` - Bearing config error sanitization
- `fastcrm/backend/internal/handler/pdf_template.go` - PDF template error sanitization
- `fastcrm/backend/internal/handler/bulk.go` - Bulk operations error sanitization
- `fastcrm/backend/internal/handler/lookup.go` - Lookup autocomplete error sanitization
- `fastcrm/backend/internal/handler/org_settings.go` - Org settings error sanitization
- `fastcrm/backend/internal/handler/flow.go` - Screen flow error sanitization
- `fastcrm/backend/internal/handler/data_explorer.go` - Data explorer query error sanitization
- `fastcrm/backend/internal/handler/version.go` - Version/migration error sanitization
- `fastcrm/backend/internal/handler/related.go` - Legacy related records error sanitization
- `fastcrm/backend/internal/handler/auth.go` - Auth handler fallback error sanitization
- `fastcrm/backend/internal/handler/api_token.go` - API token error sanitization
- `fastcrm/backend/internal/handler/listview.go` - List view error sanitization
- `fastcrm/backend/internal/handler/import.go` - CSV import error sanitization
- `fastcrm/backend/internal/handler/custom_page.go` - Custom page error sanitization
- `fastcrm/backend/internal/handler/admin.go` - Admin rollup validation error sanitization

## Decisions Made
- **auth.go permission errors:** Used util.NewAPIErrorWithMessage because the error message "only owners and admins can X" is safe to expose and helpful for users
- **api_token.go scope errors:** Used util.NewAPIErrorWithMessage because ErrInvalidScope messages are user-facing validation messages

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added custom_page.go sanitization**
- **Found during:** Task 2 (low-occurrence handlers)
- **Issue:** custom_page.go had util import but 11 unsanitized err.Error() patterns
- **Fix:** Replaced all 11 raw err.Error() patterns with util.NewAPIError
- **Files modified:** fastcrm/backend/internal/handler/custom_page.go
- **Verification:** Build passes, grep confirms zero patterns remain
- **Committed in:** 48898d0 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Auto-fix necessary for complete handler package sanitization. No scope creep.

## Issues Encountered
None - plan executed smoothly.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 06 Critical Fixes fully complete
- All 5 CRIT-03 truths now verified:
  1. util.NewAPIError pattern established
  2. Error classification by type (7 categories)
  3. request_id in all error responses
  4. Development mode error details
  5. All handlers use sanitized patterns
- Ready for Phase 07

---
*Phase: 06-critical-fixes*
*Completed: 2026-02-04*
