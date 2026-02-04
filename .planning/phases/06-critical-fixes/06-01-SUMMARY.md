---
phase: 06-critical-fixes
plan: 01
subsystem: security
tags: [cors, hsts, jwt, config, middleware]

# Dependency graph
requires: []
provides:
  - Centralized config package with environment validation
  - CORS middleware with origin allowlist validation
  - HSTS middleware for secure transport enforcement
  - Startup validation that fails fast on missing JWT_SECRET
affects: [07-token-security, 08-session-management]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Centralized config loading at startup"
    - "Silent CORS reject for non-allowlisted origins"
    - "Fail-fast validation for production requirements"

key-files:
  created:
    - fastcrm/backend/internal/config/config.go
    - fastcrm/backend/internal/middleware/security.go
  modified:
    - fastcrm/backend/cmd/api/main.go

key-decisions:
  - "Silent CORS reject (no headers) rather than 403 error to prevent origin enumeration"
  - "HSTS with 1-year max-age and includeSubDomains directive"
  - "Production detection via ENVIRONMENT env var OR presence of TURSO_URL"

patterns-established:
  - "Config: Load and validate at startup before any other initialization"
  - "CORS: Return CORS headers only for explicitly allowed origins"
  - "Security middleware: Applied after recover/logger, before rate limiting"

# Metrics
duration: 5min
completed: 2026-02-04
---

# Phase 06 Plan 01: Foundation Security Summary

**CORS allowlist validation, HSTS headers, and JWT_SECRET startup validation to address CRIT-01, CRIT-04, CRIT-05**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-04T00:22:40Z
- **Completed:** 2026-02-04T00:27:18Z
- **Tasks:** 3
- **Files created/modified:** 3

## Accomplishments

- Centralized config package with startup validation for production requirements
- CORS middleware that silently rejects non-allowlisted origins (no CORS headers returned)
- HSTS middleware applying Strict-Transport-Security header on all responses
- Application now refuses to start in production without JWT_SECRET

## Task Commits

Each task was committed atomically:

1. **Task 1: Create centralized config with startup validation** - `57e7388` (feat)
2. **Task 2: Create security middleware for CORS and HSTS** - `53007dd` (feat)
3. **Task 3: Integrate config and security middleware into main.go** - `71a6f97` (feat)

Note: Task 3 commit includes additional auth rate limiter changes from 06-02 plan that were merged together.

## Files Created/Modified

- `fastcrm/backend/internal/config/config.go` - Centralized config with Load(), IsProduction(), GetJWTSecret()
- `fastcrm/backend/internal/middleware/security.go` - HSTS() and NewCORS() middleware functions
- `fastcrm/backend/cmd/api/main.go` - Integrated config loading and security middleware

## Decisions Made

1. **Silent CORS reject pattern** - Non-allowlisted origins receive NO CORS headers rather than a 403 error. This prevents attackers from enumerating valid origins.

2. **HSTS configuration** - 1-year max-age (31536000 seconds) with includeSubDomains. Standard secure configuration that forces HTTPS for all subsequent requests.

3. **Production detection logic** - Environment is considered production if ENVIRONMENT=production/prod OR if TURSO_URL is set. This ensures database-connected deployments are always treated as production.

4. **Development localhost auto-allow** - In development mode, localhost origins (any port) are automatically allowed for CORS to simplify local development.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

**Environment variables for production deployment:**

```bash
# Required in production
JWT_SECRET=your-secure-random-secret

# Recommended - comma-separated list of allowed origins
ALLOWED_ORIGINS=https://quanticocrm.com,https://app.quanticocrm.com

# Auto-detected as production if set
TURSO_URL=libsql://your-db.turso.io
```

## Verification Results

All three critical security controls verified:

1. **CRIT-01 (CORS):** Requests from `https://evil.com` receive NO CORS headers (verified)
2. **CRIT-04 (JWT_SECRET):** App exits with fatal error if JWT_SECRET not set in production (verified)
3. **CRIT-05 (HSTS):** All responses include `Strict-Transport-Security: max-age=31536000; includeSubDomains` (verified)

## Next Phase Readiness

- Foundation security controls complete
- Ready for 06-02: Auth rate limiting (can use cfg.AuthRateLimit)
- Ready for 06-03: Error sanitization (can use cfg.IsProduction())

---
*Phase: 06-critical-fixes*
*Plan: 01*
*Completed: 2026-02-04*
