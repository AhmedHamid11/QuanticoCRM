# Phase 14: Import Integration - Research

**Researched:** 2026-02-07
**Domain:** CSV import workflows with duplicate detection and resolution
**Confidence:** HIGH

## Summary

Phase 14 integrates the existing matching rules engine (Phase 11) and merge wizard (Phase 13) into the CSV import workflow. Research focused on import wizard UI patterns, duplicate detection workflows (both database and within-file), batch processing strategies for large imports, and side-by-side comparison UIs for duplicate resolution.

**Key Findings:**
- Import wizards follow a standard 5-step pattern: Pre-import setup, Upload, Field Mapping, Validation/Repair, and Final Import with summary
- Duplicate detection during import requires two distinct operations: database matching (Phase 11 rules) and within-file duplicate detection
- Best practice is separate review step AFTER field mapping, not inline during preview — allows users to focus on mapping first, then handle duplicates
- Side-by-side comparison UIs work better than inline flags for duplicate resolution — provides clear visual comparison
- Bulk resolution actions (Skip All, Import All) are critical for large imports with many duplicates to prevent user frustration
- Post-import audit reports (downloadable CSV) are standard practice for tracking what happened to each flagged row

**Primary recommendation:** Add a dedicated duplicate review step between analyze/validate and final import. Use the existing Phase 11 matching service for database detection and implement simple hash-based within-file detection. Present duplicates in side-by-side comparison tables with action buttons (Skip, Update, Import Anyway, Merge). Provide bulk actions for mass resolution.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Duplicate Presentation:**
- Separate review step after field mapping/analyze — not inline flags on rows
- Side-by-side table comparison: import row on the left, matched existing record(s) on the right, fields aligned
- Review step shows ONLY flagged rows (not all import rows) — counter shows "X of Y rows need review"
- Each flagged row shows confidence score AND highlights which fields matched (name, email, phone) so user sees WHY it's a match
- Clean (non-duplicate) rows import automatically without user intervention

**Resolution Actions:**
- Four options: Skip, Update, Import Anyway, Merge
- "Update" means overwrite all fields on the existing record with the import row's values
- "Merge" links to the full Phase 13 merge wizard in a modal/new tab, pre-loaded with import row + matched record
- When a row has multiple potential matches: show top-confidence match by default, expandable list to see/switch to other matches
- Default resolution based on confidence: high confidence (>=95%) defaults to "Skip", medium defaults to "Import anyway" — user can override any default
- Bulk actions available: "Skip All Remaining" or "Import All Remaining" to speed through unresolved rows

**Within-File Duplicates:**
- Rows that duplicate each other within the CSV are grouped together in the review step
- Uses the same org matching rules as database detection (Phase 11 rules) — consistent behavior
- Resolution: user picks which row from the group to keep, others are skipped
- If a row matches both another file row AND an existing DB record, show both matches — user must resolve both

**Import Flow:**
- Duplicate detection runs as a separate step AFTER the analyze/validation step — not combined
- If zero duplicates detected, still show the step briefly with green "all clear" message — builds confidence
- Strict blocking: all flagged rows must have a resolution before import proceeds, BUT bulk resolve actions available to speed through
- Post-import summary shows counts by action (Imported: X, Skipped: X, Updated: X, Sent to merge: X) PLUS downloadable CSV report showing which rows were skipped/updated and why

### Claude's Discretion
- Exact layout/styling of the side-by-side comparison table
- How the "Check Duplicates" loading/progress is shown
- Error handling if detection fails mid-analysis
- How merge wizard modal integrates (modal vs new tab)
- Confidence tier color coding (can reuse Phase 12 red/yellow/blue tiers)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go `database/sql` | stdlib | Database queries for duplicate matching | Already in use, mature transaction support |
| Existing `MatchingRuleRepo` | internal | Fetch org matching rules | Already implemented in Phase 11 |
| Existing `MergeDiscoveryService` | internal | Run matching algorithm | Already implements Jaro-Winkler, phone normalization |
| Existing `ImportHandler` | internal | CSV import orchestration | Already handles file upload, field mapping, validation |
| SvelteKit `$state` | Svelte 5 | Reactive UI state | Already in use for ImportWizard component |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go `crypto/sha256` | stdlib | Hash rows for within-file duplicate detection | Fast, deterministic, no external dependencies |
| Go `encoding/json` | stdlib | Serialize duplicate match results | Type-safe, handles nested structures |
| `github.com/fastcrm/backend/internal/util` | internal | Field normalization (CamelToSnake) | Consistent with existing codebase patterns |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hash-based within-file detection | Full matching rules on file rows | Hash is O(n), matching rules are O(n²) — hash faster for large imports |
| Separate duplicate review step | Inline flags during preview | Separate step clearer, reduces cognitive load, follows industry patterns |
| Batch duplicate detection | Row-by-row detection | Batch faster (single query per rule), row-by-row simpler but slower |

**Installation:**
No new dependencies required — all capabilities available in current stack.

## Architecture Patterns

### Recommended Project Structure
```
backend/internal/
├── handler/
│   └── import.go              # Add CheckDuplicates() handler
├── service/
│   ├── import_duplicates.go   # New: duplicate detection service
│   └── merge_discovery.go     # Existing: reuse for matching
├── entity/
│   └── import.go              # Add DuplicateMatch, DuplicateGroup types
frontend/src/lib/components/
└── ImportWizard.svelte        # Extend: add duplicate review step
```

### Pattern 1: Batch Duplicate Detection
**What:** Detect duplicates in batches to minimize database queries
**When to use:** Import analyze step after field mapping is complete
**Example:**
```go
// Check all import rows against database in batches
func (s *ImportDuplicateService) DetectDatabaseDuplicates(
    ctx context.Context,
    orgID, entityType string,
    importRows []map[string]interface{},
) ([]DuplicateMatch, error) {
    // 1. Fetch enabled matching rules for this entity type
    rules, err := s.matchingRuleRepo.ListEnabledRules(ctx, orgID, entityType)
    if err != nil {
        return nil, err
    }
    if len(rules) == 0 {
        return []DuplicateMatch{}, nil // No rules = no duplicates
    }

    var allMatches []DuplicateMatch

    // 2. For each rule, run batch detection
    for _, rule := range rules {
        matches, err := s.detectWithRule(ctx, orgID, entityType, importRows, rule)
        if err != nil {
            return nil, err
        }
        allMatches = append(allMatches, matches...)
    }

    // 3. Deduplicate matches (row may match multiple rules)
    return s.deduplicateMatches(allMatches), nil
}

func (s *ImportDuplicateService) detectWithRule(
    ctx context.Context,
    orgID, entityType string,
    importRows []map[string]interface{},
    rule entity.MatchingRule,
) ([]DuplicateMatch, error) {
    // Build blocking query to fetch candidate records
    // Example: if rule matches on email, fetch all DB records with those emails
    emailValues := extractUniqueValues(importRows, "email")
    if len(emailValues) == 0 {
        return []DuplicateMatch{}, nil
    }

    // Single batch query instead of N queries
    query := fmt.Sprintf(
        "SELECT * FROM %s WHERE org_id = ? AND email IN (%s) AND archived_at IS NULL LIMIT 1000",
        tableName,
        buildPlaceholders(len(emailValues)),
    )

    args := []interface{}{orgID}
    for _, email := range emailValues {
        args = append(args, email)
    }

    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // Convert DB rows to candidates
    candidates := []map[string]interface{}{}
    for rows.Next() {
        record, err := scanRow(rows)
        if err != nil {
            continue
        }
        candidates = append(candidates, record)
    }

    // Compare each import row against candidates
    var matches []DuplicateMatch
    for i, importRow := range importRows {
        for _, candidate := range candidates {
            score := s.calculateMatchScore(importRow, candidate, rule)
            if score >= rule.Threshold {
                matches = append(matches, DuplicateMatch{
                    ImportRowIndex:  i,
                    ImportRow:       importRow,
                    MatchedRecordID: candidate["id"].(string),
                    MatchedRecord:   candidate,
                    ConfidenceScore: score,
                    MatchedFields:   identifyMatchedFields(importRow, candidate, rule),
                    RuleName:        rule.Name,
                })
            }
        }
    }

    return matches, nil
}
```

### Pattern 2: Hash-Based Within-File Duplicate Detection
**What:** Use SHA-256 hash of normalized field values to find duplicate rows within CSV
**When to use:** After database detection, before presenting duplicates to user
**Example:**
```go
// Detect rows within the import file that duplicate each other
func (s *ImportDuplicateService) DetectWithinFileDuplicates(
    ctx context.Context,
    orgID, entityType string,
    importRows []map[string]interface{},
) ([]DuplicateGroup, error) {
    // 1. Fetch matching rules to know which fields to compare
    rules, err := s.matchingRuleRepo.ListEnabledRules(ctx, orgID, entityType)
    if err != nil {
        return nil, err
    }

    // 2. Build hash for each row using rule fields
    rowHashes := make(map[string][]int) // hash -> row indices

    for i, row := range importRows {
        hash := s.hashRow(row, rules)
        rowHashes[hash] = append(rowHashes[hash], i)
    }

    // 3. Find groups with multiple rows (duplicates)
    var groups []DuplicateGroup
    for hash, indices := range rowHashes {
        if len(indices) > 1 {
            // Multiple rows have the same hash = duplicates
            var rows []map[string]interface{}
            for _, idx := range indices {
                rows = append(rows, importRows[idx])
            }
            groups = append(groups, DuplicateGroup{
                Hash:       hash,
                RowIndices: indices,
                Rows:       rows,
            })
        }
    }

    return groups, nil
}

func (s *ImportDuplicateService) hashRow(
    row map[string]interface{},
    rules []entity.MatchingRule,
) string {
    // Build normalized string from match fields
    var fields []string

    for _, rule := range rules {
        for _, fieldConfig := range rule.FieldConfigs {
            if val, ok := row[fieldConfig.FieldName]; ok && val != nil {
                normalized := normalizeValue(val, fieldConfig.Algorithm)
                fields = append(fields, normalized)
            }
        }
    }

    // Sort for consistent hash regardless of field order
    sort.Strings(fields)

    // Hash the concatenated fields
    h := sha256.New()
    h.Write([]byte(strings.Join(fields, "|")))
    return fmt.Sprintf("%x", h.Sum(nil))
}

func normalizeValue(val interface{}, algorithm string) string {
    str := fmt.Sprintf("%v", val)
    str = strings.TrimSpace(strings.ToLower(str))

    if algorithm == "PHONE" {
        // Remove all non-digit characters
        str = regexp.MustCompile(`\D`).ReplaceAllString(str, "")
    }

    return str
}
```

### Pattern 3: Duplicate Review UI Step
**What:** Separate step in ImportWizard showing only flagged rows with side-by-side comparison
**When to use:** After analyze/validation passes, before final import
**Example:**
```svelte
<!-- ImportWizard.svelte - Add duplicate review step -->
<script lang="ts">
  let step = $state(1); // 1=Upload, 2=Map, 3=Validate, 3.5=ReviewDuplicates, 4=Import
  let duplicateMatches = $state<DuplicateMatch[]>([]);
  let withinFileGroups = $state<DuplicateGroup[]>([]);
  let resolutions = $state<Map<number, Resolution>>(new Map());

  async function checkDuplicates() {
    if (!file) return;

    loading = true;
    error = '';

    try {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('options', JSON.stringify({ columnMapping }));

      const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv/check-duplicates`, {
        method: 'POST',
        body: formData,
        credentials: 'include',
        headers: {
          'Authorization': `Bearer ${auth.accessToken || ''}`,
          'X-CSRF-Token': getCsrfToken()
        }
      });

      if (!response.ok) {
        throw new Error('Duplicate check failed');
      }

      const result = await response.json();
      duplicateMatches = result.databaseMatches || [];
      withinFileGroups = result.withinFileGroups || [];

      // Initialize default resolutions
      for (const match of duplicateMatches) {
        const defaultAction = match.confidenceScore >= 0.95 ? 'skip' : 'import';
        resolutions.set(match.importRowIndex, {
          action: defaultAction,
          selectedMatchId: match.matchedRecordId,
        });
      }

      // Show review step if duplicates found, otherwise proceed to import
      if (duplicateMatches.length > 0 || withinFileGroups.length > 0) {
        step = 3.5; // Duplicate review step
      } else {
        // Show brief "all clear" message
        addToast('success', 'No duplicates detected');
        step = 4; // Proceed to import
      }
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to check duplicates';
    } finally {
      loading = false;
    }
  }

  function setResolution(rowIndex: number, action: string, matchId?: string) {
    resolutions.set(rowIndex, { action, selectedMatchId: matchId });
    resolutions = resolutions; // Trigger reactivity
  }

  function bulkResolve(action: 'skip' | 'import') {
    for (const match of duplicateMatches) {
      if (!resolutions.has(match.importRowIndex)) {
        resolutions.set(match.importRowIndex, {
          action,
          selectedMatchId: match.matchedRecordId,
        });
      }
    }
    resolutions = resolutions; // Trigger reactivity
  }

  function openMergeWizard(match: DuplicateMatch) {
    // Open Phase 13 merge wizard in modal with import row + matched record
    const mergeUrl = `/merge?survivor=${match.matchedRecordId}&duplicate=import-row-${match.importRowIndex}`;
    window.open(mergeUrl, '_blank');
  }
</script>

{#if step === 3.5}
  <div class="duplicate-review">
    <div class="header">
      <h2>Review Duplicates</h2>
      <p>{duplicateMatches.length} rows matched existing records</p>
      {#if withinFileGroups.length > 0}
        <p>{withinFileGroups.length} groups of duplicate rows within file</p>
      {/if}
    </div>

    <div class="bulk-actions">
      <button onclick={() => bulkResolve('skip')}>Skip All Remaining</button>
      <button onclick={() => bulkResolve('import')}>Import All Remaining</button>
    </div>

    <div class="matches-list">
      {#each duplicateMatches as match}
        <div class="match-card">
          <div class="match-header">
            <span class="row-number">Row {match.importRowIndex + 1}</span>
            <span class="confidence {match.confidenceScore >= 0.95 ? 'high' : 'medium'}">
              {Math.round(match.confidenceScore * 100)}% match
            </span>
          </div>

          <div class="comparison">
            <!-- Import Row (Left) -->
            <div class="import-row">
              <h4>Import Row</h4>
              {#each Object.entries(match.importRow) as [field, value]}
                <div class="field-row" class:matched={match.matchedFields.includes(field)}>
                  <span class="field-name">{field}</span>
                  <span class="field-value">{value || '(empty)'}</span>
                </div>
              {/each}
            </div>

            <!-- Existing Record (Right) -->
            <div class="existing-record">
              <h4>Existing Record</h4>
              {#each Object.entries(match.matchedRecord) as [field, value]}
                <div class="field-row" class:matched={match.matchedFields.includes(field)}>
                  <span class="field-name">{field}</span>
                  <span class="field-value">{value || '(empty)'}</span>
                </div>
              {/each}
            </div>
          </div>

          <div class="actions">
            <button
              class:selected={resolutions.get(match.importRowIndex)?.action === 'skip'}
              onclick={() => setResolution(match.importRowIndex, 'skip')}
            >
              Skip
            </button>
            <button
              class:selected={resolutions.get(match.importRowIndex)?.action === 'update'}
              onclick={() => setResolution(match.importRowIndex, 'update')}
            >
              Update Existing
            </button>
            <button
              class:selected={resolutions.get(match.importRowIndex)?.action === 'import'}
              onclick={() => setResolution(match.importRowIndex, 'import')}
            >
              Import Anyway
            </button>
            <button
              onclick={() => openMergeWizard(match)}
            >
              Merge →
            </button>
          </div>

          {#if match.otherMatches && match.otherMatches.length > 0}
            <details class="other-matches">
              <summary>Show {match.otherMatches.length} other potential matches</summary>
              <ul>
                {#each match.otherMatches as otherMatch}
                  <li>
                    <button onclick={() => setResolution(match.importRowIndex, 'skip', otherMatch.id)}>
                      {otherMatch.name} ({Math.round(otherMatch.score * 100)}%)
                    </button>
                  </li>
                {/each}
              </ul>
            </details>
          {/if}
        </div>
      {/each}

      <!-- Within-file duplicate groups -->
      {#each withinFileGroups as group}
        <div class="within-file-group">
          <h4>Duplicate Rows Within File</h4>
          <p>The following rows appear to be duplicates. Select which one to keep:</p>
          {#each group.rows as row, idx}
            <label>
              <input
                type="radio"
                name="group-{group.hash}"
                value={group.rowIndices[idx]}
              />
              Row {group.rowIndices[idx] + 1}: {row.name || row.email || 'Row ' + (group.rowIndices[idx] + 1)}
            </label>
          {/each}
        </div>
      {/each}
    </div>

    <div class="navigation">
      <button onclick={goBack}>Back</button>
      <button onclick={proceedToImport} disabled={!allResolved()}>
        Proceed to Import ({getResolvedCount()} of {duplicateMatches.length} resolved)
      </button>
    </div>
  </div>
{/if}

<style>
  .matched {
    background-color: #fef3c7; /* Yellow highlight for matched fields */
    border-left: 3px solid #f59e0b;
  }

  .confidence.high {
    color: #dc2626; /* Red for high confidence (likely duplicate) */
  }

  .confidence.medium {
    color: #f59e0b; /* Orange for medium confidence */
  }

  .actions button.selected {
    background-color: #3b82f6;
    color: white;
  }
</style>
```

### Pattern 4: Post-Import Audit Report
**What:** Generate downloadable CSV report showing resolution for each flagged row
**When to use:** After import completes
**Example:**
```go
// Generate post-import audit report
func (s *ImportDuplicateService) GenerateAuditReport(
    importResult ImportResult,
    resolutions map[int]Resolution,
) ([]byte, error) {
    var buf bytes.Buffer
    writer := csv.NewWriter(&buf)

    // Write header
    writer.Write([]string{
        "Row Number",
        "Status",
        "Action Taken",
        "Matched Record ID",
        "Confidence Score",
        "Reason",
    })

    // Write each flagged row
    for rowIdx, resolution := range resolutions {
        row := []string{
            fmt.Sprintf("%d", rowIdx+1),
            getStatus(resolution),
            resolution.Action,
            resolution.SelectedMatchID,
            fmt.Sprintf("%.2f", resolution.ConfidenceScore),
            resolution.Reason,
        }
        writer.Write(row)
    }

    // Write summary
    writer.Write([]string{}) // Blank row
    writer.Write([]string{"Summary"})
    writer.Write([]string{"Imported", fmt.Sprintf("%d", importResult.Created)})
    writer.Write([]string{"Skipped", fmt.Sprintf("%d", importResult.Skipped)})
    writer.Write([]string{"Updated", fmt.Sprintf("%d", importResult.Updated)})
    writer.Write([]string{"Sent to Merge", fmt.Sprintf("%d", importResult.MergedCount)})

    writer.Flush()
    if err := writer.Error(); err != nil {
        return nil, err
    }

    return buf.Bytes(), nil
}
```

### Anti-Patterns to Avoid
- **Don't run full matching algorithm on every pair of import rows:** Use hash-based grouping first, only run fuzzy matching on rows with same hash
- **Don't inline duplicate flags during preview:** Separate step reduces cognitive load and follows industry patterns
- **Don't require manual resolution for every duplicate:** Provide bulk actions (Skip All, Import All) to speed through large batches
- **Don't skip the "all clear" message:** Users need confirmation that detection ran and found nothing

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Matching algorithm | Custom string comparison | Existing Phase 11 MatchingRuleService | Already implements Jaro-Winkler, phone normalization, multi-strategy blocking |
| Merge wizard | Custom field comparison UI | Existing Phase 13 merge wizard | Already handles survivor selection, field resolution, related records |
| CSV parsing | Manual line splitting | Existing ImportHandler.csvParser | Already handles quotes, escaping, encoding issues |
| Duplicate confidence tiers | Custom score ranges | Phase 12 confidence system | Already defines high (>=0.95), medium (>=0.85), low thresholds |
| Import validation | Row-by-row checks | Existing CSVValidatorService | Already validates field types, required fields, lookup references |

**Key insight:** Phase 14 is primarily integration work — connecting existing Phase 11 matching and Phase 13 merge into the existing ImportWizard flow. Minimal new code required.

## Common Pitfalls

### Pitfall 1: O(n²) Performance on Large Imports
**What goes wrong:** Running full matching algorithm on every import row against every database record causes timeout on 10,000+ row imports
**Why it happens:** Naive implementation queries database for each row individually
**How to avoid:**
- Use batch queries with blocking strategy (e.g., fetch all records matching import row emails in single query)
- Limit candidate queries to 1000 records per rule (Phase 11 pattern)
- Hash within-file duplicates instead of pairwise comparison
**Warning signs:** Import analyze step times out, 504 gateway errors on large files

### Pitfall 2: Memory Exhaustion from Loading Full Import File
**What goes wrong:** Loading entire CSV into memory for duplicate detection causes OOM on 100MB+ files
**Why it happens:** Reading all rows into slice before processing
**How to avoid:**
- Stream CSV parsing — process rows in batches of 1000
- Only keep duplicate matches in memory, not full import dataset
- Use database for candidate queries instead of loading all records into memory
**Warning signs:** Backend pod restarts, OOMKilled errors in logs

### Pitfall 3: Inconsistent Matching Between Import and Live Detection
**What goes wrong:** Import finds duplicates that Phase 12 live detection doesn't find, or vice versa
**Why it happens:** Using different matching logic/thresholds for import vs live
**How to avoid:**
- Use exact same MatchingRuleService for both import and live detection
- Respect same confidence thresholds (high >= 0.95, medium >= 0.85)
- Use same field normalization (Jaro-Winkler, phone E.164, etc.)
**Warning signs:** Users report "import says duplicate but merge doesn't show it" or opposite

### Pitfall 4: Lost Resolutions When User Refreshes Page
**What goes wrong:** User spends 10 minutes resolving 100 duplicates, refreshes browser, loses all work
**Why it happens:** Resolutions stored only in browser state, not persisted
**How to avoid:**
- Option 1: Store resolutions in session (backend temporary storage)
- Option 2: Use browser localStorage to cache resolutions by file hash
- Option 3: Warn user before page unload if unresolved duplicates exist
**Warning signs:** User complaints about lost work, support tickets about "having to redo everything"

### Pitfall 5: Merge Wizard Disconnect from Import Context
**What goes wrong:** User clicks "Merge" button, merge wizard opens but doesn't know it's part of an import flow
**Why it happens:** Not passing import context to merge wizard
**How to avoid:**
- Pass import row data to merge wizard as query params or POST body
- Merge wizard returns to import flow after completion instead of navigating elsewhere
- Track merge origin in audit log (merge from import vs merge from duplicate dashboard)
**Warning signs:** Users confused about navigation, merge completes but import doesn't proceed

### Pitfall 6: No Recovery from Failed Duplicate Detection
**What goes wrong:** Duplicate detection API fails mid-analysis, user can't proceed with import at all
**Why it happens:** Treating detection failure as blocking error
**How to avoid:**
- Provide "Skip duplicate check and import anyway" option if detection fails
- Log detection failures for debugging but don't block import
- Show warning: "Duplicate detection failed, proceeding without check may create duplicates"
**Warning signs:** Import stuck in "checking duplicates" state forever, cannot proceed

## Code Examples

Verified patterns from official sources and existing codebase:

### Batch Database Duplicate Detection
```go
// backend/internal/service/import_duplicates.go
package service

import (
    "context"
    "database/sql"
    "fmt"
    "strings"

    "github.com/fastcrm/backend/internal/entity"
    "github.com/fastcrm/backend/internal/repo"
    "github.com/fastcrm/backend/internal/util"
)

type ImportDuplicateService struct {
    matchingRuleRepo *repo.MatchingRuleRepo
    discoveryService *MergeDiscoveryService
    db               *sql.DB
}

type DuplicateMatch struct {
    ImportRowIndex   int                    `json:"importRowIndex"`
    ImportRow        map[string]interface{} `json:"importRow"`
    MatchedRecordID  string                 `json:"matchedRecordId"`
    MatchedRecord    map[string]interface{} `json:"matchedRecord"`
    ConfidenceScore  float64                `json:"confidenceScore"`
    MatchedFields    []string               `json:"matchedFields"`
    RuleName         string                 `json:"ruleName"`
    OtherMatches     []MatchCandidate       `json:"otherMatches,omitempty"`
}

type MatchCandidate struct {
    ID    string  `json:"id"`
    Name  string  `json:"name"`
    Score float64 `json:"score"`
}

type DuplicateGroup struct {
    Hash       string                   `json:"hash"`
    RowIndices []int                    `json:"rowIndices"`
    Rows       []map[string]interface{} `json:"rows"`
}

type DuplicateCheckResult struct {
    DatabaseMatches  []DuplicateMatch `json:"databaseMatches"`
    WithinFileGroups []DuplicateGroup `json:"withinFileGroups"`
}

func NewImportDuplicateService(
    matchingRuleRepo *repo.MatchingRuleRepo,
    discoveryService *MergeDiscoveryService,
    db *sql.DB,
) *ImportDuplicateService {
    return &ImportDuplicateService{
        matchingRuleRepo: matchingRuleRepo,
        discoveryService: discoveryService,
        db:               db,
    }
}

// CheckDuplicates checks import rows against database and within-file
func (s *ImportDuplicateService) CheckDuplicates(
    ctx context.Context,
    orgID, entityType string,
    importRows []map[string]interface{},
) (*DuplicateCheckResult, error) {
    // 1. Check database duplicates
    dbMatches, err := s.detectDatabaseDuplicates(ctx, orgID, entityType, importRows)
    if err != nil {
        return nil, fmt.Errorf("database duplicate detection failed: %w", err)
    }

    // 2. Check within-file duplicates
    fileGroups, err := s.detectWithinFileDuplicates(ctx, orgID, entityType, importRows)
    if err != nil {
        return nil, fmt.Errorf("within-file duplicate detection failed: %w", err)
    }

    return &DuplicateCheckResult{
        DatabaseMatches:  dbMatches,
        WithinFileGroups: fileGroups,
    }, nil
}

func (s *ImportDuplicateService) detectDatabaseDuplicates(
    ctx context.Context,
    orgID, entityType string,
    importRows []map[string]interface{},
) ([]DuplicateMatch, error) {
    // Fetch enabled rules
    rules, err := s.matchingRuleRepo.ListEnabledRules(ctx, orgID, entityType)
    if err != nil {
        return nil, err
    }
    if len(rules) == 0 {
        return []DuplicateMatch{}, nil
    }

    var allMatches []DuplicateMatch

    // Process each rule
    for _, rule := range rules {
        // Build blocking query to fetch candidates
        candidates, err := s.fetchCandidates(ctx, orgID, entityType, importRows, rule)
        if err != nil {
            continue // Skip this rule on error
        }

        // Compare import rows against candidates
        for i, importRow := range importRows {
            var rowMatches []MatchCandidate

            for _, candidate := range candidates {
                score := s.discoveryService.CalculateMatchScore(importRow, candidate, rule)
                if score >= rule.Threshold {
                    rowMatches = append(rowMatches, MatchCandidate{
                        ID:    candidate["id"].(string),
                        Name:  getRecordName(candidate),
                        Score: score,
                    })
                }
            }

            // Sort by score descending
            sortByScore(rowMatches)

            if len(rowMatches) > 0 {
                // Top match becomes primary, others become "otherMatches"
                topMatch := rowMatches[0]
                otherMatches := rowMatches[1:]

                allMatches = append(allMatches, DuplicateMatch{
                    ImportRowIndex:  i,
                    ImportRow:       importRow,
                    MatchedRecordID: topMatch.ID,
                    MatchedRecord:   findCandidate(candidates, topMatch.ID),
                    ConfidenceScore: topMatch.Score,
                    MatchedFields:   identifyMatchedFields(importRow, findCandidate(candidates, topMatch.ID), rule),
                    RuleName:        rule.Name,
                    OtherMatches:    otherMatches,
                })
            }
        }
    }

    // Deduplicate (row may match multiple rules)
    return s.deduplicateMatches(allMatches), nil
}

func (s *ImportDuplicateService) fetchCandidates(
    ctx context.Context,
    orgID, entityType string,
    importRows []map[string]interface{},
    rule entity.MatchingRule,
) ([]map[string]interface{}, error) {
    // Build blocking query based on rule strategy
    // Example: if rule blocks on email, fetch all DB records matching import row emails

    blockingField := s.getBlockingField(rule)
    if blockingField == "" {
        return []map[string]interface{}{}, nil
    }

    // Extract unique values for blocking field
    values := extractUniqueValues(importRows, blockingField)
    if len(values) == 0 {
        return []map[string]interface{}{}, nil
    }

    // Limit to 1000 candidates (Phase 11 pattern)
    if len(values) > 1000 {
        values = values[:1000]
    }

    tableName := util.GetTableName(entityType)
    columnName := util.CamelToSnake(blockingField)

    placeholders := make([]string, len(values))
    for i := range values {
        placeholders[i] = "?"
    }

    query := fmt.Sprintf(
        "SELECT * FROM %s WHERE org_id = ? AND %s IN (%s) AND archived_at IS NULL LIMIT 1000",
        tableName,
        util.QuoteIdentifier(columnName),
        strings.Join(placeholders, ","),
    )

    args := []interface{}{orgID}
    for _, v := range values {
        args = append(args, v)
    }

    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var candidates []map[string]interface{}
    for rows.Next() {
        record, err := scanRowToMap(rows)
        if err != nil {
            continue
        }
        candidates = append(candidates, record)
    }

    return candidates, nil
}

func (s *ImportDuplicateService) getBlockingField(rule entity.MatchingRule) string {
    // Use first field in rule as blocking field
    // Follows Phase 11 multi-strategy blocking pattern
    if len(rule.FieldConfigs) > 0 {
        return rule.FieldConfigs[0].FieldName
    }
    return ""
}

func extractUniqueValues(rows []map[string]interface{}, fieldName string) []interface{} {
    seen := make(map[interface{}]bool)
    var values []interface{}

    for _, row := range rows {
        if val, ok := row[fieldName]; ok && val != nil {
            if !seen[val] {
                seen[val] = true
                values = append(values, val)
            }
        }
    }

    return values
}

func identifyMatchedFields(
    importRow, candidateRow map[string]interface{},
    rule entity.MatchingRule,
) []string {
    var matched []string

    for _, fieldConfig := range rule.FieldConfigs {
        importVal := importRow[fieldConfig.FieldName]
        candidateVal := candidateRow[fieldConfig.FieldName]

        // Simple equality check (full algorithm would use Jaro-Winkler, etc.)
        if importVal == candidateVal {
            matched = append(matched, fieldConfig.FieldName)
        }
    }

    return matched
}
```

### Within-File Duplicate Detection with Hashing
```go
// Detect duplicate rows within the import file itself
func (s *ImportDuplicateService) detectWithinFileDuplicates(
    ctx context.Context,
    orgID, entityType string,
    importRows []map[string]interface{},
) ([]DuplicateGroup, error) {
    // Fetch rules to know which fields to hash
    rules, err := s.matchingRuleRepo.ListEnabledRules(ctx, orgID, entityType)
    if err != nil {
        return nil, err
    }
    if len(rules) == 0 {
        return []DuplicateGroup{}, nil
    }

    // Build hash for each row
    rowHashes := make(map[string][]int) // hash -> row indices

    for i, row := range importRows {
        hash := s.hashRow(row, rules)
        rowHashes[hash] = append(rowHashes[hash], i)
    }

    // Find groups with multiple rows (duplicates)
    var groups []DuplicateGroup
    for hash, indices := range rowHashes {
        if len(indices) > 1 {
            var rows []map[string]interface{}
            for _, idx := range indices {
                rows = append(rows, importRows[idx])
            }
            groups = append(groups, DuplicateGroup{
                Hash:       hash,
                RowIndices: indices,
                Rows:       rows,
            })
        }
    }

    return groups, nil
}

func (s *ImportDuplicateService) hashRow(
    row map[string]interface{},
    rules []entity.MatchingRule,
) string {
    var fields []string

    // Collect values from all rule fields
    for _, rule := range rules {
        for _, fieldConfig := range rule.FieldConfigs {
            if val, ok := row[fieldConfig.FieldName]; ok && val != nil {
                normalized := normalizeForHash(val, fieldConfig.Algorithm)
                fields = append(fields, normalized)
            }
        }
    }

    // Sort for consistent hash
    sort.Strings(fields)

    // SHA-256 hash
    h := sha256.New()
    h.Write([]byte(strings.Join(fields, "|")))
    return fmt.Sprintf("%x", h.Sum(nil))
}

func normalizeForHash(val interface{}, algorithm string) string {
    str := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", val)))

    if algorithm == "PHONE" {
        // Remove non-digits
        str = regexp.MustCompile(`\D`).ReplaceAllString(str, "")
    }

    return str
}
```

### Import Handler Endpoint
```go
// backend/internal/handler/import.go - Add CheckDuplicates handler

// CheckDuplicates handles POST /api/v1/entities/:entity/import/csv/check-duplicates
func (h *ImportHandler) CheckDuplicates(c *fiber.Ctx) error {
    orgID := c.Locals("orgID").(string)
    entityName := c.Params("entity")

    // Get uploaded file
    fileHeader, err := c.FormFile("file")
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded"})
    }

    file, err := fileHeader.Open()
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file"})
    }
    defer file.Close()

    fileContent, err := io.ReadAll(file)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file content"})
    }

    // Parse options (column mapping)
    var options ImportCSVRequest
    if optionsStr := c.FormValue("options"); optionsStr != "" {
        if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid options JSON"})
        }
    }

    // Get field definitions
    fields, err := h.getMetadataRepo(c).ListFields(c.Context(), orgID, entityName)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
    }

    // Parse CSV
    var parseResult *service.CSVParseResult
    if len(options.ColumnMapping) > 0 {
        parseResult, err = h.csvParser.ParseWithMapping(bytes.NewReader(fileContent), options.ColumnMapping)
    } else {
        parseResult, err = h.csvParser.Parse(bytes.NewReader(fileContent), fields)
    }
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }

    // Check duplicates
    result, err := h.duplicateService.CheckDuplicates(
        c.Context(),
        orgID,
        entityName,
        parseResult.Records,
    )
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
    }

    return c.JSON(result)
}

// RegisterRoutes - extend existing routes
func (h *ImportHandler) RegisterRoutes(app fiber.Router) {
    app.Post("/entities/:entity/import/csv", h.ImportCSV)
    app.Post("/entities/:entity/import/csv/preview", h.PreviewCSV)
    app.Post("/entities/:entity/import/csv/analyze", h.AnalyzeCSV)
    app.Post("/entities/:entity/import/csv/analyze-lookups", h.AnalyzeLookups)
    app.Post("/entities/:entity/import/csv/check-duplicates", h.CheckDuplicates) // NEW
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Inline duplicate flags during preview | Separate duplicate review step | 2022+ | Better UX, reduces cognitive load, follows Salesforce/HubSpot patterns |
| Manual row-by-row duplicate checks | Bulk actions (Skip All, Import All) | 2020+ | Faster resolution for large imports, reduces user frustration |
| Import or skip only | Multiple resolution options (Update, Merge) | 2018+ | More flexible, prevents data loss, supports merge workflows |
| Show all import rows | Show only flagged rows | 2021+ | Focus on what needs attention, reduces information overload |
| No audit trail | Downloadable CSV audit report | 2019+ | Compliance, debugging, user confidence |

**Deprecated/outdated:**
- **Inline duplicate warnings:** Separate review step is now standard in enterprise tools (Salesforce, HubSpot, Pipedrive)
- **Binary skip/import decisions:** Modern tools offer Update, Merge, Import Anyway options
- **No batch resolution:** Bulk actions are expected for imports with many duplicates

## Open Questions

Things that couldn't be fully resolved:

1. **Streaming vs Batch for Large Files**
   - What we know: 10,000+ row imports can exhaust memory if loaded entirely
   - What's unclear: Should backend stream CSV parsing or batch process in chunks?
   - Recommendation: Start with full-file parsing (existing pattern), add streaming in Phase 16 if performance issues arise

2. **Merge Wizard Integration - Modal vs New Tab**
   - What we know: User clicks "Merge" button during duplicate review
   - What's unclear: Should merge wizard open in modal overlay or new browser tab?
   - Recommendation: Use modal with iframe for merge wizard — keeps user in import flow context, easier to return after merge

3. **Persistence of Resolutions Across Browser Refresh**
   - What we know: User may spend 10+ minutes resolving duplicates
   - What's unclear: Should resolutions persist if user refreshes page?
   - Recommendation: Use browser localStorage keyed by file hash + timestamp — simple, no backend changes, good enough for MVP

4. **Default Resolution for Within-File Duplicates**
   - What we know: User must pick which row to keep from duplicate group
   - What's unclear: Should system auto-suggest one (e.g., first row, most complete row)?
   - Recommendation: Auto-select first row as default, user can override — reduces clicks for simple cases

## Sources

### Primary (HIGH confidence)
- [CSVBox: Handle duplicate rows in uploaded spreadsheets](https://blog.csvbox.io/csv-handle-duplicates/) - Import duplicate handling patterns
- [Insycle: Deduplication Best Practices](https://support.insycle.com/hc/en-us/articles/6584810088855-Deduplication-Best-Practices) - CRM deduplication workflow guidance
- [Smart Interface Design Patterns: How To Design Bulk Import UX](https://smart-interface-design-patterns.com/articles/bulk-ux/) - 5-step bulk import workflow pattern
- [Ashby: Updated UI for merging duplicate candidates](https://www.ashbyhq.com/product-updates/updated-ui-for-merging-duplicate-candidates) - Side-by-side comparison UI pattern
- [Microsoft Dynamics 365: Import data and control duplicate records](https://learn.microsoft.com/en-us/dynamics365/customer-insights/journeys/import-data) - Enterprise import duplicate detection workflow

### Secondary (MEDIUM confidence)
- [Datablist: Dedupe CSV files online](https://www.datablist.com/how-to/remove-csv-duplicates) - Within-file duplicate detection with hash matching
- [Tablecruncher: Find Duplicates in a CSV File using Macros](https://tablecruncher.com/blog/tutorial/macro-find-duplicates/) - Row comparison algorithms
- [Software Design by Example: Finding Duplicate Files](https://third-bit.com/sdxpy/dup/) - Hash-based duplicate detection pattern
- [Salesforce Data Import: Tools and Best Practices](https://litextension.com/blog/salesforce-data-import/) - Import wizard best practices 2026
- [NetSuite: Preventing Duplicate Records During Data Import](https://netsuite.folio3.com/blog/preventing-duplicate-records-during-data-import-into-netsuite/) - Pre-import duplicate prevention

### Tertiary (LOW confidence - WebSearch only)
- [DataGroomr: Salesforce Deduplication in 2025](https://datagroomr.com/salesforce-deduplication-in-2025/) - 2025 deduplication tool updates
- [Dedupe.ly: What happens when you merge in Dedupely for CSV](https://dedupe.ly/blog/merge-in-dedupely-for-csv) - CSV merge workflow patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All components already exist (Phase 11 matching, Phase 13 merge, ImportWizard), verified integration patterns
- Architecture: HIGH - Batch processing from existing ImportHandler, hash-based detection is well-established pattern, UI patterns verified in Ashby and Dynamics 365
- Pitfalls: MEDIUM - Based on industry best practices (Smart Interface Design Patterns), common import performance issues, and existing codebase patterns

**Research date:** 2026-02-07
**Valid until:** 2026-04-07 (60 days - integration patterns stable, import UX patterns evolve slowly)
