# GSD State: Quantico CRM

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-05)

**Core value:** Fast, secure multi-tenant CRM where customer data is protected
**Current focus:** Phase 14 - Import Integration (complete)

## Current Position

**Milestone:** v3.0 Deduplication System
**Phase:** 16 of 16 (Admin UI)
**Plan:** 1 of 4 complete
**Status:** In progress

**Last activity:** 2026-02-08 - Completed 16-01-PLAN.md (Foundation & API Client)

Progress: [█████░░░░░] 86% (18/21 plans)

## Performance Metrics

**Velocity:**
- Total plans completed: 49 (9 v1.0 + 22 v2.0 + 18 v3.0)
- Average duration: 3.9 min
- Total execution time: ~210 min

**By Milestone:**

| Milestone | Phases | Plans | Duration |
|-----------|--------|-------|----------|
| v1.0 Platform Update | 01-05 | 9 | ~40 min |
| v2.0 Security | 06-10 | 22 | ~91 min |
| v3.0 Deduplication | 11-16 | 18/21 | ~79 min |

*Updated after each plan completion*

## Accumulated Context

### Key Decisions

| Phase | Decision | Rationale | Date |
|-------|----------|-----------|------|
| 16-01 | ListAllPending sorts by highest_confidence DESC, detected_at DESC | Highest confidence alerts first per user decisions from Phase 12 context | 2026-02-08 |
| 16-01 | data-quality.ts re-exports utilities from dedup.ts | Avoid duplication of getBannerClass, formatConfidence functions | 2026-02-08 |
| 16-01 | PaginatedResponse generic type for all paginated endpoints | Consistent pagination interface across rules, alerts, merge history, scan jobs | 2026-02-08 |
| 15-03 | Notification message: "{Entity} scan complete" with NO duplicate count | Simplicity and consistency per CONTEXT.md locked decisions | 2026-02-08 |
| 15-03 | Failure notification: "{Entity} scan failed at X% -- click to retry" | Shows progress percentage at failure point for context | 2026-02-08 |
| 15-03 | SSE uses Fiber StreamWriter with fasthttp, 30s keepalive pings | Per RESEARCH.md pattern, prevents timeout on idle connections | 2026-02-08 |
| 15-03 | Non-blocking SSE broadcast (skip if channel full) | Prevents slow clients from blocking the broadcaster | 2026-02-08 |
| 15-03 | Scheduler starts after migration propagation | Ensures all migrations complete before scheduled jobs can run | 2026-02-08 |
| 15-02 | Service receives tenantDB from caller, doesn't manage DB connections | Handler/middleware already has tenant DB, avoids service needing org credentials | 2026-02-08 |
| 15-02 | Goroutine async execution for ExecuteScan | API returns immediately with jobID, scan runs in background | 2026-02-08 |
| 15-02 | 100ms sleep between chunks for WAL checkpoint window | Prevents WAL checkpoint starvation on long-running scans per RESEARCH.md | 2026-02-08 |
| 15-02 | RegisterOrgDB pattern for scheduler | Scheduled jobs run outside HTTP context, need cached tenant DB connection | 2026-02-08 |
| 15-01 | Scan schedules stored in master DB, jobs/checkpoints in tenant DB | Scheduler needs cross-org visibility to configure gocron jobs, but job data is org-specific | 2026-02-08 |
| 15-01 | One checkpoint per job with UNIQUE(job_id) constraint | Resume logic only needs latest checkpoint, reduces table size and query complexity | 2026-02-08 |
| 15-01 | Notifications auto-expire after 30 days | Prevents unbounded growth while supporting recent history checks, aligns with merge snapshot retention | 2026-02-08 |
| 14-03 | 'update' resolution overwrites existing record with import row values | Allows user to replace outdated database records with newer CSV data while preserving system fields | 2026-02-08 |
| 14-03 | Audit report tracks every resolution action | Compliance and debugging: shows what happened to each flagged row with reason | 2026-02-08 |
| 14-03 | Within-file skip indices calculated from group selections | All non-keeper rows in duplicate groups are excluded from import automatically | 2026-02-08 |
| 14-02 | High confidence (>=95%) defaults to Skip, medium defaults to Import Anyway | Prevents accidental duplicates while preserving data from uncertain matches | 2026-02-08 |
| 14-02 | Bulk actions don't override user decisions | Skip All/Import All only affect unresolved rows, respecting manual selections | 2026-02-08 |
| 14-02 | All clear message auto-proceeds after 2 seconds | Zero-duplicate case shows brief green toast then advances to import | 2026-02-08 |
| 14-02 | Merge button opens merge wizard in new tab | Preserves import wizard state while allowing merge workflow | 2026-02-08 |
| 14-01 | Reuse Phase 11 Detector for database duplicate detection | Ensures consistent matching behavior between real-time and import detection | 2026-02-08 |
| 14-01 | SHA-256 hash of normalized match field values for within-file grouping | Fast exact-duplicate detection within CSV; groups rows with identical normalized values | 2026-02-08 |
| 14-01 | Return empty results (not error) when no matching rules exist | Import can proceed without duplicate checking if rules aren't configured | 2026-02-08 |
| 13-04 | In-memory entityType filtering in History endpoint | MergeRepo.ListByOrg doesn't support entityType parameter, in-memory filtering simpler for low-traffic API | 2026-02-07 |
| 13-04 | Merge endpoints on protected (not admin) route group | Any authenticated user should merge records they access, consistent with bulk operations | 2026-02-07 |
| 13-04 | Audit logger early initialization in services section | MergeService constructor requires auditLogger, moved before handlers for clean dependency injection | 2026-02-07 |
| 13-03 | FK transfer happens BEFORE archiving duplicates | Per research Pitfall #2, archived records may cause FK constraint errors in some configurations | 2026-02-07 |
| 13-03 | Audit events consolidated in audit.go | Consistency with existing pattern where ALL audit event types live in entity/audit.go | 2026-02-07 |
| 13-02 | Related record discovery is metadata-driven (no hardcoded entity lists) | New entities automatically work, supports custom entities, single source of truth | 2026-02-07 |
| 13-02 | Simple completeness scoring (filled/total ratio) | Transparent and understandable, no hidden weighting, system fields excluded | 2026-02-07 |
| 13-01 | Archive columns added dynamically at merge time | Entity tables are dynamically created per-org, custom entities exist, not all will be merged | 2026-02-07 |
| 13-01 | Merge snapshots use JSON fields for flexibility | Supports any entity structure without type coupling | 2026-02-07 |
| 13-01 | 30-day undo window with expiration and consumed_at | Balance between data retention and undo capability, prevent double-undo | 2026-02-07 |
| 12-04 | Keep Both uses onCreateAnyway instead of onDismiss | Semantic difference: dismissed = "not duplicates", created_anyway = "keep both anyway" | 2026-02-07 |
| 12-04 | Alert wrapper reloads when recordId changes | Enables seamless navigation between records without manual refresh | 2026-02-07 |
| 12-04 | Silent failure on alert load errors | Alert display is non-critical enhancement, shouldn't disrupt user flow | 2026-02-07 |
| 12-03 | 404 on getPendingAlert returns null | "No alert" is expected state, not failure - simplifies client code | 2026-02-06 |
| 12-03 | Color-coded confidence tiers (red/yellow/blue) | Visual hierarchy: red=urgent, yellow=caution, blue=info | 2026-02-06 |
| 12-03 | Block mode requires typing "DUPLICATE" | Prevent accidental duplicate creation when block mode configured | 2026-02-06 |
| 12-02 | Async detection AFTER database save | Optimistic save pattern: Fast UX with immediate save, detection runs in background | 2026-02-06 |
| 12-02 | Use context.Background() with 30s timeout | Avoid Fiber context pooling issues documented in RESEARCH.md | 2026-02-06 |
| 12-02 | Panic recovery in detection goroutines | Prevent detection bugs from crashing the API server | 2026-02-06 |
| 12-02 | Default IsBlockMode to false | matching_rules table doesn't have block_mode column yet (future enhancement) | 2026-02-06 |
| 12-01 | Use INSERT OR REPLACE for alert upsert | Handle rapid edits where detection re-runs and replaces existing pending alert | 2026-02-06 |
| 12-01 | Include is_block_mode field in alerts | Frontend needs to know whether to show warning banner or blocking modal | 2026-02-06 |
| 12-01 | Alert endpoints not admin-only | Regular users need to see alerts on records they view and resolve them | 2026-02-06 |
| 12-01 | Store top 3 matches with record name | UI can display "Possible duplicates: John Smith, Jane Doe, Bob Johnson" | 2026-02-06 |
| 11-03 | Use go-phonetics for Soundex encoding | Well-maintained library, avoid reinventing Soundex | 2026-02-06 |
| 11-03 | Limit candidate queries to 1000 records | Prevent memory/performance issues from huge result sets | 2026-02-06 |
| 11-03 | Multi-strategy blocker with OR logic | Different strategies serve different use cases | 2026-02-06 |
| 11-03 | Process rules by priority with first-match deduplication | Prevent duplicate detection across rules | 2026-02-06 |
| 11-02 | Use Jaro-Winkler over Levenshtein for names | Better prefix weighting for person names | 2026-02-06 |
| 11-02 | Email weighting 80% local / 20% domain | Local part more important for person identity | 2026-02-06 |
| 11-02 | Binary phone matching after E.164 normalization | Phone numbers either match or don't, no fuzzy | 2026-02-06 |
| 11-02 | Skip empty fields without penalty | Don't penalize incomplete records | 2026-02-06 |
| 11-01 | Use DedupFieldConfig instead of FieldConfig | Avoid naming conflict with related_list.go | 2026-02-06 |
| 11-01 | Three-tier confidence system (high 0.95+, medium 0.85+, low) | Different merge workflows for different confidence levels | 2026-02-06 |
| 11-01 | Support cross-entity matching via target_entity_type | Enable Contact-Lead deduplication | 2026-02-06 |

_See PROJECT.md Key Decisions table for v1.0 and v2.0 decisions._

### Pending Todos

None.

### Blockers/Concerns

None.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 014 | Fix navigation tabs reprovision bugs (schema fix) | 2026-02-07 | 86ee548 | [014-fix-nav-tabs-reprovision](./quick/014-fix-nav-tabs-reprovision/) |
| 013 | Debug Guardare-Operations missing navigation tabs | 2026-02-07 | b1e47d2 | [013-debug-guardare-nav-options](./quick/013-debug-guardare-nav-options/) |
| 012 | CSV Import with field mapping and validation | 2026-02-05 | b7539e2 | [012-csv-import-with-field-mapping-validat](./quick/012-csv-import-with-field-mapping-validat/) |
| 011 | Edit page layout structure matching | 2026-02-05 | c893d1d | [011-edit-page-match-detail-page-layout-struc](./quick/011-edit-page-match-detail-page-layout-struc/) |
| 010 | Add Gmail Extension to Profile Settings | 2026-02-05 | 76c3efa | [010-extension-setup-page](./quick/010-extension-setup-page/) |

## Session Continuity

Last session: 2026-02-08 22:10:39
Stopped at: Completed 16-01-PLAN.md - Foundation & API Client
Resume file: None

---

*Updated: 2026-02-08 - Phase 16 in progress (Admin UI)*
