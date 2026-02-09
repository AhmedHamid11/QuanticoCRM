---
quick_plan: 013
title: Debug Guardare-Operations Missing Navigation Options
focus: database-debugging
atomic: true
status: ready
---

<objective>
Debug and fix why Guardare-Operations org (00DKGHZ44SC000F1G0) is missing navigation tab options in admin panel, despite navigation_options and navigation_tabs existing in the system.

**Goal:** Identify root cause of provisioning failure and restore navigation for the org.

**Output:** Working navigation tabs in Guardare-Operations admin panel
</objective>

<execution_context>
Database: `/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend`
Org ID: `00DKGHZ44SC000F1G0` (Guardare-Operations)
</execution_context>

<context>
Progress shows navigation_tabs table was created via ensureMetadataTables() in commit 898358b.
However, admin panel still showing no navigation options for Guardare-Operations.

Possible causes:
1. navigation_tabs table exists but has no rows for this org
2. Provisioning ran but didn't insert navigation tab records
3. Query filtering by org_id is missing/broken
4. Table schema mismatch between master and tenant DB
</context>

<tasks>

<task type="auto">
  <name>Query Navigation Options and Debug Database State</name>
  <files>
    backend/internal/repo/navigation.go
    backend/internal/service/provisioning.go
  </files>
  <action>
1. Connect to Guardare-Operations tenant database (use Railway database connector or local inspection)
2. Query navigation_tabs table for org_id = '00DKGHZ44SC000F1G0':
   ```sql
   SELECT id, org_id, label, href, icon, entity_name, sort_order, is_visible FROM navigation_tabs
   WHERE org_id = '00DKGHZ44SC000F1G0'
   ORDER BY sort_order;
   ```
3. If rows exist: Debug why they're not returned by GET /api/v1/navigation endpoint
   - Check ListByOrg() method in navigation.go for filtering issues
   - Verify org_id from context matches database records
4. If no rows exist: Check provisioning logs
   - Query database logs for migration 002 execution
   - Confirm ensureMetadataTables() created the table
   - Check if navigation seed data was created (see provisionMetadata in provisioning.go)
5. Document findings in task action
  </action>
  <verify>
- Run SQL query above and capture row count
- If rows exist: Test GET /api/v1/navigation endpoint to confirm it returns them
- If no rows: Check provisioning.go to confirm navigation seed step exists (around line 150-200)
- Confirm table schema matches with SHOW COLUMNS FROM navigation_tabs
  </verify>
  <done>
Database state documented. If rows exist, issue is in ListByOrg() filtering. If no rows, issue is provisioning didn't seed navigation data.
  </done>
</task>

<task type="auto">
  <name>Fix and Re-provision Navigation Data if Missing</name>
  <files>
    backend/internal/service/provisioning.go
  </files>
  <action>
1. Based on findings from Task 1:

**IF NO ROWS EXIST IN navigation_tabs:**
- Open provisioning.go and search for navigation seeding code (should be in provisionMetadata())
- Confirm navigation tabs are created for default tabs: ["Contacts", "Accounts", "Admin"]
- Each tab needs: label, href, icon, entity_name, sort_order, is_visible
- Example seed data:
  ```go
  navigationTabs := []map[string]interface{}{
      {"label": "Contacts", "href": "/contacts", "icon": "users", "entity_name": "Contact", "sort_order": 1, "is_visible": 1, "is_system": 1},
      {"label": "Accounts", "href": "/accounts", "icon": "building", "entity_name": "Account", "sort_order": 2, "is_visible": 1, "is_system": 1},
      {"label": "Admin", "href": "/admin", "icon": "cog", "entity_name": "", "sort_order": 999, "is_visible": 1, "is_system": 1},
  }
  ```
- If seeding code is missing, add it to provisionMetadata()
- Commit change with message: "fix(provisioning): ensure navigation tabs are seeded during provisioning"

**IF ROWS EXIST BUT ENDPOINT NOT RETURNING THEM:**
- Open navigation.go and check ListByOrg() method
- Verify it filters by org_id correctly: WHERE org_id = ?
- Check that context org_id is being extracted properly in handler
- Add logging if needed to debug filtering issue
- Commit change with message: "fix(navigation): fix org_id filtering in ListByOrg query"

2. Deploy fix to Railway
3. Trigger re-provisioning via "Repair Metadata" button in Guardare-Operations admin panel
  </action>
  <verify>
1. After fix deployed, navigate to Guardare-Operations admin panel
2. Click "Repair Metadata" button
3. Wait for success message
4. Check admin panel - navigation tabs should now appear (Contacts, Accounts, Admin)
5. Click on each tab to verify they load correctly
6. Check browser console for any 500 errors
7. Query database again to confirm rows exist:
   ```sql
   SELECT COUNT(*) as count FROM navigation_tabs WHERE org_id = '00DKGHZ44SC000F1G0';
   ```
  </verify>
  <done>
Navigation tabs seeded and visible in Guardare-Operations admin panel. All tabs clickable. No 500 errors in console.
  </done>
</task>

</tasks>

<verification>
After both tasks complete:
1. Guardare-Operations admin panel shows navigation tabs
2. Clicking tabs navigates to correct pages
3. Database query confirms rows exist
4. No 500 errors in production logs
5. Progress file updated with root cause and fix
</verification>

<success_criteria>
Guardare-Operations navigation tabs visible and functional:
- [ ] Navigation tabs appear in admin panel sidebar
- [ ] Tabs include: Contacts, Accounts, Admin (or custom tabs if configured)
- [ ] Clicking tabs navigates to correct pages
- [ ] Database query shows rows in navigation_tabs for org 00DKGHZ44SC000F1G0
- [ ] GET /api/v1/navigation endpoint returns data without errors
- [ ] No 500 errors in production logs
</success_criteria>

<notes>
- Previous fix in commit 898358b added navigation_tabs table creation, but may not have added seeding
- Provisioning service is in backend/internal/service/provisioning.go
- Navigation handler is in backend/internal/handler/navigation.go (or similar)
- Need to verify schema matches between master and tenant DBs
- If issue persists after re-provisioning, consider running migration on tenant directly
</notes>
