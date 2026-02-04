# Phase 10: Audit Infrastructure - Research

**Researched:** 2026-02-04
**Domain:** Security audit logging, tamper-evident storage, CI security scanning
**Confidence:** HIGH

## Summary

This phase implements comprehensive audit logging for security events, tamper-evident storage, and CI pipeline security scanning. The research establishes patterns for three distinct areas:

1. **Audit Event Capture** - Extending the existing `service/audit.go` to persist events to SQLite with hash chaining for tamper evidence. The existing codebase already has audit event types defined (LOGIN_SUCCESS, LOGIN_FAILED, ROLE_CHANGE, etc.) but only logs to stdout.

2. **Admin UI for Audit Logs** - Building an activity feed (timeline-style) in the admin section using Tailwind CSS. Flowbite-style patterns work well with the existing SvelteKit frontend.

3. **CI Security Scanning** - Integrating gosec for SAST and govulncheck/Dependabot for dependency scanning via GitHub Actions.

**Primary recommendation:** Persist audit events to an `audit_logs` table with SHA-256 hash chaining for tamper-evidence, expose via API with org-scoped access control, and display in a timeline-style activity feed in the admin panel.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| crypto/sha256 | stdlib | Hash chaining | Standard Go, no dependencies, FIPS-compliant |
| encoding/json | stdlib | Canonical serialization | Native Go, deterministic with sorted keys |
| gosec | v2.x | Go SAST scanning | Dominant Go security scanner, CWE-mapped rules |
| govulncheck | latest | Dependency vuln scanning | Official Go tool, low false positives |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| time | stdlib | UTC timestamps | All audit timestamps |
| github/codeql-action | v2 | SARIF upload | GitHub security dashboard integration |
| flowbite-svelte | existing | Timeline UI | Activity feed components |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| SHA-256 hash chain | Merkle tree | Merkle trees better for verification at scale, but hash chain simpler for append-only sequential logs |
| gosec | Semgrep | Semgrep more general but gosec is Go-specific with better rule coverage |
| govulncheck | nancy | Nancy uses OSS Index, govulncheck uses Go's official vuln DB with call-graph analysis (fewer false positives) |

**Installation:**
```bash
# gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## Architecture Patterns

### Recommended Project Structure
```
backend/
├── internal/
│   ├── entity/
│   │   └── audit.go           # Audit event types and structs
│   ├── service/
│   │   └── audit.go           # Extend existing - add persistence
│   ├── repo/
│   │   └── audit.go           # New - database operations
│   ├── handler/
│   │   └── audit.go           # New - API endpoints
│   └── migrations/
│       └── 049_create_audit_logs.sql  # New migration

frontend/
├── src/routes/admin/
│   └── audit-logs/
│       └── +page.svelte       # Activity feed UI
```

### Pattern 1: Hash Chain for Tamper Evidence
**What:** Each audit log entry includes the hash of the previous entry, creating a cryptographic chain.
**When to use:** Any append-only log requiring tamper detection.
**Example:**
```go
// Source: Hash chain patterns from SQLite and Go research
type AuditLogEntry struct {
    ID            string    `json:"id" db:"id"`
    OrgID         string    `json:"orgId" db:"org_id"`
    EventType     string    `json:"eventType" db:"event_type"`
    ActorID       string    `json:"actorId,omitempty" db:"actor_id"`
    ActorEmail    string    `json:"actorEmail,omitempty" db:"actor_email"`
    TargetID      string    `json:"targetId,omitempty" db:"target_id"`
    TargetType    string    `json:"targetType,omitempty" db:"target_type"`
    IPAddress     string    `json:"ipAddress,omitempty" db:"ip_address"`
    UserAgent     string    `json:"userAgent,omitempty" db:"user_agent"`
    Details       string    `json:"details,omitempty" db:"details"` // JSON
    Success       bool      `json:"success" db:"success"`
    ErrorMsg      string    `json:"errorMsg,omitempty" db:"error_msg"`
    PrevHash      string    `json:"prevHash" db:"prev_hash"`
    EntryHash     string    `json:"entryHash" db:"entry_hash"`
    CreatedAt     time.Time `json:"createdAt" db:"created_at"`
}

// ComputeEntryHash creates deterministic hash of entry + previous hash
func (e *AuditLogEntry) ComputeEntryHash() string {
    // Canonical representation (sorted keys, consistent format)
    data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%t|%s|%s",
        e.ID, e.OrgID, e.EventType, e.ActorID, e.ActorEmail,
        e.TargetID, e.TargetType, e.IPAddress, e.Details,
        e.Success, e.ErrorMsg, e.PrevHash,
        e.CreatedAt.UTC().Format(time.RFC3339Nano))
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}
```

### Pattern 2: Org-Scoped Audit Log Chain
**What:** Each org has its own independent hash chain, preventing cross-org tampering detection issues.
**When to use:** Multi-tenant audit systems.
**Example:**
```go
// Get the last entry hash for an org to chain the next entry
func (r *AuditRepo) GetLastEntryHash(ctx context.Context, orgID string) (string, error) {
    var prevHash string
    err := r.db.QueryRowContext(ctx, `
        SELECT entry_hash FROM audit_logs
        WHERE org_id = ?
        ORDER BY created_at DESC LIMIT 1
    `, orgID).Scan(&prevHash)
    if err == sql.ErrNoRows {
        return "GENESIS", nil // First entry in chain
    }
    return prevHash, err
}
```

### Pattern 3: Authorization Failure Logging Middleware
**What:** Intercept 403 responses to automatically log authorization failures.
**When to use:** Capturing denied access attempts without modifying every handler.
**Example:**
```go
// Source: Fiber middleware pattern
func AuditAuthorizationFailures(auditService *AuditService) fiber.Handler {
    return func(c *fiber.Ctx) error {
        err := c.Next()

        // Check if response was 403 Forbidden
        if c.Response().StatusCode() == fiber.StatusForbidden {
            auditService.LogAuthorizationFailure(c.Context(), AuditEvent{
                EventType:  AuditEventAuthorizationDenied,
                ActorID:    c.Locals("userID").(string),
                ActorEmail: c.Locals("email").(string),
                OrgID:      c.Locals("orgID").(string),
                IPAddress:  c.IP(),
                UserAgent:  c.Get("User-Agent"),
                Details: map[string]interface{}{
                    "path":   c.Path(),
                    "method": c.Method(),
                },
            })
        }
        return err
    }
}
```

### Pattern 4: Activity Feed API Response
**What:** Paginated, filtered audit log response optimized for timeline UI.
**When to use:** Admin audit log viewing.
**Example:**
```go
type AuditLogListResponse struct {
    Items      []AuditLogEntry `json:"items"`
    Total      int             `json:"total"`
    Page       int             `json:"page"`
    PageSize   int             `json:"pageSize"`
    HasMore    bool            `json:"hasMore"`
    DateRange  DateRange       `json:"dateRange,omitempty"`
}

type AuditLogFilters struct {
    EventTypes []string   `query:"eventTypes"`
    UserID     string     `query:"userId"`
    DateFrom   *time.Time `query:"dateFrom"`
    DateTo     *time.Time `query:"dateTo"`
    Page       int        `query:"page"`
    PageSize   int        `query:"pageSize"`
}
```

### Anti-Patterns to Avoid
- **Storing plain logs without hash chaining:** Defeats tamper-evidence, any admin could modify historical logs undetected
- **Using mutable fields in hash computation:** If you include fields that can change (like "viewed" status), hash verification will break
- **Global hash chain across orgs:** Makes cross-org queries expensive and chain breaks in one org affect all
- **Synchronous audit logging in hot paths:** Use goroutines for fire-and-forget logging to avoid blocking user requests

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Go security scanning | Custom regex rules | gosec | 40+ CWE-mapped rules, maintained by security community |
| Dependency vuln scanning | Manual CVE checking | govulncheck | Official Go tool with call-graph analysis, updated vuln DB |
| GitHub code scanning | Custom reporting | SARIF + codeql-action | Native GitHub integration, dashboard, alerts |
| Timeline UI components | Custom CSS timeline | flowbite-svelte Timeline | Already in codebase patterns, accessible |
| Deterministic JSON | json.Marshal | Canonical JSON with sorted keys | Standard json.Marshal has non-deterministic key order |

**Key insight:** Security tooling requires constant maintenance as new vulnerabilities emerge. Using standard tools means benefiting from community updates.

## Common Pitfalls

### Pitfall 1: Non-Deterministic Hash Input
**What goes wrong:** JSON serialization produces different byte sequences for identical data due to map iteration order
**Why it happens:** Go maps don't guarantee iteration order, and json.Marshal reflects this
**How to avoid:** Use explicit field concatenation with fixed order, or a canonical JSON library
**Warning signs:** Hash verification fails intermittently on identical-looking records

### Pitfall 2: Blocking User Requests on Audit Logging
**What goes wrong:** Login/logout becomes slow because audit logging does synchronous DB writes
**Why it happens:** Treating audit logging as part of the critical path
**How to avoid:** Use goroutines with `go auditService.Log(...)` pattern (fire and forget)
**Warning signs:** Increased latency on auth endpoints, especially during DB slowdowns

### Pitfall 3: Missing Event Type Coverage
**What goes wrong:** Important security events aren't captured, creating compliance gaps
**Why it happens:** Adding audit calls only to obvious places like login
**How to avoid:** Systematic review of all auth/admin handlers against requirements checklist
**Warning signs:** Compliance audit reveals missing event types

### Pitfall 4: gosec False Positive Fatigue
**What goes wrong:** Team ignores gosec findings because too many false positives
**Why it happens:** Running gosec on all code including test files and generated code
**How to avoid:** Use `--exclude-generated`, exclude test files, use `.gosec` config file for known false positives
**Warning signs:** Excessive #nosec annotations, CI failures being ignored

### Pitfall 5: Verification Never Running
**What goes wrong:** Tamper-evident chain exists but tampering is never detected
**Why it happens:** No scheduled verification process
**How to avoid:** Add periodic verification job or on-demand verification endpoint
**Warning signs:** Chain integrity only checked during development

## Code Examples

Verified patterns from research:

### Audit Log Migration (SQLite)
```sql
-- Migration: 049_create_audit_logs.sql
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    actor_id TEXT,
    actor_email TEXT,
    target_id TEXT,
    target_type TEXT,
    ip_address TEXT,
    user_agent TEXT,
    details TEXT,  -- JSON blob
    success INTEGER NOT NULL DEFAULT 1,
    error_msg TEXT,
    prev_hash TEXT NOT NULL,
    entry_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Foreign key intentionally omitted to allow logging even if user is deleted
    -- Audit logs must persist independently of user lifecycle

    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_created ON audit_logs(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_type ON audit_logs(org_id, event_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(org_id, actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_prev_hash ON audit_logs(prev_hash);
```

### gosec GitHub Actions Workflow
```yaml
# .github/workflows/security.yml
name: Security Scanning

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  gosec:
    name: Go Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: securego/gosec@master
        with:
          args: >
            -fmt sarif
            -out results.sarif
            -exclude-generated
            -severity high
            ./backend/...

      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: results.sarif

  govulncheck:
    name: Dependency Vulnerabilities
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ./FastCRM/fastcrm/backend
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest

      - name: Run govulncheck
        run: govulncheck ./...
```

### Activity Feed Svelte Component Structure
```svelte
<!-- Pattern based on existing admin pages and flowbite timeline -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from '$lib/utils/api';

  interface AuditLogEntry {
    id: string;
    eventType: string;
    actorEmail: string;
    targetType?: string;
    targetId?: string;
    details?: Record<string, unknown>;
    success: boolean;
    createdAt: string;
  }

  let logs = $state<AuditLogEntry[]>([]);
  let loading = $state(true);
  let filters = $state({
    eventType: '',
    dateFrom: '',
    dateTo: '',
    page: 1,
    pageSize: 50
  });
</script>

<!-- Timeline structure -->
<ol class="relative border-s border-gray-200 ml-3">
  {#each logs as log (log.id)}
    <li class="mb-10 ms-6">
      <span class="absolute flex items-center justify-center w-6 h-6
                   {log.success ? 'bg-blue-100' : 'bg-red-100'}
                   rounded-full -start-3 ring-8 ring-white">
        <!-- Event type icon -->
      </span>
      <div class="p-4 bg-white border border-gray-200 rounded-lg shadow-sm">
        <div class="items-center justify-between sm:flex">
          <time class="text-xs text-gray-400">
            {new Date(log.createdAt).toLocaleString()}
          </time>
          <span class="text-xs font-medium text-gray-500">
            {log.eventType}
          </span>
        </div>
        <div class="text-sm font-normal text-gray-500">
          <span class="font-semibold">{log.actorEmail}</span>
          {getEventDescription(log)}
        </div>
      </div>
    </li>
  {/each}
</ol>
```

### Hash Chain Verification
```go
// VerifyChainIntegrity checks the hash chain for tampering
func (r *AuditRepo) VerifyChainIntegrity(ctx context.Context, orgID string) (*ChainVerificationResult, error) {
    rows, err := r.db.QueryContext(ctx, `
        SELECT id, event_type, actor_id, actor_email, target_id, target_type,
               ip_address, details, success, error_msg, prev_hash, entry_hash, created_at
        FROM audit_logs
        WHERE org_id = ?
        ORDER BY created_at ASC
    `, orgID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    result := &ChainVerificationResult{Valid: true}
    var prevHash string = "GENESIS"

    for rows.Next() {
        var entry AuditLogEntry
        // scan entry...

        // Verify prev_hash points to previous entry
        if entry.PrevHash != prevHash {
            result.Valid = false
            result.Errors = append(result.Errors, fmt.Sprintf(
                "Entry %s: prev_hash mismatch (expected %s, got %s)",
                entry.ID, prevHash, entry.PrevHash))
        }

        // Verify entry_hash is computed correctly
        computedHash := entry.ComputeEntryHash()
        if entry.EntryHash != computedHash {
            result.Valid = false
            result.Errors = append(result.Errors, fmt.Sprintf(
                "Entry %s: entry_hash mismatch (stored %s, computed %s)",
                entry.ID, entry.EntryHash, computedHash))
        }

        prevHash = entry.EntryHash
        result.EntriesVerified++
    }

    return result, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Log to stdout only | Persist to DB with hash chain | 2020+ | Enables compliance reporting, tamper detection |
| Manual security review | SAST in CI (gosec) | 2018+ | Catches vulnerabilities before merge |
| Manual CVE checking | govulncheck | 2022 | Official Go tool, call-graph aware |
| Dense data tables | Activity feed/timeline | 2020+ | Better UX for admins scanning logs |

**Deprecated/outdated:**
- **nancy for Go deps:** govulncheck is now preferred (official, call-graph analysis)
- **Synchronous audit logging:** Async patterns standard to avoid blocking

## Open Questions

Things that couldn't be fully resolved:

1. **Log Retention Period**
   - What we know: Logs should be retained for compliance, typically 90 days to 7 years
   - What's unclear: Specific retention requirements for this project
   - Recommendation: Implement configurable retention with 90-day default, add cleanup job

2. **Cross-Org Platform Admin View**
   - What we know: Platform admins need to view all orgs' audit logs
   - What's unclear: Should there be a separate platform-wide audit view or filter within org view?
   - Recommendation: Implement both - org-scoped by default, platform view for admins

3. **Real-time Updates**
   - What we know: User wants activity feed like GitHub
   - What's unclear: Whether real-time updates (WebSocket/SSE) are needed
   - Recommendation: Start with manual refresh, add polling if requested

## Sources

### Primary (HIGH confidence)
- [gosec GitHub Repository](https://github.com/securego/gosec) - Installation, CI integration, rules
- [govulncheck Tutorial](https://go.dev/doc/tutorial/govulncheck) - Official Go docs
- Existing codebase analysis - `service/audit.go`, `middleware/auth.go`, `handler/auth.go`

### Secondary (MEDIUM confidence)
- [Flowbite Svelte Timeline](https://flowbite-svelte.com/docs/components/timeline) - Activity feed UI patterns
- [Viget Hash Chain in SQLite](https://www.viget.com/articles/lets-make-a-hash-chain-in-sqlite) - SQLite hash chain implementation
- [Tamper-Evident Audit Log with SHA-256](https://dev.to/veritaschain/building-a-tamper-evident-audit-log-with-sha-256-hash-chains-zero-dependencies-h0b) - Hash chain verification patterns
- [GitHub Dependabot Docs](https://docs.github.com/en/code-security/dependabot/dependabot-alerts) - Dependency scanning

### Tertiary (LOW confidence)
- Web search results for audit logging best practices - general patterns confirmed with official sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using Go stdlib crypto and official Go tools
- Architecture: HIGH - Based on existing codebase patterns and official docs
- Pitfalls: MEDIUM - Derived from community patterns and research articles

**Research date:** 2026-02-04
**Valid until:** 30 days (stable domain, tools update periodically)
