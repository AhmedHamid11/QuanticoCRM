---
phase: quick-011
plan: 01
subsystem: ui-forms
tags: [layout, edit-pages, sections, multi-column, consistency]
requires: [layout-system-v2, section-renderer]
provides: [section-based-edit-forms, layout-consistency]
affects: []

tech-stack:
  added: []
  patterns: [section-based-form-rendering, reusable-edit-components]

key-files:
  created:
    - frontend/src/lib/components/EditSectionRenderer.svelte
  modified:
    - frontend/src/routes/accounts/[id]/edit/+page.svelte
    - frontend/src/routes/contacts/[id]/edit/+page.svelte
    - frontend/src/routes/[entity=customentity]/[id]/edit/+page.svelte

decisions:
  - decision: Create dedicated EditSectionRenderer component instead of reusing SectionRenderer
    rationale: Edit forms require different field rendering (inputs vs display), validation handling, and reactive bindings
    alternatives: [modify-section-renderer, inline-form-logic]
    chosen: dedicated-component

  - decision: Use same section visibility and column layout as detail pages
    rationale: Provides visual consistency between viewing and editing, reduces cognitive load for users
    alternatives: [flat-form-layout, custom-edit-layout]
    chosen: match-detail-layout

metrics:
  duration: 5 minutes
  commits: 2
  files-changed: 4
  lines-added: 397
  lines-removed: 524
  net-change: -127

completed: 2026-02-05
---

# Quick Task 011: Edit Page Layout Structure Matching

**One-liner:** Edit pages now use section-based layouts with multi-column rendering matching detail page structure

## Objective

Make edit pages use the same multi-column section layout structure as detail pages for visual consistency between viewing and editing records.

## What Was Built

### 1. EditSectionRenderer Component
**File:** `frontend/src/lib/components/EditSectionRenderer.svelte`

A new Svelte component that renders form fields within section layouts:

**Key features:**
- Mirrors SectionRenderer's visual structure (section headers, column grids)
- Supports all field types: text, bool, enum, multiEnum, link, linkMultiple, stream
- Handles field visibility rules based on form data
- Displays validation errors inline per field
- Excludes read-only fields from forms
- Wide fields (text, stream) automatically span full width

**Props interface:**
```typescript
{
  section: LayoutSectionV2;          // Section definition with columns, label, fields
  fields: FieldDef[];                // Field definitions for type/validation
  formData: Record<string, unknown>; // Reactive form state (bindable)
  lookupNames: Record<string, string>;
  multiLookupValues: Record<string, LookupRecord[]>;
  getFieldError: (fieldName: string) => FieldValidationError | undefined;
  onLookupChange: (fieldName: string, id: string | null, name: string) => void;
  onMultiLookupChange: (fieldName: string, values: LookupRecord[]) => void;
}
```

### 2. Updated Edit Pages

All three edit page types refactored to use section-based rendering:

**Changes applied to:**
- `frontend/src/routes/accounts/[id]/edit/+page.svelte`
- `frontend/src/routes/contacts/[id]/edit/+page.svelte`
- `frontend/src/routes/[entity=customentity]/[id]/edit/+page.svelte`

**Refactoring pattern:**
1. **State changes:**
   - Changed `layoutFields = $state<string[]>([])` to `layout = $state<LayoutDataV2 | null>(null)`
   - Added `let visibleSections = $derived(() => layout ? getVisibleSections(layout, formData) : [])`

2. **Import additions:**
   - `import EditSectionRenderer from '$lib/components/EditSectionRenderer.svelte'`
   - `import { getVisibleSections } from '$lib/types/layout'`

3. **Load function updates:**
   - Store full layout structure instead of flattened field names
   - Changed from `layoutFields = getAllFieldNames(layout)` to `layout = parsedLayout`

4. **Template refactoring:**
   - Replaced flat `{#each editableFields}` iteration with section iteration
   - Each section renders via `<EditSectionRenderer />` component
   - Preserved form submission logic, error handling, and action buttons

**Example template structure:**
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

## Benefits

1. **Visual Consistency:** Edit and detail pages now have matching section structure
2. **User Experience:** Reduced cognitive load - same fields appear in same locations
3. **Layout Flexibility:** Edit pages automatically adopt any layout customizations made in admin
4. **Code Reduction:** Net -127 lines by consolidating form rendering logic into reusable component
5. **Maintainability:** Single component to update for form rendering improvements

## Technical Details

### Field Type Support

EditSectionRenderer handles all field types:

| Type | Rendering | Special Handling |
|------|-----------|------------------|
| text | textarea | Spans full width (col-span-full) |
| bool | checkbox | Single checkbox input |
| enum | select dropdown | Parses JSON or CSV options |
| multiEnum | checkbox group | Tracks array of selected values |
| link | LookupField component | Stores both ID and Name |
| linkMultiple | MultiLookupField component | Stores JSON arrays of IDs/Names |
| stream | StreamField component | Spans full width, entry + log |
| Other | input[type] | Appropriate HTML5 input type |

### Layout System Integration

Edit pages now fully integrate with the backend layout system:
- Fetches layout from `/entities/{Entity}/layouts/detail` API
- Uses `parseLayoutData()` to handle v1, v2, and legacy formats
- Applies `getVisibleSections()` for conditional visibility
- Respects column configuration (1, 2, or 3 columns per section)

### Error Handling

Each edit page handles validation errors differently but all integrate:
- **Accounts:** Uses `getFieldError()` helper that searches `fieldErrors` array
- **Contacts:** Uses `useFormErrors` hook, adapts to `getFieldError` callback
- **Custom entities:** Uses `getFieldError()` helper for direct field error lookup

## Deviations from Plan

None - plan executed exactly as written.

## Testing Notes

To verify the changes:

1. **TypeScript check passes:** `cd frontend && npm run check` (unrelated auth errors pre-exist)
2. **Visual verification needed:**
   - Navigate to `/accounts/{id}/edit` - verify sections appear
   - Check section headers display
   - Verify fields arrange in multi-column layout
   - Test form submission still works
   - Confirm validation errors display correctly

## Next Phase Readiness

**Ready for:**
- Any additional edit page improvements
- Layout customization features (will automatically apply to edit pages)
- Field-level conditional visibility enhancements

**No blockers or concerns.**

## Commits

| Commit | Task | Description |
|--------|------|-------------|
| 44d30e7 | 1 | Create EditSectionRenderer component |
| edf94a1 | 2 | Update edit pages to use section-based layout |

---

*Summary completed: 2026-02-05*
*Total execution time: ~5 minutes*
