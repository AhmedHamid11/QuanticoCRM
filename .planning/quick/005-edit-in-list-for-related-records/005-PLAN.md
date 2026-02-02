---
task: 005
type: quick
title: Edit in List for Related Records
complexity: medium
files_modified:
  - FastCRM/fastcrm/backend/internal/migrations/009_add_edit_in_list_to_related_configs.sql
  - FastCRM/fastcrm/backend/internal/entity/related_list.go
  - FastCRM/fastcrm/backend/internal/repo/related_list.go
  - FastCRM/fastcrm/frontend/src/lib/types/related-list.ts
  - FastCRM/fastcrm/frontend/src/lib/components/RelatedList.svelte
  - FastCRM/fastcrm/frontend/src/routes/admin/entity-manager/[entity]/related-lists/+page.svelte
---

<objective>
Add "Edit in List" option to related lists that, when enabled, allows users to create new records inline within the related list instead of navigating to a separate page. The new row appears as an editable row in the list, and after saving, the user stays on the parent record page.

Purpose: Improve UX by reducing navigation when creating related records - users can quickly add items without losing context of the parent record.

Output:
- Backend migration adding `edit_in_list` column to related_list_configs
- Updated entity and repo to handle the new field
- Frontend RelatedList component with inline edit mode
- Admin UI toggle to enable/disable edit-in-list per related list
</objective>

<context>
@FastCRM/fastcrm/backend/internal/entity/related_list.go
@FastCRM/fastcrm/backend/internal/repo/related_list.go
@FastCRM/fastcrm/frontend/src/lib/components/RelatedList.svelte
@FastCRM/fastcrm/frontend/src/lib/types/related-list.ts
@FastCRM/fastcrm/frontend/src/routes/admin/entity-manager/[entity]/related-lists/+page.svelte
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add edit_in_list column to database and Go backend</name>
  <files>
    FastCRM/fastcrm/backend/internal/migrations/009_add_edit_in_list_to_related_configs.sql
    FastCRM/fastcrm/backend/internal/entity/related_list.go
    FastCRM/fastcrm/backend/internal/repo/related_list.go
  </files>
  <action>
1. Create migration file `009_add_edit_in_list_to_related_configs.sql`:
   - Add `edit_in_list INTEGER DEFAULT 0` column to related_list_configs table

2. Update `entity/related_list.go`:
   - Add `EditInList bool` field to `RelatedListConfig` struct with json tag "editInList" and db tag "edit_in_list"
   - Add `EditInList bool` field to `RelatedListConfigCreateInput` struct
   - Add `EditInList *bool` field to `RelatedListConfigUpdateInput` struct (pointer for optional update)

3. Update `repo/related_list.go`:
   - Add `edit_in_list` to all SELECT queries in ListByEntity, ListEnabledByEntity, GetByID
   - Add `edit_in_list` to INSERT in Create method
   - Add `edit_in_list` to UPDATE in Update method
   - Add `edit_in_list` to INSERT in BulkSave method
   - Update EnsureSchema to check for and add `edit_in_list` column if missing (handles existing tenant DBs)
  </action>
  <verify>
    - `cd FastCRM/fastcrm/backend && go build ./...` compiles without errors
    - Review the migration SQL is valid
    - Verify all repo queries include the new column
  </verify>
  <done>
    - Migration file exists
    - Go structs have EditInList field
    - All repo methods handle the new field
    - Backend compiles successfully
  </done>
</task>

<task type="auto">
  <name>Task 2: Add inline editing UI to RelatedList component</name>
  <files>
    FastCRM/fastcrm/frontend/src/lib/types/related-list.ts
    FastCRM/fastcrm/frontend/src/lib/components/RelatedList.svelte
  </files>
  <action>
1. Update `types/related-list.ts`:
   - Add `editInList?: boolean` to RelatedListConfig interface
   - Add `editInList?: boolean` to RelatedListConfigCreateInput interface

2. Update `RelatedList.svelte` to add inline editing mode:
   - Add state: `isAddingInline = $state(false)` and `newRecord = $state<Record<string, unknown>>({})` and `saving = $state(false)`
   - Modify `handleCreateNew()`: If `config.editInList` is true, set `isAddingInline = true` and pre-fill the lookup field in newRecord. Otherwise use existing navigation behavior.
   - Add `saveInlineRecord()` async function:
     - POST to `/entities/{relatedEntity}/records` with newRecord data
     - Include the lookup field value (parentId) in the request
     - On success: set `isAddingInline = false`, clear `newRecord`, call `loadData()` to refresh
     - On error: show error message, keep form open
   - Add `cancelInline()` function to reset isAddingInline and clear newRecord
   - In the template, when `isAddingInline` is true, render an editable row at the top of the table body:
     - For each displayField, render an input field based on field type (text input for most, or appropriate control)
     - Use simple text inputs initially - can enhance later with proper field type handling
     - Add Save and Cancel buttons at the end of the row
   - Style the inline row with a light blue/highlighted background to distinguish from existing rows
  </action>
  <verify>
    - `cd FastCRM/fastcrm/frontend && npm run check` passes
    - Types are correct with no TypeScript errors
    - Component renders without errors
  </verify>
  <done>
    - Types updated with editInList field
    - RelatedList shows "+ New" button behavior based on editInList setting
    - When editInList=true, clicking New shows inline editable row
    - User can fill fields, save (creates record), or cancel
    - After save, list refreshes and shows new record
  </done>
</task>

<task type="auto">
  <name>Task 3: Add edit-in-list toggle to admin configuration UI</name>
  <files>
    FastCRM/fastcrm/frontend/src/routes/admin/entity-manager/[entity]/related-lists/+page.svelte
  </files>
  <action>
1. Update the related lists admin page to include edit-in-list toggle:
   - In the "Enabled Related Lists" section, add a toggle/checkbox for each config
   - Add after the label input, before the "fields" button
   - Use a small toggle switch or checkbox with label "Edit in List"
   - When toggled, update `config.editInList` and trigger reactivity with `configuredLists = [...configuredLists]`

2. The toggle should:
   - Show current state (checked if editInList is true)
   - On change, update the config object
   - Changes are saved when user clicks "Save Changes" (existing save flow)

3. Add helpful tooltip/title: "When enabled, clicking 'New' creates an inline editable row instead of navigating to a new page"
  </action>
  <verify>
    - Admin page loads without errors
    - Toggle appears for each enabled related list
    - Toggle state persists after saving
    - `npm run check` passes
  </verify>
  <done>
    - Admin UI shows toggle for edit-in-list per related list config
    - Toggle changes are saved via existing bulk save endpoint
    - Setting persists and is returned in config
  </done>
</task>

</tasks>

<verification>
1. Backend compiles: `cd FastCRM/fastcrm/backend && go build ./...`
2. Frontend checks: `cd FastCRM/fastcrm/frontend && npm run check`
3. Manual test flow:
   - Go to Admin > Entity Manager > Account > Related Lists
   - Enable "Edit in List" for Contacts related list
   - Save changes
   - Go to an Account detail page
   - Click "+ New" on the Contacts related list
   - Verify inline row appears (not navigation)
   - Fill in fields, click Save
   - Verify new contact is created and appears in list
</verification>

<success_criteria>
- Migration adds edit_in_list column
- Go backend handles editInList field in all CRUD operations
- Frontend types include editInList
- RelatedList component supports inline editing when editInList=true
- Admin UI provides toggle to enable/disable edit-in-list per related list
- Existing behavior (navigate to new page) preserved when editInList=false
</success_criteria>

<output>
After completion, create `.planning/quick/005-edit-in-list-for-related-records/005-SUMMARY.md`
</output>
