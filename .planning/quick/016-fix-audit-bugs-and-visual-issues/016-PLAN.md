---
phase: quick
plan: 016
type: execute
wave: 1
depends_on: []
autonomous: true
files_modified:
  - frontend/src/routes/admin/audit-logs/+page.svelte
  - frontend/src/routes/admin/+page.svelte
  - frontend/src/routes/admin/settings/+page.svelte
  - frontend/src/routes/contacts/[id]/+page.svelte
  - frontend/src/routes/quotes/+page.svelte
  - frontend/src/routes/accounts/[id]/+page.svelte
  - frontend/src/routes/+layout.svelte
  - frontend/src/routes/admin/pdf-templates/+page.svelte
  - frontend/src/routes/admin/users/+page.svelte
  - frontend/src/routes/admin/data-quality/duplicate-rules/+page.svelte
  - frontend/src/routes/admin/import/+page.svelte
---

<objective>
Fix all code bugs and visual issues found during v3.0 UI audit.

Purpose: Clean up broken pages (500s, 404s), functional bugs (duplicate description, raw IDs), and visual readability issues (truncation, unclear buttons) across the admin and CRM interfaces.

Output: All 13 audit issues resolved. No 500s, no 404s, no visual truncation, all links clickable and showing display names.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
</execution_context>

<context>
Code repo: /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/
  - frontend/ (SvelteKit)
  - backend/ (Go/Fiber)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix broken pages (500s, 404s, redirect)</name>
  <files>
    frontend/src/routes/admin/audit-logs/+page.svelte
    frontend/src/routes/admin/+page.svelte
    frontend/src/routes/admin/settings/+page.svelte
    backend/internal/handler/audit.go
  </files>
  <action>
**Issue 1 - Audit Logs 500:** The audit logs page calls `get('/admin/audit-logs')` and `get('/admin/audit-logs/event-types')`. The backend handler calls `auditRepo.EnsureTableExists()` which queries the tenant DB. The 500 is most likely a tenant DB resolution issue or the table creation failing. Debug by:
1. Check the backend handler at `backend/internal/handler/audit.go` - the `getRepo(c)` method falls back to the master repo if no tenant DB is found. The master DB may not have the `audit_logs` table. The `EnsureTableExists` should handle this, but confirm the query is correct.
2. Add error handling in the frontend audit-logs page: wrap both `loadEventTypes()` and `loadLogs()` in proper try-catch that gracefully shows an empty state instead of breaking. The event-types endpoint at line 71 does NOT catch errors properly (it logs to console but the page may still break if the parent call chain fails).
3. The `GetEventTypes` handler (line 277 of audit.go) does NOT call `getRepo(c)` or `EnsureTableExists` - it returns static data. It should NOT 500 unless there's an auth/middleware issue. Check if the route registration uses `adminProtected` which requires OrgAdminRequired - the user must be an admin to see this page.

**Issue 2 - Screen Flows 404:** The admin card at line 184 links to `/admin/flows`, which already has a page at `frontend/src/routes/admin/flows/+page.svelte`. The card text says "Screen Flows" but the route exists. The 404 was likely a user navigation issue. Verify the link href is correct (`/admin/flows`). No change needed - the route exists.

**Issue 3 - Custom Pages 404:** The admin card at line 98 links to `/admin/pages`, which already has a page at `frontend/src/routes/admin/pages/+page.svelte`. Same as above - verify the link. No change needed.

**Issue 4 - Org Settings redirect to login:** The admin card at line 78 links to `/admin/settings`. The settings page at `frontend/src/routes/admin/settings/+page.svelte` calls `get('/settings')` (line 23) which hits the public `orgSettingsHandler.RegisterPublicRoutes(protected)` route. BUT the `/admin/settings` SvelteKit route is NOT behind the admin layout guard - it uses the admin layout from `frontend/src/routes/admin/+layout.svelte`. Check if the admin layout requires admin role and redirects non-admins. The actual API call goes to `/settings` (public route), so the API should work. The redirect might be caused by the `/admin/settings/+page.svelte` itself or the admin layout.

Fix: In `frontend/src/routes/admin/settings/+page.svelte`, the `loadNavigation()` call on line 5 imports navigation store. If `getNavigationTabs()` on line 18 triggers a fetch that fails (e.g., navigation not loaded yet), it could cause issues. Ensure the settings page handles the case where navigation tabs are not yet loaded by providing a fallback. Also check `frontend/src/routes/admin/+layout.svelte` for any auth guard that might redirect.

For all 500 errors, wrap API calls in better error handling with user-friendly messages.
  </action>
  <verify>
1. Navigate to /admin/audit-logs in browser - page loads without 500 (may show empty state if no audit logs exist)
2. Navigate to /admin/flows - page loads (verify the admin card link works)
3. Navigate to /admin/pages - page loads (verify the admin card link works)
4. Navigate to /admin/settings - page loads without redirecting to login
5. Check browser console for no 500 errors on any of these pages
  </verify>
  <done>All four admin pages load without errors. Audit logs shows either data or empty state. Settings page stays on page without redirect.</done>
</task>

<task type="auto">
  <name>Task 2: Fix functional bugs (Contact account link, duplicate description, Quotes actions)</name>
  <files>
    frontend/src/routes/contacts/[id]/+page.svelte
    frontend/src/routes/quotes/+page.svelte
    frontend/src/routes/accounts/[id]/+page.svelte
  </files>
  <action>
**Issue 5 - Contact detail Account shows raw ID:** The `getLinkInfo` function in contacts detail page (lines 179-209) should correctly resolve account links. The logic:
- `fieldName = 'accountId'`, `value = UUID`
- Looks for `contact['accountName']` for display text
- Falls back to `String(id)` if no display name

Debug: The most likely cause is that the field definition returned by `/entities/Contact/fields` doesn't have `type: 'link'` for the `accountId` field, OR `linkEntity` is missing/null. This would cause `getLinkInfo` to return null, and `formatFieldValue` would show the raw UUID.

Fix approach:
1. First verify the field definitions are correct by checking the API response
2. If the field metadata is correct, the issue might be in how `SectionRenderer` calls `renderLink`. Check that `renderLink` prop is being passed correctly.
3. As a defensive fix, update the `getLinkInfo` function to also handle the case where field type might not be 'link' but the field name pattern matches (ends with 'Id' and has a corresponding 'Name' field). Add a fallback: if `fieldName` ends with 'Id' and `contact[fieldName.replace(/Id$/, 'Name')]` exists, create a link even without the field type being 'link'.

**Issue 6 - Contact detail Description appears twice:** The contact detail page has:
- Layout-rendered sections via `SectionRenderer` (which includes description if it's in the layout)
- A standalone description block at lines 307-312: `{#if contact.description} <div>...Description...</div> {/if}`

Fix: Remove the standalone description section (lines 307-312). The description field is already included in the provisioned layout (it's in the field definitions and rendered by SectionRenderer). If description is NOT in the layout, SectionRenderer won't show it anyway, and the standalone block becomes redundant/duplicative. Replace with a conditional that checks if `description` field appears in any visible section:

```svelte
<!-- Description (only if NOT already in layout sections) -->
{#if contact.description && !visibleSections().some(s => s.fields.some(f => f.name === 'description'))}
    <div class="bg-white shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">Description</h2>
        <p class="text-sm text-gray-900 whitespace-pre-wrap">{contact.description}</p>
    </div>
{/if}
```

**Issue 7 - Quotes list rows not clickable, only Delete action:** Looking at the quotes list page, line 176 already has `onclick={() => goto(`/quotes/${quote.id}`)}` on the `<tr>`. So rows ARE clickable. But the "only Delete" action issue is at lines 197-204 - the Actions column only has a Delete button.

Fix: Add Edit and View buttons to the Actions column in the quotes list table:

```svelte
<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
    <a href="/quotes/{quote.id}" onclick={(e) => e.stopPropagation()} class="text-blue-600 hover:text-blue-900 mr-3">View</a>
    <a href="/quotes/{quote.id}/edit" onclick={(e) => e.stopPropagation()} class="text-gray-600 hover:text-gray-900 mr-3">Edit</a>
    <button onclick={(e) => { e.stopPropagation(); deleteQuote(quote.id); }} class="text-red-600 hover:text-red-900">Delete</button>
</td>
```

**Issue 8 - Account detail 404 console errors:** The account detail page makes parallel API calls at lines 104-109. Two of these may 404:
- `get('/entities/Account/bearings')` - if bearings feature isn't set up for Account
- `get('/entities/Account/def')` - if entity def endpoint has issues

Both already have `.catch(() => [])` or `.catch(() => null)`, so they won't crash the page. The 404s are just console noise. Fix by suppressing the console error for expected 404s - these `.catch()` blocks should also catch the error silently without the `api` function logging to console.

Alternatively, check if the `api` utility logs errors before they're caught. If it does, the 404s are benign but noisy. We can leave this as-is since it's cosmetic console noise.
  </action>
  <verify>
1. Navigate to a contact detail page - Account field shows "Acme Corporation" (or similar name) as a clickable blue link, NOT a raw UUID
2. Contact detail page shows Description only once (in the layout section, not duplicated below)
3. Quotes list page shows View, Edit, Delete action buttons on each row
4. Clicking View goes to /quotes/{id}, clicking Edit goes to /quotes/{id}/edit
5. Account detail page loads without visible errors (console 404s for missing bearings/def are acceptable)
  </verify>
  <done>Contact detail shows account as clickable link with display name. Description appears exactly once. Quotes list has View/Edit/Delete actions per row.</done>
</task>

<task type="auto">
  <name>Task 3: Fix visual/readability issues (truncation, button styling, column widths)</name>
  <files>
    frontend/src/routes/+layout.svelte
    frontend/src/routes/admin/pdf-templates/+page.svelte
    frontend/src/routes/admin/users/+page.svelte
    frontend/src/routes/admin/data-quality/duplicate-rules/+page.svelte
    frontend/src/routes/admin/import/+page.svelte
  </files>
  <action>
**Issue 9 - Org name wraps to 3 lines in navbar:** In `+layout.svelte`, the org name is displayed in two places:
- Line 216 (org switcher button): `<span class="font-medium">{auth.currentOrg?.orgName}</span>`
- Line 244 (single org): `<span class="text-sm text-gray-600">{auth.currentOrg.orgName}</span>`

Fix: Add `whitespace-nowrap truncate max-w-[200px]` to both spans to prevent wrapping and truncate long names:
- Line 216: `<span class="font-medium whitespace-nowrap truncate max-w-[200px]">{auth.currentOrg?.orgName}</span>`
- Line 244: `<span class="text-sm text-gray-600 whitespace-nowrap truncate max-w-[200px]">{auth.currentOrg.orgName}</span>`

**Issue 10 - PDF Template names truncated ("Standa..."):** In pdf-templates page, line 132: `<h3 class="text-lg font-medium text-gray-900 truncate">{tpl.name}</h3>`. The card grid uses `lg:grid-cols-3` which makes each card too narrow for template names.

Fix: Remove `truncate` from the h3 class. Template names should wrap to 2 lines if needed rather than being truncated. Also add `title={tpl.name}` attribute for tooltip on hover:
```svelte
<h3 class="text-lg font-medium text-gray-900" title={tpl.name}>{tpl.name}</h3>
```

**Issue 11 - User Management "JOINED" date truncated:** In users page, the Joined column header at line 334 is just "Joined" with standard column width. The date values at line 420 use `whitespace-nowrap`. The truncation is caused by the table not having enough width for all columns.

Fix: Add `min-w-[100px]` to the Joined th element (line 334) to ensure minimum width:
```svelte
<th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider min-w-[100px]">
    Joined
</th>
```
Also add `min-w-[100px]` to the Last Login column (line 337).

**Issue 12 - Data Quality "PRIORITY" column truncated:** In duplicate-rules page, the Priority column header at line 342 shows "PRIOI..." because the table has too many columns (7) and the Priority column gets squeezed.

Fix: The "Priority" text in the th at line 342 is short enough. The issue is likely the overall table being constrained. Add `min-w-[80px]` to the Priority th:
```svelte
<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider min-w-[80px]">
    Priority
</th>
```
Also reduce padding on some columns. Change `px-6` to `px-4` on the less important columns (Status, Priority, Threshold, Fields, Actions) to save space.

**Issue 13 - Import Data "Start Import" button washed out:** In import page, the button at lines 87-91:
```svelte
<button ... disabled={!selectedEntity}
    class="w-full bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed">
    Start Import
</button>
```
The `disabled:opacity-50` makes the button look too faded. When no entity is selected, it's unclear if it's a real button.

Fix: Change `disabled:opacity-50` to `disabled:bg-gray-300 disabled:text-gray-500` for clearer disabled state. This makes it obviously a disabled button rather than a washed-out purple button:
```svelte
class="w-full bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:bg-gray-300 disabled:text-gray-500 disabled:cursor-not-allowed"
```
  </action>
  <verify>
1. Navbar org name displays on single line, truncated with ellipsis if too long
2. PDF Templates page shows full template names (or wraps to 2 lines), no "Standa..." truncation
3. User Management table shows full "Joined" and "Last Login" date columns without truncation
4. Data Quality duplicate rules table shows "Priority" column header fully visible
5. Import Data "Start Import" button shows as clearly disabled (gray) when no entity selected, and as blue when entity is selected
  </verify>
  <done>All 5 visual issues fixed: navbar org name doesn't wrap, PDF template names visible, table columns have adequate width, import button has clear disabled/enabled states.</done>
</task>

</tasks>

<verification>
After all 3 tasks complete:
1. Navigate through all admin pages linked from /admin - no 500s or unexpected 404s
2. Contact detail page shows account as clickable link and description only once
3. Quotes list has View/Edit/Delete per row
4. All visual truncation issues resolved
5. Import button has clear enabled/disabled visual distinction
</verification>

<success_criteria>
- Zero 500 errors on admin pages
- Zero unexpected 404s from admin cards
- Contact account field renders as "Account Name" clickable link, not raw UUID
- Description field appears exactly once on contact detail
- Quotes list has 3 action buttons (View, Edit, Delete) per row
- No text truncation on table headers or template names
- Navbar org name stays on one line
- Import button clearly distinguishes enabled vs disabled states
</success_criteria>

<output>
After completion, update progress.txt with all changes made and verify in browser.
</output>
