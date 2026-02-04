# Phase 09: Session Management - Context

**Gathered:** 2026-02-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Control user session lifecycle with configurable timeout enforcement, CSRF protection for state-changing requests, and comprehensive integration tests proving tenant data isolation. This phase builds on the token architecture from Phase 07.

</domain>

<decisions>
## Implementation Decisions

### Inactivity Timeout UX
- Countdown warning appears 5 minutes before timeout as a toast notification (non-blocking, corner position)
- Generic message: "Session expires in X:XX" with "Stay logged in" button
- User must explicitly click "Stay logged in" — continuing to work does NOT extend session
- Activity that resets timer: clicks and typing only (not passive mouse movement)
- Unsaved form data is NOT preserved — the warning is the protection
- Timeout is configurable per organization (admin setting with min/max bounds, e.g., 15-60 min)

### Absolute Session Limits
- 24-hour max session is also configurable per organization (admin setting with bounds, e.g., 8h-72h)
- No "Remember me" option — all sessions have consistent limits
- Same 5-minute toast warning before absolute limit expires
- After session expires, re-login redirects back to the page user was on

### CSRF Implementation
- Claude's discretion on token delivery mechanism (cookie+header, endpoint, or meta tag)
- Claude's discretion on which HTTP methods require validation (recommend POST/PUT/DELETE/PATCH)
- Claude's discretion on error messaging when CSRF fails
- Claude's discretion on API token exemption from CSRF (recommend Bearer token requests exempt)

### Tenant Isolation Testing
- Integration tests in Go (not E2E browser tests)
- Test ALL CRUD endpoints — all data is sensitive and must be org-scoped
- Must test impersonation scenario: when platform admin impersonates an org, they are fully scoped to that org only (can't see other orgs)
- Claude's discretion on test structure (matrix vs scenario-based)
- Claude's discretion on CI integration approach

### Claude's Discretion
- CSRF token delivery mechanism
- CSRF-protected HTTP methods
- CSRF error messaging
- API token CSRF exemption
- Test structure (matrix vs scenario)
- CI pipeline integration strategy

</decisions>

<specifics>
## Specific Ideas

- Toast notification for warnings — less intrusive than modal, but still visible
- "Must click button" model for session extension — prevents accidental session extension
- Impersonation isolation is critical — admin should be fully constrained to impersonated org

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-session-management*
*Context gathered: 2026-02-04*
