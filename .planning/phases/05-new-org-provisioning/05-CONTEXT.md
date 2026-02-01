# Phase 5: New Org Provisioning - Context

**Gathered:** 2026-02-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Ensure new organizations are created with the current platform version and receive all standard metadata that version includes. This integrates the existing provisioning flow with the versioning system built in Phases 1-4.

</domain>

<decisions>
## Implementation Decisions

### Version assignment
- Stamp org with current platform version at creation time (not after setup or first login)
- Read platform version from `platform_versions` table via `VersionRepo.GetPlatformVersion()`
- Set `current_version` field on org record during `CreateOrganization`

### Metadata baseline
- Use existing `ProvisioningService.ProvisionDefaultMetadata()` — already provisions entities, fields, layouts, navigation
- No changes to what gets provisioned (Contact, Account, Task, Quote, QuoteLineItem with all standard fields and layouts)
- Provisioning continues to use INSERT OR REPLACE (idempotent, re-runnable)

### Failure handling
- Continue using `org.ProvisioningError` field to capture errors
- If provisioning fails, org still gets created but without metadata
- "Repair Metadata" button in Admin panel already handles re-provisioning
- No new failure modes — version assignment is a single field update

### Claude's Discretion
- Whether to log version assignment
- Error message wording if platform version lookup fails
- Whether to fall back to v0.1.0 if no platform version exists (current behavior in VersionRepo)

</decisions>

<specifics>
## Specific Ideas

- Integration point is `AuthService.CreateOrganization()` in `auth.go:815`
- VersionRepo already exists and is used by VersionHandler
- Just need to wire VersionRepo into AuthService and call GetPlatformVersion during org creation

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-new-org-provisioning*
*Context gathered: 2026-02-01*
