# Phase 1: Platform Versioning - Context

**Gathered:** 2026-01-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Track platform schema version and each org's current version. All orgs stay on the same version — updates roll out to everyone automatically. This phase establishes version tracking infrastructure that change tracking and notifications build upon.

</domain>

<decisions>
## Implementation Decisions

### Version Format
- Semver format: X.Y.Z (major.minor.patch)
- Standard semver rules: major = breaking changes, minor = new features, patch = fixes
- Starting version: 0.1.0
- No pre-release labels (no -beta, -rc suffixes) — keep it simple

### Storage Location
- Platform version stored in database table (replicated to each org's Turso DB)
- Version history table with timestamps (track when versions were released)
- Org version as column on orgs table (current version only, no history)
- Each org DB has its own copy of version info

### Versioning Model
- All orgs always on the same version — no multi-version support
- Updates are automatic, not opt-in
- Simpler codebase: no version-conditional logic needed
- Users informed via changelog, but don't control timing

### Update Propagation
- Version bump happens automatically on deploy (CI/CD)
- Background job updates all org databases after deploy
- Not lazy/on-demand — proactive sync to all orgs

### Notifications
- Changelog section in admin panel (not in-app banner)
- Users can see what changed in each version

### Claude's Discretion
- Exact database schema for version tables
- Background job implementation (queue, worker, etc.)
- CI/CD integration details
- Error handling for failed org updates

</decisions>

<specifics>
## Specific Ideas

- "I am always going to opt for the simple solution" — simplicity is a priority
- Version tracking enables future features but shouldn't over-engineer for them

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-platform-versioning*
*Context gathered: 2026-01-31*
