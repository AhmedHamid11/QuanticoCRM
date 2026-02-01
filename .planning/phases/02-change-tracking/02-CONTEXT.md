# Phase 2: Change Tracking - Context

**Gathered:** 2026-01-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Record what changed in the **platform codebase** between versions. This tracks code changes (features, fixes, API changes) — not org-specific metadata or data. Each org's custom entities, fields, and layouts are their own and are not platform changes.

</domain>

<decisions>
## Implementation Decisions

### What gets tracked
- Changes to the platform codebase only
- NOT org-specific metadata (custom entities, fields, layouts)
- NOT org data
- Examples: new features, bug fixes, API changes, UI changes, internal improvements

### Change categories
- Conventional changelog format:
  - Added
  - Changed
  - Fixed
  - Removed
  - Deprecated
  - Security

### Entry structure
- Simple entries: category + description text
- No additional structure needed (no affected area, no links)

### Entry creation
- Manual entries written by developer when bumping platform version
- Not extracted from git commits
- Not managed through admin UI

### Claude's Discretion
- Database schema for storing changelog entries
- API design for querying changes between versions
- How entries are organized in the codebase (file-based vs database)

</decisions>

<specifics>
## Specific Ideas

- Phase 1's versioning tracked platform code version, not metadata — this continues that pattern
- Keep it simple: category + description is sufficient

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-change-tracking*
*Context gathered: 2026-01-31*
