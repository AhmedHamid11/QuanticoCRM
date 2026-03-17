---
phase: "quick-018"
plan: "01"
type: "execute"
wave: 1
depends_on: []
files_modified:
  - "backend/internal/service/provisioning.go"
autonomous: true
must_haves:
  truths:
    - "Deploying the backend never drops existing metadata tables (entity_defs, field_defs, layout_defs)"
    - "Existing user-customized layouts, nav tabs, and related list configs survive re-provisioning"
    - "New orgs still get metadata tables and default data created correctly"
  artifacts:
    - path: "backend/internal/service/provisioning.go"
      provides: "Safe ensureMetadataTables using PRAGMA table_info, INSERT OR IGNORE for defaults"
      contains: "PRAGMA table_info(entity_defs)"
  key_links:
    - from: "ensureMetadataTables()"
      to: "PRAGMA table_info"
      via: "column existence check replaces fragile sql LIKE check"
      pattern: "PRAGMA table_info"
---

<objective>
Fix metadata corruption on deployment by removing destructive DROP/recreate logic from ensureMetadataTables and changing INSERT OR REPLACE to INSERT OR IGNORE for default data.

Purpose: Every deployment currently risks wiping all org metadata (entity_defs, field_defs, layout_defs) due to a fragile schema check that triggers a nuclear DROP TABLE cascade. This must be fixed so deployments are safe.

Output: Updated provisioning.go with non-destructive table checking and data insertion.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@backend/internal/service/provisioning.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Replace fragile schema check with PRAGMA table_info in ensureMetadataTables</name>
  <files>backend/internal/service/provisioning.go</files>
  <action>
In `ensureMetadataTables()` (line 94-262), replace the fragile schema validation logic (lines 110-128) that uses `sql LIKE '%org_id%name%'` to detect UNIQUE constraints — this pattern fails unpredictably and triggers `dropAndRecreateMetadataTables()` which destroys all metadata.

Replace with a `PRAGMA table_info(entity_defs)` approach (same pattern already used successfully in `ensureNavigationTabsTable()` at line 275). The logic should be:

1. If entity_defs table exists (line 109 check stays as-is), use `PRAGMA table_info(entity_defs)` to check if the `org_id` column exists.
2. If `org_id` column exists: the table schema is fine. Log "entity_defs table exists with correct schema, skipping creation" and return nil. Do NOT check for UNIQUE constraints — if the table has org_id, it was created with the correct schema.
3. If `org_id` column does NOT exist: this is a genuinely broken legacy table (pre-migration-019). Log a warning but do NOT drop it. Instead return nil — the migration system is responsible for schema changes, not provisioning.
4. Remove the call to `dropAndRecreateMetadataTables()` from this function entirely (delete lines 123-128).

The `dropAndRecreateMetadataTables()` function body (lines 328-onwards) can remain in the file as dead code for potential emergency manual use, but add a comment: "// DEPRECATED: This function is dangerous and should never be called automatically. Retained for emergency manual use only."

The rest of `ensureMetadataTables()` (lines 131-261, the "table doesn't exist" path with CREATE TABLE IF NOT EXISTS) stays unchanged — that path is correct for new orgs.
  </action>
  <verify>
Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` to confirm compilation succeeds. Then grep the file to confirm:
- No remaining call to `dropAndRecreateMetadataTables` in `ensureMetadataTables`
- `PRAGMA table_info(entity_defs)` is present
- The `sql LIKE '%org_id%name%'` pattern is removed
  </verify>
  <done>ensureMetadataTables uses PRAGMA table_info for safe column check, never calls dropAndRecreateMetadataTables, and the CREATE TABLE IF NOT EXISTS path for new orgs is preserved unchanged.</done>
</task>

<task type="auto">
  <name>Task 2: Change INSERT OR REPLACE to INSERT OR IGNORE for default data</name>
  <files>backend/internal/service/provisioning.go</files>
  <action>
Change three INSERT OR REPLACE statements to INSERT OR IGNORE so that existing user-customized data survives re-provisioning:

1. **`createNavTabWithError()`** (line 1113-1117): Change `INSERT OR REPLACE INTO navigation_tabs` to `INSERT OR IGNORE INTO navigation_tabs`. Update the comment from "Use INSERT OR REPLACE to fix stale/broken tab data during reprovision" to "Use INSERT OR IGNORE to preserve existing user-customized tabs during reprovision".

2. **`createLayout()`** (line 1132-1136): Change `INSERT OR REPLACE INTO layout_defs` to `INSERT OR IGNORE INTO layout_defs`. Update the comment from "Use INSERT OR REPLACE to handle potential duplicates" to "Use INSERT OR IGNORE to preserve existing user-customized layouts during reprovision".

3. **`createDefaultRelatedListConfigs()`** (line 1270-1278): Change `INSERT OR REPLACE INTO related_list_configs` to `INSERT OR IGNORE INTO related_list_configs`. Update the comment from "Insert each related list config using INSERT OR REPLACE for safe re-provisioning" to "Insert each related list config using INSERT OR IGNORE to preserve existing customizations".

INSERT OR IGNORE will silently skip if a row with the same UNIQUE constraint already exists, preserving any user modifications. New orgs (with empty tables) will get all defaults inserted normally.
  </action>
  <verify>
Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` to confirm compilation. Then grep provisioning.go:
- `grep -c "INSERT OR REPLACE" provisioning.go` should return 0 (all replaced)
- `grep -c "INSERT OR IGNORE" provisioning.go` should show 3 occurrences (the three changed locations)
  </verify>
  <done>All three INSERT OR REPLACE statements changed to INSERT OR IGNORE. Existing user customizations (layouts, nav tabs, related list configs) now survive re-provisioning. New orgs still get all defaults.</done>
</task>

</tasks>

<verification>
1. `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...` passes with no errors
2. `grep "dropAndRecreateMetadataTables" provisioning.go` shows only the function definition (with DEPRECATED comment) but no calls to it
3. `grep "PRAGMA table_info(entity_defs)" provisioning.go` returns a match
4. `grep "INSERT OR REPLACE" provisioning.go` returns no matches
5. `grep "INSERT OR IGNORE" provisioning.go` returns 3 matches
6. `grep "sql LIKE" provisioning.go` returns no matches in ensureMetadataTables
</verification>

<success_criteria>
- Backend compiles cleanly
- ensureMetadataTables never drops existing tables
- Default data insertion uses INSERT OR IGNORE (preserves customizations)
- New org provisioning still works (CREATE TABLE IF NOT EXISTS path unchanged)
- dropAndRecreateMetadataTables is dead code with DEPRECATED comment
</success_criteria>

<output>
After completion, create `.planning/quick/18-fix-metadata-corruption-on-deployment-re/18-SUMMARY.md`
</output>
