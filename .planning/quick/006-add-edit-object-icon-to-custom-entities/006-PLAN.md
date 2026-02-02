---
phase: quick-006
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - fastcrm/frontend/src/routes/[entity=customentity]/[id]/+page.svelte
autonomous: true

must_haves:
  truths:
    - "Custom entity detail pages show a settings icon in the header actions"
    - "Clicking the icon navigates to /admin/entity-manager/{entityName}"
  artifacts:
    - path: "fastcrm/frontend/src/routes/[entity=customentity]/[id]/+page.svelte"
      provides: "Settings icon button in header"
      contains: "entity-manager"
  key_links:
    - from: "Settings icon button"
      to: "/admin/entity-manager/{entityName}"
      via: "href attribute"
      pattern: "entity-manager.*entityName"
---

<objective>
Add an "Edit Object Settings" icon button to custom entity detail pages that links to the entity's admin configuration page.

Purpose: Gives quick access to entity configuration (fields, layouts, bearings, validation rules) directly from a record detail view, matching the UX pattern established on the Account detail page.

Output: Updated custom entity detail page with settings icon in the action buttons area.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@fastcrm/frontend/src/routes/[entity=customentity]/[id]/+page.svelte
@fastcrm/frontend/src/routes/admin/entity-manager/[entity]/+page.svelte (reference for icon style)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add settings icon button to custom entity detail page header</name>
  <files>fastcrm/frontend/src/routes/[entity=customentity]/[id]/+page.svelte</files>
  <action>
In the action buttons section (around line 278-301, inside `<div class="flex gap-2">`), add a settings icon button BEFORE the FlowButtons loop.

Add this button:
```svelte
<a
  href="/admin/entity-manager/{entityName}"
  class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
  title="Entity Settings"
>
  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
  </svg>
</a>
```

This uses:
- A cog/gear icon (standard settings iconography)
- Icon-only button with tooltip via title attribute
- Subtle styling that matches the action bar aesthetic
- Links to `/admin/entity-manager/{entityName}` which is the entity configuration page

Position it first in the action buttons (before FlowButtons) so it appears on the left side of the actions, near the record info but distinct from record-specific actions like Edit/Delete.
  </action>
  <verify>
    - Navigate to any custom entity detail page (e.g., /tickets/some-id)
    - Verify settings icon (cog) appears in the header action area
    - Hover shows "Entity Settings" tooltip
    - Click navigates to /admin/entity-manager/{EntityName}
  </verify>
  <done>
    - Settings icon visible on custom entity detail pages
    - Icon links to correct admin entity manager URL
    - Hover tooltip displays "Entity Settings"
  </done>
</task>

</tasks>

<verification>
1. Navigate to a custom entity detail page
2. Confirm settings cog icon is visible in the action buttons area
3. Hover to see "Entity Settings" tooltip
4. Click and verify navigation to /admin/entity-manager/{entityName}
5. Verify no console errors
</verification>

<success_criteria>
- Custom entity detail pages display a settings icon button
- Clicking the icon navigates to the entity's admin configuration page
- The icon styling matches the existing UI aesthetic (subtle, icon-only with tooltip)
</success_criteria>

<output>
After completion, create `.planning/quick/006-add-edit-object-icon-to-custom-entities/006-SUMMARY.md`
</output>
