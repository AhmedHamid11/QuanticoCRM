# Phase 19: Audit Logging & Admin Configuration - Research

**Researched:** 2026-02-10
**Domain:** Audit logging, SOX compliance, admin UI patterns
**Confidence:** HIGH

## Summary

Phase 19 extends the existing audit logging system to capture Salesforce merge instruction delivery events and provides admin UI for querying audit logs. The codebase already has a robust, tamper-evident audit logging foundation with hash chains (migration 049_create_audit_logs.sql). This phase focuses on **extending, not rebuilding**.

Key findings: (1) Existing audit system supports 7-year retention and tamper-evident storage via hash chains. (2) SOX compliance requires comprehensive logging of financial system changes with 7-year retention. (3) Salesforce delivery events need dedicated audit event types. (4) Admin UI needs filtering by batch_id, date range, org, and result status. (5) Connection status monitoring follows real-time dashboard patterns with clear status indicators.

**Primary recommendation:** Extend existing audit system with Salesforce-specific event types, add retention cleanup job, and build admin audit log query UI following existing patterns in `/admin/integrations/salesforce`.

## Standard Stack

### Core (Already in Place)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| SQLite | 3.x | Audit log storage | Multi-tenant DB, existing migration 049 |
| Go stdlib crypto/sha256 | 1.24 | Hash chain computation | Zero-dependency tamper detection |
| Fiber v2 | 2.x | HTTP handlers for audit APIs | Matches existing backend stack |
| SvelteKit | 2.x | Admin UI components | Matches existing frontend stack |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Fiber JSON encoder | stdlib | Serialize audit event details | All audit log writes |
| SQLite date functions | builtin | Date range filtering | Audit log queries |
| SQLite indexes | builtin | Fast timeline queries | Existing in migration 049 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| SQLite | External audit DB (Postgres) | SQLite simpler, multi-tenant per-org DB already works |
| Hash chain | Digital signatures (RSA/ECDSA) | Hash chain simpler, sufficient for tamper detection without key management |
| Manual cleanup | SQLite triggers | Manual cleanup more explicit, easier to verify compliance |

**Installation:**
No new dependencies required. Extend existing audit system.

## Architecture Patterns

### Recommended Project Structure
```
backend/
├── internal/
│   ├── entity/
│   │   └── audit.go              # Add Salesforce event types
│   ├── service/
│   │   ├── audit.go              # Add Salesforce logging methods
│   │   └── audit_retention.go   # NEW: 7-year retention cleanup
│   ├── repo/
│   │   └── audit.go              # Extend query filters
│   └── handler/
│       └── audit.go              # Add admin endpoints
frontend/
├── src/routes/admin/
│   └── audit-logs/
│       └── +page.svelte          # NEW: Audit log query UI
```

### Pattern 1: Salesforce Audit Event Logging
**What:** Log every merge instruction delivery attempt with batch_id, instruction_id, winner/loser IDs, delivery status, and Salesforce API response.

**When to use:** In `salesforce_delivery.go` during batch delivery (success, error, retry).

**Example:**
```go
// In deliverBatch() after HTTP request completes
func (s *SFDeliveryService) deliverBatch(...) (int, error) {
    // ... existing delivery logic ...

    // Log delivery attempt
    auditLogger.LogSalesforceMergeDelivery(ctx, entity.SalesforceDeliveryEvent{
        OrgID:         orgID,
        BatchID:       job.BatchID,
        InstructionID: instructionID, // from batch payload
        WinnerID:      winnerID,
        LoserID:       loserID,
        DeliveryStatus: "success", // or "error", "retry"
        StatusCode:    resp.StatusCode,
        ResponseBody:  truncate(body, 1000), // Store first 1KB
        RetryCount:    job.RetryCount,
    })
}
```

### Pattern 2: Retention Policy Cleanup Job
**What:** Scheduled job that deletes audit logs older than 7 years, preserving hash chain integrity.

**When to use:** Run daily at low-traffic hours (e.g., 3 AM UTC).

**Example:**
```go
// backend/internal/service/audit_retention.go
type RetentionService struct {
    repo *repo.AuditRepo
}

func (s *RetentionService) CleanupOldLogs(ctx context.Context, orgID string) error {
    // SOX compliance: 7 years = 2555 days
    cutoffDate := time.Now().UTC().AddDate(-7, 0, 0)

    // Delete entries older than cutoff
    // Note: Hash chain breaks here, but that's acceptable for deleted records
    return s.repo.DeleteOlderThan(ctx, orgID, cutoffDate)
}
```

### Pattern 3: Admin Audit Log Query UI
**What:** Paginated audit log viewer with filters for batch_id, date range, event type, result status.

**When to use:** Admin users investigating delivery issues or compliance audits.

**Example:**
```svelte
<!-- frontend/src/routes/admin/audit-logs/+page.svelte -->
<script lang="ts">
    let filters = $state({
        batchId: '',
        dateFrom: '',
        dateTo: '',
        eventTypes: ['SALESFORCE_MERGE_DELIVERY'],
        resultStatus: 'all', // all, success, error
        page: 1,
        pageSize: 50
    });

    async function loadAuditLogs() {
        const queryParams = new URLSearchParams({
            page: filters.page,
            pageSize: filters.pageSize,
            eventTypes: filters.eventTypes.join(','),
            dateFrom: filters.dateFrom,
            dateTo: filters.dateTo
        });

        if (filters.batchId) {
            // Search in details JSON field
            queryParams.append('batchId', filters.batchId);
        }

        const response = await get(`/audit-logs?${queryParams}`);
        logs = response.data;
    }
</script>
```

### Anti-Patterns to Avoid
- **Don't delete audit logs manually** - Use retention service with compliance verification
- **Don't store entire Salesforce response** - Truncate to 1KB to avoid bloat
- **Don't query audit logs without pagination** - Always use LIMIT/OFFSET
- **Don't expose audit logs to non-admin users** - Requires role check

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tamper detection | Custom hash verification | Existing hash chain in `audit.go` | Already implemented, battle-tested |
| Audit log storage | Custom append-only file | Existing SQLite table | Hash chain already integrated |
| Date range filtering | Custom date parsing | SQLite datetime functions | Built-in, indexed, efficient |
| Admin role checking | Custom RBAC | Existing middleware.RequireAdmin() | Already enforces admin-only access |
| CSV export | Custom CSV writer | Existing audit handler Export() | Already implements CSV export |

**Key insight:** The audit system foundation is already compliance-ready. Phase 19 is about **extending event types** and **adding admin query UI**, not rebuilding the audit infrastructure.

## Common Pitfalls

### Pitfall 1: Breaking Hash Chain Integrity During Cleanup
**What goes wrong:** Deleting old audit logs breaks the hash chain, causing verification to fail.

**Why it happens:** Hash chain links each entry to the previous entry's hash. Deleting old entries leaves orphaned prev_hash references.

**How to avoid:**
- Document that retention cleanup intentionally breaks the chain for deleted records
- Only delete entries older than 7 years (compliance boundary)
- Provide VerifyChainSince(date) method that verifies from a specific date forward
- Log retention cleanup actions in a separate retention_log table

**Warning signs:** Chain verification fails after cleanup, prev_hash points to non-existent entry.

### Pitfall 2: Storing Full Salesforce Response Bodies
**What goes wrong:** Audit logs table grows rapidly (multi-GB), queries slow down.

**Why it happens:** Salesforce composite API responses can be large (10KB+ per batch), storing every response creates bloat.

**How to avoid:**
- Truncate response bodies to first 1KB (capture error message, not entire payload)
- Store only essential fields: status_code, error_message, batch_id, instruction_id
- Use batch_payload in sync_jobs table for full payloads (separate table, not audit logs)

**Warning signs:** audit_logs table >1GB after a few months, queries take >5 seconds.

### Pitfall 3: Querying Audit Logs Without Org Isolation
**What goes wrong:** Admin from Org A sees audit logs from Org B (data leak).

**Why it happens:** Forgetting to filter by orgID in WHERE clause.

**How to avoid:**
- Always include `org_id = ?` in WHERE clause (enforced by AuditRepo.List)
- Platform admins can override orgID via query param (explicit permission check)
- Existing audit handler already enforces this pattern (lines 43-54 in handler/audit.go)

**Warning signs:** Audit logs show events from multiple orgs when filtered for one org.

### Pitfall 4: Not Paginating Audit Log Queries
**What goes wrong:** Admin UI loads 100K+ audit entries, browser freezes.

**Why it happens:** No LIMIT/OFFSET in query, loading entire audit history.

**How to avoid:**
- Default pageSize: 50, max pageSize: 100 (existing pattern in AuditLogFilters)
- Always show page controls in UI
- Use cursor-based pagination for very large result sets (optional optimization)

**Warning signs:** Audit log page takes >30 seconds to load, browser becomes unresponsive.

## Code Examples

Verified patterns from existing codebase:

### Logging a Salesforce Delivery Event
```go
// backend/internal/service/audit.go (extend with new method)
func (a *AuditLogger) LogSalesforceMergeDelivery(ctx context.Context, event SalesforceDeliveryEvent) {
    a.Log(ctx, AuditEvent{
        EventType: AuditEventSalesforceMergeDelivery,
        ActorID:   "system", // Automated delivery
        OrgID:     event.OrgID,
        Success:   event.DeliveryStatus == "success",
        ErrorMsg:  event.ErrorMessage,
        Details: map[string]interface{}{
            "batchId":        event.BatchID,
            "instructionId":  event.InstructionID,
            "winnerId":       event.WinnerID,
            "loserId":        event.LoserID,
            "deliveryStatus": event.DeliveryStatus,
            "statusCode":     event.StatusCode,
            "retryCount":     event.RetryCount,
            "responseBody":   event.ResponseBody, // Truncated to 1KB
        },
    })
}
```

### Querying Audit Logs by Batch ID
```go
// backend/internal/repo/audit.go (extend List method)
// Add BatchID filter to AuditLogFilters
if filters.BatchID != "" {
    // Search in details JSON field
    whereClauses = append(whereClauses, "details LIKE ?")
    args = append(args, "%\"batchId\":\""+filters.BatchID+"\"%")
}
```

### Admin UI Filter Form
```svelte
<!-- frontend/src/routes/admin/audit-logs/+page.svelte -->
<div class="space-y-4">
    <input
        type="text"
        bind:value={filters.batchId}
        placeholder="Filter by Batch ID"
        class="block w-full rounded-md border-gray-300"
    />

    <div class="grid grid-cols-2 gap-4">
        <input
            type="date"
            bind:value={filters.dateFrom}
            placeholder="From Date"
            class="block w-full rounded-md border-gray-300"
        />
        <input
            type="date"
            bind:value={filters.dateTo}
            placeholder="To Date"
            class="block w-full rounded-md border-gray-300"
        />
    </div>

    <select bind:value={filters.resultStatus} class="block w-full rounded-md border-gray-300">
        <option value="all">All Results</option>
        <option value="success">Success Only</option>
        <option value="error">Errors Only</option>
    </select>

    <button onclick={loadAuditLogs} class="px-4 py-2 bg-blue-600 text-white rounded-md">
        Apply Filters
    </button>
</div>
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Log to stdout only | Persist to database with hash chain | Phase 11 (2025) | Tamper-evident, queryable |
| No retention policy | Manual 7-year retention cleanup | Phase 19 (2026) | SOX compliance |
| Admin queries logs via SSH | Admin UI for audit log queries | Phase 19 (2026) | Self-service compliance audits |
| Generic audit events | Domain-specific event types (Salesforce) | Phase 19 (2026) | Better filtering, compliance reporting |

**Deprecated/outdated:**
- Stdout-only logging: Now persisted to database (stdout still used as backup)
- Single AuditEvent type: Now has 20+ specific event types (login, merge, API token, Salesforce delivery)

## Open Questions

1. **Retention cleanup frequency**
   - What we know: SOX requires 7-year retention
   - What's unclear: Should cleanup run daily, weekly, or monthly?
   - Recommendation: Run weekly (Sunday 3 AM UTC) - balances storage costs with compliance safety margin

2. **Admin audit log export limits**
   - What we know: Existing Export endpoint limits to 10K records
   - What's unclear: Is 10K sufficient for compliance audits?
   - Recommendation: Add "Export All" option for platform admins, with async job for large exports

3. **Hash chain verification after retention cleanup**
   - What we know: Deleting old entries breaks the chain
   - What's unclear: How to verify chain integrity for records within retention window?
   - Recommendation: Add VerifyChainSince(date) method that verifies from cutoff date forward

4. **Multi-org audit log queries for platform admins**
   - What we know: Existing handler allows platform admins to query other orgs
   - What's unclear: Should platform admins see aggregated cross-org audit logs?
   - Recommendation: Add /admin/platform/audit-logs endpoint for cross-org queries (separate UI, explicit permission)

## Sources

### Primary (HIGH confidence)
- Existing audit system: `/backend/internal/entity/audit.go`, `/backend/internal/service/audit.go`, `/backend/internal/repo/audit.go`
- Migration 049: `/migrations/049_create_audit_logs.sql`
- Salesforce delivery service: `/backend/internal/service/salesforce_delivery.go`
- Salesforce handler: `/backend/internal/handler/salesforce.go`
- Admin UI pattern: `/frontend/src/routes/admin/integrations/salesforce/+page.svelte`

### Secondary (MEDIUM confidence)
- [SOX Compliance Data Retention Requirements](https://pathlock.com/learn/sox-data-retention-requirements/) - 7-year retention requirement
- [Security Log Retention Best Practices Guide](https://auditboard.com/blog/security-log-retention-best-practices-guide) - Audit logging compliance patterns
- [SOX Data Retention Requirements 2026 Guide](https://www.armstrongarchives.com/sox-data-retention-requirements/) - Updated compliance guidance
- [Top Admin Dashboard Design Ideas for 2026](https://www.fanruan.com/en/blog/top-admin-dashboard-design-ideas-inspiration) - Real-time status monitoring patterns
- [12 Best Status Dashboard Software of 2026](https://cpoclub.com/tools/best-status-dashboard-software/) - Integration status monitoring
- [Fiber Monitor Middleware](https://docs.gofiber.io/api/middleware/monitor/) - Built-in monitoring capabilities
- [OpenTelemetry in Go Fiber 2026](https://oneuptime.com/blog/post/2026-02-06-opentelemetry-go-fiber-otelfiber/view) - Modern observability patterns

### Tertiary (LOW confidence)
- WebSearch on SQLite retention policies - no native retention features found, must implement manually
- WebSearch on admin UI patterns - general design trends, not specific implementation guidance

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Existing code review, verified patterns
- Architecture: HIGH - Extends existing audit system, minimal new patterns
- Pitfalls: HIGH - Based on existing codebase patterns and common SQLite issues
- SOX compliance: MEDIUM - WebSearch verified, but not legal advice
- Admin UI patterns: MEDIUM - General trends, not specific to this codebase

**Research date:** 2026-02-10
**Valid until:** 60 days (stable domain - audit logging patterns change slowly)

**Key constraints:**
- Must extend existing audit system, not rebuild
- Must maintain hash chain compatibility
- Must enforce 7-year retention (SOX compliance)
- Must isolate audit logs by orgID
- Admin UI must follow existing Salesforce admin page patterns
