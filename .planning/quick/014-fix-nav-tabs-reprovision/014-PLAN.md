---
phase: quick
plan: 014
type: execute
wave: 1
depends_on: []
files_modified:
  - backend/internal/service/provisioning.go
  - frontend/src/lib/stores/navigation.svelte.ts
autonomous: true

must_haves:
  truths:
    - "After Repair Metadata, navigation tabs appear in the sidebar"
    - "Reprovision fixes stale/broken nav tab data (invisible tabs become visible)"
    - "Orgs with empty nav tabs API response still show default navigation"
  artifacts:
    - path: "backend/internal/service/provisioning.go"
      provides: "Navigation tabs table creation and upsert logic"
      contains: "CREATE TABLE IF NOT EXISTS navigation_tabs"
    - path: "frontend/src/lib/stores/navigation.svelte.ts"
      provides: "Fallback when API returns empty array"
  key_links:
    - from: "ensureMetadataTables"
      to: "navigation_tabs table"
      via: "CREATE TABLE IF NOT EXISTS before early return"
      pattern: "CREATE TABLE IF NOT EXISTS navigation_tabs"
    - from: "createNavTabWithError"
      to: "navigation_tabs"
      via: "INSERT OR REPLACE"
      pattern: "INSERT OR REPLACE INTO navigation_tabs"
    - from: "navigation.svelte.ts"
      to: "fallback defaults"
      via: "empty array check"
      pattern: "length.*===.*0"
---

<objective>
Fix three bugs preventing navigation tabs from appearing after Repair Metadata (reprovision) on production orgs.

Purpose: Navigation tabs are completely absent for orgs where provisioning partially failed or had stale data. This makes the app unusable since users can't navigate to any entity pages.

Output: Backend fixes ensure `navigation_tabs` table is always created during reprovision and that stale tab data gets replaced. Frontend fix ensures empty API responses trigger fallback defaults.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@FastCRM/fastcrm/backend/internal/service/provisioning.go
@FastCRM/fastcrm/frontend/src/lib/stores/navigation.svelte.ts
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix backend provisioning bugs (table creation + upsert)</name>
  <files>FastCRM/fastcrm/backend/internal/service/provisioning.go</files>
  <action>
  Two fixes in provisioning.go:

  **Fix 1: Ensure navigation_tabs table exists during reprovision**

  In `ensureMetadataTables()` (starts at line 94), the early return at line 113 skips navigation_tabs creation. Add a `CREATE TABLE IF NOT EXISTS navigation_tabs` statement BEFORE the early return. Specifically:

  After line 101 (`tableExists := err == nil`) and before the `if tableExists {` block at line 102, add:

  ```go
  // Always ensure navigation_tabs exists, regardless of entity_defs status
  // This fixes the case where entity_defs exists but navigation_tabs doesn't
  _, _ = s.db.ExecContext(ctx, `
      CREATE TABLE IF NOT EXISTS navigation_tabs (
          id TEXT PRIMARY KEY,
          org_id TEXT NOT NULL,
          label TEXT NOT NULL,
          href TEXT NOT NULL,
          icon TEXT DEFAULT '',
          entity_name TEXT,
          sort_order INTEGER DEFAULT 0,
          is_visible INTEGER DEFAULT 1,
          is_system INTEGER DEFAULT 0,
          created_at TEXT DEFAULT CURRENT_TIMESTAMP,
          modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
          UNIQUE(org_id, href)
      )
  `)
  ```

  This ensures the table exists even when the early return fires at line 113.

  **Fix 2: Change INSERT OR IGNORE to INSERT OR REPLACE in createNavTabWithError**

  In `createNavTabWithError()` at line 1034, change:
  - `INSERT OR IGNORE INTO navigation_tabs` to `INSERT OR REPLACE INTO navigation_tabs`
  - Update the comment on line 1033 to: `// Use INSERT OR REPLACE to fix stale/broken tab data during reprovision`

  This matches the pattern already used by `createLayout` (which uses INSERT OR REPLACE at line 1053 area) and ensures reprovision actually fixes broken tab data (e.g., tabs with is_visible=0).
  </action>
  <verify>
  Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` to confirm no compilation errors.

  Grep to confirm both fixes:
  - `grep -n "CREATE TABLE IF NOT EXISTS navigation_tabs" internal/service/provisioning.go` should show the new early creation AND the existing one
  - `grep -n "INSERT OR REPLACE INTO navigation_tabs" internal/service/provisioning.go` should show the updated upsert
  - `grep -n "INSERT OR IGNORE INTO navigation_tabs" internal/service/provisioning.go` should return NO results
  </verify>
  <done>
  - navigation_tabs table is created in ensureMetadataTables BEFORE the early return path
  - createNavTabWithError uses INSERT OR REPLACE instead of INSERT OR IGNORE
  - Backend compiles without errors
  </done>
</task>

<task type="auto">
  <name>Task 2: Fix frontend fallback for empty nav tabs response</name>
  <files>FastCRM/fastcrm/frontend/src/lib/stores/navigation.svelte.ts</files>
  <action>
  In `loadNavigation()` (line 26), the success path at line 35 (`tabs = navResult`) assigns the API result directly. If the API returns an empty array `[]` (success, not error), no tabs render and the fallback defaults in the catch block never trigger.

  After line 35 (`tabs = navResult;`), add a check:

  ```typescript
  // If API returns empty array, use fallback defaults
  // This handles orgs where navigation_tabs table exists but has no rows
  if (navResult.length === 0) {
      tabs = [
          { id: 'nav_contacts', label: 'Contacts', href: '/contacts', icon: 'users', entityName: 'Contact', sortOrder: 1, isVisible: true, isSystem: true },
          { id: 'nav_accounts', label: 'Accounts', href: '/accounts', icon: 'building', entityName: 'Account', sortOrder: 2, isVisible: true, isSystem: true },
          { id: 'nav_admin', label: 'Admin', href: '/admin', icon: 'settings', sortOrder: 100, isVisible: true, isSystem: true }
      ];
  }
  ```

  This reuses the exact same fallback defaults from the catch block (lines 40-44). The duplication is intentional -- these are the same hardcoded defaults that should appear whenever the API can't provide real tabs.
  </action>
  <verify>
  Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5` to confirm no TypeScript errors.

  Grep to confirm: `grep -n "navResult.length" src/lib/stores/navigation.svelte.ts` should show the new empty-check.
  </verify>
  <done>
  - Empty array from navigation API triggers fallback defaults
  - TypeScript compiles without errors
  - Fallback tabs match the existing defaults in catch block (Contacts, Accounts, Admin)
  </done>
</task>

</tasks>

<verification>
1. `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` -- backend compiles
2. `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend && npx svelte-check --tsconfig ./tsconfig.json` -- frontend compiles
3. Confirm no remaining `INSERT OR IGNORE INTO navigation_tabs` in provisioning.go
4. Confirm `CREATE TABLE IF NOT EXISTS navigation_tabs` appears before the early return at line ~113
5. Confirm `navigation.svelte.ts` checks for empty array before assigning tabs
</verification>

<success_criteria>
- Backend: ensureMetadataTables creates navigation_tabs table even on the early-return path (entity_defs exists)
- Backend: createNavTabWithError uses INSERT OR REPLACE to fix stale tab data during reprovision
- Frontend: Empty nav tabs API response triggers fallback defaults instead of showing nothing
- All code compiles without errors
</success_criteria>

<output>
After completion, create `.planning/quick/014-fix-nav-tabs-reprovision/014-SUMMARY.md`
</output>
