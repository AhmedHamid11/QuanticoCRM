# FastCRM Security Assessment Report

**Report Date:** January 25, 2026
**Assessment Type:** Comprehensive Security Audit
**Conducted By:** CISO Security Review
**Application:** FastCRM (Quantico CRM)
**Stack:** Go 1.22+/Fiber Backend, SvelteKit Frontend, Turso SQLite Edge Database

---

## Executive Summary

This security assessment identified **25 security issues** across the FastCRM application:

| Severity | Count | Immediate Action Required |
|----------|-------|---------------------------|
| **Critical** | 5 | Yes - Fix before production |
| **High** | 5 | Yes - Fix within 1 sprint |
| **Medium** | 6 | Plan for remediation |
| **Low** | 5 | Address opportunistically |
| **Informational** | 4 | Best practices recommendations |

**Overall Risk Rating: HIGH**

The application contains several critical vulnerabilities that must be addressed before any production deployment. The most severe issues relate to CORS misconfiguration, weak JWT secret handling, potential SQL injection, and missing rate limiting.

---

## Critical Vulnerabilities

### CRIT-001: Overly Permissive CORS Configuration

**File:** `backend/cmd/api/main.go:110-114`
**CVSS Score:** 9.1 (Critical)

```go
app.Use(cors.New(cors.Config{
    AllowOrigins: "*", // TODO: Restrict in production
    AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
    AllowHeaders: "Origin,Content-Type,Accept,Authorization",
}))
```

**Risk:**
- Any website can make authenticated cross-origin requests to your API
- Enables Cross-Site Request Forgery (CSRF) attacks
- Attackers can steal data or perform actions on behalf of logged-in users
- Complete compromise of user sessions possible

**Remediation:**
```go
allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
if allowedOrigins == "" {
    log.Fatal("ALLOWED_ORIGINS must be set in production")
}
app.Use(cors.New(cors.Config{
    AllowOrigins:     allowedOrigins,
    AllowCredentials: true,
    AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
    AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token",
}))
```

---

### CRIT-002: Weak Default JWT Secret in Development Mode

**File:** `backend/cmd/api/main.go:38-44`
**CVSS Score:** 9.8 (Critical)

```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    jwtSecret = "dev-secret-change-in-production"
    log.Println("WARNING: Using default JWT secret...")
}
```

**Risk:**
- Predictable JWT secret allows token forgery
- Anyone with source code access can create valid admin tokens
- If accidentally deployed without environment variable, complete auth bypass
- Attackers can impersonate any user including platform admins

**Remediation:**
```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    if os.Getenv("ENVIRONMENT") == "production" {
        log.Fatal("CRITICAL: JWT_SECRET must be set in production")
    }
    jwtSecret = generateRandomSecret(64) // Generate unique dev secret
    log.Printf("DEV MODE: Using random JWT secret (session won't persist across restarts)")
}
if len(jwtSecret) < 32 {
    log.Fatal("JWT_SECRET must be at least 32 characters")
}
```

---

### CRIT-003: SQL Injection via String Interpolation

**File:** `backend/internal/handler/data_explorer.go:326`
**CVSS Score:** 8.6 (High-Critical)

```go
orgFilter := fmt.Sprintf("org_id = '%s'", orgID)
// Later concatenated into SQL query
modifiedQuery := query[:insertPos] + orgFilter + " AND " + query[insertPos:]
```

**Risk:**
- While orgID comes from JWT claims, string interpolation is inherently unsafe
- Future code changes could introduce injection vectors
- Does not follow secure coding practices
- Database schema and data could be exposed or modified

**Remediation:**
- Use parameterized queries exclusively
- Implement a proper SQL parser if dynamic queries are required
- Consider using an ORM with built-in injection protection

---

### CRIT-004: Sensitive Data in Error Responses

**Files:** Multiple handlers
**CVSS Score:** 5.3 (Medium) but critical for information disclosure

```go
// Example from data_explorer.go:87-88
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
    "error": err.Error(),  // Raw database errors exposed
})
```

**Affected Files:**
- `handler/data_explorer.go` (lines 87, 96, 116, 134, 188, 197, 226, 244)
- `handler/contact.go` (lines 40, 54, 105)
- `handler/generic_entity.go` (multiple locations)
- Global error handler in `main.go:100-104`

**Risk:**
- Database schema exposure through error messages
- SQL syntax errors reveal table/column names
- Stack traces could leak sensitive paths and configurations
- Attackers learn system internals for targeted attacks

**Remediation:**
```go
// Create error mapper
func mapError(err error) (int, string) {
    switch {
    case errors.Is(err, sql.ErrNoRows):
        return 404, "Resource not found"
    case errors.Is(err, ErrValidation):
        return 400, "Invalid input"
    default:
        log.Errorf("Internal error: %v", err) // Log full error
        return 500, "An internal error occurred"
    }
}
```

---

### CRIT-005: Tokens Stored in localStorage (XSS Vulnerability)

**File:** `frontend/src/lib/stores/auth.svelte.ts:79,88`
**CVSS Score:** 8.1 (High)

```typescript
localStorage.setItem(STORAGE_KEY, JSON.stringify(toStore));
// Stores: accessToken, refreshToken, user data
```

**Risk:**
- Any XSS vulnerability allows complete token theft
- JavaScript has unrestricted access to localStorage
- Tokens persist across browser sessions increasing exposure window
- No HttpOnly flag protection

**Remediation:**
- Store refresh tokens in HttpOnly, Secure, SameSite=Strict cookies
- Keep access tokens in memory only with short expiration
- Implement token rotation on each refresh
- Add Content Security Policy headers

---

## High Severity Vulnerabilities

### HIGH-001: Missing Rate Limiting on Authentication Endpoints

**File:** `backend/cmd/api/main.go:127-131`
**CVSS Score:** 7.5

**Unprotected Endpoints:**
- `POST /auth/register` - No rate limiting
- `POST /auth/login` - No rate limiting
- `POST /auth/accept-invite` - No rate limiting
- `POST /auth/refresh` - No rate limiting

**Risk:**
- Brute force attacks on passwords
- Email enumeration via registration errors
- Account lockout denial of service
- Credential stuffing attacks

**Remediation:**
```go
import "github.com/gofiber/fiber/v2/middleware/limiter"

// Per-IP rate limiting
loginLimiter := limiter.New(limiter.Config{
    Max:        5,
    Expiration: 1 * time.Minute,
    KeyGenerator: func(c *fiber.Ctx) string {
        return c.IP()
    },
    LimitReached: func(c *fiber.Ctx) error {
        return c.Status(429).JSON(fiber.Map{
            "error": "Too many attempts. Please try again later.",
        })
    },
})

auth.Post("/login", loginLimiter, authHandler.Login)
```

---

### HIGH-002: Weak Password Validation Policy

**File:** `backend/internal/service/auth.go:695-700`
**CVSS Score:** 6.5

```go
func (s *AuthService) validatePassword(password string) error {
    if len(password) < 8 {
        return ErrPasswordTooWeak
    }
    return nil
}
```

**Risk:**
- Accepts weak passwords like "12345678"
- No complexity requirements
- Vulnerable to dictionary attacks
- No common password blocklist

**Remediation:**
Follow NIST 800-63B guidelines:
- Minimum 8 characters (allow up to 64+)
- Check against common password lists (e.g., Have I Been Pwned API)
- No arbitrary complexity rules (they reduce security)
- Implement rate limiting instead of lockouts

---

### HIGH-003: Refresh Token Without Rotation

**File:** `backend/internal/service/auth.go:172-199`
**CVSS Score:** 6.8

```go
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (*entity.AuthResponse, error) {
    // Deletes old session but no token family tracking
    _ = s.repo.DeleteSession(ctx, session.ID)
    return s.createAuthResponse(ctx, user, session.OrgID, session.IsImpersonation, session.ImpersonatedBy)
}
```

**Risk:**
- Stolen refresh tokens remain valid until expiration
- No detection of token reuse attacks
- Session hijacking difficult to detect
- Token theft provides extended access window

**Remediation:**
- Implement refresh token rotation (new token on each refresh)
- Add token family tracking
- Invalidate entire family if reuse detected
- Reduce refresh token lifetime

---

### HIGH-004: Insufficient Impersonation Controls

**File:** `backend/internal/service/auth.go:238-296`
**CVSS Score:** 7.2

**Issues:**
- No time limit on impersonation sessions
- Limited audit logging
- No automatic session termination
- No reason field required for compliance

**Risk:**
- Admin could maintain unauthorized access indefinitely
- Compliance violations (SOC2, GDPR)
- Difficulty detecting unauthorized impersonation
- No audit trail for security investigations

**Remediation:**
```go
type ImpersonateInput struct {
    TargetUserID string `json:"targetUserId"`
    OrgID        string `json:"orgId"`
    Reason       string `json:"reason" validate:"required,min=10"`
    Duration     int    `json:"duration" validate:"max=60"` // Max 60 minutes
}

// Implement:
// - Automatic session expiration after duration
// - Comprehensive audit logging
// - Notification to impersonated user
// - Reason requirement for compliance
```

---

### HIGH-005: API Token Scope Enforcement Gaps

**File:** `backend/internal/middleware/auth.go:167-205`
**CVSS Score:** 6.5

```go
func (m *AuthMiddleware) RequireScope(scope string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        isAPIToken, ok := c.Locals("isAPIToken").(bool)
        if !ok || !isAPIToken {
            return c.Next()  // JWT tokens bypass scope checking!
        }
        // ...
    }
}
```

**Risk:**
- JWT tokens have implicit full access
- API tokens with "read" scope may access write endpoints if scope not checked
- No default-deny for unchecked routes
- Inconsistent authorization model

**Remediation:**
- Implement explicit scope requirements on all endpoints
- Default to deny if scopes not checked
- Create scope constants and validate at compile time
- Add integration tests for authorization

---

## Medium Severity Vulnerabilities

### MED-001: Multi-Tenant Data Isolation Inconsistencies

**Files:** Multiple handlers and repositories
**CVSS Score:** 6.1

**Issue:** While org_id filtering is implemented, it's not consistently applied across all data access paths.

**Locations with concerns:**
- `handler/generic_entity.go:144-149` - Table name not quoted (potential injection)
- `repo/contact.go` - Manual org_id filtering
- `handler/data_explorer.go` - Dynamic query construction

**Risk:**
- Cross-tenant data access
- Data leakage between organizations
- One missed filter = complete tenant isolation failure

**Remediation:**
- Implement row-level security at database layer
- Create centralized data access layer with automatic org_id filtering
- Add integration tests specifically for cross-org access attempts

---

### MED-002: Missing CSRF Protection

**Affected:** All state-changing endpoints
**CVSS Score:** 5.8

**Issue:** No CSRF token implementation despite using bearer tokens (which can be sent from any origin with CORS: *).

**Remediation:**
- Implement double-submit cookie CSRF tokens
- Add SameSite=Strict to all cookies
- Validate Origin/Referer headers
- Fix CORS configuration (CRIT-001)

---

### MED-003: No Session Timeout for Idle Users

**CVSS Score:** 4.3

**Issue:** Access tokens have 24-hour expiration regardless of activity.

**Remediation:**
- Implement sliding session expiration
- Add explicit idle timeout (e.g., 30 minutes)
- Require re-authentication for sensitive operations
- Add "remember me" option for extended sessions

---

### MED-004: Invitation Token Expiration Not Enforced at Acceptance

**File:** `backend/internal/service/auth.go:364-419`
**CVSS Score:** 4.7

```go
invitation, err := s.repo.GetInvitationByToken(ctx, input.Token)
// No explicit check: if invitation.ExpiresAt.Before(time.Now())
```

**Risk:**
- Race condition between DB query and acceptance
- Expired invitations might be accepted in edge cases

---

### MED-005: No Audit Logging Infrastructure

**Affected:** All administrative operations
**CVSS Score:** 5.5

**Missing Audit Events:**
- User creation/deletion/role changes
- API token creation/revocation
- Login attempts (success/failure)
- Impersonation sessions
- Data exports
- Configuration changes

**Compliance Impact:**
- SOC2 requirement violation
- GDPR Article 30 (records of processing)
- Difficult security incident investigation

---

### MED-006: Email Verification Not Enforced

**File:** `backend/internal/entity/user.go`
**CVSS Score:** 4.0

**Issue:** EmailVerified field exists but is never checked or enforced.

**Risk:**
- Accounts created with typo'd emails
- No proof of email ownership
- Password reset to wrong recipients

---

## Low Severity Vulnerabilities

### LOW-001: Bcrypt Cost Factor Hardcoded

**File:** `backend/internal/service/auth.go:54`

```go
BcryptCost: 12,  // Hardcoded, should increase over time
```

**Remediation:** Make configurable via environment variable.

---

### LOW-002: Missing HTTPS Enforcement

**File:** `backend/cmd/api/main.go`

**Missing:**
- HSTS header (Strict-Transport-Security)
- HTTP to HTTPS redirect
- Secure cookie flag enforcement

---

### LOW-003: No Request Size Limits

**Issue:** No explicit limits on request body size.

**Risk:**
- Denial of service via large payloads
- Memory exhaustion attacks

---

### LOW-004: Session Cleanup Not Scheduled

**File:** `backend/internal/repo/auth.go:592-596`

```go
func (r *AuthRepo) CleanExpiredSessions(ctx context.Context) error {
    // Exists but never called
}
```

**Issue:** Expired sessions accumulate in database.

---

### LOW-005: Token Hash Without Salt

**File:** `backend/internal/service/auth.go`

**Issue:** Refresh tokens hashed with SHA256 without salt.

**Risk:**
- Rainbow table attacks on token hashes
- Reduced security if database is compromised

---

## Informational Findings

### INFO-001: Missing Security Headers

**Missing Headers:**
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Permissions-Policy`
- `Content-Security-Policy`

---

### INFO-002: No Content Security Policy (CSP)

**Risk:** No protection against inline script injection or XSS.

**Recommended CSP:**
```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self' https://api.yourdomain.com; frame-ancestors 'none';
```

---

### INFO-003: Development Artifacts in Production Code

**Locations:**
- `main.go:111` - `// TODO: Restrict in production`
- Multiple `log.Println` statements that should use structured logging

---

### INFO-004: Flow Expression Evaluation Risk

**File:** `backend/internal/flow/expression.go`

**Issue:** Dynamic expression evaluation in workflow engine could be exploited if user input reaches expression parser.

**Recommendation:**
- Whitelist allowed functions
- Implement expression complexity limits
- Sandbox expression evaluation

---

## Compliance Gaps

### GDPR Compliance

| Requirement | Status | Issue |
|-------------|--------|-------|
| Article 17 (Right to Erasure) | Partial | No data deletion workflow |
| Article 25 (Privacy by Design) | Partial | Missing encryption at rest |
| Article 30 (Records of Processing) | Missing | No audit logging |
| Article 32 (Security of Processing) | Partial | Multiple vulnerabilities |

### SOC 2 Compliance

| Control | Status | Issue |
|---------|--------|-------|
| CC6.1 (Logical Access) | Partial | Weak password policy |
| CC6.6 (External Access) | Failing | CORS misconfiguration |
| CC7.1 (Incident Detection) | Missing | No security monitoring |
| CC7.2 (Incident Response) | Missing | No incident playbooks |

---

## Remediation Priority Matrix

### Immediate (Before Any Production Use)

1. **CRIT-001:** Fix CORS configuration
2. **CRIT-002:** Enforce JWT secret in production
3. **HIGH-001:** Implement rate limiting on auth endpoints
4. **CRIT-004:** Sanitize error responses

### Sprint 1 (Next 2 Weeks)

1. **CRIT-003:** Fix SQL injection patterns
2. **CRIT-005:** Move tokens to HttpOnly cookies
3. **HIGH-002:** Strengthen password validation
4. **HIGH-003:** Implement token rotation

### Sprint 2 (Next Month)

1. **MED-001:** Centralize tenant isolation
2. **MED-002:** Implement CSRF protection
3. **MED-005:** Add audit logging
4. **HIGH-004:** Add impersonation controls

### Backlog

1. All LOW severity items
2. Security headers (INFO-001, INFO-002)
3. Compliance gap remediation

---

## Security Testing Recommendations

### Immediate Actions

1. **Penetration Test:** Engage external security firm for penetration testing before production launch
2. **Dependency Audit:** Run `go mod tidy && go list -m all | nancy` and `npm audit`
3. **SAST Scan:** Implement static analysis (e.g., gosec, semgrep)

### Ongoing Practices

1. **Security Reviews:** Require security review for auth-related PRs
2. **Dependency Updates:** Weekly automated dependency updates
3. **Bug Bounty:** Consider bug bounty program post-launch
4. **Monitoring:** Implement security event monitoring (failed logins, rate limit hits, etc.)

---

## Email Security (DKIM/SPF/DMARC)

### Current State: No Email Sending Implemented

Your application currently has **no email sending capability**. The invitation system (`InviteUser` in `auth.go:319-361`) generates tokens but does not send emails - tokens must be shared manually.

### When You Implement Email (Required for Production)

You'll need email for:
- User invitations
- Password reset flows
- Notification emails
- Activity alerts

### Required Email Authentication Records

When you add email sending, configure these DNS records for your sending domain:

#### 1. SPF (Sender Policy Framework)
```dns
# Example for SendGrid
v=spf1 include:sendgrid.net ~all

# Example for AWS SES
v=spf1 include:amazonses.com ~all

# Example for multiple providers
v=spf1 include:sendgrid.net include:amazonses.com ~all
```

**What it does:** Specifies which mail servers can send email on behalf of your domain.

#### 2. DKIM (DomainKeys Identified Mail)
```dns
# Add CNAME records provided by your email service
# Example for SendGrid:
s1._domainkey.yourdomain.com -> s1.domainkey.u12345.wl.sendgrid.net
s2._domainkey.yourdomain.com -> s2.domainkey.u12345.wl.sendgrid.net
```

**What it does:** Cryptographically signs emails to prove they weren't modified in transit.

#### 3. DMARC (Domain-based Message Authentication)
```dns
_dmarc.yourdomain.com TXT "v=DMARC1; p=quarantine; rua=mailto:dmarc@yourdomain.com; pct=100"
```

**Recommended progression:**
1. Start with `p=none` (monitoring only)
2. Move to `p=quarantine` after reviewing reports
3. Eventually `p=reject` for maximum protection

**What it does:** Tells receiving servers what to do when SPF/DKIM fail.

### Email Security Checklist

| Item | Priority | Status |
|------|----------|--------|
| Choose transactional email provider (SendGrid, Postmark, SES) | High | Not Started |
| Configure SPF record | High | Not Started |
| Configure DKIM signing | High | Not Started |
| Configure DMARC policy | Medium | Not Started |
| Set up DMARC reporting | Medium | Not Started |
| Implement email rate limiting | Medium | Not Started |
| Add unsubscribe headers (CAN-SPAM) | Medium | Not Started |
| Use dedicated subdomain for transactional email | Low | Not Started |

### Recommended Email Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Email Sending Flow                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   FastCRM Backend                                            │
│        │                                                     │
│        ▼                                                     │
│   Email Service (interface)                                  │
│        │                                                     │
│        ├──► SendGrid (production)                           │
│        ├──► Mailhog (development - catches all emails)      │
│        └──► Console Logger (testing)                        │
│                                                              │
│   Sending Domain: mail.fastcrm.com (subdomain)              │
│        │                                                     │
│        ├── SPF: v=spf1 include:sendgrid.net ~all            │
│        ├── DKIM: Configured via SendGrid                    │
│        └── DMARC: p=quarantine; rua=mailto:dmarc@...        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Email Provider Security Comparison

| Provider | DKIM | SPF | Deliverability | Best For |
|----------|------|-----|----------------|----------|
| **Postmark** | Auto | Auto | Excellent | Transactional only |
| **SendGrid** | Manual | Manual | Good | High volume |
| **AWS SES** | Manual | Manual | Good | AWS ecosystem |
| **Resend** | Auto | Auto | Good | Developer-friendly |

### Implementation Code Pattern

```go
// email/service.go
type EmailService interface {
    SendInvitation(ctx context.Context, to, inviterName, orgName, token string) error
    SendPasswordReset(ctx context.Context, to, token string) error
    SendNotification(ctx context.Context, to, subject, body string) error
}

// email/sendgrid.go
type SendGridService struct {
    apiKey     string
    fromEmail  string
    fromName   string
}

func (s *SendGridService) SendInvitation(ctx context.Context, to, inviterName, orgName, token string) error {
    // Use pre-built template for consistent branding
    // Include unsubscribe link
    // Rate limit per org
}
```

### Additional Email Security Considerations

1. **Rate Limiting**
   - Limit invitations per org per hour
   - Prevent email bombing attacks
   - Log all email sends for audit

2. **Token Security**
   - Invitation tokens already use secure random generation ✓
   - Add expiration display in email
   - Single-use tokens (already implemented) ✓

3. **Privacy**
   - Don't leak user existence via "already registered" messages
   - Use generic "check your email" responses
   - Implement email confirmation for new registrations

4. **Compliance**
   - CAN-SPAM: Include physical address, unsubscribe link
   - GDPR: Link to privacy policy, consent tracking
   - CCPA: Include opt-out mechanisms

---

## Appendix A: Security Headers Configuration

```go
app.Use(func(c *fiber.Ctx) error {
    c.Set("X-Content-Type-Options", "nosniff")
    c.Set("X-Frame-Options", "DENY")
    c.Set("X-XSS-Protection", "1; mode=block")
    c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
    c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
    c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
    return c.Next()
})
```

---

## Appendix B: Secure Cookie Configuration

```go
cookie := &fiber.Cookie{
    Name:     "refresh_token",
    Value:    token,
    HTTPOnly: true,
    Secure:   true,
    SameSite: "Strict",
    Path:     "/api/v1/auth",
    MaxAge:   7 * 24 * 60 * 60, // 7 days
}
```

---

## Document Control

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-01-25 | CISO Review | Initial assessment |

**Classification:** INTERNAL - SECURITY SENSITIVE

**Distribution:** Engineering Leadership, Security Team, DevOps

---

*This assessment should be reviewed quarterly or after any significant architectural changes.*
