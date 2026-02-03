# Requirements: Quantico CRM

**Defined:** 2026-02-03
**Core Value:** Fast, secure multi-tenant CRM where customer data is protected

## v2.0 Requirements

Requirements for Security Hardening milestone. Each maps to roadmap phases.

### Critical Fixes (P0)

- [ ] **CRIT-01**: CORS configuration restricts origins to explicit allowlist (no wildcards in production)
- [ ] **CRIT-02**: Auth endpoints are rate limited (5 requests/minute per IP)
- [ ] **CRIT-03**: Error responses never expose internal details (database errors, stack traces, schema)
- [ ] **CRIT-04**: JWT secret is required in production (no fallback to weak default)
- [ ] **CRIT-05**: HSTS header enforces HTTPS with 1-year max-age

### Security Hardening (P1)

- [ ] **HARD-01**: Security headers set (X-Frame-Options, X-Content-Type-Options, Content-Security-Policy)
- [ ] **HARD-02**: Refresh tokens stored in HttpOnly, Secure, SameSite=Strict cookies
- [ ] **HARD-03**: Access tokens stored in memory only (not localStorage)
- [ ] **HARD-04**: Token rotation implemented with family tracking for reuse detection
- [ ] **HARD-05**: Password policy follows NIST 800-63B (length-based, breach check, no complexity rules)
- [ ] **HARD-06**: Request body size limited (prevent DoS via large payloads)
- [ ] **HARD-07**: Input validation hardened across all endpoints (prevent injection)

### Session & Audit (P2)

- [ ] **SESS-01**: Sessions expire after idle timeout (30 minutes)
- [ ] **SESS-02**: Sessions have absolute timeout (24 hours max)
- [ ] **SESS-03**: CSRF protection via double-submit cookie pattern
- [ ] **SESS-04**: Multi-tenant isolation verified with integration tests
- [ ] **AUDT-01**: Audit log infrastructure captures authentication events
- [ ] **AUDT-02**: Audit log captures admin actions (user CRUD, role changes)
- [ ] **AUDT-03**: Audit log captures authorization failures
- [ ] **AUDT-04**: Audit logs retained with tamper-evident storage
- [ ] **SCAN-01**: SAST scanning integrated (gosec for Go)
- [ ] **SCAN-02**: Dependency vulnerability monitoring enabled (nancy/Dependabot)

## v3.0 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Authentication Enhancements

- **AUTH-01**: User can enable MFA via TOTP authenticator app
- **AUTH-02**: User can use WebAuthn/passkey for passwordless login
- **AUTH-03**: SSO integration via SAML for enterprise customers
- **AUTH-04**: SSO integration via OIDC for enterprise customers

### GDPR Compliance

- **GDPR-01**: User can request data erasure (right to be forgotten)
- **GDPR-02**: User can export all personal data (data portability)
- **GDPR-03**: Data retention policies auto-delete after defined periods
- **GDPR-04**: Consent tracking records when/how consent was given

### Advanced Security

- **ASEC-01**: Per-tenant encryption keys for data at rest
- **ASEC-02**: Field-level encryption for highly sensitive data
- **ASEC-03**: Real-time anomaly detection for unusual access patterns
- **ASEC-04**: API token IP restrictions

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Mandatory password complexity rules | NIST 800-63B explicitly removed; leads to weaker passwords |
| Periodic password expiration | NIST removed; causes predictable patterns |
| Security questions | Easily researched via social engineering; use MFA instead |
| CAPTCHA on every form | Frustrates users; rate limiting is more effective |
| IP-based blocking as primary defense | IPs easily rotated; use as one signal only |
| Session invalidation on IP change | Breaks mobile users and VPN users |
| Custom cryptographic algorithms | Use standard libraries only |
| Real-time chat encryption | Not building chat features |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CRIT-01 | Phase 06 | Pending |
| CRIT-02 | Phase 06 | Pending |
| CRIT-03 | Phase 06 | Pending |
| CRIT-04 | Phase 06 | Pending |
| CRIT-05 | Phase 06 | Pending |
| HARD-01 | Phase 08 | Pending |
| HARD-02 | Phase 07 | Pending |
| HARD-03 | Phase 07 | Pending |
| HARD-04 | Phase 07 | Pending |
| HARD-05 | Phase 08 | Pending |
| HARD-06 | Phase 08 | Pending |
| HARD-07 | Phase 08 | Pending |
| SESS-01 | Phase 09 | Pending |
| SESS-02 | Phase 09 | Pending |
| SESS-03 | Phase 09 | Pending |
| SESS-04 | Phase 09 | Pending |
| AUDT-01 | Phase 10 | Pending |
| AUDT-02 | Phase 10 | Pending |
| AUDT-03 | Phase 10 | Pending |
| AUDT-04 | Phase 10 | Pending |
| SCAN-01 | Phase 10 | Pending |
| SCAN-02 | Phase 10 | Pending |

**Coverage:**
- v2.0 requirements: 22 total
- Mapped to phases: 22
- Unmapped: 0

---
*Requirements defined: 2026-02-03*
*Last updated: 2026-02-03 after roadmap creation*
