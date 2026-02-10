---
phase: 18-rate-limiting-error-handling
plan: 01
subsystem: api
tags: [rate-limiting, salesforce, api-quota, sliding-window, sqlite]

# Dependency graph
requires:
  - phase: 17-core-integration
    provides: Salesforce sync infrastructure (OAuth, batch delivery, sync jobs)
provides:
  - API usage tracking database schema (api_usage_log table with org/time indexes)
  - RateLimitService with sliding window queries for 24-hour rolling usage
  - Quota status calculations (usage, limit, percentage, isPaused)
  - QuotaExceededError entity type for distinguishing quota errors
  - Foundation for Phase 18 Plan 02 delivery pausing integration
affects: [18-02-delivery-integration, 18-03-frontend-quota-ui]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Sliding window rate limiting with 24-hour rolling queries"
    - "Tenant DB graceful degradation for missing tables (no such table handling)"
    - "80% capacity threshold pattern (80K of 100K Salesforce API calls)"

key-files:
  created:
    - backend/internal/migrations/062_add_api_usage_tracking.sql
    - backend/internal/service/salesforce_ratelimit.go
  modified:
    - backend/internal/entity/salesforce_sync.go

key-decisions:
  - "80% API capacity threshold (80,000 of 100,000 calls/day) before automatic pause"
  - "24-hour sliding window for API usage tracking (not fixed daily reset)"
  - "25-hour cleanup window (1-hour buffer) to avoid boundary issues"
  - "Graceful handling of missing api_usage_log table (return 0 usage) for tenant DBs without migration"
  - "api_calls_made column on sync_jobs for per-job tracking"

patterns-established:
  - "Quota enforcement pattern: GetAPIUsageLast24Hours → CanMakeAPICalls → RecordAPIUsage"
  - "QuotaStatus response type with isPaused flag and percentage calculation"
  - "CleanupOldUsage maintenance pattern for removing stale usage records"

# Metrics
duration: 1.8min
completed: 2026-02-10
---

# Phase 18 Plan 01: API Usage Tracking Foundation Summary

**Sliding window API usage tracking with 24-hour rolling queries, 80% quota threshold enforcement, and graceful tenant DB degradation**

## Performance

- **Duration:** 1.8 min (106s)
- **Started:** 2026-02-10T15:18:37Z
- **Completed:** 2026-02-10T15:20:23Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- API usage tracking database schema with org/timestamp indexes for efficient sliding window queries
- RateLimitService with 6 methods covering full quota lifecycle (check, record, status, cleanup)
- Quota entity types (QuotaExceededError, QuotaStatus, APIUsageLog) for integration with delivery service
- Graceful handling of missing api_usage_log table in tenant DBs (Phase 17 migration propagation pattern)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create API usage tracking migration and entity types** - `6177ab7` (feat)
   - Migration 062 with api_usage_log table and sync_jobs ALTER TABLE
   - QuotaExceededError, QuotaStatus, APIUsageLog entity types
   - SyncStatusPaused constant and quota limit constants

2. **Task 2: Create RateLimitService with sliding window API usage tracking** - `7f4a9c4` (feat)
   - RateLimitService with dbManager and authRepo dependencies
   - GetAPIUsageLast24Hours with 24-hour rolling window query
   - CanMakeAPICalls for threshold validation
   - RecordAPIUsage for usage log insertion and sync job updates
   - GetQuotaStatus for percentage calculations
   - CleanupOldUsage for stale record removal

## Files Created/Modified
- `backend/internal/migrations/062_add_api_usage_tracking.sql` - Creates api_usage_log table with org_id/timestamp composite index, adds api_calls_made column to sync_jobs
- `backend/internal/service/salesforce_ratelimit.go` - RateLimitService with sliding window queries, quota calculations, and graceful "no such table" handling
- `backend/internal/entity/salesforce_sync.go` - Added QuotaExceededError (error interface), QuotaStatus (JSON response), APIUsageLog (DB model), SyncStatusPaused constant, and quota limit constants (100K max, 80K threshold)

## Decisions Made

1. **24-hour sliding window**: Rolling 24-hour lookback (not fixed daily reset at midnight) ensures consistent quota enforcement across time zones and prevents midnight quota exhaustion exploits.

2. **80% pause threshold**: Pausing delivery at 80,000 of 100,000 daily API calls provides safety margin for critical operations and prevents hard quota violations.

3. **25-hour cleanup window**: Deleting records older than 25 hours (not 24) provides 1-hour buffer to avoid race conditions where records at exactly 24h boundary are deleted during active queries.

4. **Graceful tenant DB degradation**: All RateLimitService methods handle missing api_usage_log table gracefully (return 0 usage, skip recording) to avoid crashes on tenant DBs where migration hasn't propagated yet (consistent with Phase 17 patterns).

5. **Per-job API call tracking**: api_calls_made column on sync_jobs enables forensic analysis of which jobs consumed quota during incidents.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - migration and service compiled successfully on first attempt. Followed existing patterns from Phase 17 (salesforce_delivery.go, salesforce_sync.go) for multi-tenant DB access and error handling.

## User Setup Required

None - no external service configuration required. Migration 062 will be propagated to tenant DBs via existing MigrationPropagator service.

## Next Phase Readiness

**Ready for Phase 18 Plan 02 (Delivery Integration):**
- RateLimitService.CanMakeAPICalls() ready for pre-delivery checks
- RateLimitService.RecordAPIUsage() ready for post-delivery tracking
- QuotaExceededError type available for pausing logic
- GetQuotaStatus() ready for frontend quota dashboard endpoint

**No blockers** - foundation complete for quota enforcement integration with SFDeliveryService.

## Self-Check: PASSED

**Files exist:**
- FOUND: backend/internal/migrations/062_add_api_usage_tracking.sql (790 bytes)
- FOUND: backend/internal/service/salesforce_ratelimit.go (6.5 KB)

**Commits exist:**
- FOUND: 6177ab7 (feat(18-01): add API usage tracking migration and entity types)
- FOUND: 7f4a9c4 (feat(18-01): add RateLimitService with sliding window tracking)

**Go compilation:**
- SUCCESS: `go build ./...` completed without errors

---
*Phase: 18-rate-limiting-error-handling*
*Completed: 2026-02-10*
