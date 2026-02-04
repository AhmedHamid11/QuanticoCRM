---
phase: 09-session-management
plan: 02
subsystem: security
tags: [csrf, session-management, fiber-middleware, org-settings, timeout-configuration]

# Dependency graph
requires:
  - phase: 08-security-hardening
    provides: Security headers, CORS, and body limits
  - phase: 07-token-architecture
    provides: HttpOnly cookie-based refresh tokens
provides:
  - CSRF protection middleware using Fiber's double-submit cookie pattern
  - Per-organization session timeout configuration (idle and absolute)
  - Session timeout validation with enforced bounds
affects: [09-03-timeout-enforcement, admin-ui]

# Tech tracking
tech-stack:
  added: [github.com/gofiber/fiber/v2/middleware/csrf]
  patterns: [double-submit cookie CSRF, per-org configurable timeouts]

key-files:
  created:
    - backend/internal/middleware/csrf.go
    - backend/migrations/048_org_session_settings.sql
  modified:
    - backend/internal/entity/org_settings.go
    - backend/internal/repo/org_settings.go
    - backend/internal/handler/org_settings.go
    - backend/cmd/api/main.go

key-decisions:
  - "CSRF protection using Fiber's built-in middleware with X-CSRF-Token header"
  - "API tokens (fcr_ prefix) exempt from CSRF validation"
  - "Session timeout bounds: 15-60 min idle, 8-72h (480-4320 min) absolute"
  - "Default timeouts: 30 min idle, 24h absolute"

patterns-established:
  - "CSRF middleware registered after CORS, before rate limiting"
  - "Safe HTTP methods (GET/HEAD/OPTIONS) automatically skip CSRF"
  - "Org settings validation pattern with bounds checking"

# Metrics
duration: 3.5min
completed: 2026-02-04
---

# Phase 09 Plan 02: CSRF Protection and Session Timeout Configuration Summary

**CSRF protection via double-submit cookies and per-org configurable session timeouts with validation bounds**

## Performance

- **Duration:** 3.5 min
- **Started:** 2026-02-04T09:44:57Z
- **Completed:** 2026-02-04T09:48:25Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- CSRF middleware blocks state-changing requests without valid token
- API token requests (Bearer fcr_) exempt from CSRF protection
- Organizations can configure idle timeout (15-60 min) and absolute timeout (8-72h)
- Timeout validation enforces bounds before saving

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement CSRF middleware** - `937fadc` (feat)
2. **Task 2: Add session timeout settings to org_settings** - `0b6a322` (feat)
3. **Task 3: Add org settings update endpoint** - `bf9f4aa` (feat)

## Files Created/Modified
- `backend/internal/middleware/csrf.go` - CSRF middleware using Fiber's double-submit cookie pattern with X-CSRF-Token header validation
- `backend/migrations/048_org_session_settings.sql` - Add idle_timeout_minutes and absolute_timeout_minutes to org_settings
- `backend/internal/entity/org_settings.go` - Session timeout fields and bounds constants (15-60 min idle, 8-72h absolute)
- `backend/internal/repo/org_settings.go` - Validation function and Update method for timeout settings
- `backend/internal/handler/org_settings.go` - Update endpoint with timeout validation
- `backend/cmd/api/main.go` - CSRF middleware registration after CORS

## Decisions Made
- **CSRF token delivery:** Header-based (X-CSRF-Token) for SPA compatibility
- **CSRF cookie settings:** HttpOnly=false (JS must read to set header), SameSite=Strict, Secure in production
- **API token exemption:** Bearer tokens with fcr_ prefix skip CSRF check (not browser-based)
- **Timeout granularity:** Stored in minutes, allowing 1-minute precision for configuration
- **Validation timing:** Bounds checked before saving, not enforced at database level

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - CSRF middleware integrated cleanly, migration applied successfully, validation pattern followed existing org_settings structure.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for Plan 03 (Timeout Enforcement Middleware):**
- CSRF protection active for state-changing requests
- Org settings table has timeout columns with defaults
- API endpoint available for admin UI to configure timeouts
- Validation ensures only valid timeout values are stored

**Blockers/Concerns:**
- None - session table already has created_at field for absolute timeout (from Phase 07)
- Next plan will add last_activity_at tracking and enforcement middleware

---
*Phase: 09-session-management*
*Completed: 2026-02-04*
