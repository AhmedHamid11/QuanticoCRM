---
phase: 13-merge-engine
plan: 02
subsystem: merge-persistence
tags: [merge, repository, discovery, metadata, multi-tenant]
requires:
  - "13-01: Merge entity types and SFID prefix"
provides:
  - "Merge snapshot CRUD operations"
  - "Dynamic related record discovery via metadata"
  - "Completeness scoring for survivor suggestion"
affects:
  - "13-03: Merge execution service will use MergeRepo for snapshot persistence"
  - "13-04: Merge API handlers will use discovery service for preview"
tech-stack:
  added: []
  patterns:
    - "Repository pattern with db.DBConn interface"
    - "Metadata-driven dynamic discovery (no hardcoded entity lists)"
    - "Defensive table existence checking"
key-files:
  created:
    - backend/internal/repo/merge.go
    - backend/internal/service/merge_discovery.go
  modified: []
decisions:
  - id: archive-columns-defensive
    summary: "EnsureArchiveColumns checks and adds archived_at column dynamically"
    rationale: "Entity tables are created per-org and custom entities exist, not all will have archived_at initially"
    alternatives: "Require migration - rejected due to dynamic entity creation"
  - id: metadata-driven-discovery
    summary: "Related record discovery scans all entities and fields via metadata repo"
    rationale: "No hardcoded entity lists means new entities automatically work, supports custom entities"
    alternatives: "Hardcode standard entities - rejected as not extensible"
  - id: completeness-simple-ratio
    summary: "Completeness = filled fields / total fields (0.0-1.0)"
    rationale: "Simple, transparent scoring that users can understand"
    alternatives: "Weighted scoring - rejected as too opaque"
metrics:
  duration: "2.3 minutes"
  completed: 2026-02-07
---

# Phase 13 Plan 02: Merge Persistence & Discovery Summary

**One-liner:** Merge snapshot repository for undo capability and metadata-driven related record discovery.

## What Was Built

### 1. Merge Snapshot Repository (`repo/merge.go`)
- **MergeRepo** with full CRUD operations:
  - `Save()`: Create new merge snapshot
  - `GetByID()`: Fetch snapshot by ID
  - `GetBySurvivor()`: Fetch all snapshots for a survivor record
  - `ListByOrg()`: Paginated list of all org snapshots
  - `MarkConsumed()`: Mark snapshot as used for undo
  - `CleanupExpired()`: Delete snapshots past 30-day window
- **EnsureTableExists()**: Defensive table creation with indexes
- **EnsureArchiveColumns()**: Dynamic column addition for archived_at
- Uses `db.DBConn` interface for multi-tenant database routing
- RFC3339 timestamp handling for all datetime fields

### 2. Merge Discovery Service (`service/merge_discovery.go`)
- **MergeDiscoveryService** with metadata-driven discovery:
  - `DiscoverRelatedRecords()`: Scans all entities for lookup fields pointing to target entity
  - `CountRelatedRecords()`: Counts related records per duplicate for merge preview
  - `CalculateCompleteness()`: Scores records 0.0-1.0 based on filled fields
  - `SuggestSurvivor()`: Recommends most complete record as survivor
- No hardcoded entity lists - fully dynamic via metadata repo
- Defensively handles missing tables and archived_at columns
- Excludes archived records from related record counts

## Key Implementation Details

### Snapshot Persistence Pattern
```go
// Save snapshot with 30-day expiration
snapshot := &entity.MergeSnapshot{
    OrgID:          orgID,
    EntityType:     "Contact",
    SurvivorID:     "con_abc",
    DuplicateIDs:   `["con_123","con_456"]`,
    SurvivorBefore: `{"id":"con_abc","name":"John Doe"}`,
    // ... rest of snapshot data
}
mergeRepo.Save(ctx, snapshot)
```

### Dynamic Discovery Pattern
```go
// Discovers related records by scanning metadata
groups := discoveryService.DiscoverRelatedRecords(ctx, db, orgID, "Contact", recordID)
// Returns: []RelatedRecordGroup with entity type, FK field, and related records
// Example: Opportunities with contactId pointing to this Contact
```

### Completeness Scoring
```go
// Simple ratio: filled fields / total fields
score := discoveryService.CalculateCompleteness(record)
// Returns: 0.0 to 1.0
// Example: record with 8 of 10 fields filled = 0.8
```

## Decisions Made

### 1. Archive Columns Added Dynamically
**Decision:** `EnsureArchiveColumns()` checks PRAGMA table_info and adds missing columns

**Rationale:**
- Entity tables are created dynamically per-org
- Custom entities may not have archived_at column
- Merge needs to exclude archived records from related record discovery
- Migration-based approach doesn't work for dynamic entity creation

**Implementation:**
```go
// Check if archived_at exists, add if missing
rows := db.QueryContext(ctx, "PRAGMA table_info(tableName)")
// ... scan for archived_at column
if !hasArchivedAt {
    db.ExecContext(ctx, "ALTER TABLE tableName ADD COLUMN archived_at TEXT")
}
```

### 2. Metadata-Driven Discovery (No Hardcoded Entities)
**Decision:** Discovery scans all entities and fields via metadata repo

**Rationale:**
- New entities automatically work without code changes
- Supports custom entities created by users
- Single source of truth (metadata) instead of duplicated entity lists

**Trade-offs:**
- Slightly slower than hardcoded lookups (requires metadata queries)
- Acceptable because discovery is preview-time, not real-time operation

### 3. Simple Completeness Scoring
**Decision:** Completeness = (filled fields / total fields)

**Rationale:**
- Transparent and understandable by users
- No hidden weighting that confuses "why is this the survivor?"
- System fields (id, timestamps) excluded from scoring

**Example:**
```
Record A: name, email, phone filled (3/5 business fields) = 0.6
Record B: name, email filled (2/5 business fields) = 0.4
→ Suggest Record A as survivor
```

## Architecture Patterns

### 1. Repository Pattern with DBConn Interface
- All repos accept `db.DBConn` interface (not raw `*sql.DB`)
- Enables tenant database routing via `WithDB(conn)`
- Supports retry logic via TenantDB/TursoDB wrappers

### 2. Defensive Table Existence Checking
- Discovery service checks if table exists before querying
- Prevents errors when entities defined in metadata but table not yet created
- Gracefully skips missing tables instead of failing entire discovery

### 3. Metadata as Single Source of Truth
- Discovery reads entity/field definitions from metadata repo
- No duplicate lists of entities in code
- Extensible to any entity type without code changes

## Testing Approach

### Manual Verification
All tests passed:
1. ✅ `go build ./...` compiles successfully
2. ✅ MergeRepo has all listed methods
3. ✅ Uses db.DBConn interface for multi-tenant support
4. ✅ Discovery service scans metadata dynamically
5. ✅ Completeness scoring returns 0.0-1.0
6. ✅ Survivor suggestion returns ID of most complete record

### Example Usage
```go
// Persist merge snapshot
mergeRepo := repo.NewMergeRepo(db)
snapshot := &entity.MergeSnapshot{...}
err := mergeRepo.Save(ctx, snapshot)

// Later: retrieve for undo
snapshot, err := mergeRepo.GetByID(ctx, orgID, snapshotID)

// Discover related records
discoveryService := service.NewMergeDiscoveryService(metadataRepo)
groups, err := discoveryService.DiscoverRelatedRecords(ctx, db, orgID, "Contact", recordID)

// Suggest survivor
records := []map[string]interface{}{record1, record2}
survivorID := discoveryService.SuggestSurvivor(records)
```

## Next Phase Readiness

### Ready For 13-03 (Merge Execution Service)
✅ **Snapshot persistence:** Save/GetByID/MarkConsumed implemented
✅ **Related record discovery:** DiscoverRelatedRecords returns FK groups
✅ **Archive column handling:** EnsureArchiveColumns utility available
✅ **Multi-tenant support:** db.DBConn interface used throughout

### Blockers
None. All required infrastructure is in place.

### Recommendations
1. **13-03 merge execution** should:
   - Call `EnsureArchiveColumns()` before archiving duplicates
   - Save snapshot before modifying records (undo capability)
   - Use `DiscoverRelatedRecords()` to find all FKs needing update
2. **13-04 merge API** should:
   - Use `CountRelatedRecords()` for preview endpoint
   - Use `SuggestSurvivor()` to recommend survivor in preview
   - Provide completeness scores in preview response

## Deviations from Plan

None - plan executed exactly as written.

## Related Documentation

- Phase 13 Plan 01: Merge entity types (entity/merge.go)
- RESEARCH.md: Merge engine domain research
- CONTEXT.md: Manual merge user experience vision

---

**Commits:**
- c0911a5: feat(13-02): add merge snapshot repository
- 95cf47b: feat(13-02): add merge discovery service

**Duration:** 2.3 minutes
**Status:** Complete ✅

## Self-Check: PASSED

All files created and commits verified:
- ✅ backend/internal/repo/merge.go
- ✅ backend/internal/service/merge_discovery.go
- ✅ Commit c0911a5
- ✅ Commit 95cf47b
