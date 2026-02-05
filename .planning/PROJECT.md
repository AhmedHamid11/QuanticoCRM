# Quantico CRM

## What This Is

A high-performance, multi-tenant CRM rebuilt from EspoCRM concepts. Go/Fiber backend with SvelteKit frontend, using Turso (SQLite edge) for per-tenant databases. Focuses on speed (<50ms perceived), optimistic UI, security hardened for production, and transparent platform updates.

## Core Value

Fast, secure multi-tenant CRM where customer data is protected and platform updates are transparent.

## Current State

**Shipped:** v2.0 Security Hardening (2026-02-04)
**Codebase:** 218,000+ LOC (Go/TypeScript/Svelte)
**Status:** Production-ready with SOC 2/GDPR compliance foundation

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
- ✓ CORS lockdown with origin allowlist — v2.0
- ✓ Auth rate limiting (5/min per IP) — v2.0
- ✓ Error sanitization (no stack traces) — v2.0
- ✓ JWT secret validation — v2.0
- ✓ HSTS enforcement (1-year max-age) — v2.0
- ✓ Security headers (X-Frame-Options, CSP) — v2.0
- ✓ HttpOnly refresh token cookies — v2.0
- ✓ Memory-only access tokens — v2.0
- ✓ Token rotation with reuse detection — v2.0
- ✓ NIST 800-63B password policy — v2.0
- ✓ Request body size limits — v2.0
- ✓ Session timeouts (30min idle, 24h absolute) — v2.0
- ✓ CSRF protection (double-submit cookie) — v2.0
- ✓ Tenant isolation verified — v2.0
- ✓ Tamper-evident audit logging — v2.0
- ✓ CI security scanning (gosec) — v2.0

### Active

<!-- Current scope. Building toward these. -->

Ready for next milestone. See `/gsd:new-milestone` to define v3.0 requirements.

### Out of Scope

<!-- Explicit boundaries. Includes reasoning to prevent re-adding. -->

- SSO (SAML/OIDC) — Enterprise feature, defer to v3.0
- Per-tenant encryption keys — High complexity, defer
- Real-time anomaly detection — Advanced ML feature, future
- Mobile app — Web-first, mobile later
- Mandatory password complexity rules — NIST removed; leads to weaker passwords
- Periodic password expiration — NIST removed; causes predictable patterns

## Context

**Tech Stack:**
- Backend: Go 1.22 / Fiber v2.52.0
- Frontend: SvelteKit 2.x / TypeScript
- Database: Turso (SQLite edge) with per-tenant isolation
- Deployment: Railway (backend), Vercel (frontend)

**Security Posture (v2.0):**
- OWASP Top 10 2025 addressed
- XSS-immune token storage
- Tamper-evident audit trails
- CI security scanning enforced

## Constraints

- **Stack**: Go 1.22+/Fiber, SvelteKit 2.x, Turso — no changes
- **Deployment**: Railway (backend), Vercel (frontend)
- **Performance**: <50ms response time target
- **Backwards compatibility**: Must maintain existing integrations

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Per-tenant Turso databases | Isolation by default, simpler security model | ✓ Good |
| JWT with refresh tokens | Stateless auth, scalable | ✓ Good (with rotation) |
| HttpOnly cookie for refresh token | XSS immunity | ✓ Good |
| Memory-only access tokens | XSS immunity | ✓ Good |
| Token family rotation | Reuse detection | ✓ Good |
| NIST 800-63B password policy | Modern security best practice | ✓ Good |
| Silent CORS reject | Prevent origin enumeration | ✓ Good |
| SHA-256 hash chain for audit | Tamper evidence | ✓ Good |

---
*Last updated: 2026-02-04 after v2.0 milestone*
