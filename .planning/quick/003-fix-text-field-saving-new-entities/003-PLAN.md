---
phase: quick
plan: 003
type: execute
wave: 1
depends_on: []
files_modified:
  - FastCRM/fastcrm/backend/internal/handler/generic_entity.go
  - FastCRM/fastcrm/backend/internal/util/tables.go
autonomous: true

must_haves:
  truths:
    - "Text fields save values when creating records on custom entities"
    - "Text fields save values when updating records on custom entities"
    - "Column sync happens before insert, not just logged"
  artifacts:
    - path: "FastCRM/fastcrm/backend/internal/handler/generic_entity.go"
      provides: "Create and Update handlers for generic entities"
    - path: "FastCRM/fastcrm/backend/internal/util/tables.go"
      provides: "SyncFieldColumns for schema drift fix"
  key_links:
    - from: "generic_entity.go Create/Update"
      to: "SyncFieldColumns"
      via: "function call before SQL execution"
---

<objective>
Fix text field values not saving on custom entities

Purpose: Taxrise reports that when saving values in a new entity (433d object), text fields like firstName are not persisting.

Output: Backend fix ensuring all field values save correctly for custom entities
</objective>

<context>
@.planning/STATE.md
@FastCRM/fastcrm/backend/internal/handler/generic_entity.go
@FastCRM/fastcrm/backend/internal/util/tables.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Debug and identify root cause</name>
  <files>
    FastCRM/fastcrm/backend/internal/handler/generic_entity.go
    FastCRM/fastcrm/backend/internal/util/tables.go
  </files>
  <action>
Add debug logging to identify why text fields are not saving:

1. In `generic_entity.go` Create handler (around line 668):
   - Log the incoming `body` map to see what fields are received
   - Log each field being processed in the loop (line 698)
   - Log whether the field value is found in body: `log.Printf("Processing field %s (type=%s): found=%v, val=%v", field.Name, field.Type, ok, val)`

2. In `generic_entity.go` Create handler after field processing:
   - Log the final columns and values before INSERT
   - Log: `log.Printf("INSERT columns: %v", columns)`
   - Log: `log.Printf("INSERT values: %v", values)`

3. In `util/tables.go` SyncFieldColumns:
   - Log when checking for missing columns
   - Log: `log.Printf("SyncFieldColumns: checking field %s (type=%s), snakeName=%s, exists=%v", field.Name, field.Type, snakeName, existingCols[snakeName])`

4. Check the specific issue: The loop in Create (line 697-758) only adds fields that exist in the body. But looking at line 747:
   ```go
   if val, ok := body[field.Name]; ok {
   ```
   This checks for exact field name match. Verify that:
   - Frontend sends `firstName` (camelCase)
   - Field def has `Name: "firstName"` (camelCase)
   - Column is `first_name` (snake_case via CamelToSnake)

Run backend locally and test creating a record on a custom entity to see debug output.
  </action>
  <verify>
    Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./... && echo "Build successful"` to verify code compiles
  </verify>
  <done>Debug logging added, build passes, and root cause identified from log output</done>
</task>

<task type="auto">
  <name>Task 2: Fix the text field saving issue</name>
  <files>
    FastCRM/fastcrm/backend/internal/handler/generic_entity.go
    FastCRM/fastcrm/backend/internal/util/tables.go
  </files>
  <action>
Based on code analysis, the likely issues are:

**Issue 1: SyncFieldColumns not blocking on errors**
In `generic_entity.go` Create handler (line 675-679), column sync errors are only logged, not returned:
```go
if columnsAdded, syncErr := util.SyncFieldColumns(...); syncErr != nil {
    log.Printf("WARNING: Failed to sync columns for %s: %v", entityName, syncErr)
} else if columnsAdded > 0 {
    log.Printf("INFO: Added %d missing columns to %s table", columnsAdded, tableName)
}
```
This means if SyncFieldColumns fails, the Insert proceeds without the columns.

**Fix 1**: Return error if SyncFieldColumns fails:
```go
if columnsAdded, syncErr := util.SyncFieldColumns(c.Context(), h.getDB(c), entityName, fields); syncErr != nil {
    log.Printf("ERROR: Failed to sync columns for %s: %v", entityName, syncErr)
    return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
        "error": fmt.Sprintf("Failed to prepare table schema: %v", syncErr),
    })
}
```

**Issue 2: Table may not exist at all**
Before SyncFieldColumns is called in Create, we call `ensureTableExists` (line 654-657). If the table doesn't exist and is created fresh, SyncFieldColumns should work. But if the table exists with missing columns and SyncFieldColumns fails, we proceed anyway.

**Fix 2**: Make SyncFieldColumns more robust - check if the error is about table not existing vs column issues.

**Issue 3: Field names might not match**
Check if the field definitions in the database have the correct names. The frontend sends camelCase, and we look up `body[field.Name]`. If `field.Name` is stored differently (e.g., snake_case), the lookup will fail silently.

**Verify field storage**: Query field_defs to confirm field names are stored in camelCase.

Apply fixes and remove excessive debug logging, keeping only essential error logging.
  </action>
  <verify>
    Run `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go test ./internal/handler/... -run GenericEntity -v` to run relevant tests
  </verify>
  <done>
    Fix applied:
    - SyncFieldColumns errors properly returned to client instead of silently logged
    - Backend tests pass
    - Text fields save correctly on custom entities
  </done>
</task>

<task type="auto">
  <name>Task 3: Verify fix and clean up</name>
  <files>
    FastCRM/fastcrm/backend/internal/handler/generic_entity.go
  </files>
  <action>
1. Remove any verbose debug logging added in Task 1 (keep essential error logs)

2. Ensure the same fix is applied to:
   - `Create` handler (line ~632)
   - `Update` handler (line ~806)
   - `upsertCreate` helper (line ~1229)
   - `upsertUpdate` helper (line ~1349)

3. Test end-to-end:
   - Start backend: `cd backend && air` (or `go run cmd/api/main.go`)
   - Create a custom entity via admin UI or API
   - Add a text field to the entity
   - Create a record with a value for that field
   - Verify the value persists when fetching the record

4. Commit the fix with descriptive message
  </action>
  <verify>
    `cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend && go build ./... && go test ./... -count=1` - all tests pass
  </verify>
  <done>
    Fix complete:
    - Text fields save correctly on new custom entities
    - Schema sync errors properly surfaced to users
    - All backend tests pass
    - Changes committed
  </done>
</task>

</tasks>

<verification>
1. Backend compiles without errors
2. All existing tests pass
3. Text field values save on custom entities (verified via API or UI)
4. Schema sync errors return proper error response instead of silently failing
</verification>

<success_criteria>
- Text fields on custom entities save values when creating records
- Text fields on custom entities save values when updating records
- Schema sync errors return HTTP 500 with descriptive message
- All backend tests pass
</success_criteria>

<output>
After completion, create `.planning/quick/003-fix-text-field-saving-new-entities/003-SUMMARY.md`
</output>
