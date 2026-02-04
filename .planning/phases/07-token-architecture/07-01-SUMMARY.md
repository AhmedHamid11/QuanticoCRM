---
phase: 07-token-architecture
plan: 01
subsystem: auth
tags: [jwt, refresh-token, token-rotation, security, reuse-detection]

# Dependency graph
requires:
  - phase: 06-critical-fixes
    provides: sanitized error handling foundation
provides:
  - Token family tracking for refresh tokens
  - Soft revocation via is_revoked column
  - Token rotation within families
  - Reuse detection with family-wide revocation
affects: [07-02, 07-03, frontend auth state management]

# Tech tracking
tech-stack:
  added: []
  patterns: ["token-family-rotation", "reuse-detection"]

key-files:
  created:
    - fastcrm/migrations/046_add_token_family_columns.sql
    - fastcrm/backend/internal/migrations/046_add_token_family_columns.sql
  modified:
    - fastcrm/backend/internal/entity/session.go
    - fastcrm/backend/internal/repo/auth.go
    - fastcrm/backend/internal/service/auth.go
    - fastcrm/backend/internal/sfid/sfid.go

key-decisions:
  - "Token family uses sfid pattern with 0Tf prefix"
  - "Soft revocation (is_revoked) instead of immediate deletion for reuse detection"
  - "Login/register/org-switch create new families (security context boundary)"
  - "Token refresh maintains same family for rotation tracking"

patterns-established:
  - "Token family pattern: group tokens from same login session"
  - "Reuse detection: check is_revoked before accepting refresh token"
  - "Family revocation: invalidate all tokens on reuse detection"

# Metrics
duration: 3min
completed: 2026-02-04
---

# Phase 07 Plan 01: Token Family Infrastructure Summary

**Token family tracking with rotation and reuse detection for refresh tokens using family_id grouping and soft revocation**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-04T02:06:17Z
- **Completed:** 2026-02-04T02:09:18Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- Sessions table extended with family_id and is_revoked columns
- Token rotation marks old tokens as revoked, creates new with same family
- Reuse detection triggers family-wide revocation (security measure against token theft)
- New sfid prefix 0Tf for token family IDs

## Task Commits

Each task was committed atomically:

1. **Task 1: Create token family migration and update Session entity** - `396bff7` (feat)
2. **Task 2: Add token family repo methods** - `1e5d85c` (feat)
3. **Task 3: Implement token rotation with reuse detection in service** - `f4bc12e` (feat)

## Files Created/Modified

- `fastcrm/migrations/046_add_token_family_columns.sql` - Schema migration for family_id and is_revoked
- `fastcrm/backend/internal/migrations/046_add_token_family_columns.sql` - Embedded migration copy
- `fastcrm/backend/internal/entity/session.go` - Added FamilyID and IsRevoked fields
- `fastcrm/backend/internal/sfid/sfid.go` - Added NewTokenFamily() and 0Tf prefix
- `fastcrm/backend/internal/repo/auth.go` - Added CreateSessionWithFamily, RevokeSession, RevokeTokenFamily
- `fastcrm/backend/internal/service/auth.go` - Token rotation logic with reuse detection

## Decisions Made

1. **Soft revocation over deletion:** Tokens are marked is_revoked=1 instead of being deleted, enabling reuse detection
2. **Family ID via sfid:** Using consistent ID generation pattern with 0Tf prefix
3. **Security context boundaries:** Login/register/org-switch/stop-impersonation start new families
4. **Family-wide revocation:** On reuse detection, all tokens in family are invalidated (protects against stolen tokens)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Token family infrastructure complete and ready for Plan 07-02 (HttpOnly cookie migration)
- Migration applied successfully to local database
- Backend compiles and all patterns verified

---
*Phase: 07-token-architecture*
*Completed: 2026-02-04*
