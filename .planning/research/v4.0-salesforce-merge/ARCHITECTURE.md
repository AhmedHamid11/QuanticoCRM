# Architecture: Salesforce Merge Instruction Integration

**Project:** Quantico CRM v4.0 - Salesforce Merge Instruction Delivery
**Researched:** 2026-02-09
**Overall Confidence:** HIGH

## Executive Summary

This architecture extends Quantico's v3.0 dedup engine to send merge instructions to Salesforce using proven patterns from the existing codebase. The design follows Quantico's handler → service → repo layers with async job processing (same pattern as v3.0 scan jobs), per-tenant rate limiting via database-backed token buckets, and idempotent delivery using HTTP headers.

**Key Architectural Decisions:**

| Decision Area | Choice | Rationale |
|--------------|--------|-----------|
| **Data Flow** | Async job queue (goroutines) | Proven in v3.0 ScanJobService, fast UX, retryable |
| **Database** | 3 new tables: sync_jobs, salesforce_oauth_credentials, sync_rate_limits | Follows v3.0 scan_jobs pattern, per-org isolation |
| **API Design** | REST endpoints for queue, status, OAuth | Consistent with existing /api/v1/ patterns |
| **Delivery** | Fire-and-forget HTTP POST with idempotency keys | Matches spec, prevents duplicates |
| **Rate Limiting** | Database-backed token bucket | Multi-instance safe, persists across restarts |
| **Retry Strategy** | Exponential backoff in Go service layer (max 3 retries) | Full control, testable, proven pattern |
| **Monitoring** | Prometheus metrics + audit logs | Existing patterns from v3.0 |
| **Testing** | Interface mocks + httptest.Server | Standard Go best practices |

---

## 1. Data Flow: How Merge Instructions Get to Salesforce

**Question Answered:** After dedup resolution, merge instructions flow through async job queue → HTTP POST to Salesforce staging object.

### Why Async (Not Direct API Call)

- Salesforce API calls take 1-5 seconds (too slow for synchronous UX)
- Retry logic requires job state tracking
- Pattern proven in v3.0 ScanJobService

### Flow Diagram

```
┌────────────────────────┐
│ Review Queue Frontend  │ User resolves merge
└────────┬───────────────┘
         │ POST /salesforce-sync/queue
         ▼
┌────────────────────────┐
│ SFSyncHandler          │ Creates sync_jobs record
└────────┬───────────────┘ Returns HTTP 202 {jobId}
         │
         ▼
┌────────────────────────┐
│ SFSyncService          │ Async goroutine:
│ (Background)           │ 1. Fetch/refresh OAuth token
│                        │ 2. Check rate limit
│                        │ 3. Build JSON payload
│                        │ 4. HTTP POST to Salesforce
│                        │ 5. Update job status
│                        │ 6. Create audit log
└────────┬───────────────┘
         │ HTTPS (Bearer Token)
         ▼
┌────────────────────────┐
│ Salesforce             │ Receives batch
│ Staging Object         │ → Apex trigger
│                        │ → Merges records
└────────────────────────┘
```

---

## 2. Database Schema: Tracking Merge Instructions

**Question Answered:** 3 new tables (sync_jobs, salesforce_oauth_credentials, sync_rate_limits) + 2 columns on pending_alerts.

### sync_jobs (Job Status Tracking)

```sql
CREATE TABLE IF NOT EXISTS sync_jobs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    merge_group_id TEXT,
    action TEXT NOT NULL,
    survivor_salesforce_id TEXT,
    duplicate_salesforce_ids TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    batch_payload TEXT,
    error_message TEXT,
    idempotency_key TEXT NOT NULL UNIQUE,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Why idempotency_key:** Prevents duplicate delivery if API retried. Use job ID as key.

### salesforce_oauth_credentials (Per-Org OAuth Tokens)

```sql
CREATE TABLE IF NOT EXISTS salesforce_oauth_credentials (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL UNIQUE,
    instance_url TEXT NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Why separate table:** OAuth tokens change frequently (refresh every 2 hours), separating from org_settings reduces table churn.

### sync_rate_limits (Token Bucket State)

```sql
CREATE TABLE IF NOT EXISTS sync_rate_limits (
    org_id TEXT PRIMARY KEY,
    tokens_remaining INTEGER NOT NULL DEFAULT 100,
    last_refill_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Why database-backed:** Multi-instance deployment (Railway auto-scaling) requires shared state.

---

## 3. API Design: REST Endpoints

**Question Answered:** 7 endpoints for queueing, status queries, OAuth, and retries.

### POST /api/v1/salesforce-sync/queue

Queue merge instructions.

**Request:**
```json
{
  "mergeGroupId": "mg001",
  "entityType": "Contact",
  "action": "merge",
  "survivorSalesforceId": "003xx000004TmiQAAS",
  "duplicateSalesforceIds": ["003xx000004TmiRAAS"]
}
```

**Response (HTTP 202):**
```json
{
  "jobId": "sj001",
  "status": "pending",
  "queuedAt": "2026-02-09T12:34:56Z"
}
```

### GET /api/v1/salesforce-sync/jobs/{jobId}

Query job status.

### POST /api/v1/salesforce-sync/jobs/{jobId}/retry

Retry failed job.

### POST /api/v1/salesforce-sync/oauth/connect

Initiate OAuth flow.

### POST /api/v1/salesforce-sync/oauth/callback

Handle OAuth callback.

---

## 4. Event-Driven vs Polling

**Question Answered:** Quantico pushes to Salesforce (not polling).

**Decision:** Fire-and-forget HTTP POST. Salesforce does NOT poll Quantico.

**Rationale:** Spec defines push pattern, simpler setup, Quantico controls timing.

---

## 5. Concurrency & Rate Limiting

**Question Answered:** Database-backed token bucket for multi-org, multi-instance rate limiting.

### Implementation: Token Bucket Algorithm

```go
func (s *SFSyncService) checkRateLimit(ctx context.Context, orgID string) error {
    tx, _ := s.db.BeginTx(ctx, nil)
    defer tx.Rollback()

    // Lock row for update (prevents race conditions)
    var rateLimit entity.SyncRateLimit
    tx.QueryRow(
        "SELECT tokens_remaining, last_refill_at FROM sync_rate_limits WHERE org_id = ? FOR UPDATE",
        orgID,
    ).Scan(&rateLimit.TokensRemaining, &rateLimit.LastRefillAt)

    // Refill tokens based on elapsed time
    elapsed := time.Since(rateLimit.LastRefillAt)
    refillTokens := int(elapsed.Hours() * 100)
    rateLimit.TokensRemaining = min(rateLimit.TokensRemaining + refillTokens, 100)

    if rateLimit.TokensRemaining < 1 {
        return errors.New("rate limit exceeded")
    }

    rateLimit.TokensRemaining--
    tx.Exec("UPDATE sync_rate_limits SET tokens_remaining = ?...", rateLimit.TokensRemaining)
    tx.Commit()
    return nil
}
```

**Why SELECT FOR UPDATE:** Prevents race conditions across multiple Railway instances.

---

## 6. Idempotency: Preventing Duplicate Deliveries

**Question Answered:** Idempotency keys in HTTP headers + database unique constraint.

### Implementation

```go
req.Header.Set("X-Idempotency-Key", job.IdempotencyKey)
```

**Database constraint:**
```sql
CREATE UNIQUE INDEX idx_sync_jobs_idempotency ON sync_jobs(idempotency_key);
```

**Salesforce-side:** Apex trigger checks IdempotencyKey__c field before processing.

---

## 7. Retry Strategy: Exponential Backoff in Go

**Question Answered:** Retry logic in SFSyncService (not HTTP client or Salesforce Flow).

### Implementation

```go
func (s *SFSyncService) deliverBatchWithRetry(ctx context.Context, job *entity.SyncJob) error {
    maxRetries := 3
    baseDelay := 1 * time.Second

    for attempt := 0; attempt <= maxRetries; attempt++ {
        err := s.deliverBatch(ctx, job)
        if err == nil {
            return nil
        }

        if isRetryable(err) && attempt < maxRetries {
            delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
            jitter := time.Duration(rand.Intn(100)) * time.Millisecond
            time.Sleep(delay + jitter)
            job.RetryCount = attempt + 1
            s.syncJobRepo.Update(ctx, job)
            continue
        }

        return err
    }
    return fmt.Errorf("max retries exceeded")
}
```

**Retry schedule:** 1s, 2s, 4s (+ jitter)

---

## 8. Monitoring: Tracking Delivery Health

**Question Answered:** Prometheus metrics + audit logs + database-queryable status.

### Metrics

**Prometheus:**
```go
syncJobsTotal.WithLabelValues(job.Status, job.OrgID).Inc()
syncLatency.WithLabelValues(job.Status).Observe(time.Since(start).Seconds())
rateLimitTokens.WithLabelValues(orgID).Set(float64(tokens))
```

**Audit logs:**
```json
{
  "action": "salesforce_sync_completed",
  "details": {
    "jobId": "sj001",
    "deliveredAt": "2026-02-09T12:35:12Z",
    "latency": 0.45
  }
}
```

---

## 9. Testing Architecture

**Question Answered:** Interface mocks for unit tests, httptest.Server for integration tests.

### Unit Tests with Mocks

```go
type SalesforceClient interface {
    PostMergeInstructions(ctx context.Context, payload []byte, key string) error
}

type mockSalesforceClient struct {
    postError error
}

func TestSFSyncService_Success(t *testing.T) {
    mockClient := &mockSalesforceClient{postError: nil}
    service := NewSFSyncService(mockClient, jobRepo, rateLimitRepo)
    err := service.ExecuteSyncJob(context.Background(), "sj001")
    assert.NoError(t, err)
}
```

### Integration Tests with httptest.Server

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    assert.Equal(t, "Bearer fake_token", r.Header.Get("Authorization"))
    assert.NotEmpty(t, r.Header.Get("X-Idempotency-Key"))
    w.WriteHeader(http.StatusCreated)
}))
```

---

## 10. Deployment: Database Migrations

**Question Answered:** Migration file 061_create_salesforce_sync_tables.sql

```sql
CREATE TABLE IF NOT EXISTS sync_jobs (...);
CREATE TABLE IF NOT EXISTS salesforce_oauth_credentials (...);
CREATE TABLE IF NOT EXISTS sync_rate_limits (...);
ALTER TABLE pending_alerts ADD COLUMN salesforce_synced_at TEXT;
ALTER TABLE pending_alerts ADD COLUMN sync_job_id TEXT;
```

**Execution:** `go run cmd/migrate/main.go` (local), auto-run on Railway deployment.

---

## Architecture Decision Records

### ADR-001: Async Job Queue vs Synchronous API

**Decision:** Async job queue.

**Rationale:** Salesforce calls take 1-5s (too slow for UX), retry logic requires state tracking, pattern proven in v3.0.

### ADR-002: Database-Backed vs In-Memory Rate Limiting

**Decision:** Database-backed token bucket.

**Rationale:** Railway auto-scales instances (in-memory doesn't work multi-instance), state persists across restarts.

### ADR-003: Retry in Service Layer vs HTTP Client

**Decision:** Service layer retry.

**Rationale:** Full control over error classification, can update job status, easier to test.

---

## Sources

**HIGH Confidence:**
- [Salesforce OAuth Flows](https://www.salesforceben.com/salesforce-to-salesforce-integration-using-oauth-2-0-a-comprehensive-guide/)
- [Coordinated Rate Limiting - Salesforce Engineering](https://engineering.salesforce.com/coordinated-rate-limiting-in-microservices-bb8861126beb/)
- [REST API Idempotency](https://blog.bytebytego.com/p/the-art-of-rest-api-design-idempotency)
- [AWS Retry with Backoff](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/retry-backoff.html)
- [Testing External APIs in Go](https://liza.io/testing-external-api-calls-in-go/)

---

*Architecture designed: 2026-02-09*
*Confidence: HIGH (based on official docs, proven Go patterns, existing Quantico architecture)*
