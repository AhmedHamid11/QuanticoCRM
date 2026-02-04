---
phase: 10-audit-infrastructure
plan: 01
subsystem: security
tags: [audit, compliance, tamper-detection, hash-chain, go, sqlite]

requires:
  - phase: 09-session-management
    plan: all
    reason: Session management complete, ready for audit infrastructure

provides:
  - audit_logs table with tamper-evident hash chain
  - AuditLogEntry entity with SHA-256 hash computation
  - AuditRepo with Create, List, VerifyChainIntegrity operations
  - Extended AuditService with database persistence
  - Fire-and-forget pattern for non-blocking audit logging

affects:
  - phase: 10-audit-infrastructure
    plan: 02
    impact: Authentication event capture will use this persistence layer
  - phase: 10-audit-infrastructure
    plan: 03
    impact: Admin event capture will use this persistence layer
  - phase: 10-audit-infrastructure
    plan: 04
    impact: Audit UI will query via AuditRepo

tech-stack:
  added: []
  patterns:
    - Cryptographic hash chain for tamper detection (SHA-256)
    - Fire-and-forget goroutines for async DB writes
    - Per-org hash chain isolation (GENESIS per org)
    - Deterministic hash computation for consistency

key-files:
  created:
    - fastcrm/backend/internal/migrations/049_create_audit_logs.sql
    - fastcrm/backend/internal/entity/audit.go
    - fastcrm/backend/internal/repo/audit.go
  modified:
    - fastcrm/backend/internal/service/audit.go
    - fastcrm/backend/internal/handler/auth.go
    - fastcrm/backend/cmd/api/main.go
    - fastcrm/backend/internal/sfid/sfid.go

decisions:
  - id: AUD-01
    what: Use SHA-256 for entry hash computation
    why: Industry standard, sufficient collision resistance, widely supported
    alternatives: [SHA-512 (overkill), BLAKE2 (less common)]
  - id: AUD-02
    what: Fire-and-forget goroutine pattern for DB persistence
    why: Audit logging must not block user requests (< 50ms target)
    alternatives: [Synchronous writes (would add latency), Message queue (over-engineered)]
  - id: AUD-03
    what: Per-org hash chain with GENESIS start
    why: Org isolation maintains multi-tenant security boundaries
    alternatives: [Global chain (would leak cross-org data), No chain (no tamper detection)]
  - id: AUD-04
    what: No foreign keys on actor_id/target_id
    why: Audit logs must persist even if referenced users are deleted
    alternatives: [FK with ON DELETE SET NULL (loses information), FK with CASCADE (loses audit trail)]
  - id: AUD-05
    what: Re-export event types from entity package in service
    why: Maintains backwards compatibility for existing code using service.AuditEventType
    alternatives: [Break compatibility (would require updating all imports)]

metrics:
  duration: 4 min
  completed: 2026-02-04
  tasks: 3/3
  commits: 3
  deviations: 0

completed: 2026-02-04
---

# Phase 10 Plan 01: Audit Infrastructure Summary

**One-liner:** Tamper-evident audit log persistence with SHA-256 hash chain and fire-and-forget async writes

## What Was Built

### Migration: audit_logs table
- **File:** `049_create_audit_logs.sql`
- **Schema:** 15 columns including `prev_hash` and `entry_hash` for chain
- **Indexes:** 4 indexes for timeline queries, event filtering, actor filtering, chain verification
- **Hash chain:** Each entry links to previous via `prev_hash`, starts with "GENESIS" per org

### Entity: AuditLogEntry
- **File:** `entity/audit.go`
- **Purpose:** Database representation of audit events with hash computation
- **Key method:** `ComputeEntryHash()` - Deterministic SHA-256 hash of all fields
- **Event types added:** LOGOUT, USER_CREATE, USER_UPDATE, USER_DELETE, AUTHORIZATION_DENIED, ORG_SETTINGS_CHANGE
- **Supporting types:** AuditLogFilters, AuditLogListResponse, ChainVerificationResult

### Repository: AuditRepo
- **File:** `repo/audit.go`
- **Operations:**
  - `Create()` - Inserts entry with automatic hash chain linking
  - `GetLastEntryHash()` - Gets previous hash for chaining (returns "GENESIS" for first)
  - `List()` - Paginated queries with filters (event types, actor, date range)
  - `VerifyChainIntegrity()` - Validates entire org's hash chain for tampering
- **Helper:** `ConvertEventToEntry()` - Converts AuditEvent to AuditLogEntry

### Service: Extended AuditLogger
- **File:** `service/audit.go`
- **Changes:**
  - Accepts `*repo.AuditRepo` in constructor
  - `Log()` now persists to database after stdout (backwards compatible)
  - Fire-and-forget goroutine prevents blocking
  - Errors logged but don't fail caller
- **New convenience methods:**
  - `LogLogout()`
  - `LogUserCreate/Update/Delete()`
  - `LogOrgSettingsChange()`
  - `LogAuthorizationDenied()`
- **Backwards compatibility:** Re-exported types from entity package

### Wiring: Dependency Injection
- **File:** `cmd/api/main.go`
- Added `auditRepo` initialization with `masterDBConn`
- Pass `auditRepo` to `service.NewAuditLogger()`
- Pass `auditLogger` to `handler.NewAuthHandler()`
- **File:** `handler/auth.go`
- Updated `NewAuthHandler` to accept `auditLogger` parameter

### SFID Support
- **File:** `sfid/sfid.go`
- Added `PrefixAuditLog = "0Ad"`
- Added `NewAuditLog()` helper function

## Decisions Made

**AUD-01: SHA-256 for entry hashing**
- SHA-256 provides 256-bit collision resistance
- Deterministic field concatenation ensures consistency
- Format: `ID|OrgID|EventType|...|CreatedAt(RFC3339Nano)`

**AUD-02: Fire-and-forget async persistence**
- User requests complete without waiting for DB write
- Errors logged to stdout but don't fail requests
- Maintains < 50ms response time target

**AUD-03: Per-org hash chain isolation**
- Each org's chain starts with "GENESIS"
- Prevents cross-org data leakage
- Enables independent chain verification per org

**AUD-04: No foreign keys on actor/target**
- Audit logs persist forever, even if users deleted
- Stores `actor_email` redundantly for forensics
- Trade-off: Manual cleanup not automatic

**AUD-05: Backwards compatibility via re-exports**
- Existing code using `service.AuditEventType` still works
- Avoids mass import changes across codebase
- Entity package is source of truth

## Deviations from Plan

None - plan executed exactly as written.

## Technical Details

### Hash Chain Verification

```go
// Example chain for org "00D123":
Entry 1: prev_hash = "GENESIS", entry_hash = "abc123..."
Entry 2: prev_hash = "abc123...", entry_hash = "def456..."
Entry 3: prev_hash = "def456...", entry_hash = "ghi789..."
```

**Tamper detection:**
- If entry 2's data is modified, its hash changes
- Entry 3's `prev_hash` no longer matches
- `VerifyChainIntegrity()` detects the break

### Fire-and-Forget Pattern

```go
// In AuditLogger.Log()
go func() {
    entry, err := repo.ConvertEventToEntry(event)
    if err != nil {
        log.Printf("[AUDIT ERROR] Failed to convert: %v", err)
        return
    }
    if err := a.repo.Create(context.Background(), entry); err != nil {
        log.Printf("[AUDIT ERROR] Failed to persist: %v", err)
    }
}()
// User request continues immediately
```

### Database Schema Highlights

- **prev_hash TEXT NOT NULL** - Links to previous entry
- **entry_hash TEXT NOT NULL UNIQUE** - This entry's computed hash
- **success INTEGER NOT NULL DEFAULT 1** - SQLite boolean (0/1)
- **details TEXT** - JSON blob for event-specific data
- **Index on prev_hash** - Enables efficient chain traversal

## Testing Recommendations

1. **Hash computation consistency:**
   - Create entry, compute hash, verify it matches
   - Modify entry data, verify hash changes

2. **Chain integrity:**
   - Create 3 entries, verify chain
   - Manually modify entry 2 in DB, verify verification fails

3. **Fire-and-forget:**
   - Trigger audit event with repo error (bad connection)
   - Verify user request completes successfully
   - Verify error logged to stdout

4. **Multi-tenant isolation:**
   - Create entries for org A and org B
   - Verify org A's chain starts with GENESIS
   - Verify org B's chain starts with GENESIS independently

## Next Phase Readiness

**Ready for Plan 02 (Authentication Event Capture):**
- ✅ Audit infrastructure persists events to database
- ✅ Fire-and-forget pattern won't block login/logout
- ✅ New event types (LOGOUT, etc.) defined and ready

**Ready for Plan 03 (Admin Event Capture):**
- ✅ USER_CREATE/UPDATE/DELETE event types defined
- ✅ ORG_SETTINGS_CHANGE event type defined
- ✅ AUTHORIZATION_DENIED event type defined

**Ready for Plan 04 (Audit Log UI):**
- ✅ AuditRepo.List() supports filtering and pagination
- ✅ ChainVerificationResult ready for UI display
- ✅ Event types have human-readable labels in entity

## Commits

| Hash    | Message                                         |
|---------|-------------------------------------------------|
| 94017ae | feat(10-01): create audit_logs migration and entity |
| 5fc2892 | feat(10-01): create audit repository            |
| 8cb6499 | feat(10-01): extend audit service with database persistence |

---

**Status:** ✅ Complete - All success criteria met
**Duration:** 4 minutes
**Blockers:** None
