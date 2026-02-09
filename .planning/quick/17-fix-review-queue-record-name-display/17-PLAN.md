---
phase: 17-fix-review-queue-record-name-display
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - backend/internal/dedup/detector.go
  - backend/internal/service/scan_job.go
  - backend/internal/handler/dedup.go
autonomous: true

must_haves:
  truths:
    - "Review Queue shows human-readable record names (e.g., 'John Smith') for both the source record and matched records, not UUIDs"
    - "Real-time duplicate check endpoint returns record names alongside record IDs"
    - "Scan job stores correct match record names (matched record's name, not source record's name)"
  artifacts:
    - path: "backend/internal/dedup/detector.go"
      provides: "DuplicateMatch struct with RecordName field"
      contains: "RecordName"
    - path: "backend/internal/service/scan_job.go"
      provides: "Correct record name assignment for match records in scan alerts"
    - path: "backend/internal/handler/dedup.go"
      provides: "Record name enrichment for CheckDuplicates endpoint"
  key_links:
    - from: "backend/internal/handler/dedup.go"
      to: "backend/internal/util/records.go"
      via: "GetRecordDisplayName for enriching match names"
      pattern: "util\\.GetRecordDisplayName"
    - from: "backend/internal/dedup/detector.go"
      to: "frontend/src/lib/api/dedup.ts"
      via: "DuplicateMatch JSON response matches frontend interface"
      pattern: "recordName"
---

<objective>
Fix Review Queue and duplicate detection to display human-readable record names instead of raw UUIDs.

Purpose: Users currently see cryptic UUIDs in the Review Queue instead of contact/account names, making it impossible to meaningfully review duplicate alerts without clicking into each record.

Output: Review Queue shows proper record names; real-time check endpoint returns names; scan job stores correct names.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@backend/internal/dedup/detector.go — DuplicateMatch struct (missing RecordName field)
@backend/internal/entity/pending_alert.go — DuplicateAlertMatch struct (has RecordName, used by alerts)
@backend/internal/handler/dedup.go — CheckDuplicates + ListPendingAlerts handlers
@backend/internal/service/scan_job.go — Background scan job that creates alerts
@backend/internal/util/records.go — GetRecordDisplayName utility
@frontend/src/lib/api/dedup.ts — Frontend DuplicateMatch interface (already has optional recordName)
@frontend/src/routes/admin/data-quality/review-queue/+page.svelte — Review Queue UI (already renders recordName with fallback)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add RecordName to DuplicateMatch struct and enrich in CheckDuplicates handler</name>
  <files>
    backend/internal/dedup/detector.go
    backend/internal/handler/dedup.go
  </files>
  <action>
1. In `backend/internal/dedup/detector.go`, add `RecordName` field to the `DuplicateMatch` struct:
   ```go
   type DuplicateMatch struct {
       RecordID    string              `json:"recordId"`
       RecordName  string              `json:"recordName,omitempty"`
       MatchResult *entity.MatchResult `json:"matchResult"`
   }
   ```

2. In `backend/internal/handler/dedup.go`, in the `CheckDuplicates` method (around line 249), enrich the matches with record names BEFORE returning the response. After `matches` are obtained (and after the retry-after-provisioning block), add name enrichment:
   ```go
   // Enrich match record names for display
   conn := h.getDB(c)
   rawDB := db.GetRawDB(conn)
   tableName := util.GetTableName(entityType)
   for i := range matches {
       if matches[i].RecordName == "" {
           record, fetchErr := util.FetchRecordAsMap(c.Context(), rawDB, tableName, matches[i].RecordID, orgID)
           if fetchErr == nil && record != nil {
               matches[i].RecordName = util.GetRecordDisplayName(entityType, record)
           }
       }
   }
   ```
   Place this enrichment block BEFORE the `return c.JSON(fiber.Map{"duplicates": matches, "count": len(matches)})` line. Handle both the normal path and the post-retry path by placing it right before the final return.

Note: The frontend `DuplicateMatch` interface in `dedup.ts` already has `recordName?: string` so no frontend changes needed for this endpoint.
  </action>
  <verify>
Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` to confirm the code compiles without errors.
  </verify>
  <done>
DuplicateMatch struct has RecordName field; CheckDuplicates endpoint returns record names in its response; code compiles successfully.
  </done>
</task>

<task type="auto">
  <name>Task 2: Fix scan job storing wrong record name on matches</name>
  <files>
    backend/internal/service/scan_job.go
  </files>
  <action>
In `backend/internal/service/scan_job.go`, around line 348-355, there is a bug where the alert match's `RecordName` is set to the SOURCE record's name instead of the MATCHED record's name:

```go
// CURRENT (BUG): Uses source record's name for the match
alertMatches := []entity.DuplicateAlertMatch{
    {
        RecordID:    match.RecordID,
        RecordName:  s.getRecordName(record),  // BUG: 'record' is the source, not the match
        MatchResult: match.MatchResult,
    },
}
```

Fix this by looking up the matched record's name instead. The matched record needs to be fetched from the database since the scan only has the source `record` in scope. Use the existing `getRecordName` helper but on the correct record:

```go
// Look up the matched record's display name
matchRecordName := ""
matchedRecord, fetchErr := util.FetchRecordAsMap(ctx, db.GetRawDB(tenantDB), tableName, match.RecordID, orgID)
if fetchErr == nil && matchedRecord != nil {
    matchRecordName = util.GetRecordDisplayName(entityType, matchedRecord)
}

alertMatches := []entity.DuplicateAlertMatch{
    {
        RecordID:    match.RecordID,
        RecordName:  matchRecordName,
        MatchResult: match.MatchResult,
    },
}
```

You will need to import `util` and `db` packages if not already imported in this file. Check existing imports first -- `util` may already be available via `"github.com/fastcrm/backend/internal/util"`. The `db` package (`"github.com/fastcrm/backend/internal/db"`) may also need importing. The `tenantDB` and `tableName` variables should already be in scope from the surrounding function context -- verify by reading the full function.

This ensures that when a scan job creates an alert, the match record shows the correct name (the person/account that was matched), not the source record's name.
  </action>
  <verify>
Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` to confirm the code compiles without errors.
  </verify>
  <done>
Scan job correctly stores the matched record's display name (not the source record's name) when creating duplicate alerts; code compiles successfully.
  </done>
</task>

</tasks>

<verification>
1. `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` passes with no errors
2. `go vet ./...` passes with no warnings
3. Verify the `DuplicateMatch` struct in detector.go has the `RecordName` field
4. Verify the `CheckDuplicates` handler enriches record names before returning
5. Verify scan_job.go uses the matched record (not source record) for match name
</verification>

<success_criteria>
- Backend compiles successfully
- DuplicateMatch struct includes RecordName field with proper JSON tag
- CheckDuplicates endpoint enriches match names via GetRecordDisplayName before response
- Scan job correctly fetches and stores the matched record's name (not source record's name)
- No changes needed on frontend (already handles optional recordName with fallback to recordId)
- Review Queue will show human-readable names for both alerts created by real-time checks and background scans
</success_criteria>

<output>
After completion, create `.planning/quick/17-fix-review-queue-record-name-display/17-SUMMARY.md`
</output>
