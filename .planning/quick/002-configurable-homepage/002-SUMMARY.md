# Quick Task 002 Summary: Configurable Homepage

## Completed

Added ability for each org to configure a custom homepage that users are redirected to when visiting `/`.

## Changes Made

### Backend
- **Migration:** `044_create_org_settings.sql` - Creates `org_settings` table with `home_page` field
- **Entity:** `org_settings.go` - OrgSettings struct with OrgID and HomePage
- **Repo:** `org_settings.go` - Get (with auto-create) and UpdateHomePage
- **Handler:** `org_settings.go` - GET /settings and PUT /admin/settings/homepage
- **main.go:** Wired up OrgSettingsHandler routes

### Frontend
- **navigation.svelte.ts:** Extended to load org settings alongside nav tabs
- **+page.svelte:** Added effect to redirect to configured homepage
- **admin/+page.svelte:** Added "Org Settings" link
- **admin/settings/+page.svelte:** New settings page with homepage dropdown

## How It Works
1. Admin goes to Setup > Org Settings
2. Selects homepage from dropdown (Welcome Page or any nav tab)
3. Saves - stored in org_settings table
4. When user visits `/`, frontend checks orgSettings.homePage
5. If not `/`, redirects to configured page

## Testing
Verified in browser:
1. Set homepage to `/contacts`
2. Navigated to `/`
3. Redirected to `/contacts` ✓
