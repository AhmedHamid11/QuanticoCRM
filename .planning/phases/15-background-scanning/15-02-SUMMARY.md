---
phase: 15-background-scanning
plan: 02
subsystem: data-quality
tags: [background-jobs, gocron, scheduling, chunked-processing, checkpoints, goroutines]
dependencies:
  requires:
    - phase: 15-01
      provides: [scan-schema, scan-entities, scan-repositories]
    - phase: 11-03
      provides: [dedup-detector, blocking-strategies, scoring]
    - phase: 12-01
      provides: [pending-alert-entity, pending-alert-repo]
  provides:
    - scan-job-service (chunked execution with checkpoint recovery)
    - scan-scheduler (gocron v2 wrapper with calendar-aware scheduling)
    - progress-event-type (for SSE broadcasting)
  affects: [15-03-handler-api, 15-04-frontend-ui]
tech-stack:
  added: [gocron/v2@v2.19.1]
  patterns: [chunked-processing-with-checkpoints, goroutine-panic-recovery, per-tenant-rate-limiting, gocron-singleton-mode]
key-files:
  created:
    - backend/internal/service/scan_job.go
    - backend/internal/service/scan_scheduler.go
  modified:
    - backend/go.mod
    - backend/go.sum
decisions:
  - slug: service-receives-tenantdb
    summary: "ScanJobService receives tenantDB from caller, doesn't manage DB connections"
    rationale: "Handler/middleware already has tenant DB from middleware, avoids service needing org credentials"
  - slug: goroutine-async-execution
    summary: "ExecuteScan launches goroutine immediately, returns jobID"
    rationale: "Async pattern allows API to return immediately while scan runs in background"
  - slug: 100ms-sleep-between-chunks
    summary: "Sleep 100ms between chunks to allow WAL checkpoint window"
    rationale: "Per RESEARCH.md Pitfall #4: prevents WAL checkpoint starvation on long-running scans"
  - slug: register-org-db-pattern
    summary: "Scheduler uses RegisterOrgDB to store tenant DB for scheduled execution"
    rationale: "Scheduled jobs run outside HTTP context, need cached DB connection"
metrics:
  duration: 4 min
  completed: 2026-02-08
---

# Phase 15 Plan 02: Service & Scheduler Summary

Chunked scan execution with 500-record batches, checkpoint recovery, per-tenant rate limiting, and gocron v2 calendar-aware scheduling with singleton mode.

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-08T13:28:21Z
- **Completed:** 2026-02-08T13:32:47Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 dependency files)

## Accomplishments

- ScanJobService processes records in 500-record chunks with checkpoint after each chunk
- Auto-retry failed chunk once, then mark job failed with partial results preserved
- Per-tenant rate limiting enforces max 2 concurrent jobs per org
- ScanScheduler wraps gocron v2 with DailyJob/WeeklyJob/MonthlyJob calendar awareness
- Singleton mode prevents overlapping scheduled runs per CONTEXT.md requirement
- Manual "Run Now" triggers immediate scan with running-job check
- Graceful shutdown via context cancellation propagates through chunk loop

## Task Commits

Each task was committed atomically:

1. **Task 1: ScanJobService with chunked execution, checkpoint, and retry** - `88e7beb` (feat)
2. **Task 2: ScanScheduler with gocron v2 and schedule management** - `6bb04a7` (feat)

## Files Created/Modified

**Created:**
- `backend/internal/service/scan_job.go` (539 lines) - Chunked scan execution with checkpoint recovery, per-tenant rate limiting, progress event broadcasting, retry logic
- `backend/internal/service/scan_scheduler.go` (336 lines) - gocron v2 wrapper with daily/weekly/monthly scheduling, singleton mode, hot-reload schedule updates

**Modified:**
- `backend/go.mod` - Added gocron/v2@v2.19.1 dependency
- `backend/go.sum` - Dependency checksums (gocron v2, clockwork, cron/v3)

## What Was Built

### ScanJobService (Chunked Execution Engine)

**Core responsibilities:**
1. Execute background duplicate scans in 500-record chunks (avoids Turso 5-second timeout)
2. Save checkpoint after every chunk for resume-from-failure capability
3. Auto-retry failed chunk once before marking job failed
4. Enforce per-tenant rate limit (max 2 concurrent jobs per org)
5. Emit progress events for SSE consumers
6. Store detected duplicates as PendingDuplicateAlerts for review queue

**Key methods:**
- `ExecuteScan(ctx, tenantDB, orgID, entityType, triggerType, scheduleID)` - Main entry point, launches goroutine
- `executeChunkedScan(...)` - Chunk loop with checkpoint recovery, context cancellation checks, 100ms sleep between chunks
- `fetchChunk(...)` - LIMIT/OFFSET cursor-based pagination with `ORDER BY id`
- `processChunk(...)` - Calls Detector.CheckForDuplicates for each record, stores alerts with canonical pair keys
- `handleChunkFailure(...)` - Auto-retry logic: retry once, then mark failed
- `RetryJob(ctx, tenantDB, orgID, failedJobID)` - Creates new job resuming from failed job's checkpoint
- `CanRunJob(orgID)` - Per-tenant rate limit check (exported for handler use)

**Chunked processing pattern:**
```
for {
    select { case <-ctx.Done(): /* graceful shutdown */ }
    records := fetchChunk(chunkSize=500, offset)
    duplicates := processChunk(records)
    updateJobProgress(offset, duplicates)
    saveCheckpoint(offset)
    emitProgress(event)
    sleep(100ms)  // WAL checkpoint window
}
```

**Per-tenant rate limiting:**
- In-memory map `runningJobs[orgID] -> count`
- Mutex-protected increment/decrement on job start/end
- Rejects new jobs if `runningJobs[orgID] >= 2`

**Progress broadcasting:**
- `ProgressEvent` struct exported for handler SSE consumption
- `SetProgressCallback(fn)` wires handler's broadcast function
- `emitProgress(event)` called after each chunk + final completion/failure

**Panic recovery:**
Per Phase 12 decision, goroutine execution wrapped in defer/recover to prevent detector bugs from crashing API server.

**Checkpoint format:**
- `ScanCheckpoint.LastOffset` - LIMIT/OFFSET cursor position
- `ScanCheckpoint.RetryCount` - Tracks retry attempts (max 1)
- `ScanCheckpoint.ChunkSize` - Configurable (default 500)
- Unique constraint on `job_id` - one checkpoint per job

**Partial results visibility:**
Detected duplicates stored as PendingDuplicateAlerts during execution, so they appear in review queue immediately even if job fails midway.

### ScanScheduler (gocron v2 Wrapper)

**Core responsibilities:**
1. Load all enabled schedules from master DB at startup
2. Register gocron jobs with calendar-aware triggers (daily/weekly/monthly)
3. Enable hot-reload schedule changes without server restart
4. Enforce singleton mode to prevent overlapping runs
5. Provide "Run Now" manual trigger with running-job check

**Key methods:**
- `Start(ctx)` - Loads all enabled schedules, registers jobs, starts gocron
- `Shutdown()` - Gracefully stops scheduler and all scheduled jobs
- `registerSchedule(schedule)` - Creates gocron job with DailyJob/WeeklyJob/MonthlyJob definition
- `executeScheduledScan(schedule)` - Task function that runs when schedule triggers
- `UpdateSchedule(ctx, schedule)` - Hot-reloads schedule (remove old job, register new if enabled)
- `RemoveSchedule(ctx, orgID, entityType)` - Deletes schedule and removes gocron job
- `TriggerManualScan(ctx, tenantDB, orgID, entityType)` - Run Now functionality with duplicate check
- `calculateNextRun(schedule)` - Computes next run time based on frequency

**gocron v2 job definition patterns:**

*Daily:*
```go
gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(hour, minute, 0)))
```

*Weekly:*
```go
gocron.WeeklyJob(1, gocron.NewWeekdays(weekday), gocron.NewAtTimes(...))
```

*Monthly:*
```go
gocron.MonthlyJob(1, gocron.NewDaysOfTheMonth(day), gocron.NewAtTimes(...))
```

**Singleton mode:**
```go
gocron.WithSingletonMode(gocron.LimitModeReschedule)
```
Per CONTEXT.md: "If a scan is already running for an entity when next scheduled run triggers, skip the new run and log that it was skipped"

**Org database credentials:**
- `RegisterOrgDB(orgID, dbURL, authToken, tenantDB)` stores tenant DB for scheduled execution
- Scheduled jobs run outside HTTP request context, need cached connection
- orgDBInfo map: `orgID -> (dbURL, authToken, tenantDB)`

**Safety checks:**
- Before executing scheduled scan, check if job already running for entity
- This is a safety net on top of gocron's singleton mode (defense in depth)

**Hot-reload pattern:**
When schedule updated:
1. Remove existing gocron job via `scheduler.RemoveJob(job.ID())`
2. If enabled, call `registerSchedule(schedule)` to create new job
3. Persist to DB via `scanJobRepo.UpsertSchedule()`

## Architecture Patterns

### Pattern 1: Chunked Processing with Checkpoint Recovery

**Problem:** Long-running scans (10k+ records) hit Turso's 5-second transaction timeout and lose progress on failure.

**Solution:** Process in 500-record chunks, save checkpoint after each chunk, resume from checkpoint on retry.

**Implementation:**
- `fetchChunk(limit=500, offset)` uses LIMIT/OFFSET cursor pagination with `ORDER BY id`
- After each chunk: `saveCheckpoint(offset)` persists progress state
- On job start: check for existing checkpoint, set `offset = checkpoint.LastOffset`
- Retry failed chunk once (increment `retry_count`), then mark job failed

**Benefits:**
- Max 500 records per transaction stays under 5-second timeout
- Resume from checkpoint on server restart/crash
- Partial results preserved (alerts stored during execution)

**Trade-offs:**
- Checkpoint writes add DB overhead (mitigated by INSERT OR REPLACE performance)
- LIMIT/OFFSET can be slow on very large tables (acceptable for 10k-100k records)

### Pattern 2: Per-Tenant Rate Limiting

**Problem:** Large customer runs scan, monopolizes worker pool, small customers' scans queue indefinitely.

**Solution:** In-memory map tracks running jobs per org, rejects new jobs if limit reached.

**Implementation:**
```go
runningJobs map[string]int // orgID -> count
runningMu sync.Mutex

// On job start
runningMu.Lock()
if runningJobs[orgID] >= 2 { error }
runningJobs[orgID]++
runningMu.Unlock()

// On job end (defer)
runningMu.Lock()
runningJobs[orgID]--
runningMu.Unlock()
```

**Benefits:**
- Fair resource allocation across tenants
- Prevents single tenant from starving others
- Simple in-memory solution (no external queue needed)

**Trade-offs:**
- Limit enforced per server instance (not global in multi-server setup)
- Acceptable: server restarts clear map, jobs can be manually retried

### Pattern 3: gocron Singleton Mode

**Problem:** Scheduled scan triggers while previous scan still running (e.g., Monday 3 AM scan takes 20 minutes, but next Monday scan starts anyway).

**Solution:** gocron's `WithSingletonMode(LimitModeReschedule)` skips new run if previous run still executing.

**Implementation:**
```go
gocron.WithSingletonMode(gocron.LimitModeReschedule)
```

**LimitModeReschedule behavior:** If job already running, skip this execution and reschedule for next interval.

**Safety net:** `executeScheduledScan` also checks `GetRunningJobForEntity` before starting (defense in depth).

**Benefits:**
- Prevents duplicate detection and wasted resources
- Automatic skipping with logging per CONTEXT.md requirement
- No manual locking code needed

### Pattern 4: Progress Event Broadcasting

**Problem:** Frontend needs real-time progress updates for live progress bar.

**Solution:** Service emits ProgressEvent after each chunk, handler broadcasts via SSE.

**Implementation:**
```go
type ProgressEvent struct {
    JobID, OrgID, EntityType string
    ProcessedRecords, TotalRecords, DuplicatesFound int
    Status string
}

SetProgressCallback(fn func(ProgressEvent)) // Handler sets callback
emitProgress(event)  // Called after each chunk
```

**Benefits:**
- Loose coupling: service doesn't know about SSE, just emits events
- Handler decides broadcast mechanism (SSE, polling, WebSocket)
- Progress visible immediately (not just on completion)

### Pattern 5: Goroutine Panic Recovery

**Problem:** Detector bug could crash entire API server during background scan.

**Solution:** Wrap goroutine execution in defer/recover, mark job failed on panic.

**Implementation:**
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("PANIC in scan job %s: %v", jobID, r)
            // Mark job failed, store panic message
        }
    }()
    executeChunkedScan(...)
}()
```

**Benefits:**
- Server stays up despite detector bugs
- Panic message stored in job error_message for debugging
- User sees failed job (not silent failure)

## Decisions Made

### 1. Service receives tenantDB from caller, doesn't manage DB connections

**Context:** ScanJobService needs tenant DB to execute scans, but db.Manager.GetTenantDB requires org credentials (dbURL, authToken).

**Decision:** Handler/middleware already has tenant DB from auth middleware, pass it to service methods.

**Rationale:**
- Handler has tenant DB via `c.Locals("tenantDB")` from middleware
- Service doesn't need org credentials or db.Manager dependency
- Simpler service constructor (no db.Manager parameter)
- Scheduler uses `RegisterOrgDB` to cache tenant DB for scheduled execution

**Implementation impact:**
- `ExecuteScan(ctx, tenantDB *sql.DB, ...)` signature
- `RetryJob(ctx, tenantDB *sql.DB, ...)` signature
- Scheduler maintains `orgDBInfo` map for scheduled jobs

**Alternatives considered:**
- Service stores db.Manager, fetches org on each scan: Requires org credentials, adds repo dependency
- Service stores orgID -> tenantDB map: Duplicates db.Manager logic, cache management complexity

### 2. ExecuteScan launches goroutine immediately, returns jobID

**Context:** Background scan takes minutes to complete, API handler needs to return immediately.

**Decision:** Create job record, launch goroutine, return jobID synchronously.

**Rationale:**
- API returns HTTP 200 with jobID instantly (optimistic pattern)
- Client polls for progress or subscribes to SSE stream
- Async execution pattern established in Phase 12 (duplicate detection)

**Implementation impact:**
- Handler returns jobID immediately
- Client uses jobID to fetch progress via GET endpoint or SSE stream
- Job status transitions: pending → running → completed/failed

**Alternatives considered:**
- Synchronous execution: Would block API handler for minutes (unacceptable UX)
- Job queue system: Overkill for single-server deployment, adds complexity

### 3. Sleep 100ms between chunks to allow WAL checkpoint window

**Context:** Long-running scans combined with active users create continuous readers, WAL file grows unbounded.

**Decision:** Add `time.Sleep(100 * time.Millisecond)` between chunks.

**Rationale:**
- SQLite WAL mode requires "reader gaps" to checkpoint (RESEARCH.md Pitfall #4)
- 100ms pause allows checkpoint window without significantly slowing scan
- Prevents WAL file growth to gigabytes on large scans

**Implementation impact:**
- Each chunk has 100ms pause: 10k records / 500 per chunk = 20 chunks × 100ms = 2 seconds overhead
- Acceptable trade-off for database health

**Alternatives considered:**
- PRAGMA wal_checkpoint(TRUNCATE) after job completion: Too late, WAL already large
- Schedule scans during low-traffic hours only: Inflexible, doesn't solve problem for 24/7 apps

### 4. RegisterOrgDB pattern for scheduled execution

**Context:** Scheduled jobs run outside HTTP request context, don't have access to middleware's tenant DB.

**Decision:** Scheduler exposes `RegisterOrgDB(orgID, dbURL, authToken, tenantDB)` method, handler calls it when creating schedule.

**Rationale:**
- Scheduled jobs need tenant DB but run in gocron's background goroutine
- Can't rely on middleware or HTTP context
- Cache tenant DB connection per org for reuse across scheduled runs

**Implementation impact:**
- Handler calls `scheduler.RegisterOrgDB(...)` when schedule created/enabled
- Scheduler stores in `orgDBInfo` map: `orgID -> (dbURL, authToken, tenantDB)`
- `executeScheduledScan` looks up tenant DB from map

**Alternatives considered:**
- Scheduler stores db.Manager, resolves DB on each run: Requires master DB access, org lookup overhead
- Scheduler re-creates connection each time: Connection churn, slower execution

## Deviations from Plan

None - plan executed exactly as written.

## Technical Debt

None. Code follows existing patterns:
- Service-repo pattern matches Phase 11-14 (dedup system)
- Goroutine panic recovery matches Phase 12 decision
- Chunked processing with checkpoints follows RESEARCH.md recommendations
- gocron v2 usage matches official examples from pkg.go.dev

## Next Phase Readiness

**Phase 15-03 (Handler API)** is ready to proceed with:
- ✅ ScanJobService.ExecuteScan for starting scans
- ✅ ScanJobService.RetryJob for retry functionality
- ✅ ScanJobService.CanRunJob for rate limit checks
- ✅ ScanScheduler.TriggerManualScan for Run Now button
- ✅ ScanScheduler.UpdateSchedule for hot-reload schedule changes
- ✅ ProgressEvent type for SSE endpoint broadcasting

**Phase 15-04 (Frontend UI)** dependencies:
- ⏳ Requires Phase 15-03 HTTP endpoints to fetch data
- ✅ ProgressEvent JSON structure defined for SSE consumption
- ✅ Job status constants (pending/running/completed/failed/cancelled)

**Blockers/concerns:** None.

## Self-Check: PASSED

**Files created verification:**
- ✅ backend/internal/service/scan_job.go exists (539 lines)
- ✅ backend/internal/service/scan_scheduler.go exists (336 lines)

**Commits verification:**
- ✅ 88e7beb: feat(15-02): implement ScanJobService with chunked execution and checkpoint recovery
- ✅ 6bb04a7: feat(15-02): implement ScanScheduler with gocron v2

All files and commits exist as documented.

---
*Phase: 15-background-scanning*
*Completed: 2026-02-08*
