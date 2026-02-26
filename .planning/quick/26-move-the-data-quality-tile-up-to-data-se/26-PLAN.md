---
phase: quick-26
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - frontend/src/routes/admin/+page.svelte
autonomous: true
requirements: [QUICK-26]

must_haves:
  truths:
    - "Data Quality tile appears under the Data section on the admin dashboard"
    - "Data Quality no longer appears as its own standalone section"
    - "All other tiles and sections remain unchanged"
  artifacts:
    - path: "frontend/src/routes/admin/+page.svelte"
      provides: "Admin dashboard tile configuration"
      contains: "section: 'Data'"
  key_links:
    - from: "Data Quality tile object"
      to: "sectionOrder array"
      via: "section property matching sectionOrder entries"
      pattern: "section: 'Data'"
---

<objective>
Move the Data Quality tile from its own "Data Quality" section into the "Data" section on the admin dashboard, and remove the now-empty "Data Quality" entry from sectionOrder.

Purpose: Clean up admin dashboard layout by consolidating Data Quality under the existing Data section.
Output: Updated admin dashboard with Data Quality tile in Data section.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@frontend/src/routes/admin/+page.svelte
</context>

<tasks>

<task type="auto">
  <name>Task 1: Move Data Quality tile to Data section and clean up sectionOrder</name>
  <files>frontend/src/routes/admin/+page.svelte</files>
  <action>
    In `frontend/src/routes/admin/+page.svelte`, make these changes:

    1. Find the Data Quality tile definition (around line 32):
       ```
       { title: 'Data Quality', ..., section: 'Data Quality' },
       ```
       Change `section: 'Data Quality'` to `section: 'Data'`.

    2. Remove the `// Data Quality` comment on the line above the tile (around line 31) since it no longer has its own section. Optionally move the tile to sit alongside the other Data tiles (Data Mirror, Data Import, Data Explorer) for readability.

    3. Find the `sectionOrder` array (around line 50):
       ```
       const sectionOrder = ['Customization', 'Data', 'Data Quality', 'Automation', 'System', 'Platform Administration'];
       ```
       Remove `'Data Quality'` from the array so it becomes:
       ```
       const sectionOrder = ['Customization', 'Data', 'Automation', 'System', 'Platform Administration'];
       ```

    Do NOT change anything else -- tile href, colors, icons, description, or any other tiles.
  </action>
  <verify>
    Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend && npm run build` to confirm no build errors.
    Then grep to confirm:
    - `grep "section: 'Data Quality'" src/routes/admin/+page.svelte` returns NO results
    - `grep "'Data Quality'" src/routes/admin/+page.svelte` returns NO results
    - `grep "Data Quality" src/routes/admin/+page.svelte` only matches the tile title/description, not section references
  </verify>
  <done>
    The Data Quality tile has `section: 'Data'` instead of `section: 'Data Quality'`. The sectionOrder array no longer contains 'Data Quality'. Build succeeds with no errors.
  </done>
</task>

</tasks>

<verification>
- Frontend builds without errors
- No remaining references to `section: 'Data Quality'` or `'Data Quality'` in sectionOrder
- Data Quality tile still exists with correct title, description, href, and styling
- Only the section assignment and sectionOrder were changed
</verification>

<success_criteria>
- Data Quality tile renders under the "Data" section heading on the admin dashboard
- "Data Quality" section heading no longer appears on the admin dashboard
- All other tiles and sections are unaffected
</success_criteria>

<output>
After completion, create `.planning/quick/26-move-the-data-quality-tile-up-to-data-se/26-SUMMARY.md`
</output>
