# FastCRM Bug Log

## Open Issues

### FCRM-002: Related List Error - "no such column: account_id_id"
**Ticket:** FCRM-002
**Date Opened:** 2026-01-14
**Status:** RESOLVED (2026-01-14)
**Priority:** High
**Severity:** Blocker (prevents viewing related contacts on accounts)

---

#### Summary
When viewing an Account detail page, the Contacts related list shows the error:
"failed to count related records: no such column: account_id_id"

#### Steps to Reproduce
1. Navigate to http://localhost:5173/accounts/001ACC000000010
2. Observe the Contacts related list section
3. **Expected:** Contact list should load showing related contacts
4. **Actual:** Error message "failed to count related records: no such column: account_id_id"

#### Technical Details

**Affected File:**
`backend/internal/handler/related_list.go`

**Root Cause Analysis:**
In the `GetRelatedRecords` handler, when building SQL queries for single lookup fields:
1. The lookup field name `accountId` is converted to snake_case: `account_id`
2. The code then appends `_id` again: `account_id` + `_id` = `account_id_id`
3. The actual database column is `account_id` (not `account_id_id`)

**Code Location (Lines 299-315 and 367-392):**
```go
// For single lookup fields, the column name has _id suffix
lookupColumn := snakeCaseLookupField + "_id"  // BUG: account_id becomes account_id_id
```

**Fix Applied:**
Added check to avoid double `_id` suffix:
```go
lookupColumn := snakeCaseLookupField
if !strings.HasSuffix(lookupColumn, "_id") {
    lookupColumn = lookupColumn + "_id"
}
```

**Resolution:** Fix verified working. Related list now returns contacts correctly.

#### Related Files
- `backend/internal/handler/related_list.go` - Query building logic
- `backend/internal/entity/contact.go` - Contact entity with `account_id` field
- `frontend/src/lib/components/RelatedList.svelte` - Frontend component

#### Assignee
Unassigned

#### Labels
`bug`, `backend`, `related-list`, `blocker`

---

### FCRM-003: Rollup Field Values Show Dash Instead of Computed Value
**Ticket:** FCRM-003
**Date Opened:** 2026-01-14
**Status:** RESOLVED (2026-01-14)
**Priority:** High
**Severity:** Major (rollup fields non-functional on standard entities)

---

#### Summary
Rollup fields on Account records display "-" instead of the computed rollup value. The `test_rollup` field on Account shows a dash even though rollup logic exists in the generic entity handler.

#### Steps to Reproduce
1. Navigate to http://localhost:5173/accounts/001ACC000000010
2. Observe the `test_rollup` field value
3. **Expected:** Computed rollup value should display
4. **Actual:** Shows "-" (dash)

#### Technical Details

**Issue 1: AccountHandler missing rollup execution (FIXED)**

**Affected File:**
`backend/internal/handler/account.go`

**Root Cause Analysis:**
The `AccountHandler.Get()` method returns account data directly from the repository without executing rollup fields. Unlike `GenericEntityHandler` which has rollup execution logic, the standard entity handlers (Account, Contact, etc.) do not process rollup fields.

**Contrast with GenericEntityHandler (Lines 518-533):**
```go
// Execute rollup fields
rollupSvc := service.NewRollupService(h.db)
for i := range fields {
    if fields[i].Type == entity.FieldTypeRollup {
        // ... executes rollup and adds to response
    }
}
```

**AccountHandler (Original - No rollup logic):**
```go
func (h *AccountHandler) Get(c *fiber.Ctx) error {
    account, err := h.repo.GetByID(c.Context(), orgID, id)
    // ... no rollup execution
    return c.JSON(account)  // Returns without rollup values
}
```

**Fix Applied:**
Added rollup execution to AccountHandler.Get():
1. Added `db` and `metadataRepo` dependencies to `AccountHandler`
2. Updated `NewAccountHandler` constructor signature
3. Added rollup execution logic that converts account to map and adds rollup values

**Resolution:** Backend now executes rollup queries and returns results. API response now includes `test_rollup` and `test_rollupError` fields.

---

**Issue 2: Rollup query uses incorrect table/column names (FIXED - data corrected)**

After fixing Issue 1, the API returned:
```json
{
  "test_rollup": null,
  "test_rollupError": "rollup query execution failed: no such table: Contact"
}
```

**Original rollup query in database:**
```sql
SELECT COUNT(*) FROM Contact WHERE accountId = '{{id}}'
```

**Problems identified:**
1. Table name `Contact` should be `contacts` (lowercase, plural)
2. Column name `accountId` should be `account_id` (snake_case)

**Resolution:** Updated via admin API to: `SELECT COUNT(*) FROM contacts WHERE account_id = '{{id}}'`

---

**Issue 3: Rollup field update API not saving rollup fields (FIXED)**

The `FieldDefUpdateInput` struct was missing rollup-related fields, preventing rollup queries from being updated via the API.

**Fix Applied:**
Added to `entity/metadata.go` FieldDefUpdateInput:
```go
RollupQuery         *string  `json:"rollupQuery,omitempty"`
RollupResultType    *string  `json:"rollupResultType,omitempty"`
RollupDecimalPlaces *int     `json:"rollupDecimalPlaces,omitempty"`
```

Updated `repo/metadata.go` UpdateField() to handle these new fields in the SQL UPDATE statement.

---

**Issue 4: Rollup query placeholder inside quotes breaks parameter binding (DATA GUIDANCE)**

When rollup queries are created with the `{{id}}` placeholder inside quotes like:
```sql
SELECT COUNT(*) FROM contacts WHERE account_id = '{{id}}'
```

The code replaces `{{id}}` with `?` for parameterized queries, resulting in:
```sql
SELECT COUNT(*) FROM contacts WHERE account_id = '?'
```

This treats `'?'` as a literal string, not a parameter placeholder!

**Correct format (no quotes around placeholder):**
```sql
SELECT COUNT(*) FROM contacts WHERE account_id = {{id}}
```

**Recommendation:** Add validation in rollup field creation to warn/reject queries with `'{{id}}'` (quoted placeholder)

#### Related Files
- `backend/internal/handler/account.go` - Account handler (missing rollup logic)
- `backend/internal/handler/generic_entity.go` - Has working rollup logic
- `backend/internal/service/rollup.go` - Rollup execution service
- `backend/cmd/api/main.go` - Handler initialization

#### Assignee
Unassigned

#### Labels
`bug`, `backend`, `rollup`, `standard-entities`

---

### FCRM-001: Rollup Field Type - UI Conditional Rendering Not Working
**Ticket:** FCRM-001
**Date Opened:** 2026-01-13
**Status:** OPEN - Unresolved
**Priority:** High
**Severity:** Blocker (prevents rollup field creation via UI)

---

#### Summary
When selecting "Rollup" from the field type dropdown in the Add Field modal, the rollup-specific configuration fields (Result Type dropdown, SQL Query textarea) do not appear. Instead, the "Max Length" field (which should only appear for varchar type) incorrectly displays.

#### Steps to Reproduce
1. Navigate to Administration > Entity Manager > [Any Entity] > Fields
2. Click "+ Add Field" button
3. Enter a Label (e.g., "Test Rollup")
4. From the "Type" dropdown, select "Rollup"
5. **Expected:** Result Type dropdown, Decimal Places input, and SQL Query textarea should appear
6. **Actual:** Max Length input field appears instead (this is the varchar field)

#### Environment
- **Framework:** SvelteKit with Svelte 5.46.1
- **Vite Plugin:** vite-plugin-svelte@3 (warning suggests upgrade to @4)
- **Browser:** [All browsers tested]
- **Backend:** Go Fiber v2.52.0

#### Technical Details

**Affected File:**
`frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte`

**Root Cause Analysis:**
Svelte 5's fine-grained reactivity system with `$state` objects does not automatically trigger `{#if}` conditional block re-evaluation when nested properties change via `bind:value`. The issue occurs because:

1. `newField` is declared as `$state<FieldDefCreateInput>({...})`
2. The select dropdown uses `bind:value={newField.type}`
3. When `newField.type` changes, Svelte's fine-grained reactivity updates the property
4. However, `{#if newField.type === 'rollup'}` blocks do not re-evaluate

**Attempted Fixes (All Failed):**

| # | Approach | Code | Result |
|---|----------|------|--------|
| 1 | Object reassignment on change | `onchange={() => { newField = { ...newField }; }}` | No effect |
| 2 | Separate state variable | `let selectedFieldType = $state('varchar')` | No effect |
| 3 | Derived state | `let selectedFieldType = $derived(newField.type)` | No effect |
| 4 | {#key} block wrapper | `{#key newField.type}...{/key}` | No effect |

**Current Code State (Line 437-580):**
```svelte
<select
    id="fieldType"
    bind:value={newField.type}
    onchange={() => { newField = { ...newField }; }}
    class="..."
>
    {#each fieldTypes as type}
        <option value={type.name}>{type.label} - {type.description}</option>
    {/each}
</select>

{#key newField.type}
    <!-- Rollup configuration - DOES NOT APPEAR -->
    {#if newField.type === 'rollup'}
        <div><!-- Result Type, SQL Query fields --></div>
    {/if}

    <!-- Max length for varchar - INCORRECTLY APPEARS -->
    {#if newField.type === 'varchar'}
        <div><!-- Max Length field --></div>
    {/if}
{/key}
```

#### Possible Causes to Investigate

1. **Vite Plugin Version Mismatch:** Console shows warning about vite-plugin-svelte@3 with Svelte 5.46.1 - may need upgrade to @4
2. **SSR/Hydration Issue:** Possible mismatch between server-rendered HTML and client-side state
3. **Hot Module Replacement (HMR) Issue:** State may not be properly resetting during development
4. **Browser Caching:** Old compiled code may be cached
5. **Field Types API Response:** The `fieldTypes` array may not include 'rollup' type from backend

#### Debugging Steps Needed

1. [ ] Add `console.log(newField.type)` in onchange handler to verify value is changing
2. [ ] Check browser DevTools for `newField` state in Svelte DevTools extension
3. [ ] Verify `/api/v1/admin/field-types` endpoint returns 'rollup' in response
4. [ ] Try hard refresh (Cmd+Shift+R) to clear all caches
5. [ ] Test in incognito/private browser window
6. [ ] Upgrade vite-plugin-svelte to @4 as suggested by console warning
7. [ ] Check if issue occurs with other field types (enum, link, etc.)

#### Workaround
Currently none. Rollup fields cannot be created via the UI. As a temporary workaround, rollup fields can be created via direct API call:

```bash
curl -X POST http://localhost:8080/api/v1/admin/entities/{entity}/fields \
  -H "Content-Type: application/json" \
  -d '{
    "name": "contact_count",
    "label": "Contact Count",
    "type": "rollup",
    "rollupResultType": "numeric",
    "rollupQuery": "SELECT COUNT(*) FROM contacts WHERE account_id = '\''{{id}}'\''",
    "rollupDecimalPlaces": 0
  }'
```

#### Related Files
- `frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte` - Main UI
- `frontend/src/lib/types/admin.ts` - TypeScript types (includes rollup fields)
- `backend/internal/entity/metadata.go` - Backend entity types
- `backend/internal/handler/admin.go` - Backend validation
- `backend/internal/service/rollup.go` - Rollup service

#### Assignee
Unassigned

#### Labels
`bug`, `frontend`, `svelte5`, `reactivity`, `blocker`

---

## Resolved Issues

### FCRM-005: Jobs Related List Error - "no such column: org_id"
**Ticket:** FCRM-005
**Date Opened:** 2026-01-20
**Status:** RESOLVED (2026-01-20)
**Priority:** High
**Severity:** Blocker (prevents viewing related jobs on contacts)

---

#### Summary
When viewing a Contact detail page with a Jobs related list configured, the error "failed to count related records: no such column: org_id" appears.

#### Steps to Reproduce
1. Navigate to http://localhost:5173/contacts/003CON000000019
2. Observe the Jobs related list section
3. **Expected:** Jobs list should load showing related jobs
4. **Actual:** Error message "failed to count related records: no such column: org_id"

#### Technical Details

**Root Cause Analysis (3 issues):**

1. **Custom entity column naming differs from standard entities:**
   - Standard entities: field `accountId` → column `account_id`
   - Custom entities: field `hiringManagerId` → column `hiring_manager_id_id`
   - The code was using standard entity naming for all entities

2. **Custom entities don't have `org_id` or `deleted` columns:**
   - The query was including `WHERE org_id = ? AND deleted = 0` for all entities
   - Custom entities don't have these columns

3. **Cross-org entity definition lookup failed:**
   - Job entity defined in org `00DKFC4GC5S000CA70`
   - User viewing contact in org `00DKFBKQG1G000E4VG`
   - `GetEntity(orgID, "Job")` returned nil, so `isCustomEntity` stayed false

**Affected File:**
`backend/internal/handler/related_list.go`

**Fix Applied:**

1. Updated column naming logic to handle custom entities differently:
```go
lookupColumn := snakeCaseLookupField
if isCustomEntity {
    // Custom entities always use {snake_name}_id column naming
    lookupColumn = snakeCaseLookupField + "_id"
}
// For standard entities, if the field doesn't end with "Id", append "_id"
if !isCustomEntity && !strings.HasSuffix(lookupField, "Id") {
    lookupColumn = snakeCaseLookupField + "_id"
}
```

2. Added `tableHasColumn()` helper to check table schema directly:
```go
func (h *RelatedListHandler) tableHasColumn(tableName, columnName string) bool {
    query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", tableName)
    var count int
    err := h.db.QueryRow(query, columnName).Scan(&count)
    return err == nil && count > 0
}
```

3. Added fallback for cross-org entity detection:
```go
relatedEntityDef, err := h.metadataRepo.GetEntity(c.Context(), orgID, relatedEntity)
if err == nil && relatedEntityDef != nil {
    isCustomEntity = relatedEntityDef.IsCustom
} else {
    // Entity def not found in current org - check if table has org_id column
    hasOrgID := h.tableHasColumn(tableName, "org_id")
    isCustomEntity = !hasOrgID
}
```

**Resolution:** Jobs related list now loads correctly for custom entities.

#### Related Files
- `backend/internal/handler/related_list.go` - Query building logic with column detection
- `backend/internal/repo/metadata.go` - Entity definition lookup

#### Labels
`bug`, `backend`, `related-list`, `custom-entities`, `blocker`

---

### FCRM-004: Contacts Related List - Standard Entity Column Naming
**Ticket:** FCRM-004
**Date Opened:** 2026-01-20
**Status:** RESOLVED (2026-01-20)
**Priority:** High
**Severity:** Blocker (prevents viewing related contacts on accounts)

---

#### Summary
Follow-up to FCRM-002. The original fix checked if column name ended with `_id`, but this didn't account for the field name (camelCase) vs column name (snake_case) difference properly.

#### Technical Details

**Root Cause:**
The original fix checked `!strings.HasSuffix(lookupColumn, "_id")` but `lookupColumn` was already converted to snake_case. The check should be on the original field name to determine if it already includes "Id".

**Example:**
- Field: `accountId`
- After snake_case: `account_id`
- Original fix checked: `account_id` ends with `_id` → true → don't append
- This worked by coincidence, but the logic was fragile

**Fix Applied:**
Changed to check the original field name instead:
```go
// For standard entities, if the field doesn't end with "Id", append "_id"
if !isCustomEntity && !strings.HasSuffix(lookupField, "Id") {
    lookupColumn = snakeCaseLookupField + "_id"
}
```

**Resolution:** Contacts related list works correctly with proper column naming.

#### Labels
`bug`, `backend`, `related-list`, `standard-entities`

---

### 2. Cancel Button in Add Field Modal Not Working
**Date:** 2026-01-13
**Status:** Resolved (2026-01-14)
**Priority:** Medium

**Description:**
The Cancel button in the "Add New Field" modal does not close the modal when clicked.

**Root Cause:**
The `closeModals` function reference was not being invoked properly in Svelte 5.

**Fix:**
1. Added `type="button"` attribute to prevent form submission behavior
2. Changed from `onclick={closeModals}` to inline arrow function `onclick={() => { showAddModal = false; }}`
3. Applied same fix to Edit modal Cancel button

**Files Modified:**
- `frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte`

---

### 1. API Name Field Not Auto-Generating from Label
**Date:** 2026-01-13
**Status:** Resolved

**Description:**
When typing in the Label field, the API Name field was not automatically populating with an underscore-separated version.

**Fix:**
Changed from `value={newField.label}` to `bind:value={newField.label}` for proper two-way binding in Svelte 5.

**Files Modified:**
- `frontend/src/routes/admin/entity-manager/[entity]/fields/+page.svelte`
