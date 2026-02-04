---
phase: 06-critical-fixes
plan: 04
subsystem: security-error-handling
tags: [error-sanitization, security, production-hardening]

dependency_graph:
  requires: [06-03]
  provides: ["Complete error sanitization coverage"]
  affects: []

tech_stack:
  added: []
  patterns: ["util.NewAPIError", "util.ClassifyError", "util.ErrCategoryDatabase"]

key_files:
  created: []
  modified:
    - FastCRM/fastcrm/backend/internal/handler/contact.go
    - FastCRM/fastcrm/backend/internal/handler/account.go
    - FastCRM/fastcrm/backend/internal/handler/schema.go
    - FastCRM/fastcrm/backend/internal/handler/navigation.go
    - FastCRM/fastcrm/backend/internal/handler/admin.go
    - FastCRM/fastcrm/backend/internal/handler/tripwire.go
    - FastCRM/fastcrm/backend/internal/handler/validation.go
    - FastCRM/fastcrm/backend/internal/handler/custom_page.go
    - FastCRM/fastcrm/backend/internal/handler/quote.go
    - FastCRM/fastcrm/backend/internal/handler/import.go
    - FastCRM/fastcrm/backend/cmd/api/main.go

decisions:
  - decision: "Used util.NewAPIErrorWithMessage for PDF generation errors"
    rationale: "Generic 'Failed to render' message is user-safe without exposing internal details"
  - decision: "Used util.ErrCategoryDatabase for platform admin routes"
    rationale: "Organization CRUD operations are database operations, classified appropriately"

metrics:
  duration: 12min
  completed: 2026-02-04
---

# Phase 06 Plan 04: Error Sanitization Gap Closure Summary

**One-liner:** Complete error sanitization coverage across 11 handler files with 60+ replacements using util.NewAPIError pattern

## What Was Done

### Task 1: Sanitize core entity handlers (completed earlier)
**Commit:** `0dd6a5b`

Updated error responses in:
- `contact.go`: 9 occurrences replaced
- `account.go`: 9 occurrences replaced
- `schema.go`: 9 occurrences replaced
- `navigation.go`: 8 occurrences replaced

### Task 2: Sanitize admin and system handlers
**Commit:** `d5786d9`

Updated error responses in:
- `tripwire.go`: 11 occurrences replaced
- `validation.go`: 8 occurrences replaced
- `quote.go`: 10 occurrences replaced (including PDF generation with safe messages)
- `admin.go`, `custom_page.go`, `import.go`: Updated in parallel commits

### Task 3: Sanitize platform admin routes in main.go
**Commit:** `35a900b`

Fixed 4 platform admin routes:
- `POST /platform/orgs` - create organization
- `GET /platform/orgs` - list organizations
- `PATCH /platform/orgs/:id` - update organization
- `DELETE /platform/orgs/:id` - delete organization

All now use `util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ErrCategoryDatabase)`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] pdf_template.go build failure**
- **Found during:** Task 2 (detected during build verification)
- **Issue:** pdf_template.go had util imported but 2 calls still used raw err.Error()
- **Fix:** Updated remaining error handlers in pdf_template.go
- **Files modified:** fastcrm/backend/internal/handler/pdf_template.go
- **Commit:** Part of parallel 06-05 work

## Technical Details

### Pattern Applied
```go
// Before (unsafe)
return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
    "error": err.Error(),
})

// After (safe)
return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ClassifyError(err))
```

### Special Cases
- **PDF generation errors:** Used `util.NewAPIErrorWithMessage` with generic "Failed to render" messages
- **Platform admin routes:** Used `util.ErrCategoryDatabase` explicitly (all are org CRUD operations)
- **Validation errors in import:** Used `util.ErrCategoryValidation` for CSV parsing errors

## Verification Results

```
=== Task 1 files ===
contact.go: 0 raw err.Error()
account.go: 0 raw err.Error()
schema.go: 0 raw err.Error()
navigation.go: 0 raw err.Error()

=== Task 2 files ===
admin.go: 0 raw err.Error()
tripwire.go: 0 raw err.Error()
validation.go: 0 raw err.Error()
custom_page.go: 0 raw err.Error()
quote.go: 0 raw err.Error()
import.go: 0 raw err.Error()

=== Task 3 (main.go) ===
1 (allowed: development-mode detail line)

Backend build: PASS
```

## Commits

| Hash | Message |
|------|---------|
| 0dd6a5b | fix(06-04): sanitize error responses in core entity handlers |
| d5786d9 | fix(06-04): sanitize error responses in admin and system handlers |
| 35a900b | fix(06-04): sanitize platform admin error responses in main.go |

## Next Phase Readiness

**Status:** Ready

All error sanitization is complete. The codebase now consistently uses:
- `util.NewAPIError()` for database/internal errors
- `util.NewAPIErrorWithMessage()` for user-facing validation messages
- `util.ClassifyError()` for automatic error categorization

No blockers for proceeding to Phase 07.
