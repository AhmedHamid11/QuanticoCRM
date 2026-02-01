# Phase 4: Update Propagation - Context

**Gathered:** 2026-02-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Automatically update all org databases after deploy. Background job runs migrations on startup, applies schema changes additively to all orgs, with transaction-safe updates and failure tracking.

</domain>

<decisions>
## Implementation Decisions

### Job Execution
- Blocking on startup — app waits for all org migrations before accepting requests
- Sequential processing — one org at a time (safest, predictable)
- Migrations run automatically when backend starts — no manual trigger for initial run

### Failure Handling
- Skip and continue — if an org fails, log it and move to next org
- Per-org transaction — each org's migration is atomic; if it fails, that org stays at previous version
- Manual retry only — admin must trigger retry from admin panel for failed orgs
- No force update — only orgs behind current version can be updated

### Alerting & Observability
- Admin panel indicator for failed orgs — visible warning in admin
- Full context for failures: org name, error message, timestamp, version attempted
- Show both success and failures — full migration history for visibility
- Display on changelog page — add migration status section to existing changelog page

### Update Ordering
- No exclusions — all orgs get migrated for consistent platform version

### Claude's Discretion
- Org processing order (creation order, alphabetical, or other sensible default)
- Version-aware migrations vs idempotent re-run (pick based on existing patterns)
- Exact UI placement of migration status on changelog page
- Error message formatting and detail level

</decisions>

<specifics>
## Specific Ideas

- All orgs run the same version (auto-updated on deploy)
- CI/CD pipeline already deploys code; this phase handles the database migrations that support new features
- Migration status integrated into existing changelog page rather than a new page

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-update-propagation*
*Context gathered: 2026-02-01*
