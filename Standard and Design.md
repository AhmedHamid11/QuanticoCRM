# FastCRM Standards and Design Guidelines

## Lookup Fields

### Overview
Lookup fields create relationships between entities. Every lookup field MUST expose three derived fields to enable proper display and navigation.

### Naming Convention
All lookup fields that reference another entity must follow this naming pattern:
- `[fieldName]Id` - Stores the ID of the related record (TEXT)
- `[fieldName]Name` - Stores the display name of the related record (TEXT)
- `[fieldName]Link` - The URL path to the related record (derived at runtime)

### Example
For a Contact entity that links to an Account via field `account`:
- `accountId` - The ID of the linked Account
- `accountName` - The name of the linked Account
- `accountLink` - `/accounts/{accountId}` (derived)

For a Prospect entity that links to an Account via field `companySubmittedTo`:
- `companySubmittedToId` - The ID of the linked Account
- `companySubmittedToName` - The name of the linked Account
- `companySubmittedToLink` - `/accounts/{companySubmittedToId}` (derived)

### Database Schema
For each lookup field, TWO columns are stored in the database:
```sql
-- For field "companySubmittedTo" linking to Account
company_submitted_to_id TEXT,
company_submitted_to_name TEXT DEFAULT ''
```

### Field Definition (field_defs table)
```sql
INSERT INTO field_defs (name, type, link_entity, link_type, link_display_field)
VALUES ('companySubmittedTo', 'link', 'Account', 'belongsTo', 'name');
```

### Go Entity Structure
```go
CompanySubmittedToID   *string `json:"companySubmittedToId" db:"company_submitted_to_id"`
CompanySubmittedToName string  `json:"companySubmittedToName" db:"company_submitted_to_name"`
```

### API Response Format
When fetching a record, the API MUST return all three derived fields for each lookup:
```json
{
  "id": "RecXYZ123",
  "name": "John Doe",
  "companySubmittedToId": "AccABC456",
  "companySubmittedToName": "Acme Corp",
  "companySubmittedToLink": "/accounts/AccABC456"
}
```

### Backend Implementation Requirements
1. **On Record Fetch**: The GenericEntityHandler must:
   - Identify all lookup fields from field_defs (type = 'link')
   - For each lookup field, return `{fieldName}Id`, `{fieldName}Name`, and `{fieldName}Link`
   - The Link is derived as `/{linkedEntityPlural}/{id}`

2. **On Record Create/Update**:
   - Accept `{fieldName}Id` in the input
   - Automatically fetch and store `{fieldName}Name` from the linked entity
   - Or accept both if provided by frontend (for performance)

3. **Table Schema Generation**:
   - When a lookup field is added to an entity, create BOTH columns:
     - `{field_name}_id TEXT`
     - `{field_name}_name TEXT DEFAULT ''`

### Frontend Behavior
- The lookup field UI should allow searching and selecting from available records
- When saved, both the ID and Name are stored
- When viewing, the Name is displayed as a clickable link using the Link value
- The frontend should use `{fieldName}Link` for navigation href

### Frontend Display Component
```svelte
{#if field.type === 'link' && record[`${field.name}Id`]}
  <a href={record[`${field.name}Link`]} class="text-blue-600 hover:underline">
    {record[`${field.name}Name`] || record[`${field.name}Id`]}
  </a>
{:else}
  -
{/if}
```

### Benefits
1. **Performance**: No joins required to display the linked entity name
2. **Consistency**: Predictable field naming across all entities
3. **Simplicity**: Frontend can display names without additional API calls
4. **Navigation**: Direct links to related records without additional computation

### Required for New Entities
All new entities that will be used as lookup targets must have a `name` field that serves as the display field.

---

## Regression Testing

### Overview
All core functionality must have automated regression tests to ensure changes don't break existing features.

### Test Classes Required

#### 1. Custom Entity CRUD Tests
Test file: `backend/internal/handler/generic_entity_test.go`

| Test Case | Description | Priority |
|-----------|-------------|----------|
| `TestCustomEntity_Create` | Create a new custom entity and verify table creation | High |
| `TestCustomEntity_AddField` | Add a field to existing entity, verify ALTER TABLE | High |
| `TestCustomEntity_AddLookupField` | Add lookup field, verify dual columns created | High |
| `TestCustomEntity_CreateRecord` | Create a record in custom entity | High |
| `TestCustomEntity_UpdateRecord` | Update a record including lookup fields | High |
| `TestCustomEntity_GetRecord` | Fetch record, verify all derived fields returned | High |
| `TestCustomEntity_ListRecords` | List records with pagination | Medium |
| `TestCustomEntity_DeleteRecord` | Soft delete a record | Medium |

#### 2. Lookup Field Tests
Test file: `backend/internal/handler/lookup_field_test.go`

| Test Case | Description | Priority |
|-----------|-------------|----------|
| `TestLookup_DualColumnCreation` | Verify `_id` and `_name` columns created | High |
| `TestLookup_DerivedFields` | Verify `Id`, `Name`, `Link` returned in API | High |
| `TestLookup_UpdateWithIdAndName` | Update lookup with both ID and Name | High |
| `TestLookup_LinkGeneration` | Verify link URL format is correct | Medium |
| `TestLookup_NullHandling` | Handle null/empty lookup values | Medium |

#### 3. Layout System Tests
Test file: `backend/internal/handler/layout_test.go`

| Test Case | Description | Priority |
|-----------|-------------|----------|
| `TestLayout_SaveDetailLayout` | Save a detail layout for entity | High |
| `TestLayout_GetDetailLayout` | Retrieve detail layout | High |
| `TestLayout_LayoutAffectsDisplay` | Verify layout controls visible fields | High |
| `TestLayout_EmptyLayout` | Handle missing layout gracefully | Medium |

#### 4. Table Schema Tests
Test file: `backend/internal/handler/schema_test.go`

| Test Case | Description | Priority |
|-----------|-------------|----------|
| `TestSchema_RequiredColumns` | All tables have id, name, created_at, modified_at, modified_by | High |
| `TestSchema_AlterTableAddColumn` | Adding field to existing table works | High |
| `TestSchema_SnakeCaseConversion` | camelCase to snake_case works correctly | Medium |

### Known Regression Issues

| Issue | Description | Date Found | Test to Add |
|-------|-------------|------------|-------------|
| Missing `modified_by` column | Custom entity tables created without `modified_by` column causing update failures | 2026-01-13 | `TestSchema_RequiredColumns` |

### Running Tests
```bash
cd backend
go test ./internal/handler/... -v
```

### Test Coverage Requirements
- All High priority tests must pass before merging
- Minimum 80% code coverage for handler package
- All regression issues must have corresponding test cases
