---
phase: 13-merge-engine
verified: 2026-02-07T23:30:00Z
status: gaps_found
score: 4/5 must-haves verified
gaps:
  - truth: "POST /api/v1/merge/undo/:snapshotId restores pre-merge state within 30-day window"
    status: failed
    reason: "Route registration has typo - missing '/' before :snapshotId parameter"
    artifacts:
      - path: "backend/internal/handler/merge.go"
        issue: "Line 283: merge.Post(\"/undo:snapshotId\", h.Undo) should be \"/undo/:snapshotId\""
    missing:
      - "Fix route registration to use correct path: merge.Post(\"/undo/:snapshotId\", h.Undo)"
---

# Phase 13: Merge Engine Verification Report

**Phase Goal:** Complete merge capability with field selection, related record transfer, audit logging, and undo

**Verified:** 2026-02-07T23:30:00Z

**Status:** gaps_found

**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can merge 2+ duplicate records (API supports survivor/field selection) | ✓ VERIFIED | POST /merge/execute accepts survivorId, duplicateIds, mergedFields, entityType |
| 2 | All related records transfer to survivor automatically | ✓ VERIFIED | DiscoverRelatedRecords scans metadata for lookup fields, ExecuteMerge transfers FKs in transaction |
| 3 | Merge executes atomically with full audit log | ✓ VERIFIED | db.BeginTx, tx.Commit, deferred rollback, LogRecordMerge called |
| 4 | Merge preview shows before/after comparison, related counts, data loss warnings | ✓ VERIFIED | POST /merge/preview returns records, completenessScores, suggestedSurvivorId, relatedRecordCounts, fields |
| 5 | User can undo merge within 30 days | ✗ FAILED | UndoMerge logic correct, but route registration has typo: "/undo:snapshotId" instead of "/undo/:snapshotId" |

**Score:** 4/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `backend/internal/migrations/055_create_merge_snapshots.sql` | CREATE TABLE with snapshot columns | ✓ VERIFIED | 21 lines, substantive, has id/org_id/survivor_id/duplicate_ids/consumed_at/expires_at |
| `backend/internal/migrations/056_add_archive_columns.sql` | Archive column schema contract | ✓ VERIFIED | 16 lines, documents archived_at/archived_reason/survivor_id pattern |
| `backend/internal/entity/merge.go` | Go entity types for merge | ✓ VERIFIED | 94 lines, MergeSnapshot, MergeRequest, MergeResult, MergePreview, RelatedRecordGroup, FKChange |
| `backend/internal/sfid/sfid.go` | SFID prefix for merge snapshots | ✓ VERIFIED | PrefixMergeSnapshot = "0Ms", NewMergeSnapshot() function exists |
| `backend/internal/repo/merge.go` | Merge snapshot CRUD operations | ✓ VERIFIED | 399 lines, Save/GetByID/GetBySurvivor/ListByOrg/MarkConsumed/CleanupExpired/EnsureArchiveColumns |
| `backend/internal/service/merge_discovery.go` | Related record discovery via metadata | ✓ VERIFIED | 355 lines, DiscoverRelatedRecords, CountRelatedRecords, CalculateCompleteness, SuggestSurvivor |
| `backend/internal/service/merge.go` | Atomic merge execution and undo | ✓ VERIFIED | 534 lines, ExecuteMerge with transaction, UndoMerge validates expiry/consumed |
| `backend/internal/entity/audit.go` | RECORD_MERGE and MERGE_UNDO audit events | ✓ VERIFIED | Lines 44-45: AuditEventRecordMerge, AuditEventMergeUndo |
| `backend/internal/service/audit.go` | LogRecordMerge and LogMergeUndo methods | ✓ VERIFIED | Lines 314-345: LogRecordMerge and LogMergeUndo convenience methods |
| `backend/internal/handler/merge.go` | HTTP handlers for merge preview/execute/undo/history | ⚠️ PARTIAL | 285 lines, 4 handlers exist BUT undo route has typo (line 283) |
| `backend/cmd/api/main.go` | Route registration | ✓ VERIFIED | Lines 138, 151-152, 247, 423: mergeRepo, discoveryService, mergeService, handler initialization and route registration |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| service/merge.go | repo/merge.go | snapshot persistence | ✓ WIRED | Lines 67, 72, 294: s.mergeRepo.EnsureArchiveColumns, EnsureTableExists, CleanupExpired |
| service/merge.go | service/merge_discovery.go | related record discovery | ✓ WIRED | Line 121: s.discoveryService.DiscoverRelatedRecords |
| service/merge.go | service/audit.go | audit logging | ✓ WIRED | Lines 288, 411: go s.auditLogger.LogRecordMerge, LogMergeUndo |
| handler/merge.go | service/merge.go | merge execution | ✓ WIRED | Execute calls mergeService.ExecuteMerge, Undo calls mergeService.UndoMerge |
| handler/merge.go | service/merge_discovery.go | preview completeness | ✓ WIRED | Preview calls discoveryService.CalculateCompleteness, SuggestSurvivor, CountRelatedRecords |
| cmd/api/main.go | handler/merge.go | route registration | ⚠️ PARTIAL | Line 423: mergeHandler.RegisterRoutes(protected) called, but undo route has typo |

### Requirements Coverage

| Requirement | Status | Blocking Issue |
|-------------|--------|----------------|
| MERGE-01: Merge two or more records | ✓ SATISFIED | ExecuteMerge accepts 1 survivor + N duplicates |
| MERGE-02: User selects survivor record | ✓ SATISFIED | MergeRequest has survivorId field |
| MERGE-03: User selects field values via side-by-side UI | ✓ SATISFIED | Backend API supports via mergedFields map, UI is Phase 16 |
| MERGE-04: Transfer all related records | ✓ SATISFIED | DiscoverRelatedRecords + FK transfer loop in ExecuteMerge |
| MERGE-05: Dynamically discover related records | ✓ SATISFIED | Metadata-driven discovery, no hardcoded entity lists |
| MERGE-06: Atomic transaction | ✓ SATISFIED | db.BeginTx, tx.Commit, deferred rollback on error |
| MERGE-07: Audit log with who/when/what | ✓ SATISFIED | LogRecordMerge called with actorID, orgID, survivorID, duplicateIDs |
| MERGE-08: Store pre-merge snapshots | ✓ SATISFIED | MergeSnapshot persists survivor_before, duplicate_snapshots, related_record_fks |
| MERGE-09: Undo within 30 days | ✗ BLOCKED | Logic correct (validates ExpiresAt, ConsumedAt), but route broken |
| MERGE-10: Multi-record merge via pair merges | ✓ SATISFIED | Backend accepts 1 survivor + N dups per call, frontend does sequential |
| MERGE-11: Merge preview with comparison | ✓ SATISFIED | Preview endpoint returns records, scores, survivor suggestion, related counts |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| backend/internal/handler/merge.go | 283 | Route typo: "/undo:snapshotId" missing "/" | 🛑 Blocker | Undo endpoint unreachable, 404 for POST /merge/undo/:snapshotId |

### Gaps Summary

**1 critical gap found:**

The undo route registration has a typo that makes the endpoint unreachable. Line 283 in `backend/internal/handler/merge.go` reads:

```go
merge.Post("/undo:snapshotId", h.Undo)
```

But should be:

```go
merge.Post("/undo/:snapshotId", h.Undo)
```

The missing `/` before `:snapshotId` means Fiber will not parse the route parameter correctly. The route will not match `/merge/undo/0Ms123...` requests.

**All other functionality is fully implemented:**
- Migrations exist and are valid
- Go types compile and match schema
- Repository has all CRUD operations
- Discovery service is metadata-driven (no hardcoded entities)
- Merge service uses atomic transactions with snapshot, FK transfer, survivor update, and duplicate archiving
- Undo validates 30-day window and consumed state
- Audit logging wired and called
- Preview endpoint returns all required data for UI
- Routes registered on protected group (authentication required)

**Impact:** High - undo functionality cannot be tested or used until route is fixed. However, this is a trivial 1-character fix.

---

_Verified: 2026-02-07T23:30:00Z_
_Verifier: Claude (gsd-verifier)_
