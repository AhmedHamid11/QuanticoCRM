# Quantico CRM Roadmap

## Milestones

- **v1.0 Platform Update System** — Phases 01-05 (shipped 2026-02-01) → [archive](milestones/v1.0-ROADMAP.md)
- **v2.0 Security Hardening** — Phases 06-10 (in progress)

---

# Milestone v2.0: Security Hardening

**Goal:** Address critical security vulnerabilities and achieve SOC 2/GDPR readiness for production deployment.
**Phases:** 06-10 (5 phases)
**Requirements:** 22 mapped

## Phase Structure

| # | Phase | Goal | Requirements | Success Criteria |
|---|-------|------|--------------|------------------|
| 06 | Critical Fixes | Eliminate critical vulnerabilities blocking production deployment | CRIT-01, CRIT-02, CRIT-03, CRIT-04, CRIT-05 | 5 |
| 07 | Token Architecture | Secure token storage and rotation to prevent XSS token theft | HARD-02, HARD-03, HARD-04 | 4 |
| 08 | Security Hardening | Harden application against common attack vectors | HARD-01, HARD-05, HARD-06, HARD-07 | 4 |
| 09 | Session Management | Control session lifecycle and verify tenant isolation | SESS-01, SESS-02, SESS-03, SESS-04 | 4 |
| 10 | Audit Infrastructure | Track security events and enable compliance reporting | AUDT-01, AUDT-02, AUDT-03, AUDT-04, SCAN-01, SCAN-02 | 5 |

---

## Phases

### Phase 06: Critical Fixes

**Goal:** Eliminate critical vulnerabilities that block production deployment.
**Depends on:** Nothing (first phase of milestone)
**Requirements:** CRIT-01, CRIT-02, CRIT-03, CRIT-04, CRIT-05
**Plans:** 5 plans (3 original + 2 gap closure)

**Success Criteria:**

1. API rejects requests from non-allowlisted origins in production
2. Auth endpoint returns 429 after 5 requests per minute from same IP
3. Error responses never include database errors, stack traces, or schema details
4. Application refuses to start in production without JWT_SECRET environment variable
5. All HTTP responses include HSTS header with 1-year max-age

Plans:
- [x] 06-01-PLAN.md — Foundation security (CORS, HSTS, JWT validation)
- [x] 06-02-PLAN.md — Auth rate limiting
- [x] 06-03-PLAN.md — Error sanitization (critical handlers)
- [ ] 06-04-PLAN.md — Gap closure: sanitize high-occurrence handlers (contact, account, schema, admin, etc.)
- [ ] 06-05-PLAN.md — Gap closure: sanitize remaining handlers (task, bulk, lookup, etc.)

---

### Phase 07: Token Architecture

**Goal:** Secure token storage and rotation to prevent XSS token theft.
**Depends on:** Phase 06 (HSTS must be working for Secure cookies)
**Requirements:** HARD-02, HARD-03, HARD-04

**Success Criteria:**

1. Refresh tokens are stored in HttpOnly cookies (not accessible via JavaScript)
2. Access tokens exist only in memory (not in localStorage or sessionStorage)
3. Token refresh returns new refresh token (rotation), invalidating the old one
4. Reusing an old refresh token invalidates entire token family (reuse detection)

**Plans:** TBD

---

### Phase 08: Security Hardening

**Goal:** Harden application against common attack vectors.
**Depends on:** Phase 06 (foundation security must be in place)
**Requirements:** HARD-01, HARD-05, HARD-06, HARD-07

**Success Criteria:**

1. Response headers include X-Frame-Options: DENY, X-Content-Type-Options: nosniff, and Content-Security-Policy
2. Password registration rejects passwords under 8 characters and accepts passwords up to 128 characters
3. Password registration warns or blocks passwords found in breach database
4. API rejects request bodies larger than configured limit (default 1MB)

**Plans:** TBD

---

### Phase 09: Session Management

**Goal:** Control session lifecycle and verify tenant isolation.
**Depends on:** Phase 07 (token architecture must be complete for session control)
**Requirements:** SESS-01, SESS-02, SESS-03, SESS-04

**Success Criteria:**

1. User is logged out automatically after 30 minutes of inactivity
2. User is logged out automatically after 24 hours regardless of activity
3. State-changing requests without valid CSRF token are rejected
4. Integration tests verify no data leakage between tenants

**Plans:** TBD

---

### Phase 10: Audit Infrastructure

**Goal:** Track security events and enable compliance reporting.
**Depends on:** Phases 06-09 (all security controls must exist to audit them)
**Requirements:** AUDT-01, AUDT-02, AUDT-03, AUDT-04, SCAN-01, SCAN-02

**Success Criteria:**

1. Login success/failure, logout, and password changes are recorded in audit log
2. User CRUD, role changes, and org settings changes are recorded in audit log
3. Authorization failures (403 responses) are recorded with actor and resource details
4. Audit logs are append-only with tamper-evident integrity verification
5. CI pipeline runs gosec and fails on high-severity findings

**Plans:** TBD

---

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 06. Critical Fixes | v2.0 | 3/5 | Gap closure needed | - |
| 07. Token Architecture | v2.0 | 0/TBD | Not started | - |
| 08. Security Hardening | v2.0 | 0/TBD | Not started | - |
| 09. Session Management | v2.0 | 0/TBD | Not started | - |
| 10. Audit Infrastructure | v2.0 | 0/TBD | Not started | - |

---

*Created: 2026-02-03*
*Last updated: 2026-02-04*
