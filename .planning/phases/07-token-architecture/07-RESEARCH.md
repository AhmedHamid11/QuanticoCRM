# Phase 07: Token Architecture - Research

**Researched:** 2026-02-03
**Domain:** JWT Authentication, Refresh Token Security, XSS Prevention
**Confidence:** HIGH

## Summary

This phase transforms the current token storage architecture to prevent XSS-based token theft. Currently, the FastCRM frontend stores both access tokens and refresh tokens in localStorage, making them vulnerable to JavaScript-based attacks. The target architecture moves refresh tokens to HttpOnly cookies (inaccessible to JavaScript) while keeping access tokens in memory only.

The implementation involves three interconnected changes: (1) backend changes to set refresh tokens as HttpOnly cookies and read them from cookies on refresh requests, (2) frontend changes to store access tokens only in memory and use `credentials: 'include'` for cross-origin requests, and (3) database schema changes to support token family tracking for reuse detection.

**Primary recommendation:** Implement HttpOnly cookie-based refresh tokens with token rotation and family-based reuse detection, while keeping access tokens in memory only on the frontend.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/golang-jwt/jwt/v5 | v5.x | JWT creation/validation | Already in use, industry standard for Go JWT handling |
| github.com/gofiber/fiber/v2 | v2.x | HTTP framework with cookie support | Already in use, native Cookie API with security flags |
| SvelteKit 2.x | 2.x | Frontend with SSR cookie handling | Already in use, built-in secure cookie defaults |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| crypto/sha256 | stdlib | Token hashing | Already in use for token hash storage |
| crypto/rand | stdlib | Secure token generation | Already in use for refresh token generation |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| HttpOnly cookies | BFF (Backend for Frontend) pattern | BFF adds infrastructure complexity; direct HttpOnly cookies are simpler for this use case |
| Memory-only tokens | sessionStorage | sessionStorage is still accessible to XSS; memory-only is more secure |
| Family tracking | Simple token invalidation | Family tracking catches reuse attacks; simple invalidation misses stolen tokens |

**Installation:**
```bash
# No new dependencies required - using existing stack
```

## Architecture Patterns

### Token Flow Diagram
```
Login/Register:
1. User submits credentials
2. Backend validates, generates access_token + refresh_token
3. Backend sets refresh_token as HttpOnly cookie
4. Backend returns access_token in response body
5. Frontend stores access_token in memory (Svelte state)

API Requests:
1. Frontend sends access_token in Authorization header
2. Backend validates JWT
3. Request proceeds or returns 401

Token Refresh:
1. Access token expires (or proactive refresh before expiry)
2. Frontend calls /auth/refresh (no body needed)
3. Browser auto-sends refresh_token cookie
4. Backend validates cookie, issues new tokens + rotates refresh
5. Backend sets new refresh_token cookie
6. Backend returns new access_token in body
7. Frontend updates memory state

Logout:
1. Frontend calls /auth/logout
2. Backend invalidates token family
3. Backend clears refresh_token cookie
4. Frontend clears memory state
```

### Recommended Session Table Changes
```sql
-- Add token family support for reuse detection
ALTER TABLE sessions ADD COLUMN family_id TEXT NOT NULL;
ALTER TABLE sessions ADD COLUMN is_revoked INTEGER DEFAULT 0;
ALTER TABLE sessions ADD COLUMN previous_token_hash TEXT;

CREATE INDEX idx_sessions_family ON sessions(family_id);
CREATE INDEX idx_sessions_revoked ON sessions(family_id, is_revoked);
```

### Pattern 1: HttpOnly Cookie Setting (Backend)
**What:** Set refresh token as secure HttpOnly cookie in Fiber
**When to use:** Every time a refresh token is issued (login, register, refresh)
**Example:**
```go
// Source: Fiber official docs
func setRefreshTokenCookie(c *fiber.Ctx, token string, expiry time.Duration) {
    c.Cookie(&fiber.Cookie{
        Name:     "refresh_token",
        Value:    token,
        Path:     "/api/v1/auth", // Restrict to auth endpoints only
        Expires:  time.Now().Add(expiry),
        HTTPOnly: true,           // Prevents JavaScript access
        Secure:   isProduction(), // HTTPS only in production
        SameSite: "Strict",       // Prevent CSRF
    })
}

// Clear cookie on logout
func clearRefreshTokenCookie(c *fiber.Ctx) {
    c.Cookie(&fiber.Cookie{
        Name:     "refresh_token",
        Value:    "",
        Path:     "/api/v1/auth",
        Expires:  time.Now().Add(-time.Hour), // Expire immediately
        HTTPOnly: true,
        Secure:   isProduction(),
        SameSite: "Strict",
    })
}
```

### Pattern 2: Reading Refresh Token from Cookie (Backend)
**What:** Read refresh token from HttpOnly cookie instead of request body
**When to use:** On /auth/refresh and /auth/logout endpoints
**Example:**
```go
// Source: Fiber Cookie API
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
    // Read from cookie instead of body
    refreshToken := c.Cookies("refresh_token")
    if refreshToken == "" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "Refresh token not found",
        })
    }

    response, err := h.authService.RefreshTokens(c.Context(), refreshToken)
    if err != nil {
        // On token reuse detection, clear cookie
        if errors.Is(err, service.ErrTokenReuse) {
            clearRefreshTokenCookie(c)
        }
        return h.handleAuthError(c, err)
    }

    // Set new refresh token cookie
    setRefreshTokenCookie(c, response.RefreshToken, h.config.RefreshTokenExpiry)

    // Return access token in body (frontend stores in memory)
    return c.JSON(fiber.Map{
        "accessToken": response.AccessToken,
        "expiresAt":   response.ExpiresAt,
        "user":        response.User,
    })
}
```

### Pattern 3: Token Rotation with Family Tracking
**What:** Issue new refresh token on each use, track token families for reuse detection
**When to use:** Every refresh operation
**Example:**
```go
// Source: Auth0 refresh token rotation pattern
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (*entity.AuthResponse, error) {
    tokenHash := s.hashToken(refreshToken)

    session, err := s.repo.GetSessionByRefreshToken(ctx, tokenHash)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrInvalidToken
        }
        return nil, err
    }

    // Check if token has already been used (reuse detection)
    if session.IsRevoked {
        // Token reuse detected! Invalidate entire family
        _ = s.repo.RevokeTokenFamily(ctx, session.FamilyID)
        return nil, ErrTokenReuse
    }

    // Mark current token as revoked (it's been used)
    _ = s.repo.RevokeSession(ctx, session.ID)

    // Generate new tokens maintaining the family
    return s.createAuthResponseWithFamily(ctx, session.UserID, session.OrgID, session.FamilyID)
}

// Repository method
func (r *AuthRepo) RevokeTokenFamily(ctx context.Context, familyID string) error {
    query := `UPDATE sessions SET is_revoked = 1 WHERE family_id = ?`
    _, err := r.db.ExecContext(ctx, query, familyID)
    return err
}
```

### Pattern 4: Memory-Only Token Storage (Frontend)
**What:** Store access token only in Svelte reactive state, not in storage
**When to use:** All auth state management
**Example:**
```typescript
// Source: SvelteKit auth best practices
// Create reactive state WITHOUT localStorage persistence
let state = $state<AuthState>({
    user: null,
    currentOrg: null,
    accessToken: null,      // Memory only - lost on refresh
    expiresAt: null,
    isAuthenticated: false,
    isLoading: true
});

// Remove localStorage persistence for tokens
// Only persist user/org info (non-sensitive)
function persistState() {
    if (typeof window === 'undefined') return;
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify({
            user: state.user,
            currentOrg: state.currentOrg,
            // DO NOT persist accessToken
        }));
    } catch (e) {
        console.error('Failed to persist auth state:', e);
    }
}
```

### Pattern 5: Fetch with Credentials (Frontend)
**What:** Include cookies in cross-origin requests
**When to use:** All API calls that need authentication
**Example:**
```typescript
// Source: SvelteKit fetch credentials
async function authFetch<T>(endpoint: string, options: FetchOptions = {}): Promise<T> {
    const { method = 'GET', body, requiresAuth = true } = options;

    const headers: Record<string, string> = {
        'Content-Type': 'application/json'
    };

    // Add access token from memory for authenticated requests
    if (requiresAuth && state.accessToken) {
        headers['Authorization'] = `Bearer ${state.accessToken}`;
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
        credentials: 'include', // CRITICAL: Include cookies for refresh token
    });

    // Handle 401 - attempt silent refresh
    if (response.status === 401 && requiresAuth) {
        const refreshed = await silentRefresh();
        if (refreshed) {
            // Retry with new access token
            headers['Authorization'] = `Bearer ${state.accessToken}`;
            return authFetch(endpoint, options);
        }
        // Refresh failed, redirect to login
        await handleSessionExpired();
    }

    return response.json();
}

async function silentRefresh(): Promise<boolean> {
    try {
        // No body needed - browser sends cookie automatically
        const response = await fetch(`${API_BASE}/auth/refresh`, {
            method: 'POST',
            credentials: 'include', // Send refresh_token cookie
        });

        if (response.ok) {
            const data = await response.json();
            state.accessToken = data.accessToken;
            state.expiresAt = new Date(data.expiresAt);
            return true;
        }
        return false;
    } catch {
        return false;
    }
}
```

### Pattern 6: CORS Configuration for Credentials
**What:** Configure CORS to allow credentials from frontend origin
**When to use:** Backend CORS middleware setup
**Example:**
```go
// Source: Fiber CORS middleware
app.Use(cors.New(cors.Config{
    AllowOrigins:     frontendOrigin, // Specific origin, not "*"
    AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
    AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
    AllowCredentials: true, // CRITICAL: Required for cookies
    ExposeHeaders:    "Content-Length",
}))
```

### Anti-Patterns to Avoid
- **Storing access tokens in localStorage/sessionStorage:** Vulnerable to XSS attacks
- **Storing refresh tokens in localStorage:** Can be stolen by malicious scripts
- **Setting SameSite=None without Secure:** Browser will reject the cookie
- **Using CORS AllowOrigins="*" with AllowCredentials=true:** Not allowed by CORS spec
- **Not rotating refresh tokens:** Compromised token stays valid until expiry
- **Simple token invalidation without family tracking:** Misses token theft detection

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JWT signing | Custom HMAC implementation | golang-jwt/jwt library | Timing attack vulnerabilities, algorithm confusion |
| Secure random tokens | math/rand | crypto/rand | math/rand is predictable |
| Cookie security flags | Manual header setting | Fiber's Cookie struct | Easy to miss required combinations |
| Token hashing | MD5 or simple hash | SHA-256 | Collision resistance matters for security |

**Key insight:** Token security is about defense in depth. Each layer (HttpOnly, Secure, SameSite, rotation, family tracking) provides protection against different attack vectors. Missing any layer creates vulnerabilities.

## Common Pitfalls

### Pitfall 1: Forgetting credentials: 'include' on Frontend
**What goes wrong:** Cookies don't get sent to API, refresh always fails
**Why it happens:** fetch() defaults to same-origin credentials only
**How to avoid:** Always use `credentials: 'include'` for API calls
**Warning signs:** Refresh endpoint always returns 401, cookies not in request headers

### Pitfall 2: CORS Misconfiguration
**What goes wrong:** Browser blocks preflight requests or cookies
**Why it happens:** AllowCredentials requires specific origin (not "*")
**How to avoid:** Set AllowOrigins to exact frontend origin, AllowCredentials=true
**Warning signs:** CORS errors in browser console, cookies not sent

### Pitfall 3: Cookie Path Too Broad
**What goes wrong:** Refresh token sent to all API endpoints unnecessarily
**Why it happens:** Default path is "/" which matches everything
**How to avoid:** Set Path="/api/v1/auth" to limit cookie scope
**Warning signs:** refresh_token in requests to non-auth endpoints

### Pitfall 4: Missing Secure Flag in Production
**What goes wrong:** Cookies sent over HTTP, vulnerable to MITM attacks
**Why it happens:** Forgetting to check production environment
**How to avoid:** Always set Secure=true in production, detect via environment variable
**Warning signs:** HSTS active but cookies not Secure

### Pitfall 5: Not Handling Page Refresh
**What goes wrong:** User loses session after browser refresh
**Why it happens:** Access token was in memory, lost on page load
**How to avoid:** On page load, attempt silent refresh using cookie before showing UI
**Warning signs:** Users constantly getting logged out on page refresh

### Pitfall 6: Token Family Cleanup
**What goes wrong:** sessions table grows unboundedly
**Why it happens:** Revoked sessions never deleted
**How to avoid:** Scheduled cleanup of expired/revoked sessions older than N days
**Warning signs:** Slow session queries, large table size

### Pitfall 7: Development vs Production Cookie Behavior
**What goes wrong:** Works locally, fails in production
**Why it happens:** Secure cookies require HTTPS; localhost gets special treatment
**How to avoid:** Test with HTTPS locally using mkcert, or disable Secure in dev only
**Warning signs:** Works on localhost:3000, fails on production domain

## Code Examples

### Complete Login Response Handler (Backend)
```go
// Source: Combined from Fiber docs and auth best practices
func (h *AuthHandler) Login(c *fiber.Ctx) error {
    var input entity.LoginInput
    if err := c.BodyParser(&input); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid request body",
        })
    }

    response, err := h.authService.Login(c.Context(), input)
    if err != nil {
        return h.handleAuthError(c, err)
    }

    // Set refresh token as HttpOnly cookie
    c.Cookie(&fiber.Cookie{
        Name:     "refresh_token",
        Value:    response.RefreshToken,
        Path:     "/api/v1/auth",
        Expires:  time.Now().Add(h.config.RefreshTokenExpiry),
        HTTPOnly: true,
        Secure:   h.isProduction(),
        SameSite: "Strict",
    })

    // Return access token in body only (not refresh token)
    return c.JSON(fiber.Map{
        "accessToken": response.AccessToken,
        "expiresAt":   response.ExpiresAt,
        "user":        response.User,
    })
}
```

### Complete Refresh Handler (Backend)
```go
// Source: Combined from Auth0 patterns and Fiber docs
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
    refreshToken := c.Cookies("refresh_token")
    if refreshToken == "" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "Refresh token not found",
        })
    }

    response, err := h.authService.RefreshTokens(c.Context(), refreshToken)
    if err != nil {
        // On any auth error, clear the cookie
        c.Cookie(&fiber.Cookie{
            Name:     "refresh_token",
            Value:    "",
            Path:     "/api/v1/auth",
            Expires:  time.Now().Add(-time.Hour),
            HTTPOnly: true,
            Secure:   h.isProduction(),
            SameSite: "Strict",
        })
        return h.handleAuthError(c, err)
    }

    // Set new refresh token cookie (rotation)
    c.Cookie(&fiber.Cookie{
        Name:     "refresh_token",
        Value:    response.RefreshToken,
        Path:     "/api/v1/auth",
        Expires:  time.Now().Add(h.config.RefreshTokenExpiry),
        HTTPOnly: true,
        Secure:   h.isProduction(),
        SameSite: "Strict",
    })

    return c.JSON(fiber.Map{
        "accessToken": response.AccessToken,
        "expiresAt":   response.ExpiresAt,
        "user":        response.User,
    })
}
```

### Token Family Session Creation (Backend)
```go
// Source: Auth0 refresh token rotation pattern
func (s *AuthService) createAuthResponseWithFamily(
    ctx context.Context,
    user *entity.User,
    orgID string,
    familyID string, // nil for new login, existing for refresh
) (*entity.AuthResponse, error) {
    // Generate new family ID for new logins
    if familyID == "" {
        familyID = sfid.NewTokenFamily()
    }

    // Generate new refresh token
    refreshToken, err := s.generateSecureToken(32)
    if err != nil {
        return nil, err
    }

    // Hash for storage
    refreshTokenHash := s.hashToken(refreshToken)

    // Create session with family tracking
    _, err = s.repo.CreateSessionWithFamily(ctx, CreateSessionInput{
        UserID:           user.ID,
        OrgID:            orgID,
        RefreshTokenHash: refreshTokenHash,
        FamilyID:         familyID,
        ExpiresAt:        time.Now().Add(s.config.RefreshTokenExpiry),
    })
    if err != nil {
        return nil, err
    }

    // Generate access token (unchanged)
    accessToken, expiresAt, err := s.generateAccessToken(user, orgID)
    if err != nil {
        return nil, err
    }

    return &entity.AuthResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresAt:    expiresAt,
        User:         user,
    }, nil
}
```

### Frontend Auth Initialization on Page Load
```typescript
// Source: SvelteKit auth patterns
export async function initAuth(): Promise<void> {
    state.isLoading = true;

    // Try to restore user info from localStorage (non-sensitive)
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
        try {
            const data = JSON.parse(stored);
            state.user = data.user;
            state.currentOrg = data.currentOrg;
        } catch (e) {
            console.error('Failed to parse stored auth:', e);
        }
    }

    // If we have user info, try silent refresh to get access token
    if (state.user) {
        const refreshed = await silentRefresh();
        if (refreshed) {
            state.isAuthenticated = true;
        } else {
            // Refresh failed, clear state
            state.user = null;
            state.currentOrg = null;
            localStorage.removeItem(STORAGE_KEY);
        }
    }

    state.isLoading = false;
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| localStorage tokens | HttpOnly cookies for refresh | 2020+ | XSS cannot steal refresh tokens |
| No token rotation | Rotate on every use | 2021+ | Stolen tokens detected quickly |
| Simple invalidation | Family-based revocation | 2022+ | Catches token replay attacks |
| Long-lived access tokens | Short-lived (15-30 min) + refresh | 2019+ | Limits exposure window |

**Deprecated/outdated:**
- Storing any tokens in localStorage (XSS vulnerable)
- Single long-lived tokens without refresh mechanism
- Refresh tokens without rotation
- SameSite=None without understanding the CSRF implications

## Open Questions

Things that couldn't be fully resolved:

1. **Grace Period for Token Rotation**
   - What we know: Network issues can cause client to miss new token
   - What's unclear: Optimal grace period length for FastCRM use case
   - Recommendation: Start with 5-10 seconds, monitor failed refreshes

2. **Mobile App Compatibility**
   - What we know: Mobile apps may not support cookies the same way
   - What's unclear: Future mobile app plans for FastCRM
   - Recommendation: Current web-only approach is fine; revisit for mobile

3. **Cross-Tab Token Synchronization**
   - What we know: Memory-only tokens don't sync between browser tabs
   - What's unclear: User expectation for multi-tab behavior
   - Recommendation: Use BroadcastChannel API for token sync between tabs

## Sources

### Primary (HIGH confidence)
- [Fiber Cookie API docs](https://docs.gofiber.io/api/ctx/#cookie) - Cookie struct and security attributes
- [Auth0 Refresh Token Rotation](https://auth0.com/docs/secure/tokens/refresh-tokens/refresh-token-rotation) - Family tracking, reuse detection
- [golang-jwt/jwt v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) - JWT validation best practices

### Secondary (MEDIUM confidence)
- [SvelteKit Auth Patterns](https://dev.to/jiprochazka/sending-sveltekit-server-requests-with-httponly-cookies-56p6) - Credentials and cookie handling
- [Security Boulevard: Refresh Token Best Practices 2026](https://securityboulevard.com/2026/01/what-are-refresh-tokens-complete-implementation-guide-security-best-practices/) - Current best practices
- [Okta Token Rotation Guide](https://developer.okta.com/docs/guides/refresh-tokens/main/) - Industry standard implementation

### Tertiary (LOW confidence)
- Various DEV.to articles on JWT implementation patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using existing libraries already in codebase
- Architecture: HIGH - Well-established patterns from Auth0, Okta
- Pitfalls: MEDIUM - Based on common issues reported in search results
- Cookie/CORS interaction: HIGH - Verified against Fiber and browser specs

**Research date:** 2026-02-03
**Valid until:** 2026-03-03 (30 days - stable security patterns)
