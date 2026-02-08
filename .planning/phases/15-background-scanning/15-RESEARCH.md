# Phase 15: Background Scanning - Research

**Researched:** 2026-02-08
**Domain:** Background job scheduling, chunked database processing, checkpoint recovery
**Confidence:** HIGH

## Summary

Background scanning requires three core technical capabilities: (1) scheduled job execution with preset intervals (daily/weekly/monthly), (2) chunked cursor-based processing to avoid Turso's 5-second transaction timeout, and (3) checkpoint-based recovery for failed jobs. The standard Go stack for this is **gocron v2** for scheduling, **SQLite cursor pagination** with LIMIT/OFFSET for chunking, and **database-backed checkpoint storage** for recovery state. For real-time progress tracking, **Fiber SSE** (Server-Sent Events) provides unidirectional server-to-client updates without WebSocket complexity. In-app notifications use the **eager insertion pattern** (write one record per user immediately) stored in a `notifications` table, polled by the frontend.

The architecture follows a **service-repo pattern** matching the existing codebase: a `ScanJobService` orchestrates scheduling and execution, a `ScanJobRepo` handles database persistence, and a `ScanJobHandler` exposes HTTP endpoints. Jobs execute in goroutines with context cancellation for graceful shutdown, and per-tenant rate limiting uses a simple in-memory map with sync.Mutex.

**Primary recommendation:** Use gocron v2 with DailyJob/WeeklyJob/MonthlyJob definitions, cursor-based chunking with 500 records per chunk, and checkpoint table with `job_id + last_processed_id` for resume capability.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Scheduling & triggers:**
- Preset interval options: daily, weekly, monthly (no cron expressions)
- Admin picks time of day alongside frequency (e.g., weekly on Mondays at 3 AM)
- Manual "Run Now" button to trigger scans on demand
- Scans configured per entity type (each entity gets its own schedule)
- If a scan is already running for an entity when next scheduled run triggers, skip the new run and log that it was skipped

**Progress & monitoring:**
- Live progress bar that updates in real time as chunks complete, showing records processed count
- Dedicated scan jobs page (Settings > Data Quality > Scan Jobs) showing active, scheduled, and historical runs
- Small header indicator in the app when a scan is actively running
- Historical runs show summary row (date, entity type, duration, records scanned, duplicates found, status)
- Each historical run supports drill-down to see the full duplicate list from that scan

**Results & notifications:**
- In-app notification only (no email) when scan completes
- Simple notification message: "Contact scan complete" with link to results (no duplicate count in notification)
- Always notify, even when zero duplicates found — confirms the scan ran
- Scan results feed into the central duplicate review queue (same queue Phase 16 will build UI for)
- Failure notification: same in-app mechanism, different message: "Contact scan failed at 45% — click to retry"

**Failure & recovery:**
- Auto-retry the failed chunk once; if it fails again, save checkpoint and mark job as failed
- Retry resumes from last checkpoint (does not restart from scratch)
- Partial results from failed scans are visible immediately in the review queue — admin can act on them

### Claude's Discretion

- Chunk size for cursor-based processing
- Checkpoint storage format and granularity
- Progress bar update mechanism (polling interval vs SSE)
- Header indicator design
- Notification persistence and dismissal behavior

### Deferred Ideas (OUT OF SCOPE)

- **Salesforce external ID tracking on merge results** — This is a new integration capability for its own phase

</user_constraints>

## Standard Stack

The established libraries/tools for background job scheduling and chunked processing in Go:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gocron v2 | github.com/go-co-op/gocron/v2 | Job scheduling | Most popular Go scheduler (14k+ stars), v2 API is clean and type-safe, supports daily/weekly/monthly jobs with specific times, singleton mode prevents concurrent runs |
| stdlib context | Go 1.22+ | Cancellation signals | Built-in, universally used for graceful shutdown and timeout handling |
| Turso/SQLite | libsql driver | Persistent storage | Already in use, supports cursor pagination, WAL mode for concurrent reads during writes |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Fiber SSE | gofiber/fiber/v2 | Real-time progress | For live progress bar updates (alternative: polling endpoint) |
| sync.Mutex | stdlib | Per-tenant locking | Prevents duplicate job execution per entity |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gocron v2 | robfig/cron | robfig/cron is lower-level (requires manual job tracking), gocron provides job management + singleton mode out-of-box |
| gocron v2 | time.Ticker | Ticker is manual scheduling (no calendar awareness), can't express "daily at 3 AM" without date math |
| SSE | WebSocket | WebSocket is bidirectional (overkill for server-only updates), more complex connection management |
| SSE | Polling | Polling works but higher latency (2-5 sec delay), more HTTP overhead |

**Installation:**
```bash
cd backend && go get github.com/go-co-op/gocron/v2
```

## Architecture Patterns

### Recommended Project Structure
```
backend/
├── internal/
│   ├── entity/
│   │   └── scan_job.go           # Job, schedule, checkpoint entities
│   ├── repo/
│   │   └── scan_job.go            # DB operations for jobs, schedules, checkpoints
│   ├── service/
│   │   ├── scan_job.go            # Job execution orchestration
│   │   └── scan_scheduler.go     # gocron wrapper, schedule management
│   ├── handler/
│   │   └── scan_job.go            # HTTP API for job management
│   └── migrations/
│       ├── 057_create_scan_schedules.sql
│       ├── 058_create_scan_jobs.sql
│       ├── 059_create_scan_checkpoints.sql
│       └── 060_create_notifications.sql
```

### Pattern 1: Job Scheduling with gocron v2
**What:** Declarative job definitions with calendar-aware scheduling
**When to use:** All scheduled background tasks (not just scans — reusable pattern)
**Example:**
```go
// Source: https://pkg.go.dev/github.com/go-co-op/gocron/v2
package service

import (
    "context"
    "time"
    "github.com/go-co-op/gocron/v2"
)

type ScanScheduler struct {
    scheduler gocron.Scheduler
    scanService *ScanJobService
}

func NewScanScheduler(scanService *ScanJobService) (*ScanScheduler, error) {
    s, err := gocron.NewScheduler()
    if err != nil {
        return nil, err
    }
    return &ScanScheduler{
        scheduler: s,
        scanService: scanService,
    }, nil
}

// ScheduleDailyScan configures a daily scan at specific time
func (s *ScanScheduler) ScheduleDailyScan(ctx context.Context, orgID, entityType string, hour, minute int) error {
    job, err := s.scheduler.NewJob(
        gocron.DailyJob(
            1, // every 1 day
            gocron.NewAtTimes(
                gocron.NewAtTime(uint(hour), uint(minute), 0),
            ),
        ),
        gocron.NewTask(
            func() {
                // Execute scan in background
                s.scanService.ExecuteScan(ctx, orgID, entityType)
            },
        ),
        gocron.WithSingletonMode(gocron.LimitModeReschedule), // Skip if already running
        gocron.WithName(fmt.Sprintf("%s-%s-daily", orgID, entityType)),
    )
    if err != nil {
        return err
    }
    return nil
}

// ScheduleWeeklyScan configures weekly scan on specific weekday
func (s *ScanScheduler) ScheduleWeeklyScan(ctx context.Context, orgID, entityType string, weekday time.Weekday, hour, minute int) error {
    _, err := s.scheduler.NewJob(
        gocron.WeeklyJob(
            1, // every 1 week
            gocron.NewWeekdays(weekday),
            gocron.NewAtTimes(
                gocron.NewAtTime(uint(hour), uint(minute), 0),
            ),
        ),
        gocron.NewTask(
            func() {
                s.scanService.ExecuteScan(ctx, orgID, entityType)
            },
        ),
        gocron.WithSingletonMode(gocron.LimitModeReschedule),
        gocron.WithName(fmt.Sprintf("%s-%s-weekly", orgID, entityType)),
    )
    return err
}

// Start begins all scheduled jobs
func (s *ScanScheduler) Start() {
    s.scheduler.Start()
}

// Shutdown gracefully stops scheduler
func (s *ScanScheduler) Shutdown() error {
    return s.scheduler.Shutdown()
}
```

### Pattern 2: Cursor-Based Chunked Processing
**What:** Process large datasets in fixed-size chunks using LIMIT/OFFSET with checkpointing
**When to use:** Any operation on large tables that could exceed Turso's 5-second transaction timeout
**Example:**
```go
// Source: SQLite pagination best practices + Turso timeout handling
package service

type ScanJobService struct {
    detector *dedup.Detector
    repo     *repo.ScanJobRepo
}

// ExecuteScan runs chunked duplicate detection with checkpoints
func (s *ScanJobService) ExecuteScan(ctx context.Context, db *sql.DB, orgID, entityType string) error {
    jobID := sfid.NewScanJob()
    tableName := util.GetTableName(entityType)

    // Create job record
    job := &entity.ScanJob{
        ID: jobID,
        OrgID: orgID,
        EntityType: entityType,
        Status: "running",
        TotalRecords: 0,
        ProcessedRecords: 0,
        DuplicatesFound: 0,
        StartedAt: time.Now(),
    }
    if err := s.repo.CreateJob(ctx, db, job); err != nil {
        return err
    }

    // Check for existing checkpoint (resume from failure)
    checkpoint, _ := s.repo.GetCheckpoint(ctx, db, jobID)
    offset := 0
    if checkpoint != nil {
        offset = checkpoint.LastOffset
    }

    chunkSize := 500 // Recommended chunk size for Turso (avoids 5-sec timeout)

    for {
        // Check context cancellation (graceful shutdown)
        select {
        case <-ctx.Done():
            s.repo.UpdateJobStatus(ctx, db, jobID, "cancelled")
            return ctx.Err()
        default:
        }

        // Fetch chunk
        query := fmt.Sprintf("SELECT * FROM %s WHERE org_id = ? ORDER BY id LIMIT ? OFFSET ?", tableName)
        rows, err := db.QueryContext(ctx, query, orgID, chunkSize, offset)
        if err != nil {
            s.handleChunkFailure(ctx, db, jobID, offset, err)
            return err
        }

        records, err := util.ScanRowsToMaps(rows)
        rows.Close()
        if err != nil || len(records) == 0 {
            break // No more records
        }

        // Process chunk (duplicate detection)
        duplicatesInChunk := 0
        for _, record := range records {
            matches, err := s.detector.CheckForDuplicates(ctx, db, orgID, entityType, record, record["id"].(string))
            if err != nil {
                continue // Log and continue
            }
            if len(matches) > 0 {
                // Store matches in pending_alerts table (Phase 12 pattern)
                duplicatesInChunk += len(matches)
            }
        }

        // Update progress
        offset += len(records)
        s.repo.UpdateJobProgress(ctx, db, jobID, offset, duplicatesInChunk)

        // Save checkpoint (enables resume on failure)
        s.repo.SaveCheckpoint(ctx, db, jobID, offset)

        // Emit progress event for SSE clients
        s.emitProgress(jobID, offset, duplicatesInChunk)

        if len(records) < chunkSize {
            break // Last chunk
        }
    }

    // Mark complete and create notification
    s.repo.UpdateJobStatus(ctx, db, jobID, "completed")
    s.createNotification(ctx, db, orgID, jobID, entityType)

    return nil
}

func (s *ScanJobService) handleChunkFailure(ctx context.Context, db *sql.DB, jobID string, offset int, err error) {
    // Auto-retry logic: check retry count
    checkpoint, _ := s.repo.GetCheckpoint(ctx, db, jobID)
    retryCount := 0
    if checkpoint != nil {
        retryCount = checkpoint.RetryCount
    }

    if retryCount < 1 {
        // Retry once
        s.repo.IncrementRetryCount(ctx, db, jobID)
        // Will retry on next chunk iteration
    } else {
        // Mark failed after 2nd attempt
        s.repo.UpdateJobStatus(ctx, db, jobID, "failed")
        s.createFailureNotification(ctx, db, jobID, offset, err)
    }
}
```

### Pattern 3: SSE for Real-Time Progress Updates
**What:** Unidirectional server-to-client streaming for progress bars
**When to use:** Live updates without polling overhead (progress bars, live feeds, notifications)
**Example:**
```go
// Source: https://docs.gofiber.io/recipes/sse/
package handler

import (
    "bufio"
    "fmt"
    "github.com/gofiber/fiber/v2"
    "github.com/valyala/fasthttp"
)

type ScanJobHandler struct {
    service *service.ScanJobService
    progressChannels map[string]chan ProgressEvent // jobID -> event channel
    mu sync.RWMutex
}

type ProgressEvent struct {
    JobID string `json:"jobId"`
    ProcessedRecords int `json:"processedRecords"`
    DuplicatesFound int `json:"duplicatesFound"`
    Status string `json:"status"`
}

// StreamProgress sends real-time job progress via SSE
func (h *ScanJobHandler) StreamProgress(c *fiber.Ctx) error {
    jobID := c.Query("jobId")

    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Transfer-Encoding", "chunked")

    c.Status(fiber.StatusOK).RequestCtx().SetBodyStreamWriter(
        fasthttp.StreamWriter(func(w *bufio.Writer) {
            // Subscribe to progress events for this job
            eventChan := h.subscribeToProgress(jobID)
            defer h.unsubscribeFromProgress(jobID)

            for {
                select {
                case event := <-eventChan:
                    // Send SSE event
                    fmt.Fprintf(w, "event: progress\n")
                    fmt.Fprintf(w, "data: {\"jobId\":\"%s\",\"processedRecords\":%d,\"duplicatesFound\":%d,\"status\":\"%s\"}\n\n",
                        event.JobID, event.ProcessedRecords, event.DuplicatesFound, event.Status)

                    if err := w.Flush(); err != nil {
                        return // Client disconnected
                    }

                    // Close stream if job completed/failed
                    if event.Status == "completed" || event.Status == "failed" {
                        return
                    }
                case <-c.Context().Done():
                    return // Client disconnected
                }
            }
        }),
    )
    return nil
}

func (h *ScanJobHandler) subscribeToProgress(jobID string) chan ProgressEvent {
    h.mu.Lock()
    defer h.mu.Unlock()
    ch := make(chan ProgressEvent, 10)
    h.progressChannels[jobID] = ch
    return ch
}

func (h *ScanJobHandler) unsubscribeFromProgress(jobID string) {
    h.mu.Lock()
    defer h.mu.Unlock()
    if ch, ok := h.progressChannels[jobID]; ok {
        close(ch)
        delete(h.progressChannels, jobID)
    }
}

// EmitProgress broadcasts progress event to all SSE listeners
func (h *ScanJobHandler) EmitProgress(event ProgressEvent) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    if ch, ok := h.progressChannels[event.JobID]; ok {
        select {
        case ch <- event:
        default: // Channel full, skip
        }
    }
}
```

### Pattern 4: In-App Notifications (Eager Insertion)
**What:** Write one notification record per user immediately when event occurs
**When to use:** In-app notification systems requiring per-user read/dismiss tracking
**Example:**
```go
// Source: https://blog.algomaster.io/p/design-a-scalable-notification-service
package service

type NotificationService struct {
    repo *repo.NotificationRepo
}

// CreateScanCompleteNotification inserts notification for all org admins
func (s *NotificationService) CreateScanCompleteNotification(ctx context.Context, db *sql.DB, orgID, jobID, entityType string) error {
    // Get all users in org (or just admins, depending on requirements)
    users, err := s.getUsersForOrg(ctx, db, orgID)
    if err != nil {
        return err
    }

    // Eager insertion: one record per user
    for _, userID := range users {
        notification := &entity.Notification{
            ID: sfid.NewNotification(),
            OrgID: orgID,
            UserID: userID,
            Type: "scan_complete",
            Title: fmt.Sprintf("%s scan complete", entityType),
            Message: fmt.Sprintf("%s scan complete", entityType), // No duplicate count per CONTEXT.md
            LinkURL: fmt.Sprintf("/data-quality/scan-jobs/%s", jobID),
            IsRead: false,
            CreatedAt: time.Now(),
        }
        if err := s.repo.CreateNotification(ctx, db, notification); err != nil {
            // Log error but continue for other users
            continue
        }
    }
    return nil
}

// CreateScanFailureNotification sends failure notification
func (s *NotificationService) CreateScanFailureNotification(ctx context.Context, db *sql.DB, orgID, jobID, entityType string, progressPercent int) error {
    users, _ := s.getUsersForOrg(ctx, db, orgID)

    for _, userID := range users {
        notification := &entity.Notification{
            ID: sfid.NewNotification(),
            OrgID: orgID,
            UserID: userID,
            Type: "scan_failed",
            Title: fmt.Sprintf("%s scan failed", entityType),
            Message: fmt.Sprintf("%s scan failed at %d%% — click to retry", entityType, progressPercent),
            LinkURL: fmt.Sprintf("/data-quality/scan-jobs/%s", jobID),
            IsRead: false,
            CreatedAt: time.Now(),
        }
        s.repo.CreateNotification(ctx, db, notification)
    }
    return nil
}
```

### Anti-Patterns to Avoid

- **Using DEFERRED transactions for writes:** SQLite immediately returns SQLITE_BUSY without respecting timeout settings. Use BEGIN IMMEDIATE for all write transactions.
- **Processing all records in single transaction:** Turso has 5-second transaction timeout. Always chunk into smaller transactions with checkpoints.
- **Polling for progress every second:** Causes HTTP overhead and latency. Use SSE for sub-second updates or poll at 3-5 second intervals.
- **Storing checkpoint in memory:** Lost on server restart. Always persist checkpoints to database.
- **Running scheduler without graceful shutdown:** Jobs can be killed mid-chunk, corrupting state. Always use context cancellation and WaitGroup.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Calendar-aware scheduling | Manual time.Ticker with date math | gocron v2 DailyJob/WeeklyJob | Handles DST, leap years, month boundaries, timezone conversions automatically |
| Duplicate job prevention | Manual in-memory flags | gocron SingletonMode | Race conditions between scheduler instances, memory leaks from forgotten flags |
| Progress streaming | Long-polling with JSON | Fiber SSE | Persistent connection reduces overhead, browser EventSource API handles reconnects |
| Graceful shutdown | Manual flag checking | context.WithCancel + signal.NotifyContext | Propagates cancellation through entire call stack, prevents goroutine leaks |
| Job queue with retries | Custom retry loop | Checkpoint table + retry counter | Handles crashes/restarts, prevents infinite retry loops, auditable failure history |

**Key insight:** Job scheduling seems simple until edge cases appear (timezone changes, server restarts mid-job, concurrent executions). Use battle-tested libraries that handle these cases.

## Common Pitfalls

### Pitfall 1: Transaction Timeout on Large Tables
**What goes wrong:** Attempting to scan entire Contact table (10k+ records) in single transaction hits Turso's 5-second timeout, job fails with "transaction timeout" error.
**Why it happens:** Developers assume SQLite transactions can run indefinitely (local SQLite has no timeout, but Turso enforces 5-second limit for connection health).
**How to avoid:**
- Use chunked processing with LIMIT/OFFSET (500 records per chunk recommended)
- Each chunk is a separate transaction
- Save checkpoint after each successful chunk
**Warning signs:**
- Jobs fail consistently at same record count
- Error messages mention "timeout" or "SQLITE_BUSY"
- Jobs succeed on small orgs but fail on large orgs

### Pitfall 2: Concurrent Job Execution (Same Entity)
**What goes wrong:** Scheduled scan triggers while previous scan still running (e.g., Monday 3 AM scan takes 20 minutes, but next Monday scan starts at 3 AM anyway), causing duplicate detection and wasted resources.
**Why it happens:** Default schedulers fire on schedule regardless of whether previous job completed.
**How to avoid:**
- Use gocron's SingletonMode with LimitModeReschedule
- Alternative: Database lock pattern (SELECT ... FOR UPDATE on schedule record)
- Per CONTEXT.md: "Skip the new run and log that it was skipped"
**Warning signs:**
- Multiple jobs with same entity_type in "running" status
- Duplicate pending_alerts appearing for same record pairs
- CPU/memory spikes at scheduled times

### Pitfall 3: Lost Progress on Server Restart
**What goes wrong:** Job running for 15 minutes (processed 5,000 records), server restarts (deploy, crash, OOM), job disappears or restarts from beginning, wasting 15 minutes of work.
**Why it happens:** Job state stored in memory (goroutine variables), not persisted to database.
**How to avoid:**
- Save checkpoint to database after every chunk (not just on failure)
- On service startup, check for jobs with status="running" and resume from checkpoint
- Use context.WithCancel to detect shutdown signals and mark job as "interrupted"
**Warning signs:**
- Jobs restart from 0% after server restart
- Job history shows multiple "started" entries with no "completed"
- Customer complains scan "takes forever" (actually restarting repeatedly)

### Pitfall 4: WAL Checkpoint Starvation
**What goes wrong:** Background scan runs continuously with overlapping readers (frontend users viewing records), WAL file grows to gigabytes, database performance degrades, queries slow down.
**Why it happens:** SQLite WAL mode requires "reader gaps" (times when no processes are reading) to checkpoint. Long-running scans combined with active users create continuous readers.
**How to avoid:**
- Schedule scans during low-traffic hours (e.g., 3 AM, not business hours)
- Add 1-second sleep between chunks to allow checkpoint window
- Monitor WAL file size, alert if exceeds 100 MB
- Set PRAGMA wal_checkpoint(TRUNCATE) after job completion
**Warning signs:**
- Database file has large `-wal` companion file (check `ls -lh *.db-wal`)
- Query performance degrades during scans
- Disk space grows unexpectedly

### Pitfall 5: Per-Tenant Rate Limiting Not Applied
**What goes wrong:** Large customer (100k records) runs scan, monopolizes worker pool, small customers' scans queue indefinitely, violating multi-tenant fairness.
**Why it happens:** Naive global worker pool (e.g., 10 goroutines) doesn't differentiate tenants.
**How to avoid:**
- Per requirement BACKGROUND-05: "max 2 concurrent jobs per tenant"
- Use sync.Map to track running jobs per org: `map[orgID][]jobID`
- Before starting job, check: `if len(runningJobs[orgID]) >= 2 { return error }`
- Alternative: Weighted queue (small orgs get priority)
**Warning signs:**
- One tenant's jobs always running, others always pending
- Customer complaints about scan delays
- Metrics show uneven job distribution across tenants

### Pitfall 6: Notification Spam (Zero Duplicates)
**What goes wrong:** Admin schedules daily scan, every day gets notification "Contact scan complete", even when zero duplicates found, notifications become noise, admin ignores them.
**Why it happens:** Per CONTEXT.md: "Always notify, even when zero duplicates found — confirms the scan ran" is the requirement, but UX impact not considered.
**How to avoid:**
- This is actually INTENDED behavior per CONTEXT.md (confirmation notifications)
- Mitigate by: Allow notification preferences (enable/disable scan notifications)
- Group notifications (single notification for all daily scans, not per entity)
- Provide notification filters in frontend (show only failures, show only duplicates found)
**Warning signs:**
- User feedback: "too many notifications"
- High notification dismiss rate (users dismissing without reading)
- Low notification click rate (users ignoring links)

## Code Examples

Verified patterns from official sources:

### Monthly Job with Multiple Days
```go
// Source: https://pkg.go.dev/github.com/go-co-op/gocron/v2
s, _ := gocron.NewScheduler()
defer func() { _ = s.Shutdown() }()

// Run on 1st and 15th of each month at 9:00 AM
_, _ = s.NewJob(
    gocron.MonthlyJob(
        1, // every 1 month
        gocron.NewDaysOfTheMonth(1, 15),
        gocron.NewAtTimes(
            gocron.NewAtTime(9, 0, 0),
        ),
    ),
    gocron.NewTask(
        func() {
            // Task execution
        },
    ),
)
s.Start()
```

### Graceful Shutdown with Context
```go
// Source: https://federicoleon.com/graceful-shutdown-with-context-in-go/
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "sync"
)

func main() {
    // Create root context with cancellation
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Listen for OS signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    // Start background workers
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        defer wg.Done()
        runBackgroundScan(ctx)
    }()

    // Wait for signal
    <-sigChan
    log.Println("Shutdown signal received, canceling context...")
    cancel()

    // Wait for workers to finish (with timeout)
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        log.Println("Clean shutdown")
    case <-time.After(30 * time.Second):
        log.Println("Shutdown timeout, forcing exit")
    }
}

func runBackgroundScan(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            log.Println("Scan worker received cancellation signal")
            return
        default:
            // Process chunk
            processChunk()
        }
    }
}
```

### SQLite Cursor Pagination (Avoid Timeout)
```go
// Source: SQLite pagination best practices + Turso timeout handling
package repo

func (r *ScanJobRepo) FetchRecordsInChunks(ctx context.Context, db *sql.DB, tableName, orgID string, chunkSize, offset int) ([]map[string]interface{}, error) {
    // Use ORDER BY id for consistent pagination (not created_at, may have duplicates)
    query := fmt.Sprintf("SELECT * FROM %s WHERE org_id = ? ORDER BY id LIMIT ? OFFSET ?", tableName)

    rows, err := db.QueryContext(ctx, query, orgID, chunkSize, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    return util.ScanRowsToMaps(rows)
}
```

### WAL Mode Configuration
```go
// Source: https://sqlite.org/wal.html
package db

func EnableWALMode(db *sql.DB) error {
    // Enable WAL mode for concurrent reads during scans
    _, err := db.Exec("PRAGMA journal_mode=WAL")
    if err != nil {
        return err
    }

    // Set busy timeout to 5000ms (5 seconds) to match Turso
    _, err = db.Exec("PRAGMA busy_timeout=5000")
    if err != nil {
        return err
    }

    return nil
}

// TruncateWAL forces checkpoint after large operations
func TruncateWAL(db *sql.DB) error {
    _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
    return err
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| robfig/cron | gocron v2 | 2024 | v2 API is type-safe, cleaner job management, better error handling |
| Polling for progress | SSE/WebSockets | 2020+ | Real-time updates, lower latency, less server load |
| Global worker pool | Per-tenant rate limiting | 2023+ (multi-tenant SaaS) | Fairness, prevents noisy neighbor problem |
| Single transaction | Chunked with checkpoints | 2025+ (Turso timeout) | Reliability, handles large datasets, resume on failure |

**Deprecated/outdated:**
- **robfig/cron v1/v2**: Unmaintained since 2020, use netresearch/go-cron fork or gocron instead
- **Polling every 500ms for progress**: Causes server load, use SSE (sub-second) or 3-5 sec polling
- **Manual cron expression parsing**: Error-prone (timezone, DST), use gocron's DailyJob/WeeklyJob

## Open Questions

Things that couldn't be fully resolved:

1. **Progress bar update frequency vs server load**
   - What we know: SSE supports sub-second updates, polling typically 3-5 seconds
   - What's unclear: Optimal update frequency for 100+ concurrent scans (balance responsiveness vs CPU/memory)
   - Recommendation: Start with SSE updating after each chunk (every 500 records), measure server load, fall back to polling if >1000 concurrent connections

2. **Checkpoint granularity: per-chunk vs per-N-chunks**
   - What we know: Per-chunk checkpoint = max reliability (lose at most 500 records on crash), but more DB writes
   - What's unclear: Database write overhead of checkpoint on every chunk for 100k record scan (200 checkpoints)
   - Recommendation: Start with per-chunk (500 records), monitor checkpoint table size, consider per-5-chunks (2500 records) if DB write load becomes issue

3. **Notification dismissal: auto-dismiss after X days or persist forever**
   - What we know: Eager insertion pattern writes one record per user, can accumulate over time
   - What's unclear: User preference for notification retention (need UX research)
   - Recommendation: Auto-dismiss after 30 days (matches merge snapshot expiration), add "mark all read" and "clear all" buttons

4. **Header indicator design: global banner vs icon badge**
   - What we know: CONTEXT.md requires "small header indicator when scan actively running"
   - What's unclear: UI/UX pattern (banner, badge, icon, toast)
   - Recommendation: Icon badge with count (e.g., "2 scans running"), click to open job list, auto-hide when all complete

## Sources

### Primary (HIGH confidence)
- gocron v2 API: https://pkg.go.dev/github.com/go-co-op/gocron/v2
- gocron v2 examples: https://github.com/go-co-op/gocron/blob/v2/example_test.go
- Fiber SSE recipe: https://docs.gofiber.io/recipes/sse/
- SQLite WAL documentation: https://sqlite.org/wal.html
- Context cancellation guide: https://federicoleon.com/graceful-shutdown-with-context-in-go/

### Secondary (MEDIUM confidence)
- Notification system design (eager insertion): https://blog.algomaster.io/p/design-a-scalable-notification-service
- Turso concurrent writes: https://turso.tech/blog/beyond-the-single-writer-limitation-with-tursos-concurrent-writes
- Multi-tenant rate limiting: https://www.gravitee.io/blog/rate-limiting-apis-scale-patterns-strategies
- Go background job processing 2026: https://oneuptime.com/blog/post/2026-01-30-go-background-job-processing/view

### Tertiary (LOW confidence)
- Drizzle ORM cursor pagination (SQLite): https://orm.drizzle.team/docs/guides/cursor-based-pagination (different ORM, but pattern applies)
- gocron singleton mode (community examples): https://github.com/go-co-op/gocron (v1 docs, v2 API differs)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - gocron v2 is proven, well-documented, actively maintained (last release 2025)
- Architecture: HIGH - Patterns verified in official examples, match existing FastCRM codebase (service-repo pattern)
- Pitfalls: MEDIUM - Based on Turso timeout documentation + general SQLite WAL knowledge, but specific timeout threshold (5 seconds) needs verification with actual scans

**Research date:** 2026-02-08
**Valid until:** 2026-03-08 (30 days, gocron v2 is stable, no major version changes expected)
