# Phase 08: Security Hardening - Context

**Gathered:** 2026-02-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Harden application against common attack vectors: security response headers, password policy enforcement, and request body size limits. This phase covers X-Frame-Options, X-Content-Type-Options, CSP, password requirements per NIST guidelines, and configurable request limits.

Note: Password breach database checking explicitly skipped per user decision.

</domain>

<decisions>
## Implementation Decisions

### Content Security Policy
- Strict CSP — no inline scripts/styles, explicit allowlist for sources
- API backend only (Go/Fiber sets CSP headers); SvelteKit frontend handles its own via Vercel config
- External resources allowed only if whitelisted in UI (dynamic allowlist capability)
- No violation reporting — enforce silently

### Security Headers
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- CSP: strict policy, self-only default with configurable allowlist

### Password Requirements
- Follow NIST SP 800-63B guidelines: length-based (8-128 characters), no complexity rules
- Check against common passwords list (NIST recommends this)
- Real-time validation feedback as user types
- Specific error messages ("Password must be at least 8 characters")
- Force existing users with weak passwords to update on next login

### Request Body Limits
- Default: 1MB for all endpoints
- File uploads: 10MB limit
- Configurable via MAX_BODY_SIZE environment variable (default 1MB)
- Upload limit configurable via MAX_UPLOAD_SIZE env var (default 10MB)
- Return 413 Payload Too Large with JSON message when exceeded

### Claude's Discretion
- Exact CSP directive syntax and structure
- Common passwords list source and size
- How to detect "file upload" endpoints vs regular endpoints
- Password strength indicator UI design
- Implementation of forced password update flow

</decisions>

<specifics>
## Specific Ideas

- NIST SP 800-63B is the reference standard for password policy
- Real-time password feedback should feel responsive, not intrusive
- Forced password update should explain why ("Your password doesn't meet our updated security requirements")

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

**Explicitly skipped (not deferred):**
- Password breach database checking (HaveIBeenPwned) — user decided not needed

</deferred>

---

*Phase: 08-security-hardening*
*Context gathered: 2026-02-03*
