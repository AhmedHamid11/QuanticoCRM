# Phase 06: Critical Fixes - Context

**Gathered:** 2026-02-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Eliminate critical security vulnerabilities that block production deployment: CORS lockdown, rate limiting on auth endpoints, error sanitization, JWT_SECRET validation at startup, and HSTS headers on all responses.

</domain>

<decisions>
## Implementation Decisions

### CORS Configuration
- Allowed origins via environment variable (ALLOWED_ORIGINS)
- Auto-allow localhost:* in development mode (when ENV=development)
- Support for Vercel preview URLs and localhost during development, then lock to quanticoCRM.com later
- Silent reject for blocked origins (no CORS headers returned)
- Log blocked origins for monitoring/debugging

### Rate Limiting
- Scope: Auth endpoints only (login, register, password reset)
- Storage: In-memory (resets on restart, per-instance)
- Response: 429 with JSON body explaining limit and when to retry
- Configurable via environment variable (AUTH_RATE_LIMIT, default 5 req/min)

### Error Sanitization
- Production: Category-based generic messages ("A database error occurred" vs "A validation error occurred")
- Development: Full details including stack traces and DB errors
- Logging: Structured JSON logs with error details, request ID, timestamp
- Response: Include request_id in error responses for support correlation

### Startup Validation
- Required: JWT_SECRET only (other vars may warn but not block)
- Failure mode: Fatal exit with clear message ("FATAL: JWT_SECRET not set") and exit code 1
- Timing: Validate at config load/startup (fail fast)
- Strength: Presence validation only (not length/complexity)

### Claude's Discretion
- HSTS header implementation details (exact header format follows spec)
- Specific log format/structure
- Error category taxonomy
- Rate limit window implementation details

</decisions>

<specifics>
## Specific Ideas

- Domain will eventually be quanticoCRM.com but currently uses Vercel temporary frontend and localhost
- Rate limiting should match success criteria exactly (5 req/min default)
- Error response pattern: generic message + request_id for support lookup

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-critical-fixes*
*Context gathered: 2026-02-03*
