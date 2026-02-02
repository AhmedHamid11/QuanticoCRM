# Quick Task 004: Fix Related Records 500 Error

## Problem

The `GetRelatedRecords` endpoint was returning 500 errors for custom entity relationships:
```
GET /api/v1/form433ds/RecKGFV8KMN000FZF0/related/Form433dInstallmentIncreaseDecrease | 500
```

The error logs showed healthy database connections but no specific error message, indicating the SQL query was failing silently.

## Root Cause Analysis

The issue was a combination of:

1. **Missing table validation** - No check if the related entity's table actually exists before querying
2. **Missing column validation** - No check if the lookup column exists in the table
3. **Inconsistent table name generation** - Using local `camelToSnake + "s"` instead of canonical `util.GetTableName`
4. **Insufficient error logging** - Errors were generic "failed to count related records" without context

## Changes Made

### File: `backend/internal/handler/related_list.go`

1. **Added `util` import** for consistent table name generation

2. **Replaced inline table name calculation with `util.GetTableName`** (line 385):
   - Before: `tableName := camelToSnake(relatedEntity) + "s"`
   - After: `tableName := util.GetTableName(relatedEntity)`

3. **Added `tableExistsWithDB` function** to check if table exists before querying

4. **Added table existence validation** with informative error message:
   ```go
   if !tableExists {
       return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
           "error": fmt.Sprintf("Related entity table '%s' not found. The entity may not have been fully provisioned.", relatedEntity),
       })
   }
   ```

5. **Improved lookup column discovery logic** - Now uses schema introspection to find actual column:
   - First tries `{field_name}_id` pattern (custom entity standard)
   - Falls back to `{field_name}` directly (legacy patterns)
   - Then tries standard entity pattern

6. **Added column existence validation** with informative error:
   ```go
   if !tableHasColumnWithDB(dbConn, tableName, lookupColumn) {
       return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
           "error": fmt.Sprintf("Lookup column '%s' not found in table '%s'. The relationship may not be configured correctly.", lookupColumn, tableName),
       })
   }
   ```

7. **Added debug logging** to trace:
   - Entity lookup fallback cases
   - Table/column validation results
   - Generated query with arguments

## Testing

- [x] Go build compiles successfully
- [ ] Deployed to production (pending deploy)

## Expected Outcome

After deployment:
1. 500 errors should become 404 (table not found) or 400 (column not found) with clear messages
2. Debug logs will show exact table/column names being used
3. Root cause will be visible in logs for further investigation

## Potential Follow-up

If the table genuinely doesn't exist, the user needs to:
1. Verify the custom entity `Form433dInstallmentIncreaseDecrease` was properly created
2. Check if the entity has fields defined
3. Use "Repair Metadata" in admin panel to re-provision if needed
