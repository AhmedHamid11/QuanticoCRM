---
phase: quick-012
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - backend/internal/handler/import.go
  - backend/internal/service/csv_validator.go
  - frontend/src/routes/admin/import/+page.svelte
  - frontend/src/lib/components/ImportWizard.svelte
autonomous: true

must_haves:
  truths:
    - "User can select target entity for import"
    - "User can upload CSV and see auto-mapped columns"
    - "User can manually remap CSV columns to entity fields"
    - "System validates data types and enum values before import"
    - "User sees validation errors with row/column details"
    - "User can proceed to import after successful validation"
    - "Imported records appear in entity list view"
  artifacts:
    - path: "backend/internal/service/csv_validator.go"
      provides: "CSV validation logic for types, enums, required fields"
      min_lines: 100
    - path: "backend/internal/handler/import.go"
      provides: "Analyze endpoint for pre-import validation"
      contains: "AnalyzeCSV"
    - path: "frontend/src/routes/admin/import/+page.svelte"
      provides: "Import wizard page"
      min_lines: 50
    - path: "frontend/src/lib/components/ImportWizard.svelte"
      provides: "Multi-step import wizard component"
      min_lines: 200
  key_links:
    - from: "frontend/src/lib/components/ImportWizard.svelte"
      to: "/api/v1/entities/:entity/import/csv/analyze"
      via: "fetch POST for validation"
      pattern: "fetch.*analyze"
    - from: "frontend/src/lib/components/ImportWizard.svelte"
      to: "/api/v1/entities/:entity/import/csv"
      via: "fetch POST for actual import"
      pattern: "fetch.*import/csv"
---

<objective>
Build CSV Import wizard with field mapping and pre-import validation.

Purpose: Allow users to bulk import data from CSV files into any entity with data type validation, enum validation, and clear error reporting before committing the import.

Output: Working 3-step import wizard (Upload & Map -> Validate -> Import) accessible from Admin panel.
</objective>

<context>
**Existing Backend Infrastructure (already built):**
- `backend/internal/service/csv_parser.go` - CSV parsing with auto header mapping
- `backend/internal/handler/import.go` - Import handler with create/update/upsert/delete modes
- Preview endpoint: `POST /entities/:entity/import/csv/preview`
- Import endpoint: `POST /entities/:entity/import/csv` with `validateOnly` option
- Entity field definitions available via `ListFields()`

**What's missing:**
1. Dedicated analyze endpoint that validates: enum values match options, data types parse correctly, required fields mapped
2. Frontend UI - no import wizard exists yet

**Field types to validate:**
- `enum`: value must be in field.Options
- `multiEnum`: all values must be in field.Options
- `int`: must parse as integer
- `float`/`currency`: must parse as float
- `bool`: must be true/false/1/0/yes/no
- `date`/`datetime`: must parse as valid date format
- `email`: basic email format validation
- `url`: basic URL format validation
- Required fields: must be mapped and non-empty

**Security:**
- Sanitize all string values (no HTML/script injection)
- Max file size: 50MB (already enforced)
- Max rows: 10,000 (already enforced)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create CSV Validator Service</name>
  <files>backend/internal/service/csv_validator.go</files>
  <action>
Create a new CSVValidatorService that validates parsed CSV records against entity field definitions.

**Struct:**
```go
type CSVValidatorService struct{}

type ValidationIssue struct {
    Row       int    `json:"row"`
    Column    string `json:"column"`
    FieldName string `json:"fieldName"`
    Value     string `json:"value"`
    IssueType string `json:"issueType"` // "invalid_enum", "invalid_type", "required_missing", "invalid_format"
    Message   string `json:"message"`
    Expected  string `json:"expected,omitempty"` // e.g., valid enum options
}

type AnalyzeResult struct {
    Valid        bool              `json:"valid"`
    TotalRows    int               `json:"totalRows"`
    ValidRows    int               `json:"validRows"`
    InvalidRows  int               `json:"invalidRows"`
    Issues       []ValidationIssue `json:"issues"`
    MappedFields []string          `json:"mappedFields"`   // Fields that have data
    MissingRequired []string       `json:"missingRequired"` // Required fields not mapped
}
```

**Validate method:**
```go
func (v *CSVValidatorService) Validate(records []map[string]interface{}, fields []entity.FieldDef) *AnalyzeResult
```

**Validation logic per field type:**
- `enum`: Check value is in JSON-parsed options array
- `multiEnum`: Split on comma, check each value in options
- `int`: Use strconv.Atoi, report error if fails
- `float`/`currency`: Use strconv.ParseFloat, report error if fails
- `bool`: Accept true/false/1/0/yes/no (case-insensitive)
- `date`: Accept YYYY-MM-DD format
- `datetime`: Accept YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS formats
- `email`: Use regexp `^[^\s@]+@[^\s@]+\.[^\s@]+$`
- `url`: Use url.Parse and check scheme is http/https
- Required check: If field.IsRequired and not mapped or empty value

**Sanitization (apply to all string values):**
- Strip HTML tags using regexp
- Escape any remaining < > & characters
- Trim whitespace

Return issues array with row numbers (1-indexed, excluding header).
  </action>
  <verify>
Add test in `backend/tests/csv_validator_test.go`:
- Test valid enum values pass
- Test invalid enum values fail with correct message
- Test int validation catches "abc" in int field
- Test required field missing detection
- Test email format validation
- Run: `cd backend && go test ./tests/... -run TestCSVValidator -v`
  </verify>
  <done>
CSVValidatorService exists with Validate method that checks all field types, returns structured issues with row/column/message details.
  </done>
</task>

<task type="auto">
  <name>Task 2: Add Analyze Endpoint to Import Handler</name>
  <files>backend/internal/handler/import.go, backend/cmd/api/main.go</files>
  <action>
Add new endpoint to import.go:

**Handler method:**
```go
// AnalyzeCSV handles POST /api/v1/entities/:entity/import/csv/analyze
func (h *ImportHandler) AnalyzeCSV(c *fiber.Ctx) error
```

**Logic:**
1. Parse uploaded CSV file (same as ImportCSV)
2. Get entity fields from metadata repo
3. Call CSVValidatorService.Validate(records, fields)
4. Return AnalyzeResult JSON

**Add to ImportHandler struct:**
```go
csvValidator *service.CSVValidatorService
```

**Register route in RegisterRoutes:**
```go
app.Post("/entities/:entity/import/csv/analyze", h.AnalyzeCSV)
```

**Update NewImportHandler to accept csvValidator parameter** or instantiate internally.

Also update `cmd/api/main.go` to wire up the validator service if needed.
  </action>
  <verify>
Test with curl:
```bash
# Create test CSV
echo -e "firstName,lastName,status\nJohn,Doe,Active\nJane,Smith,Invalid" > /tmp/test.csv

# Call analyze endpoint (adjust auth token)
curl -X POST http://localhost:8080/api/v1/entities/Contact/import/csv/analyze \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/tmp/test.csv"
```

Should return JSON with issues array showing "Invalid" is not a valid status option.
  </verify>
  <done>
AnalyzeCSV endpoint exists, returns validation issues for invalid data types/enums/required fields.
  </done>
</task>

<task type="auto">
  <name>Task 3: Create Import Wizard UI Component</name>
  <files>frontend/src/lib/components/ImportWizard.svelte</files>
  <action>
Create a multi-step import wizard component with 3 steps.

**Props:**
```typescript
export let entityName: string;
export let onComplete: () => void;
export let onCancel: () => void;
```

**State:**
```typescript
let step = 1; // 1=Upload, 2=Validate, 3=Import
let file: File | null = null;
let previewData: PreviewResponse | null = null;
let columnMapping: Record<string, string> = {};
let analyzeResult: AnalyzeResult | null = null;
let importResult: ImportResponse | null = null;
let loading = false;
let error = '';
```

**Step 1 - Upload & Map:**
- File input for CSV upload
- On file select, call `/entities/{entity}/import/csv/preview`
- Show table: CSV Header | Sample Data | Maps To (dropdown of entity fields)
- Auto-select mapped fields from previewData.mappedHeaders
- Allow manual remapping via dropdowns
- "Analyze" button to proceed

**Step 2 - Validate:**
- On mount, POST to `/entities/{entity}/import/csv/analyze` with file and columnMapping
- Show validation summary: X rows valid, Y rows with issues
- If issues, show table: Row | Column | Value | Issue | Expected
- "Back" button to return to mapping
- "Import" button (disabled if issues exist, or show warning)

**Step 3 - Import:**
- POST to `/entities/{entity}/import/csv` with file, columnMapping, mode=create
- Show progress/spinner during import
- On complete: Show success message with counts (created, failed)
- Show any import errors
- "Done" button calls onComplete()

**Styling:**
- Use existing Tailwind patterns from codebase
- Step indicator at top (1-2-3 with current highlighted)
- Card-based layout for each step
- Error states in red, success in green
  </action>
  <verify>
Component compiles without TypeScript errors:
`cd frontend && npm run check`

Visual verification will happen in Task 4.
  </verify>
  <done>
ImportWizard.svelte exists with 3-step flow: upload/map -> validate -> import, with proper state management and API calls.
  </done>
</task>

<task type="auto">
  <name>Task 4: Create Admin Import Page</name>
  <files>frontend/src/routes/admin/import/+page.svelte</files>
  <action>
Create the admin import page that uses the ImportWizard component.

**Page structure:**
```svelte
<script>
  import { goto } from '$app/navigation';
  import ImportWizard from '$lib/components/ImportWizard.svelte';
  import { api } from '$lib/api';

  let selectedEntity = '';
  let entities: EntityDef[] = [];
  let showWizard = false;

  onMount(async () => {
    // Fetch available entities
    const res = await api.get('/admin/entities');
    entities = await res.json();
  });

  function startImport() {
    if (selectedEntity) showWizard = true;
  }

  function handleComplete() {
    showWizard = false;
    // Optionally navigate to the entity list
    goto(`/${selectedEntity.toLowerCase()}s`);
  }

  function handleCancel() {
    showWizard = false;
    selectedEntity = '';
  }
</script>

<div class="p-6">
  <h1 class="text-2xl font-bold mb-6">Import Data</h1>

  {#if !showWizard}
    <div class="max-w-md">
      <label class="block mb-2 font-medium">Select Entity</label>
      <select bind:value={selectedEntity} class="w-full border rounded p-2 mb-4">
        <option value="">Choose entity...</option>
        {#each entities as entity}
          <option value={entity.name}>{entity.labelPlural || entity.label}</option>
        {/each}
      </select>

      <button
        onclick={startImport}
        disabled={!selectedEntity}
        class="bg-blue-600 text-white px-4 py-2 rounded disabled:opacity-50"
      >
        Start Import
      </button>
    </div>
  {:else}
    <ImportWizard
      entityName={selectedEntity}
      onComplete={handleComplete}
      onCancel={handleCancel}
    />
  {/if}
</div>
```

**Add to admin navigation:**
If admin layout has navigation, add "Import" link pointing to `/admin/import`.
  </action>
  <verify>
1. Start frontend dev server: `cd frontend && npm run dev`
2. Navigate to http://localhost:5173/admin/import
3. Verify entity dropdown populates
4. Select an entity, click Start Import
5. Upload a CSV file
6. Verify columns map and sample data shows
7. Click Analyze and verify validation runs
8. If valid, click Import and verify records are created
  </verify>
  <done>
Admin import page exists at /admin/import, entity selection works, wizard flows through all 3 steps, records are successfully imported.
  </done>
</task>

</tasks>

<verification>
1. Backend tests pass: `cd backend && go test ./tests/... -v`
2. Frontend compiles: `cd frontend && npm run check && npm run build`
3. End-to-end flow works:
   - Navigate to /admin/import
   - Select Contact entity
   - Upload CSV with headers: firstName,lastName,email
   - Verify auto-mapping
   - Click Analyze
   - If valid, click Import
   - Navigate to /contacts and verify records exist
</verification>

<success_criteria>
- [ ] CSVValidatorService validates enum values against field.Options
- [ ] CSVValidatorService validates data types (int, float, bool, date, email, url)
- [ ] CSVValidatorService detects missing required fields
- [ ] Analyze endpoint returns structured issues with row/column details
- [ ] Import wizard shows 3 steps with working navigation
- [ ] Column mapping dropdown allows manual remapping
- [ ] Validation errors display with row numbers and clear messages
- [ ] Successful import shows count of created records
- [ ] Imported records appear in entity list view
</success_criteria>

<output>
After completion, create `.planning/quick/012-csv-import-with-field-mapping-validat/012-SUMMARY.md`
</output>
