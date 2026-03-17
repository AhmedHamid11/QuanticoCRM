# Dedup Scan Fix — Full Context

**Date:** 2026-02-20
**Status:** Deployed to production

---

## Problem Statement

The dedup scan on 33,794 Contact records had three persistent issues:
1. **Stuck at ~22k/33k** — scan stopped making progress after processing ~22,000 records
2. **0 duplicates found** — scan completed (or stalled) without detecting any duplicates
3. **Extremely slow progress updates** — status field updated infrequently

---

## Root Causes (Identified by Agent Team)

### Cause 1: OFFSET-based pagination (stuck at 22k)

**File:** `backend/internal/service/scan_job.go` (fetchChunk)

```sql
-- OLD: Turso must scan 22,000 rows before returning the next 500
SELECT * FROM contacts WHERE org_id = ? ORDER BY id LIMIT 500 OFFSET 22000

-- NEW: O(1) regardless of position in dataset
SELECT * FROM contacts WHERE org_id = ? AND id > ? ORDER BY id LIMIT 500
```

At OFFSET 22,000 on Turso HTTP, each query took 15-30+ seconds. The 30-second chunk timeout fired, causing the scan to fail and appear "stuck."

### Cause 2: Per-record DB queries (fundamental bottleneck)

**File:** `backend/internal/service/scan_job.go` (processChunk)

The scan did **per-record** `FindCandidates` + `fetchRecordsBatch` queries:
- 500 records/chunk x 2 queries/record = ~1,000 Turso HTTP requests/chunk
- At ~100ms/request = **100 seconds per chunk** (exceeds 30s timeout)
- Railway logs confirmed: `error code 502` on FindCandidates queries

**Fix:** Replaced with `BatchFindCandidates` (1 query loads ALL candidates for the entire chunk), then score all pairs in memory. Same pattern already proven in `import_duplicates.go`.

### Cause 3: snake_case/camelCase mismatch (0 duplicates)

**File:** `backend/internal/service/scan_job.go` (old fetchChunk)

The old `fetchChunk` manually scanned rows and returned snake_case keys (`email_address`, `last_name`). But `GenerateBlockingKeys` and the scorer expected camelCase keys (`emailAddress`, `lastName`). Every field lookup returned empty string, producing 0 blocking keys and 0 match scores.

**Fix:** Replaced manual row scanning with `util.ScanRowsToMaps` which auto-converts snake_case to camelCase.

### Cause 4: Backfill WHERE clause (re-processing loop)

**File:** `backend/internal/dedup/provision.go` (BackfillBlockingKeysForEntity)

```sql
-- OLD: Records with no useful data matched again every batch (18k/17k bug)
WHERE dedup_email_domain IS NULL OR (dedup_email_domain = '' AND dedup_last_name_soundex = '' ...)

-- NEW: Only match unprocessed records
WHERE dedup_email_domain IS NULL
```

### Cause 5: In-memory backfill cache (missed new records)

**File:** `backend/internal/dedup/provision.go` (entityBackfilled sync.Map)

The `entityBackfilled` cache persisted for the process lifetime. If records were added between scans, the second scan skipped backfill entirely (cache hit), leaving new records without blocking keys.

**Fix:** Added `ClearBackfillCache()` called before each scan.

---

## Commits (chronological)

| Commit | Description |
|--------|-------------|
| `71d9d5f` | Fix backfill re-processing loop + slow progress updates (batch 5000->500) |
| `b5b9ce4` | Fix 0 duplicates: snake_case/camelCase mismatch + cache rules before scan |
| `2fdf559` | Batch-fetch candidates in detector + reduce FindCandidates LIMIT 1000->100 |
| `eb77704` | Replace OFFSET pagination with cursor-based keyset pagination |
| `8270d69` | Batch candidate lookup (1000 Turso requests/chunk -> 2) + clear backfill cache |

---

## Files Modified

| File | What Changed |
|------|-------------|
| `backend/internal/service/scan_job.go` | Cursor pagination, batch processChunk, cached rules, ScanRowsToMaps in fetchChunk, removed N+1 FetchRecordAsMap |
| `backend/internal/dedup/provision.go` | Batch size 5000->500, WHERE clause fix, ClearBackfillCache() |
| `backend/internal/dedup/detector.go` | CheckForDuplicatesWithRules (pre-fetched rules), fetchRecordsBatch, RecordName in DuplicateMatch |
| `backend/internal/dedup/blocker.go` | FindCandidates LIMIT 1000->100 |

---

## Architecture: Scan Pipeline (After Fix)

```
ExecuteScan(ctx, tenantDB, orgID, "Contact", "manual", nil)
  |
  v
executeChunkedScan()
  |
  +-- Phase 1: ClearBackfillCache() + BackfillBlockingKeysForEntity()
  |     - Populates dedup_* columns for records with NULL keys
  |     - Batches of 500 in transactions (single Turso HTTP request per batch)
  |     - Emits "Preparing data: X/Y records indexed" status
  |
  +-- Pre-fetch: ListEnabledRules() once (not per-record)
  |
  +-- Phase 2: Chunked scan loop
        |
        +-- fetchChunk(ctx, tenantDB, tableName, orgID, 500, lastID)
        |     - Cursor-based: WHERE id > lastID ORDER BY id LIMIT 500
        |     - ScanRowsToMaps -> camelCase keys
        |
        +-- processChunk(ctx, tenantDB, orgID, "Contact", records, rules)
        |     1. GenerateBlockingKeys for ALL 500 records (in-memory)
        |     2. BatchFindCandidates: ONE query loads all candidates
        |     3. Build in-memory index by blocking key values
        |     4. For each record, find matching candidates from index
        |     5. ScoreRecord in-memory (no DB queries)
        |     6. Upsert PendingDuplicateAlert for matches
        |
        +-- Update progress + save checkpoint (lastID cursor)
        +-- 100ms sleep between chunks
```

**DB queries per 500-record chunk:** ~2-3 (BatchFindCandidates + alert upserts)
**Previous:** ~1,000+ (FindCandidates + fetchRecordsBatch per record)

---

## Key Data Flow

### Field Name Conversion

| DB Column (snake_case) | After ScanRowsToMaps (camelCase) | Used By |
|------------------------|----------------------------------|---------|
| `last_name` | `lastName` | GenerateBlockingKeys, Scorer |
| `email_address` | `emailAddress` | GenerateBlockingKeys, Scorer |
| `phone_number` | `phoneNumber` | GenerateBlockingKeys |
| `first_name` | `firstName` | Scorer |

### Blocking Keys

| Key Column | Source Field | Algorithm |
|------------|-------------|-----------|
| `dedup_last_name_soundex` | lastName or name | Soundex encoding |
| `dedup_last_name_prefix` | lastName or name | First 3 chars normalized |
| `dedup_email_domain` | emailAddress or email | Domain extraction |
| `dedup_phone_e164` | phoneNumber or phone | E.164 normalization |

### Default Matching Rule (seeded by provisioning)

```json
{
  "name": "Contact Email Match",
  "entityType": "Contact",
  "threshold": 0.70,
  "fieldConfigs": [
    {"fieldName": "emailAddress", "weight": 60, "algorithm": "email", "threshold": 0.95, "exactMatchBoost": true},
    {"fieldName": "lastName", "weight": 25, "algorithm": "jaro_winkler", "threshold": 0.88},
    {"fieldName": "firstName", "weight": 15, "algorithm": "jaro_winkler", "threshold": 0.85}
  ]
}
```

---

## Local Test Results (from agent team)

| Test Org | Contacts | Duplicates Found | Time |
|----------|----------|-----------------|------|
| Import Test (real data) | 701 | 414 | 1.0s |
| TestCorp (synthetic) | 2,520 | 244,951 | 14.0s |

Confirms the scan logic is correct. Production stalling was purely a Turso HTTP latency issue.

---

## Code Reuse: Scan vs Import

Both paths now share the same pattern:

| Concern | Scan Path | Import Path |
|---------|-----------|-------------|
| Candidate lookup | `BatchFindCandidates` | `BatchFindCandidates` |
| Key generation | `GenerateBlockingKeys` | `GenerateBlockingKeys` |
| Scoring | `ScoreRecord` | `ScoreRecord` |
| Rules | Pre-fetched once | Pre-fetched once |
| Key indexing | In-memory map by key values | In-memory map by key values |

The scan path additionally handles:
- Backfill of blocking keys (import skips this)
- Checkpoint/resume from failure
- Progress events (SSE + DB polling)
- PendingDuplicateAlert creation

---

## Potential Future Issues

1. **BatchFindCandidates LIMIT 10000** — if an org has >10k candidates matching a chunk's blocking keys, some may be missed. Increase limit if needed.
2. **Alert upserts still per-match** — each duplicate alert is a separate Turso request. Could batch into a transaction if alert volume is high.
3. **Common email domains (gmail.com)** — many contacts sharing the same domain produces large candidate sets. The in-memory scoring handles this efficiently but memory usage scales with candidate count.
4. **Scorer skips empty fields** — if contacts lack email AND lastName AND firstName, the score is 0.0 regardless of other similarities. Consider adding more scoring fields if needed.
