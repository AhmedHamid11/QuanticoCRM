# Phase 09: Session Management - Research

**Researched:** 2026-02-04
**Domain:** Session Lifecycle, CSRF Protection, Multi-Tenant Isolation Testing
**Confidence:** HIGH

## Summary

This phase implements session lifecycle management with configurable timeouts, CSRF protection, and comprehensive tenant isolation testing. The existing codebase (from Phase 07) already has HttpOnly cookie-based refresh tokens with token rotation and family-based reuse detection. This phase adds timeout enforcement (idle + absolute), CSRF protection for state-changing requests, and integration tests proving no data leakage between tenants.

The approach builds on the existing architecture: the backend already tracks sessions with `created_at` and `expires_at` fields, allowing us to add `last_activity_at` for idle timeout and use `created_at` for absolute timeout calculations. The frontend already has a toast system and auth store that can be extended to show session warnings. CSRF will use Fiber's built-in CSRF middleware with double-submit cookie pattern.

**Primary recommendation:** Extend the existing session table with timeout fields, implement CSRF middleware with header-based token delivery, and create matrix-based integration tests covering all CRUD endpoints across multiple orgs.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/gofiber/fiber/v2/middleware/csrf | v2.52+ | CSRF protection | Built-in Fiber middleware, well-tested, secure defaults |
| Go testing package | stdlib | Integration tests | Native Go, t.Parallel() support, table-driven tests |
| svelte-toast (existing) | custom | Session warnings | Already in codebase, simple to extend for countdown |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| crypto/rand | stdlib | CSRF token generation | Secure random for tokens (Fiber uses internally) |
| time | stdlib | Timeout calculations | Duration arithmetic, comparisons |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Fiber CSRF middleware | Custom CSRF implementation | Fiber middleware is well-tested; custom adds maintenance burden |
| Double-submit cookie | Synchronizer token pattern | Double-submit simpler (stateless); synchronizer requires session storage |
| Header-based CSRF | Meta tag or endpoint delivery | Header cleaner for SPA; meta tag requires DOM parsing |

**Installation:**
```bash
# No new dependencies - using existing Fiber CSRF middleware
```

## Architecture Patterns

### Recommended Project Structure
```
backend/
  internal/
    middleware/
      csrf.go           # CSRF middleware configuration
      session_timeout.go # Timeout enforcement middleware
    entity/
      session.go        # Extended with timeout fields
    service/
      session.go        # Session timeout logic
    repo/
      session.go        # Session queries with activity tracking
  tests/
    isolation_test.go   # Multi-tenant isolation tests
    csrf_test.go        # CSRF protection tests

frontend/
  src/lib/
    stores/
      session.svelte.ts # Session timeout tracking, warning state
    components/
      SessionWarning.svelte # Countdown toast component
```

### Pattern 1: Session Timeout Tracking
**What:** Track both idle time and absolute session age
**When to use:** Every authenticated request
**Example:**
```go
// Source: Fiber session middleware patterns
type Session struct {
    // Existing fields...
    ID               string     `json:"id" db:"id"`
    UserID           string     `json:"userId" db:"user_id"`
    OrgID            string     `json:"orgId" db:"org_id"`
    FamilyID         string     `json:"familyId" db:"family_id"`
    // New timeout fields
    CreatedAt        time.Time  `json:"createdAt" db:"created_at"`      // For absolute timeout
    LastActivityAt   time.Time  `json:"lastActivityAt" db:"last_activity_at"` // For idle timeout
    IdleTimeout      int        `json:"idleTimeout" db:"idle_timeout"`   // Minutes, from org settings
    AbsoluteTimeout  int        `json:"absoluteTimeout" db:"absolute_timeout"` // Minutes, from org settings
}

// Timeout check in middleware
func (m *TimeoutMiddleware) CheckTimeout(c *fiber.Ctx) error {
    session := getSessionFromContext(c)
    now := time.Now()

    // Check absolute timeout first
    sessionAge := now.Sub(session.CreatedAt)
    if sessionAge > time.Duration(session.AbsoluteTimeout)*time.Minute {
        return m.handleSessionExpired(c, "absolute")
    }

    // Check idle timeout
    idleTime := now.Sub(session.LastActivityAt)
    if idleTime > time.Duration(session.IdleTimeout)*time.Minute {
        return m.handleSessionExpired(c, "idle")
    }

    return c.Next()
}
```

### Pattern 2: Per-Org Timeout Configuration
**What:** Store timeout settings per organization
**When to use:** Admin settings panel, session creation
**Example:**
```go
// Source: Existing OrgSettings pattern in codebase
type SessionSettings struct {
    IdleTimeoutMinutes     int `json:"idleTimeoutMinutes"`     // 15-60, default 30
    AbsoluteTimeoutMinutes int `json:"absoluteTimeoutMinutes"` // 480-4320 (8h-72h), default 1440 (24h)
    WarningMinutes         int `json:"warningMinutes"`         // Fixed 5 minutes
}

// Bounds enforcement
func ValidateSessionSettings(s *SessionSettings) error {
    if s.IdleTimeoutMinutes < 15 || s.IdleTimeoutMinutes > 60 {
        return errors.New("idle timeout must be between 15 and 60 minutes")
    }
    if s.AbsoluteTimeoutMinutes < 480 || s.AbsoluteTimeoutMinutes > 4320 {
        return errors.New("absolute timeout must be between 8 and 72 hours")
    }
    return nil
}
```

### Pattern 3: CSRF Double-Submit Cookie Pattern
**What:** CSRF token in cookie AND header, compared server-side
**When to use:** All POST/PUT/DELETE/PATCH requests
**Example:**
```go
// Source: Fiber CSRF middleware docs
app.Use(csrf.New(csrf.Config{
    KeyLookup:      "header:X-CSRF-Token",
    CookieName:     "csrf_token",
    CookieSameSite: "Strict",
    CookieSecure:   isProduction,
    CookieHTTPOnly: false, // Must be false - JS needs to read for header
    Expiration:     1 * time.Hour,
    KeyGenerator:   utils.UUIDv4,
    ErrorHandler: func(c *fiber.Ctx, err error) error {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "Invalid or missing CSRF token",
            "code":  "CSRF_VALIDATION_FAILED",
        })
    },
    Next: func(c *fiber.Ctx) bool {
        // Skip CSRF for:
        // 1. Safe methods (GET, HEAD, OPTIONS)
        // 2. API token requests (Bearer token = fcr_ prefix)
        method := c.Method()
        if method == "GET" || method == "HEAD" || method == "OPTIONS" {
            return true
        }
        auth := c.Get("Authorization")
        if strings.HasPrefix(auth, "Bearer fcr_") {
            return true // API tokens are exempt
        }
        return false
    },
}))
```

### Pattern 4: Frontend Session Warning with Countdown
**What:** Toast with countdown timer, explicit "Stay logged in" button
**When to use:** 5 minutes before either timeout expires
**Example:**
```typescript
// Source: Svelte 5 runes pattern
// session.svelte.ts
let sessionState = $state({
    warningVisible: false,
    secondsRemaining: 300,
    expiryType: 'idle' as 'idle' | 'absolute',
    idleTimeout: 30 * 60 * 1000,   // From org settings
    absoluteTimeout: 24 * 60 * 60 * 1000,
    sessionStart: Date.now(),
    lastActivity: Date.now(),
});

// Activity tracking (clicks and keystrokes only, not mouse movement)
function trackActivity() {
    sessionState.lastActivity = Date.now();
    // Note: This does NOT extend the session - user must click "Stay logged in"
}

// Check for warning display (called on interval)
function checkSessionTimeout() {
    const now = Date.now();
    const warningThreshold = 5 * 60 * 1000; // 5 minutes

    // Check idle first
    const idleTime = now - sessionState.lastActivity;
    const idleRemaining = sessionState.idleTimeout - idleTime;

    if (idleRemaining <= warningThreshold && idleRemaining > 0) {
        showWarning('idle', Math.floor(idleRemaining / 1000));
        return;
    }

    // Check absolute
    const sessionAge = now - sessionState.sessionStart;
    const absoluteRemaining = sessionState.absoluteTimeout - sessionAge;

    if (absoluteRemaining <= warningThreshold && absoluteRemaining > 0) {
        showWarning('absolute', Math.floor(absoluteRemaining / 1000));
        return;
    }

    // Check if expired
    if (idleRemaining <= 0 || absoluteRemaining <= 0) {
        handleSessionExpired();
    }
}

// Called when user clicks "Stay logged in"
export async function extendSession() {
    await authFetch('/auth/extend-session', { method: 'POST' });
    sessionState.lastActivity = Date.now();
    sessionState.warningVisible = false;
}
```

### Pattern 5: Matrix-Based Tenant Isolation Tests
**What:** Test all CRUD operations across multiple orgs, verify no cross-tenant access
**When to use:** Integration test suite
**Example:**
```go
// Source: Go table-driven test patterns
func TestTenantIsolation(t *testing.T) {
    // Setup: Create two orgs with users
    app := SetupTestApp(t)
    defer app.Cleanup()

    org1User := app.CreateTestUser(t, "user1@org1.com", "password123", "Org 1")
    org2User := app.CreateTestUser(t, "user2@org2.com", "password123", "Org 2")

    // Create test data in each org
    contact1 := app.CreateContact(t, org1User.AccessToken, "Contact in Org 1")
    contact2 := app.CreateContact(t, org2User.AccessToken, "Contact in Org 2")

    // Define test matrix
    testCases := []struct {
        name        string
        actor       *TestUser    // Who is making the request
        targetOrg   string       // Whose data they're trying to access
        targetID    string       // Specific record ID
        endpoint    string       // API endpoint
        method      string       // HTTP method
        expectCode  int          // Expected status code
    }{
        // Org 1 user accessing Org 1 data (should succeed)
        {"org1_reads_own_contact", org1User, org1User.OrgID, contact1.ID, "/contacts/{id}", "GET", 200},
        {"org1_updates_own_contact", org1User, org1User.OrgID, contact1.ID, "/contacts/{id}", "PUT", 200},
        {"org1_deletes_own_contact", org1User, org1User.OrgID, contact1.ID, "/contacts/{id}", "DELETE", 200},

        // Org 1 user accessing Org 2 data (should fail)
        {"org1_reads_org2_contact", org1User, org2User.OrgID, contact2.ID, "/contacts/{id}", "GET", 404},
        {"org1_updates_org2_contact", org1User, org2User.OrgID, contact2.ID, "/contacts/{id}", "PUT", 404},
        {"org1_deletes_org2_contact", org1User, org2User.OrgID, contact2.ID, "/contacts/{id}", "DELETE", 404},

        // Same pattern for Org 2
        {"org2_reads_own_contact", org2User, org2User.OrgID, contact2.ID, "/contacts/{id}", "GET", 200},
        {"org2_reads_org1_contact", org2User, org1User.OrgID, contact1.ID, "/contacts/{id}", "GET", 404},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            endpoint := strings.Replace(tc.endpoint, "{id}", tc.targetID, 1)
            resp := app.MakeRequest(t, tc.method, "/api/v1"+endpoint, nil, tc.actor.AccessToken)
            AssertStatus(t, resp, tc.expectCode)
        })
    }
}
```

### Pattern 6: Impersonation Isolation Test
**What:** Verify platform admin is fully scoped when impersonating
**When to use:** Security-critical impersonation feature testing
**Example:**
```go
// Source: FastCRM impersonation requirements
func TestImpersonationIsolation(t *testing.T) {
    app := SetupTestApp(t)
    defer app.Cleanup()

    // Setup: Platform admin, two orgs
    platformAdmin := app.CreatePlatformAdmin(t, "admin@platform.com", "password123")
    org1User := app.CreateTestUser(t, "user1@org1.com", "password123", "Org 1")
    org2User := app.CreateTestUser(t, "user2@org2.com", "password123", "Org 2")

    // Create data in both orgs
    contact1 := app.CreateContact(t, org1User.AccessToken, "Contact in Org 1")
    contact2 := app.CreateContact(t, org2User.AccessToken, "Contact in Org 2")

    // Platform admin impersonates Org 1
    impersonationToken := app.Impersonate(t, platformAdmin.AccessToken, org1User.OrgID)

    // Verify: Can see Org 1 data
    resp := app.MakeRequest(t, "GET", "/api/v1/contacts/"+contact1.ID, nil, impersonationToken)
    AssertStatus(t, resp, 200)

    // Verify: CANNOT see Org 2 data
    resp = app.MakeRequest(t, "GET", "/api/v1/contacts/"+contact2.ID, nil, impersonationToken)
    AssertStatus(t, resp, 404) // Should not find - different org

    // Verify: Cannot access platform admin endpoints while impersonating
    resp = app.MakeRequest(t, "GET", "/api/v1/platform/organizations", nil, impersonationToken)
    AssertStatus(t, resp, 403) // Forbidden during impersonation
}
```

### Anti-Patterns to Avoid
- **Extending session on any activity:** Decision requires explicit "Stay logged in" click
- **Mouse movement resetting idle timer:** Only clicks and keystrokes count per requirements
- **Preserving form data on timeout:** Warning is the protection, not data preservation
- **CSRF token in localStorage:** Defeats XSS protection; must be in cookie
- **Testing isolation only at API level:** Must verify at database query level too
- **Shared test data between orgs:** Each test should create isolated fixtures

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CSRF token generation | Custom UUID/random | Fiber CSRF middleware | Timing-safe comparison, proper entropy |
| Cookie security flags | Manual Set-Cookie header | Fiber Cookie struct | Easy to miss HTTPOnly, Secure, SameSite combo |
| Countdown timer | setInterval math | RequestAnimationFrame + $effect | More accurate, doesn't drift |
| Multi-org test isolation | Shared test database | Per-test user/org creation | Prevents test interdependence |

**Key insight:** Session security requires multiple layers (idle timeout, absolute timeout, CSRF, isolation) working together. Missing any layer creates vulnerabilities that appear as "edge cases" but are actually security holes.

## Common Pitfalls

### Pitfall 1: Race Condition in Session Extension
**What goes wrong:** User clicks "Stay logged in" but session expires between click and server response
**Why it happens:** Network latency during the warning period
**How to avoid:** Add grace period (10-30 seconds) on server side; don't reject extends that arrive slightly late
**Warning signs:** Users report "couldn't extend session" despite clicking button

### Pitfall 2: CSRF Token Not Sent on First Request After Page Load
**What goes wrong:** First mutating request fails with CSRF error
**Why it happens:** Cookie not yet set when SPA makes first POST
**How to avoid:** Call a GET endpoint (like /auth/me) on page load to establish CSRF cookie
**Warning signs:** CSRF errors only on fresh page loads, works after any GET request

### Pitfall 3: Impersonation Context Leaking to Wrong Org
**What goes wrong:** Platform admin sees data from multiple orgs while impersonating
**Why it happens:** orgID context not properly scoped in all middleware paths
**How to avoid:** Always use claims.OrgID from JWT, never trust route params for org context
**Warning signs:** Integration tests pass individually but fail when run in parallel

### Pitfall 4: Idle Timeout Reset on API Polling
**What goes wrong:** Background refresh/polling keeps session alive indefinitely
**Why it happens:** All authenticated requests updating last_activity_at
**How to avoid:** Only update idle timer for user-initiated requests (exclude /auth/refresh, polling endpoints)
**Warning signs:** Sessions never expire despite user walking away

### Pitfall 5: CSRF Cookie vs Auth Cookie Conflict
**What goes wrong:** CSRF validation fails after session refresh
**Why it happens:** CSRF and refresh cookies have different expiration/rotation schedules
**How to avoid:** Ensure CSRF expiration >= auth token expiration; rotate CSRF with auth
**Warning signs:** CSRF errors appear ~1 hour after login (default CSRF expiry)

### Pitfall 6: Test Parallelism Breaking Isolation
**What goes wrong:** Tests pass locally, fail in CI
**Why it happens:** Parallel tests share database state
**How to avoid:** Each test creates its own users/orgs/data; never rely on pre-existing data
**Warning signs:** Flaky tests, "data already exists" errors

## Code Examples

### Complete CSRF Middleware Setup (Backend)
```go
// Source: Fiber CSRF middleware docs + FastCRM patterns
package middleware

import (
    "strings"
    "time"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/csrf"
    "github.com/gofiber/fiber/v2/utils"
)

// CSRFConfig returns the CSRF middleware configured for FastCRM
func CSRFConfig(isProduction bool) csrf.Config {
    return csrf.Config{
        // Token delivery: Cookie + Header
        KeyLookup:      "header:X-CSRF-Token",
        CookieName:     "csrf_token",
        CookiePath:     "/",
        CookieDomain:   "",  // Current domain only
        CookieSecure:   isProduction,
        CookieHTTPOnly: false, // JS must read cookie to set header
        CookieSameSite: "Strict",

        // Token lifecycle
        Expiration:     1 * time.Hour,
        KeyGenerator:   utils.UUIDv4,

        // Error handling
        ErrorHandler: func(c *fiber.Ctx, err error) error {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "CSRF validation failed",
                "code":  "CSRF_INVALID",
            })
        },

        // Skip CSRF for safe methods and API tokens
        Next: func(c *fiber.Ctx) bool {
            method := c.Method()

            // Safe methods don't need CSRF
            if method == "GET" || method == "HEAD" || method == "OPTIONS" || method == "TRACE" {
                return true
            }

            // API tokens (fcr_ prefix) are exempt - they're not browser-based
            auth := c.Get("Authorization")
            if strings.HasPrefix(auth, "Bearer fcr_") {
                return true
            }

            return false
        },
    }
}
```

### Session Warning Component (Frontend)
```svelte
<!-- SessionWarning.svelte -->
<script lang="ts">
    import { sessionState, extendSession, handleSessionExpired } from '$lib/stores/session.svelte';

    // Countdown effect
    $effect(() => {
        if (!sessionState.warningVisible) return;

        const interval = setInterval(() => {
            sessionState.secondsRemaining--;
            if (sessionState.secondsRemaining <= 0) {
                clearInterval(interval);
                handleSessionExpired();
            }
        }, 1000);

        return () => clearInterval(interval);
    });

    function formatTime(seconds: number): string {
        const mins = Math.floor(seconds / 60);
        const secs = seconds % 60;
        return `${mins}:${secs.toString().padStart(2, '0')}`;
    }

    async function handleStayLoggedIn() {
        await extendSession();
    }
</script>

{#if sessionState.warningVisible}
    <div class="fixed bottom-4 right-4 bg-yellow-50 border border-yellow-200 rounded-lg shadow-lg p-4 max-w-sm z-50">
        <div class="flex items-start gap-3">
            <div class="flex-shrink-0">
                <svg class="w-5 h-5 text-yellow-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
                </svg>
            </div>
            <div class="flex-1">
                <p class="text-sm font-medium text-yellow-800">
                    Session expires in {formatTime(sessionState.secondsRemaining)}
                </p>
                <p class="mt-1 text-xs text-yellow-700">
                    Click below to stay logged in
                </p>
                <button
                    onclick={handleStayLoggedIn}
                    class="mt-2 px-3 py-1.5 text-sm font-medium text-yellow-800 bg-yellow-100 hover:bg-yellow-200 rounded transition-colors"
                >
                    Stay logged in
                </button>
            </div>
        </div>
    </div>
{/if}
```

### Complete Tenant Isolation Test Suite
```go
// isolation_test.go
package tests

import (
    "net/http"
    "testing"
)

// TestTenantIsolation_AllEndpoints verifies no cross-tenant data access
func TestTenantIsolation_AllEndpoints(t *testing.T) {
    app := SetupTestApp(t)
    defer app.Cleanup()

    // Create two orgs
    org1 := app.CreateTestUser(t, "user@org1.com", "password123", "Org One")
    org2 := app.CreateTestUser(t, "user@org2.com", "password123", "Org Two")

    // Create test records in Org 1
    contact1 := app.CreateContact(t, org1.AccessToken, map[string]string{
        "firstName": "Test",
        "lastName":  "Contact",
    })
    account1 := app.CreateAccount(t, org1.AccessToken, map[string]string{
        "name": "Test Account",
    })
    task1 := app.CreateTask(t, org1.AccessToken, map[string]string{
        "subject": "Test Task",
    })

    // Entity endpoints to test
    entities := []struct {
        name     string
        endpoint string
        id       string
    }{
        {"Contact", "/api/v1/contacts", contact1.ID},
        {"Account", "/api/v1/accounts", account1.ID},
        {"Task", "/api/v1/tasks", task1.ID},
    }

    // CRUD operations
    operations := []struct {
        method     string
        pathSuffix string
        expectCode int
    }{
        {"GET", "/{id}", http.StatusNotFound},     // Read by ID
        {"PUT", "/{id}", http.StatusNotFound},     // Update
        {"DELETE", "/{id}", http.StatusNotFound},  // Delete
        {"GET", "", http.StatusOK},                // List (returns empty, not 404)
    }

    for _, entity := range entities {
        for _, op := range operations {
            testName := entity.name + "_" + op.method + "_from_other_org"
            t.Run(testName, func(t *testing.T) {
                path := entity.endpoint
                if op.pathSuffix == "/{id}" {
                    path += "/" + entity.id
                }

                var body interface{}
                if op.method == "PUT" {
                    body = map[string]string{"name": "Modified"}
                }

                // Org 2 user tries to access Org 1's data
                resp := app.MakeRequest(t, op.method, path, body, org2.AccessToken)

                if op.pathSuffix == "" {
                    // List endpoint should return empty, not 404
                    AssertStatus(t, resp, http.StatusOK)
                    // Verify empty results
                    var result map[string]interface{}
                    app.DecodeResponse(t, resp, &result)
                    data := result["data"].([]interface{})
                    if len(data) > 0 {
                        t.Errorf("Expected empty list, got %d items", len(data))
                    }
                } else {
                    AssertStatus(t, resp, op.expectCode)
                }
            })
        }
    }
}

// TestImpersonation_FullIsolation verifies platform admin is scoped during impersonation
func TestImpersonation_FullIsolation(t *testing.T) {
    app := SetupTestApp(t)
    defer app.Cleanup()

    // Create platform admin and two orgs
    admin := app.CreatePlatformAdmin(t, "admin@platform.com", "password123")
    org1 := app.CreateTestUser(t, "user@org1.com", "password123", "Org One")
    org2 := app.CreateTestUser(t, "user@org2.com", "password123", "Org Two")

    // Create data in both orgs
    contact1 := app.CreateContact(t, org1.AccessToken, map[string]string{"firstName": "Org1 Contact"})
    contact2 := app.CreateContact(t, org2.AccessToken, map[string]string{"firstName": "Org2 Contact"})

    // Admin impersonates Org 1
    impToken := app.Impersonate(t, admin.AccessToken, org1.OrgID, "")

    t.Run("can_access_impersonated_org_data", func(t *testing.T) {
        resp := app.MakeRequest(t, "GET", "/api/v1/contacts/"+contact1.ID, nil, impToken)
        AssertStatus(t, resp, http.StatusOK)
    })

    t.Run("cannot_access_other_org_data", func(t *testing.T) {
        resp := app.MakeRequest(t, "GET", "/api/v1/contacts/"+contact2.ID, nil, impToken)
        AssertStatus(t, resp, http.StatusNotFound)
    })

    t.Run("cannot_list_other_org_data", func(t *testing.T) {
        resp := app.MakeRequest(t, "GET", "/api/v1/contacts", nil, impToken)
        AssertStatus(t, resp, http.StatusOK)

        var result map[string]interface{}
        app.DecodeResponse(t, resp, &result)
        data := result["data"].([]interface{})

        // Should only see Org 1's contact
        for _, item := range data {
            c := item.(map[string]interface{})
            if c["firstName"] == "Org2 Contact" {
                t.Error("Saw Org 2 data while impersonating Org 1")
            }
        }
    })

    t.Run("platform_admin_endpoints_blocked", func(t *testing.T) {
        // This endpoint is already blocked per existing middleware
        resp := app.MakeRequest(t, "GET", "/api/v1/platform/organizations", nil, impToken)
        AssertStatus(t, resp, http.StatusForbidden)
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Fixed 30-min timeout | Configurable per-org | 2024+ | Better UX for different security needs |
| Session extension on any activity | Explicit button click | 2023+ | Clearer security boundary |
| CSRF in session storage | Double-submit cookie | 2022+ | Stateless, scalable |
| Single CSRF token per session | Rotating tokens | 2023+ | Better compromise detection |

**Deprecated/outdated:**
- CSRF meta tags in HTML (SPAs don't have server-rendered HTML)
- Session-storage CSRF (still vulnerable to XSS)
- "Remember me" extending past absolute timeout (security risk)

## Open Questions

Things that couldn't be fully resolved:

1. **Browser Tab Synchronization**
   - What we know: Session warning should appear in all tabs
   - What's unclear: Optimal approach for multi-tab sync in Svelte 5
   - Recommendation: Use BroadcastChannel API to sync warning state across tabs

2. **Timeout Granularity**
   - What we know: Requirements specify 15-60min idle, 8-72h absolute
   - What's unclear: Whether 1-minute granularity is sufficient for UI
   - Recommendation: Store in minutes, display in human-readable format

3. **CI Test Parallelism**
   - What we know: Tests must pass in parallel
   - What's unclear: Optimal -parallel flag value for CI environment
   - Recommendation: Start with GOMAXPROCS (default), tune based on CI metrics

## Sources

### Primary (HIGH confidence)
- [Fiber CSRF middleware](https://pkg.go.dev/github.com/gofiber/fiber/v2/middleware/csrf) - Configuration options, token handling
- [Fiber Session middleware](https://github.com/gofiber/fiber/blob/main/docs/middleware/session.md) - IdleTimeout, AbsoluteTimeout patterns
- Existing FastCRM codebase - auth.go, auth.svelte.ts, tests/setup_test.go

### Secondary (MEDIUM confidence)
- [Multi-tenant isolation testing](https://testgrid.io/blog/multi-tenancy/) - Testing patterns and strategies
- [Go parallel testing](https://www.glukhov.org/post/2025/12/parallel-table-driven-tests-in-go/) - Table-driven parallel test patterns
- [Bootstrap Session Timeout](https://github.com/orangehill/bootstrap-session-timeout) - Countdown timer UX patterns

### Tertiary (LOW confidence)
- Various DEV.to articles on session management patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using existing Fiber middleware, Go stdlib
- Architecture: HIGH - Extends existing patterns in codebase
- Pitfalls: MEDIUM - Based on common issues, some from experience
- CSRF: HIGH - Fiber middleware is well-documented
- Isolation tests: HIGH - Follows existing test patterns in codebase

**Research date:** 2026-02-04
**Valid until:** 2026-03-04 (30 days - stable patterns)
