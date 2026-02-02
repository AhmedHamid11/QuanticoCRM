---
task: 005
type: quick
title: Edit in List for Related Records
completed: 2026-02-02
commits:
  - 37ca093
  - 433459f
  - 8158119
files_created:
  - FastCRM/fastcrm/backend/internal/migrations/045_add_edit_in_list_to_related_configs.sql
files_modified:
  - FastCRM/fastcrm/backend/internal/entity/related_list.go
  - FastCRM/fastcrm/backend/internal/repo/related_list.go
  - FastCRM/fastcrm/frontend/src/lib/types/related-list.ts
  - FastCRM/fastcrm/frontend/src/lib/components/RelatedList.svelte
  - FastCRM/fastcrm/frontend/src/routes/admin/entity-manager/[entity]/related-lists/+page.svelte
---

# Quick Task 005: Edit in List for Related Records

## Summary

Added "Edit in List" feature to related lists that allows users to create new records inline within the related list instead of navigating to a separate page. When enabled, clicking the "+ New" button shows an editable row at the top of the list where users can enter field values and save without leaving the parent record's detail page.

## What Was Done

### Task 1: Backend Changes (37ca093)

- Created migration `045_add_edit_in_list_to_related_configs.sql` adding `edit_in_list INTEGER DEFAULT 0` column
- Added `EditInList bool` field to `RelatedListConfig` struct with proper JSON and DB tags
- Added `EditInList bool` to `RelatedListConfigCreateInput` struct
- Added `EditInList *bool` to `RelatedListConfigUpdateInput` struct (pointer for optional update)
- Updated all repo methods (ListByEntity, ListEnabledByEntity, GetByID, Create, Update, BulkSave) to handle the new field
- Updated EnsureSchema to auto-add column for existing tenant databases

### Task 2: Frontend RelatedList Component (433459f)

- Added `editInList?: boolean` to TypeScript interfaces
- Added inline editing state variables: `isAddingInline`, `newRecord`, `saving`, `inlineError`
- Modified `handleCreateNew()` to check `config.editInList` and show inline row instead of navigating
- Added `saveInlineRecord()` function that POSTs to `/entities/{entity}/records`
- Added `cancelInline()` function to reset inline editing state
- Added editable row UI with text inputs for each display field
- Styled inline row with light blue background (`bg-blue-50`) for visual distinction
- Added Save/Cancel buttons with proper disabled states during save

### Task 3: Admin Configuration UI (8158119)

- Added `editInList` mapping when loading configs from API
- Added `toggleEditInList()` function to toggle the setting
- Added toggle switch UI between label input and fields button
- Toggle shows current state with blue/gray coloring
- Added tooltip explaining the feature: "When enabled, clicking 'New' creates an inline editable row instead of navigating to a new page"
- Changes saved via existing bulk save endpoint

## Technical Details

### Database Schema

```sql
ALTER TABLE related_list_configs ADD COLUMN edit_in_list INTEGER DEFAULT 0;
```

### API Changes

The existing endpoints automatically support the new field:
- `GET /entities/{entity}/related-list-configs` - Returns `editInList` field
- `PUT /entities/{entity}/related-list-configs` - Accepts `editInList` in bulk save

### Frontend Inline Editing Flow

1. User clicks "+ New" on a related list with `editInList=true`
2. Component sets `isAddingInline=true` and pre-fills lookup field in `newRecord`
3. Editable row appears at top of table with inputs for each display field
4. User fills fields and clicks "Save"
5. Component POSTs to `/entities/{relatedEntity}/records` with newRecord data
6. On success: row disappears, list refreshes, toast shows success
7. On error: row stays open, error shown via toast

## Verification

- Backend compiles successfully with `go build ./...`
- Frontend passes `npm run check` (no new errors from these changes)
- Manual testing flow:
  1. Go to Admin > Entity Manager > [Entity] > Related Lists
  2. Enable "Inline" toggle for a related list
  3. Click "Save Changes"
  4. Navigate to entity detail page
  5. Click "+ New" on the related list
  6. Verify inline row appears instead of navigation
  7. Fill fields and save
  8. Verify record created and appears in list

## Deviations from Plan

None - plan executed exactly as written.
