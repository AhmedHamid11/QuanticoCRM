# Codebase Concerns

**Analysis Date:** 2026-02-03

## Critical Security Issues

### CRIT-001: Overly Permissive CORS Configuration

**Files:** `backend/cmd/api/main.go:110-114`

**Issue:** CORS is configured with `AllowOrigins: "*"`, allowing any origin to make authenticated requests.

**Risk:**
- Cross-Site Request Forgery (CSRF) attacks enabled
- Any website can steal user data or perform actions
- Complete compromise of user sessions possible

**Fix approach:**
- Read `ALLOWED_ORIGINS` from environment variable
- Fail startup in production if not set
- Use comma-separated list of specific origins only

**Priority:** Critical - Fix before production deployment

---

### CRIT-002: Weak Default JWT Secret

**Files:** `backend/cmd/api/main.go:38-44`

**Issue:** Falls back to hardcoded `"dev-secret-change-in-production"` if JWT_SECRET not set.

**Risk:**
- Predictable tokens allow account takeover
- Anyone with source code can forge admin tokens
- If accidentally deployed without env var, auth bypass

**Fix approach:**
- Require JWT_SECRET in production (use `os.Getenv("ENVIRONMENT")` check)
- For dev, generate random 64-char secret
- Validate secret length is at least 32 chars

**Priority:** Critical - Fix before production deployment

---

### CRIT-003: SQL Injection via String Interpolation

**Files:** `backend/internal/handler/data_explorer.go:326`

**Issue:** Uses `fmt.Sprintf("org_id = '%s'", orgID)` to build SQL queries despite `orgID` coming from JWT claims.

**Risk:**
- Database schema exposure
- Data exfiltration or modification possible
- Future code changes could amplify risk

**Fix approach:**
- Use parameterized queries exclusively (?) for all dynamic SQL
- Remove all string interpolation from query building
- Consider using ORM for dynamic query safety

**Priority:** Critical

---

### CRIT-004: Sensitive Data in Error Responses

**Files:** Multiple - `handler/data_explorer.go` (8 locations), `handler/generic_entity.go`, `handler/contact.go`, `cmd/api/main.go:100-104`

**Issue:** Raw database errors returned in JSON responses expose schema and internal details.

**Example:**
```go
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
    "error": err.Error(),  // Exposes schema via "no such column: account_id_id"
})
```

**Risk:**
- Database schema disclosure
- Attackers learn system internals for targeted attacks
- Stack traces could leak sensitive paths

**Fix approach:**
- Create error mapper that sanitizes messages
- Log full errors internally only
- Return generic "An internal error occurred" to clients
- Map specific errors to standard messages (404 for missing, 400 for validation, etc.)

**Priority:** Critical

---

### CRIT-005: Tokens Stored in localStorage (XSS Vulnerability)

**Files:** `frontend/src/lib/stores/auth.svelte.ts:79,88`

**Issue:** Both accessToken and refreshToken stored in localStorage.

**Risk:**
- Any XSS vulnerability allows complete token theft
- Tokens persist across sessions, expanding exposure window
- No HttpOnly flag protection (JavaScript can access)

**Fix approach:**
- Store refresh tokens in HttpOnly, Secure, SameSite=Strict cookies only
- Keep access tokens in memory only (expires on page reload)
- Implement token rotation on each refresh
- Add Content Security Policy headers

**Priority:** Critical

---

## High Severity Issues

### HIGH-001: Missing Rate Limiting on Auth Endpoints

**Files:** `backend/cmd/api/main.go:127-131`

**Unprotected endpoints:**
- `POST /auth/register` - No rate limiting
- `POST /auth/login` - No rate limiting
- `POST /auth/accept-invite` - No rate limiting
- `POST /auth/refresh` - No rate limiting

**Risk:** Brute force attacks, credential stuffing, email enumeration

**Fix approach:**
- Add per-IP rate limiting: 5 attempts per minute
- Use Fiber's `limiter` middleware
- Return 429 when exceeded

**Priority:** High - Implement in next sprint

---

### HIGH-002: Database Connection State Fragility

**Files:** `backend/internal/handler/related_list.go:78-88`, `backend/internal/db/manager.go`

**Issue:** Code detects connection failures via string matching on error messages:
```go
strings.Contains(errStr, "database is closed") ||
strings.Contains(errStr, "bad connection") ||
strings.Contains(errStr, "connection refused")
```

**Risk:**
- String matching is fragile across database drivers/versions
- May miss new error formats
- Not a reliable connection health check
- Multiple database files can cause stale connections

**Fix approach:**
- Use connection pooling health checks instead of error inspection
- Implement proper connection lifecycle management
- Consolidate all database file paths (currently 2.3MB db exists alongside multiple smaller ones)
- Use connection context deadlines

**Priority:** High

---

### HIGH-003: Large Monolithic Handler Files

**Files:**
- `backend/internal/handler/generic_entity.go` (1519 lines)
- `backend/internal/handler/import.go` (1099 lines)
- `backend/internal/handler/related_list.go` (853 lines)
- `backend/internal/handler/bulk.go` (740 lines)

**Issue:** Single files contain multiple logical concerns mixed together.

**Risk:**
- Difficult to test individual operations
- High change frequency increases bug surface
- Refactoring becomes risky
- Cognitive load for understanding code

**Fix approach:**
- Split `generic_entity.go` into:
  - `generic_entity_list.go` (List, Get operations)
  - `generic_entity_write.go` (Create, Update, Delete)
  - `generic_entity_rollup.go` (Rollup execution)
- Apply same refactoring to `import.go` and others
- Extract helper functions into separate packages

**Priority:** High - Impacts maintenance and testing

---

### HIGH-004: Missing TODO Implementation - Record Fetching in Flows

**Files:** `backend/internal/handler/flow.go:309`

**Code:**
```go
// TODO: Fetch record from entity service
// For now, just store the record ID in variables
record = map[string]interface{}{
    "id": req.RecordID,
}
```

**Issue:** Flow execution doesn't load full record data when starting flow with source record.

**Risk:**
- Flow variables don't have access to record fields
- Expressions can't reference `{{accountName}}` or other fields
- Limited flow functionality

**Fix approach:**
- Fetch record using GenericEntityHandler.Get pattern
- Convert to map[string]interface{}
- Load into flow context variables
- Handle missing records gracefully

**Priority:** High - Blocks advanced flow features

---

## Medium Severity Issues

### MED-001: Open Bug - Rollup Field UI Not Rendering

**Status:** OPEN

**Files:** `frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte:437-580`

**Issue:** Svelte 5 fine-grained reactivity doesn't re-evaluate `{#if}` blocks when nested `$state` properties change via `bind:value`.

**Symptoms:**
- Selecting "Rollup" from field type dropdown doesn't show rollup-specific fields
- Max Length field (varchar) appears instead
- Cannot create rollup fields via UI

**Risk:**
- Rollup fields can only be created via direct API calls
- User experience broken for rollup field configuration
- Admin interface inconsistent

**Tested approaches that failed:**
- Object reassignment: `newField = { ...newField }`
- Separate state variable tracking
- `$derived` variables
- `{#key}` block wrapper

**Fix approach:**
- Upgrade vite-plugin-svelte from @3 to @4 (recommended by build system)
- If persists, use reactive assignment pattern: update derived state on select change
- Alternative: Use separate `selectedType` state synced with select via onChange handler
- Verify `/api/v1/admin/field-types` returns 'rollup' type

**Priority:** Medium - Workaround exists (API calls)

---

### MED-002: Unused or Deprecated Functions

**Files:** Multiple deprecated functions marked but not removed:
- `backend/internal/db/manager.go:129` - `GetTenantDB()` (deprecated, use `GetTenantDBConn`)
- `backend/internal/middleware/tenant.go:153` - Same deprecation
- `backend/internal/handler/related_list.go:46` - Same deprecation

**Issue:** Code contains deprecated functions alongside new ones causing confusion.

**Risk:**
- Maintenance burden - developers must know which to use
- Accidental use of deprecated versions may miss retry logic
- Code duplication (both versions maintained)

**Fix approach:**
- Complete migration from old to new functions
- Remove deprecated versions entirely
- Add linting rule to detect `GetTenantDB` usage and reject

**Priority:** Medium - Technical debt cleanup

---

### MED-003: DEBUG Logging Left in Production Code

**Files:** `backend/internal/handler/generic_entity.go:665, 799, 852, 944, 1456`

**Code examples:**
```go
// DEBUG: Log received body for troubleshooting field saving issues
// DEBUG: Log INSERT details for troubleshooting
// DEBUG: Log UPDATE details
```

**Issue:** Debug comments and potential debug logging statements in handlers.

**Risk:**
- May slow down production requests
- Information disclosure if sensitive data logged
- Noise in logs makes debugging harder
- Should be removed before production

**Fix approach:**
- Search codebase for all `// DEBUG` comments
- Remove or convert to structured logging with debug level
- Use logger with configurable levels (only log in dev)
- Add pre-commit hook to prevent DEBUG commits

**Priority:** Medium

---

## Low Severity / Technical Debt

### LOW-001: Multiple Database Files

**Files:**
- `/fastcrm/fastcrm.db` (2.3MB)
- `/fastcrm/backend/fastcrm.db` (mentioned in ONGOING_DOCUMENTATION.md)
- `fastcrm.db.backup` (backup file)

**Issue:** Multiple database files can cause state divergence. Previously caused rollup fields to disappear after restart.

**Risk:**
- Stale connections to wrong database
- Inconsistent state if migrations run on only one copy
- Confusion during development/testing

**Fix approach:**
- Consolidate to single `DATABASE_PATH` location
- Set consistently in `services.sh` and development scripts
- Document database path convention
- Add validation to startup to ensure single db file

**Priority:** Low - Mitigated by services.sh convention

---

### LOW-002: Large Migration File Count

**Files:** 40+ migration files in `/migrations/` directory

**Issue:** Many small migration files make it hard to understand schema evolution.

**Risk:**
- Long startup time for new databases (runs all migrations)
- Hard to understand current state without reading all files
- Difficult to debug schema issues

**Fix approach:**
- Create schema snapshot migrations periodically
- Consolidate small related migrations
- Document key schema decisions in migration comments

**Priority:** Low - Not blocking

---

### LOW-003: Test Coverage Gaps

**Status:** Minimal test coverage

**Files with minimal/no tests:**
- `backend/internal/handler/custom_page.go` - No dedicated tests
- `backend/internal/handler/pdf_template.go` - No dedicated tests
- `backend/internal/service/pdf_*.go` - Integration tests only
- `backend/internal/repo/query.go` - No tests
- Frontend: Most pages tested via manual browser verification only

**Test files count:** 13 test files covering ~36K lines of code

**Risk:**
- High regression risk when modifying handlers
- Refactoring unsafe
- PDF rendering failures would go undetected

**Fix approach:**
- Add unit tests for all handlers (target 70%+ coverage)
- Mock database in handler tests
- Test error cases (permission denied, not found, validation)
- Add integration tests for query-heavy functions

**Priority:** Low - Testing phase can come after core features stabilize

---

### LOW-004: Missing Validation on Rollup Queries

**Files:** `backend/internal/entity/metadata.go`, `backend/internal/handler/admin.go`

**Issue:** Rollup queries with quoted placeholders fail silently:
```sql
-- WRONG: {{id}} inside quotes becomes literal '?'
SELECT COUNT(*) FROM contacts WHERE account_id = '{{id}}'

-- RIGHT: No quotes around placeholder
SELECT COUNT(*) FROM contacts WHERE account_id = {{id}}
```

**Risk:**
- Users create non-functional rollup fields without error
- Debug nightmare - query appears valid but returns nulls

**Fix approach:**
- Add validation in field creation endpoint
- Check for pattern `'{{.*}}'` (quoted placeholder)
- Return error: "Placeholder must not be quoted"
- Add documentation warning

**Priority:** Low - Documented in BUG_LOG.md, users can self-fix

---

### LOW-005: Error Handling Inconsistency

**Pattern:** Naked `return nil` after errors in some places:

```go
json.Unmarshal([]byte(idsStr), &ids)  // Error ignored
```

Found ~592 instances of `return nil` across codebase (many legitimate, some swallowing errors).

**Risk:**
- Silent failures in data transformation
- Difficult bugs to trace
- Data corruption possible in edge cases

**Fix approach:**
- Audit all error returns in critical paths
- Ensure JSON unmarshaling errors are handled
- Use linting tools to enforce error handling
- Consider `errcheck` linter

**Priority:** Low - Mostly safe patterns, but should be audited

---

## Performance Bottlenecks

### PERF-001: Rollup Batch Execution Improvements Possible

**Files:** `backend/internal/handler/generic_entity.go:416-451`

**Current approach:**
- Batch execution per field (1 query per rollup field)
- For 10 records × 3 rollup fields = 3 queries total ✓ (good)

**Risk:**
- If many rollup fields, still N separate queries
- No caching between requests
- Recalculates on every list view

**Improvement opportunity:**
- Cache rollup results per record for duration of request
- Add optional result caching layer (Redis-backed)
- Consider rollup materialization for high-frequency fields

**Priority:** Low - Only relevant at scale (>1000 records/sec)

---

### PERF-002: JSON Unmarshaling in Lists

**Files:** `backend/internal/handler/generic_entity.go:386`

**Issue:**
```go
json.Unmarshal([]byte(idsStr), &ids)  // In loop for each record
```

Unmarshaling JSON arrays for multi-lookup fields happens per record in hot loop.

**Risk:**
- CPU intensive for large lists with multi-lookups
- Could cause slow list loads with 100+ records

**Improvement approach:**
- Cache parsed array per field/record
- Pre-compile JSON parsing if field is static
- Lazy-evaluate only if field requested in response

**Priority:** Low - Only visible at 100+ record lists

---

## Fragile Areas Requiring Careful Modification

### FRAG-001: Related List Query Building

**Files:** `backend/internal/handler/related_list.go:299-400`

**Why fragile:**
- Complex column naming logic (standard vs custom entities)
- Multiple lookups on standard entity naming conventions
- Cross-org entity detection has fallback logic
- Database schema detection via PRAGMA queries

**Safe modification approach:**
- Write comprehensive test cases first covering:
  - Standard entities (Contact, Account, etc.)
  - Custom entities (Jobs, Candidates, etc.)
  - Cross-org lookups
  - Missing columns gracefully
- Use generated test data from seeds
- Never change column naming without extensive testing

**Current issues fixed:**
- FCRM-002: Double `_id` suffix bug (RESOLVED)
- FCRM-004: Standard entity column naming (RESOLVED)
- FCRM-005: Custom entity org_id detection (RESOLVED)

**Priority:** High caution when modifying this area

---

### FRAG-002: Multi-Tenant Database Isolation

**Files:** `backend/internal/middleware/tenant.go`, `backend/internal/db/manager.go`

**Why fragile:**
- OrgID enforcement scattered across handlers
- Tenant DB context must be passed through middleware
- Missing org_id filter on any query = data breach
- Multiple data paths (tenant DB vs default DB)

**Safe modification approach:**
- Use `tableHasColumn()` pattern to detect schema dynamically
- Always include org_id in WHERE clauses for standard entities
- Test cross-org isolation explicitly
- Add SQL query validation middleware

**Priority:** Critical - Any bug = data breach

---

## Known Bugs (From BUG_LOG.md)

### FCRM-001: Rollup Field Type UI Not Rendering
**Status:** OPEN
**Root Cause:** Svelte 5 reactivity issue with {#if} blocks
**Workaround:** Create via API instead of UI

### FCRM-003: Rollup Field Execution Issues
**Status:** RESOLVED (multiple sub-issues)
- AccountHandler didn't execute rollups ✓ Fixed
- Rollup queries had wrong table/column names ✓ Fixed
- Field update API didn't save rollup fields ✓ Fixed
- Quoted placeholders break parameter binding - DATA GUIDANCE only

---

## Summary by Category

| Category | Count | Severity |
|----------|-------|----------|
| Security | 5 critical | Critical |
| Performance | 2 | Low |
| Test Coverage | 1 major | Medium |
| Technical Debt | 5 | Low-Medium |
| Bugs | 1 open | Medium |
| Code Quality | 3 | Medium |

**Blocking Production:** 5 critical security issues must be resolved first

**Next Sprint Focus:** Rate limiting (HIGH-001), monolithic handler split (HIGH-003), SQL injection audit (CRIT-003)

---

*Concerns audit: 2026-02-03*
