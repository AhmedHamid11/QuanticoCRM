# P1/P2 Fix Test Scenarios

**Date:** 2026-02-15
**Target:** Mirror Ingest API dedup endpoints validation
**Priority:** Critical path verification before v5.0 production deployment

---

## Overview

This document defines test scenarios specifically targeting the P1 (critical) and P2 (high) bug fixes implemented in the Mirror Ingest Layer. These tests verify error handling, data validation, and transaction safety.

---

## P1 Critical Scenarios

### P1-1: "no such table" Errors → Graceful Empty Response

**Target Files:**
- `/backend/internal/handler/dedup_results.go:42-44` (ListImports)
- `/backend/internal/handler/dedup_results.go:77-79` (GetDedupResults - GetJob)
- `/backend/internal/handler/dedup_results.go:89-94` (GetDedupResults - GetDecisions)

**Root Cause:** Tenant DBs created before `import_jobs`/`import_dedup_decisions` tables were added to migrations.

#### Scenario P1-1a: ListImports with Missing Table
```
GIVEN: Fresh org with no import_jobs table in tenant DB
WHEN: GET /api/v1/ingest/imports
THEN:
  - Status: 200 OK (NOT 500)
  - Response body: {"imports": [], "total": 0, "page": 1, "pageSize": 20}
  - No raw database error message in response
  - Console log contains: "[DEDUP-API] ListImports error" (NOT visible to client)
```

#### Scenario P1-1b: GetDedupResults with Missing import_jobs Table
```
GIVEN: Fresh org with no import_jobs table
WHEN: GET /api/v1/ingest/imports/{uuid}/dedup-results
THEN:
  - Status: 404 Not Found
  - Response body: {"error": "Import job not found"}
  - NO error message containing "no such table: import_jobs"
```

#### Scenario P1-1c: GetDedupResults with Missing import_dedup_decisions Table
```
GIVEN: Org with import_jobs table but missing import_dedup_decisions table
  AND: Valid import job exists in import_jobs
WHEN: GET /api/v1/ingest/imports/{valid-job-id}/dedup-results
THEN:
  - Status: 200 OK
  - Response body contains job metadata (import_id, entity_type, counts, created_at)
  - Response.decisions: [] (empty array, NOT error)
  - NO "no such table" error visible in response
```

### P1-2: No Raw err.Error() Leaking in ANY Error Path

**Target:** All error responses in dedup_results.go

#### Scenario P1-2a: Database Connection Failure
```
GIVEN: Simulated tenant DB connection failure
WHEN: GET /api/v1/ingest/imports
THEN:
  - Status: 500 Internal Server Error
  - Response body: {"error": "Database connection error"}
  - NO raw error like "unable to open database file"
```

#### Scenario P1-2b: SQL Query Syntax Error (Unexpected)
```
GIVEN: Hypothetical SQL error (not "no such table")
WHEN: Calling ListJobs/GetJob/GetDecisions
THEN:
  - Status: 500 Internal Server Error
  - Response: {"error": "Failed to retrieve imports"} OR {"error": "Failed to retrieve import"}
  - Console logs error details
  - Client sees NO raw error message
```

#### Scenario P1-2c: Missing Organization Context
```
GIVEN: Invalid or missing ingestOrgID in request context
WHEN: GET /api/v1/ingest/imports
THEN:
  - Status: 401 Unauthorized
  - Response: {"error": "Missing organization context"}
```

---

## P2 High-Priority Scenarios

### P2-1: Invalid DecisionType/Action Values

**Target Files:**
- `/backend/internal/handler/import.go:591-600` (Validation logic)
- `/backend/internal/handler/import.go:614-616` (Warnings on skip)

#### Scenario P2-1a: Invalid DecisionType (Wrong Case)
```
GIVEN: POST /api/v1/entities/Contact/import/csv
  AND: Request includes dedupDecisions with entry:
       {"decisionType": "Within_File", "action": "skip", ...}
WHEN: Import executes
THEN:
  - Import completes successfully (best-effort)
  - Response.warnings contains: "Skipped 1 dedup decisions with invalid type or action"
  - Invalid decision NOT persisted to import_dedup_decisions
  - Valid decisions (if any) ARE persisted
```

#### Scenario P2-1b: Invalid Action (Unknown Value)
```
GIVEN: POST /api/v1/entities/Contact/import/csv
  AND: dedupDecisions contains:
       {"decisionType": "db_match", "action": "auto_merge", ...}
WHEN: Import executes
THEN:
  - Response.warnings: "Skipped 1 dedup decisions with invalid type or action"
  - Decision with "auto_merge" not saved
  - Console: "[IMPORT] Warning: skipped 1 dedup decisions with invalid decisionType or action"
```

#### Scenario P2-1c: Empty DecisionType/Action
```
GIVEN: dedupDecisions with:
       {"decisionType": "", "action": "", ...}
WHEN: Import executes
THEN:
  - Skipped as invalid
  - Warning appears in response
```

#### Scenario P2-1d: Case Sensitivity Check
```
GIVEN: Multiple dedup decisions:
  [
    {"decisionType": "within_file", "action": "skip"},      ✅ valid
    {"decisionType": "WITHIN_FILE", "action": "skip"},      ❌ invalid
    {"decisionType": "db_match", "action": "UPDATE"},       ❌ invalid
    {"decisionType": "db_match", "action": "update"}        ✅ valid
  ]
WHEN: Import executes
THEN:
  - Response.warnings: "Skipped 2 dedup decisions with invalid type or action"
  - Only 2 valid decisions persisted
  - Verify DB query: SELECT COUNT(*) FROM import_dedup_decisions WHERE import_job_id = ? → returns 2
```

### P2-2: SaveDecisions Transaction Rollback

**Target:** `/backend/internal/repo/import_job.go:145-180` (SaveDecisions method)

#### Scenario P2-2a: Database Constraint Violation (Simulated)
```
GIVEN: Attempt to insert duplicate decision IDs (hypothetical unique constraint)
WHEN: SaveDecisions called with:
  - Decision 1: valid
  - Decision 2: duplicate ID causing constraint violation
THEN:
  - tx.Rollback() called
  - NO decisions persisted (all-or-nothing)
  - Error returned: "insert dedup decisions batch: ..."
  - Verify: SELECT COUNT(*) FROM import_dedup_decisions → 0 (full rollback)
```

#### Scenario P2-2a-variant: Table Missing During SaveDecisions
```
GIVEN: import_jobs table exists, but import_dedup_decisions table missing
WHEN: SaveDecisions executes
THEN:
  - Transaction rollback triggered
  - Error propagates up to import handler
  - Response.warnings: "Import succeeded but failed to save dedup decision records"
  - Import itself completes successfully (best-effort pattern)
```

### P2-3: Warnings Field in Response

**Target:** `/backend/internal/handler/import.go:566-620` (Deferred persistence block)

#### Scenario P2-3a: Import Job Creation Failure
```
GIVEN: Tenant DB available but CreateJob fails (e.g., disk full simulation)
WHEN: Import completes successfully
THEN:
  - Response.created/updated counts accurate
  - Response.warnings: ["Import succeeded but failed to save import tracking record"]
  - Response.importId: "" (empty, not set)
  - No dedup decisions persisted (SaveDecisions skipped if CreateJob fails)
```

#### Scenario P2-3b: SaveDecisions Failure (Job Created Successfully)
```
GIVEN: CreateJob succeeds, SaveDecisions fails
WHEN: Import completes
THEN:
  - Response.importId: "{valid-uuid}" (job created)
  - Response.warnings: ["Import succeeded but failed to save dedup decision records"]
  - Verify: import_jobs table has 1 row
  - Verify: import_dedup_decisions table has 0 rows
```

#### Scenario P2-3c: Multiple Warnings Accumulate
```
GIVEN: Both invalid decisions AND SaveDecisions failure
WHEN: Import executes
THEN:
  - Response.warnings: [
      "Skipped 3 dedup decisions with invalid type or action",
      "Import succeeded but failed to save dedup decision records"
    ]
  - Array contains both warnings in order
```

#### Scenario P2-3d: Panic Recovery During Persistence
```
GIVEN: Simulated panic inside persistence defer block (lines 562-568)
WHEN: Panic occurs (e.g., nil pointer dereference)
THEN:
  - Panic recovered via defer recover()
  - Response.warnings: ["Import succeeded but failed to save dedup tracking data"]
  - Console: "[IMPORT] Warning: panic persisting import job: {panic message}"
  - Import overall status: 201 Created (import succeeded)
```

### P2-4: Mix of Valid/Invalid Decisions

**Target:** `/backend/internal/handler/import.go:589-613` (Validation + filtering)

#### Scenario P2-4a: Partial Valid Set
```
GIVEN: 10 dedup decisions:
  - 7 valid (correct decisionType + action)
  - 2 invalid decisionType
  - 1 invalid action
WHEN: Import completes
THEN:
  - Response.warnings: "Skipped 3 dedup decisions with invalid type or action"
  - Console: "[IMPORT] Warning: skipped 3 dedup decisions..."
  - Database query: SELECT COUNT(*) → 7 rows in import_dedup_decisions
  - Verify 7 valid decisions have correct fields (keptExternalId, discardedExternalId, etc.)
```

#### Scenario P2-4b: All Invalid Decisions
```
GIVEN: 5 dedup decisions, all with invalid values
WHEN: Import completes
THEN:
  - Response.warnings: "Skipped 5 dedup decisions with invalid type or action"
  - SaveDecisions receives empty array []
  - SaveDecisions returns early (line 141-143: if len == 0, return nil)
  - No transaction created
  - Database: 0 rows in import_dedup_decisions
```

---

## Edge Case Scenarios

### E1: Empty Dedup Decisions Array
```
GIVEN: dedupDecisions: []
WHEN: Import executes
THEN:
  - No warnings
  - SaveDecisions called with empty array → returns nil immediately
  - No DB writes attempted
```

### E2: Null/Undefined dedupDecisions Field
```
GIVEN: Import request WITHOUT dedupDecisions field (omitted entirely)
WHEN: Import executes
THEN:
  - len(options.DedupDecisions) == 0
  - Block at line 590 skipped
  - No warnings, no SaveDecisions call
```

### E3: Large Batch (>50 decisions)
```
GIVEN: 150 valid dedup decisions
WHEN: SaveDecisions executes
THEN:
  - 3 batches: 50 + 50 + 50 (line 152: batchSize = 50)
  - All batches inserted within single transaction
  - tx.Commit() called once at end
  - Verify: 150 rows in import_dedup_decisions
```

### E4: Valid Decision Type Constants
```
GIVEN: Map of valid values (line 591-592):
  validDecisionTypes: {"within_file": true, "db_match": true}
  validActions: {"skip": true, "update": true, "import": true, "merge": true}
WHEN: Testing exact valid values
THEN:
  - "within_file" + "skip" → persisted ✅
  - "within_file" + "import" → persisted ✅
  - "db_match" + "update" → persisted ✅
  - "db_match" + "merge" → persisted ✅
```

---

## Test Data Templates

### Valid DedupDecisionInput
```json
{
  "keptExternalId": "SF_EXT_001",
  "discardedExternalId": "SF_EXT_002",
  "matchField": "email",
  "matchValue": "john@example.com",
  "decisionType": "within_file",
  "action": "skip",
  "matchedRecordId": ""
}
```

### Invalid DedupDecisionInput Examples
```json
// Invalid DecisionType (wrong case)
{"decisionType": "Within_File", "action": "skip", ...}

// Invalid Action
{"decisionType": "db_match", "action": "delete", ...}

// Empty values
{"decisionType": "", "action": "", ...}

// Typo in type
{"decisionType": "within-file", "action": "skip", ...}
```

---

## Verification Queries

### Check Import Job Created
```sql
SELECT id, org_id, entity_type, total_rows, created_count, updated_count, skipped_count, merged_count, failed_count
FROM import_jobs
WHERE id = '{import_id}';
```

### Check Dedup Decisions Persisted
```sql
SELECT id, decision_type, action, kept_external_id, discarded_external_id, match_field, match_value, matched_record_id
FROM import_dedup_decisions
WHERE import_job_id = '{import_id}'
ORDER BY created_at;
```

### Count Valid vs Invalid Decisions
```sql
-- Should match count of valid decisions submitted
SELECT COUNT(*) as persisted_count
FROM import_dedup_decisions
WHERE import_job_id = '{import_id}';

-- Check for unexpected data
SELECT DISTINCT decision_type FROM import_dedup_decisions WHERE import_job_id = '{import_id}';
SELECT DISTINCT action FROM import_dedup_decisions WHERE import_job_id = '{import_id}';
```

---

## Success Criteria

### P1 Critical
- ✅ All "no such table" errors return proper HTTP status codes (200/404, NOT 500)
- ✅ Zero raw database error messages visible in client responses
- ✅ Empty arrays returned instead of errors for missing tables
- ✅ Console logs contain error details for debugging

### P2 High-Priority
- ✅ Invalid decisionType/action values filtered out with warnings
- ✅ Transaction rollback verified on SaveDecisions failure
- ✅ Warnings array populated correctly in response
- ✅ Partial valid decisions persist even when some are invalid
- ✅ Import success independent of dedup persistence failure

### Edge Cases
- ✅ Empty dedup arrays handled without errors
- ✅ Large batches (>50) processed correctly in multi-batch transaction
- ✅ Panic recovery prevents import failure
- ✅ Case-sensitive validation enforced

---

## Test Execution Order

**Recommended sequence:**

1. **P1 Critical First** (blocks deployment if failing)
   - P1-1a, P1-1b, P1-1c (no such table)
   - P1-2a, P1-2b, P1-2c (error message sanitization)

2. **P2 Validation Logic**
   - P2-1a through P2-1d (invalid input handling)

3. **P2 Transaction Safety**
   - P2-2a (rollback verification)

4. **P2 Warning System**
   - P2-3a through P2-3d (warnings array)

5. **P2 Mixed Scenarios**
   - P2-4a, P2-4b (partial valid sets)

6. **Edge Cases**
   - E1 through E4 (boundary conditions)

---

## Notes for Test Implementation Team

### Fresh Tenant DB Setup
To simulate "no such table" scenarios:
1. Create new org via provisioning API
2. Verify `import_jobs` and `import_dedup_decisions` tables do NOT exist in tenant DB
3. Manually drop tables if needed: `DROP TABLE IF EXISTS import_jobs; DROP TABLE IF EXISTS import_dedup_decisions;`

### Verifying "no such table" Detection
Check helper function logic:
```go
// backend/internal/handler/dedup.go:20-25
func isNoSuchTableError(err error) bool {
    if err == nil {
        return false
    }
    errStr := strings.ToLower(err.Error())
    return strings.Contains(errStr, "no such table")
}
```

### Console Log Verification
When testing error paths, verify console output contains:
- `[DEDUP-API] ListImports error for org={orgID}: {error details}`
- `[DEDUP-API] GetJob error for org={orgID} job={jobID}: {error details}`
- `[DEDUP-API] GetDecisions error for org={orgID} job={jobID}: {error details}`
- `[IMPORT] Warning: skipped N dedup decisions with invalid decisionType or action`
- `[IMPORT] Warning: failed to persist dedup decisions: {error}`

### Transaction Rollback Testing
To verify tx.Rollback() behavior:
1. Use database triggers or constraints to force insert failure
2. Check row count before and after SaveDecisions call
3. Confirm NO partial data persisted (all-or-nothing guarantee)

---

**End of Test Scenarios Document**
