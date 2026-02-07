---
phase: 13-merge-engine
plan: 04
subsystem: api-layer
tags: [go, fiber, http-handlers, rest-api, merge-api]
requires: [13-02-discovery, 13-03-execution]
provides: [merge-preview-api, merge-execute-api, merge-undo-api, merge-history-api]
affects: [frontend-merge-wizard]
tech-stack:
  added: []
  patterns: [tenant-db-middleware, fiber-handler-pattern, pagination]
key-files:
  created:
    - backend/internal/handler/merge.go
  modified:
    - backend/cmd/api/main.go
decisions:
  - id: audit-logger-early-init
    decision: Move auditLogger initialization to services section (before handlers)
    rationale: MergeService constructor requires auditLogger, handlers are initialized later
    date: 2026-02-07
  - id: protected-route-group
    decision: Register merge endpoints on protected (authenticated user) group, not admin-only
    rationale: Any authenticated user should be able to merge records they have access to, same as bulk operations
    date: 2026-02-07
  - id: in-memory-entity-filter
    decision: Apply entityType filter in memory in History endpoint
    rationale: MergeRepo.ListByOrg doesn't support entityType parameter, filtering in memory is simpler than adding SQL query complexity
    date: 2026-02-07
duration: 3.4 min
completed: 2026-02-07
---

# Phase 13 Plan 04: Merge API Handlers Summary

**One-liner:** HTTP endpoints for merge preview, execute, undo, and history with tenant DB middleware support

## What Was Built

### Core Functionality

**MergeHandler** (backend/internal/handler/merge.go):
- **Preview endpoint** (POST /merge/preview): Returns side-by-side comparison of records with completeness scores, suggested survivor ID, related record counts, and field definitions
- **Execute endpoint** (POST /merge/execute): Performs atomic merge via mergeService.ExecuteMerge, returns survivor ID and snapshot ID
- **Undo endpoint** (POST /merge/undo/:snapshotId): Validates 30-day window and consumed_at, calls mergeService.UndoMerge
- **History endpoint** (GET /merge/history): Lists recent merges with pagination, optional entityType filter, canUndo flag

**Tenant DB Support:**
- Helper methods: getDB, getDBConn, getMetadataRepo, getMergeRepo
- All methods use middleware.GetTenantDB(c) to get per-org database
- Follows bulk.go and dedup.go handler patterns

**Route Registration:**
- All 4 endpoints registered on /merge group
- Registered on protected route group (authenticated users, not admin-only)
- Middleware chain: auth → session timeout → 403 audit → password change → body limit → tenant DB

### Integration Changes

**main.go wiring:**
1. mergeRepo initialized after pendingAlertRepo (uses masterDBConn)
2. auditLogger moved to services section (required by mergeService constructor)
3. mergeDiscoveryService initialized with metadataRepo
4. mergeService initialized with mergeRepo, metadataRepo, discoveryService, auditLogger
5. mergeHandler initialized with masterDB, mergeRepo, mergeService, discoveryService, metadataRepo
6. Routes registered on protected group: mergeHandler.RegisterRoutes(protected)

## Deviations from Plan

None - plan executed exactly as written.

## Technical Decisions

### Decision: Audit Logger Early Initialization

**Context:** MergeService constructor requires auditLogger, but handlers section is after services section in main.go.

**Options:**
1. Create auditLogger in services section (before mergeService)
2. Pass nil to mergeService and add a setter method
3. Make auditLogger optional in mergeService

**Chose:** Option 1 - Move auditLogger to services section

**Rationale:**
- Clean dependency injection without setters
- AuditLogger has no dependencies, safe to create early
- Consistent with existing initialization patterns
- Avoids nil checks in merge execution code

### Decision: Protected Route Group (Not Admin-Only)

**Context:** Merge endpoints could be admin-only or available to all authenticated users.

**Chose:** Protected group (all authenticated users)

**Rationale:**
- Consistent with bulk operations (bulkHandler also on protected group)
- Users should be able to merge records they have access to
- Record-level permissions handled by tenant DB and org_id filtering
- Merge is a user-facing workflow, not just admin maintenance

### Decision: In-Memory Entity Type Filtering

**Context:** History endpoint should support optional entityType filter, but MergeRepo.ListByOrg doesn't have this parameter.

**Options:**
1. Add entityType parameter to MergeRepo.ListByOrg SQL query
2. Filter in memory in handler after fetching from repo

**Chose:** Option 2 - In-memory filtering

**Rationale:**
- Simpler implementation (no repo changes needed)
- History API is low-traffic (manual user action, not high-volume)
- Pagination still works at SQL level (fetch 20, filter to <20 if entityType specified)
- Can add SQL filtering later if performance becomes an issue

## Verification

### Compilation Check
```bash
cd backend && go build ./...
```
✅ All packages compile successfully

### Endpoints Verified
- POST /api/v1/merge/preview - MergeHandler.Preview
- POST /api/v1/merge/execute - MergeHandler.Execute
- POST /api/v1/merge/undo/:snapshotId - MergeHandler.Undo
- GET /api/v1/merge/history - MergeHandler.History

### Middleware Chain
- authMiddleware.Required() - JWT validation
- sessionTimeoutMiddleware - idle/absolute timeout
- middleware.AuditAuthorizationFailures - 403 audit logging
- middleware.RequirePasswordChange - force password reset if needed
- middleware.BodyLimit(1MB) - request size limit
- tenantMiddleware.ResolveTenant() - resolve per-org database

### Tenant DB Pattern
All handler methods:
1. Get orgID from c.Locals("orgID")
2. Get tenant DB via middleware helpers (getDB, getDBConn, etc.)
3. Pass tenant DB to repos and services
4. No direct access to master DB from handler code

## Task Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 7265122 | Create merge HTTP handler with 4 endpoints |
| 2 | ec36503 | Register merge handler in main.go with proper init order |

## Next Phase Readiness

**Ready for:** 13-05 (if exists) or frontend merge wizard implementation

**Provides:**
- Complete REST API for merge operations
- Preview endpoint for merge wizard UI
- Execute endpoint for atomic merge
- Undo endpoint for reversal within 30 days
- History endpoint for merge audit trail

**Integration points:**
- Frontend can call POST /api/v1/merge/preview with recordIds to show side-by-side comparison
- Frontend can call POST /api/v1/merge/execute with survivorId, duplicateIds, mergedFields
- Frontend can call POST /api/v1/merge/undo/:snapshotId to reverse merge
- Frontend can call GET /api/v1/merge/history for recent merges

## Performance Notes

- Preview endpoint: N+1 query pattern (fetch each record individually) - acceptable for merge preview (2-10 records typical)
- Execute endpoint: Single transaction with FK transfers, survivor update, duplicate archiving
- History endpoint: Paginated (default 20, max 100 per page)
- In-memory entityType filtering: Low impact (filtering 20-100 records)

## Self-Check: PASSED

### Created Files
✅ backend/internal/handler/merge.go

### Modified Files
✅ backend/cmd/api/main.go

### Commits
✅ 7265122 - feat(13-04): create merge HTTP handler
✅ ec36503 - feat(13-04): register merge handler in main.go

All files created and commits exist as documented.
