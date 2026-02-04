# GSD State: Quantico CRM

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-03)

**Core value:** Fast, secure multi-tenant CRM where customer data is protected
**Current focus:** Phase 08 - Security Hardening - VERIFIED ✓

## Current Position

**Milestone:** v2.0 Security Hardening
**Phase:** 08 of 10 (Security Hardening) - VERIFIED ✓
**Plan:** 04 of 04 complete
**Status:** Phase 08 verified (5/5 truths), ready for Phase 09

**Last activity:** 2026-02-04 - Phase 08 verified, all 5 success criteria met

Progress: [███-------] 30% (3/10 phases complete)

## Performance Metrics

**Velocity:**
- Total plans completed: 12
- Average duration: 4.8 min
- Total execution time: 57 min

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 06-critical-fixes | 5 | 37min | 7.4min |
| 07-token-architecture | 3 | 7min | 2.3min |
| 08-security-hardening | 4 | 13min | 3.3min |

*Updated after each plan completion*

## Accumulated Context

### Key Decisions

- Use existing Fiber middleware for rate limiting
- Move refresh tokens to HttpOnly cookies
- Keep access tokens in memory only (not localStorage)
- Implement token rotation with family tracking
- Silent CORS reject (no headers) rather than 403 error to prevent origin enumeration
- HSTS with 1-year max-age and includeSubDomains directive
- Production detection via ENVIRONMENT env var OR presence of TURSO_URL
- **06-02:** Custom rate limiter for consistent 429 JSON response format
- **06-02:** In-memory sync.Map rate limit storage (acceptable per-instance reset)
- **06-02:** Rate limit applied at auth group level (protects all auth endpoints)
- **06-03:** 7 error categories for classification (database, validation, auth, permission, not_found, conflict, internal)
- **06-03:** Pattern-based error classification using error string analysis
- **06-03:** request_id field for support correlation on all error responses
- **06-03:** Focus on critical handlers (metadata, generic_entity); lower-risk handlers can be updated incrementally
- **06-04:** util.NewAPIErrorWithMessage for user-safe PDF generation errors
- **06-04:** util.ErrCategoryDatabase for platform admin routes
- **06-05:** auth.go permission errors use NewAPIErrorWithMessage (safe to expose)
- **06-05:** api_token.go scope errors use NewAPIErrorWithMessage (user-facing validation)
- **06-05:** custom_page.go added to plan (deviation) to complete handler package sanitization
- **07-01:** Token family uses sfid pattern with 0Tf prefix
- **07-01:** Soft revocation (is_revoked) for reuse detection vs deletion
- **07-01:** Login/register/org-switch start new token families (security boundary)
- **07-02:** Refresh tokens stored in HttpOnly cookies (XSS-immune)
- **07-02:** Cookie path restricted to /api/v1/auth
- **07-02:** SameSite=Strict for CSRF protection
- **07-03:** Access tokens memory-only in Svelte reactive state
- **07-03:** credentials: include on all frontend API calls
- **07-03:** silentRefresh on page load restores session from cookie
- **08-01:** Manual security headers instead of Fiber Helmet (full control, explicit policy)
- **08-01:** Content-Length check before reading body (prevent memory exhaustion)
- **08-01:** Separate route groups for different body limits (1MB default, 10MB imports)
- **08-02:** go:embed directive for compile-time password list embedding
- **08-02:** Case-insensitive common password matching
- **08-02:** Unicode character count (utf8.RuneCountInString) not byte count
- **08-02:** Maximum 128 characters to support long passphrases per NIST
- **08-03:** Simple 4-level password strength without external library (red/orange/yellow/green)
- **08-03:** Bindable value prop pattern for reusable form components
- **08-03:** Real-time strength feedback via $derived reactive statement
- **08-04:** JWT claim for mustChangePassword flag (stateless enforcement)
- **08-04:** Middleware after auth but before tenant resolution for password change enforcement
- **08-04:** API tokens skip password change requirement (org-level vs user-level)
- **08-04:** Password change returns new tokens with mustChangePassword=false (seamless re-auth)

### Blockers/Concerns

- Token migration must maintain backwards compatibility
- Need to verify CORS changes don't break legitimate clients (VERIFIED: localhost works in dev, allowlisted origins work in prod)

## Quick Tasks Completed (v1.0)

| # | Description | Date | Commit |
|---|-------------|------|--------|
| 001 | Exit impersonation on own org select | 2026-02-01 | 64dbcd9 |
| 002 | Configurable homepage per org | 2026-02-02 | 4b135e2 |
| 003 | Fix text field saving on custom entities | 2026-02-02 | 09fc2a3 |
| 004 | Fix related records 500 error | 2026-02-02 | 4c78931 |
| 005 | Edit in list for related records | 2026-02-02 | 8158119 |
| 006 | Add edit object icon to custom entities | 2026-02-02 | 0779eed |
| 007 | Soft delete custom entities | 2026-02-02 | 9f43d06 |
| 008 | Add created/modified by user tracking | 2026-02-03 | 82b4912 |
| 009 | Experimental styling (fonts + colors) | 2026-02-03 | d7c147d |

## Session Continuity

Last session: 2026-02-04
Stopped at: Phase 08 verified, ready for Phase 09
Resume file: None

---

*Updated: 2026-02-04 - Phase 08 verified (5/5 truths) - Ready for Phase 09*
