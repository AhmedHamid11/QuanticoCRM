---
phase: 04-update-propagation
plan: 01
subsystem: backend-platform
tags: [migration-tracking, database, multi-tenant]

requires:
  - 01-01 # Platform versioning infrastructure
  - 01-02 # Version API endpoints

provides:
  - migration_runs table schema
  - MigrationRun entity types
  - MigrationRepo CRUD operations

affects:
  - 04-02 # Will use MigrationRepo for propagation service
  - 04-03 # Admin UI will query failed runs via repo

tech-stack:
  added:
    - PrefixMigrationRun (0Mr) in sfid package
  patterns:
    - DBConn interface for repository layer (Turso compatibility)
    - RFC3339 timestamp format for consistency
    - Null-safe SQL scanning for optional fields

key-files:
  created:
    - FastCRM/fastcrm/migrations/043_migration_runs.sql
    - FastCRM/fastcrm/backend/internal/entity/migration.go
    - FastCRM/fastcrm/backend/internal/repo/migration.go
  modified:
    - FastCRM/fastcrm/backend/internal/sfid/sfid.go

decisions:
  - name: Store migration runs in master database
    rationale: Centralized tracking across all orgs, admin visibility
    impact: Migration status queries don't hit tenant databases
  - name: Track most recent run per org
    rationale: Admin UI needs to show current failure state
    impact: GetFailedRuns uses MAX(started_at) subquery
  - name: Three-state status model (running/success/failed)
    rationale: Enables progress tracking and failure detection
    impact: Check constraint ensures valid status values

metrics:
  duration: 8 minutes
  completed: 2026-02-01
---

# Phase 04 Plan 01: Migration Tracking Infrastructure Summary

**One-liner:** Migration run persistence with status tracking, error logging, and org-level query support

## What Was Built

Created the foundation for tracking schema migration propagation across all tenant organizations:

1. **Database Schema (043_migration_runs.sql)**
   - Tracks each migration attempt per org
   - Three-state status: running → success/failed
   - Stores error messages for debugging failed migrations
   - Indexes on org_id, status, started_at for efficient querying

2. **Entity Types (entity/migration.go)**
   - `MigrationRun`: Individual org migration record
   - `PropagationResult`: Multi-org propagation summary
   - `MigrationStatusResponse`: Admin API response format
   - `FailedOrg`: Failed migration display info

3. **Repository Layer (repo/migration.go)**
   - `CreateRun`: Insert new migration run
   - `UpdateRunStatus`: Mark completion with status/error
   - `GetFailedRuns`: Most recent failure per org
   - `GetRunsByOrg`: Migration history for specific org
   - `GetLastRunTime`: Find last propagation timestamp

## Key Implementation Decisions

**Status Tracking Model:**
- Chose three states instead of two to support in-progress visibility
- Running → Success/Failed state transitions only
- Error messages stored for debugging without separate error table

**Query Optimization:**
- GetFailedRuns uses subquery with MAX(started_at) to find latest per org
- Avoids returning historical failures that were later resolved
- Admin UI shows current failure state, not full history

**ID Generation:**
- Added PrefixMigrationRun (0Mr) to sfid package
- Consistent with existing entity ID patterns
- 18-character Salesforce-style IDs

**Time Handling:**
- RFC3339 format matches version.go pattern
- UTC normalization in CreateRun
- Null-safe scanning for optional completed_at

## Deviations from Plan

None - plan executed exactly as written.

## Testing Approach

Compilation verified:
- `go build ./...` passes without errors
- Entity package exports all types
- Repo package compiles with DBConn interface

Schema validated:
- SQL syntax correct (CREATE TABLE, CHECK constraint)
- Foreign key references organizations table
- Indexes on expected columns

## Commits

| Hash    | Message |
|---------|---------|
| 99d6371 | feat(04-01): create migration_runs table schema |
| 7972702 | feat(04-01): create migration entity types |
| a182935 | feat(04-01): create MigrationRepo with CRUD operations |

## Next Phase Readiness

**Ready for 04-02 (Propagation Service):**
- ✅ MigrationRepo available for status persistence
- ✅ Entity types defined for service layer
- ✅ Database schema ready for inserts/updates

**Ready for 04-03 (Admin UI):**
- ✅ GetFailedRuns query returns admin-displayable data
- ✅ MigrationStatusResponse type matches API contract
- ✅ GetLastRunTime provides "last propagation" timestamp

**Potential Issues:**
- Migration 043 not yet applied to master database
- Need migration runner to execute schema before service can persist
- Consider adding GetRunsByVersion for version-specific history

## Files Changed

```
CREATE  migrations/043_migration_runs.sql (18 lines)
CREATE  internal/entity/migration.go (46 lines)
CREATE  internal/repo/migration.go (166 lines)
MODIFY  internal/sfid/sfid.go (+1 constant)
```

## Related Documentation

- Plan: `.planning/phases/04-update-propagation/04-01-PLAN.md`
- Context: `.planning/phases/04-update-propagation/CONTEXT.md`
- Next: `.planning/phases/04-update-propagation/04-02-PLAN.md` (propagation service)
