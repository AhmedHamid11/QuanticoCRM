# Phase 10: Audit Infrastructure - Context

**Gathered:** 2026-02-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Track security events and enable compliance reporting. This includes:
- Authentication events (login success/failure, logout, password changes)
- User lifecycle events (CRUD, role changes, org settings changes)
- Authorization failures (403 responses with actor and resource)
- Append-only logs with tamper-evident integrity
- CI pipeline security scanning (gosec)

The audit system should be comprehensive for admins/compliance but **invisible to regular users** — they shouldn't notice it's there.

</domain>

<decisions>
## Implementation Decisions

### Admin UI for Audit Logs
- **Access control:** Org admins can view their own org's audit logs; platform admins can view all orgs
- **Display format:** Activity feed (timeline-style like GitHub activity) — more readable than dense tables
- **Search/filter:** Basic filters only — date range, event type, user — simple and fast
- **Export:** CSV and JSON export of filtered results for compliance reports

### Claude's Discretion
- Event capture scope beyond requirements (field changes, API calls, read access)
- Log storage approach (where, how long, rotation)
- Tamper-evidence implementation (hash chaining, checksums, verification frequency)
- Pagination, real-time updates, and other UI polish
- Activity feed item design and information density
- gosec integration specifics and threshold tuning

</decisions>

<specifics>
## Specific Ideas

- **User impact philosophy:** Maximum safety with minimal end-user disruption — audit system should be invisible to regular users
- Activity feed should feel similar to GitHub's activity timeline — clean, scannable, not overwhelming

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-audit-infrastructure*
*Context gathered: 2026-02-04*
