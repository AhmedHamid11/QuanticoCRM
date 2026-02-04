---
phase: 09-session-management
plan: 01
subsystem: authentication
completed: 2026-02-04
duration: 4 minutes
tags: [session, timeout, security, idle-timeout, absolute-timeout]

requires:
  - 07-token-architecture

provides:
  - Session timeout enforcement (idle and absolute)
  - ExtendSession endpoint for explicit session extension
  - Activity tracking with endpoint exclusions

affects:
  - 09-02 (will use session timeout settings from org_settings)
  - 09-03 (frontend session warning will call extend-session)

tech-stack:
  added: []
  patterns:
    - Middleware-based timeout enforcement
    - Fire-and-forget activity updates (non-blocking)
    - Endpoint path exclusions for polling routes

key-files:
  created:
    - backend/internal/migrations/047_session_timeout.sql
    - backend/internal/middleware/session_timeout.go
  modified:
    - backend/internal/entity/session.go
    - backend/internal/repo/auth.go
    - backend/internal/service/auth.go
    - backend/internal/handler/auth.go
    - backend/cmd/api/main.go

decisions:
  - Use default timeouts (30min idle, 1440min/24h absolute) until Plan 02 adds org settings
  - Activity updates are fire-and-forget (goroutine) to not block requests
  - Skip activity tracking for /auth/refresh and /auth/extend-session endpoints
  - Return 401 with SESSION_EXPIRED code (includes type: idle or absolute)
  - Apply session timeout middleware to ALL protected route groups
---

# Phase 09 Plan 01: Session Timeout Enforcement Summary

**One-liner:** Backend session timeout enforcement with idle (30min) and absolute (24h) limits, explicit session extension endpoint, and activity tracking that skips polling routes.

## What Was Built

Implemented backend session timeout enforcement with three core components:

1. **Database Schema:** Added `last_activity_at`, `idle_timeout_minutes` (default 30), and `absolute_timeout_minutes` (default 1440) columns to sessions table
2. **Timeout Middleware:** Enforces both idle timeout (time since last activity) and absolute timeout (time since session creation), rejects expired sessions with 401 + SESSION_EXPIRED code
3. **ExtendSession Endpoint:** POST /auth/extend-session allows users to explicitly extend their session when warned about timeout
4. **Activity Tracking:** Updates `last_activity_at` for user-initiated requests (excludes /auth/refresh, /auth/extend-session to prevent polling from extending sessions)

## Changes from Plan

None - plan executed exactly as written.

## Deviations from Plan

None.

## Decisions Made

1. **Fire-and-forget activity updates:** Activity updates run in goroutines to avoid blocking requests (acceptable trade-off: slight delay updating timestamp vs blocking user)
2. **401 response includes timeout type:** SESSION_EXPIRED response includes `type: idle` or `type: absolute` to help frontend distinguish between timeout types
3. **Middleware applied to all protected groups:** Session timeout middleware was added to 7 route groups (authProtected, authAdmin, authPlatformAdmin, protected, adminProtected, platformAdmin, importProtected)

## Testing Done

- Migration 047 applied successfully to local database
- Backend compiles without errors
- Server starts successfully with middleware chain in correct order
- All session timeout fields present in Session entity

## Known Issues

None.

## Next Steps (Plan 02)

- Add session timeout settings to org_settings table (configurable idle/absolute timeouts per org)
- Update session creation to use org-specific timeout values instead of defaults
- Add bounds enforcement (idle: 15-60 min, absolute: 8-72 hours)

## Files Changed

### Created
- `backend/internal/migrations/047_session_timeout.sql` - Migration adding timeout tracking columns
- `backend/internal/middleware/session_timeout.go` - Timeout enforcement middleware

### Modified
- `backend/internal/entity/session.go` - Added LastActivityAt, IdleTimeoutMinutes, AbsoluteTimeoutMinutes fields
- `backend/internal/repo/auth.go` - Added GetSessionByUserAndOrg, UpdateLastActivity methods
- `backend/internal/service/auth.go` - Added GetSessionWithTimeouts, UpdateSessionActivity, ExtendSession methods
- `backend/internal/handler/auth.go` - Added ExtendSession handler, added time import
- `backend/cmd/api/main.go` - Wired session timeout middleware to all protected route groups

## Verification

**Must-have truths verified:**
✓ Session is rejected after idle timeout exceeds limit (middleware checks idleTime > IdleTimeoutMinutes)
✓ Session is rejected after absolute timeout exceeds limit (middleware checks sessionAge > AbsoluteTimeoutMinutes)
✓ User can extend session by calling extend-session endpoint (ExtendSession handler updates LastActivityAt)
✓ Activity update only happens for user-initiated requests (SkipActivityUpdate excludes /auth/refresh, /auth/extend-session)

**Must-have artifacts verified:**
✓ backend/migrations/047_session_timeout.sql exists and contains last_activity_at column
✓ backend/internal/middleware/session_timeout.go exists and exports NewSessionTimeoutMiddleware, SessionTimeoutConfig
✓ backend/internal/handler/auth.go contains ExtendSession method

**Must-have key links verified:**
✓ backend/internal/middleware/session_timeout.go calls UpdateSessionActivity from auth service
✓ backend/cmd/api/main.go registers SessionTimeoutMiddleware after auth middleware

## Performance Impact

- Minimal: Activity updates are fire-and-forget goroutines (non-blocking)
- One additional database query per request to fetch session with timeout config (cached by repo if needed in future)
- UpdateLastActivity is a simple UPDATE statement (indexed on session.id)

## Security Impact

- **Positive:** Enforces idle timeout to prevent abandoned sessions
- **Positive:** Enforces absolute timeout to limit session lifespan
- **Positive:** Explicit extension prevents accidental session extension from background polling
- **Neutral:** Default timeouts (30min idle, 24h absolute) are reasonable but will become configurable per org in Plan 02

---

**Phase:** 09-session-management
**Plan:** 01
**Completed:** 2026-02-04
**Duration:** 4 minutes
