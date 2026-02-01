# Summary: 05-02 Fix Register() Version Lookup

**Status:** Complete
**Completed:** 2026-02-01
**Commit:** 1b78f5a fix(05-02): add version lookup to Register() function

## What Was Done

Added platform version lookup to the `Register()` function in `auth.go` so that new organizations created during user registration get stamped with the current platform version.

## Changes Made

**File:** `backend/internal/service/auth.go`

1. Added version lookup block before org creation (lines 125-136):
   ```go
   platformVersion := "v0.1.0"
   if s.versionRepo != nil {
       pv, err := s.versionRepo.GetPlatformVersion(ctx)
       if err != nil {
           log.Printf("[Register] Warning: failed to get platform version: %v", err)
       } else {
           platformVersion = pv.Version
       }
   }
   ```

2. Updated both `CreateOrganization` calls to include `CurrentVersion: platformVersion`:
   - Tenant provisioning path (line 142)
   - Legacy path (line 163)

## Verification

- [x] Backend compiles without errors
- [x] `versionRepo.GetPlatformVersion` called in Register (line 128)
- [x] Both org creation paths pass CurrentVersion
- [x] Pattern matches CreateOrganization service method (lines 859)

## Gap Closed

This closes the gap where `Register()` was calling `repo.CreateOrganization` directly, bypassing the version logic in `CreateOrganization` service method. Now both registration and manual org creation flows stamp new orgs with the platform version.

---
*Phase 05 plan 02 - gap closure*
