# GSD State: Quantico CRM

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-09)

**Core value:** Fast, secure multi-tenant CRM where customer data is protected
**Current focus:** v4.0 Salesforce Merge Integration (Phase 19 next)

## Current Position

**Milestone:** v4.0 Salesforce Merge Integration
**Phase:** 18 of 19 (Rate Limiting & Error Handling) — COMPLETE
**Plan:** Complete
**Status:** Phase 18 verified, ready for Phase 19

**Last activity:** 2026-02-10 — Phase 18 complete (2/2 plans, verified 5/5 criteria)

Progress: [█████████░] 100% of v1.0-v3.0 (53/53 plans), v4.0 in progress (7/7 plans for P17-18 complete)

## Performance Metrics

**Velocity:**
- Total plans completed: 60 (9 v1.0 + 22 v2.0 + 22 v3.0 + 7 v4.0)
- Average duration: 3.5 min
- Total execution time: ~253 min

**By Milestone:**

| Milestone | Phases | Plans | Duration |
|-----------|--------|-------|----------|
| v1.0 Platform Update | 01-05 | 9 | ~40 min |
| v2.0 Security | 06-10 | 22 | ~91 min |
| v3.0 Deduplication | 11-16 | 22 | ~96 min |
| v4.0 Salesforce Integration | 17-19 | 7/7 (P17-18) | ~17.3 min |

**Recent Plan Execution:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 18-02 | 4.3 min | 2 | 5 |
| Phase 18-01 | 1.8 min | 2 | 3 |
| Phase 17-04 | 4.4 min | 2 | 3 |

## Accumulated Context

### Key Decisions

_All milestone decisions archived. See PROJECT.md Key Decisions table for cumulative record._

**Recent (v4.0):**
- Staging object pattern chosen for Salesforce integration (external systems insert data, Salesforce Apex processes merges)
- OAuth 2.0 with proactive token refresh (before 5-min expiry) to avoid mid-batch failures
- 7-year audit log retention for SOX compliance
- 80% API capacity threshold (80,000 of 100,000 calls/day) before automatic pause
- AES-256-GCM encryption for OAuth tokens with environment variable key storage (17-01)
- Master DB for OAuth config, tenant DB for sync job history (17-01)
- Use golang.org/x/oauth2 for token lifecycle (industry-standard, auto-refresh) (17-02)
- State-based CSRF protection for OAuth callback (base64 encoded orgID + random bytes) (17-02)
- OAuth callback as public route (no auth cookie required, redirected from Salesforce) (17-02)
- Support Salesforce sandbox via SALESFORCE_AUTH_URL and SALESFORCE_TOKEN_URL env vars (17-02)
- Load field mappings from database, fallback to field name if no mapping exists (17-03)
- Implement 15-to-18 character Salesforce ID checksum algorithm in Go (17-03)
- Batch size limit of 200 instructions per batch (Salesforce Composite API limit) (17-03)
- Real-time batch ID format QTC-YYYYMMDD-RT-NNN to distinguish from scheduled batches (17-03)
- Async delivery with per-org concurrency control (max 1 concurrent delivery per org) (17-04)
- HTTP 202 pattern for immediate job ID return with background execution (17-04)
- Idempotency key format {orgID}-{batchID} to prevent duplicate deliveries on retry (17-04)
- Basic retry for 5xx errors (max 2 attempts, 2s delay) - Phase 18 adds exponential backoff (17-04)
- 24-hour sliding window for API usage tracking (not fixed daily reset) (18-01)
- 25-hour cleanup window for stale usage records (1-hour buffer for boundary safety) (18-01)
- Graceful degradation for missing api_usage_log table in tenant DBs (18-01)
- Per-job API call tracking via api_calls_made column on sync_jobs (18-01)
- Exponential backoff: 5s initial, 2x multiplier, 40s max, 50% jitter, max 5 retries (18-02)
- Force flag on ManualTrigger bypasses quota check for emergency overrides (18-02)
- Record API usage on both success and failure deliveries (18-02)
- HTTP 429 with usage/threshold/hint for quota violations (18-02)

### Pending Todos

None.

### Blockers/Concerns

None.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 017 | Fix Review Queue record name display (show names not UUIDs) | 2026-02-09 | b17c317 | [017-fix-review-queue-record-name-display](./quick/17-fix-review-queue-record-name-display/) |
| 016 | Fix audit bugs and visual issues (13 items) | 2026-02-09 | d8fcc56 | [016-fix-audit-bugs-and-visual-issues](./quick/016-fix-audit-bugs-and-visual-issues/) |
| 014 | Fix navigation tabs reprovision bugs (schema fix) | 2026-02-07 | 86ee548 | [014-fix-nav-tabs-reprovision](./quick/014-fix-nav-tabs-reprovision/) |
| 013 | Debug Guardare-Operations missing navigation tabs | 2026-02-07 | b1e47d2 | [013-debug-guardare-nav-options](./quick/013-debug-guardare-nav-options/) |
| 012 | CSV Import with field mapping and validation | 2026-02-05 | b7539e2 | [012-csv-import-with-field-mapping-validat](./quick/012-csv-import-with-field-mapping-validat/) |
| 011 | Edit page layout structure matching | 2026-02-05 | c893d1d | [011-edit-page-match-detail-page-layout-struc](./quick/011-edit-page-match-detail-page-layout-struc/) |
| 010 | Add Gmail Extension to Profile Settings | 2026-02-05 | 76c3efa | [010-extension-setup-page](./quick/010-extension-setup-page/) |

## Session Continuity

Last session: 2026-02-10
Stopped at: Phase 18 complete and verified. Phase 19 not yet planned.
Resume file: None

---

*Updated: 2026-02-10 — Phase 18 complete (verified 5/5 criteria)*
