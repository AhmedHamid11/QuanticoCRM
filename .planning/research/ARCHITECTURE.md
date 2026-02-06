# Architecture Research: Deduplication System

**Domain:** CRM Record Deduplication
**Researched:** 2026-02-05
**Confidence:** HIGH (based on existing codebase analysis + industry patterns)

## Executive Summary

The deduplication system integrates with Quantico CRM's existing Go/Fiber architecture following established patterns: handler -> service -> repo. The system requires three modes of operation: **real-time prevention** (block duplicates on create), **background scanning** (find existing duplicates), and **manual review/merge** (resolve detected duplicates). Given the multi-tenant Turso architecture with per-org databases, duplicate detection must be scoped per-tenant.

## Integration Points with Existing Architecture

### 1. Handler Layer Integration

| Existing Handler | Dedup Integration Point | Integration Type |
|------------------|------------------------|------------------|
| `generic_entity.go` | Hook into Create() for real-time detection | Pre-save validation |
| `import.go` | Call dedup service in import pipeline | Batch detection |
| `bulk.go` | Call dedup service for bulk creates | Batch detection |
| New: `dedup.go` | Manage rules, review duplicates, execute merges | Full CRUD handler |

**Pattern to follow:** The existing `ValidationService` integration in `generic_entity.go` shows the pattern:
```go
// Validate before save (line ~890)
if h.validationService != nil {
    result, err := h.validationService.ValidateOperation(...)
    if !result.Valid {
        return c.Status(422).JSON(...)
    }
}
```

The dedup service integrates identically:
```go
// Check for duplicates before save
if h.dedupService != nil {
    result, err := h.dedupService.DetectDuplicates(ctx, orgID, entityName, body)
    if len(result.Matches) > 0 {
        return c.Status(409).JSON(fiber.Map{
            "error": "Potential duplicate detected",
            "duplicates": result.Matches,
        })
    }
}
```

### 2. Service Layer Integration

| Existing Service | Dedup Relationship | Notes |
|-----------------|-------------------|-------|
| `validation.go` | Sibling pattern | Both fire on CREATE/UPDATE, share caching pattern |
| `tripwire.go` | Post-save events | Tripwires fire after dedup (merge completes) |
| `csv_parser.go` | Input pipeline | Dedup runs after parsing, before import |
| New: `dedup.go` | Core dedup logic | Scoring, matching, merge execution |

### 3. Repository Layer Integration

| Existing Repo | Dedup Usage | Notes |
|--------------|-------------|-------|
| `metadata.go` | Get field definitions | Know which fields to compare |
| `auth.go` | User lookup for merge attribution | Track who merged what |
| New: `dedup.go` | Store rules, match groups, merge history | New tables |

### 4. Multi-Tenant Database Integration

```
                    Request Flow
                         |
                         v
              +-------------------+
              | Tenant Middleware |  <-- Resolves org_id -> tenant DB
              +-------------------+
                         |
                         v
              +-------------------+
              | Dedup Service     |  <-- Uses tenant DB for data queries
              +-------------------+       Uses master DB for rules (if shared)
                         |
              +----------+---------+
              |                    |
              v                    v
        +-----------+        +-----------+
        | Tenant DB |        | Master DB |
        | (records) |        | (dedup    |
        +-----------+        |  rules*)  |
                             +-----------+

        * Rules can be per-tenant or platform-level
```

**Critical:** All record queries MUST include `org_id` filter. This is already enforced throughout the codebase.

## New Components Required

### 1. Handler: `internal/handler/dedup.go`

```go
type DedupHandler struct {
    dedupService      *service.DedupService
    metadataRepo      *repo.MetadataRepo
    tripwireService   TripwireServiceInterface
}

// API Endpoints:
// GET  /api/v1/admin/dedup/rules                   - List dedup rules
// POST /api/v1/admin/dedup/rules                   - Create dedup rule
// PUT  /api/v1/admin/dedup/rules/:id               - Update dedup rule
// DELETE /api/v1/admin/dedup/rules/:id             - Delete dedup rule
// POST /api/v1/admin/dedup/scan                    - Trigger background scan
// GET  /api/v1/admin/dedup/scan/status/:jobId      - Check scan status
// GET  /api/v1/admin/dedup/groups                  - List duplicate groups
// GET  /api/v1/admin/dedup/groups/:id              - Get duplicate group details
// POST /api/v1/admin/dedup/merge                   - Execute merge
// POST /api/v1/admin/dedup/dismiss/:groupId        - Dismiss false positive
```

### 2. Service: `internal/service/dedup.go`

```go
type DedupService struct {
    db             db.DBConn
    dedupRepo      *repo.DedupRepo
    metadataRepo   *repo.MetadataRepo
    ruleCache      *sync.Map  // Cached rules per org+entity
    cacheTTL       time.Duration
}

// Core methods:
func (s *DedupService) DetectDuplicates(ctx, orgID, entityType string, record map[string]interface{}) (*DetectionResult, error)
func (s *DedupService) ScanEntity(ctx, orgID, entityType string, options ScanOptions) (*ScanResult, error)
func (s *DedupService) ExecuteMerge(ctx, orgID string, group *DuplicateGroup, masterID string, fieldSelections map[string]string) (*MergeResult, error)
func (s *DedupService) CalculateMatchScore(record1, record2 map[string]interface{}, rules []FieldRule) float64
```

### 3. Service: `internal/service/similarity.go`

```go
// String similarity algorithms
// Using: github.com/hbollon/go-edlib or github.com/smashedtoatoms/gofuzz

func JaroWinklerSimilarity(s1, s2 string) float64
func LevenshteinSimilarity(s1, s2 string) float64
func NormalizeForComparison(s string) string  // lowercase, trim, remove punctuation
func EmailDomainMatch(e1, e2 string) bool
func PhoneNormalize(phone string) string
```

### 4. Repository: `internal/repo/dedup.go`

```go
type DedupRepo struct {
    db db.DBConn
}

// CRUD for dedup rules
func (r *DedupRepo) CreateRule(ctx, orgID string, input DedupRuleInput) (*DedupRule, error)
func (r *DedupRepo) ListRules(ctx, orgID, entityType string) ([]DedupRule, error)
func (r *DedupRepo) GetRule(ctx, ruleID string) (*DedupRule, error)
func (r *DedupRepo) UpdateRule(ctx, ruleID string, input DedupRuleInput) (*DedupRule, error)
func (r *DedupRepo) DeleteRule(ctx, ruleID string) error

// Duplicate groups
func (r *DedupRepo) CreateDuplicateGroup(ctx context.Context, group *DuplicateGroup) error
func (r *DedupRepo) ListDuplicateGroups(ctx, orgID, entityType string, status string) ([]DuplicateGroup, error)
func (r *DedupRepo) GetDuplicateGroup(ctx, groupID string) (*DuplicateGroup, error)
func (r *DedupRepo) UpdateDuplicateGroup(ctx, groupID string, status string) error

// Merge history
func (r *DedupRepo) LogMerge(ctx context.Context, merge *MergeLog) error
func (r *DedupRepo) GetMergeHistory(ctx, orgID, entityType string) ([]MergeLog, error)
```

### 5. Entity Types: `internal/entity/dedup.go`

```go
// DedupRule defines matching criteria for an entity
type DedupRule struct {
    ID             string            `json:"id"`
    OrgID          string            `json:"orgId"`
    EntityType     string            `json:"entityType"`
    Name           string            `json:"name"`
    Enabled        bool              `json:"enabled"`
    Priority       int               `json:"priority"`
    MatchThreshold float64           `json:"matchThreshold"`  // 0.0-1.0
    BlockOnMatch   bool              `json:"blockOnMatch"`    // Real-time block vs warn
    FieldRules     []FieldRule       `json:"fieldRules"`
    CreatedAt      time.Time         `json:"createdAt"`
    ModifiedAt     time.Time         `json:"modifiedAt"`
}

// FieldRule defines how to compare a specific field
type FieldRule struct {
    FieldName      string   `json:"fieldName"`
    MatchType      string   `json:"matchType"`  // exact, fuzzy, phonetic, email_domain
    Weight         float64  `json:"weight"`     // Importance in overall score
    CaseSensitive  bool     `json:"caseSensitive"`
    Threshold      float64  `json:"threshold"`  // For fuzzy: minimum similarity
}

// DuplicateGroup represents a set of matching records
type DuplicateGroup struct {
    ID            string      `json:"id"`
    OrgID         string      `json:"orgId"`
    EntityType    string      `json:"entityType"`
    RecordIDs     []string    `json:"recordIds"`
    MatchScore    float64     `json:"matchScore"`
    Status        string      `json:"status"`  // pending, merged, dismissed
    DetectedAt    time.Time   `json:"detectedAt"`
    DetectedBy    string      `json:"detectedBy"`  // scan, realtime, import
    ResolvedAt    *time.Time  `json:"resolvedAt,omitempty"`
    ResolvedByID  *string     `json:"resolvedById,omitempty"`
}

// MergeLog tracks merge operations for audit
type MergeLog struct {
    ID              string                 `json:"id"`
    OrgID           string                 `json:"orgId"`
    EntityType      string                 `json:"entityType"`
    MasterRecordID  string                 `json:"masterRecordId"`
    MergedRecordIDs []string               `json:"mergedRecordIds"`
    FieldSelections map[string]string      `json:"fieldSelections"`  // field -> source record ID
    MergedByID      string                 `json:"mergedById"`
    MergedAt        time.Time              `json:"mergedAt"`
    // Store full record snapshots for potential undo
    MasterSnapshot  map[string]interface{} `json:"masterSnapshot"`
    MergedSnapshots []map[string]interface{} `json:"mergedSnapshots"`
}
```

## Data Flow Diagrams

### Flow 1: Real-Time Duplicate Detection (Create)

```
POST /api/v1/entities/Contact/records
         |
         v
+-------------------+
| Auth Middleware   |
+-------------------+
         |
         v
+-------------------+
| Tenant Middleware |  <-- Sets tenant DB
+-------------------+
         |
         v
+-------------------+
| GenericEntity     |
| Handler.Create()  |
+-------------------+
         |
         v
+-------------------+     +-------------------+
| Validation        | --> | DedupService      |
| Service           |     | .DetectDuplicates |
+-------------------+     +-------------------+
                                   |
                                   v
                          +-------------------+
                          | Load active rules |
                          | for Contact       |
                          +-------------------+
                                   |
                                   v
                          +-------------------+
                          | Query candidates: |
                          | SELECT * FROM     |
                          | contacts WHERE    |
                          | email LIKE '%...' |
                          | OR name LIKE '%'  |
                          +-------------------+
                                   |
                                   v
                          +-------------------+
                          | Score each        |
                          | candidate         |
                          +-------------------+
                                   |
                    +--------------+--------------+
                    |                             |
                    v                             v
         +------------------+           +------------------+
         | Score >= 0.85    |           | Score < 0.85     |
         | AND BlockOnMatch |           | OR !BlockOnMatch |
         +------------------+           +------------------+
                    |                             |
                    v                             v
         +------------------+           +------------------+
         | 409 Conflict     |           | Proceed with     |
         | Return matches   |           | record creation  |
         +------------------+           +------------------+
```

### Flow 2: Background Scan

```
POST /api/v1/admin/dedup/scan
{
  "entityType": "Contact",
  "fullScan": true,
  "threshold": 0.8
}
         |
         v
+-------------------+
| DedupHandler      |
| .TriggerScan()    |
+-------------------+
         |
         v
+-------------------+
| Create scan job   |
| Return job ID     |
+-------------------+
         |
         v  (Background goroutine)
+-------------------+
| DedupService      |
| .ScanEntity()     |
+-------------------+
         |
         v
+-------------------+
| Fetch all records |
| in batches of 100 |
+-------------------+
         |
         v (For each batch)
+-------------------+
| Compare pairwise  |
| with scoring      |
+-------------------+
         |
         v
+-------------------+
| Group records     |
| with score >= thr |
+-------------------+
         |
         v
+-------------------+
| Store groups in   |
| duplicate_groups  |
| table             |
+-------------------+
         |
         v
+-------------------+
| Update job status |
| to 'completed'    |
+-------------------+
```

### Flow 3: Merge Execution

```
POST /api/v1/admin/dedup/merge
{
  "groupId": "dg_abc123",
  "masterRecordId": "Rec_xyz",
  "fieldSelections": {
    "phone": "Rec_other",
    "address": "Rec_xyz"
  }
}
         |
         v
+-------------------+
| DedupHandler      |
| .ExecuteMerge()   |
+-------------------+
         |
         v
+-------------------+
| Validate group    |
| status = pending  |
+-------------------+
         |
         v
+-------------------+
| Load all records  |
| in the group      |
+-------------------+
         |
         v
+-------------------+
| Build merged      |
| record from       |
| selections        |
+-------------------+
         |
         v
+-------------------+
| BEGIN TRANSACTION |
+-------------------+
         |
         v
+-------------------+
| UPDATE master     |
| record with       |
| merged values     |
+-------------------+
         |
         v
+-------------------+
| Re-point related  |
| records:          |
| - Lookup fields   |
| - Related lists   |
+-------------------+
         |
         v
+-------------------+
| DELETE merged     |
| (subordinate)     |
| records           |
+-------------------+
         |
         v
+-------------------+
| Log to merge_log  |
| (with snapshots)  |
+-------------------+
         |
         v
+-------------------+
| Update group      |
| status = 'merged' |
+-------------------+
         |
         v
+-------------------+
| COMMIT            |
+-------------------+
         |
         v
+-------------------+
| Fire tripwires    |
| for UPDATE on     |
| master record     |
+-------------------+
```

## Database Schema Additions

### New Tables

```sql
-- Migration: 048_create_dedup_rules.sql
CREATE TABLE dedup_rules (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    name TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    priority INTEGER DEFAULT 0,
    match_threshold REAL DEFAULT 0.85,
    block_on_match INTEGER DEFAULT 0,
    field_rules TEXT NOT NULL,  -- JSON array of FieldRule
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_dedup_rules_org_entity ON dedup_rules(org_id, entity_type, enabled);

-- Migration: 049_create_duplicate_groups.sql
CREATE TABLE duplicate_groups (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    record_ids TEXT NOT NULL,  -- JSON array of record IDs
    match_score REAL NOT NULL,
    status TEXT DEFAULT 'pending',  -- pending, merged, dismissed
    detected_at TEXT DEFAULT CURRENT_TIMESTAMP,
    detected_by TEXT NOT NULL,  -- scan, realtime, import
    resolved_at TEXT,
    resolved_by_id TEXT
);
CREATE INDEX idx_duplicate_groups_org_status ON duplicate_groups(org_id, entity_type, status);
CREATE INDEX idx_duplicate_groups_detected ON duplicate_groups(detected_at DESC);

-- Migration: 050_create_merge_logs.sql
CREATE TABLE merge_logs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    duplicate_group_id TEXT,
    master_record_id TEXT NOT NULL,
    merged_record_ids TEXT NOT NULL,  -- JSON array
    field_selections TEXT NOT NULL,   -- JSON object
    master_snapshot TEXT NOT NULL,    -- JSON: full record before merge
    merged_snapshots TEXT NOT NULL,   -- JSON: array of full records
    merged_by_id TEXT NOT NULL,
    merged_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (duplicate_group_id) REFERENCES duplicate_groups(id)
);
CREATE INDEX idx_merge_logs_org ON merge_logs(org_id, entity_type);
CREATE INDEX idx_merge_logs_master ON merge_logs(master_record_id);

-- Migration: 051_create_scan_jobs.sql (optional, for tracking background jobs)
CREATE TABLE dedup_scan_jobs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    status TEXT DEFAULT 'pending',  -- pending, running, completed, failed
    progress_current INTEGER DEFAULT 0,
    progress_total INTEGER DEFAULT 0,
    duplicates_found INTEGER DEFAULT 0,
    error_message TEXT,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_scan_jobs_org ON dedup_scan_jobs(org_id, status);
```

### Table Location (Master vs Tenant)

| Table | Location | Rationale |
|-------|----------|-----------|
| `dedup_rules` | **Tenant DB** | Rules are org-specific, data isolation |
| `duplicate_groups` | **Tenant DB** | References tenant records |
| `merge_logs` | **Tenant DB** | Audit per tenant |
| `dedup_scan_jobs` | **Tenant DB** | Scans are per-tenant |

Note: No master DB tables needed for dedup. All dedup data is per-tenant.

## Scoring Algorithm

### Field-Level Scoring

```go
func (s *DedupService) scoreField(val1, val2 interface{}, rule FieldRule) float64 {
    str1 := normalize(toString(val1))
    str2 := normalize(toString(val2))

    if str1 == "" || str2 == "" {
        return 0.0  // Can't compare empty values
    }

    switch rule.MatchType {
    case "exact":
        if str1 == str2 {
            return 1.0
        }
        return 0.0

    case "fuzzy":
        // Jaro-Winkler preferred for names (O(m+n) vs Levenshtein's O(m*n))
        similarity := JaroWinklerSimilarity(str1, str2)
        if similarity >= rule.Threshold {
            return similarity
        }
        return 0.0

    case "phonetic":
        // Soundex/Metaphone for names
        if Soundex(str1) == Soundex(str2) {
            return 0.85  // Fixed score for phonetic match
        }
        return 0.0

    case "email_domain":
        // john@acme.com matches jane@acme.com
        domain1 := extractDomain(str1)
        domain2 := extractDomain(str2)
        if domain1 == domain2 && domain1 != "" {
            return 0.5  // Partial match
        }
        // Full email match
        if str1 == str2 {
            return 1.0
        }
        return 0.0

    case "phone":
        // Normalize and compare last 10 digits
        phone1 := normalizePhone(str1)
        phone2 := normalizePhone(str2)
        if phone1 == phone2 {
            return 1.0
        }
        return 0.0
    }

    return 0.0
}

// Overall record score using weighted average
func (s *DedupService) CalculateMatchScore(
    record1, record2 map[string]interface{},
    rules []FieldRule,
) float64 {
    var totalWeight float64
    var weightedScore float64

    for _, rule := range rules {
        val1 := record1[rule.FieldName]
        val2 := record2[rule.FieldName]

        fieldScore := s.scoreField(val1, val2, rule)
        weightedScore += fieldScore * rule.Weight
        totalWeight += rule.Weight
    }

    if totalWeight == 0 {
        return 0.0
    }

    return weightedScore / totalWeight
}
```

### Example Rule Configuration

```json
{
  "name": "Standard Contact Matching",
  "entityType": "Contact",
  "matchThreshold": 0.85,
  "blockOnMatch": true,
  "fieldRules": [
    {
      "fieldName": "email",
      "matchType": "exact",
      "weight": 3.0,
      "caseSensitive": false
    },
    {
      "fieldName": "firstName",
      "matchType": "fuzzy",
      "weight": 1.0,
      "threshold": 0.8
    },
    {
      "fieldName": "lastName",
      "matchType": "fuzzy",
      "weight": 2.0,
      "threshold": 0.8
    },
    {
      "fieldName": "phone",
      "matchType": "phone",
      "weight": 2.5
    }
  ]
}
```

## Survivorship Rules for Merge

### Master Record Selection

```go
type MasterSelectionRule string

const (
    MasterLatestModified  MasterSelectionRule = "latest_modified"
    MasterMostFields      MasterSelectionRule = "most_fields"
    MasterMostActivities  MasterSelectionRule = "most_activities"
    MasterManual          MasterSelectionRule = "manual"
)

func (s *DedupService) SelectMaster(records []map[string]interface{}, rule MasterSelectionRule) string {
    switch rule {
    case MasterLatestModified:
        // Return ID of most recently modified record
    case MasterMostFields:
        // Return ID of record with most non-null fields
    case MasterMostActivities:
        // Return ID of record with most related activities
    default:
        // Manual selection required
        return ""
    }
}
```

### Field-Level Survivorship

```go
type FieldSurvivorship string

const (
    SurviveFromMaster   FieldSurvivorship = "master"         // Always use master value
    SurviveNonEmpty     FieldSurvivorship = "non_empty"      // Use first non-empty
    SurviveLatest       FieldSurvivorship = "latest"         // Use most recent value
    SurviveManual       FieldSurvivorship = "manual"         // User must choose
    SurviveConcatenate  FieldSurvivorship = "concatenate"    // Combine values (for notes)
)
```

## Build Order (Recommended Phase Sequence)

### Phase 1: Foundation
1. Create entity types (`internal/entity/dedup.go`)
2. Create database migrations (rules, groups, logs, jobs)
3. Create repository (`internal/repo/dedup.go`)
4. Create similarity service (`internal/service/similarity.go`)

**Why first:** No dependencies, purely additive.

### Phase 2: Detection Engine
5. Create dedup service core (`internal/service/dedup.go`)
   - Rule loading and caching
   - Candidate selection (blocking queries)
   - Scoring algorithm
6. Create admin handler for rules (`internal/handler/dedup.go`)
   - CRUD for dedup rules

**Why second:** Requires Phase 1 complete.

### Phase 3: Real-Time Integration
7. Integrate into `GenericEntityHandler.Create()`
8. Integrate into `ImportHandler.ImportCSV()`
9. Integrate into `BulkHandler.BulkCreate()`

**Why third:** Requires detection engine.

### Phase 4: Background Scanning
10. Implement scan job system (goroutine-based initially)
11. Add scan endpoints to handler
12. Implement pairwise comparison with batching

**Why fourth:** Can operate independently once detection works.

### Phase 5: Merge System
13. Implement merge execution logic
14. Implement related record re-pointing
15. Implement merge logging with snapshots
16. Add merge endpoints

**Why fifth:** Most complex, requires all above.

### Phase 6: UI Components
17. Admin UI for rule management
18. Duplicate review queue
19. Merge wizard with field selection

**Why last:** Backend must be complete first.

## Performance Considerations

### Candidate Selection Optimization

```sql
-- Instead of comparing ALL records, use blocking
-- Only compare records that share at least one matching attribute

-- For email-based blocking:
SELECT * FROM contacts
WHERE org_id = ?
AND email LIKE ?  -- First 3 chars of new email

-- For name-based blocking:
SELECT * FROM contacts
WHERE org_id = ?
AND (
    LOWER(first_name) LIKE ?
    OR LOWER(last_name) LIKE ?
    OR SOUNDEX(first_name) = ?
    OR SOUNDEX(last_name) = ?
)
```

### Indexing Recommendations

```sql
-- Add to existing entity tables for dedup performance
CREATE INDEX idx_contacts_email_lower ON contacts(LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_contacts_name_search ON contacts(LOWER(first_name), LOWER(last_name));
CREATE INDEX idx_contacts_phone_norm ON contacts(phone) WHERE phone IS NOT NULL;
```

### Batch Processing

For background scans with large datasets:
- Process in batches of 100-500 records
- Use cursor-based pagination (not OFFSET)
- Score comparisons are O(n^2) worst case - use blocking to reduce to O(n * k) where k << n
- Store intermediate results to allow pause/resume

## Anti-Patterns to Avoid

### 1. Comparing All Records Against All
```go
// BAD: O(n^2) comparison
for _, r1 := range records {
    for _, r2 := range records {
        score := compare(r1, r2)  // Explodes with scale
    }
}
```

### 2. Synchronous Background Scans
```go
// BAD: Blocks request until scan completes
func (h *Handler) TriggerScan(c *fiber.Ctx) error {
    result := h.dedupService.ScanEntity(...)  // Might take minutes
    return c.JSON(result)
}

// GOOD: Return job ID, process async
func (h *Handler) TriggerScan(c *fiber.Ctx) error {
    job := h.dedupService.CreateScanJob(...)
    go h.dedupService.RunScanJob(job.ID)  // Background
    return c.JSON(fiber.Map{"jobId": job.ID})
}
```

### 3. Missing Transaction for Merge
```go
// BAD: Partial merge if interrupted
h.db.Exec("UPDATE master SET ...")
h.db.Exec("UPDATE related SET ...")  // Crash here = inconsistent state
h.db.Exec("DELETE FROM merged ...")

// GOOD: All or nothing
tx, _ := h.db.BeginTx(ctx, nil)
tx.Exec("UPDATE master SET ...")
tx.Exec("UPDATE related SET ...")
tx.Exec("DELETE FROM merged ...")
tx.Commit()
```

## External Dependencies

### Required Go Package
```go
// go get github.com/hbollon/go-edlib
// Provides: Jaro-Winkler, Levenshtein, Soundex, etc.

import edlib "github.com/hbollon/go-edlib"

similarity := edlib.JaroWinklerSimilarity("string1", "string2")
```

### Alternative: github.com/smashedtoatoms/gofuzz
- More algorithms (Metaphone, NYSIIS)
- Based on rockymadden/stringmetric (well-tested)

## Sources

- [CRM Deduplication Guide 2025](https://www.rtdynamic.com/blog/crm-deduplication-guide-2025/)
- [Jaro-Winkler Distance Explained](https://www.datablist.com/learn/data-cleaning/fuzzy-matching-jaro-winkler-distance)
- [Fuzzy Matching Algorithms for Deduplication](https://tilores.io/fuzzy-matching-algorithms)
- [go-edlib Package](https://pkg.go.dev/github.com/hbollon/go-edlib)
- [gofuzz Package](https://github.com/smashedtoatoms/gofuzz)
- [River Queue for Background Jobs](https://riverqueue.com/)
- [Survivorship Rules in Data Matching](https://help.qlik.com/talend/en-US/data-matching/8.0/using-survivorship-functions-to-merge-two-records)
- [Master Deciding Rules in CRM](https://www.inogic.com/blog/2025/10/new-release-merge-duplicate-records-automatically-in-dynamics-365-crm-with-master-deciding-rules/)
- [Batch vs Real-Time Deduplication Tradeoffs](https://risingwave.com/blog/effective-deduplication-of-events-in-batch-and-stream-processing/)
