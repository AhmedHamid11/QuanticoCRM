---
phase: quick-011
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - frontend/src/lib/components/EditSectionRenderer.svelte
  - frontend/src/routes/accounts/[id]/edit/+page.svelte
  - frontend/src/routes/contacts/[id]/edit/+page.svelte
  - frontend/src/routes/[entity=customentity]/[id]/edit/+page.svelte
autonomous: true

must_haves:
  truths:
    - "Edit page displays fields in same section groupings as detail page"
    - "Edit page uses same column layout (1, 2, or 3 columns) as detail page sections"
    - "Section headers appear on edit page matching detail page labels"
  artifacts:
    - path: "frontend/src/lib/components/EditSectionRenderer.svelte"
      provides: "Section-aware form field rendering with column layout"
      min_lines: 100
  key_links:
    - from: "edit/+page.svelte"
      to: "EditSectionRenderer.svelte"
      via: "component import and iteration"
      pattern: "EditSectionRenderer"
    - from: "EditSectionRenderer.svelte"
      to: "LayoutSectionV2"
      via: "section prop type"
      pattern: "LayoutSectionV2"
---

<objective>
Make edit pages use the same multi-column section layout structure as detail pages.

Purpose: Currently detail pages render fields organized by sections with configurable columns (via `SectionRenderer`), but edit pages flatten the layout to a simple list. This creates inconsistency between viewing and editing records.

Output: Edit pages will display form fields grouped by sections with matching column layouts.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md

Reference implementation (detail page with sections):
@frontend/src/routes/accounts/[id]/+page.svelte
@frontend/src/lib/components/SectionRenderer.svelte
@frontend/src/lib/types/layout.ts

Current edit page implementations (need updating):
@frontend/src/routes/accounts/[id]/edit/+page.svelte
@frontend/src/routes/contacts/[id]/edit/+page.svelte
@frontend/src/routes/[entity=customentity]/[id]/edit/+page.svelte
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create EditSectionRenderer component</name>
  <files>frontend/src/lib/components/EditSectionRenderer.svelte</files>
  <action>
Create a new Svelte component that renders form fields within a section layout, mirroring SectionRenderer's structure but with editable inputs.

**Props interface:**
- section: LayoutSectionV2 (the section definition with columns, label, fields)
- fields: FieldDef[] (field definitions for type/validation info)
- formData: Record<string, unknown> (reactive form state, use bind:)
- lookupNames: Record<string, string> (display names for link fields)
- multiLookupValues: Record<string, LookupRecord[]> (values for linkMultiple fields)
- getFieldError: (fieldName: string) => FieldValidationError | undefined
- onLookupChange: (fieldName: string, id: string | null, name: string) => void
- onMultiLookupChange: (fieldName: string, values: LookupRecord[]) => void

**Template structure:**
1. Outer container: `bg-white shadow rounded-lg overflow-hidden` (matches SectionRenderer)
2. Section header: `px-6 py-4 bg-gray-50 border-b border-gray-200` with section.label
3. Content area: `p-6` containing a grid
4. Grid: Use `grid-template-columns: repeat({section.columns}, minmax(0, 1fr))` with `gap-x-8 gap-y-4`
5. Field rendering: Similar to existing edit page field rendering logic but within grid items

**Field types to support:**
- text (textarea)
- bool (checkbox)
- enum (select dropdown)
- multiEnum (checkbox group)
- link (LookupField component)
- linkMultiple (MultiLookupField component)
- stream (StreamField component)
- All other types (input with appropriate type attribute)

**Important details:**
- Use evaluateVisibility from layout.ts to filter visible fields
- Skip read-only fields in the form (or render disabled)
- For text/textarea fields, span full width (`col-span-full` or all columns)
- Import LookupField, MultiLookupField, StreamField components
- Use fieldNameToKey for camelCase conversion
- Include field labels, required indicators, error states, tooltips
  </action>
  <verify>
Component file exists at frontend/src/lib/components/EditSectionRenderer.svelte and exports properly.
Run: `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend && npm run check`
  </verify>
  <done>EditSectionRenderer.svelte created with section-aware form rendering matching SectionRenderer's visual structure.</done>
</task>

<task type="auto">
  <name>Task 2: Update edit pages to use EditSectionRenderer</name>
  <files>
frontend/src/routes/accounts/[id]/edit/+page.svelte
frontend/src/routes/contacts/[id]/edit/+page.svelte
frontend/src/routes/[entity=customentity]/[id]/edit/+page.svelte
  </files>
  <action>
Update all three edit pages to use the layout's section structure instead of flattening to field names.

**Changes to each file:**

1. **Import changes:**
   - Add: `import EditSectionRenderer from '$lib/components/EditSectionRenderer.svelte';`
   - Add: `import { getVisibleSections } from '$lib/types/layout';` (if not already imported)
   - Keep existing parseLayoutData import

2. **State changes:**
   - Change `layoutFields = $state<string[]>([])` to `layout = $state<LayoutDataV2 | null>(null)`
   - Keep the layout parsing but store full layout instead of just field names

3. **Derived state:**
   - Add: `let visibleSections = $derived(() => layout ? getVisibleSections(layout, formData) : []);`
   - Remove: `editableFields` derived (no longer needed)

4. **Load function update:**
   - Change from `layoutFields = getAllFieldNames(layout)` to `layout = parsedLayout` (keep full structure)

5. **Template changes:**
   Replace the flat field iteration:
   ```svelte
   {#each editableFields as field}
     <!-- field rendering -->
   {/each}
   ```

   With section iteration:
   ```svelte
   {#each visibleSections() as section (section.id)}
     <EditSectionRenderer
       {section}
       {fields}
       bind:formData
       {lookupNames}
       {multiLookupValues}
       {getFieldError}
       onLookupChange={(fieldName, id, name) => {
         formData[`${fieldName}Id`] = id;
         formData[`${fieldName}Name`] = name;
         lookupNames[fieldName] = name;
       }}
       onMultiLookupChange={(fieldName, values) => {
         multiLookupValues[fieldName] = values;
         formData[`${fieldName}Ids`] = JSON.stringify(values.map(v => v.id));
         formData[`${fieldName}Names`] = JSON.stringify(values.map(v => v.name));
       }}
     />
   {/each}
   ```

6. **Keep existing:**
   - Form submission logic (handleSubmit)
   - Error handling (ValidationErrors component at top)
   - Action buttons (Cancel/Save at bottom)
   - Breadcrumb navigation

**File-specific notes:**

- **accounts/[id]/edit:** Has SYSTEM_FIELDS set, lookupNames, multiLookupValues state already
- **contacts/[id]/edit:** Uses useFormErrors hook, different error handling pattern
- **[entity]/[id]/edit:** Generic entity, no SYSTEM_FIELDS, simpler pattern

For contacts edit page, adapt the error handling to work with EditSectionRenderer's getFieldError callback.
  </action>
  <verify>
All three edit pages render correctly:
1. `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend && npm run check`
2. Start frontend dev server and navigate to /accounts/{id}/edit
3. Verify sections appear with section headers and multi-column layout
4. Verify form submission still works
  </verify>
  <done>All edit pages updated to render fields in sections matching detail page layout structure.</done>
</task>

</tasks>

<verification>
1. TypeScript check passes: `cd frontend && npm run check`
2. Account edit page shows sections with proper column layout
3. Contact edit page shows sections with proper column layout
4. Custom entity edit page shows sections with proper column layout
5. Form submissions work correctly on all three edit pages
6. Field validation errors display correctly within sections
</verification>

<success_criteria>
- EditSectionRenderer component exists and renders form fields in section groups
- All edit pages use section-based layout matching their detail pages
- Column layout (1, 2, or 3 columns per section) is respected
- Section headers display with correct labels
- Form functionality (submit, validation, errors) unchanged
</success_criteria>

<output>
After completion, create `.planning/quick/011-edit-page-match-detail-page-layout-struc/011-SUMMARY.md`
</output>
