# Phase 18: Rate Limiting & Error Handling - Research

**Researched:** 2026-02-10
**Domain:** API rate limiting, exponential backoff, sliding window tracking, error classification
**Confidence:** HIGH

## Summary

Phase 18 adds intelligent rate limiting and error handling to the Salesforce integration to prevent API quota exhaustion and handle transient failures gracefully. The core challenges are: tracking API usage over 24-hour rolling windows per org, implementing exponential backoff with jitter for 429 errors, pausing batch delivery at 80% capacity, and allowing manual override for testing/recovery.

The Go ecosystem provides mature libraries for these patterns: `golang.org/x/time/rate` for token bucket rate limiting, `cenkalti/backoff/v4` for exponential backoff with jitter, and sliding window algorithms for 24-hour tracking. Salesforce Enterprise Edition provides 100,000 API calls per 24 hours, returning HTTP 429 with optional Retry-After headers when limits are exceeded.

**Primary recommendation:** Use sliding window algorithm with Redis or in-memory storage for 24-hour API tracking, `cenkalti/backoff/v4` for exponential backoff (5s, 10s, 20s, 40s with jitter), and add `api_calls_made` column to track per-job API consumption.

## Standard Stack

### Core Libraries

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/time/rate | latest | Token bucket rate limiter | Official Go extended library, proven at scale |
| github.com/cenkalti/backoff/v4 | v4 | Exponential backoff with jitter | Industry standard, configurable retry policies |

### Supporting Libraries (Optional)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/sony/gobreaker | v1/v2 | Circuit breaker pattern | If adding failure threshold circuit breaking |
| github.com/prometheus/client_golang | latest | Metrics instrumentation | For monitoring rate limit usage in production |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| cenkalti/backoff | jpillora/backoff or cloudflare/backoff | Less features, simpler API (acceptable for basic use cases) |
| golang.org/x/time/rate | Custom token bucket | Reinventing the wheel, miss edge cases |
| Sliding window (custom) | Fixed time windows | Boundary problem (users can make 2x requests at window boundaries) |

**Installation:**
```bash
go get golang.org/x/time/rate
go get github.com/cenkalti/backoff/v4
```

## Architecture Patterns

### Pattern 1: Sliding Window API Usage Tracking

**What:** Track API calls over a 24-hour rolling window (not midnight-to-midnight fixed window)

**When to use:** For Salesforce's "100K calls per 24 hours" quota enforcement

**Why sliding window:** Fixed windows allow users to make 2x requests by splitting across window boundaries (e.g., 100K at 11:59 PM + 100K at 12:01 AM = 200K in 2 minutes)

**Schema design:**
```sql
-- Add to sync_jobs table
ALTER TABLE sync_jobs ADD COLUMN api_calls_made INTEGER NOT NULL DEFAULT 0;

-- New table for API usage tracking (tenant DB)
CREATE TABLE IF NOT EXISTS api_usage_log (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    api_calls INTEGER NOT NULL DEFAULT 1,
    job_id TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_api_usage_org_time ON api_usage_log(org_id, timestamp);
```

**Implementation:**
```go
// Track API usage with sliding window (24-hour lookback)
func (s *RateLimitService) GetAPIUsageLast24Hours(ctx context.Context, orgID string) (int, error) {
    cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

    query := `
        SELECT COALESCE(SUM(api_calls), 0)
        FROM api_usage_log
        WHERE org_id = ? AND timestamp >= ?
    `

    var total int
    err := db.QueryRowContext(ctx, query, orgID, cutoff).Scan(&total)
    return total, err
}

// Check if org is within quota (80% threshold for safety)
func (s *RateLimitService) CanMakeAPICalls(ctx context.Context, orgID string, callCount int) (bool, error) {
    usage, err := s.GetAPIUsageLast24Hours(ctx, orgID)
    if err != nil {
        return false, err
    }

    const maxCalls = 100000
    const pauseThreshold = 80000 // 80% of quota

    return usage + callCount <= pauseThreshold, nil
}

// Record API usage after each successful batch delivery
func (s *RateLimitService) RecordAPIUsage(ctx context.Context, orgID, jobID string, callCount int) error {
    query := `
        INSERT INTO api_usage_log (id, org_id, timestamp, api_calls, job_id)
        VALUES (?, ?, ?, ?, ?)
    `

    id := sfid.New("apilog")
    timestamp := time.Now().UTC().Format(time.RFC3339)
    _, err := db.ExecContext(ctx, query, id, orgID, timestamp, callCount, jobID)
    return err
}
```

**Cleanup strategy:** Periodically delete records older than 24 hours to prevent unbounded growth:
```go
func (s *RateLimitService) CleanupOldUsage(ctx context.Context) error {
    cutoff := time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339)
    query := `DELETE FROM api_usage_log WHERE timestamp < ?`
    _, err := db.ExecContext(ctx, query, cutoff)
    return err
}
```

### Pattern 2: Exponential Backoff with Jitter for 429 Errors

**What:** Retry failed requests with exponentially increasing delays (5s, 10s, 20s, 40s) plus random jitter

**When to use:** For HTTP 429 (Rate Limit Exceeded) errors from Salesforce

**Why jitter:** Prevents "thundering herd" problem where all clients retry at the same time and overwhelm the API again

**Implementation with cenkalti/backoff:**
```go
import (
    "github.com/cenkalti/backoff/v4"
    "time"
)

// Configure exponential backoff per requirements (5s, 10s, 20s, 40s)
func newExponentialBackoff() *backoff.ExponentialBackOff {
    b := backoff.NewExponentialBackOff()
    b.InitialInterval = 5 * time.Second
    b.Multiplier = 2.0
    b.MaxInterval = 40 * time.Second
    b.MaxElapsedTime = 0 // No max elapsed time (use max retries instead)
    b.RandomizationFactor = 0.5 // Add 50% jitter (5s becomes 2.5s-7.5s)
    return b
}

// Retry operation with exponential backoff (max 5 retries per requirements)
func retryWithBackoff(operation func() error) error {
    b := newExponentialBackoff()
    maxRetries := 5
    attempts := 0

    retryOp := func() error {
        if attempts >= maxRetries {
            return backoff.Permanent(fmt.Errorf("max retries exceeded"))
        }

        attempts++
        err := operation()

        // Classify errors
        if err == nil {
            return nil
        }

        // Permanent errors (don't retry)
        if isPermananentError(err) {
            return backoff.Permanent(err)
        }

        // Transient errors (retry with backoff)
        return err
    }

    return backoff.Retry(retryOp, b)
}
```

**Salesforce Retry-After header support:**
```go
func (s *SFDeliveryService) deliverBatchWithRetry(ctx context.Context, req *http.Request, client *http.Client) (*http.Response, error) {
    b := newExponentialBackoff()
    maxRetries := 5
    attempts := 0

    var lastResp *http.Response

    retryOp := func() error {
        if attempts >= maxRetries {
            return backoff.Permanent(fmt.Errorf("max retries exceeded"))
        }
        attempts++

        resp, err := client.Do(req)
        if err != nil {
            return err // Network error, retry
        }

        lastResp = resp

        switch resp.StatusCode {
        case 200, 201:
            return nil // Success

        case 429:
            // Rate limit - check Retry-After header
            retryAfter := resp.Header.Get("Retry-After")
            if retryAfter != "" {
                // Parse Retry-After (can be seconds or HTTP-date)
                if seconds, err := strconv.Atoi(retryAfter); err == nil {
                    // Override backoff with Salesforce's suggested delay
                    time.Sleep(time.Duration(seconds) * time.Second)
                    return fmt.Errorf("rate limited, retry after %ds", seconds)
                }
            }
            return fmt.Errorf("rate limit exceeded (429)")

        case 400, 401, 403, 404:
            // Permanent errors - don't retry
            body, _ := io.ReadAll(resp.Body)
            resp.Body.Close()
            return backoff.Permanent(fmt.Errorf("permanent error (%d): %s", resp.StatusCode, body))

        case 500, 502, 503, 504:
            // Server errors - retry
            return fmt.Errorf("server error (%d)", resp.StatusCode)

        default:
            return backoff.Permanent(fmt.Errorf("unexpected status: %d", resp.StatusCode))
        }
    }

    err := backoff.Retry(retryOp, b)
    return lastResp, err
}
```

### Pattern 3: Pre-Delivery Quota Check with Pause

**What:** Check API quota before starting batch delivery, pause if org is at 80% capacity

**When to use:** Before calling `executeBatchDelivery` goroutine

**Why 80% threshold:** Safety margin to prevent hitting hard limit (100K calls/day) due to concurrent operations or calculation lag

**Implementation:**
```go
func (s *SFDeliveryService) QueueMergeInstructions(
    ctx context.Context,
    orgID string,
    inputs []MergeInstructionInput,
) (string, error) {
    // Existing checks (connection status, etc.)...

    // NEW: Check API quota before queuing
    usage, err := s.rateLimitService.GetAPIUsageLast24Hours(ctx, orgID)
    if err != nil {
        return "", fmt.Errorf("failed to check API usage: %w", err)
    }

    const pauseThreshold = 80000 // 80% of 100K daily limit
    if usage >= pauseThreshold {
        return "", &QuotaExceededError{
            OrgID:     orgID,
            Usage:     usage,
            Threshold: pauseThreshold,
            Message:   fmt.Sprintf("API quota exceeded: %d/%d calls used (80%% threshold)", usage, pauseThreshold),
        }
    }

    // Estimate API calls for this batch
    estimatedCalls := len(inputs) // Simplified: 1 call per merge instruction
    if usage + estimatedCalls > pauseThreshold {
        return "", &QuotaExceededError{
            OrgID:     orgID,
            Usage:     usage,
            Threshold: pauseThreshold,
            Message:   fmt.Sprintf("Batch would exceed quota: %d current + %d new > %d threshold", usage, estimatedCalls, pauseThreshold),
        }
    }

    // Proceed with normal flow...
}
```

### Pattern 4: Manual Override for Testing/Recovery

**What:** Admin-triggered endpoint that bypasses rate limiting checks

**When to use:** For emergency recovery or testing scenarios

**Implementation:**
```go
// Add forceDelivery flag to manual trigger endpoint
func (h *SalesforceHandler) ManualTrigger(c *fiber.Ctx) error {
    orgID := c.Locals("orgID").(string)

    var input struct {
        Instructions []service.MergeInstructionInput `json:"instructions"`
        Force        bool                            `json:"force"` // NEW: bypass rate limiting
    }
    if err := c.BodyParser(&input); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid request body",
        })
    }

    // Queue with force flag
    jobID, err := h.deliveryService.QueueMergeInstructionsWithOptions(
        c.Context(),
        orgID,
        input.Instructions,
        service.DeliveryOptions{
            Force:       input.Force,
            TriggerType: entity.SyncTriggerManual,
        },
    )

    if err != nil {
        // Check if it's a quota error
        if quotaErr, ok := err.(*service.QuotaExceededError); ok {
            return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
                "error":     quotaErr.Message,
                "usage":     quotaErr.Usage,
                "threshold": quotaErr.Threshold,
                "hint":      "Use {\"force\": true} to override",
            })
        }

        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": fmt.Sprintf("Failed to trigger instructions: %v", err),
        })
    }

    return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
        "jobId":   jobID,
        "status":  "pending",
        "message": "Merge instructions manually triggered for delivery",
    })
}
```

### Recommended Project Structure for Phase 18

```
backend/internal/
├── service/
│   ├── salesforce_delivery.go      # Existing - update with backoff retry
│   ├── salesforce_ratelimit.go     # NEW - API usage tracking service
│   └── salesforce_backoff.go       # NEW - exponential backoff helpers
├── entity/
│   └── salesforce_sync.go          # Existing - add QuotaExceededError
└── migrations/
    └── 062_add_api_usage_tracking.sql  # NEW - api_usage_log table
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exponential backoff | Custom retry loop with sleep() | cenkalti/backoff/v4 | Handles jitter, max retries, permanent errors, context cancellation |
| Token bucket rate limiting | Counter with time checks | golang.org/x/time/rate | Handles burst capacity, concurrency, refill rates correctly |
| Time window calculations | Manual timestamp math | Store timestamps, query with >= cutoff | SQL handles timezone edge cases, daylight saving transitions |
| Circuit breaker | Custom failure counter | sony/gobreaker (optional) | Handles half-open state, success threshold, timeout recovery |

**Key insight:** Rate limiting and retry logic have subtle edge cases (concurrent requests, clock skew, boundary conditions) that mature libraries handle correctly. Custom implementations miss these and fail under load.

## Common Pitfalls

### Pitfall 1: Fixed Time Windows (Midnight-to-Midnight)

**What goes wrong:** Org uses 99K calls at 11:59 PM, then 99K at 12:01 AM = 198K calls in 2 minutes, hitting Salesforce's hard limit

**Why it happens:** Naive implementation resets counter at midnight without considering rolling 24-hour window

**How to avoid:** Use sliding window algorithm - track timestamps and query SUM(api_calls) WHERE timestamp >= NOW() - 24 hours

**Warning signs:** Users report quota errors despite being "under limit yesterday"

### Pitfall 2: Thundering Herd on Retry

**What goes wrong:** 100 concurrent batch deliveries all get 429 error at same time, all retry after exactly 5 seconds, overwhelming API again

**Why it happens:** No jitter - all clients use same backoff delay

**How to avoid:** Add randomization factor to backoff (cenkalti/backoff's `RandomizationFactor = 0.5` spreads retries across 50% window)

**Warning signs:** Repeated 429 errors in waves at predictable intervals

### Pitfall 3: Ignoring Retry-After Header

**What goes wrong:** Salesforce says "retry after 60 seconds" but code uses 5-second backoff, gets rate-limited again immediately

**Why it happens:** Not parsing Retry-After response header from 429 responses

**How to avoid:** Check `resp.Header.Get("Retry-After")` and use it to override backoff delay

**Warning signs:** Rapid successive 429 errors even with backoff enabled

### Pitfall 4: Timezone Confusion in 24-Hour Windows

**What goes wrong:** Org in PST timezone hits quota at "midnight" but it's actually 8 AM UTC, causing unexpected pauses

**Why it happens:** Mixing UTC and local time in calculations

**How to avoid:** ALWAYS use UTC for API timestamps (`time.Now().UTC()`). Store as RFC3339 in database.

**Warning signs:** Quota resets at unexpected times, queries return wrong usage totals

### Pitfall 5: Not Recording API Calls on Partial Success

**What goes wrong:** Batch of 100 instructions delivers 80 successfully before failure, but code only records 0 API calls because job "failed"

**Why it happens:** Only recording usage on full completion

**How to avoid:** Record API usage incrementally or parse Salesforce response to count actually processed records

**Warning signs:** Quota tracking shows 0 usage but Salesforce dashboard shows thousands of API calls

### Pitfall 6: Circuit Breaker Misuse

**What goes wrong:** Circuit breaker trips after 3 failures, blocks ALL orgs' Salesforce access for 60 seconds

**Why it happens:** Using a global circuit breaker instead of per-org circuit breakers

**How to avoid:** If using circuit breakers, create per-org instances or use per-org failure counters

**Warning signs:** One org's rate limit causes all orgs to stop syncing

## Code Examples

Verified patterns from research:

### Example 1: Sliding Window Query (24-Hour Lookback)

```go
// Source: Pattern adapted from RussellLuo/slidingwindow Go library
func (s *RateLimitService) GetAPIUsageLast24Hours(ctx context.Context, orgID string) (int, error) {
    // Calculate cutoff timestamp (24 hours ago)
    cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

    query := `
        SELECT COALESCE(SUM(api_calls), 0)
        FROM api_usage_log
        WHERE org_id = ? AND timestamp >= ?
    `

    var total int
    err := s.tenantDB.QueryRowContext(ctx, query, orgID, cutoff).Scan(&total)
    if err != nil {
        return 0, fmt.Errorf("failed to query API usage: %w", err)
    }

    return total, nil
}
```

### Example 2: Exponential Backoff with Jitter Configuration

```go
// Source: cenkalti/backoff/v4 documentation
import "github.com/cenkalti/backoff/v4"

func newSalesforceBackoff() backoff.BackOff {
    b := backoff.NewExponentialBackOff()
    b.InitialInterval = 5 * time.Second
    b.Multiplier = 2.0 // Double each retry: 5s, 10s, 20s, 40s
    b.MaxInterval = 40 * time.Second
    b.MaxElapsedTime = 0 // Use max retries instead of time limit
    b.RandomizationFactor = 0.5 // Add ±50% jitter

    return backoff.WithMaxRetries(b, 5) // Max 5 retries per requirements
}

// Usage
func deliverWithRetry(operation func() error) error {
    return backoff.Retry(operation, newSalesforceBackoff())
}
```

### Example 3: Recording API Usage After Delivery

```go
// Source: Adapted from existing salesforce_delivery.go pattern
func (s *SFDeliveryService) deliverBatch(
    ctx context.Context,
    orgID string,
    client *http.Client,
    payload []byte,
    job *entity.SyncJob,
) (int, error) {
    // Execute delivery (existing code)...
    resp, err := client.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == 200 || resp.StatusCode == 201 {
        // Parse Salesforce response to count API calls
        var results []map[string]interface{}
        json.Unmarshal(body, &results)
        deliveredCount := len(results)

        // NEW: Record API usage
        if err := s.rateLimitService.RecordAPIUsage(ctx, orgID, job.ID, deliveredCount); err != nil {
            log.Printf("Warning: Failed to record API usage for job %s: %v", job.ID, err)
            // Don't fail the job - usage tracking is non-critical
        }

        return deliveredCount, nil
    }

    // Handle errors...
}
```

### Example 4: Pre-Delivery Quota Check

```go
// Source: Pattern from existing QueueMergeInstructions flow
func (s *SFDeliveryService) QueueMergeInstructions(
    ctx context.Context,
    orgID string,
    inputs []MergeInstructionInput,
) (string, error) {
    // Existing checks...

    // NEW: Check API quota before queuing
    usage, err := s.rateLimitService.GetAPIUsageLast24Hours(ctx, orgID)
    if err != nil {
        log.Printf("Warning: Failed to check API usage, proceeding anyway: %v", err)
    } else {
        const pauseThreshold = 80000 // 80% of 100K
        if usage >= pauseThreshold {
            return "", &QuotaExceededError{
                OrgID:     orgID,
                Usage:     usage,
                Threshold: pauseThreshold,
            }
        }
    }

    // Proceed with batch assembly...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Fixed retry delays (2s, 2s, 2s) | Exponential backoff with jitter | 2023-2024 | Prevents thundering herd, respects server recovery time |
| Global rate limiters | Per-user/per-org rate limiters | 2024-2025 | Prevents one user's abuse from blocking others |
| Fixed time windows (midnight reset) | Sliding windows | 2022-2023 | Prevents boundary exploitation (2x requests in 2 minutes) |
| Ignore Retry-After header | Parse and respect Retry-After | Always standard | Respects server's explicit guidance |
| Token bucket only | Hybrid token bucket + sliding window | 2025 | Token bucket for burst, sliding window for long-term quota |

**Deprecated/outdated:**
- **Hard-coded 5-second delays:** Replaced by configurable exponential backoff
- **Counting retries without jitter:** Causes synchronized retry storms
- **UTC midnight resets:** Timezone-dependent, replaced by rolling 24-hour windows

## Open Questions

### Question 1: Should we use Redis for distributed API usage tracking?

**What we know:**
- Current implementation uses single Go process (Railway deployment)
- Turso tenant DBs provide distributed storage
- Redis would add operational complexity

**What's unclear:**
- Will we scale to multiple backend instances in Phase 19+?
- Is SQL query performance adequate for real-time quota checks?

**Recommendation:**
- Phase 18 MVP: Use tenant DB with indexed queries (simple, leverages existing Turso infrastructure)
- Future optimization: Add Redis if query latency becomes bottleneck (profile first)
- Decision point: If P99 latency for GetAPIUsageLast24Hours exceeds 100ms under load

### Question 2: Should we implement circuit breakers for Salesforce API?

**What we know:**
- Circuit breakers prevent cascading failures by failing fast after threshold
- sony/gobreaker is mature Go implementation
- Salesforce already has rate limiting (429 responses)

**What's unclear:**
- Would circuit breaker provide value beyond exponential backoff?
- Risk of circuit breaker tripping unnecessarily during normal rate limiting

**Recommendation:**
- Phase 18: Skip circuit breakers (exponential backoff is sufficient)
- Rationale: 429 errors are expected behavior, not outages - we want to retry, not fail fast
- Circuit breakers are for server failures (5xx), which we already retry with backoff
- If needed later: Add per-org circuit breaker for 5xx errors only (not 429)

### Question 3: How to display API usage percentage in admin UI?

**What we know:**
- Success criteria requires "displays current usage percentage"
- Backend has GetAPIUsageLast24Hours method
- Frontend needs real-time quota visibility

**What's unclear:**
- Should usage refresh on page load or poll continuously?
- Should we show detailed breakdown (per hour, per entity type)?

**Recommendation:**
- Phase 18 MVP: Add GET /salesforce/quota endpoint returning `{"usage": 45000, "limit": 100000, "percentage": 45, "threshold": 80000}`
- Frontend fetches on page load + after each manual trigger
- Future enhancement: WebSocket for real-time updates (Phase 19+)

## Sources

### Primary (HIGH confidence)

**Go Rate Limiting:**
- [golang.org/x/time/rate - Official Go rate limiter package](https://pkg.go.dev/golang.org/x/time/rate)
- [How to Implement Rate Limiting in Go (2026-01-23)](https://oneuptime.com/blog/post/2026-01-23-go-rate-limiting/view)
- [Implementing a Go and Redis-powered Sliding Window Rate Limiter](https://leapcell.io/blog/implementing-a-go-and-redis-powered-sliding-window-rate-limiter)

**Exponential Backoff:**
- [cenkalti/backoff/v4 - Go package documentation](https://pkg.go.dev/github.com/cenkalti/backoff/v4)
- [How to Implement Retry Logic in Go with Exponential Backoff (2026-01-07)](https://oneuptime.com/blog/post/2026-01-07-go-retry-exponential-backoff/view)
- [Exponential Backoff and Jitter - Tyler Crosse](https://www.tylercrosse.com/ideas/2022/exponential-backoff/)

**Salesforce API Limits:**
- [Salesforce API Rate Limits - Coefficient](https://coefficient.io/salesforce-api/salesforce-api-rate-limits)
- [API Request Limits and Allocations - Salesforce Developer Docs](https://developer.salesforce.com/docs/atlas.en-us.salesforce_app_limits_cheatsheet.meta/salesforce_app_limits_cheatsheet/salesforce_app_limits_platform_api.htm)
- [Best Practices to Prevent Rate-Limiting - Salesforce Marketing Cloud](https://developer.salesforce.com/docs/marketing/marketing-cloud/guide/rate-limiting-best-practices.html)

**Circuit Breakers:**
- [sony/gobreaker - Official GitHub repository](https://github.com/sony/gobreaker)
- [How to Implement Circuit Breakers in Go with sony/gobreaker (2026-01-07)](https://oneuptime.com/blog/post/2026-01-07-go-circuit-breaker/view)

### Secondary (MEDIUM confidence)

**Sliding Window Algorithms:**
- [Rate Limiting: The Sliding Window Algorithm - Medium](https://medium.com/@m-elbably/rate-limiting-the-sliding-window-algorithm-daa1d91e6196)
- [RussellLuo/slidingwindow - Go implementation](https://github.com/RussellLuo/slidingwindow)
- [Visualizing algorithms for rate limiting](https://smudge.ai/blog/ratelimit-algorithms)

**Monitoring:**
- [Instrumenting HTTP server in Go - Prometheus official tutorial](https://prometheus.io/docs/tutorials/instrumenting_http_server_in_go/)
- [Instrumenting & Monitoring Go Apps with Prometheus](https://betterstack.com/community/guides/monitoring/prometheus-golang/)

**HTTP 429 Handling:**
- [Retry-After header - MDN Web Docs](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Retry-After)
- [HTTP Error 429 (Too Many Requests) - Postman Blog](https://blog.postman.com/http-error-429/)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - golang.org/x/time/rate and cenkalti/backoff are industry standard, well-documented
- Architecture patterns: HIGH - Sliding window and exponential backoff are proven patterns, verified in research
- Salesforce specifics: MEDIUM - API limits confirmed via official docs, but Retry-After header format not fully verified
- Common pitfalls: HIGH - Thundering herd, timezone issues, boundary problems are well-documented anti-patterns

**Research date:** 2026-02-10
**Valid until:** 2026-03-10 (30 days - rate limiting patterns are stable)

**Notes:**
- Existing Phase 17 code already has basic retry (2s delay, max 2 attempts) - Phase 18 replaces with exponential backoff
- sync_jobs table exists in migration 061, no schema changes needed for job tracking
- API usage tracking requires new migration (062) for api_usage_log table
- Manual override endpoint already exists (/salesforce/trigger) - just needs force flag added
