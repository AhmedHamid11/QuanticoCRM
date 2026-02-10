package changelog

import (
	"sort"

	"golang.org/x/mod/semver"
)

// Category represents a changelog category following Keep a Changelog convention
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

// VersionChangelog represents all changes for a specific version
type VersionChangelog struct {
	Version string  `json:"version"`
	Entries []Entry `json:"entries"`
}

// Entries contains changelog data for all platform versions
// Version keys MUST have v prefix (e.g., "v0.1.0") to match semver convention
var Entries = map[string][]Entry{
	"v0.2.0": {
		{Category: CategoryAdded, Description: "Stream field type for journal/Twitter-style timestamped entries"},
		{Category: CategoryAdded, Description: "Inline entry submission on detail pages without navigating to edit mode"},
		{Category: CategoryAdded, Description: "Delete entries from stream field logs with confirmation"},
		{Category: CategoryAdded, Description: "Keyboard shortcut (Ctrl+Enter) for quick stream entry submission"},
	},
	"v0.1.0": {
		{Category: CategoryAdded, Description: "Initial platform version with core CRM entities"},
		{Category: CategoryAdded, Description: "Contact, Account, Task, Quote entity support"},
		{Category: CategoryAdded, Description: "Multi-tenant database architecture"},
	},
}

// GetSortedVersions returns all version keys sorted in descending order (newest first)
func GetSortedVersions() []string {
	versions := make([]string, 0, len(Entries))
	for v := range Entries {
		versions = append(versions, v)
	}

	// Sort descending (newest first) using semver comparison
	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(versions[i], versions[j]) > 0
	})

	return versions
}

// GetEntriesForVersion returns changelog entries for a specific version
// Returns the entries and a boolean indicating if the version was found
func GetEntriesForVersion(version string) ([]Entry, bool) {
	entries, found := Entries[version]
	return entries, found
}

// GetEntriesBetweenVersions returns changelogs for versions in range (fromVersion, toVersion]
// - Excludes fromVersion (exclusive)
// - Includes toVersion (inclusive)
// Results are ordered newest to oldest
func GetEntriesBetweenVersions(fromVersion, toVersion string) []VersionChangelog {
	result := []VersionChangelog{}

	for _, version := range GetSortedVersions() {
		// Include if: version > fromVersion AND version <= toVersion
		if semver.Compare(version, fromVersion) > 0 && semver.Compare(version, toVersion) <= 0 {
			entries, _ := GetEntriesForVersion(version)
			result = append(result, VersionChangelog{
				Version: version,
				Entries: entries,
			})
		}
	}

	return result
}
