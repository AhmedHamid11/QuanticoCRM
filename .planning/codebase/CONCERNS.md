# Codebase Concerns

**Analysis Date:** 2026-01-31

## Tech Debt

**Rollup Field Type - UI Conditional Rendering Not Working (FCRM-001):**
- Issue: When selecting "Rollup" field type in Entity Manager, the rollup-specific configuration fields do not appear. Max Length field (for varchar) incorrectly displays instead.
- Files: `frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte`
- Impact: Rollup fields cannot be created via UI. Users must use direct API calls as workaround.
- Fix approach: Investigate Svelte 5 fine-grained reactivity issue with `{#if}` blocks on `$state` objects. Attempted fixes (object reassignment, separate state variable, derived state, {#key} wrapper) have failed. Likely requires vite-plugin-svelte upgrade from @3 to @4 or architectural refactor.
- Priority: HIGH - blocks standard CRM field type creation feature

**Flow Handler Record Fetching Incomplete:**
- Issue: Flow handler has TODO comment at line 309 indicating record fetching from entity service is not implemented.
- Files: `backend/internal/handler/flow.go:309`
- Impact: Flow execution cannot access full record context, only stores basic record ID in variables. Limits workflow automation capabilities.
- Fix approach: Implement record fetching logic that retrieves record data from entity service and populates flow variables for expression evaluation.
- Priority: MEDIUM - affects advanced workflow features

**Multiple Database Files Causing Data Loss Risk:**
- Issue: Two database files exist and are used depending on how backend is started:
  - `fastcrm/fastcrm.db` (278KB) - used by `services.sh`
  - `fastcrm/backend/fastcrm.db` (286KB) - used when running backend directly
- Files: Both locations in `/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/`
- Impact: Created data can disappear on restart if different startup method is used. Already occurred with `test_rollup` field.
- Fix approach: Consolidate to single database file location. Set `DATABASE_PATH` environment variable consistently across all startup methods. Document standard startup procedure.
- Priority: HIGH - production data loss risk

**Unquoted Table Names in SQL Queries:**
- Issue: Some queries use unquoted table names with string interpolation, creating potential injection vectors if entity names become dynamic.
- Files:
  - `backend/internal/handler/generic_entity.go` - multiple lines with `fmt.Sprintf` and table names
  - `backend/internal/handler/import.go`
  - `backend/internal/handler/bulk.go`
- Impact: While entity names are controlled internally, this pattern violates secure coding practices and creates maintenance risk.
- Fix approach: Use `util.QuoteIdentifier()` for all table/column names in dynamic queries. Audit all `fmt.Sprintf` queries.
- Priority: MEDIUM - defensive coding improvement

**Expression Parser Lacks Sandbox and Limits:**
- Issue: Flow expression evaluator supports dynamic function evaluation without complexity limits, input validation, or sandboxing.
- Files: `backend/internal/flow/expression.go` (879 lines)
- Impact: Malicious or poorly-written flow expressions could cause performance issues or infinite loops. No limits on execution time or resource usage.
- Fix approach: Implement expression complexity scoring, timeout enforcement, and whitelisted function list. Add input sanitization.
- Priority: MEDIUM-HIGH - security and stability concern for user-created flows

---

## Known Bugs

**Related List Column Naming Inconsistency (FCRM-002, FCRM-004, FCRM-005):**
- Symptoms: Related lists fail with "no such column" errors due to different field-to-column naming conventions between standard and custom entities.
- Files: `backend/internal/handler/related_list.go` (lines 299-315, 367-392)
- Root cause:
  - Standard entities: field `accountId` → column `account_id`
  - Custom entities: field `hiringManagerId` → column `hiring_manager_id_id`
  - Code applies standard naming to all entities
- Current mitigation: Fixed in code with entity type detection and `tableHasColumn()` helper. Logic now properly handles both custom and standard entity naming.
- Status: RESOLVED but fragile - could break if entity definitions change without code update.

**Rollup Field Computation Not Executing for Standard Entities (FCRM-003):**
- Symptoms: Rollup fields display "-" (dash) instead of computed value on Account, Contact, etc. records.
- Files:
  - `backend/internal/handler/account.go` - missing rollup execution
  - `backend/internal/handler/generic_entity.go` - has rollup logic
- Root causes: Multiple issues fixed:
  1. AccountHandler missing rollup execution logic (fixed)
  2. Rollup queries using incorrect table/column names (fixed - requires correct SQL: `SELECT COUNT(*) FROM contacts WHERE account_id = {{id}}`)
  3. FieldDefUpdateInput missing rollup fields (fixed)
  4. Rollup query placeholder inside quotes breaks parameter binding
- Current issue: Rollup queries must not quote the `{{id}}` placeholder: `'{{id}}'` becomes literal `'?'` instead of parameter.
- Recommendation: Add validation in rollup field creation to warn if placeholder is quoted.
- Status: PARTIALLY RESOLVED - data queries work but placeholder validation missing.

**List Layout Configuration Fallback Ambiguity (ONGOING):**
- Symptoms: List views show fallback 5 columns even when layout is configured as empty.
- Files:
  - `backend/internal/handler/admin.go:416-434`
  - `frontend/src/routes/[entity=customentity]/+page.svelte`
- Root cause: API returns `"layoutData": "[]"` for both "never configured" and "explicitly configured as empty" states.
- Current fix: Backend now includes `exists` boolean flag in layout API response.
- Status: RESOLVED

**Invitation Token Expiration Not Enforced (MED-004):**
- Symptoms: Expired invitation tokens might be accepted due to missing expiration check at acceptance time.
- Files: `backend/internal/service/auth.go:364-419`
- Root cause: No explicit `if invitation.ExpiresAt.Before(time.Now())` check in `AcceptInvitation` method.
- Fix approach: Add expiration validation immediately after token lookup with clear error message.
- Priority: MEDIUM - compliance and security gap

---

## Security Considerations

**CRITICAL: Overly Permissive CORS Configuration:**
- Risk: Any website can make authenticated requests to your API. Enables CSRF attacks.
- Files: `backend/cmd/api/main.go:110-114`
- Current state: `AllowOrigins: "*"` with TODO comment about restricting in production.
- Current mitigation: Applied in production via `ALLOWED_ORIGINS` environment variable check, but code still contains the anti-pattern.
- Recommendations:
  1. Remove `*` default entirely - make `ALLOWED_ORIGINS` required
  2. Add `AllowCredentials: true` for cookie-based auth
  3. Include `X-CSRF-Token` in allowed headers
- Priority: CRITICAL - must fix before production

**CRITICAL: Weak Default JWT Secret:**
- Risk: Predictable JWT secret allows token forgery. If deployed without JWT_SECRET env var, complete auth bypass.
- Files: `backend/cmd/api/main.go:38-44`
- Current state: Falls back to "dev-secret-change-in-production"
- Current mitigation: Code checks if Turso is configured and fails if JWT_SECRET missing, but default value still exists.
- Recommendations:
  1. Remove hardcoded default entirely
  2. Only allow random generation in dev mode
  3. Enforce minimum 32-character secret length
- Priority: CRITICAL - fix before any production deployment

**HIGH: Sensitive Data in Error Responses:**
- Risk: Raw database errors expose schema and structure for targeted attacks.
- Files: Multiple handlers leaking errors:
  - `backend/internal/handler/data_explorer.go` (lines 87, 96, 116, 134, 188, 197, 226, 244)
  - `backend/internal/handler/contact.go` (lines 40, 54, 105)
  - `backend/internal/handler/generic_entity.go` (multiple locations)
  - Global error handler in `backend/cmd/api/main.go:100-104`
- Current state: All handlers return `err.Error()` directly to clients.
- Fix approach: Implement error mapper that logs full details internally but returns generic user-facing messages.
- Priority: CRITICAL - information disclosure risk

**HIGH: Tokens Stored in localStorage (XSS Vulnerability):**
- Risk: Any XSS vulnerability allows complete token theft. No HttpOnly protection.
- Files: `frontend/src/lib/stores/auth.svelte.ts:79,88`
- Current state: Both accessToken and refreshToken stored in localStorage with full persistence across sessions.
- Fix approach:
  1. Move refresh tokens to HttpOnly, Secure, SameSite=Strict cookies
  2. Keep access tokens in memory only with short expiration
  3. Implement token rotation on each refresh
- Priority: CRITICAL - common attack vector for SPAs

**HIGH: Missing Rate Limiting on Auth Endpoints:**
- Risk: Brute force attacks on login, email enumeration via registration, DDoS on auth.
- Files: `backend/cmd/api/main.go:127-131` - unprotected endpoints
- Affected endpoints: `/auth/register`, `/auth/login`, `/auth/accept-invite`, `/auth/refresh`
- Current state: No rate limiting middleware applied.
- Fix approach: Implement per-IP rate limiting (5 attempts per minute for login).
- Priority: HIGH - standard security requirement

**HIGH: Weak Password Validation Policy:**
- Risk: Accepts weak passwords like "12345678". No complexity requirements.
- Files: `backend/internal/service/auth.go:695-700`
- Current state: Only checks minimum 8-character length.
- Recommendations:
  1. Check against common password lists (Have I Been Pwned API)
  2. No arbitrary complexity rules (they reduce security)
  3. Implement rate limiting instead of lockouts per NIST guidelines
- Priority: HIGH - standard security requirement

**HIGH: Refresh Token Without Rotation:**
- Risk: Stolen refresh tokens remain valid until expiration. No token family tracking for reuse detection.
- Files: `backend/internal/service/auth.go:172-199`
- Current state: Token is deleted after refresh but old token remains valid if stolen.
- Fix approach: Implement token rotation (issue new token on each refresh) and token family tracking.
- Priority: HIGH - standard OAuth best practice

**MEDIUM: Multi-Tenant Data Isolation Inconsistencies:**
- Risk: While org_id filtering exists, inconsistent application could leak cross-tenant data.
- Files: Multiple repos and handlers apply org_id filtering differently
- Current state: Filtering is implemented but not enforced at database layer.
- Fix approach: Implement row-level security at database layer. Add integration tests for cross-org access attempts.
- Priority: MEDIUM - centralize isolation logic

**MEDIUM: Missing CSRF Protection:**
- Risk: State-changing endpoints vulnerable to CSRF attacks despite using bearer tokens.
- Current state: No CSRF token implementation. Depends on CORS restriction (which is misconfigured).
- Fix approach:
  1. Implement double-submit cookie CSRF tokens
  2. Add SameSite=Strict to all cookies
  3. Validate Origin/Referer headers
- Priority: MEDIUM - standard web security

**MEDIUM: No Session Timeout for Idle Users:**
- Risk: Access tokens valid for 24 hours regardless of activity. Increased exposure window.
- Current state: Tokens expire after 24 hours, no idle timeout.
- Fix approach: Implement sliding session expiration with explicit idle timeout (e.g., 30 minutes).
- Priority: MEDIUM - standard security practice

**MEDIUM: No Audit Logging Infrastructure:**
- Risk: Cannot detect unauthorized access, admin abuse, or security incidents.
- Compliance impact: SOC2, GDPR Article 30 violations.
- Missing audit events: User creation/deletion, API tokens, login attempts, impersonation, data exports, config changes.
- Fix approach: Implement centralized audit log service with events for all sensitive operations.
- Priority: MEDIUM-HIGH - compliance requirement

**MEDIUM: Insufficient Impersonation Controls:**
- Risk: Admins can maintain unauthorized access indefinitely without audit trail.
- Files: `backend/internal/service/auth.go:238-296`
- Issues: No time limit, limited audit logging, no automatic termination, no reason field.
- Fix approach: Add max 60-minute duration, reason field requirement, automatic expiration, comprehensive audit logging.
- Priority: MEDIUM - compliance requirement

**MEDIUM: Email Verification Not Enforced:**
- Risk: Accounts created with typo'd emails. No proof of email ownership.
- Files: `backend/internal/entity/user.go` - EmailVerified field exists but unused.
- Fix approach: Require email verification before account activation. Send confirmation emails.
- Priority: MEDIUM - affects user account creation flow

**LOW: Bcrypt Cost Factor Hardcoded:**
- Files: `backend/internal/service/auth.go:54`
- Issue: Cost factor set to 12, should be configurable to increase as computing power improves.
- Fix approach: Make configurable via environment variable.
- Priority: LOW - nice to have for security maintenance

**LOW: Missing HTTPS Enforcement:**
- Missing: HSTS header, HTTP to HTTPS redirect, Secure cookie flag enforcement.
- Fix approach: Add security headers in main.go middleware.
- Priority: LOW - standard production requirement

**LOW: No Request Size Limits:**
- Risk: Denial of service via large payloads, memory exhaustion.
- Fix approach: Set explicit limits on request body size via Fiber middleware.
- Priority: LOW - operational security

---

## Performance Bottlenecks

**Large Handler Functions with Complex Logic:**
- Problem: Several handlers exceed 1400+ lines, mixing business logic, data access, and HTTP handling.
- Files:
  - `backend/internal/handler/generic_entity.go` (1472 lines)
  - `backend/internal/handler/import.go` (1099 lines)
  - `backend/internal/service/auth.go` (1007 lines)
  - `backend/internal/repo/auth.go` (1000 lines)
- Impact: Difficult to understand, test, and modify. High cognitive load increases bugs.
- Improvement path: Break into smaller focused functions. Extract business logic from HTTP handlers. Separate concerns.
- Priority: MEDIUM - maintenance and reliability improvement

**Lack of Database Query Optimization:**
- Problem: No indexes explicitly created in migrations. Multiple queries without pagination limits.
- Files: `backend/migrations/`, all repository files
- Impact: Performance degrades as data grows. List views could slow with large datasets.
- Improvement path: Add database indexes on frequently filtered columns (org_id, lookup fields, status). Implement efficient pagination.
- Priority: MEDIUM - performance at scale

**Flow Expression Parser Without Limits:**
- Problem: No timeout, complexity limit, or recursion depth check. Potential DoS vector.
- Files: `backend/internal/flow/expression.go`
- Impact: Malformed flow expressions could cause server slowdown or hang.
- Improvement path: Add execution timeout (e.g., 100ms), recursion depth limit, and complexity scoring.
- Priority: MEDIUM - operational safety

**Missing Database Connection Pooling Configuration:**
- Problem: Connection pool settings are hardcoded and not tunable per environment.
- Files: `backend/cmd/api/main.go:90-92`
- Current: `MaxOpen=25, MaxIdle=10, MaxLifetime=5m`
- Impact: May be suboptimal for different load profiles (local dev vs. production).
- Improvement path: Make pool settings configurable via environment variables.
- Priority: LOW - optimization for different deployment scenarios

---

## Fragile Areas

**Related List Handler - Column Detection Logic:**
- Files: `backend/internal/handler/related_list.go`
- Why fragile: Complex logic to detect custom vs. standard entities and handle different column naming. Uses `tableHasColumn()` helper that queries schema at runtime.
- Safe modification: Add comprehensive tests covering edge cases (cross-org entities, custom entities with standard entity lookup fields, etc.). Document the column naming conventions.
- Test coverage: Related list tests exist but may not cover all entity naming combinations.

**Data Explorer with Dynamic Query Injection:**
- Files: `backend/internal/handler/data_explorer.go:285-378`
- Why fragile: Attempts to inject org_id filter into user-provided SELECT queries using string manipulation.
- Safe modification: Treat as high-risk area. Any changes require security review. Consider replacing with parameterized query builder instead of string manipulation.
- Test coverage: Should add security tests for SQL injection attempts.

**Flow Expression Evaluator with Nested Logic:**
- Files: `backend/internal/flow/expression.go` (879 lines)
- Why fragile: Complex recursive evaluation logic with multiple operator precedence levels. String parsing without proper tokenization.
- Safe modification: Any changes to operator precedence or evaluation order need extensive testing. Use formal expression grammar/parser instead of string manipulation.
- Test coverage: Needs comprehensive test suite for operator combinations and edge cases.

**Multi-Tenant Database Switching:**
- Files: `backend/internal/middleware/tenant.go`, `backend/cmd/api/main.go`, all repositories
- Why fragile: Tenant database selection happens at middleware layer and is passed through context. Easy to forget to use tenant DB and accidentally query master DB.
- Safe modification: Add compile-time checks (interfaces) to prevent direct masterDB access in data handlers. Centralize tenant DB selection logic.
- Test coverage: Integration tests should verify org_id filtering for all repository operations.

---

## Scaling Limits

**Per-Tenant Turso Database Creation:**
- Current capacity: Each org gets dedicated Turso database at organization creation.
- Limit: Turso API rate limits for database creation. Unknown number of orgs before hitting limits.
- Scaling path: Implement database pooling strategy (e.g., hash-based assignment to shared databases). Add monitoring for Turso API quotas.
- Priority: MEDIUM - affects multi-tenant scalability

**SQLite WAL File Size:**
- Current state: Large WAL file exists (`fastcrm.db-wal` 3.9MB) indicating heavy write activity.
- Limit: WAL files can grow unbounded if checkpoint doesn't occur frequently enough.
- Scaling path: Implement regular WAL checkpoints. Monitor WAL file size and implement cleanup logic.
- Priority: MEDIUM - database maintenance

**List View Pagination:**
- Current state: No explicit pagination shown in code, may load all records.
- Limit: Memory usage grows linearly with record count. UI performance degrades with large datasets.
- Scaling path: Implement server-side pagination (cursor-based for efficiency). Add default page size limits.
- Priority: MEDIUM - affects UI performance at scale

---

## Dependencies at Risk

**Svelte 5 Early Adoption - Fine-Grained Reactivity Issues:**
- Risk: Svelte 5.46.1 with vite-plugin-svelte@3 causing reactivity bugs. Console warns about plugin version mismatch.
- Impact: Field type selection modal doesn't work (FCRM-001). May have other hidden reactivity issues.
- Migration plan: Upgrade vite-plugin-svelte to @4 as recommended. Test all conditional blocks thoroughly. Consider filing Svelte issue or rolling back to Svelte 4.
- Priority: HIGH - affects feature development

**TursoDB HTTP Client with Auto-Reconnect:**
- Risk: Custom TursoDB wrapper (`backend/internal/db/turso.go`) implements auto-reconnect logic. If connection logic has bugs, all tenant databases affected.
- Impact: Stale connections could cause query failures or data loss.
- Migration plan: Test reconnection logic thoroughly. Add circuit breaker pattern if needed. Monitor connection health.
- Priority: MEDIUM - production stability

**Go SQLite Driver (go-sqlite3):**
- Risk: CGO dependency. May have platform-specific issues. Could limit deployment options.
- Impact: Build complexity, platform compatibility concerns.
- Migration plan: Consider pure Go implementation (e.g., `modernc.org/sqlite`) if platform issues arise.
- Priority: LOW - alternative available if needed

---

## Test Coverage Gaps

**Missing Integration Tests for Multi-Tenant Isolation:**
- What's not tested: Cross-org data access attempts. Org-id filtering verification across all endpoints.
- Files: `backend/tests/` - no dedicated multi-tenant test file
- Risk: Tenant isolation bugs could expose user data across organizations.
- Priority: HIGH

**Missing Security Tests:**
- What's not tested: SQL injection attempts, CSRF attacks, unauthorized access patterns, token manipulation.
- Files: All handler tests lack security-focused test cases.
- Risk: Security vulnerabilities could deploy undetected.
- Priority: HIGH

**Incomplete Flow Evaluation Tests:**
- What's not tested: Complex expression combinations, operator precedence, infinite loop scenarios, timeout behavior.
- Files: `backend/tests/` - no flow evaluation test file
- Risk: User-created flows could cause unexpected behavior or server issues.
- Priority: MEDIUM

**Missing Rollup Field Tests:**
- What's not tested: Rollup query execution, placeholder substitution, quoted placeholder edge case, cross-org rollups.
- Files: `backend/tests/` - no rollup-specific tests
- Risk: Rollup field functionality could break without detection.
- Priority: MEDIUM

**Frontend Component Tests:**
- What's not tested: RelatedList.svelte interactions, form validation, optimistic updates, error recovery.
- Files: `frontend/src/lib/components/` - minimal test coverage
- Risk: UI regressions could impact user experience.
- Priority: MEDIUM

**API Token Scope Enforcement Tests:**
- What's not tested: API token scope validation, privilege escalation attempts.
- Files: `backend/tests/` - no API token test file
- Risk: Scope enforcement bypass could expose privileged endpoints.
- Priority: MEDIUM

---

## Missing Critical Features

**Email Sending System:**
- Problem: Application has no email sending capability. Invitation system generates tokens but cannot send them.
- Blocks: User onboarding workflows, password reset flows, notification emails.
- Current state: Manual token sharing workaround required.
- Priority: CRITICAL - required for production

**Audit Logging:**
- Problem: No audit log system. Cannot track admin actions, security events, or user activity.
- Blocks: Compliance certifications (SOC2, GDPR), security incident investigation.
- Current state: Some applications track basic events but no centralized system.
- Priority: HIGH - compliance requirement

**Role-Based Access Control (RBAC) Enforcement:**
- Problem: User roles exist but authorization checks are minimal. Need granular permission system.
- Blocks: Complex permission scenarios, admin role customization.
- Current state: Basic role existence but incomplete enforcement.
- Priority: MEDIUM - needed for enterprise customers

**Data Export/Import with Encryption:**
- Problem: No encrypted data export for backup/migration.
- Blocks: User data recovery, compliance requirements, disaster recovery.
- Priority: MEDIUM - data safety feature

---

*Concerns audit: 2026-01-31*
