---
phase: 14-import-integration
plan: 01
subsystem: import
tags: [duplicate-detection, csv-import, deduplication, backend-api]
dependencies:
  requires:
    - 11-02-SUMMARY.md # Detector.CheckForDuplicates() for database matching
    - 11-03-SUMMARY.md # Jaro-Winkler scorer for fuzzy matching
  provides:
    - POST /entities/:entity/import/csv/check-duplicates API endpoint
    - ImportDuplicateService for database + within-file detection
    - Import duplicate entity types (ImportDuplicateMatch, ImportDuplicateGroup)
  affects:
    - 14-02 # Frontend duplicate review step will consume this API
    - 14-03 # Import execution will use ImportResolution type
tech-stack:
  added: []
  patterns:
    - Reuse of Phase 11 Detector for consistent duplicate detection
    - SHA-256 hashing of normalized field values for within-file grouping
    - Multi-source duplicate detection (database + within-file)
key-files:
  created:
    - backend/internal/entity/import_duplicate.go
    - backend/internal/service/import_duplicates.go
  modified:
    - backend/internal/handler/import.go
    - backend/cmd/api/main.go
decisions:
  - what: Reuse existing Detector from Phase 11 for database duplicate detection
    why: Ensures consistent matching behavior between real-time and import detection
    alternatives: []
  - what: Use SHA-256 hash of normalized match field values for within-file grouping
    why: Fast exact-duplicate detection within CSV; groups rows with identical normalized values
    alternatives: []
  - what: Return empty results (not error) when no matching rules exist
    why: Import can proceed without duplicate checking if rules aren't configured
    alternatives: []
  - what: Default KeepIndex to first row in duplicate groups
    why: Per research recommendation - first occurrence is typically most complete
    alternatives: []
metrics:
  duration: 3m
  completed: 2026-02-08
---

# Phase 14 Plan 01: Import Duplicate Detection Service Summary

**One-liner:** Backend duplicate detection service for CSV import using Phase 11 Detector for database matches and SHA-256 hashing for within-file duplicate grouping.

## What Was Built

Created the backend duplicate detection service and API endpoint that the import wizard will call to check for duplicates both against existing database records and within the CSV file itself.

### Components Created

1. **Entity Types** (`backend/internal/entity/import_duplicate.go`):
   - `ImportDuplicateMatch`: Import row matched to existing database record
   - `ImportMatchCandidate`: Alternative matches (lower scores)
   - `ImportDuplicateGroup`: Rows within CSV that duplicate each other
   - `DuplicateCheckResult`: Combined database + within-file detection results
   - `ImportResolution`: User decision for flagged rows (for Plan 14-03)

2. **ImportDuplicateService** (`backend/internal/service/import_duplicates.go`):
   - `CheckDuplicates()`: Main method combining database + within-file detection
   - `detectDatabaseDuplicates()`: Calls Detector.CheckForDuplicates() for each import row
   - `detectWithinFileDuplicates()`: Groups rows by SHA-256 hash of normalized match fields
   - `computeRowHash()`: Normalizes field values (lowercase, trim, strip phone digits) and hashes
   - `extractRecordName()`: Gets display name from record (tries "name", then firstName+lastName)

3. **API Endpoint** (`backend/internal/handler/import.go`):
   - `POST /entities/:entity/import/csv/check-duplicates`
   - Accepts uploaded CSV file and optional column mapping
   - Parses CSV, calls ImportDuplicateService.CheckDuplicates()
   - Returns DuplicateCheckResult with database matches and within-file groups

4. **Wiring** (`backend/cmd/api/main.go`):
   - Created ImportDuplicateService using existing detector instance
   - Injected into ImportHandler constructor
   - Route registered in ImportHandler.RegisterRoutes()

## Technical Decisions

### Database Duplicate Detection

**Reuses Phase 11 Detector.CheckForDuplicates()** for each import row:
- Consistent matching behavior between real-time (on record save) and import (bulk CSV)
- Leverages existing blocking strategies, Jaro-Winkler scoring, confidence tiers
- Returns top match as primary, remaining matches as OtherMatches (allows user to switch)

**Match Data Structure:**
- `ImportRowIndex`: 0-based row index in CSV
- `MatchedRecordID` + `MatchedRecord`: Top database match
- `ConfidenceScore` + `ConfidenceTier`: Pre-computed for frontend
- `MatchedFields`: Fields that contributed to match (for UI highlighting)
- `OtherMatches`: Alternative database matches (user can choose different match)

### Within-File Duplicate Detection

**Hash-based grouping** for exact duplicates within CSV:
1. Get enabled matching rules for entity
2. For each row, extract all match field values
3. Normalize values (lowercase, trim; strip non-digits for phone)
4. Create sorted concatenation: `fieldName:normalizedValue|...`
5. SHA-256 hash the concatenation
6. Group rows by hash (2+ rows = duplicate group)

**Group Data Structure:**
- `GroupID`: SHA-256 hash (unique identifier)
- `RowIndices`: List of 0-based row indices in group
- `Rows`: Full row data for each duplicate
- `KeepIndex`: Defaults to first row index (per research recommendation)

### Empty Rules Handling

**When no matching rules exist, return empty results (not error):**
- `DatabaseMatches: []`
- `WithinFileGroups: []`
- `FlaggedRows: 0`
- Import wizard can proceed without duplicate checking

### Flagged Rows Calculation

**Unique row indices from both detection types:**
- If row 3 matches database AND is in within-file group → counted once
- `FlaggedRows = len(unique row indices needing review)`

## Task Commits

| Task | Name                                       | Commit  | Files                                                                                  |
| ---- | ------------------------------------------ | ------- | -------------------------------------------------------------------------------------- |
| 1    | Create entity types                        | f10f4b8 | backend/internal/entity/import_duplicate.go                                            |
| 2    | Create service and wire endpoint           | 9fe9f26 | backend/internal/service/import_duplicates.go, backend/internal/handler/import.go, backend/cmd/api/main.go |

## Verification Results

All verification checks passed:

1. ✅ `cd backend && go build ./...` compiles without errors
2. ✅ `grep -rn "check-duplicates" backend/` shows route registration
3. ✅ `grep -rn "ImportDuplicateService" backend/` shows service creation in main.go and usage in handler
4. ✅ `grep -rn "detector.CheckForDuplicates" backend/internal/service/import_duplicates.go` confirms reuse of Phase 11 detector

## Success Criteria

- [x] ImportDuplicateService created with CheckDuplicates method
- [x] Database duplicate detection reuses existing Detector.CheckForDuplicates() (Phase 11)
- [x] Within-file detection uses SHA-256 hash of normalized match field values
- [x] CheckDuplicates endpoint registered and handler implemented
- [x] ImportHandler constructor updated with new dependency
- [x] main.go wires ImportDuplicateService using existing detector instance
- [x] Backend compiles successfully

## Next Phase Readiness

**Ready for Plan 14-02 (Frontend Duplicate Review Step):**
- API endpoint `/entities/:entity/import/csv/check-duplicates` ready
- Response format includes all data needed for UI:
  - Database matches with confidence scores, matched fields, alternative matches
  - Within-file groups with default keep selection
  - Total rows and flagged rows count

**Ready for Plan 14-03 (Import Execution with Resolutions):**
- `ImportResolution` type defined for user decisions
- Structure prepared for "skip", "update", "import", "merge" actions

**No blockers for next plans.**

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

All created files exist:
- backend/internal/entity/import_duplicate.go ✅
- backend/internal/service/import_duplicates.go ✅

All commits exist:
- f10f4b8 ✅
- 9fe9f26 ✅
