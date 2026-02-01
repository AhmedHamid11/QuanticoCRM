# Quantico CRM Roadmap

## Milestone 1: Platform Update System

**Goal:** Enable versioned platform updates that orgs can adopt on their own schedule with full transparency.

**Principle:** Updates are additive. Existing org data is never altered. All orgs stay on the same version (auto-updated on deploy).

---

### Phase 1: Platform Versioning
**Goal:** Track platform schema version and each org's current version.

- Platform version stored centrally (increments with schema changes)
- Each org tracks which platform version they're on
- Version comparison to detect available updates
- Foundation for all update features

**Delivers:** Version tracking infrastructure

**Plans:** 2 plans

Plans:
- [x] 01-01-PLAN.md — Database schema and version comparison service
- [x] 01-02-PLAN.md — Version API endpoints

---

### Phase 2: Change Tracking
**Goal:** Record what changed between platform versions.

- Changelog entries for each version bump
- Structured change records (entity added, field added, layout modified, etc.)
- Human-readable change notes for each update
- API to query changes between any two versions

**Delivers:** Changelog system with queryable change history

---

### Phase 3: Changelog UI
**Goal:** Let org admins see what changed in each version.

- Admin panel changelog section
- Display changes with clear descriptions per version
- Show what was added/modified in recent updates
- Version history visible to admins

**Delivers:** Changelog page in admin panel

---

### Phase 4: Update Propagation
**Goal:** Automatically update all org databases after deploy.

- Background job runs after deploy
- Apply schema changes additively to all orgs
- Transaction-safe update process
- Retry failed orgs, alert on persistent failures

**Delivers:** Automatic org update mechanism

---

### Phase 5: New Org Provisioning
**Goal:** New orgs automatically get the latest platform version.

- Integrate versioning with org creation flow
- New orgs stamped with current platform version
- All standard entities/fields/layouts from latest version
- Consistent starting point for all new orgs

**Delivers:** Version-aware org provisioning

---

## Status

| Phase | Status | Notes |
|-------|--------|-------|
| 1. Platform Versioning | ✓ Complete | 2 plans executed, verified |
| 2. Change Tracking | Not started | |
| 3. Changelog UI | Not started | |
| 4. Update Propagation | Not started | |
| 5. New Org Provisioning | Not started | |

---

*Milestone: Platform Update System*
*Created: 2026-01-31*
