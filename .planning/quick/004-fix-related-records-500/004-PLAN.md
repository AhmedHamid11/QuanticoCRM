# Quick Task 004: Fix Related Records 500 Error

## Problem Analysis

The `GetRelatedRecords` endpoint returns 500 errors for custom entity relationships:
```
GET /api/v1/form433ds/RecKGFV8KMN000FZF0/related/Form433dInstallmentIncreaseDecrease | 500
```

### Root Cause

In `related_list.go` at lines 467-484, the lookup column naming logic has a bug:

```go
lookupColumn := snakeCaseLookupField
customColumn := snakeCaseLookupField + "_id"

if isCustomEntity {
    lookupColumn = customColumn
}
```

When a custom entity has a link field named `form433DId`:
1. `snakeCaseLookupField = "form433_d_id"` (already has `_id` suffix from field name)
2. `customColumn = "form433_d_id_id"` (double `_id` - WRONG)
3. Query fails because column `form433_d_id_id` doesn't exist

The actual column in the database is `form433_d_id` (created by `EnsureTableExists` which adds `_id` to the snake_case field name).

### Solution

Fix the column naming logic to avoid double `_id` suffix:
- If field name already ends in `Id`, the snake_case version already ends in `_id`
- Don't append another `_id` in this case

## Tasks

### Task 1: Fix lookup column naming logic

**File:** `backend/internal/handler/related_list.go`

**Location:** Lines 467-484 (first occurrence) and Lines 572-585 (second occurrence - same logic duplicated)

**Change:** Add check to avoid double `_id` suffix:

```go
// Determine column name: custom entities use {snake_name}_id pattern
// BUT if the field already ends in "Id" (e.g., "form433DId"),
// the snake_case version already ends in "_id", so don't double it
lookupColumn := snakeCaseLookupField
if isCustomEntity {
    if !strings.HasSuffix(snakeCaseLookupField, "_id") {
        lookupColumn = snakeCaseLookupField + "_id"
    }
    // else: already ends in _id, use as-is
} else if tableHasColumnWithDB(dbConn, tableName, snakeCaseLookupField + "_id") {
    // Schema shows custom naming pattern - use it even if metadata lookup failed
    if !strings.HasSuffix(snakeCaseLookupField, "_id") {
        lookupColumn = snakeCaseLookupField + "_id"
    }
} else if !strings.HasSuffix(lookupField, "Id") {
    // Standard entity without "Id" suffix - append "_id"
    lookupColumn = snakeCaseLookupField + "_id"
}
```

## Acceptance Criteria

- [ ] Related records endpoint returns 200 for custom entities with `*Id` named link fields
- [ ] Both occurrences of the column naming logic are fixed (count query and select query)
- [ ] Existing functionality for standard entities is preserved
