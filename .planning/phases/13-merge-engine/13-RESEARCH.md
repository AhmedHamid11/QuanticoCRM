# Phase 13: Merge Engine - Research

**Researched:** 2026-02-07
**Domain:** Database merge operations, atomic transactions, snapshot-based undo
**Confidence:** HIGH

## Summary

The Merge Engine phase requires implementing atomic multi-table operations for merging duplicate records in a Go/Fiber backend with SQLite/Turso databases. Research focused on SQLite transaction patterns, Go transaction best practices, snapshot storage for undo capability, soft-delete/archive patterns, and SvelteKit UI patterns for side-by-side comparison.

**Key Findings:**
- SQLite provides ACID-compliant transactions that guarantee atomicity across multiple table updates within a single database
- Go's `database/sql` package with deferred rollback pattern ensures safe transaction management
- JSON column storage is ideal for merge snapshots and audit logs (already implemented in existing audit system)
- Archive table pattern is superior to soft-delete flags for merge scenarios with undo requirements
- Existing codebase has strong patterns for transactions (`bulk.go`), audit logging (`audit.go`), and multi-tenant database routing

**Primary recommendation:** Use SQLite transactions with deferred rollback, store pre-merge snapshots as JSON in an audit table, implement archive pattern with `archived_at` timestamp and `survivor_id` reference, discover related records dynamically via entity metadata lookup fields.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Survivor selection:**
- System auto-suggests the most complete record as survivor (most filled fields)
- User can override the suggestion via radio button / explicit selection
- Completeness score should be visible so the suggestion is transparent

**Field value resolution:**
- All fields default to the survivor's values
- User overrides individual fields by clicking the duplicate's value
- If survivor field is empty and duplicate has data, auto-fill from duplicate
- Auto-filled fields are visually highlighted so the user sees what was auto-selected
- User can still override any auto-filled value

**Multi-record merge (3+ records):**
- Pair-by-pair merging, not all-at-once
- User merges two records first, then merges the result with the next duplicate
- Simpler UI, avoids complex multi-column comparison

**Related record transfer:**
- All related records (Tasks, Notes, Activities, lookup references) transfer to survivor automatically
- No cherry-picking — everything transfers
- If both survivor and duplicate have the same related record linked, keep both (no deduplication of related records)

**Merge preview:**
- Show related record counts per entity type (e.g., "5 Tasks, 3 Notes will transfer")
- Expandable list of actual related records under each count
- Preview displays before user confirms the merge

**Duplicate record fate:**
- After merge, duplicate record is archived with a reference to the survivor
- Archived records are hidden from normal UI (list views, search)
- Archive state supports the 30-day undo requirement

### Claude's Discretion
- Merge confirmation UI layout and flow
- Exact visual treatment of auto-filled field highlights
- Audit log schema and storage approach
- Undo implementation mechanics (snapshot vs reverse operations)
- Data loss warning specifics and thresholds
- Atomic transaction strategy

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go `database/sql` | stdlib | Transaction management | Official Go database interface, production-ready |
| SQLite/Turso | SQLite 3.x | Database engine | Already in use, supports ACID transactions |
| Go Fiber | v2.x | HTTP framework | Already in use, provides request context |
| JSON functions | SQLite built-in | Snapshot storage | Native JSON support, no external dependencies |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/fastcrm/backend/internal/db` | internal | Multi-tenant DB routing | Already implemented for org-specific databases |
| `github.com/fastcrm/backend/internal/entity` | internal | Audit log entities | Already has hash-chain audit logging |
| `github.com/fastcrm/backend/internal/repo` | internal | Repository pattern | Consistent data access layer |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Archive table | Soft-delete flag | Archive table better for undo (full snapshot), soft-delete simpler but harder to restore |
| JSON snapshots | Separate history tables | JSON more flexible, history tables more queryable but complex schema |
| Deferred rollback | Explicit error handling | Deferred rollback safer (automatic cleanup on panic), explicit gives more control |

**Installation:**
No new dependencies required — all capabilities available in current stack.

## Architecture Patterns

### Recommended Project Structure
```
backend/internal/
├── entity/
│   ├── merge.go              # Merge entities (MergeOperation, MergeSnapshot, MergeAudit)
│   └── audit.go              # Existing audit log (extend for merge events)
├── repo/
│   ├── merge.go              # Merge repository (CRUD for merge operations)
│   └── metadata.go           # Existing metadata repo (used for field discovery)
├── service/
│   ├── merge.go              # Merge orchestration service
│   └── related_record.go     # Related record discovery and transfer
├── handler/
│   └── merge.go              # HTTP handlers for merge API
└── migrations/
    └── 0XX_merge_tables.sql  # Merge snapshots, audit, archive tables
```

### Pattern 1: Deferred Rollback Transaction
**What:** Go's idiomatic pattern for safe transaction management
**When to use:** All merge operations (atomic requirement)
**Example:**
```go
// Source: https://go.dev/doc/database/execute-transactions
func (s *MergeService) ExecuteMerge(ctx context.Context, survivorID, duplicateID string) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    // Deferred rollback - no-op if commit succeeds, safety net if error occurs
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r) // re-throw
        }
    }()

    // 1. Create snapshot
    if err := s.createSnapshot(ctx, tx, survivorID, duplicateID); err != nil {
        tx.Rollback()
        return err
    }

    // 2. Transfer related records
    if err := s.transferRelatedRecords(ctx, tx, survivorID, duplicateID); err != nil {
        tx.Rollback()
        return err
    }

    // 3. Update survivor fields
    if err := s.updateSurvivorFields(ctx, tx, survivorID, mergedFields); err != nil {
        tx.Rollback()
        return err
    }

    // 4. Archive duplicate
    if err := s.archiveDuplicate(ctx, tx, duplicateID, survivorID); err != nil {
        tx.Rollback()
        return err
    }

    // 5. Create audit log
    if err := s.createAuditLog(ctx, tx, mergeDetails); err != nil {
        tx.Rollback()
        return err
    }

    // Commit - if this succeeds, deferred rollback is a no-op
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}
```

### Pattern 2: JSON Snapshot Storage
**What:** Store pre-merge state as JSON for undo capability
**When to use:** Before every merge operation
**Example:**
```go
// Source: https://til.simonwillison.net/sqlite/json-audit-log
type MergeSnapshot struct {
    ID                string    `json:"id" db:"id"`
    OrgID             string    `json:"orgId" db:"org_id"`
    SurvivorID        string    `json:"survivorId" db:"survivor_id"`
    SurvivorBefore    string    `json:"survivorBefore" db:"survivor_before"`    // JSON snapshot
    DuplicateIDs      string    `json:"duplicateIds" db:"duplicate_ids"`        // JSON array
    DuplicateSnapshots string   `json:"duplicateSnapshots" db:"duplicate_snapshots"` // JSON array of objects
    RelatedRecordFKs  string    `json:"relatedRecordFks" db:"related_record_fks"`   // JSON map of FK changes
    CreatedAt         time.Time `json:"createdAt" db:"created_at"`
    ExpiresAt         time.Time `json:"expiresAt" db:"expires_at"`              // 30 days from creation
}

// In migration:
CREATE TABLE merge_snapshots (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    survivor_id TEXT NOT NULL,
    survivor_before TEXT NOT NULL,          -- JSON: full record state before merge
    duplicate_ids TEXT NOT NULL,            -- JSON: ["dup1", "dup2"]
    duplicate_snapshots TEXT NOT NULL,      -- JSON: [{"id": "dup1", "data": {...}}, ...]
    related_record_fks TEXT NOT NULL,       -- JSON: {"tasks": [{"id": "t1", "old_fk": "dup1"}], ...}
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    INDEX idx_merge_snapshots_survivor (org_id, survivor_id),
    INDEX idx_merge_snapshots_expires (expires_at)
);
```

### Pattern 3: Archive Pattern (Not Soft-Delete)
**What:** Move merged records to archived state with survivor reference
**When to use:** After successful merge (supports undo requirement)
**Example:**
```sql
-- Add to entity tables (contacts, accounts, etc.)
ALTER TABLE contacts ADD COLUMN archived_at TEXT DEFAULT NULL;
ALTER TABLE contacts ADD COLUMN archived_reason TEXT DEFAULT NULL;  -- 'MERGED'
ALTER TABLE contacts ADD COLUMN survivor_id TEXT DEFAULT NULL;      -- Reference to survivor

-- Archive query (in transaction)
UPDATE contacts
SET archived_at = ?,
    archived_reason = 'MERGED',
    survivor_id = ?
WHERE id = ? AND org_id = ?;

-- List queries exclude archived by default
SELECT * FROM contacts WHERE org_id = ? AND archived_at IS NULL;

-- Undo restores from archive
UPDATE contacts
SET archived_at = NULL,
    archived_reason = NULL,
    survivor_id = NULL
WHERE id = ? AND org_id = ?;
```

**Why archive pattern over soft-delete:** Archive preserves the full record for 30-day undo window without interfering with active data queries. Soft-delete flag (`deleted = 1`) would require every query to filter it, and doesn't capture merge-specific metadata (survivor_id).

### Pattern 4: Dynamic Related Record Discovery
**What:** Discover related records by scanning entity metadata for lookup fields
**When to use:** During merge preview and transfer operations
**Example:**
```go
// Discover which entities have lookup fields pointing to the entity being merged
func (s *MergeService) DiscoverRelatedRecords(ctx context.Context, orgID, entityType, recordID string) (map[string][]RelatedRecord, error) {
    // 1. Get all entities in the org
    entities, err := s.metadataRepo.ListEntities(ctx, orgID)
    if err != nil {
        return nil, err
    }

    relatedRecords := make(map[string][]RelatedRecord)

    // 2. For each entity, check if it has lookup fields pointing to this entity type
    for _, entity := range entities {
        fields, err := s.metadataRepo.ListFields(ctx, orgID, entity.Name)
        if err != nil {
            continue
        }

        // 3. Find lookup fields targeting this entity type
        for _, field := range fields {
            if field.Type == "link" && field.TargetEntity == entityType {
                // 4. Query for records with this FK
                records, err := s.queryRelatedRecords(ctx, entity.Name, field.Name, recordID, orgID)
                if err != nil {
                    continue
                }
                relatedRecords[entity.Name] = append(relatedRecords[entity.Name], records...)
            }
        }
    }

    return relatedRecords, nil
}

// Transfer FK references from duplicate to survivor
func (s *MergeService) TransferRelatedRecords(ctx context.Context, tx *sql.Tx, relatedMap map[string][]RelatedRecord, duplicateID, survivorID string) error {
    for entityType, records := range relatedMap {
        tableName := util.GetTableName(entityType)

        for _, record := range records {
            // Update FK to point to survivor
            query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE id = ? AND org_id = ?", tableName, record.FKField)
            _, err := tx.ExecContext(ctx, query, survivorID, record.ID, record.OrgID)
            if err != nil {
                return fmt.Errorf("failed to transfer %s record %s: %w", entityType, record.ID, err)
            }
        }
    }
    return nil
}
```

### Pattern 5: Fiber Context for Transactions
**What:** Pass transaction context through Fiber request lifecycle
**When to use:** Multi-step merge operations spanning HTTP request
**Example:**
```go
// Source: https://docs.gofiber.io/next/guide/go-context/
// Handler passes Fiber context to service
func (h *MergeHandler) ExecuteMerge(c *fiber.Ctx) error {
    orgID := c.Locals("orgID").(string)
    userID := c.Locals("userID").(string)

    var req MergeRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": err.Error()})
    }

    // Use c.Context() for database operations (respects deadlines, cancellation)
    result, err := h.mergeService.ExecuteMerge(c.Context(), orgID, userID, req)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.JSON(result)
}
```

### Anti-Patterns to Avoid
- **Don't use WAL mode for multi-database transactions:** SQLite WAL mode doesn't guarantee atomicity across multiple attached databases. Since this project uses one database per org, transactions are within a single DB and WAL is safe.
- **Don't call non-transaction methods inside transaction:** Avoid mixing `db.ExecContext()` and `tx.ExecContext()` — stick to transaction handle inside transaction block.
- **Don't skip context deadlines:** Always pass `ctx` to database operations for timeout/cancellation support.
- **Don't store transaction handle in struct fields:** Transactions are request-scoped, pass as function parameters.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Hash-chain audit logging | Custom audit append logic | Existing `internal/entity/audit.go` pattern | Already implements SHA-256 hash chain with tamper detection |
| Multi-tenant DB routing | Manual DB connection switching | Existing `middleware.GetTenantDB()` | Already routes requests to org-specific Turso databases |
| Field metadata discovery | Hardcoded entity schemas | Existing `MetadataRepo.ListFields()` | Already supports dynamic field definitions per org |
| Transaction retry logic | Manual retry loops | Existing `internal/db` DBConn interface | Already has retry-enabled connections for Turso |
| JSON serialization | String concatenation | Go `encoding/json` stdlib | Type-safe, handles escaping, supports nested objects |
| Optimistic UI updates | Custom state management | SvelteKit's `use:enhance` + optimistikit library | Handles rollback, pending states, race conditions |

**Key insight:** The codebase already has production-ready patterns for transactions (`bulk.go` batching), audit logging (`audit.go` hash chains), and multi-tenant routing. Extend these patterns rather than creating new ones.

## Common Pitfalls

### Pitfall 1: Incomplete Related Record Discovery
**What goes wrong:** Hard-coding related entity types (e.g., "only check Tasks and Notes") misses custom entities or future additions
**Why it happens:** Trying to optimize by avoiding metadata queries
**How to avoid:** Always query entity metadata to discover lookup fields dynamically. Cache entity/field definitions per org to reduce query overhead.
**Warning signs:** Merge completes but leaves orphaned records pointing to archived duplicate

### Pitfall 2: Foreign Key Constraint Violations
**What goes wrong:** SQLite foreign key constraints block archiving the duplicate record if related records still reference it
**Why it happens:** Transferring FKs after archiving instead of before
**How to avoid:**
1. Transfer all FK references first (UPDATE related records)
2. Archive duplicate last (UPDATE duplicate SET archived_at = ...)
3. Verify FK transfer in transaction before committing
**Warning signs:** Transaction rollback with "FOREIGN KEY constraint failed" error

### Pitfall 3: Snapshot Too Large for JSON Column
**What goes wrong:** Storing complete record snapshots for records with large custom_fields or blob data exceeds practical JSON column limits
**Why it happens:** Assuming all record data fits in a JSON text column
**How to avoid:**
- For merge snapshots, store only fields that changed (delta approach)
- For undo, store pre-merge survivor state separately from duplicate snapshots
- Consider 1MB JSON column practical limit (SQLite TEXT max is 2GB but impractical)
**Warning signs:** Slow merge operations, database bloat, query timeouts on undo operations

### Pitfall 4: Undo Window Cleanup Not Automated
**What goes wrong:** Merge snapshots accumulate indefinitely, bloating database
**Why it happens:** Creating snapshots with `expires_at` but no cleanup job
**How to avoid:**
- Add daily cron job to delete expired snapshots: `DELETE FROM merge_snapshots WHERE expires_at < datetime('now')`
- Or cleanup on-demand before new merges: "While processing merge, delete 10 expired snapshots"
**Warning signs:** Database size grows continuously, snapshot table has millions of rows

### Pitfall 5: Race Condition on Multi-Record Merge
**What goes wrong:** User initiates 3-record pair merge (A+B, then result+C), but concurrent process modifies record C between merges
**Why it happens:** Multi-record merge UI doesn't lock records during pair-by-pair flow
**How to avoid:**
- Fetch fresh snapshot before each pair merge
- Validate record hasn't changed since preview (compare `modified_at` timestamp)
- Return error if record was modified, force user to refresh preview
**Warning signs:** User complains merged data doesn't match preview, unexpected field values in final survivor

### Pitfall 6: Forgetting to Update Denormalized Fields
**What goes wrong:** Survivor has `account_name` field denormalized from Account lookup, but merge doesn't update it when `account_id` changes
**Why it happens:** Focusing on FK references, forgetting denormalized copies
**How to avoid:**
- After updating survivor fields, re-run denormalization logic (fetch related record names)
- Or include denormalized fields in merge field selection UI (user sees both `account_id` and `account_name`)
**Warning signs:** Survivor shows correct ID but wrong name in list views

### Pitfall 7: Audit Log Missing User Context
**What goes wrong:** Merge audit log records system action but not which user initiated it
**Why it happens:** Passing empty `userID` to audit service
**How to avoid:** Always extract `userID` from Fiber context `c.Locals("userID")` and include in audit event
**Warning signs:** Audit log shows `actor_id = ""` for merge operations, can't trace who merged what

## Code Examples

Verified patterns from official sources:

### Atomic Merge Transaction
```go
// Based on https://go.dev/doc/database/execute-transactions
func (s *MergeService) ExecuteMerge(ctx context.Context, orgID, userID, survivorID string, duplicateIDs []string, mergedFields map[string]interface{}) (*MergeResult, error) {
    db := s.getOrgDB(orgID) // Multi-tenant DB routing

    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to begin transaction: %w", err)
    }

    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
    }()

    // Step 1: Create snapshot (for undo)
    snapshot := &MergeSnapshot{
        ID:         sfid.NewMergeSnapshot(),
        OrgID:      orgID,
        SurvivorID: survivorID,
        CreatedAt:  time.Now().UTC(),
        ExpiresAt:  time.Now().UTC().Add(30 * 24 * time.Hour), // 30 days
    }

    // Capture survivor pre-merge state
    survivorBefore, err := s.fetchRecordSnapshot(ctx, tx, survivorID, orgID)
    if err != nil {
        tx.Rollback()
        return nil, err
    }
    snapshot.SurvivorBefore = survivorBefore

    // Capture duplicate snapshots
    duplicateSnapshots := []map[string]interface{}{}
    for _, dupID := range duplicateIDs {
        dupSnapshot, err := s.fetchRecordSnapshot(ctx, tx, dupID, orgID)
        if err != nil {
            tx.Rollback()
            return nil, err
        }
        duplicateSnapshots = append(duplicateSnapshots, dupSnapshot)
    }
    snapshot.DuplicateSnapshots = toJSON(duplicateSnapshots)

    // Step 2: Discover and transfer related records
    relatedRecordMap := make(map[string]interface{})
    for _, dupID := range duplicateIDs {
        relatedRecords, err := s.discoverRelatedRecords(ctx, tx, orgID, dupID)
        if err != nil {
            tx.Rollback()
            return nil, err
        }

        fkChanges, err := s.transferRelatedRecords(ctx, tx, relatedRecords, dupID, survivorID, orgID)
        if err != nil {
            tx.Rollback()
            return nil, err
        }
        relatedRecordMap[dupID] = fkChanges
    }
    snapshot.RelatedRecordFKs = toJSON(relatedRecordMap)

    // Step 3: Update survivor with merged field values
    if err := s.updateSurvivorFields(ctx, tx, survivorID, orgID, mergedFields); err != nil {
        tx.Rollback()
        return nil, err
    }

    // Step 4: Archive duplicates (AFTER FK transfer)
    for _, dupID := range duplicateIDs {
        if err := s.archiveDuplicate(ctx, tx, dupID, survivorID, orgID); err != nil {
            tx.Rollback()
            return nil, err
        }
    }

    // Step 5: Save snapshot
    if err := s.saveSnapshot(ctx, tx, snapshot); err != nil {
        tx.Rollback()
        return nil, err
    }

    // Step 6: Create audit log entry
    auditEvent := &entity.AuditEvent{
        Timestamp:  time.Now().UTC(),
        EventType:  "RECORD_MERGE",
        ActorID:    userID,
        TargetID:   survivorID,
        OrgID:      orgID,
        Details: map[string]interface{}{
            "survivor_id":   survivorID,
            "duplicate_ids": duplicateIDs,
            "snapshot_id":   snapshot.ID,
        },
        Success: true,
    }
    if err := s.auditRepo.Create(ctx, toAuditLogEntry(auditEvent)); err != nil {
        tx.Rollback()
        return nil, err
    }

    // Commit all changes atomically
    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("failed to commit merge: %w", err)
    }

    return &MergeResult{
        SurvivorID: survivorID,
        SnapshotID: snapshot.ID,
    }, nil
}
```

### Related Record Discovery via Metadata
```go
// Discover all records with FK references to a specific record
func (s *MergeService) discoverRelatedRecords(ctx context.Context, tx *sql.Tx, orgID, recordID string) ([]RelatedRecordGroup, error) {
    // Get current entity type from record ID prefix
    entityType := getEntityTypeFromID(recordID) // e.g., "con_..." -> "Contact"

    // Query all entities in this org
    entities, err := s.metadataRepo.ListEntities(ctx, orgID)
    if err != nil {
        return nil, err
    }

    var groups []RelatedRecordGroup

    for _, entity := range entities {
        fields, err := s.metadataRepo.ListFields(ctx, orgID, entity.Name)
        if err != nil {
            continue
        }

        // Find lookup fields targeting our entity type
        for _, field := range fields {
            if field.Type != "link" || field.TargetEntity != entityType {
                continue
            }

            // Query records with this FK
            tableName := util.GetTableName(entity.Name)
            query := fmt.Sprintf(`
                SELECT id, %s as fk_value
                FROM %s
                WHERE org_id = ? AND %s = ? AND archived_at IS NULL
            `, field.Name, tableName, field.Name)

            rows, err := tx.QueryContext(ctx, query, orgID, recordID)
            if err != nil {
                continue
            }
            defer rows.Close()

            var records []RelatedRecord
            for rows.Next() {
                var rec RelatedRecord
                if err := rows.Scan(&rec.ID, &rec.FKValue); err != nil {
                    continue
                }
                rec.EntityType = entity.Name
                rec.FKField = field.Name
                records = append(records, rec)
            }

            if len(records) > 0 {
                groups = append(groups, RelatedRecordGroup{
                    EntityType: entity.Name,
                    FKField:    field.Name,
                    Records:    records,
                })
            }
        }
    }

    return groups, nil
}
```

### Undo Merge Operation
```go
// Undo merge by restoring from snapshot (within 30-day window)
func (s *MergeService) UndoMerge(ctx context.Context, orgID, userID, snapshotID string) error {
    db := s.getOrgDB(orgID)

    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin undo transaction: %w", err)
    }
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
    }()

    // Step 1: Fetch snapshot (verify not expired)
    snapshot, err := s.fetchSnapshot(ctx, tx, snapshotID, orgID)
    if err != nil {
        tx.Rollback()
        return err
    }

    if time.Now().UTC().After(snapshot.ExpiresAt) {
        tx.Rollback()
        return fmt.Errorf("snapshot expired, cannot undo merge after 30 days")
    }

    // Step 2: Restore survivor to pre-merge state
    survivorBefore := fromJSON(snapshot.SurvivorBefore)
    if err := s.restoreRecordFromSnapshot(ctx, tx, snapshot.SurvivorID, orgID, survivorBefore); err != nil {
        tx.Rollback()
        return err
    }

    // Step 3: Un-archive duplicates
    duplicateIDs := fromJSONArray(snapshot.DuplicateIDs)
    duplicateSnapshots := fromJSONArray(snapshot.DuplicateSnapshots)
    for i, dupID := range duplicateIDs {
        if err := s.unarchiveDuplicate(ctx, tx, dupID, orgID); err != nil {
            tx.Rollback()
            return err
        }
        // Restore duplicate to pre-merge state
        if err := s.restoreRecordFromSnapshot(ctx, tx, dupID, orgID, duplicateSnapshots[i]); err != nil {
            tx.Rollback()
            return err
        }
    }

    // Step 4: Restore FK references (point related records back to duplicates)
    relatedFKs := fromJSON(snapshot.RelatedRecordFKs)
    for dupID, fkChanges := range relatedFKs {
        for entityType, records := range fkChanges {
            tableName := util.GetTableName(entityType)
            for _, record := range records {
                // Revert FK back to duplicate
                query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE id = ? AND org_id = ?",
                    tableName, record.FKField)
                _, err := tx.ExecContext(ctx, query, dupID, record.RecordID, orgID)
                if err != nil {
                    tx.Rollback()
                    return err
                }
            }
        }
    }

    // Step 5: Mark snapshot as consumed (prevent double-undo)
    _, err = tx.ExecContext(ctx,
        "UPDATE merge_snapshots SET consumed_at = ? WHERE id = ? AND org_id = ?",
        time.Now().UTC().Format(time.RFC3339), snapshotID, orgID)
    if err != nil {
        tx.Rollback()
        return err
    }

    // Step 6: Audit log for undo
    auditEvent := &entity.AuditEvent{
        Timestamp: time.Now().UTC(),
        EventType: "MERGE_UNDO",
        ActorID:   userID,
        OrgID:     orgID,
        Details: map[string]interface{}{
            "snapshot_id": snapshotID,
            "survivor_id": snapshot.SurvivorID,
        },
        Success: true,
    }
    if err := s.auditRepo.Create(ctx, toAuditLogEntry(auditEvent)); err != nil {
        tx.Rollback()
        return err
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit undo: %w", err)
    }

    return nil
}
```

### SvelteKit Side-by-Side Comparison UI
```svelte
<script>
  // Source: SvelteKit $state and use:enhance patterns
  import { enhance } from '$app/forms';

  let survivor = $state(data.survivor);
  let duplicate = $state(data.duplicate);
  let mergedFields = $state({});

  // Auto-suggest survivor (most complete record)
  $effect(() => {
    const survivorScore = calculateCompleteness(survivor);
    const duplicateScore = calculateCompleteness(duplicate);

    if (duplicateScore > survivorScore) {
      // Swap suggestion
      suggestedSurvivor = duplicate;
      suggestedDuplicate = survivor;
    }
  });

  // Auto-fill empty fields from duplicate
  $effect(() => {
    for (const [field, value] of Object.entries(survivor)) {
      if (!value && duplicate[field]) {
        mergedFields[field] = {
          value: duplicate[field],
          source: 'duplicate',
          autoFilled: true
        };
      } else {
        mergedFields[field] = {
          value: value,
          source: 'survivor',
          autoFilled: false
        };
      }
    }
  });

  function selectField(fieldName, source) {
    const value = source === 'survivor' ? survivor[fieldName] : duplicate[fieldName];
    mergedFields[fieldName] = {
      value,
      source,
      autoFilled: false
    };
  }

  function calculateCompleteness(record) {
    let filled = 0;
    let total = 0;
    for (const [key, value] of Object.entries(record)) {
      if (key !== 'id' && key !== 'org_id') {
        total++;
        if (value && value !== '') filled++;
      }
    }
    return total > 0 ? (filled / total) : 0;
  }
</script>

<div class="merge-comparison">
  <div class="survivor-selection">
    <h3>Select Survivor Record</h3>
    <div class="radio-group">
      <label>
        <input type="radio" name="survivor" value={survivor.id} checked />
        {survivor.name} (Completeness: {Math.round(calculateCompleteness(survivor) * 100)}%)
      </label>
      <label>
        <input type="radio" name="survivor" value={duplicate.id} />
        {duplicate.name} (Completeness: {Math.round(calculateCompleteness(duplicate) * 100)}%)
      </label>
    </div>
  </div>

  <div class="field-comparison">
    <h3>Select Field Values</h3>
    {#each Object.keys(survivor) as field}
      {#if field !== 'id' && field !== 'org_id'}
        <div class="field-row">
          <div class="field-name">{field}</div>
          <button
            class="field-value {mergedFields[field]?.source === 'survivor' ? 'selected' : ''}"
            class:auto-filled={mergedFields[field]?.autoFilled && mergedFields[field]?.source === 'survivor'}
            onclick={() => selectField(field, 'survivor')}
          >
            {survivor[field] || '(empty)'}
          </button>
          <button
            class="field-value {mergedFields[field]?.source === 'duplicate' ? 'selected' : ''}"
            class:auto-filled={mergedFields[field]?.autoFilled && mergedFields[field]?.source === 'duplicate'}
            onclick={() => selectField(field, 'duplicate')}
          >
            {duplicate[field] || '(empty)'}
          </button>
        </div>
      {/if}
    {/each}
  </div>

  <div class="related-records-preview">
    <h3>Related Records to Transfer</h3>
    {#each relatedRecordCounts as { entityType, count, records }}
      <details>
        <summary>{count} {entityType} will transfer</summary>
        <ul>
          {#each records as record}
            <li>{record.name || record.id}</li>
          {/each}
        </ul>
      </details>
    {/each}
  </div>

  <form method="POST" action="?/merge" use:enhance>
    <input type="hidden" name="survivorId" value={survivor.id} />
    <input type="hidden" name="duplicateId" value={duplicate.id} />
    <input type="hidden" name="mergedFields" value={JSON.stringify(mergedFields)} />
    <button type="submit">Confirm Merge</button>
  </form>
</div>

<style>
  .auto-filled {
    background-color: #fef3c7; /* Yellow highlight for auto-filled fields */
    border: 2px dashed #f59e0b;
  }

  .field-value.selected {
    background-color: #dbeafe;
    border: 2px solid #3b82f6;
  }
</style>
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Soft-delete flag | Archive table pattern | ~2020+ | Better undo support, cleaner active data queries, explicit merge metadata |
| Manual rollback in error handlers | Deferred rollback pattern | Go 1.9+ (2017) | Safer, handles panics, cleaner code |
| Hardcoded entity schemas | Dynamic metadata queries | Modern CRMs (2018+) | Supports custom entities, flexible schema per org |
| String concatenation for JSON | `encoding/json` stdlib | Go 1.0+ | Type-safe, handles escaping, validation |
| Trigger-based cascade updates | Application-level FK management | Modern ORMs | Explicit control, better error handling, audit trail |

**Deprecated/outdated:**
- **Soft-delete flags for merge:** Use archive pattern with `archived_at`, `archived_reason`, `survivor_id` instead
- **SQLite 3.6.19 and earlier:** Missing JSON functions — require SQLite 3.38+ for JSON support (Turso uses modern SQLite)
- **Manual transaction rollback without defer:** Use deferred rollback pattern for safety

## Open Questions

Things that couldn't be fully resolved:

1. **Multi-record merge grouping strategy**
   - What we know: User decision is pair-by-pair merging
   - What's unclear: Should backend track multi-record merge groups for grouped undo, or treat each pair merge independently?
   - Recommendation: Treat pair merges independently for MVP, add merge group tracking in Phase 16 if undo UX requires it

2. **Snapshot size limits**
   - What we know: JSON columns can theoretically hold 2GB in SQLite, but practical limits are lower
   - What's unclear: What's the size threshold where snapshot storage becomes a performance issue?
   - Recommendation: Start with full-record snapshots, monitor snapshot table size, implement delta snapshots if median snapshot size exceeds 100KB

3. **Denormalized field updates**
   - What we know: Some entities have denormalized fields (e.g., `account_name` copy of Account.name)
   - What's unclear: Should merge automatically re-denormalize, or treat denormalized fields as regular user-selectable fields?
   - Recommendation: Treat as regular fields (user selects), avoid automatic denormalization complexity in merge logic

4. **Concurrent merge prevention**
   - What we know: Two users could attempt to merge the same record simultaneously
   - What's unclear: Should backend use row-level locks, optimistic locking (version field), or allow last-write-wins?
   - Recommendation: Use optimistic locking — check `modified_at` hasn't changed since preview fetch, return error if stale

## Sources

### Primary (HIGH confidence)
- [SQLite Atomic Commit Documentation](https://sqlite.org/atomiccommit.html) - Official SQLite transaction guarantees
- [SQLite Transactional Behavior](https://www.sqlite.org/transactional.html) - ACID compliance verification
- [Go Official: Executing Transactions](https://go.dev/doc/database/execute-transactions) - Deferred rollback pattern
- [Simon Willison: Tracking SQLite table history using a JSON audit log](https://til.simonwillison.net/sqlite/json-audit-log) - JSON snapshot pattern
- [SQLite Foreign Key Support](https://sqlite.org/foreignkeys.html) - CASCADE update/delete behavior
- [Go Fiber Context Documentation](https://docs.gofiber.io/next/guide/go-context/) - Context lifecycle management

### Secondary (MEDIUM confidence)
- [Three Dots Labs: Database Transactions in Go with Layered Architecture](https://threedots.tech/post/database-transactions-in-go/) - Transaction patterns in service layer
- [GeeksforGeeks: When to Use ON UPDATE CASCADE in SQLite](https://www.geeksforgeeks.org/sqlite/when-to-use-on-update-cascade-in-sqlite/) - FK cascade patterns
- [Medium: Handling Database Transactions in Go with Rollback and Commit](https://medium.com/@cosmicray001/handling-database-transactions-in-go-with-rollback-and-commit-e35c1830b825) - Best practices verified
- [Cultured Systems: Avoiding the soft delete anti-pattern](https://www.cultured.systems/2024/04/24/Soft-delete/) - Archive pattern vs soft-delete
- [Insycle: Deduplication Best Practices](https://support.insycle.com/hc/en-us/articles/6584810088855-Deduplication-Best-Practices) - CRM merge patterns
- [GitHub: optimistikit](https://github.com/paoloricciuti/optimistikit) - SvelteKit optimistic UI library

### Tertiary (LOW confidence - WebSearch only)
- [Data Ladder: Data Merging Process, Challenges & Best Practices](https://dataladder.com/merging-data-from-multiple-sources/) - General merge guidance
- [Medium: Solving Duplicate Data Problems in Large Databases Using SQL](https://medium.com/learning-sql/solving-duplicate-data-problems-in-large-databases-using-sql-da6af47df2d1) - SQL deduplication patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Go/SQLite/Fiber already in use, verified patterns from official docs
- Architecture: HIGH - Transaction patterns from Go official docs, JSON audit from Simon Willison (SQLite expert), existing codebase patterns confirmed
- Pitfalls: MEDIUM - Based on SQLite FK behavior (official), transaction pitfalls (Go docs), and general database merge experience (industry best practices)

**Research date:** 2026-02-07
**Valid until:** 2026-04-07 (60 days - stable domain, Go stdlib and SQLite patterns don't change rapidly)
