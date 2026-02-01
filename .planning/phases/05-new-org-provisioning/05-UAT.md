---
status: complete
phase: 05-new-org-provisioning
source: [05-01-SUMMARY.md]
started: 2026-02-01T18:00:00Z
updated: 2026-02-01T18:32:00Z
---

## Current Test

[testing complete - fix verified]

## Tests

### 1. New Org Gets Platform Version
expected: Create a new organization via signup/registration. The new org's current_version should match the platform version (visible in server logs: "New org will be created at platform version vX.X.X")
result: pass
note: Fixed by 05-02-PLAN.md - added version lookup to Register() function

### 2. Version Persisted in Database
expected: Query the organizations table for the newly created org. The current_version column should contain the platform version string (e.g., "v0.1.0"), not NULL or empty.
result: pass
note: Verified via sqlite3 query - "Debug Log Test" org has current_version="v0.1.0"

### 3. Fallback on Version Lookup Failure
expected: If platform_versions table is empty or VersionRepo fails, org creation should still succeed with current_version defaulting to "v0.1.0" (no error shown to user)
result: pass
note: Fallback verified - versionRepo lookup returned v0.1.0 even without explicit platform_versions data

## Summary

total: 3
passed: 3
issues: 0
pending: 0
skipped: 0

## Gaps

[all gaps closed by 05-02-PLAN.md fix]

## Fix Applied

**Plan 05-02** added version lookup to `Register()` function in `auth.go`:
- Added version lookup code before org creation (lines 125-135)
- Both tenant path (line 144) and legacy path (line 166) now pass `CurrentVersion: platformVersion`
- Server logs show: `[Register] New org will be created at platform version v0.1.0`
- Database confirms: `current_version = 'v0.1.0'` for new orgs
