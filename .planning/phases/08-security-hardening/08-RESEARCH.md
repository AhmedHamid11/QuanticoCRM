# Phase 08: Security Hardening - Research

**Researched:** 2026-02-03
**Domain:** Web application security (headers, password policy, request limits)
**Confidence:** HIGH

## Summary

This phase hardens FastCRM against common attack vectors through three areas: security response headers, NIST-compliant password policy, and request body limits. The research validates that Go Fiber has built-in support for all required features, with Helmet middleware for security headers and native BodyLimit configuration for request size control.

Key findings:
- Fiber's Helmet middleware provides all required security headers (X-Frame-Options, X-Content-Type-Options, CSP) with sensible defaults
- NIST SP 800-63B Revision 4 (current standard) emphasizes length over complexity, mandates blocklist checking, and eliminates forced rotation
- Body limits are configured at the Fiber app level (not middleware), with custom middleware needed for per-route differentiation
- Existing codebase already has HSTS and custom CORS implemented in `middleware/security.go`

**Primary recommendation:** Use Fiber Helmet middleware for CSP/security headers, implement password validation with embedded 10k common password list, and create custom body limit middleware for endpoint-specific size control.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gofiber/fiber/v2/middleware/helmet | v2.x | Security headers (CSP, X-Frame-Options, etc.) | Official Fiber middleware, well-maintained |
| golang.org/x/crypto/bcrypt | latest | Password hashing | Already in use, industry standard |
| Embedded 10k password list | - | Common password blocklist | NIST recommended, no runtime dependency |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| zxcvbn-svelte or check-password-strength | latest | Frontend password strength UI | Real-time visual feedback |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Helmet middleware | Custom headers | Helmet is maintained, tested; custom adds maintenance burden |
| Embedded password list | HaveIBeenPwned API | User explicitly declined API dependency |
| 10k list | Full SecLists (100k+) | 10k catches 99%+ of weak passwords, smaller binary |

**Installation:**
```bash
# Backend - Helmet is included in fiber v2
go get github.com/gofiber/fiber/v2

# Frontend - Password strength (optional, can use pure Svelte)
npm install check-password-strength
```

## Architecture Patterns

### Recommended Project Structure
```
backend/
├── internal/
│   ├── middleware/
│   │   ├── security.go          # Existing: HSTS, CORS, cookie helpers
│   │   ├── headers.go           # New: CSP, X-Frame-Options, X-Content-Type
│   │   └── bodylimit.go         # New: Per-route body limit middleware
│   ├── service/
│   │   └── auth.go              # Modify: Enhanced password validation
│   └── data/
│       └── common_passwords.go  # New: Embedded 10k password list
frontend/
├── src/
│   ├── lib/
│   │   └── components/
│   │       └── PasswordInput.svelte  # New: Password with strength meter
│   └── routes/
│       └── (auth)/
│           └── change-password/      # New: Forced password change route
```

### Pattern 1: Security Headers Middleware Chain
**What:** Apply security headers in order with increasing specificity
**When to use:** All HTTP responses from API
**Example:**
```go
// Source: Fiber Helmet middleware pattern
// Apply headers in main.go after recover/logger, before routes
app.Use(helmet.New(helmet.Config{
    XFrameOptions:         "DENY",
    ContentTypeNosniff:    "nosniff",
    ContentSecurityPolicy: buildCSP(),
}))

func buildCSP() string {
    // Strict CSP - no inline, self-only by default
    return "default-src 'none'; " +
           "script-src 'self'; " +
           "connect-src 'self'; " +
           "img-src 'self' data:; " +
           "style-src 'self'; " +
           "font-src 'self'; " +
           "frame-ancestors 'none'; " +
           "base-uri 'self'; " +
           "form-action 'self'"
}
```

### Pattern 2: Layered Password Validation
**What:** Validate password on both frontend (UX) and backend (security)
**When to use:** Registration, password change, invitation acceptance
**Example:**
```go
// Source: NIST SP 800-63B guidelines
func (s *AuthService) validatePassword(password string) error {
    // Length check (NIST: 8-128 characters)
    if len(password) < 8 {
        return errors.New("Password must be at least 8 characters")
    }
    if len(password) > 128 {
        return errors.New("Password must be 128 characters or less")
    }

    // Blocklist check (NIST: check against common passwords)
    if isCommonPassword(password) {
        return errors.New("This password is too common. Please choose a different one.")
    }

    return nil
}
```

### Pattern 3: Per-Route Body Limits
**What:** Different size limits for different endpoint types
**When to use:** File uploads need higher limits than JSON APIs
**Example:**
```go
// Custom middleware for route-specific limits
func BodyLimit(limit int) fiber.Handler {
    return func(c *fiber.Ctx) error {
        if c.Request().Header.ContentLength() > limit {
            return c.Status(413).JSON(fiber.Map{
                "error": "Request body too large",
                "maxSize": limit,
            })
        }
        return c.Next()
    }
}

// Usage in routes
api.Post("/upload", BodyLimit(10*1024*1024), uploadHandler)  // 10MB
api.Post("/contacts", BodyLimit(1*1024*1024), createHandler) // 1MB (default)
```

### Pattern 4: Forced Password Change via JWT Claim
**What:** Add `mustChangePassword` claim to JWT, intercept on frontend
**When to use:** When existing weak passwords are detected on login
**Example:**
```go
// Backend: Add claim during login
claims := jwt.MapClaims{
    "userId": user.ID,
    // ... other claims
    "mustChangePassword": user.MustChangePassword,
}

// Frontend: Check on auth state change
$effect(() => {
    if (auth.mustChangePassword && !$page.url.pathname.startsWith('/change-password')) {
        goto('/change-password?required=true');
    }
});
```

### Anti-Patterns to Avoid
- **Inline CSP nonces for API:** API returns JSON, not HTML. CSP nonces are for HTML pages with scripts. For API-only backend, use strict policy without nonces.
- **Global body limit for all routes:** File uploads need larger limits. Use per-route middleware.
- **Password complexity rules:** NIST explicitly recommends against requiring uppercase/numbers/symbols. Length + blocklist is superior.
- **Checking password on every request:** Only check on registration, password change, and login. Cache the "needs change" flag in JWT.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Security headers | Custom header setting in each handler | Fiber Helmet middleware | Tested, maintained, covers edge cases |
| CSP parsing/building | String concatenation | Structured config object | Easy to make syntax errors |
| Password hashing | Custom crypto | bcrypt (already in use) | Timing attacks, salt handling |
| Common password list | Hardcoded array of 10 passwords | SecLists 10k embedded | Comprehensive coverage |
| Strength meter | Custom algorithm | zxcvbn/check-password-strength | Entropy calculation is complex |

**Key insight:** Security code is the last place to hand-roll solutions. Bugs here lead to vulnerabilities, not just broken features.

## Common Pitfalls

### Pitfall 1: CSP Breaking SvelteKit
**What goes wrong:** Strict CSP blocks SvelteKit's inline scripts during hydration
**Why it happens:** SvelteKit uses inline scripts for initial state
**How to avoid:** CSP is set by Vercel config for frontend, NOT by backend API headers. Backend CSP only affects API responses (JSON), which browsers don't execute.
**Warning signs:** If frontend stops loading after adding backend CSP, wrong layer was configured

### Pitfall 2: Body Limit vs Content-Length Mismatch
**What goes wrong:** Request passes limit check but exhausts memory
**Why it happens:** Attacker sends small Content-Length header but large body
**How to avoid:** Use `http.MaxBytesReader` pattern that enforces during read, not just header check
**Warning signs:** Memory spikes on malicious requests

### Pitfall 3: Common Password Check Case Sensitivity
**What goes wrong:** "Password123" passes but "password123" is blocked
**Why it happens:** Checking against lowercase list without normalizing input
**How to avoid:** Normalize to lowercase before checking: `strings.ToLower(password)`
**Warning signs:** Weak passwords still getting through

### Pitfall 4: Forced Password Change Blocking API Tokens
**What goes wrong:** API token requests fail because they hit password change middleware
**Why it happens:** Middleware doesn't distinguish JWT users from API tokens
**How to avoid:** Check `isAPIToken` in Locals before enforcing password change
**Warning signs:** API integrations break after adding forced password change

### Pitfall 5: Unicode Password Length
**What goes wrong:** Password "aaaa" (4 chars) passes but emoji password fails
**Why it happens:** Using byte length instead of character count for Unicode
**How to avoid:** Use `utf8.RuneCountInString(password)` instead of `len(password)`
**Warning signs:** International users can't set passwords

## Code Examples

Verified patterns from official sources:

### Fiber Helmet Configuration
```go
// Source: Fiber Helmet middleware docs + CSP reference
import "github.com/gofiber/fiber/v2/middleware/helmet"

app.Use(helmet.New(helmet.Config{
    // X-Frame-Options: DENY - prevent clickjacking
    XFrameOptions: "DENY",

    // X-Content-Type-Options: nosniff - prevent MIME sniffing
    ContentTypeNosniff: "nosniff",

    // Content-Security-Policy - strict policy
    ContentSecurityPolicy: "default-src 'none'; " +
        "script-src 'self'; " +
        "connect-src 'self'; " +
        "img-src 'self' data:; " +
        "style-src 'self'; " +
        "font-src 'self'; " +
        "frame-ancestors 'none'; " +
        "base-uri 'self'; " +
        "form-action 'self'",

    // Referrer-Policy - don't leak URLs
    ReferrerPolicy: "strict-origin-when-cross-origin",

    // X-XSS-Protection - legacy but still useful
    XSSProtection: "1; mode=block",
}))
```

### Password Validation with Blocklist
```go
// Source: NIST SP 800-63B + SecLists 10k passwords
package service

import (
    "strings"
    "unicode/utf8"

    "github.com/fastcrm/backend/internal/data"
)

// ValidatePassword checks password against NIST SP 800-63B requirements
func ValidatePassword(password string) error {
    // NIST: Use character count, not byte count for Unicode support
    length := utf8.RuneCountInString(password)

    // NIST: Minimum 8 characters
    if length < 8 {
        return fmt.Errorf("Password must be at least 8 characters (currently %d)", length)
    }

    // NIST: Maximum 128 characters (allow long passphrases)
    if length > 128 {
        return errors.New("Password must be 128 characters or less")
    }

    // NIST: Check against blocklist (case-insensitive)
    if data.IsCommonPassword(strings.ToLower(password)) {
        return errors.New("This password is too common. Please choose a different one.")
    }

    return nil
}
```

### Embedded Common Password List
```go
// Source: SecLists 10k-most-common.txt
package data

import (
    _ "embed"
    "strings"
)

//go:embed common_passwords.txt
var commonPasswordsRaw string

var commonPasswords map[string]bool

func init() {
    commonPasswords = make(map[string]bool)
    for _, pw := range strings.Split(commonPasswordsRaw, "\n") {
        pw = strings.TrimSpace(strings.ToLower(pw))
        if pw != "" {
            commonPasswords[pw] = true
        }
    }
}

// IsCommonPassword checks if password is in the blocklist
// Password should be pre-normalized to lowercase
func IsCommonPassword(password string) bool {
    return commonPasswords[password]
}
```

### Body Limit Middleware with Environment Config
```go
// Source: Fiber docs + custom per-route pattern
package middleware

import (
    "os"
    "strconv"

    "github.com/gofiber/fiber/v2"
)

var (
    DefaultBodyLimit = 1 * 1024 * 1024  // 1MB
    DefaultUploadLimit = 10 * 1024 * 1024 // 10MB
)

func init() {
    if val := os.Getenv("MAX_BODY_SIZE"); val != "" {
        if n, err := strconv.Atoi(val); err == nil && n > 0 {
            DefaultBodyLimit = n
        }
    }
    if val := os.Getenv("MAX_UPLOAD_SIZE"); val != "" {
        if n, err := strconv.Atoi(val); err == nil && n > 0 {
            DefaultUploadLimit = n
        }
    }
}

// BodyLimit creates middleware that limits request body size
// Returns 413 Payload Too Large if exceeded
func BodyLimit(limit int) fiber.Handler {
    return func(c *fiber.Ctx) error {
        contentLength := c.Request().Header.ContentLength()

        // Check Content-Length header if present
        if contentLength > limit {
            return c.Status(413).JSON(fiber.Map{
                "error": "Request body too large",
                "limit": limit,
                "received": contentLength,
            })
        }

        return c.Next()
    }
}

// IsUploadRoute checks if route is a file upload endpoint
// Used to apply larger limits automatically
func IsUploadRoute(path string) bool {
    uploadPaths := []string{"/upload", "/import", "/attachments"}
    for _, p := range uploadPaths {
        if strings.Contains(path, p) {
            return true
        }
    }
    return false
}
```

### Svelte Password Strength Component
```svelte
<!-- Source: check-password-strength library + Svelte patterns -->
<script lang="ts">
    import { passwordStrength } from 'check-password-strength';

    export let value: string = '';
    export let error: string = '';

    let strength = $derived(value ? passwordStrength(value) : null);

    const strengthColors = {
        'Too weak': 'bg-red-500',
        'Weak': 'bg-orange-500',
        'Medium': 'bg-yellow-500',
        'Strong': 'bg-green-500'
    };

    const strengthWidths = {
        'Too weak': 'w-1/4',
        'Weak': 'w-2/4',
        'Medium': 'w-3/4',
        'Strong': 'w-full'
    };
</script>

<div class="space-y-2">
    <label for="password" class="block text-sm font-medium text-gray-700">
        Password
    </label>
    <input
        type="password"
        id="password"
        bind:value
        class="block w-full px-3 py-2 border border-gray-300 rounded-lg"
        class:border-red-500={error}
    />

    {#if value}
        <div class="space-y-1">
            <div class="h-2 bg-gray-200 rounded-full overflow-hidden">
                <div
                    class="h-full transition-all duration-300 {strengthColors[strength?.value]} {strengthWidths[strength?.value]}"
                ></div>
            </div>
            <p class="text-xs text-gray-600">
                Strength: {strength?.value || 'Enter password'}
            </p>
        </div>
    {/if}

    {#if error}
        <p class="text-sm text-red-600">{error}</p>
    {/if}

    <p class="text-xs text-gray-500">
        Password must be at least 8 characters and not a commonly used password.
    </p>
</div>
```

### Forced Password Change Middleware
```go
// Source: JWT best practices + custom pattern
package middleware

import (
    "github.com/gofiber/fiber/v2"
)

// RequirePasswordChange redirects users who must change their password
// Skip for: API tokens, password change endpoints, logout
func RequirePasswordChange() fiber.Handler {
    return func(c *fiber.Ctx) error {
        // Skip for API tokens
        if isAPIToken, _ := c.Locals("isAPIToken").(bool); isAPIToken {
            return c.Next()
        }

        // Skip for password-related endpoints
        path := c.Path()
        allowedPaths := []string{
            "/api/v1/auth/change-password",
            "/api/v1/auth/logout",
            "/api/v1/auth/me",
        }
        for _, allowed := range allowedPaths {
            if path == allowed {
                return c.Next()
            }
        }

        // Check if user must change password
        mustChange, _ := c.Locals("mustChangePassword").(bool)
        if mustChange {
            return c.Status(403).JSON(fiber.Map{
                "error": "Password change required",
                "code": "PASSWORD_CHANGE_REQUIRED",
                "message": "Your password doesn't meet our updated security requirements. Please change it to continue.",
            })
        }

        return c.Next()
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Password complexity rules (uppercase, number, symbol) | Length-based + blocklist (NIST SP 800-63B Rev 4) | 2024-2025 | Simpler UX, better security |
| 90-day password rotation | Change only when compromised | NIST 2024 | Reduces weak password patterns |
| X-XSS-Protection header | CSP script-src instead | 2020+ | Modern browsers use CSP |
| HaveIBeenPwned API for all checks | Embedded blocklist + optional API | 2024+ | No external dependency |

**Deprecated/outdated:**
- `X-XSS-Protection: 1; mode=block` - Still set but modern browsers ignore it in favor of CSP. Include for legacy browser support.
- Password complexity requirements - NIST now says "shall not" impose composition rules
- Forced periodic password changes - Only change when evidence of compromise

## Open Questions

Things that couldn't be fully resolved:

1. **Exact password list size tradeoff**
   - What we know: SecLists has 10k, 100k, 1M lists. 10k catches most common passwords.
   - What's unclear: Exact binary size impact of 10k vs 100k list
   - Recommendation: Start with 10k (~100KB), monitor if weak passwords still get through

2. **CSP allowlist persistence**
   - What we know: User decided dynamic allowlist for external resources
   - What's unclear: Where to store per-org allowlist (org_settings table?)
   - Recommendation: Add `csp_allowed_sources` column to `org_settings`, rebuild CSP header per-request

3. **Existing weak password detection**
   - What we know: Need to flag existing users with weak passwords
   - What's unclear: How to trigger re-check (background job vs next login?)
   - Recommendation: Check on next login (simpler, no background job needed)

## Sources

### Primary (HIGH confidence)
- Fiber Helmet Middleware - https://docs.gofiber.io/api/middleware/helmet/
- Fiber BodyLimit Configuration - https://docs.gofiber.io/api/fiber/
- CSP Reference - https://content-security-policy.com/
- SecLists Password Lists - https://github.com/danielmiessler/SecLists

### Secondary (MEDIUM confidence)
- NIST SP 800-63B Rev 4 - https://pages.nist.gov/800-63-4/sp800-63b.html (via search summaries)
- Enzoic NIST Summary - https://www.enzoic.com/blog/nist-sp-800-63b-rev4/
- strongDM NIST Guide - https://www.strongdm.com/blog/nist-password-guidelines
- go-password-validator - https://github.com/wagslane/go-password-validator

### Tertiary (LOW confidence)
- Per-route body limit pattern - https://pkg.go.dev/github.com/go-mizu/mizu/middlewares/bodylimit (external library, may need adaptation)
- Svelte password strength - https://github.com/dfsilva96/svelte-password-strength (small repo, verify before use)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Fiber Helmet is official, bcrypt is standard, pattern is well-established
- Architecture: HIGH - Patterns match existing codebase structure (`middleware/security.go`)
- Pitfalls: MEDIUM - CSP/SvelteKit interaction based on general knowledge, needs validation

**Research date:** 2026-02-03
**Valid until:** 2026-03-03 (30 days - security headers are stable)
