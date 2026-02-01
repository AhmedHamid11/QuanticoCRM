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
- Structured change records (category + description)
- Human-readable change notes for each update
- API to query changes between any two versions

**Delivers:** Changelog system with queryable change history

**Plans:** 1 plan

Plans:
- [x] 02-01-PLAN.md — Changelog package and API endpoints

---

### Phase 3: Changelog UI
**Goal:** Let org admins see what changed in each version.

- Admin panel changelog section
- Display changes with clear descriptions per version
- Show what was added/modified in recent updates
- Version history visible to admins

**Delivers:** Changelog page in admin panel

**Plans:** 1 plan

Plans:
- [x] 03-01-PLAN.md — Changelog page and admin link

---

### Phase 4: Update Propagation
**Goal:** Automatically update all org databases after deploy.

- Blocking migration on startup before accepting requests
- Sequential processing of orgs with skip-and-continue on failures
- Transaction-safe update process per org
- Admin visibility into migration status on changelog page
- Manual retry for failed orgs from admin panel

**Delivers:** Automatic org update mechanism with admin visibility

**Plans:** 3 plans

Plans:
- [x] 04-01-PLAN.md — Migration tracking schema, entity, and repository
- [x] 04-02-PLAN.md — MigrationPropagator service and startup integration
- [x] 04-03-PLAN.md — API endpoints and changelog page migration status UI

---

### Phase 5: New Org Provisioning
**Goal:** New orgs automatically get the latest platform version.

- Integrate versioning with org creation flow
- New orgs stamped with current platform version
- All standard entities/fields/layouts from latest version
- Consistent starting point for all new orgs

**Delivers:** Version-aware org provisioning

**Plans:** 1 plan

Plans:
- [ ] 05-01-PLAN.md — Wire VersionRepo into AuthService for version-aware org creation

---

## Status

| Phase | Status | Notes |
|-------|--------|-------|
| 1. Platform Versioning | Complete | 2 plans executed, verified |
| 2. Change Tracking | Complete | 1 plan executed, verified |
| 3. Changelog UI | Complete | 1 plan executed, verified |
| 4. Update Propagation | Complete | 3 plans executed, verified |
| 5. New Org Provisioning | Planned | 1 plan ready |

---

*Milestone: Platform Update System*
*Created: 2026-01-31*
