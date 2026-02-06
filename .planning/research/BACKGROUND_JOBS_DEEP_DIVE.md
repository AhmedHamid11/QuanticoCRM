# Background Jobs Deep Dive: Go Patterns for Deduplication Scanning

**Researched:** 2026-02-05
**Confidence:** HIGH (multiple authoritative sources, Context7, and official documentation)
**Focus:** Background job architecture for multi-tenant deduplication scanning in Go with Turso/SQLite

---

## Executive Summary

For Quantico CRM's deduplication scanning feature, we recommend an **in-process worker pool with SQLite-backed job persistence**. This avoids external infrastructure (Redis, RabbitMQ) while providing durability, multi-tenant fairness, and graceful handling of long-running scans.

Key recommendations:
1. Use **goqite** or build a lightweight SQLite job queue (~18k jobs/sec performance)
2. Implement **per-tenant job queues** with weighted round-robin scheduling
3. Chunk scans into **cursor-based batches** (500-1000 records) with checkpoint persistence
4. Use **BEGIN IMMEDIATE** transactions to avoid SQLite lock contention
5. Design jobs as **resumable** with stored cursor positions

---

## Table of Contents

1. [In-Process vs External Queue Decision](#1-in-process-vs-external-queue-decision)
2. [Go Background Job Patterns](#2-go-background-job-patterns)
3. [Multi-Tenant Fairness](#3-multi-tenant-fairness)
4. [Job Persistence in SQLite](#4-job-persistence-in-sqlite)
5. [Long-Running Job Management](#5-long-running-job-management)
6. [Turso/SQLite Specific Considerations](#6-tursosqlite-specific-considerations)
7. [Deduplication-Specific Patterns](#7-deduplication-specific-patterns)
8. [Recommended Architecture](#8-recommended-architecture)
9. [Implementation Patterns](#9-implementation-patterns)
10. [Sources](#10-sources)

---

## 1. In-Process vs External Queue Decision

### Recommendation: In-Process with SQLite Persistence

**Decision Matrix:**

| Criterion | In-Process (SQLite) | External (Redis/River) |
|-----------|---------------------|------------------------|
| Infrastructure complexity | LOW - no additional services | HIGH - Redis/Postgres required |
| Durability | HIGH - SQLite persists to disk | HIGH - Redis persistence |
| Performance | ~12-18k jobs/sec | ~50k+ jobs/sec |
| Multi-node scaling | LIMITED - single node | HIGH - distributed workers |
| Transactional guarantees | HIGH with BEGIN IMMEDIATE | HIGH with River |
| Fits Turso architecture | YES - SQLite-native | NO - requires new database |

**Why In-Process for Quantico:**

1. **Architecture alignment**: Already using Turso/SQLite for tenant databases
2. **Deployment simplicity**: No Redis/RabbitMQ infrastructure on Railway
3. **Performance sufficient**: Dedup scans are I/O-bound, not queue-throughput-bound
4. **Single-node deployment**: Railway runs one instance per service

**When to reconsider:**

- If scaling to multiple API pods requiring shared queue
- If job throughput exceeds ~10k jobs/minute
- If requiring complex workflow orchestration (use Temporal)

---

## 2. Go Background Job Patterns

### 2.1 Worker Pool Pattern

The worker pool pattern is the foundation for background job processing in Go.

**Key Components:**

```go
// Core worker pool structure
type WorkerPool struct {
    workers    int
    jobChannel chan Job
    wg         sync.WaitGroup
    ctx        context.Context
    cancel     context.CancelFunc
}

// Job represents a unit of work
type Job struct {
    ID        string
    Type      string
    Payload   []byte
    OrgID     string    // Multi-tenant identifier
    Priority  int       // For weighted scheduling
    CreatedAt time.Time
}
```

**Library Options:**

| Library | Features | Best For |
|---------|----------|----------|
| [alitto/pond](https://github.com/alitto/pond) | Auto-scaling, type-safe results, panic recovery | General purpose pools |
| [gammazero/workerpool](https://github.com/gammazero/workerpool) | Unbounded queue, no blocking on submit | High-volume fire-and-forget |
| Custom | Full control over fairness, persistence | Multi-tenant SaaS |

**Pond Example (if we wanted a simpler approach):**

```go
import "github.com/alitto/pond"

pool := pond.New(10, 1000) // 10 workers, 1000 queue capacity

// Submit job
pool.Submit(func() {
    runDedupScan(orgID, entityType, cursor)
})

// Graceful shutdown
pool.StopAndWait()
```

### 2.2 Context and Cancellation

Proper cancellation is critical for graceful shutdowns and preventing goroutine leaks.

```go
func (p *WorkerPool) Start() {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go func(workerID int) {
            defer p.wg.Done()
            for {
                select {
                case <-p.ctx.Done():
                    log.Printf("Worker %d shutting down", workerID)
                    return
                case job := <-p.jobChannel:
                    p.processJob(job)
                }
            }
        }(i)
    }
}

func (p *WorkerPool) GracefulShutdown(timeout time.Duration) error {
    p.cancel() // Signal workers to stop

    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        return errors.New("shutdown timed out")
    }
}
```

---

## 3. Multi-Tenant Fairness

### 3.1 The Problem

Without fairness controls, one tenant's large dedup scan (scanning 100,000 contacts) could block other tenants' smaller scans for extended periods.

### 3.2 Fair Queue Strategies

**Option A: Per-Tenant Queues with Round-Robin**

The Inngest approach: each tenant (or entity type within tenant) gets its own queue.

```go
type FairJobScheduler struct {
    tenantQueues map[string]*TenantQueue
    mu           sync.RWMutex
}

type TenantQueue struct {
    OrgID    string
    Jobs     []*Job
    Priority int // Higher = more frequent picks
}

// Weighted round-robin selection
func (s *FairJobScheduler) NextJob() *Job {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Collect all non-empty queues
    var active []*TenantQueue
    for _, q := range s.tenantQueues {
        if len(q.Jobs) > 0 {
            active = append(active, q)
        }
    }

    if len(active) == 0 {
        return nil
    }

    // Weighted random selection
    totalWeight := 0
    for _, q := range active {
        totalWeight += q.Priority
    }

    r := rand.Intn(totalWeight)
    for _, q := range active {
        r -= q.Priority
        if r < 0 {
            job := q.Jobs[0]
            q.Jobs = q.Jobs[1:]
            return job
        }
    }

    return active[0].Jobs[0] // Fallback
}
```

**Option B: Rate Limiting per Tenant**

Limit concurrent jobs per tenant.

```go
type TenantRateLimiter struct {
    limits map[string]*semaphore.Weighted
    mu     sync.RWMutex
    max    int64 // Max concurrent jobs per tenant
}

func (l *TenantRateLimiter) Acquire(orgID string) bool {
    l.mu.Lock()
    sem, exists := l.limits[orgID]
    if !exists {
        sem = semaphore.NewWeighted(l.max)
        l.limits[orgID] = sem
    }
    l.mu.Unlock()

    return sem.TryAcquire(1)
}

func (l *TenantRateLimiter) Release(orgID string) {
    l.mu.RLock()
    if sem, exists := l.limits[orgID]; exists {
        sem.Release(1)
    }
    l.mu.RUnlock()
}
```

### 3.3 Recommendation for Quantico

Use **per-tenant queues with max 2 concurrent jobs per tenant**. This ensures:

- No single tenant monopolizes workers
- Large scans from one tenant don't block others
- Simple to implement and reason about

---

## 4. Job Persistence in SQLite

### 4.1 Job Table Schema

```sql
CREATE TABLE background_jobs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    type TEXT NOT NULL,              -- 'dedup_scan', 'import', etc.
    status TEXT NOT NULL DEFAULT 'pending',  -- pending/processing/completed/failed
    payload TEXT NOT NULL,           -- JSON with job-specific data
    cursor TEXT,                     -- For resumable jobs
    progress INTEGER DEFAULT 0,      -- 0-100 percentage
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    scheduled_at TEXT,               -- For delayed jobs
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (org_id) REFERENCES organizations(id)
);

-- Indexes for efficient polling
CREATE INDEX idx_jobs_status ON background_jobs(status, type, created_at);
CREATE INDEX idx_jobs_org ON background_jobs(org_id, status);
CREATE INDEX idx_jobs_scheduled ON background_jobs(status, scheduled_at)
    WHERE scheduled_at IS NOT NULL;
```

### 4.2 Atomic Job Claiming

The critical challenge: preventing multiple workers from claiming the same job.

**Solution: Use BEGIN IMMEDIATE with single atomic UPDATE**

```go
func (r *JobRepo) ClaimNextJob(ctx context.Context, jobType string, workerID string) (*Job, error) {
    tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // Atomic claim using UPDATE with RETURNING
    // Only SQLite 3.35+ supports RETURNING, so we use a two-step approach

    var job Job
    // Use rowid for atomic selection
    row := tx.QueryRowContext(ctx, `
        UPDATE background_jobs
        SET status = 'processing',
            started_at = datetime('now'),
            updated_at = datetime('now')
        WHERE rowid = (
            SELECT rowid FROM background_jobs
            WHERE status = 'pending'
              AND type = ?
              AND (scheduled_at IS NULL OR scheduled_at <= datetime('now'))
            ORDER BY created_at ASC
            LIMIT 1
        )
        RETURNING id, org_id, type, payload, cursor, retry_count
    `, jobType)

    err = row.Scan(&job.ID, &job.OrgID, &job.Type, &job.Payload, &job.Cursor, &job.RetryCount)
    if err == sql.ErrNoRows {
        return nil, nil // No jobs available
    }
    if err != nil {
        return nil, err
    }

    if err := tx.Commit(); err != nil {
        return nil, err
    }

    return &job, nil
}
```

**For SQLite versions < 3.35 (no RETURNING):**

```go
func (r *JobRepo) ClaimNextJobCompat(ctx context.Context, jobType string) (*Job, error) {
    // Use BEGIN IMMEDIATE to acquire write lock upfront
    _, err := r.db.ExecContext(ctx, "BEGIN IMMEDIATE")
    if err != nil {
        return nil, err
    }
    defer r.db.ExecContext(ctx, "ROLLBACK") // Safety net

    // Select candidate
    var jobID string
    err = r.db.QueryRowContext(ctx, `
        SELECT id FROM background_jobs
        WHERE status = 'pending' AND type = ?
        ORDER BY created_at ASC LIMIT 1
    `, jobType).Scan(&jobID)

    if err == sql.ErrNoRows {
        r.db.ExecContext(ctx, "COMMIT")
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    // Update atomically (we hold the write lock)
    result, err := r.db.ExecContext(ctx, `
        UPDATE background_jobs
        SET status = 'processing', started_at = datetime('now')
        WHERE id = ? AND status = 'pending'
    `, jobID)

    rows, _ := result.RowsAffected()
    if rows == 0 {
        // Another worker got it (shouldn't happen with BEGIN IMMEDIATE)
        r.db.ExecContext(ctx, "ROLLBACK")
        return nil, nil
    }

    r.db.ExecContext(ctx, "COMMIT")

    // Now fetch full job details
    return r.GetJob(ctx, jobID)
}
```

### 4.3 Existing SQLite Job Queue Libraries

**goqite** - Recommended if wanting a library:

```go
import "github.com/maragudk/goqite"

// Create queue backed by SQLite
q := goqite.New(goqite.NewOpts{
    DB:   db,
    Name: "dedup_jobs",
})

// Send a job
q.Send(ctx, goqite.Message{
    Body: []byte(`{"org_id":"xxx","entity":"Contact"}`),
})

// Receive and process
msg, err := q.Receive(ctx)
if msg != nil {
    // Process...
    q.Delete(ctx, msg.ID) // On success
    // OR
    q.Extend(ctx, msg.ID, 30*time.Second) // Need more time
}
```

**Performance:** ~18,500 messages/second single worker, ~12,500 with 16 parallel workers.

---

## 5. Long-Running Job Management

### 5.1 Chunking Strategy

Large dedup scans should be chunked to avoid:
- Long-held database locks
- Memory pressure from loading all records
- Loss of progress on failures

**Cursor-Based Chunking:**

```go
type DedupScanState struct {
    OrgID        string `json:"org_id"`
    EntityType   string `json:"entity_type"`
    LastCursor   string `json:"last_cursor"`   // ID of last processed record
    TotalRecords int    `json:"total_records"`
    Processed    int    `json:"processed"`
    Duplicates   int    `json:"duplicates_found"`
}

func (s *DedupScanner) ProcessChunk(ctx context.Context, state *DedupScanState) error {
    const chunkSize = 500

    // Fetch next chunk using cursor pagination
    records, err := s.repo.GetRecordsAfterCursor(ctx, state.EntityType, state.LastCursor, chunkSize)
    if err != nil {
        return err
    }

    if len(records) == 0 {
        return ErrScanComplete
    }

    // Process this chunk
    for _, record := range records {
        duplicates := s.findDuplicates(ctx, record)
        state.Duplicates += len(duplicates)
        state.Processed++
        state.LastCursor = record.ID
    }

    // Checkpoint progress
    return s.saveCheckpoint(ctx, state)
}
```

### 5.2 Checkpoint Persistence

Store checkpoint after each chunk so jobs can resume on failure.

```go
func (s *DedupScanner) saveCheckpoint(ctx context.Context, state *DedupScanState) error {
    payload, _ := json.Marshal(state)

    _, err := s.db.ExecContext(ctx, `
        UPDATE background_jobs
        SET cursor = ?,
            progress = ?,
            payload = ?,
            updated_at = datetime('now')
        WHERE id = ?
    `, state.LastCursor,
       (state.Processed * 100 / state.TotalRecords),
       payload,
       state.JobID)

    return err
}
```

### 5.3 Timeout and Heartbeat

Prevent orphaned jobs from blocking the queue.

```go
const (
    JobTimeout      = 10 * time.Minute // Max time for a job before considered stale
    HeartbeatPeriod = 30 * time.Second
)

// Worker sends heartbeat during processing
func (w *Worker) heartbeat(ctx context.Context, jobID string) {
    ticker := time.NewTicker(HeartbeatPeriod)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            w.repo.UpdateHeartbeat(ctx, jobID)
        }
    }
}

// Cleanup process reclaims stale jobs
func (r *JobRepo) ReclaimStaleJobs(ctx context.Context) (int64, error) {
    result, err := r.db.ExecContext(ctx, `
        UPDATE background_jobs
        SET status = 'pending',
            retry_count = retry_count + 1,
            error_message = 'Job timed out, retrying'
        WHERE status = 'processing'
          AND updated_at < datetime('now', '-10 minutes')
          AND retry_count < max_retries
    `)
    if err != nil {
        return 0, err
    }
    return result.RowsAffected()
}
```

### 5.4 Retry with Exponential Backoff

```go
type RetryConfig struct {
    MaxRetries     int
    InitialDelay   time.Duration
    MaxDelay       time.Duration
    BackoffFactor  float64
}

func (c *RetryConfig) GetDelay(attempt int) time.Duration {
    delay := float64(c.InitialDelay) * math.Pow(c.BackoffFactor, float64(attempt))
    if delay > float64(c.MaxDelay) {
        delay = float64(c.MaxDelay)
    }
    // Add jitter (0-25%)
    jitter := delay * 0.25 * rand.Float64()
    return time.Duration(delay + jitter)
}

// Schedule retry
func (r *JobRepo) ScheduleRetry(ctx context.Context, jobID string, attempt int, err error) error {
    config := RetryConfig{
        MaxRetries:    3,
        InitialDelay:  5 * time.Second,
        MaxDelay:      5 * time.Minute,
        BackoffFactor: 2.0,
    }

    delay := config.GetDelay(attempt)
    scheduledAt := time.Now().Add(delay)

    _, dbErr := r.db.ExecContext(ctx, `
        UPDATE background_jobs
        SET status = 'pending',
            retry_count = ?,
            error_message = ?,
            scheduled_at = ?,
            updated_at = datetime('now')
        WHERE id = ?
    `, attempt+1, err.Error(), scheduledAt.Format(time.RFC3339), jobID)

    return dbErr
}
```

### 5.5 Dead Letter Queue

After max retries, move to failed status for manual review.

```go
func (r *JobRepo) MarkFailed(ctx context.Context, jobID string, err error) error {
    _, dbErr := r.db.ExecContext(ctx, `
        UPDATE background_jobs
        SET status = 'failed',
            error_message = ?,
            completed_at = datetime('now'),
            updated_at = datetime('now')
        WHERE id = ?
    `, err.Error(), jobID)

    // Optionally log to separate failed_jobs table with full stack trace
    return dbErr
}
```

---

## 6. Turso/SQLite Specific Considerations

### 6.1 Connection Limits

Based on Quantico's existing `turso.go` implementation:

| Setting | Current Value | Notes |
|---------|---------------|-------|
| MaxOpenConns | 1 (TursoDB) / 10 (TenantDB) | libsql HTTP uses single conn |
| MaxIdleConns | 1 / 5 | Matches open conns |
| ConnMaxLifetime | 30s / 5min | Short to force refresh |
| Request Concurrency | 20 (default) | libsql client limit |

**Key constraints for background jobs:**

1. **5-second transaction timeout**: Interactive transactions in Turso timeout after 5 seconds
2. **Single writer**: SQLite serializes writes; long transactions block other writes
3. **HTTP overhead**: Each query is an HTTP request; batch when possible

### 6.2 Transaction Best Practices

```go
// DO: Short transactions with BEGIN IMMEDIATE
func goodPattern(ctx context.Context, db *sql.DB) error {
    _, err := db.ExecContext(ctx, "BEGIN IMMEDIATE")
    if err != nil {
        return err
    }

    // Quick operations only
    _, err = db.ExecContext(ctx, "UPDATE jobs SET status = 'processing' WHERE id = ?", id)
    if err != nil {
        db.ExecContext(ctx, "ROLLBACK")
        return err
    }

    _, err = db.ExecContext(ctx, "COMMIT")
    return err
}

// DON'T: Long transactions that hold locks
func badPattern(ctx context.Context, db *sql.DB) error {
    tx, _ := db.BeginTx(ctx, nil)

    // BAD: Scanning and processing within transaction
    rows, _ := tx.QueryContext(ctx, "SELECT * FROM contacts")
    for rows.Next() {
        // Processing takes time...
        findDuplicates(contact) // This holds the write lock!
    }

    return tx.Commit() // Other writers blocked for entire duration
}
```

### 6.3 WAL Mode Settings

Ensure WAL mode is enabled for better concurrency:

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;  -- 5 second wait on locks
```

### 6.4 Using Embedded Replicas (go-libsql)

For read-heavy operations like duplicate scanning, consider embedded replicas:

```go
import "github.com/tursodatabase/go-libsql"

// Create connector with local replica
connector, err := libsql.NewEmbeddedReplicaConnectorWithAutoSync(
    "/tmp/dedup-cache.db",  // Local replica path
    primaryURL,
    authToken,
    libsql.WithAutoSync(30 * time.Second), // Sync every 30s
)

db := sql.OpenDB(connector)

// Reads hit local replica (fast)
// Writes go to primary
```

**Trade-off:** Duplicate detection reads from replica may be slightly stale (up to 30 seconds), but this is acceptable for batch dedup operations.

---

## 7. Deduplication-Specific Patterns

### 7.1 Duplicate Detection Algorithm

Avoid O(n^2) comparison by using indexing:

```go
type DuplicateFinder struct {
    // Index by normalized fields
    emailIndex   map[string][]string // email -> []recordIDs
    phoneIndex   map[string][]string // phone -> []recordIDs
    nameIndex    map[string][]string // normalized name -> []recordIDs
}

func (f *DuplicateFinder) BuildIndex(records []Record) {
    for _, r := range records {
        if email := normalize(r.Email); email != "" {
            f.emailIndex[email] = append(f.emailIndex[email], r.ID)
        }
        if phone := normalizePhone(r.Phone); phone != "" {
            f.phoneIndex[phone] = append(f.phoneIndex[phone], r.ID)
        }
        // etc.
    }
}

func (f *DuplicateFinder) FindDuplicates(record Record) []string {
    candidates := make(map[string]int) // recordID -> match count

    // Check each index
    if matches, ok := f.emailIndex[normalize(record.Email)]; ok {
        for _, id := range matches {
            if id != record.ID {
                candidates[id]++
            }
        }
    }
    // Similar for phone, name...

    // Return records with 2+ field matches
    var duplicates []string
    for id, count := range candidates {
        if count >= 2 {
            duplicates = append(duplicates, id)
        }
    }
    return duplicates
}
```

**Complexity:** O(n) to build index, O(1) average for lookups.

### 7.2 Streaming Approach for Large Datasets

For very large datasets (100k+ records), stream and process in chunks:

```go
func StreamingDedupScan(ctx context.Context, entityType string) error {
    const chunkSize = 1000

    // Phase 1: Build index in chunks
    index := NewDuplicateFinder()

    var cursor string
    for {
        records, nextCursor, err := fetchChunk(ctx, entityType, cursor, chunkSize)
        if err != nil {
            return err
        }
        if len(records) == 0 {
            break
        }

        index.AddToIndex(records) // Incremental index building
        cursor = nextCursor
    }

    // Phase 2: Find duplicates
    cursor = ""
    for {
        records, nextCursor, err := fetchChunk(ctx, entityType, cursor, chunkSize)
        if err != nil {
            return err
        }
        if len(records) == 0 {
            break
        }

        for _, r := range records {
            dupes := index.FindDuplicates(r)
            if len(dupes) > 0 {
                storeDuplicates(ctx, r.ID, dupes)
            }
        }
        cursor = nextCursor
    }

    return nil
}
```

### 7.3 Storing Duplicate Results

```sql
CREATE TABLE duplicate_groups (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    master_record_id TEXT,  -- User-selected "keep" record
    status TEXT DEFAULT 'pending', -- pending/reviewed/merged/ignored
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE duplicate_group_members (
    id TEXT PRIMARY KEY,
    group_id TEXT NOT NULL,
    record_id TEXT NOT NULL,
    match_score REAL,        -- Confidence 0-1
    match_fields TEXT,       -- JSON array of matching field names
    is_master INTEGER DEFAULT 0,
    FOREIGN KEY (group_id) REFERENCES duplicate_groups(id)
);

CREATE INDEX idx_dup_groups_org ON duplicate_groups(org_id, entity_type, status);
CREATE INDEX idx_dup_members_record ON duplicate_group_members(record_id);
```

---

## 8. Recommended Architecture

### 8.1 Component Overview

```
+------------------------+
|     API Server         |
|  (Fiber/Go)            |
+------------------------+
          |
          | Enqueue jobs via HTTP
          v
+------------------------+
|   Job Queue Table      |
|   (SQLite/Turso)       |
+------------------------+
          ^
          | Poll for jobs
          |
+------------------------+
|   Background Worker    |
|   (In-process goroutine|
|    pool)               |
+------------------------+
          |
          | Process per-tenant
          v
+------------------------+
|   Tenant Databases     |
|   (Turso per-org)      |
+------------------------+
```

### 8.2 Service Structure

```go
// internal/jobs/types.go
type JobType string

const (
    JobTypeDedupScan    JobType = "dedup_scan"
    JobTypeDedupMerge   JobType = "dedup_merge"
    JobTypeDataImport   JobType = "data_import"
)

type JobHandler interface {
    Handle(ctx context.Context, job *Job) error
    CanRetry(err error) bool
}

// internal/jobs/worker.go
type BackgroundWorker struct {
    repo          *JobRepo
    handlers      map[JobType]JobHandler
    tenantLimiter *TenantRateLimiter
    workerCount   int
    pollInterval  time.Duration

    wg     sync.WaitGroup
    ctx    context.Context
    cancel context.CancelFunc
}

func (w *BackgroundWorker) Start() {
    w.ctx, w.cancel = context.WithCancel(context.Background())

    for i := 0; i < w.workerCount; i++ {
        w.wg.Add(1)
        go w.workerLoop(i)
    }

    // Cleanup goroutine for stale jobs
    w.wg.Add(1)
    go w.cleanupLoop()
}

func (w *BackgroundWorker) workerLoop(id int) {
    defer w.wg.Done()

    for {
        select {
        case <-w.ctx.Done():
            return
        default:
            job := w.claimNextJob()
            if job == nil {
                time.Sleep(w.pollInterval)
                continue
            }

            w.processJob(job)
        }
    }
}
```

### 8.3 Integration with Fiber

```go
// cmd/api/main.go
func main() {
    app := fiber.New()

    // Initialize job system
    jobRepo := jobs.NewJobRepo(db)
    worker := jobs.NewBackgroundWorker(jobRepo, 4) // 4 workers

    // Register handlers
    worker.RegisterHandler(jobs.JobTypeDedupScan, &DedupScanHandler{})

    // Start background processing
    worker.Start()

    // API endpoint to trigger scans
    app.Post("/api/v1/dedup/scan", func(c *fiber.Ctx) error {
        orgID := c.Locals("orgID").(string)

        job, err := jobRepo.Enqueue(c.Context(), jobs.Job{
            Type:   jobs.JobTypeDedupScan,
            OrgID:  orgID,
            Payload: []byte(`{"entity_type":"Contact"}`),
        })
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        return c.JSON(fiber.Map{
            "job_id": job.ID,
            "status": "pending",
        })
    })

    // Job status endpoint
    app.Get("/api/v1/jobs/:id", func(c *fiber.Ctx) error {
        job, err := jobRepo.GetJob(c.Context(), c.Params("id"))
        // ...
    })

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-quit
        worker.Shutdown(30 * time.Second)
        app.Shutdown()
    }()

    app.Listen(":3000")
}
```

---

## 9. Implementation Patterns

### 9.1 Complete Job Handler Example

```go
// internal/jobs/handlers/dedup_scan.go
type DedupScanHandler struct {
    tenantDB *db.Manager
}

type DedupScanPayload struct {
    EntityType  string `json:"entity_type"`
    LastCursor  string `json:"last_cursor"`
    TotalCount  int    `json:"total_count"`
    Processed   int    `json:"processed"`
    Duplicates  int    `json:"duplicates_found"`
}

func (h *DedupScanHandler) Handle(ctx context.Context, job *Job) error {
    // Parse payload
    var payload DedupScanPayload
    if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }

    // Get tenant database
    tenantDB, err := h.tenantDB.GetDB(job.OrgID)
    if err != nil {
        return fmt.Errorf("failed to get tenant db: %w", err)
    }

    // If first run, get total count
    if payload.TotalCount == 0 {
        count, err := h.countRecords(ctx, tenantDB, payload.EntityType)
        if err != nil {
            return err
        }
        payload.TotalCount = count
    }

    // Process in chunks
    const chunkSize = 500
    for {
        select {
        case <-ctx.Done():
            // Save checkpoint and exit cleanly
            return h.saveCheckpoint(ctx, job, &payload)
        default:
        }

        records, err := h.fetchChunk(ctx, tenantDB, payload.EntityType, payload.LastCursor, chunkSize)
        if err != nil {
            return err
        }

        if len(records) == 0 {
            // Scan complete
            return nil
        }

        // Find duplicates in this chunk
        dupes := h.findDuplicatesInChunk(ctx, tenantDB, records)
        payload.Duplicates += dupes
        payload.Processed += len(records)
        payload.LastCursor = records[len(records)-1].ID

        // Save checkpoint after each chunk
        if err := h.saveCheckpoint(ctx, job, &payload); err != nil {
            return err
        }
    }
}

func (h *DedupScanHandler) CanRetry(err error) bool {
    // Retry on transient errors
    return strings.Contains(err.Error(), "connection") ||
           strings.Contains(err.Error(), "timeout") ||
           strings.Contains(err.Error(), "busy")
}
```

### 9.2 Progress Reporting API

```go
// GET /api/v1/jobs/:id/progress
type JobProgressResponse struct {
    JobID      string `json:"job_id"`
    Status     string `json:"status"`
    Progress   int    `json:"progress"`      // 0-100
    Processed  int    `json:"processed"`
    Total      int    `json:"total"`
    Duplicates int    `json:"duplicates_found,omitempty"`
    Error      string `json:"error,omitempty"`
    StartedAt  string `json:"started_at,omitempty"`
    Duration   string `json:"duration,omitempty"`
}

func (h *JobHandler) GetProgress(c *fiber.Ctx) error {
    job, err := h.repo.GetJob(c.Context(), c.Params("id"))
    if err != nil {
        return c.Status(404).JSON(fiber.Map{"error": "job not found"})
    }

    var payload DedupScanPayload
    json.Unmarshal([]byte(job.Payload), &payload)

    resp := JobProgressResponse{
        JobID:      job.ID,
        Status:     job.Status,
        Progress:   job.Progress,
        Processed:  payload.Processed,
        Total:      payload.TotalCount,
        Duplicates: payload.Duplicates,
    }

    if job.StartedAt != nil {
        resp.StartedAt = job.StartedAt.Format(time.RFC3339)
        resp.Duration = time.Since(*job.StartedAt).String()
    }

    if job.ErrorMessage != "" {
        resp.Error = job.ErrorMessage
    }

    return c.JSON(resp)
}
```

### 9.3 Frontend Integration (SSE for Real-Time Updates)

```go
// Server-Sent Events for job progress
app.Get("/api/v1/jobs/:id/stream", func(c *fiber.Ctx) error {
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")

    jobID := c.Params("id")

    c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                job, err := jobRepo.GetJob(context.Background(), jobID)
                if err != nil {
                    return
                }

                data, _ := json.Marshal(job)
                fmt.Fprintf(w, "data: %s\n\n", data)
                w.Flush()

                if job.Status == "completed" || job.Status == "failed" {
                    return
                }
            }
        }
    }))

    return nil
})
```

---

## 10. Sources

### High Confidence (Official Documentation, Context7)

- [Turso Data Consistency](https://docs.turso.tech/reference/data-consistency) - Transaction timeouts and concurrency
- [go-libsql Repository](https://github.com/tursodatabase/go-libsql) - Embedded replica patterns
- [Turso Concurrent Writes](https://turso.tech/blog/beyond-the-single-writer-limitation-with-tursos-concurrent-writes) - Upcoming MVCC support

### Medium Confidence (Authoritative Technical Sources)

- [goqite - SQLite Message Queue](https://github.com/maragudk/goqite) - ~18k msg/sec performance benchmarks
- [pond Worker Pool](https://github.com/alitto/pond) - Auto-scaling goroutine pools
- [workerpool](https://github.com/gammazero/workerpool) - Concurrency-limiting pools
- [Shopify job-iteration](https://github.com/Shopify/job-iteration) - Resumable job pattern (Ruby, but pattern applicable)
- [River Queue](https://riverqueue.com/) - Postgres-based job queue for Go
- [Inngest Fair Queue](https://www.inngest.com/blog/building-the-inngest-queue-pt-i-fairness-multi-tenancy) - Multi-tenant fairness architecture
- [AWS Fairness Patterns](https://aws.amazon.com/builders-library/fairness-in-multi-tenant-systems/) - Rate limiting and quota patterns

### Lower Confidence (Community Sources, Verified Patterns)

- [Jason Gorman SQLite Background Jobs](https://jasongorman.uk/writing/sqlite-background-job-system/) - BEGIN IMMEDIATE pattern
- [Go Worker Pools - GoByExample](https://gobyexample.com/worker-pools) - Basic worker pool pattern
- [Graceful Shutdown Guide](https://medium.com/@karthianandhanit/a-guide-to-graceful-shutdown-in-go-with-goroutines-and-context-1ebe3654cac8) - Context cancellation patterns
- [Batch Processing in Go](https://dzone.com/articles/batch-processing-in-go) - Chunking strategies

---

## Summary: Key Decisions for Quantico

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Queue type | In-process with SQLite | Matches Turso architecture, no external deps |
| Worker count | 4-8 workers | Balance between parallelism and SQLite limits |
| Multi-tenant fairness | Per-tenant rate limit (max 2 concurrent) | Prevents one tenant from blocking others |
| Chunk size | 500-1000 records | Short transactions, resumable on failure |
| Retry strategy | 3 retries with exponential backoff | Handle transient Turso connection issues |
| Progress tracking | Store in job table, expose via API | Enable UI progress bars |
| Job claiming | BEGIN IMMEDIATE + atomic UPDATE | Prevent duplicate processing |
