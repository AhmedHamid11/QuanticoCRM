# Quantico CRM

## What This Is

A high-performance, multi-tenant CRM rebuilt from EspoCRM concepts. Go/Fiber backend with SvelteKit frontend, using Turso (SQLite edge) for per-tenant databases. Focuses on speed (<50ms perceived), optimistic UI, and transparent platform updates.

## Core Value

Fast, secure multi-tenant CRM where customer data is protected and platform updates are transparent.

## Current Milestone: v2.0 Security Hardening

**Goal:** Address critical security vulnerabilities and achieve SOC 2/GDPR readiness for production deployment.

**Target features:**
- Fix critical vulnerabilities (CORS, JWT, error disclosure, XSS)
- Implement security hardening (rate limiting, token rotation, session management)
- Add audit logging infrastructure
- Achieve OWASP Top 10 2025 compliance

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

- ✓ Platform version tracking with semver — v1.0
- ✓ Structured changelog system — v1.0
- ✓ Admin changelog UI — v1.0
- ✓ Automatic org database migration — v1.0
- ✓ Version-aware org provisioning — v1.0
- ✓ JWT authentication with refresh tokens — existing
- ✓ Bcrypt password hashing — existing
- ✓ Per-tenant database isolation — existing
- ✓ Role-based access (admin/user) — existing

### Active

<!-- Current scope. Building toward these. -->

See: `.planning/REQUIREMENTS.md` for v2.0 requirements

### Out of Scope

<!-- Explicit boundaries. Includes reasoning to prevent re-adding. -->

- SSO (SAML/OIDC) — Enterprise feature, defer to v3.0
- Per-tenant encryption keys — High complexity, defer
- Real-time anomaly detection — Advanced ML feature, future
- Mobile app — Web-first, mobile later

## Context

**Security Assessment (2026-01-31):**
- 5 Critical vulnerabilities identified in codebase audit
- 5 High severity issues requiring attention
- CORS wildcard (*) allows any origin
- JWT secret has weak default fallback
- Tokens stored in localStorage (XSS vulnerable)
- No rate limiting on auth endpoints

**Compliance targets:**
- SOC 2 Type II certifiable
- GDPR Article 32 compliant
- OWASP Top 10 2025 addressed

## Constraints

- **Stack**: Go 1.22+/Fiber, SvelteKit 2.x, Turso — no changes
- **Deployment**: Railway (backend), Vercel (frontend)
- **Backwards compatibility**: Existing auth tokens must remain valid during migration
- **Performance**: Security measures must not degrade <50ms response time target

## Key Decisions

<!-- Decisions that constrain future work. Add throughout project lifecycle. -->

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Per-tenant Turso databases | Isolation by default, simpler security model | ✓ Good |
| JWT with refresh tokens | Stateless auth, scalable | — Pending (needs rotation) |
| localStorage for tokens | Simple implementation | ⚠️ Revisit (XSS risk) |

---
*Last updated: 2026-02-03 after milestone v2.0 start*
