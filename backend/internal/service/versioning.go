package service

import (
	"strings"

	"golang.org/x/mod/semver"
)

// VersionService handles platform version tracking and comparison
type VersionService struct{}

// NewVersionService creates a new VersionService
func NewVersionService() *VersionService {
	return &VersionService{}
}

// NeedsUpdate returns true if orgVersion is older than platformVersion
func (s *VersionService) NeedsUpdate(orgVersion, platformVersion string) bool {
	// Normalize both versions to ensure v prefix
	orgVersion = s.Normalize(orgVersion)
	platformVersion = s.Normalize(platformVersion)

	if !semver.IsValid(orgVersion) || !semver.IsValid(platformVersion) {
		return false
	}
	return semver.Compare(orgVersion, platformVersion) < 0
}

// Normalize ensures version has "v" prefix and canonical form
// Returns "v0.1.0" for empty string (default version)
func (s *VersionService) Normalize(version string) string {
	if version == "" {
		return "v0.1.0"
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	// Canonical ensures X.Y.Z format (v1 -> v1.0.0)
	canonical := semver.Canonical(version)
	if canonical == "" {
		// Invalid version, return as-is with v prefix
		return version
	}
	return canonical
}

// IsValid checks if a version string is valid semver
func (s *VersionService) IsValid(version string) bool {
	return semver.IsValid(s.Normalize(version))
}

// Compare returns:
//   -1 if v1 < v2
//    0 if v1 == v2
//   +1 if v1 > v2
func (s *VersionService) Compare(v1, v2 string) int {
	return semver.Compare(s.Normalize(v1), s.Normalize(v2))
}
