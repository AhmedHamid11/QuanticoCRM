# Quantico CRM

## What This Is

A high-performance, multi-tenant CRM rebuilt from EspoCRM concepts. Go/Fiber backend with SvelteKit frontend, using Turso (SQLite edge) for per-tenant databases. Focuses on speed (<50ms perceived), optimistic UI, security hardened for production, transparent platform updates, and built-in data quality tools including deduplication and merge.

## Core Value

Fast, secure multi-tenant CRM where customer data is protected and platform updates are transparent.

## Current State

**Shipped:** v3.0 Deduplication System (2026-02-09)
**Codebase:** ~96,000 LOC (58K Go, 38K TypeScript/Svelte)
**Status:** Production-ready with security hardening, platform updates, and data quality tools

**Milestones shipped:**
- v1.0 Platform Update System (2026-02-01)
- v2.0 Security Hardening (2026-02-04)
- v3.0 Deduplication System (2026-02-09)

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
- ✓ Scoring-based duplicate detection with configurable thresholds — v3.0
- ✓ Import duplicate detection with blocking review — v3.0
- ✓ Manual merge with field-by-field selection — v3.0
- ✓ Entity-agnostic deduplication engine — v3.0
- ✓ Background duplicate scanning jobs — v3.0
- ✓ Duplicate management admin UI — v3.0

### Active

<!-- Current scope. Building toward these. -->

- [ ] Merge instruction payload builder from dedup results — v4.0
- [ ] Salesforce OAuth 2.0 integration with Connected App — v4.0
- [ ] Batch merge instruction delivery to Salesforce staging object — v4.0
- [ ] Rate limiting & exponential backoff for Salesforce API — v4.0
- [ ] Audit logging for merge instruction delivery — v4.0
- [ ] Admin UI: integration configuration and delivery monitoring — v4.0

### Out of Scope

<!-- Explicit boundaries. Includes reasoning to prevent re-adding. -->

- SSO (SAML/OIDC) — Enterprise feature, consider for future milestone
- Per-tenant encryption keys — High complexity, defer
- Real-time anomaly detection — Advanced ML feature, future
- Mobile app — Web-first, mobile later
- Mandatory password complexity rules — NIST removed; leads to weaker passwords
- Periodic password expiration — NIST removed; causes predictable patterns
- Cross-entity duplicate detection (Lead-to-Contact) — Complex, deferred from v3.0
- ML-based matching algorithms — Jaro-Winkler sufficient for now
- Auto-merge without review — Risk of data loss, manual review preferred
- Negative signal scoring (DETECT-07) — Framework ready in scorer.go, logic not yet implemented

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

**Data Quality (v3.0):**
- Entity-agnostic deduplication with Jaro-Winkler fuzzy matching
- Real-time detection on record creation
- CSV import duplicate review with resolution actions
- Background scanning with checkpoint recovery
- Full admin UI for rule management, review queue, merge wizard

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
| Jaro-Winkler for fuzzy matching | Industry-standard name similarity, 0.88 threshold | ✓ Good |
| Weighted field scoring for confidence | Flexible per-field importance, 0-100 scale | ✓ Good |
| SQL blocking strategies | Reduces candidate set before expensive comparisons | ✓ Good |
| Async detection on record create | Optimistic save, non-blocking UX | ✓ Good |
| Single-page merge wizard | User-preferred, simpler than multi-step | ✓ Good |
| 30-day undo window for merges | Safety net without indefinite storage | ✓ Good |
| Checkpoint-based background scanning | Handles Turso 5-second timeout, resume on failure | ✓ Good |
| Frequency presets over cron | Covers 95% of use cases, much simpler UX | ✓ Good |

## Current Milestone: v4.0 Salesforce Merge Integration

**Goal:** Send merge instructions from Quantico to Salesforce so customers can use Quantico as a standalone dedup/merge tool that syncs results back to their Salesforce org.

**Target features:**
- Payload builder: construct merge instructions from dedup results (winner, loser, field values)
- Salesforce integration: OAuth 2.0 authentication, REST API delivery to staging object
- Batch delivery: group instructions for efficiency, rate limiting, exponential backoff
- Audit tracking: log all merge instructions sent, delivery status, outcomes
- Admin UI: configure Salesforce org, test connection, monitor delivery status

---
*Last updated: 2026-02-09 after starting v4.0 milestone*
