# Quick Task 002: Configurable Homepage

## Goal
Allow each org to set a custom homepage/landing page instead of always showing the default welcome page.

## Approach
Extend the navigation system to include a `homePage` setting that tells the frontend where to redirect from `/`.

## Tasks

### Task 1: Add org_settings table and homepage field
**File:** Create migration `backend/internal/migrations/XXX_create_org_settings.sql`

Create an `org_settings` table to store per-org settings:
```sql
CREATE TABLE IF NOT EXISTS org_settings (
    org_id TEXT PRIMARY KEY,
    home_page TEXT DEFAULT '/',
    settings_json TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### Task 2: Add OrgSettings entity and repo
**Files:**
- `backend/internal/entity/org_settings.go`
- `backend/internal/repo/org_settings.go`

Create entity:
```go
type OrgSettings struct {
    OrgID        string `json:"orgId" db:"org_id"`
    HomePage     string `json:"homePage" db:"home_page"`
    SettingsJSON string `json:"-" db:"settings_json"`
}
```

Create repo with:
- `Get(ctx, orgID)` - get settings (create default if not exists)
- `UpdateHomePage(ctx, orgID, homePage)` - update homepage setting

### Task 3: Add API endpoints
**File:** `backend/internal/handler/org_settings.go`

Endpoints:
- `GET /api/v1/settings` - Get org settings (returns homePage)
- `PUT /api/v1/admin/settings/homepage` - Update homepage (admin only)

Wire into `main.go`.

### Task 4: Update frontend to use configurable homepage
**Files:**
- `frontend/src/routes/+page.svelte` - Redirect to configured homepage
- `frontend/src/lib/stores/navigation.svelte.ts` - Add homePage to state

When navigation loads, also fetch org settings. The homepage component checks if there's a configured homepage and redirects.

### Task 5: Add UI to set homepage in admin
**File:** `frontend/src/routes/admin/+page.svelte` (or new settings page)

Add a dropdown to select homepage from available navigation tabs.

## Acceptance Criteria
- [ ] Org settings table created
- [ ] API returns homepage setting
- [ ] Admin can set homepage via UI
- [ ] Frontend redirects from `/` to configured homepage
- [ ] Default is `/` (shows welcome page) for backwards compatibility
