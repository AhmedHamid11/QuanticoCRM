---
phase: 15-background-scanning
plan: 03
subsystem: api
tags: [go, fiber, sse, notifications, gocron, scheduler]

# Dependency graph
requires:
  - phase: 15-01
    provides: Schema migrations and repositories for scan schedules, jobs, checkpoints, and notifications
  - phase: 15-02
    provides: ScanJobService with chunked execution and ScanScheduler with gocron v2
provides:
  - NotificationService creating in-app notifications for scan completion/failure
  - ScanJobHandler with admin routes (schedule CRUD, job management, SSE) and public routes (notifications)
  - Full backend wiring in main.go with scheduler startup and graceful shutdown
  - Real-time SSE progress streaming for active scan jobs
  - Per-user notifications with unread count for header badge
affects: [frontend-scan-ui, notification-center]

# Tech tracking
tech-stack:
  added: [fasthttp StreamWriter for SSE]
  patterns:
    - NotificationService creates per-user notifications for all org admins
    - SSE broadcasting via progress callback from ScanJobService
    - Per-org subscriber management for SSE channels
    - Scheduler starts after migration propagation, before Fiber app starts
    - Graceful shutdown via defer scanScheduler.Shutdown()

key-files:
  created:
    - backend/internal/service/notification.go
    - backend/internal/handler/scan_job.go
  modified:
    - backend/internal/service/scan_job.go
    - backend/internal/repo/auth.go
    - backend/cmd/api/main.go

key-decisions:
  - "Notification message per CONTEXT.md: '{Entity} scan complete' with NO duplicate count"
  - "Failure notification: '{Entity} scan failed at X% -- click to retry'"
  - "Notifications created for all admin/owner users in org"
  - "Notifications auto-expire after 30 days"
  - "SSE uses Fiber's StreamWriter with fasthttp pattern (per RESEARCH.md)"
  - "30-second keepalive pings for SSE connections to prevent timeout"

patterns-established:
  - "NotificationService.CreateScanComplete/FailureNotification: best-effort creation (log errors, continue)"
  - "ScanJobHandler.broadcastProgress: non-blocking send to SSE subscribers (skip if channel full)"
  - "Admin routes: /scan-jobs/* (schedule and job management)"
  - "Public routes: /notifications/* (all authenticated users)"
  - "SSE subscriber lifecycle: subscribe on connect, unsubscribe on disconnect"

# Metrics
duration: 26min
completed: 2026-02-08
---

# Phase 15 Plan 03: API & Wiring Summary

**NotificationService with simple scan alerts, ScanJobHandler with admin/public routes, SSE progress streaming, and full main.go wiring with scheduler startup**

## Performance

- **Duration:** 26 min
- **Started:** 2026-02-08T13:37:47Z
- **Completed:** 2026-02-08T14:03:47Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- NotificationService creates scan complete/failed notifications for all org admins per CONTEXT.md specs
- ScanJobHandler with 15 endpoints: schedule CRUD, job management, SSE progress, notifications
- Real-time SSE progress streaming with per-org subscriber management
- Full backend wiring: repos, services, scheduler, handler, route registration
- Scheduler starts on boot, loads all enabled schedules, shuts down gracefully

## Task Commits

Each task was committed atomically:

1. **Task 1: NotificationService for scan completion and failure notifications** - `13f917c` (feat)
2. **Task 2: HTTP handler for schedule CRUD, job management, SSE progress, and notifications** - `4e1d455` (feat)
3. **Task 3: Wire services, handlers, scheduler startup, and graceful shutdown in main.go** - `7185b7b` (feat)

## Files Created/Modified
- `backend/internal/service/notification.go` - NotificationService with CreateScanCompleteNotification and CreateScanFailureNotification
- `backend/internal/handler/scan_job.go` - ScanJobHandler with 15 endpoints (admin routes: schedule CRUD, job management, SSE; public routes: notifications)
- `backend/internal/service/scan_job.go` - Added notificationService field and SetNotificationService method; call notification service on completion/failure
- `backend/internal/repo/auth.go` - Added WithDB method for tenant DB switching
- `backend/cmd/api/main.go` - Initialize repos, services, scheduler; start scheduler after migration propagation; register admin and public routes

## Decisions Made

**1. Notification message format per CONTEXT.md locked decisions**
- Scan complete: "{Entity} scan complete" (NO duplicate count)
- Scan failed: "{Entity} scan failed at X% -- click to retry"
- Always notify on completion, even when zero duplicates found
- Rationale: Simplicity and consistency per prior design decisions

**2. Notifications created for all admin/owner users**
- Use AuthRepo.ListUsersByOrg and filter by role
- Best-effort creation (log errors, continue to next user)
- Rationale: Ensures all admins are notified, failure for one user doesn't block others

**3. SSE implementation using Fiber StreamWriter with fasthttp**
- Per RESEARCH.md pattern: `fasthttp.StreamWriter` with bufio
- 30-second keepalive pings to prevent timeout
- Non-blocking send to subscriber channels (skip if full)
- Rationale: Prevents slow clients from blocking the broadcaster

**4. Scheduler startup timing**
- Initialize after migration propagation but before Fiber app starts
- Defer shutdown to ensure graceful stop
- Rationale: Ensures all migrations complete before scheduled jobs can run

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

Backend API complete for background scanning:
- ✅ Schedule CRUD endpoints for frontend admin UI
- ✅ Manual "Run Now" trigger returning job ID
- ✅ SSE progress streaming for real-time updates
- ✅ Notification endpoints for user notification center
- ✅ Scheduler auto-starts on server boot
- ✅ Full backend compiles: `go build ./cmd/api/...`

**Ready for frontend implementation:**
- Admin UI for schedule management (daily/weekly/monthly with time-of-day)
- Scan job list with pagination and entity filter
- SSE progress bar for active scans
- Notification center with unread count badge

**No blockers.**

---
*Phase: 15-background-scanning*
*Completed: 2026-02-08*

## Self-Check: PASSED

All files created and commits verified.
