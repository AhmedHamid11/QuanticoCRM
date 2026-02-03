---
phase: quick-007
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend/internal/repo/metadata.go
  - /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend/internal/handler/admin.go
  - /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/src/routes/admin/entity-manager/[entity]/+page.svelte
autonomous: true

must_haves:
  truths:
    - "Custom entities can be soft-deleted from Entity Manager detail page"
    - "Soft-deleted entities do not appear in entity lists"
    - "Soft-delete renames entity name to {name}__del to preserve all metadata"
    - "Only custom entities can be deleted (system entities are protected)"
  artifacts:
    - path: "backend/internal/repo/metadata.go"
      provides: "SoftDeleteEntity method"
      contains: "__del"
    - path: "backend/internal/handler/admin.go"
      provides: "DELETE /admin/entities/:name endpoint"
      contains: "DeleteEntity"
    - path: "frontend/src/routes/admin/entity-manager/[entity]/+page.svelte"
      provides: "Delete button with confirmation modal"
      contains: "deleteEntity"
  key_links:
    - from: "frontend delete button"
      to: "DELETE /admin/entities/:name"
      via: "del() API call"
    - from: "handler.DeleteEntity"
      to: "repo.SoftDeleteEntity"
      via: "function call"
---

<objective>
Implement soft-delete for custom entities in FastCRM.

Purpose: Allow users to remove custom entities they no longer need while preserving the underlying data for potential recovery or audit purposes.

Output: Backend DELETE endpoint that renames entity to `{name}__del`, frontend delete button with confirmation dialog on Entity Manager detail page.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend/internal/repo/metadata.go
@/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend/internal/handler/admin.go
@/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/src/routes/admin/entity-manager/[entity]/+page.svelte
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add SoftDeleteEntity to MetadataRepo and ListEntities filter</name>
  <files>/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend/internal/repo/metadata.go</files>
  <action>
1. Update `ListEntities` query to filter out soft-deleted entities:
   - Add `AND name NOT LIKE '%__del'` to the WHERE clause

2. Add new `SoftDeleteEntity` method that:
   - Takes ctx, orgID, entityName as parameters
   - Verifies entity exists and is custom (isCustom = true)
   - Returns error if entity is not custom: "Cannot delete system entity"
   - Renames entity by appending `__del` suffix to the name
   - Updates: entity_defs.name, all field_defs.entity_name, all layout_defs.entity_name
   - Also hides the navigation tab by setting is_visible = false for any nav tab linked to this entity
   - Use a transaction to ensure atomicity
   - Return nil on success
  </action>
  <verify>
Build backend: `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...`
  </verify>
  <done>
MetadataRepo has SoftDeleteEntity method that renames entity name with __del suffix and ListEntities filters out __del entities.
  </done>
</task>

<task type="auto">
  <name>Task 2: Add DeleteEntity handler and route</name>
  <files>/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend/internal/handler/admin.go</files>
  <action>
1. Add `DeleteEntity` handler method:
   ```go
   func (h *AdminHandler) DeleteEntity(c *fiber.Ctx) error {
       orgID := c.Locals("orgID").(string)
       name := c.Params("name")

       err := h.getMetadataRepo(c).SoftDeleteEntity(c.Context(), orgID, name)
       if err != nil {
           if err.Error() == "Cannot delete system entity" {
               return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                   "error": err.Error(),
               })
           }
           if err == sql.ErrNoRows {
               return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
                   "error": "Entity not found",
               })
           }
           return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
               "error": err.Error(),
           })
       }

       return c.Status(fiber.StatusNoContent).Send(nil)
   }
   ```

2. Register DELETE route in `RegisterRoutes`:
   - Add after the PATCH route: `admin.Delete("/entities/:name", h.DeleteEntity)`
  </action>
  <verify>
Build backend: `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./...`
Test with curl (after running backend): `curl -X DELETE http://localhost:8080/api/v1/admin/entities/TestEntity -H "Authorization: Bearer <token>"`
  </verify>
  <done>
DELETE /admin/entities/:name endpoint exists and calls SoftDeleteEntity, returns 204 on success, 403 for system entities, 404 for missing.
  </done>
</task>

<task type="auto">
  <name>Task 3: Add delete button with confirmation to Entity Manager UI</name>
  <files>/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/src/routes/admin/entity-manager/[entity]/+page.svelte</files>
  <action>
1. Import `del` from '$lib/utils/api' (add to existing import)

2. Import `goto` from '$app/navigation'

3. Add state variables:
   ```typescript
   let showDeleteModal = $state(false);
   let deleting = $state(false);
   let deleteError = $state<string | null>(null);
   ```

4. Add delete functions:
   ```typescript
   function openDeleteModal() {
       deleteError = null;
       showDeleteModal = true;
   }

   function closeDeleteModal() {
       showDeleteModal = false;
   }

   async function confirmDelete() {
       if (!entity) return;

       deleting = true;
       deleteError = null;

       try {
           await del(`/admin/entities/${entityName}`);
           goto('/admin/entity-manager');
       } catch (e) {
           deleteError = e instanceof Error ? e.message : 'Failed to delete entity';
       } finally {
           deleting = false;
       }
   }
   ```

5. Add Delete button in the "Entity Settings" card header (next to Edit Settings button):
   - Only show if `entity?.isCustom` is true
   - Red styling to indicate destructive action
   - Button text: "Delete Entity"
   - onClick: `openDeleteModal`

6. Add Delete Confirmation Modal at the end of the file (after Edit Settings Modal):
   ```svelte
   {#if showDeleteModal}
       <div class="fixed inset-0 z-50 overflow-y-auto">
           <div class="flex min-h-screen items-center justify-center p-4">
               <div class="fixed inset-0 bg-black bg-opacity-50" onclick={closeDeleteModal}></div>
               <div class="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
                   <div class="flex items-center justify-between mb-4">
                       <h2 class="text-xl font-semibold text-gray-900">Delete Entity</h2>
                       <button onclick={closeDeleteModal} class="text-gray-400 hover:text-gray-600">
                           <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                               <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                           </svg>
                       </button>
                   </div>

                   <div class="mb-4">
                       <div class="flex items-center gap-3 p-3 bg-red-50 border border-red-200 rounded-lg">
                           <svg class="w-6 h-6 text-red-600 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                               <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                           </svg>
                           <div class="text-sm text-red-700">
                               <p class="font-medium">Are you sure you want to delete "{entity?.label}"?</p>
                               <p class="mt-1">This entity will be hidden from the system. Existing records will be preserved but no longer accessible.</p>
                           </div>
                       </div>
                   </div>

                   {#if deleteError}
                       <div class="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
                           {deleteError}
                       </div>
                   {/if}

                   <div class="flex justify-end gap-3">
                       <button
                           type="button"
                           onclick={closeDeleteModal}
                           class="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
                       >
                           Cancel
                       </button>
                       <button
                           type="button"
                           onclick={confirmDelete}
                           disabled={deleting}
                           class="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                       >
                           {deleting ? 'Deleting...' : 'Delete Entity'}
                       </button>
                   </div>
               </div>
           </div>
       </div>
   {/if}
   ```
  </action>
  <verify>
1. Frontend compiles: `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend && npm run build`
2. Browser verification:
   - Navigate to a custom entity in Entity Manager
   - Verify "Delete Entity" button appears (red button)
   - Click button, verify confirmation modal appears
   - Verify system entities do NOT show delete button
  </verify>
  <done>
Entity Manager detail page shows Delete Entity button for custom entities with confirmation modal that calls DELETE API and redirects to entity list on success.
  </done>
</task>

</tasks>

<verification>
1. Backend builds without errors
2. Frontend builds without errors
3. Custom entity can be soft-deleted via UI
4. After deletion, entity no longer appears in entity list
5. System entities do not show delete button
6. Confirmation modal displays warning about preserved records
</verification>

<success_criteria>
- DELETE /admin/entities/:name endpoint returns 204 for custom entities
- DELETE /admin/entities/:name endpoint returns 403 for system entities
- Entity name is renamed to {name}__del in database
- Field and layout definitions reference the new name
- Navigation tab is hidden (is_visible = false)
- ListEntities filters out __del entities
- Frontend shows delete button only for custom entities
- Confirmation modal prevents accidental deletion
</success_criteria>

<output>
After completion, create `.planning/quick/007-soft-delete-custom-entities/007-SUMMARY.md`
</output>
