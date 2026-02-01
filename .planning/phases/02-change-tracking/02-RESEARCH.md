# Phase 2: Change Tracking - Research

**Researched:** 2026-01-31
**Domain:** Platform changelog management, version-to-change association, API design for change queries
**Confidence:** HIGH

## Summary

Change tracking for this platform involves recording what changed between platform versions using a conventional changelog format (Added, Changed, Fixed, Removed, Deprecated, Security). Based on user decisions, entries are simple (category + description), created manually by developers when bumping platform version, and NOT extracted from git commits or managed through admin UI.

The implementation approach has two viable paths:
1. **File-based (Recommended):** Store changelog entries in a structured CHANGELOG.md file or Go source file, read at runtime
2. **Database-based:** Store entries in a `changelog_entries` table linked to `platform_versions`

Given that entries are manually written by developers (not dynamically generated), file-based storage is simpler and keeps changelog entries version-controlled alongside code. The database approach adds value only if entries need to be queried dynamically or edited via admin UI (explicitly out of scope per user decisions).

**Primary recommendation:** Use a Go source file (`internal/changelog/entries.go`) containing a map of version to changelog entries. This approach keeps entries version-controlled, requires no migration, and is simple to query via the existing version API.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/mod/semver | Latest | Version comparison (already installed from Phase 1) | Reuse existing dependency for version ordering |
| database/sql | stdlib | Database operations if using DB storage | Already in use throughout codebase |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/gofiber/fiber/v2 | v2.52.0 | HTTP framework (already installed) | API endpoints for changelog queries |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Go source file | CHANGELOG.md + parser | Markdown is more human-readable but requires parsing; Go source is simpler and type-safe |
| Go source file | Database table | Database enables dynamic editing but user explicitly said no admin UI for entries |
| Embedded entries | External API | External changelog services exist but add complexity; embedded is simpler for this use case |

**Installation:**
```bash
# No new dependencies required - reuses Phase 1 stack
```

## Architecture Patterns

### Recommended Project Structure
```
backend/
├── internal/
│   ├── changelog/
│   │   └── entries.go        # Changelog entries by version (Go source)
│   ├── service/
│   │   └── versioning.go     # Extended to query changelog entries
│   ├── repo/
│   │   └── version.go        # Extended if using DB storage
│   └── handler/
│       └── version.go        # Extended with changelog endpoints
└── migrations/
    └── 043_create_changelog_entries.sql  # Only if using DB approach
```

### Pattern 1: Go Source Changelog Storage (Recommended)

**What:** Store changelog entries as a Go map in source code
**When to use:** When entries are manually created by developers and don't need dynamic editing
**Why:** Version-controlled, type-safe, no migration required, dead simple

**Example:**
```go
// internal/changelog/entries.go
package changelog

// Category represents a changelog entry type following Keep a Changelog convention
type Category string

const (
    CategoryAdded      Category = "Added"
    CategoryChanged    Category = "Changed"
    CategoryFixed      Category = "Fixed"
    CategoryRemoved    Category = "Removed"
    CategoryDeprecated Category = "Deprecated"
    CategorySecurity   Category = "Security"
)

// Entry represents a single changelog entry
type Entry struct {
    Category    Category `json:"category"`
    Description string   `json:"description"`
}

// VersionChangelog contains all entries for a specific version
type VersionChangelog struct {
    Version string  `json:"version"`
    Entries []Entry `json:"entries"`
}

// Entries maps version strings to their changelog entries
// Add new entries here when bumping platform version
var Entries = map[string][]Entry{
    "v0.1.0": {
        {CategoryAdded, "Initial platform version with core CRM entities"},
        {CategoryAdded, "Contact, Account, Task, Quote entity support"},
        {CategoryAdded, "Multi-tenant database architecture"},
    },
    "v0.2.0": {
        {CategoryAdded, "Platform versioning infrastructure"},
        {CategoryAdded, "Version tracking per organization"},
        {CategoryAdded, "Changelog API endpoints"},
    },
    // Add new versions here when releasing
}

// GetSortedVersions returns all versions in descending order (newest first)
func GetSortedVersions() []string {
    // Implementation uses semver.Compare for proper ordering
}

// GetEntriesForVersion returns changelog entries for a specific version
func GetEntriesForVersion(version string) ([]Entry, bool) {
    entries, ok := Entries[version]
    return entries, ok
}

// GetEntriesBetweenVersions returns all entries from versions > fromVersion and <= toVersion
func GetEntriesBetweenVersions(fromVersion, toVersion string) []VersionChangelog {
    // Returns entries for all versions in range, ordered newest to oldest
}
```

### Pattern 2: Database Changelog Storage (Alternative)

**What:** Store changelog entries in a database table linked to platform_versions
**When to use:** Only if dynamic editing via admin UI is needed (NOT in scope per user decisions)
**Schema:**
```sql
-- migrations/043_create_changelog_entries.sql
CREATE TABLE IF NOT EXISTS changelog_entries (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    category TEXT NOT NULL CHECK(category IN ('Added', 'Changed', 'Fixed', 'Removed', 'Deprecated', 'Security')),
    description TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (version) REFERENCES platform_versions(version) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_changelog_version ON changelog_entries(version);
CREATE INDEX IF NOT EXISTS idx_changelog_category ON changelog_entries(version, category);
```

### Pattern 3: API Response Structure

**What:** Standard response format for changelog queries
**When to use:** All changelog API endpoints

**Example:**
```go
// GET /api/v1/version/changelog
// Returns changelog for a specific version
type ChangelogResponse struct {
    Version string  `json:"version"`
    Entries []Entry `json:"entries"`
}

// GET /api/v1/version/changelog/since?from=v0.1.0
// Returns all changes since a specific version
type ChangelogSinceResponse struct {
    FromVersion string             `json:"fromVersion"`
    ToVersion   string             `json:"toVersion"`
    Changelogs  []VersionChangelog `json:"changelogs"`
}
```

### Anti-Patterns to Avoid
- **Auto-generating from git commits:** User explicitly decided against this; keep entries human-written
- **Storing in CHANGELOG.md and parsing at runtime:** Adds complexity; Go source is simpler
- **Dynamic admin UI for entries:** Out of scope per user decisions; keep it code-based
- **Separate changelog service:** Over-engineering; extend existing version handler/service

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Version comparison | String sorting | `semver.Compare()` from Phase 1 | String sorting fails for v10 vs v2 |
| Version ordering for changelog list | Custom sort logic | `semver.Compare` in sort function | Handles edge cases like pre-release |
| Category validation | String checks | Go constants with type | Type safety, IDE autocompletion |

**Key insight:** The changelog domain is simple enough that no external libraries are needed. Reuse Phase 1's semver library for version ordering; everything else is straightforward Go code.

## Common Pitfalls

### Pitfall 1: Version String Mismatch
**What goes wrong:** Changelog entries stored with "0.1.0" don't match version queries for "v0.1.0"
**Why it happens:** Inconsistent version prefix handling
**How to avoid:** Always normalize versions using `VersionService.Normalize()` from Phase 1 before storage or lookup
**Warning signs:** "No changelog found" for versions that should have entries

### Pitfall 2: Unordered Version Display
**What goes wrong:** Changelog versions displayed as ["v0.1.0", "v0.10.0", "v0.2.0"] instead of proper semver order
**Why it happens:** String sorting instead of semver sorting
**How to avoid:** Use `semver.Compare()` in sort function
**Warning signs:** v0.10.0 appearing before v0.9.0 in lists

### Pitfall 3: Missing Category Validation
**What goes wrong:** Typos like "Added" vs "added" or "Fix" vs "Fixed" cause inconsistent categorization
**Why it happens:** Using raw strings instead of constants
**How to avoid:** Use `Category` type constants; only accept valid enum values
**Warning signs:** Frontend receiving unexpected category strings

### Pitfall 4: Empty Changelog on New Versions
**What goes wrong:** New platform version released but changelog entries not added
**Why it happens:** Manual entry process with no enforcement
**How to avoid:**
- Add reminder in PR template: "Did you add changelog entries?"
- Optional: CI check that new version in platform_versions has corresponding entries
**Warning signs:** `GetEntriesForVersion()` returning empty for recent versions

### Pitfall 5: Querying "Changes Since" With Same Version
**What goes wrong:** `GetEntriesBetweenVersions("v0.1.0", "v0.1.0")` returns entries for v0.1.0 or empty unexpectedly
**Why it happens:** Ambiguous boundary condition handling
**How to avoid:** Define clear semantics: "since" means exclusive of fromVersion, inclusive of toVersion. Document this behavior.
**Warning signs:** Users seeing their current version's changes again after update

## Code Examples

Verified patterns from official sources and existing codebase:

### Changelog Package Implementation
```go
// internal/changelog/entries.go
package changelog

import (
    "sort"
    "golang.org/x/mod/semver"
)

type Category string

const (
    CategoryAdded      Category = "Added"
    CategoryChanged    Category = "Changed"
    CategoryFixed      Category = "Fixed"
    CategoryRemoved    Category = "Removed"
    CategoryDeprecated Category = "Deprecated"
    CategorySecurity   Category = "Security"
)

type Entry struct {
    Category    Category `json:"category"`
    Description string   `json:"description"`
}

type VersionChangelog struct {
    Version string  `json:"version"`
    Entries []Entry `json:"entries"`
}

// Entries maps version strings to their changelog entries
var Entries = map[string][]Entry{
    "v0.1.0": {
        {CategoryAdded, "Initial platform version"},
    },
}

// GetSortedVersions returns all versions in descending order (newest first)
func GetSortedVersions() []string {
    versions := make([]string, 0, len(Entries))
    for v := range Entries {
        versions = append(versions, v)
    }
    // Sort descending (newest first)
    sort.Slice(versions, func(i, j int) bool {
        return semver.Compare(versions[i], versions[j]) > 0
    })
    return versions
}

// GetEntriesForVersion returns changelog entries for a specific version
func GetEntriesForVersion(version string) ([]Entry, bool) {
    entries, ok := Entries[version]
    return entries, ok
}

// GetEntriesBetweenVersions returns entries for versions > fromVersion and <= toVersion
func GetEntriesBetweenVersions(fromVersion, toVersion string) []VersionChangelog {
    var result []VersionChangelog

    for _, version := range GetSortedVersions() {
        // Include if: version > fromVersion AND version <= toVersion
        if semver.Compare(version, fromVersion) > 0 && semver.Compare(version, toVersion) <= 0 {
            if entries, ok := Entries[version]; ok {
                result = append(result, VersionChangelog{
                    Version: version,
                    Entries: entries,
                })
            }
        }
    }

    return result
}
```

### Extended Version Handler
```go
// internal/handler/version.go (extended)

// GetChangelog returns changelog for a specific version
// GET /api/v1/version/changelog?version=v0.2.0
func (h *VersionHandler) GetChangelog(c *fiber.Ctx) error {
    version := c.Query("version")
    if version == "" {
        // Get latest platform version
        pv, err := h.repo.GetPlatformVersion(c.Context())
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "error": "Failed to get platform version: " + err.Error(),
            })
        }
        version = pv.Version
    }

    // Normalize version
    version = h.service.Normalize(version)

    entries, ok := changelog.GetEntriesForVersion(version)
    if !ok {
        return c.JSON(fiber.Map{
            "version": version,
            "entries": []changelog.Entry{},
        })
    }

    return c.JSON(fiber.Map{
        "version": version,
        "entries": entries,
    })
}

// GetChangelogSince returns all changes since a specific version
// GET /api/v1/version/changelog/since?from=v0.1.0
func (h *VersionHandler) GetChangelogSince(c *fiber.Ctx) error {
    fromVersion := c.Query("from")
    if fromVersion == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Missing 'from' query parameter",
        })
    }

    // Normalize from version
    fromVersion = h.service.Normalize(fromVersion)

    // Get current platform version as "to"
    pv, err := h.repo.GetPlatformVersion(c.Context())
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to get platform version: " + err.Error(),
        })
    }

    changelogs := changelog.GetEntriesBetweenVersions(fromVersion, pv.Version)

    return c.JSON(fiber.Map{
        "fromVersion": fromVersion,
        "toVersion":   pv.Version,
        "changelogs":  changelogs,
    })
}
```

### Route Registration
```go
// In handler/version.go RegisterRoutes()
func (h *VersionHandler) RegisterRoutes(router fiber.Router) {
    version := router.Group("/version")
    version.Get("/platform", h.GetPlatformVersion)
    version.Get("/current", h.GetCurrentVersion)
    version.Get("/history", h.GetVersionHistory)
    version.Get("/changelog", h.GetChangelog)          // NEW
    version.Get("/changelog/since", h.GetChangelogSince) // NEW
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CHANGELOG.md with manual updates | Structured changelog files or code | 2020s | Enables API access, type safety |
| Git commit parsing | Human-written entries | Preference varies | User decision: manual entries preferred |
| Database changelog tables | Embedded code or files | Depends on use case | Simpler for static entries |

**Deprecated/outdated:**
- Automatically generating changelogs from git commits (user explicitly decided against)
- Admin UI for changelog management (user explicitly decided against)

## Open Questions

Things that couldn't be fully resolved:

1. **Changelog Entry Timing**
   - What we know: Entries are added manually when bumping platform version
   - What's unclear: At what point in the release process should entries be added?
   - Recommendation: Add entries to `entries.go` in the same commit that updates the platform version. This keeps version and changelog in sync.

2. **Empty Changelog Handling**
   - What we know: API should handle versions with no entries gracefully
   - What's unclear: Should versions without entries return empty array or 404?
   - Recommendation: Return empty array with version, not 404. A version can exist without documented changes.

3. **Frontend Display Location**
   - What we know: Changelog needs to be queryable via API
   - What's unclear: Where in the UI will changelog be displayed?
   - Recommendation: Defer frontend location to Phase 2 planning; API design supports multiple consumption patterns.

## Sources

### Primary (HIGH confidence)
- [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) - Change category definitions (Added, Changed, Fixed, Removed, Deprecated, Security)
- [golang.org/x/mod/semver](https://pkg.go.dev/golang.org/x/mod/semver) - Version comparison (already in use from Phase 1)
- Existing codebase: Phase 1 implementation (`versioning.go`, `version.go` handler/repo)

### Secondary (MEDIUM confidence)
- [Conventional Commits](https://www.conventionalcommits.org/) - Commit message convention (not used, but informed category thinking)
- [HashiCorp go-changelog](https://pkg.go.dev/github.com/hashicorp/go-changelog) - File-per-change approach (inspired file-based thinking, but not used)

### Tertiary (LOW confidence)
- Various WebSearch results on changelog API design - Community patterns, not authoritative

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Reuses Phase 1 libraries, no new dependencies
- Architecture: HIGH - Simple Go source file approach is straightforward and verified
- API design: HIGH - Follows existing codebase patterns (handler/service/repo)
- Pitfalls: MEDIUM - Based on experience with version handling, some inferred

**Research date:** 2026-01-31
**Valid until:** 2026-03-31 (60 days - approach is stable, no external dependencies that might change)

**Next steps for planner:**
1. Create `internal/changelog/entries.go` with initial entries
2. Extend `VersionHandler` with changelog endpoints
3. Add changelog routes to version route group
4. Document entry creation process for developers
