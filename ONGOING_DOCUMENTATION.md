# Ongoing Documentation

This document tracks ongoing fixes, known issues, and implementation notes for the FastCRM project.

---

## 2026-01-14: List Layout Configuration Not Reflecting in List View

### Issue
The "Prospect - List Layout" admin configuration page showed "0 enabled fields", yet the actual list view still displayed data columns. The layout configuration was not being respected.

### Root Cause
1. The frontend list view component had a fallback mechanism that auto-generated 5 columns when `layoutFields` was empty
2. The backend returned `"layoutData": "[]"` both when:
   - No layout had ever been saved
   - A layout was explicitly saved with 0 fields
3. This made it impossible to distinguish between "never configured" and "configured as empty"

### Solution
**Backend Change** (`FastCRM/fastcrm/backend/internal/handler/admin.go:416-434`):
- Added an `exists` boolean flag to the layout API response
- `exists: false` when no layout record exists in the database
- `exists: true` when a layout has been explicitly saved (even if empty)

**Frontend Change** (`FastCRM/fastcrm/frontend/src/routes/[entity=customentity]/+page.svelte`):
- Added `layoutExists` state variable
- Changed `displayFields` derived logic to check `layoutExists` instead of `layoutFields.length > 0`
- Fallback (auto-generate 5 columns) only applies when no layout has ever been configured
- When layout exists but is empty, respects that configuration (shows no columns)

### Files Modified
- `FastCRM/fastcrm/backend/internal/handler/admin.go`
- `FastCRM/fastcrm/frontend/src/routes/[entity=customentity]/+page.svelte`

### Behavior After Fix
- If layout has never been saved: Uses fallback (first 5 fields)
- If layout is saved with specific fields: Shows those fields
- If layout is saved with 0 fields: Shows no columns (respects configuration)

---

## 2026-01-14: Rollup Field Disappeared After Restart

### Issue
A rollup field from Contact to Account that counts the number of contacts disappeared after a server restart.

### Status
**RESOLVED**

### Root Cause
Multiple database files existed in the project:
- `fastcrm/fastcrm.db` (278KB) - used by `services.sh`
- `fastcrm/backend/fastcrm.db` (286KB) - used when running backend directly from `backend/` directory

The rollup field `test_rollup` was created while the backend was using `backend/fastcrm.db`, but when the service was restarted via `services.sh`, it used `fastcrm/fastcrm.db` which didn't have the rollup field.

### Solution
Consolidated databases by copying `backend/fastcrm.db` to `fastcrm/fastcrm.db`:
```bash
cp fastcrm/fastcrm.db fastcrm/fastcrm.db.backup
cp fastcrm/backend/fastcrm.db fastcrm/fastcrm.db
```

### Prevention
Always use `services.sh` to start/stop the backend, or ensure `DATABASE_PATH` environment variable is set consistently when running the backend manually.
