# CRM Record Merge Deep Dive

**Domain:** CRM Deduplication - Record Merging
**Researched:** 2026-02-05
**Confidence Level:** HIGH (verified with official documentation and existing codebase patterns)

---

## Executive Summary

Record merging is the most complex and risky operation in CRM deduplication. This research covers how major CRM platforms handle related record transfer, patterns for discovering lookup relationships dynamically, transaction atomicity requirements, undo/restore mechanisms, and multi-way merge considerations.

The key insight from research: **Merge is fundamentally a three-phase operation: (1) snapshot/backup, (2) field consolidation, (3) relationship reparenting.** The complexity lies not in the merge itself but in ensuring all three phases complete atomically and can be reversed.

---

## 1. Related Record Discovery

### How Major CRMs Handle Related Records

#### Salesforce

Salesforce's native merge operation automatically:
- Reparents all related child records from duplicate to master
- Uses the APEX API for native merges on Contacts, Leads, and Accounts
- For other objects, uses "Synthetic merge" via CRUD API to re-parent related objects

**Key limitation:** REST APIs do not support merge requests - must use APEX.

**Source:** [Salesforce Apex Developer Guide - Merge Records](https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_dml_examples_merge.htm)

#### HubSpot (2025 Update)

As of January 14, 2025, HubSpot's merge creates a NEW record with a new ID:
- All associations from both records appear on the new merged record
- Primary associations are prioritized from the primary merge record
- Secondary record's primary association remains but loses the "Primary" label

**Important change:** Both original record IDs are invalidated - a new ID is created for the merged result.

**Source:** [HubSpot Updated Merge Functionality](https://developers.hubspot.com/changelog/updated-merge-functionality-for-crm-objects-including-contacts-and-companies)

#### Microsoft Dynamics 365

Dynamics 365 merge behavior:
- Master record inherits all related child records from subordinate
- Subordinate record is deactivated (soft delete), not hard deleted
- Two hidden fields track merge: `Merged` (boolean) and `Master ID` (lookup)
- Direct children are reparented; child records of children remain attached to their parents

**Key insight:** Only immediate children are reparented - grandchildren stay with their parent.

**Source:** [Dynamics 365 Merge Duplicate Records](https://learn.microsoft.com/en-us/dynamics365/customerengagement/on-premises/basics/merge-duplicate-records-accounts-contacts-leads?view=op-9-1)

### Quantico CRM: Dynamic Lookup Discovery

Based on the existing codebase, Quantico CRM already has a pattern for discovering lookup relationships via the `DiscoverRelatedLists` function in `related_list.go`:

```go
// Query pattern for discovering all entities that reference a target
query := `SELECT entity_name, name, label, type
          FROM field_defs
          WHERE org_id = ? AND (type = 'link' OR type = 'linkMultiple') AND link_entity = ?
          ORDER BY entity_name, name`
```

**For merge, we need the inverse:** Find all entities that have a lookup pointing to the entity being merged, then update those lookups.

### Recommended Discovery Pattern for Merge

```go
// FindReferencingRecords discovers ALL records that reference a specific record ID
// Returns map of entity_name -> []record_id
func (r *MergeRepo) FindReferencingRecords(ctx context.Context, orgID, entityName, recordID string) (map[string][]string, error) {
    // Step 1: Find all lookup fields pointing to this entity type
    lookupFields, err := r.findLookupFieldsTo(ctx, orgID, entityName)
    if err != nil {
        return nil, err
    }

    result := make(map[string][]string)

    // Step 2: For each lookup field, query the actual table for referencing records
    for _, lf := range lookupFields {
        tableName := util.ToSnakeCase(lf.EntityName)
        columnName := util.ToSnakeCase(lf.FieldName)

        // Handle both regular link and linkMultiple
        var query string
        if lf.Type == "linkMultiple" {
            // For linkMultiple, the column contains JSON array of IDs
            query = fmt.Sprintf(
                `SELECT id FROM "%s" WHERE org_id = ? AND %s LIKE ?`,
                tableName, columnName)
            rows, err := r.db.QueryContext(ctx, query, orgID, "%"+recordID+"%")
        } else {
            // For regular link, direct ID match
            query = fmt.Sprintf(
                `SELECT id FROM "%s" WHERE org_id = ? AND %s = ?`,
                tableName, columnName)
            rows, err := r.db.QueryContext(ctx, query, orgID, recordID)
        }

        // Collect referencing record IDs...
    }

    // Step 3: Handle polymorphic relationships (like Task.parent_type/parent_id)
    polymorphicRefs, err := r.findPolymorphicReferences(ctx, orgID, entityName, recordID)

    return result, nil
}
```

### Edge Cases for Discovery

| Case | Handling |
|------|----------|
| `linkMultiple` fields | JSON array contains ID - need `LIKE '%id%'` or JSON parse |
| Polymorphic refs (Task) | Uses `parent_type + parent_id` - query both columns |
| Self-referencing | Account.parentAccountId -> Account - must not reparent to self |
| Circular references | A -> B -> A - detect cycles before merge |
| Soft-deleted records | Include or exclude based on configuration |

---

## 2. Foreign Key Update Patterns

### Safe Update Strategy

The critical insight: **All FK updates must happen in a single transaction** to maintain referential integrity.

```go
type MergeOperation struct {
    MasterID       string
    SourceIDs      []string  // Records being merged into master
    EntityName     string
    OrgID          string
    FieldMergeRules map[string]MergeRule
}

type MergeRule string
const (
    MergeRuleKeepMaster   MergeRule = "keep_master"     // Always use master's value
    MergeRuleKeepSource   MergeRule = "keep_source"     // Use source if master is empty
    MergeRuleConcat       MergeRule = "concat"          // Append with separator
    MergeRuleSum          MergeRule = "sum"             // Add numeric values
    MergeRuleMostRecent   MergeRule = "most_recent"     // Use most recently modified
    MergeRuleMostComplete MergeRule = "most_complete"   // Use record with most filled fields
)

func (s *MergeService) ExecuteMerge(ctx context.Context, op MergeOperation) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Phase 1: Create snapshot for undo
    snapshot, err := s.createMergeSnapshot(ctx, tx, op)
    if err != nil {
        return fmt.Errorf("failed to create snapshot: %w", err)
    }

    // Phase 2: Merge field values
    err = s.mergeFieldValues(ctx, tx, op)
    if err != nil {
        return fmt.Errorf("failed to merge fields: %w", err)
    }

    // Phase 3: Reparent related records
    err = s.reparentRelatedRecords(ctx, tx, op)
    if err != nil {
        return fmt.Errorf("failed to reparent records: %w", err)
    }

    // Phase 4: Soft-delete source records
    err = s.softDeleteSources(ctx, tx, op)
    if err != nil {
        return fmt.Errorf("failed to delete sources: %w", err)
    }

    // Phase 5: Create audit log entry
    err = s.auditMerge(ctx, tx, op, snapshot)
    if err != nil {
        return fmt.Errorf("failed to audit merge: %w", err)
    }

    return tx.Commit()
}
```

### Reparent Implementation

```go
func (s *MergeService) reparentRelatedRecords(ctx context.Context, tx *sql.Tx, op MergeOperation) error {
    // Find all lookup fields pointing to this entity
    lookups, err := s.metadataRepo.FindLookupsTo(ctx, op.OrgID, op.EntityName)
    if err != nil {
        return err
    }

    for _, lookup := range lookups {
        tableName := util.ToSnakeCase(lookup.EntityName)
        columnName := util.ToSnakeCase(lookup.FieldName)

        if lookup.Type == "linkMultiple" {
            // For linkMultiple, need to update JSON array
            for _, sourceID := range op.SourceIDs {
                query := fmt.Sprintf(`
                    UPDATE "%s"
                    SET %s = REPLACE(%s, ?, ?)
                    WHERE org_id = ? AND %s LIKE ?`,
                    tableName, columnName, columnName, columnName)

                _, err := tx.ExecContext(ctx, query,
                    sourceID, op.MasterID,
                    op.OrgID, "%"+sourceID+"%")
                if err != nil {
                    return fmt.Errorf("failed to update linkMultiple %s.%s: %w",
                        tableName, columnName, err)
                }
            }
        } else {
            // For regular link, simple UPDATE
            placeholders := make([]string, len(op.SourceIDs))
            args := make([]interface{}, 0, len(op.SourceIDs)+2)
            args = append(args, op.MasterID, op.OrgID)

            for i, id := range op.SourceIDs {
                placeholders[i] = "?"
                args = append(args, id)
            }

            query := fmt.Sprintf(`
                UPDATE "%s"
                SET %s = ?
                WHERE org_id = ? AND %s IN (%s)`,
                tableName, columnName, columnName, strings.Join(placeholders, ","))

            _, err := tx.ExecContext(ctx, query, args...)
            if err != nil {
                return fmt.Errorf("failed to update link %s.%s: %w",
                    tableName, columnName, err)
            }
        }
    }

    // Handle polymorphic relationships (Task)
    err = s.reparentPolymorphicRecords(ctx, tx, op)

    return err
}
```

### SQLite-Specific Considerations

Since Quantico uses SQLite/Turso:

1. **No CASCADE UPDATE:** SQLite supports FK constraints but Quantico uses application-level enforcement
2. **JSON in columns:** `linkMultiple` stores JSON arrays - use `REPLACE()` or `json_replace()`
3. **PRAGMA foreign_key_list:** Can introspect FK definitions if needed

```sql
-- Discover foreign keys defined on a table
PRAGMA foreign_key_list('contacts');
-- Returns: id, seq, table, from, to, on_update, on_delete, match
```

---

## 3. Merge Transaction Atomicity

### Requirements for All-or-Nothing Merge

The merge must be atomic across:
1. Snapshot creation (undo data)
2. Field value consolidation
3. All FK updates across all referencing entities
4. Source record deletion/deactivation
5. Audit log entry

### SQLite Transaction Strategy

```go
// SQLite supports SAVEPOINT for nested transactions
func (s *MergeService) ExecuteMergeWithSavepoints(ctx context.Context, op MergeOperation) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Savepoint for snapshot
    _, err = tx.ExecContext(ctx, "SAVEPOINT snapshot_phase")
    if err != nil {
        return err
    }

    snapshot, err := s.createMergeSnapshot(ctx, tx, op)
    if err != nil {
        tx.ExecContext(ctx, "ROLLBACK TO snapshot_phase")
        return fmt.Errorf("snapshot failed: %w", err)
    }

    // Savepoint for reparenting (most likely to fail)
    _, err = tx.ExecContext(ctx, "SAVEPOINT reparent_phase")
    if err != nil {
        return err
    }

    err = s.reparentRelatedRecords(ctx, tx, op)
    if err != nil {
        tx.ExecContext(ctx, "ROLLBACK TO reparent_phase")
        // Try cleanup or alternative approach
        return fmt.Errorf("reparent failed: %w", err)
    }

    return tx.Commit()
}
```

### Handling Partial Failures

| Failure Point | Recovery Action |
|---------------|-----------------|
| Snapshot creation fails | Abort merge, no cleanup needed |
| Field merge fails | Rollback to snapshot point |
| Reparent fails | Rollback to reparent point, log affected records |
| Source deletion fails | Rollback entire transaction |
| Audit log fails | Log to stderr, commit anyway (non-critical) |

### Lock Strategy

For concurrent merge protection:

```go
// Use advisory locking or serialize merges per entity type
type MergeLockManager struct {
    locks sync.Map // entityName+recordID -> *sync.Mutex
}

func (m *MergeLockManager) AcquireMergeLock(entityName string, recordIDs []string) (func(), error) {
    // Sort IDs to prevent deadlock
    sort.Strings(recordIDs)

    var acquired []string
    for _, id := range recordIDs {
        key := entityName + ":" + id
        lock, _ := m.locks.LoadOrStore(key, &sync.Mutex{})

        // Try lock with timeout
        if !tryLockWithTimeout(lock.(*sync.Mutex), 5*time.Second) {
            // Release already acquired locks
            for _, acq := range acquired {
                m.unlock(entityName, acq)
            }
            return nil, fmt.Errorf("merge in progress for %s:%s", entityName, id)
        }
        acquired = append(acquired, id)
    }

    return func() {
        for _, id := range acquired {
            m.unlock(entityName, id)
        }
    }, nil
}
```

---

## 4. Undo Implementation

### Snapshot Storage Schema

```sql
CREATE TABLE merge_snapshots (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    merge_id TEXT NOT NULL,           -- Groups all snapshots for one merge operation
    entity_name TEXT NOT NULL,
    record_id TEXT NOT NULL,
    record_type TEXT NOT NULL,        -- 'source', 'master_before', 'related'
    record_data TEXT NOT NULL,        -- Full JSON of record before merge
    related_refs TEXT,                -- JSON of {entity, field, oldValue, newValue}
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,         -- For retention policy
    restored_at TEXT,                 -- NULL until restored

    UNIQUE(merge_id, entity_name, record_id)
);

CREATE INDEX idx_merge_snapshots_merge ON merge_snapshots(merge_id);
CREATE INDEX idx_merge_snapshots_expires ON merge_snapshots(org_id, expires_at);
CREATE INDEX idx_merge_snapshots_record ON merge_snapshots(org_id, entity_name, record_id);
```

### Snapshot Creation

```go
type MergeSnapshot struct {
    ID          string    `json:"id"`
    OrgID       string    `json:"orgId"`
    MergeID     string    `json:"mergeId"`
    EntityName  string    `json:"entityName"`
    RecordID    string    `json:"recordId"`
    RecordType  string    `json:"recordType"`  // source, master_before, related
    RecordData  string    `json:"recordData"`  // Full JSON
    RelatedRefs string    `json:"relatedRefs"` // JSON of FK changes
    CreatedAt   time.Time `json:"createdAt"`
    ExpiresAt   time.Time `json:"expiresAt"`
    RestoredAt  *time.Time `json:"restoredAt,omitempty"`
}

func (s *MergeService) createMergeSnapshot(ctx context.Context, tx *sql.Tx, op MergeOperation) (*MergeSnapshot, error) {
    mergeID := sfid.NewMerge()

    // 1. Snapshot source records (will be deleted)
    for _, sourceID := range op.SourceIDs {
        record, err := s.getFullRecord(ctx, tx, op.OrgID, op.EntityName, sourceID)
        if err != nil {
            return nil, err
        }

        recordJSON, _ := json.Marshal(record)
        err = s.saveSnapshot(ctx, tx, MergeSnapshot{
            ID:         sfid.NewSnapshot(),
            OrgID:      op.OrgID,
            MergeID:    mergeID,
            EntityName: op.EntityName,
            RecordID:   sourceID,
            RecordType: "source",
            RecordData: string(recordJSON),
            CreatedAt:  time.Now().UTC(),
            ExpiresAt:  time.Now().UTC().Add(30 * 24 * time.Hour), // 30 days
        })
        if err != nil {
            return nil, err
        }
    }

    // 2. Snapshot master record before changes
    masterBefore, err := s.getFullRecord(ctx, tx, op.OrgID, op.EntityName, op.MasterID)
    if err != nil {
        return nil, err
    }
    masterJSON, _ := json.Marshal(masterBefore)
    err = s.saveSnapshot(ctx, tx, MergeSnapshot{
        ID:         sfid.NewSnapshot(),
        OrgID:      op.OrgID,
        MergeID:    mergeID,
        EntityName: op.EntityName,
        RecordID:   op.MasterID,
        RecordType: "master_before",
        RecordData: string(masterJSON),
        CreatedAt:  time.Now().UTC(),
        ExpiresAt:  time.Now().UTC().Add(30 * 24 * time.Hour),
    })
    if err != nil {
        return nil, err
    }

    // 3. Snapshot related record FK changes (for undo reparenting)
    relatedRefs, err := s.captureRelatedReferences(ctx, tx, op)
    if err != nil {
        return nil, err
    }

    for _, ref := range relatedRefs {
        refJSON, _ := json.Marshal(ref)
        err = s.saveSnapshot(ctx, tx, MergeSnapshot{
            ID:         sfid.NewSnapshot(),
            OrgID:      op.OrgID,
            MergeID:    mergeID,
            EntityName: ref.EntityName,
            RecordID:   ref.RecordID,
            RecordType: "related",
            RelatedRefs: string(refJSON),
            CreatedAt:  time.Now().UTC(),
            ExpiresAt:  time.Now().UTC().Add(30 * 24 * time.Hour),
        })
    }

    return &MergeSnapshot{MergeID: mergeID}, nil
}
```

### Restore (Undo) Implementation

```go
func (s *MergeService) UndoMerge(ctx context.Context, orgID, mergeID string) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. Get all snapshots for this merge
    snapshots, err := s.getSnapshotsByMergeID(ctx, orgID, mergeID)
    if err != nil {
        return err
    }

    if len(snapshots) == 0 {
        return fmt.Errorf("merge %s not found or already expired", mergeID)
    }

    // Check if already restored
    for _, s := range snapshots {
        if s.RestoredAt != nil {
            return fmt.Errorf("merge %s already restored at %s", mergeID, s.RestoredAt)
        }
    }

    // 2. Restore related record references first (reverse reparenting)
    for _, snap := range snapshots {
        if snap.RecordType == "related" {
            err = s.restoreRelatedRef(ctx, tx, snap)
            if err != nil {
                return fmt.Errorf("failed to restore related ref: %w", err)
            }
        }
    }

    // 3. Restore master record to pre-merge state
    for _, snap := range snapshots {
        if snap.RecordType == "master_before" {
            err = s.restoreRecord(ctx, tx, snap)
            if err != nil {
                return fmt.Errorf("failed to restore master: %w", err)
            }
        }
    }

    // 4. Restore (undelete) source records
    for _, snap := range snapshots {
        if snap.RecordType == "source" {
            err = s.restoreRecord(ctx, tx, snap)
            if err != nil {
                return fmt.Errorf("failed to restore source: %w", err)
            }
        }
    }

    // 5. Mark snapshots as restored
    now := time.Now().UTC()
    for _, snap := range snapshots {
        _, err = tx.ExecContext(ctx,
            "UPDATE merge_snapshots SET restored_at = ? WHERE id = ?",
            now.Format(time.RFC3339), snap.ID)
        if err != nil {
            return err
        }
    }

    // 6. Audit the undo
    err = s.auditMergeUndo(ctx, tx, orgID, mergeID)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

### Retention Policy

Based on research, common retention periods:

| Platform | Default Retention | Notes |
|----------|-------------------|-------|
| Salesforce | 15 days | Via Recycle Bin |
| Dynamics 365 | Configurable | Audit log based |
| GDPR guidance | 6-12 months | Must be justified |

**Recommendation for Quantico CRM:** 30 days default, configurable per-org

```go
// Cleanup job to run daily
func (s *MergeService) CleanupExpiredSnapshots(ctx context.Context) (int, error) {
    result, err := s.db.ExecContext(ctx,
        `DELETE FROM merge_snapshots
         WHERE expires_at < ? AND restored_at IS NULL`,
        time.Now().UTC().Format(time.RFC3339))
    if err != nil {
        return 0, err
    }

    deleted, _ := result.RowsAffected()
    return int(deleted), nil
}
```

### What Happens to Related Records During Undo

**Critical consideration:** When undoing a merge:

1. **Source records are restored** with their original data
2. **Related records are re-reparented** back to their original parents
3. **Master record is reverted** to pre-merge state
4. **Any changes made AFTER the merge to related records are PRESERVED**

This means if a Contact was reparented from Account A (deleted) to Account B (master), and someone then edited that Contact's phone number, the undo will:
- Change the Contact's accountId back to Account A
- KEEP the new phone number (changes after merge are preserved)

---

## 5. Multi-Way Merge (3+ Records)

### Challenges

Merging 3+ records adds complexity:
1. **Master selection:** Which record becomes the surviving one?
2. **Field consolidation:** How to pick from multiple values?
3. **Ordering:** Does merge order affect result?
4. **Undo complexity:** Must restore N-1 records

### Salesforce Limitation

Native Salesforce merge only supports 2-3 records. Third-party tools like Duplicare allow unlimited records.

**Source:** [Data8 Duplicare](https://www.data-8.co.uk/solutions/duplicare/)

### Recommended Pattern: Iterative Pairwise Merge

```go
// MultiMerge handles 3+ records by iteratively merging pairs
func (s *MergeService) MultiMerge(ctx context.Context, orgID, entityName string, recordIDs []string, masterID string) error {
    if len(recordIDs) < 2 {
        return fmt.Errorf("need at least 2 records to merge")
    }

    // Verify master is in the list
    masterFound := false
    for _, id := range recordIDs {
        if id == masterID {
            masterFound = true
            break
        }
    }
    if !masterFound {
        return fmt.Errorf("master %s not in record list", masterID)
    }

    // Create a parent merge ID to group all sub-merges
    parentMergeID := sfid.NewMerge()

    // Merge each non-master into master one at a time
    for _, sourceID := range recordIDs {
        if sourceID == masterID {
            continue
        }

        op := MergeOperation{
            MasterID:   masterID,
            SourceIDs:  []string{sourceID},
            EntityName: entityName,
            OrgID:      orgID,
            ParentMergeID: parentMergeID, // Links sub-merges for grouped undo
        }

        err := s.ExecuteMerge(ctx, op)
        if err != nil {
            // Rollback all previous merges in this multi-merge
            s.UndoMergeGroup(ctx, orgID, parentMergeID)
            return fmt.Errorf("failed to merge %s: %w", sourceID, err)
        }
    }

    return nil
}
```

### Alternative: Batch Merge in Single Transaction

```go
// BatchMerge does all merges in a single transaction (more atomic)
func (s *MergeService) BatchMerge(ctx context.Context, op MergeOperation) error {
    if len(op.SourceIDs) == 0 {
        return fmt.Errorf("no source records to merge")
    }

    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    mergeID := sfid.NewMerge()

    // Snapshot ALL records first
    for _, sourceID := range op.SourceIDs {
        err = s.snapshotRecord(ctx, tx, mergeID, op.OrgID, op.EntityName, sourceID, "source")
        if err != nil {
            return err
        }
    }
    err = s.snapshotRecord(ctx, tx, mergeID, op.OrgID, op.EntityName, op.MasterID, "master_before")
    if err != nil {
        return err
    }

    // Consolidate all field values at once
    consolidatedValues, err := s.consolidateAllFields(ctx, tx, op)
    if err != nil {
        return err
    }

    // Update master with consolidated values
    err = s.updateMaster(ctx, tx, op.OrgID, op.EntityName, op.MasterID, consolidatedValues)
    if err != nil {
        return err
    }

    // Reparent ALL related records at once
    err = s.reparentAllRelated(ctx, tx, op)
    if err != nil {
        return err
    }

    // Soft-delete ALL source records
    for _, sourceID := range op.SourceIDs {
        err = s.softDelete(ctx, tx, op.OrgID, op.EntityName, sourceID)
        if err != nil {
            return err
        }
    }

    return tx.Commit()
}
```

### Field Consolidation for Multi-Way Merge

```go
type MultiFieldConsolidator struct {
    Rules map[string]MergeRule
}

func (c *MultiFieldConsolidator) Consolidate(fieldName string, values []FieldValue) interface{} {
    rule := c.Rules[fieldName]
    if rule == "" {
        rule = MergeRuleMostComplete // Default
    }

    switch rule {
    case MergeRuleKeepMaster:
        return values[0].Value // First is always master

    case MergeRuleMostRecent:
        var newest FieldValue
        for _, v := range values {
            if v.ModifiedAt.After(newest.ModifiedAt) {
                newest = v
            }
        }
        return newest.Value

    case MergeRuleConcat:
        var parts []string
        seen := make(map[string]bool)
        for _, v := range values {
            if str, ok := v.Value.(string); ok && str != "" && !seen[str] {
                parts = append(parts, str)
                seen[str] = true
            }
        }
        return strings.Join(parts, "; ")

    case MergeRuleSum:
        var sum float64
        for _, v := range values {
            if num, ok := v.Value.(float64); ok {
                sum += num
            }
        }
        return sum

    case MergeRuleMostComplete:
        // Return first non-empty value
        for _, v := range values {
            if !isEmpty(v.Value) {
                return v.Value
            }
        }
        return nil

    case MergeRuleMostFrequent:
        counts := make(map[interface{}]int)
        for _, v := range values {
            counts[v.Value]++
        }
        var maxValue interface{}
        var maxCount int
        for v, count := range counts {
            if count > maxCount {
                maxCount = count
                maxValue = v
            }
        }
        return maxValue
    }

    return values[0].Value
}
```

---

## 6. API Design Recommendations

### Merge Endpoint

```go
// POST /api/v1/{entity}/merge
type MergeRequest struct {
    MasterID     string            `json:"masterId" validate:"required"`
    SourceIDs    []string          `json:"sourceIds" validate:"required,min=1"`
    FieldRules   map[string]string `json:"fieldRules,omitempty"`   // field -> rule
    Preview      bool              `json:"preview,omitempty"`       // Dry run
}

type MergeResponse struct {
    MergeID         string                   `json:"mergeId"`
    MasterRecord    map[string]interface{}   `json:"masterRecord"`
    DeletedRecords  int                      `json:"deletedRecords"`
    ReparentedCount map[string]int           `json:"reparentedCount"` // entity -> count
    UndoAvailable   bool                     `json:"undoAvailable"`
    UndoExpiresAt   string                   `json:"undoExpiresAt"`
}

type MergePreviewResponse struct {
    ConsolidatedFields map[string]interface{}    `json:"consolidatedFields"`
    FieldConflicts     []FieldConflict           `json:"fieldConflicts"`
    RelatedRecords     map[string]int            `json:"relatedRecords"` // entity -> count
    Warnings           []string                  `json:"warnings"`
}

type FieldConflict struct {
    FieldName    string        `json:"fieldName"`
    MasterValue  interface{}   `json:"masterValue"`
    SourceValues []interface{} `json:"sourceValues"`
    Resolution   string        `json:"resolution"` // Which rule will be applied
}
```

### Undo Endpoint

```go
// POST /api/v1/merge/{mergeId}/undo
type UndoMergeResponse struct {
    RestoredRecords []string `json:"restoredRecords"`
    Success         bool     `json:"success"`
}
```

### List Merge History

```go
// GET /api/v1/{entity}/{id}/merge-history
type MergeHistoryResponse struct {
    Merges []MergeHistoryEntry `json:"merges"`
}

type MergeHistoryEntry struct {
    MergeID      string    `json:"mergeId"`
    MergedAt     time.Time `json:"mergedAt"`
    MergedBy     string    `json:"mergedBy"`
    SourceIDs    []string  `json:"sourceIds"`
    IsMaster     bool      `json:"isMaster"`      // Was this record the master?
    UndoAvailable bool     `json:"undoAvailable"`
}
```

---

## 7. Edge Cases and Pitfalls

### Critical Edge Cases

| Scenario | Risk | Mitigation |
|----------|------|------------|
| Merge during concurrent edit | Lost updates | Lock records before merge |
| Self-referential lookup | Infinite loop | Detect and prevent A -> A |
| Cascade depth | Performance | Limit depth, async for deep trees |
| Large number of related records | Timeout | Batch updates, progress tracking |
| Required field becomes empty | Validation error | Pre-validate merged result |
| Unique constraint violation | Merge fails | Check uniques before merge |

### Validation Before Merge

```go
func (s *MergeService) ValidateMerge(ctx context.Context, op MergeOperation) []ValidationError {
    var errors []ValidationError

    // 1. Check all records exist and are not deleted
    for _, id := range append(op.SourceIDs, op.MasterID) {
        exists, deleted, err := s.checkRecordStatus(ctx, op.OrgID, op.EntityName, id)
        if err != nil {
            errors = append(errors, ValidationError{
                Field:   "recordId",
                Message: fmt.Sprintf("failed to check %s: %v", id, err),
            })
        } else if !exists {
            errors = append(errors, ValidationError{
                Field:   "recordId",
                Message: fmt.Sprintf("record %s does not exist", id),
            })
        } else if deleted {
            errors = append(errors, ValidationError{
                Field:   "recordId",
                Message: fmt.Sprintf("record %s is deleted", id),
            })
        }
    }

    // 2. Check for circular references
    if s.wouldCreateCircle(ctx, op) {
        errors = append(errors, ValidationError{
            Field:   "merge",
            Message: "merge would create circular reference",
        })
    }

    // 3. Validate merged result against required fields
    merged := s.previewMergedFields(ctx, op)
    fieldDefs, _ := s.metadataRepo.ListFields(ctx, op.OrgID, op.EntityName)
    for _, field := range fieldDefs {
        if field.IsRequired && isEmpty(merged[field.Name]) {
            errors = append(errors, ValidationError{
                Field:   field.Name,
                Message: fmt.Sprintf("required field %s would be empty after merge", field.Label),
            })
        }
    }

    // 4. Check unique constraints
    for fieldName, value := range merged {
        if s.isUniqueField(ctx, op.OrgID, op.EntityName, fieldName) {
            conflicts, _ := s.checkUniqueConflict(ctx, op, fieldName, value)
            if len(conflicts) > 0 {
                errors = append(errors, ValidationError{
                    Field:   fieldName,
                    Message: fmt.Sprintf("unique constraint violation: %s already exists", value),
                })
            }
        }
    }

    return errors
}
```

---

## 8. Implementation Roadmap

### Phase 1: Core Merge (MVP)
- [ ] Two-record merge only
- [ ] Simple field rules (keep master, keep non-empty)
- [ ] Snapshot storage
- [ ] Basic undo within 7 days
- [ ] Audit logging

### Phase 2: Enhanced Merge
- [ ] Multi-way merge (3+ records)
- [ ] All field consolidation rules
- [ ] Merge preview endpoint
- [ ] Configurable retention period
- [ ] Merge history view

### Phase 3: Advanced Features
- [ ] Scheduled merge jobs
- [ ] Bulk merge API
- [ ] Merge templates (saved rule sets)
- [ ] Merge analytics/reporting

---

## Sources

### Official Documentation
- [Salesforce Apex Merge](https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_dml_examples_merge.htm)
- [HubSpot Updated Merge](https://developers.hubspot.com/changelog/updated-merge-functionality-for-crm-objects-including-contacts-and-companies)
- [Dynamics 365 Merge Records](https://learn.microsoft.com/en-us/dynamics365/customerengagement/on-premises/basics/merge-duplicate-records-accounts-contacts-leads?view=op-9-1)
- [SQLite Pragma foreign_key_list](https://www.sqlite.org/pragma.html)

### Third-Party Tools (Reference)
- [Insycle Merge Best Practices](https://support.insycle.com/hc/en-us/articles/6584810088855-Deduplication-Best-Practices)
- [Cloudingo Undo Merge](https://cloudingo.com/blog/how-to-undo-merges-and-restore-records-in-salesforce/)
- [DataGroomr Undo](https://help.datagroomr.com/support/solutions/articles/44002265857-undo-and-rollback-a-merge-)
- [Inogic Deduped Field Merge](https://docs.inogic.com/deduped/configuration/merge-settings/field-merge-criteria)

### GDPR and Retention
- [GDPR Log Management](https://last9.io/blog/gdpr-log-management/)
- [CRM GDPR Compliance](https://usercentrics.com/knowledge-hub/crm-gdpr/)

---

## Confidence Assessment

| Area | Confidence | Reason |
|------|------------|--------|
| Related record discovery | HIGH | Pattern exists in codebase (DiscoverRelatedLists) |
| FK update patterns | HIGH | Standard database patterns, verified with SQLite docs |
| Transaction atomicity | HIGH | SQLite transaction behavior well documented |
| Undo implementation | MEDIUM | Based on third-party tool patterns, needs validation |
| Multi-way merge | MEDIUM | Less common, iterative approach is safest |
| Retention policy | MEDIUM | GDPR guidance exists but no fixed requirement |
