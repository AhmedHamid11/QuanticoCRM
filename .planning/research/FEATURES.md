# Feature Landscape: CRM Security Hardening

**Domain:** Multi-Tenant CRM Security & Compliance
**Researched:** 2026-02-03
**Overall Confidence:** HIGH (verified against OWASP, NIST, GDPR, SOC 2 official sources)

---

## Executive Summary

This document categorizes security controls required for FastCRM to handle customer data responsibly and achieve SOC 2/GDPR readiness. Features are organized by compliance requirement and implementation priority.

**Current State (from SECURITY_ASSESSMENT.md):**
- 5 Critical vulnerabilities (CORS, JWT secret, SQL injection, error disclosure, XSS via localStorage)
- 5 High severity issues (rate limiting, passwords, token rotation, impersonation, scope enforcement)
- Existing: JWT auth, bcrypt passwords, per-tenant databases, role-based access

**Target State:**
- SOC 2 Type II certifiable
- GDPR Article 32 compliant
- OWASP Top 10 2025 addressed

---

## Table Stakes (Must Have for Production CRM)

Features users and enterprise customers **expect**. Missing = product is not production-ready.

| Feature | Why Expected | Complexity | Priority | Compliance Mapping |
|---------|--------------|------------|----------|-------------------|
| **Secure CORS configuration** | Prevents cross-site attacks; current wildcard (*) is critical vulnerability | Low | P0 - Immediate | SOC 2 CC6.6, OWASP A01 |
| **Rate limiting on auth endpoints** | Prevents brute force attacks; industry standard | Low | P0 - Immediate | SOC 2 CC6.1, OWASP A07 |
| **Sanitized error responses** | Prevents information disclosure; raw DB errors leak schema | Low | P0 - Immediate | SOC 2 CC7.1, OWASP A01 |
| **Enforced JWT secret in production** | Prevents token forgery; weak default is critical risk | Low | P0 - Immediate | SOC 2 CC6.1 |
| **HTTPS enforcement (HSTS)** | All data in transit must be encrypted | Low | P0 - Immediate | SOC 2 CC6.7, GDPR Art. 32 |
| **Security headers** (X-Frame-Options, X-Content-Type-Options, CSP) | Defense in depth against XSS, clickjacking | Low | P1 - Sprint 1 | SOC 2 CC6.6, OWASP A03 |
| **HttpOnly cookie tokens** | Prevents XSS token theft; localStorage is vulnerable | Medium | P1 - Sprint 1 | SOC 2 CC6.1, OWASP A07 |
| **Refresh token rotation** | Detects token theft; current implementation lacks rotation | Medium | P1 - Sprint 1 | SOC 2 CC6.1 |
| **Strong password policy** (NIST 800-63B) | Prevents weak passwords; current 8-char minimum insufficient | Medium | P1 - Sprint 1 | SOC 2 CC6.1, NIST 800-63B |
| **Request size limits** | Prevents DoS via large payloads | Low | P1 - Sprint 1 | SOC 2 CC6.6 |
| **Input validation** (all endpoints) | Prevents injection attacks | Medium | P1 - Sprint 1 | OWASP A03 (Injection) |
| **Session timeout** (idle + absolute) | Limits exposure window of stolen sessions | Low | P2 - Sprint 2 | SOC 2 CC6.1 |
| **CSRF protection** | Prevents cross-site request forgery | Medium | P2 - Sprint 2 | SOC 2 CC6.6, OWASP A01 |
| **Multi-tenant isolation verification** | Ensures tenant data never leaks; add integration tests | Medium | P2 - Sprint 2 | SOC 2 CC6.1, GDPR Art. 32 |

### Implementation Notes

**CORS (P0):**
```go
allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
if allowedOrigins == "" && os.Getenv("ENVIRONMENT") == "production" {
    log.Fatal("ALLOWED_ORIGINS must be set in production")
}
```

**Rate Limiting (P0):**
- Use `github.com/gofiber/fiber/v2/middleware/limiter`
- Auth endpoints: 5 requests/minute per IP
- API endpoints: 100 requests/minute per org
- Consider Redis for distributed rate limiting

**Password Policy (NIST 800-63B):**
- Minimum 8 characters (recommend 15+ for admin accounts)
- Maximum 64+ characters allowed
- NO complexity requirements (NIST explicitly removed these)
- Check against [Have I Been Pwned API](https://haveibeenpwned.com/API/v3#PwnedPasswords) for breached passwords
- No periodic password expiration (only reset on breach evidence)

**Token Storage:**
- Refresh token: HttpOnly, Secure, SameSite=Strict cookie
- Access token: Memory only (not localStorage), short expiration (15-30 min)
- Implement refresh token rotation with family tracking

---

## SOC 2 Requirements (Audit Controls)

SOC 2 Type II requires **demonstrated operational effectiveness** over time, not just point-in-time controls.

| Feature | SOC 2 Criterion | Complexity | Priority | Notes |
|---------|-----------------|------------|----------|-------|
| **Comprehensive audit logging** | CC7.1, CC7.2 | High | P2 - Sprint 2 | Log all admin actions, auth events, data access |
| **Audit log retention** (1+ year) | CC7.1 | Medium | P2 - Sprint 2 | Immutable storage, tamper-evident |
| **Access review capability** | CC6.2, CC6.3 | Medium | P3 - Sprint 3 | Report of who has access to what |
| **User provisioning/deprovisioning logs** | CC6.1 | Medium | P2 - Sprint 2 | Part of audit logging |
| **Change management logging** | CC8.1 | Medium | P3 - Sprint 3 | Configuration changes tracked |
| **Incident response procedures** | CC7.3, CC7.4 | Low (docs) | P3 - Sprint 3 | Documented runbooks |
| **Backup verification** | A1.2, A1.3 | Medium | P3 - Sprint 3 | Test restores, document procedures |
| **Vulnerability scanning** | CC4.1 | Low (tooling) | P2 - Sprint 2 | Automated SAST (gosec), DAST |
| **Dependency vulnerability monitoring** | CC4.1 | Low (tooling) | P2 - Sprint 2 | Dependabot, nancy for Go |
| **Security awareness training** | CC1.4 | Low (docs) | P4 - Backlog | For dev team |

### Audit Logging Requirements

**What to log (minimum for SOC 2):**
1. **Authentication events:** Login success/failure, logout, token refresh, password changes
2. **Authorization failures:** Forbidden access attempts
3. **Administrative actions:** User CRUD, role changes, org settings changes
4. **Data access:** Sensitive record views (configurable per entity)
5. **API token lifecycle:** Creation, revocation, usage patterns
6. **Impersonation:** Start/end, actions taken during impersonation

**Log schema:**
```go
type AuditLog struct {
    ID          string    `json:"id"`
    Timestamp   time.Time `json:"timestamp"`
    OrgID       string    `json:"orgId"`
    ActorID     string    `json:"actorId"`     // User or API token ID
    ActorType   string    `json:"actorType"`   // "user", "api_token", "system"
    Action      string    `json:"action"`      // "user.login", "contact.create", etc.
    ResourceType string   `json:"resourceType"` // "user", "contact", etc.
    ResourceID  string    `json:"resourceId"`
    IPAddress   string    `json:"ipAddress"`
    UserAgent   string    `json:"userAgent"`
    Status      string    `json:"status"`      // "success", "failure"
    Details     JSON      `json:"details"`     // Action-specific data
    IsImpersonation bool  `json:"isImpersonation"`
    ImpersonatedBy string `json:"impersonatedBy,omitempty"`
}
```

**Retention:** SOC 2 typically requires 1 year minimum. Store in append-only format.

---

## GDPR Requirements (Data Protection)

GDPR applies if you have EU customers or process EU resident data.

| Feature | GDPR Article | Complexity | Priority | Notes |
|---------|--------------|------------|----------|-------|
| **Encryption at rest** | Art. 32 | High | P2 - Sprint 2 | Turso supports SQLCipher; evaluate per-tenant keys |
| **Right to erasure implementation** | Art. 17 | High | P3 - Sprint 3 | Data deletion workflow across all systems |
| **Data export (portability)** | Art. 20 | Medium | P3 - Sprint 3 | Export user data in machine-readable format |
| **Consent tracking** | Art. 7 | Medium | P3 - Sprint 3 | Record when/how consent was given |
| **Privacy policy integration** | Art. 13, 14 | Low | P4 - Backlog | Link in app, record acceptance |
| **Data retention policies** | Art. 5(1)(e) | Medium | P3 - Sprint 3 | Auto-delete after defined periods |
| **Sub-processor documentation** | Art. 28 | Low (docs) | P4 - Backlog | Document Turso, Railway, Vercel |
| **Breach notification system** | Art. 33, 34 | Medium | P3 - Sprint 3 | 72-hour notification to DPA; affected user notification |
| **Records of processing activities (ROPA)** | Art. 30 | Low (docs) | P3 - Sprint 3 | Documentation of data flows |
| **Data minimization audit** | Art. 5(1)(c) | Low | P4 - Backlog | Review what data is collected vs. needed |

### Right to Erasure Technical Implementation

**Complexity: HIGH** - This is harder than it looks.

**Requirements:**
1. Delete from primary database (contacts, accounts, tasks, etc.)
2. Delete from audit logs (except legal retention requirements)
3. Delete from backups (or document retention period)
4. Delete from all sub-processors
5. Document the deletion request and completion

**Implementation approach:**
```go
type ErasureRequest struct {
    ID            string     `json:"id"`
    OrgID         string     `json:"orgId"`
    RequestedBy   string     `json:"requestedBy"`   // User making request
    SubjectEmail  string     `json:"subjectEmail"`  // Person to be forgotten
    Status        string     `json:"status"`        // pending, in_progress, completed, failed
    RequestedAt   time.Time  `json:"requestedAt"`
    CompletedAt   *time.Time `json:"completedAt,omitempty"`
    DeletionLog   []string   `json:"deletionLog"`   // What was deleted
}

// 1. Find all records where email matches
// 2. Soft delete immediately (remove from searches)
// 3. Hard delete after verification window (7 days)
// 4. Log completion for compliance
```

**Exemptions to document:**
- Legal hold requirements
- Tax/financial record retention
- Ongoing contract obligations

### Encryption at Rest Strategy

**Options for Turso/SQLite:**

1. **SQLCipher** (Turso-supported): 256-bit AES encryption
   - Transparent encryption
   - Per-database encryption key
   - Turso has [announced native encryption support](https://turso.tech/blog/fully-open-source-encryption-for-sqlite-b3858225)

2. **Column-level encryption**: For highly sensitive fields only
   - Higher complexity
   - Performance impact on queries
   - Consider for PII fields if full-database encryption not available

**Recommendation:** Use Turso's built-in encryption when available. For now, document this as a gap and implement column-level encryption for most sensitive fields (social security numbers, payment data if any).

---

## Anti-Features (Security Theater to Avoid)

Features that **seem** secure but add complexity without real protection, or that are actively harmful.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Mandatory password complexity rules** (uppercase + number + symbol) | NIST explicitly removed in 800-63B; leads to weaker passwords like "Password1!" | Enforce length (15+ chars), check breached password database |
| **Periodic password expiration** (90-day rotation) | NIST removed; causes predictable patterns (Password1, Password2...) | Only require change on breach evidence |
| **Security questions** (mother's maiden name) | Easily researched via social engineering | Use MFA instead |
| **CAPTCHA on every form** | Frustrates users; modern bots bypass | Rate limiting + invisible fraud detection |
| **IP-based blocking** (as primary defense) | IPs are easily rotated; blocks legitimate users | Use as one signal among many; don't rely solely |
| **Email masking in UI** (j***@example.com) | False sense of security; full email often in page source | Either show or don't show; masking is pointless |
| **Session invalidation on IP change** | Breaks mobile users constantly; VPNs | Accept some IP variance; monitor for anomalies |
| **Storing encrypted passwords** (not hashed) | Encryption is reversible; hashing is not | Use bcrypt (already implemented) |
| **Custom crypto algorithms** | Even experts get it wrong | Use standard libraries (crypto/rand, bcrypt) |
| **Over-logging sensitive data** | Logs become attack vector; compliance risk | Log events, not data; redact PII |

---

## Differentiators (Beyond Table Stakes)

Features that could differentiate FastCRM but are NOT required for basic compliance.

| Feature | Value Proposition | Complexity | Priority |
|---------|-------------------|------------|----------|
| **MFA (TOTP/WebAuthn)** | Enterprise requirement; becoming expected | High | P3 - Sprint 3 |
| **SSO integration** (SAML/OIDC) | Enterprise sales enabler | High | P4 - Backlog |
| **Per-tenant encryption keys** | Each org's data encrypted separately | High | P4 - Backlog |
| **Real-time anomaly detection** | Detect unusual access patterns | High | P5 - Future |
| **Compliance dashboard** | Self-service audit reports | Medium | P4 - Backlog |
| **Field-level encryption** | Encrypt specific sensitive fields | Medium | P4 - Backlog |
| **Audit log export** | Download for external SIEM | Low | P3 - Sprint 3 |
| **API token IP restrictions** | Limit API tokens to specific IPs | Low | P4 - Backlog |
| **Session management UI** | Users see/revoke active sessions | Low | P3 - Sprint 3 |

---

## Feature Dependencies

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Security Feature Dependencies                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  P0 (Immediate) - No dependencies                                           │
│  ├── Secure CORS                                                            │
│  ├── Rate Limiting                                                          │
│  ├── Error Sanitization                                                     │
│  ├── JWT Secret Enforcement                                                 │
│  └── HSTS                                                                   │
│                                                                              │
│  P1 (Sprint 1) - Build on P0                                                │
│  ├── Security Headers ─────────────────────────────────────────────┐        │
│  ├── HttpOnly Cookies ─────────┬─► Requires: HTTPS working         │        │
│  ├── Token Rotation ───────────┘                                   │        │
│  ├── Password Policy (NIST)                                        │        │
│  ├── Request Size Limits                                           │        │
│  └── Input Validation                                              │        │
│                                                                     │        │
│  P2 (Sprint 2) - Build on P1                                       │        │
│  ├── Session Timeout                                               │        │
│  ├── CSRF Protection ─────────► Requires: Token Rotation           │        │
│  ├── Tenant Isolation Tests                                        │        │
│  ├── Audit Logging ────────────────────────────────────────────────┘        │
│  │   └── (Enables all compliance reporting)                                 │
│  └── Vulnerability Scanning                                                 │
│                                                                              │
│  P3 (Sprint 3) - Build on P2                                                │
│  ├── Right to Erasure ─────────► Requires: Audit Logging                    │
│  ├── Data Export ──────────────► Requires: Audit Logging                    │
│  ├── MFA (Optional) ───────────► Requires: Token Rotation                   │
│  ├── Session Management UI                                                  │
│  ├── Access Review Reports ────► Requires: Audit Logging                    │
│  └── Breach Notification                                                    │
│                                                                              │
│  P4 (Backlog)                                                               │
│  ├── SSO (SAML/OIDC)                                                        │
│  ├── Per-tenant Encryption Keys                                             │
│  └── Compliance Dashboard ─────► Requires: Audit Logging                    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## MVP Security Recommendation

For MVP (minimum viable product), prioritize **P0 and P1** features:

### Must Complete Before Production

1. **P0 - Immediate (1-2 days):**
   - Fix CORS to use environment-based allowlist
   - Add rate limiting to auth endpoints (5/min per IP)
   - Sanitize all error responses
   - Enforce strong JWT secret in production
   - Add HSTS header

2. **P1 - Sprint 1 (1-2 weeks):**
   - Implement security headers middleware
   - Move refresh tokens to HttpOnly cookies
   - Implement token rotation with reuse detection
   - Update password validation per NIST 800-63B
   - Add request body size limits
   - Review and harden input validation

### Defer to Post-MVP

- **Audit logging infrastructure** (P2) - Important for compliance but not blocking launch
- **Right to erasure workflow** (P3) - Required for GDPR, but can handle manually initially
- **MFA** (P3) - Nice to have, not blocking
- **SSO** (P4) - Enterprise feature, defer

---

## Implementation Complexity Legend

| Level | Definition | Typical Effort |
|-------|------------|----------------|
| **Low** | Configuration change or simple middleware | 1-4 hours |
| **Medium** | New feature with database changes | 1-3 days |
| **High** | Architectural change or complex integration | 1-2 weeks |

---

## Sources

### Official Documentation (HIGH confidence)
- [OWASP Top 10 2025](https://owasp.org/Top10/2025/0x00_2025-Introduction/)
- [OWASP API Security Top 10](https://owasp.org/API-Security/)
- [NIST SP 800-63B Digital Identity Guidelines](https://pages.nist.gov/800-63-4/sp800-63b.html)
- [GDPR Article 17 - Right to Erasure](https://gdpr-info.eu/art-17-gdpr/)
- [GDPR Article 32 - Security of Processing](https://gdpr-info.eu/art-32-gdpr/)
- [GDPR Article 33 - Breach Notification](https://gdpr-info.eu/art-33-gdpr/)
- [ICO Right to Erasure Guidance](https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/individual-rights/individual-rights/right-to-erasure/)
- [HTTP Security Headers Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/HTTP_Headers_Cheat_Sheet.html)
- [MDN HSTS Documentation](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Strict-Transport-Security)
- [MDN Content Security Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CSP)
- [Auth0 Token Best Practices](https://auth0.com/docs/secure/tokens/token-best-practices)
- [Fiber Middleware Documentation](https://docs.gofiber.io/next/category/-middleware/)

### Industry Guidance (MEDIUM confidence)
- [SOC 2 Compliance Guide - Drata](https://drata.com/blog/nist-password-guidelines)
- [SOC 2 for SaaS - Sprinto](https://sprinto.com/blog/why-soc-2-for-saas-companies/)
- [Multi-Tenant Isolation Patterns - Security Boulevard](https://securityboulevard.com/2025/12/tenant-isolation-in-multi-tenant-systems-architecture-identity-and-security/)
- [Turso Encryption for SQLite](https://turso.tech/blog/fully-open-source-encryption-for-sqlite-b3858225)
- [SQLite Security Best Practices](https://dev.to/stephenc222/basic-security-practices-for-sqlite-safeguarding-your-data-23lh)
- [Session Security in 2025](https://www.techosquare.com/blog/session-security-in-2025-what-works-for-cookies-tokens-and-rotation)
- [SvelteKit CSRF Protection](https://dev.to/maxiviper117/implementing-csrf-protection-in-sveltekit-3afb)

### Project-Specific
- FastCRM SECURITY_ASSESSMENT.md (January 2026 internal audit)
