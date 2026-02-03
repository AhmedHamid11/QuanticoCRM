# Testing Patterns

**Analysis Date:** 2026-02-03

## Test Framework

**Runner:**
- Go testing: Standard `testing` package
- No external test runner (native `go test`)
- Test command: `go test ./tests/...` (located in `/backend/tests/`)

**Assertion Library:**
- Standard Go `testing.T` methods: `t.Error()`, `t.Errorf()`, `t.Fatalf()`
- No third-party assertion library (testify, require, etc.)
- Custom assertions from `setup_test.go`: `AssertStatus(t, resp, expectedStatus)`

**Run Commands:**
```bash
# Run all tests
go test ./tests/... -timeout 5m

# Run with verbose output
go test ./tests/... -v

# Run with coverage
go test ./tests/... -cover -coverprofile=coverage.out

# Run with race detection
go test ./tests/... -race

# Run specific test by name
go test ./tests/... -run TestAuth

# Run via provided script (from /backend/run_tests.sh)
./run_tests.sh                    # Run all tests
./run_tests.sh -v                 # Verbose
./run_tests.sh -c                 # With coverage
./run_tests.sh -r                 # With race detection
./run_tests.sh -t TestAuth        # Specific test
```

## Test File Organization

**Location:**
- Backend tests: `backend/tests/` directory (not co-located with source)
- Separate from implementation files
- One test file per entity: `contact_test.go`, `account_test.go`, `task_test.go`, etc.

**Naming:**
- Test files: `{entity}_test.go`
- Test functions: `Test{Entity}_{Feature}` (e.g., `TestContact_Create`)
- Sub-tests use `t.Run()` with descriptive names

**Structure:**
```
backend/
├── tests/
│   ├── setup_test.go          # Common test infrastructure
│   ├── contact_test.go        # Contact entity tests
│   ├── account_test.go        # Account entity tests
│   ├── task_test.go           # Task entity tests
│   ├── auth_test.go           # Auth/session tests
│   ├── admin_test.go          # Admin functionality tests
│   ├── bulk_test.go           # Bulk operations tests
│   ├── import_test.go         # Import functionality tests
│   ├── validation_test.go     # Field validation tests
│   ├── tripwire_test.go       # Workflow/trigger tests
│   └── ...
```

## Test Structure

**Suite Organization:**
```go
func TestContact_Create(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	user := app.CreateTestUser(t, "contact@example.com", "password123", "Contact Test Org")

	t.Run("creates contact with required fields", func(t *testing.T) {
		body := map[string]interface{}{
			"lastName": "Smith",
		}

		var response struct {
			ID       string `json:"id"`
			LastName string `json:"lastName"`
		}

		resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", body, user.AccessToken, &response)
		AssertStatus(t, resp, http.StatusCreated)

		if response.ID == "" {
			t.Error("Expected contact ID to be set")
		}
		if response.LastName != "Smith" {
			t.Errorf("Expected lastName 'Smith', got %s", response.LastName)
		}
	})
}
```

**Patterns:**
- Setup: `SetupTestApp(t)` creates test instance with temp database
- Teardown: `defer app.Cleanup()` cleans up resources
- Per-test user: `app.CreateTestUser(t, email, password, orgName)` returns `TestUser` with tokens
- Assertions: `AssertStatus(t, resp, expectedStatus)` custom helper

## Mocking

**Framework:**
- No external mocking library (testify/mock)
- In-memory SQLite database for all tests
- Actual repository implementations used (integration testing approach)

**Patterns:**
- Create temporary database: `sql.Open("sqlite3", dbPath)` in temp directory
- Run migrations: `runMigrations(db)` to set up schema
- Create test repos directly: `repo.NewContactRepo(db)`
- Use real Fiber app with test HTTP server

**Test Database:**
```go
// From setup_test.go
tmpDir := t.TempDir()
dbPath := filepath.Join(tmpDir, "test.db")

db, err := sql.Open("sqlite3", dbPath)
if err != nil {
	t.Fatalf("Failed to open database: %v", err)
}

if err := runMigrations(db); err != nil {
	t.Fatalf("Failed to run migrations: %v", err)
}
```

**What to Mock:**
- Nothing mocked; use real implementations with in-memory database
- API endpoints tested via HTTP test server: `httptest.NewServer()`

**What NOT to Mock:**
- External services (not present in codebase)
- Database calls (integration tests use real SQLite)
- HTTP clients (test server provides mock endpoints)

## Fixtures and Factories

**Test Data:**
- `CreateTestUser()` factory: Creates user with email, password, org
- Returns `TestUser` with `UserID`, `Email`, `OrgID`, `AccessToken`, `RefreshToken`
- Used for authenticated request testing

**Location:**
- Factory functions in `setup_test.go`
- Test app helpers: `SetupTestApp()`, `MakeRequest()`, `MakeRequestWithResponse()`

**Example factory usage:**
```go
user := app.CreateTestUser(t, "test@example.com", "password123", "Test Org")

// Create contact
body := map[string]interface{}{"lastName": "Smith"}
resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", body, user.AccessToken, &response)
```

## Coverage

**Requirements:**
- No enforced minimum coverage
- Coverage tracking available via `-cover` flag

**View Coverage:**
```bash
# Generate coverage report
./run_tests.sh -c

# View HTML report
go tool cover -html=coverage.out

# View summary
go tool cover -func=coverage.out | tail -1
```

Script automatically generates `coverage.html` when run with `-c` flag.

## Test Types

**Unit Tests:**
- Scope: Individual handler/repo methods
- Approach: Use real database and HTTP test server (integration style)
- No function-level isolation (repos are tested with actual DB)
- Example: `TestContact_Create`, `TestContact_Update`, `TestContact_Delete`

**Integration Tests:**
- Scope: Full request/response cycle with database
- Approach: SetupTestApp creates complete Fiber app with all middleware
- Tests: CRUD operations, validation, auth flow
- Coverage: Auth middleware, tenant routing, database transactions

**E2E Tests:**
- Not detected in codebase
- All tests are backend integration tests
- No frontend/browser testing framework found

## Common Patterns

**Async Testing:**
- Not applicable (Go tests are synchronous)
- HTTP requests handled via `http.Client`

**Error Testing:**
- Check HTTP status codes: `AssertStatus(t, resp, expectedStatus)`
- Parse error response body: `var response struct { error string }`
- Verify error messages in assertions

**Pattern for error cases:**
```go
t.Run("returns error for invalid input", func(t *testing.T) {
	body := map[string]interface{}{
		// Missing required field: lastName
	}

	resp := app.MakeRequest(t, "POST", "/api/v1/contacts/", body, user.AccessToken)
	AssertStatus(t, resp, http.StatusBadRequest)

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "error") {
		t.Error("Expected error in response")
	}
})
```

**Test isolation:**
- Each test creates fresh temp database via `t.TempDir()`
- No test pollution or shared state
- Tests can run in parallel (no global state)

**Test user creation:**
```go
// From setup_test.go - TestApp method
user := app.CreateTestUser(t, email, password, orgName)
// Returns TestUser with:
// - UserID: generated
// - AccessToken: JWT token for authenticated requests
// - OrgID: organization created for user
// - RefreshToken: token refresh token
```

**HTTP request testing:**
```go
resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/contacts/", body, user.AccessToken, &response)
// Makes HTTP request to test server with:
// - Method: POST
// - Path: /api/v1/contacts/
// - Body: JSON-encoded body
// - Auth: Bearer token from user
// - Parses response into &response struct
```

## Test Configuration

**Timeout:** 5 minutes per test run (`-timeout 5m`)

**Race detection:** Optional via `-r` flag in run_tests.sh

**Short mode:** Available via `-s` flag to skip long-running tests

**Verbosity:** Controlled via `-v` flag

## Frontend Testing

**TypeScript/Svelte:** No test files detected in codebase
- Frontend is tested manually or via browser verification (per CLAUDE.md requirements)
- No Jest, Vitest, or Playwright configuration
- Manual testing via Chrome DevTools MCP when implementing UI features

---

*Testing analysis: 2026-02-03*
