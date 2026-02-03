---
phase: quick-008
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - fastcrm/backend/internal/handler/generic_entity.go
  - fastcrm/backend/internal/repo/auth.go
  - fastcrm/backend/internal/middleware/auth.go
  - fastcrm/frontend/src/routes/[entity]/[id]/+page.svelte
autonomous: true

must_haves:
  truths:
    - "Record detail views display the name of the user who created the record"
    - "Record detail views display the name of the user who last modified the record"
    - "User names are resolved from user IDs stored in created_by_id and modified_by_id columns"
  artifacts:
    - path: "fastcrm/backend/internal/handler/generic_entity.go"
      provides: "User name resolution for created_by and modified_by"
      contains: "GetUserNamesByIDs"
    - path: "fastcrm/backend/internal/repo/auth.go"
      provides: "Batch user lookup method"
      exports: ["GetUserNamesByIDs"]
  key_links:
    - from: "generic_entity.go"
      to: "auth.go"
      via: "GetUserNamesByIDs lookup"
      pattern: "GetUserNamesByIDs"
---

<objective>
Add user name resolution for created_by and modified_by fields on entity records.

Purpose: Entity records already store `created_by_id` and `modified_by_id` columns, but the frontend displays empty names because the backend returns empty strings. Users need to see WHO created and modified records for accountability and audit purposes.

Output: Backend resolves user IDs to names and returns `createdByName` and `modifiedByName` with actual user names. Frontend "System Information" section displays these names.
</objective>

<context>
@.planning/PROJECT.md
@fastcrm/backend/internal/handler/generic_entity.go
@fastcrm/backend/internal/repo/auth.go
@fastcrm/frontend/src/routes/accounts/[id]/+page.svelte
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add batch user name lookup to auth repo</name>
  <files>fastcrm/backend/internal/repo/auth.go</files>
  <action>
Add a new method `GetUserNamesByIDs` to AuthRepo that accepts a slice of user IDs and returns a map of ID to user name.

```go
// GetUserNamesByIDs returns a map of user ID to full name for the given IDs
// This queries the platform database (users table) to resolve user names
func (r *AuthRepo) GetUserNamesByIDs(ctx context.Context, userIDs []string) (map[string]string, error) {
    if len(userIDs) == 0 {
        return map[string]string{}, nil
    }

    // Build query with placeholders
    placeholders := make([]string, len(userIDs))
    args := make([]interface{}, len(userIDs))
    for i, id := range userIDs {
        placeholders[i] = "?"
        args[i] = id
    }

    query := fmt.Sprintf(`
        SELECT id, first_name, last_name
        FROM users
        WHERE id IN (%s)
    `, strings.Join(placeholders, ","))

    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    result := make(map[string]string)
    for rows.Next() {
        var id, firstName, lastName string
        if err := rows.Scan(&id, &firstName, &lastName); err != nil {
            continue
        }
        // Build full name (same as User.Name() method)
        name := strings.TrimSpace(firstName + " " + lastName)
        if name == "" {
            name = "Unknown User"
        }
        result[id] = name
    }

    return result, nil
}
```

This method queries the platform database where users are stored (not tenant DB).
  </action>
  <verify>Run `go build ./...` in backend directory - compiles without errors</verify>
  <done>AuthRepo has GetUserNamesByIDs method that returns map[userID]userName</done>
</task>

<task type="auto">
  <name>Task 2: Resolve user names in generic entity handler</name>
  <files>fastcrm/backend/internal/handler/generic_entity.go</files>
  <action>
Modify the GenericEntityHandler to resolve user names for created_by_id and modified_by_id.

1. Add authRepo field to GenericEntityHandler struct and update constructor:
```go
type GenericEntityHandler struct {
    defaultDB         db.DBConn
    metadataRepo      *repo.MetadataRepo
    authRepo          *repo.AuthRepo  // NEW: for user name lookups
    tripwireService   TripwireServiceInterface
    validationService ValidationServiceInterface
}

func NewGenericEntityHandler(conn db.DBConn, metadataRepo *repo.MetadataRepo, authRepo *repo.AuthRepo, tripwireService TripwireServiceInterface, validationService ValidationServiceInterface) *GenericEntityHandler {
    return &GenericEntityHandler{
        defaultDB:         conn,
        metadataRepo:      metadataRepo,
        authRepo:          authRepo,
        tripwireService:   tripwireService,
        validationService: validationService,
    }
}
```

2. Create helper function to resolve user names for records:
```go
// resolveUserNames adds createdByName and modifiedByName to records
func (h *GenericEntityHandler) resolveUserNames(ctx context.Context, records []map[string]interface{}) {
    if h.authRepo == nil || len(records) == 0 {
        return
    }

    // Collect unique user IDs
    userIDSet := make(map[string]bool)
    for _, record := range records {
        if id, ok := record["createdById"].(string); ok && id != "" {
            userIDSet[id] = true
        }
        if id, ok := record["modifiedById"].(string); ok && id != "" {
            userIDSet[id] = true
        }
    }

    if len(userIDSet) == 0 {
        return
    }

    // Convert to slice
    userIDs := make([]string, 0, len(userIDSet))
    for id := range userIDSet {
        userIDs = append(userIDs, id)
    }

    // Lookup names
    userNames, err := h.authRepo.GetUserNamesByIDs(ctx, userIDs)
    if err != nil {
        log.Printf("WARNING: Failed to lookup user names: %v", err)
        return
    }

    // Apply names to records
    for _, record := range records {
        if id, ok := record["createdById"].(string); ok && id != "" {
            record["createdByName"] = userNames[id]
        }
        if id, ok := record["modifiedById"].(string); ok && id != "" {
            record["modifiedByName"] = userNames[id]
        }
    }
}
```

3. In List() method, after scanning records into the `records` slice (around line 399), call:
```go
// Resolve user names for created_by and modified_by
h.resolveUserNames(c.Context(), records)
```

4. In Get() method, after building camelRecord (around line 608), call:
```go
// Resolve user names for this single record
h.resolveUserNames(c.Context(), []map[string]interface{}{camelRecord})
```

5. Remove the hardcoded empty string placeholders in the SELECT queries:
   - Line 289-291: Remove `'' as created_by_name, '' as modified_by_name`
   - Line 511-513: Remove `'' as created_by_name, '' as modified_by_name`

   Replace with just `SELECT t.* FROM ...` (the user names will be added by resolveUserNames).

6. Update main.go (or wherever GenericEntityHandler is instantiated) to pass authRepo:
   Find the call to NewGenericEntityHandler and add the authRepo parameter.
  </action>
  <verify>
1. Run `go build ./...` - compiles without errors
2. Run backend with `air` or `go run cmd/api/main.go`
3. Fetch a record: `curl -H "Authorization: Bearer $TOKEN" http://localhost:3000/api/v1/entities/Account/records | jq '.data[0] | {id, createdById, createdByName, modifiedById, modifiedByName}'`
4. Verify createdByName and modifiedByName contain actual user names (not empty strings)
  </verify>
  <done>
- GenericEntityHandler resolves user names for created_by_id and modified_by_id
- API responses include createdByName and modifiedByName with actual user names
- Works for both List and Get endpoints
  </done>
</task>

<task type="auto">
  <name>Task 3: Verify frontend displays user names in System Information</name>
  <files>fastcrm/frontend/src/routes/accounts/[id]/+page.svelte</files>
  <action>
The frontend already has code to display createdByName and modifiedByName in the System Information section (lines 331-342).

Verify the frontend works correctly by:

1. Check that Account type definition includes the new fields (if not, add them):
   In `fastcrm/frontend/src/lib/types/account.ts`, ensure the Account interface has:
   ```typescript
   createdByName?: string;
   modifiedByName?: string;
   ```

2. The display code in accounts/[id]/+page.svelte already handles optional user names:
   ```svelte
   {#if account.createdByName}
       <span class="text-gray-500"> by {account.createdByName}</span>
   {/if}
   ```

3. For the generic entity detail page (`[entity]/[id]/+page.svelte`), ensure it also displays user names in its System Information section. The pattern should match accounts/[id]/+page.svelte.

Note: Most detail pages use the same pattern. Verify Contact, Task, Quote pages also work (they use similar code or the generic route).
  </action>
  <verify>
1. Open browser to http://localhost:5173/accounts/{id}
2. Scroll to "System Information" section
3. Verify "Created" shows date AND user name (e.g., "Jan 15, 2025 by John Smith")
4. Verify "Last Modified" shows date AND user name
5. Test same on Contacts, Tasks, Quotes pages
  </verify>
  <done>
- Frontend System Information sections display user names
- Created shows "by {userName}" after the date
- Modified shows "by {userName}" after the date
  </done>
</task>

</tasks>

<verification>
1. Backend compiles: `cd fastcrm/backend && go build ./...`
2. Start backend: `cd fastcrm/backend && air`
3. API test: Fetch records and verify createdByName/modifiedByName are populated
4. UI test: View any record detail page, verify System Information shows user names
5. Create a new record, verify your name appears as Created By
6. Edit a record, verify your name appears as Modified By
</verification>

<success_criteria>
1. API responses include createdByName and modifiedByName with actual user full names
2. Frontend detail pages display "Created: [date] by [user name]"
3. Frontend detail pages display "Last Modified: [date] by [user name]"
4. Works for all entity types (Account, Contact, Task, Quote, custom entities)
5. No performance regression - user names are batch-loaded efficiently
</success_criteria>

<output>
After completion, create `.planning/quick/008-add-created-modified-by-user-tracking/008-SUMMARY.md`
</output>
