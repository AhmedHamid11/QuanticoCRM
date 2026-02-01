# Testing Patterns

**Analysis Date:** 2026-01-31

## Test Framework

**Runner:**
- Go: standard `testing` package (stdlib)
- TypeScript: not explicitly configured (no test framework detected in `package.json`)

**Run Commands:**
```bash
# Go tests (from backend directory)
go test ./...              # Run all tests
go test -v ./tests         # Verbose output for tests directory
go test -run TestContact   # Run specific test suite
```

**Test files location:** `/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend/tests/`

## Test File Organization

**Location:**
- Go: co-located in `tests/` subdirectory within `backend/`
- Tests are separate from source code but organized by module

**Naming:**
- Convention: `{entity}_test.go` (e.g., `contact_test.go`, `auth_test.go`, `validation_test.go`)
- Setup/utilities: `setup_test.go` for test infrastructure

**Files present:**
```
backend/tests/
├── setup_test.go         # Common test setup and helpers
├── auth_test.go          # Authentication tests
├── contact_test.go       # Contact entity tests
├── account_test.go       # Account entity tests
├── task_test.go          # Task entity tests
├── validation_test.go    # Validation rule tests
├── tripwire_test.go      # Tripwire/workflow tests
├── bulk_test.go          # Bulk operation tests
├── admin_test.go         # Admin endpoint tests
└── import_test.go        # Data import tests
```

## Test Structure

**Suite Organization:**
```go
func TestContact_Create(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "password123", "Contact Test Org")

	t.Run("creates contact with required fields", func(t *testing.T) {
		// Test implementation
	})

	t.Run("creates contact with all fields", func(t *testing.T) {
		// Test implementation
	})

	t.Run("fails without lastName", func(t *testing.T) {
		// Test implementation
	})
}
```

**Patterns:**

1. **Setup per test function:** Each top-level test function calls `SetupTestApp(t)` and defers `app.Cleanup()` for isolation
   ```go
   func TestContact_Create(t *testing.T) {
       app := SetupTestApp(t)
       defer app.Cleanup()
   ```

2. **Subtests with t.Run():** Logical grouping within single test function
   ```go
   t.Run("creates contact with required fields", func(t *testing.T) {
       // Specific test case
   })
   ```

3. **Create test user first:** Most tests need authentication
   ```go
   user := app.CreateTestUser(t, "contact@example.com", "password123", "Contact Test Org")
   ```

## Test Setup and Infrastructure

**TestApp structure** (`setup_test.go` lines 26-38):
```go
type TestApp struct {
	App               *fiber.App
	DB                *sql.DB
	AuthService       *service.AuthService
	ContactRepo       *repo.ContactRepo
	AccountRepo       *repo.AccountRepo
	TaskRepo          *repo.TaskRepo
	MetadataRepo      *repo.MetadataRepo
	TripwireRepo      *repo.TripwireRepo
	ValidationRepo    *repo.ValidationRepo
	TripwireService   *service.TripwireService
	ValidationService *service.ValidationService
}
```

**TestUser structure** (`setup_test.go` lines 40-49):
```go
type TestUser struct {
	UserID       string
	Email        string
	OrgID        string
	OrgName      string
	Role         string
	AccessToken  string
	RefreshToken string
}
```

**Setup process** (`setup_test.go` lines 52-68):
1. Create temporary database file with `t.TempDir()`
2. Open SQLite connection
3. Run all migrations via `runMigrations(db)`
4. Initialize all repositories
5. Initialize all services
6. Create Fiber app with routes
7. Return TestApp for test use

## Request Testing Patterns

**Helper methods:**
- `app.MakeRequest(t, method, path, body, token)` - makes HTTP request, returns `*http.Response`
- `app.MakeRequestWithResponse(t, method, path, body, token, responseStruct)` - unmarshals JSON response into struct
- `app.CreateTestUser(t, email, password, orgName)` - creates user, org, and membership; returns TestUser with tokens

**Status assertion:**
```go
func AssertStatus(t *testing.T, resp *http.Response, expectedStatus int) {
	if resp.StatusCode != expectedStatus {
		t.Errorf("Expected status %d, got %d", expectedStatus, resp.StatusCode)
	}
}
```

**Example from `contact_test.go`:**
```go
resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", body, user.AccessToken, &response)
AssertStatus(t, resp, http.StatusCreated)

if response.ID == "" {
	t.Error("Expected contact ID to be set")
}
if response.LastName != "Smith" {
	t.Errorf("Expected lastName 'Smith', got %s", response.LastName)
}
```

## Test Scope and Coverage

**Unit Tests:**
- Focus: Individual handler methods (Create, Read, Update, Delete, List)
- Scope: Request parsing, validation, response format
- Example: `TestContact_Create` tests POST /contacts with valid and invalid inputs

**Integration Tests:**
- Focus: End-to-end request/response cycles
- Scope: Auth flow, database persistence, related record operations
- Example: `TestContact_Update` fetches contact, modifies, updates, verifies change

**Handler tests included:**
- CRUD operations (Create, Get, Update, Delete)
- List with pagination and filtering
- Validation error responses
- Authentication checks (with/without token)
- Multi-tenant isolation (orgID enforcement)

## Error Testing

**Pattern:**
```go
// Test required field validation
t.Run("fails without lastName", func(t *testing.T) {
	body := map[string]interface{}{
		"firstName": "John",
	}

	resp := app.MakeRequest(t, "POST", "/api/v1/contacts/", body, user.AccessToken)
	AssertStatus(t, resp, http.StatusBadRequest)
})

// Test authentication requirement
t.Run("fails without authentication", func(t *testing.T) {
	body := map[string]interface{}{
		"lastName": "Smith",
	}

	resp := app.MakeRequest(t, "POST", "/api/v1/contacts/", body, "")
	AssertStatus(t, resp, http.StatusUnauthorized)
})

// Test 404 for missing resource
t.Run("returns 404 for non-existent contact", func(t *testing.T) {
	resp := app.MakeRequest(t, "GET", "/api/v1/contacts/nonexistent-id", nil, user.AccessToken)
	AssertStatus(t, resp, http.StatusNotFound)
})
```

## Async Testing

Not applicable for Go (synchronous execution). TypeScript/Svelte testing not configured.

## Database Testing

**SQLite in-memory:** Tests use temporary SQLite files (`t.TempDir()`), isolated per test

**Migration execution:** Full migration suite runs before each test
- `runMigrations(db)` applies all schema migrations
- Ensures test database matches production schema

**Cleanup:** `defer app.Cleanup()` removes temporary database files

**Multi-tenant isolation verified:**
- Tests create users in different orgs
- Verify orgID enforcement in queries
- Data isolation test: User A cannot see User B's records

## Coverage

**Requirements:** Not enforced (no coverage flag in test commands)

**Manual coverage assessment:**
- Contact tests: Create, Get, List, Update, Delete covered
- Auth tests: Register, Login, Refresh tokens covered
- Validation tests: Rule evaluation covered
- Tripwire tests: Workflow triggers covered
- Admin tests: Entity/layout management covered

## Testing Best Practices Used

1. **Isolation:** Each test gets fresh database via `SetupTestApp(t)`, `defer app.Cleanup()`
2. **Named subtests:** `t.Run(name, func)` groups related assertions
3. **Predictable test data:** `CreateTestUser` provides consistent starting state
4. **Assertion helpers:** `AssertStatus` reduces boilerplate
5. **Error response validation:** Tests check both status codes and error messages
6. **Auth verification:** Every protected endpoint tested with and without token

## Frontend Testing

**Status:** No test framework configured in `package.json`
- No Jest, Vitest, or other framework present
- No test files found in frontend codebase

## Running Tests

**Execute all backend tests:**
```bash
cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend
go test ./tests
```

**Execute specific test:**
```bash
go test ./tests -run TestContact_Create
```

**Verbose output:**
```bash
go test -v ./tests
```

**With coverage (if implementing):**
```bash
go test -cover ./tests
```

---

*Testing analysis: 2026-01-31*
