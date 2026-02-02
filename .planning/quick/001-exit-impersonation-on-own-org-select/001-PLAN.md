# Quick Task 001: Exit Impersonation on Own Org Select

## Context
When a platform admin is impersonating an organization (e.g., "Lone Wulf Recruiting"), the org switcher dropdown shows both the impersonated org and the admin's own org(s) (e.g., "Quantico Operations"). Currently, clicking on the admin's own org calls `switchOrg()` which doesn't exit impersonation mode, leaving the user stuck.

## Goal
When in impersonation mode, selecting any org other than the currently impersonated org should exit impersonation and return the user to their normal session.

## Implementation

### Task 1: Update handleSwitchOrg in +layout.svelte

**File:** `FastCRM/fastcrm/frontend/src/routes/+layout.svelte`

**Change:** Modify `handleSwitchOrg` function to check impersonation status:

```svelte
// Handle org switch
async function handleSwitchOrg(orgId: string) {
    showOrgSwitcher = false;

    // If in impersonation mode and switching away from the impersonated org,
    // exit impersonation instead of switching
    if (auth.isImpersonation && orgId !== auth.currentOrg?.orgId) {
        await handleStopImpersonation();
        return;
    }

    await switchOrg({ orgId });
}
```

**Rationale:**
- When impersonating, `auth.currentOrg` is the impersonated org
- If user clicks on a different org (their own), we exit impersonation
- After `stopImpersonation()`, the backend returns the admin's normal session with their default org
- The redirect to `/admin/platform` in `handleStopImpersonation` ensures clean state

## Acceptance Criteria
- [ ] Platform admin can impersonate an org
- [ ] Org switcher shows both impersonated org and admin's own org(s)
- [ ] Clicking on admin's own org exits impersonation mode
- [ ] User is redirected to `/admin/platform` after exiting impersonation
- [ ] Impersonation banner disappears after exiting
