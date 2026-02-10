# FastCRM Regression Test Suite

A comprehensive regression test suite for the FastCRM application covering all major features.

## Test Coverage

### Authentication Tests (`auth_test.go`)
- User registration with organization creation
- Login/logout functionality
- Token refresh
- Password change
- Organization switching
- User invitations and acceptance

### Contact CRUD Tests (`contact_test.go`)
- Create, Read, Update, Delete operations
- Search and pagination
- Sorting
- Custom fields
- Organization isolation (multi-tenancy)
- Related tasks

### Account CRUD Tests (`account_test.go`)
- Create, Read, Update, Delete operations
- Search and pagination
- Sorting
- Custom fields
- Organization isolation
- Contact relationship
- Related tasks

### Task CRUD Tests (`task_test.go`)
- Create, Read, Update, Delete operations
- Task types (Todo, Call, Email, Meeting)
- Task status management
- Parent entity linking (Contact, Account)
- Filtering by status, type, parent
- Organization isolation

### Validation Rules Tests (`validation_test.go`)
- CRUD operations for validation rules
- Rule enforcement (BLOCK_SAVE action)
- Conditional validation
- Various operators (EQUALS, NOT_EQUALS, CONTAINS, NOT_EMPTY)
- Event types (CREATE, UPDATE, DELETE)
- Inactive rule handling
- Non-admin access restrictions

### Tripwire/Webhook Tests (`tripwire_test.go`)
- CRUD operations for tripwires
- Event types (CREATE, UPDATE, DELETE)
- Entity type support
- HTTP methods (POST, PUT, PATCH)
- Conditional triggers
- Organization isolation
- Non-admin access restrictions

### Bulk Operations Tests (`bulk_test.go`)
- Bulk create for multiple entity types
- Bulk update
- Partial failure handling
- Validation rule enforcement
- Organization isolation

### Admin & Authorization Tests (`admin_test.go`)
- Entity management
- Field types
- Navigation management
- User management
- Role-based access control
- Regular user restrictions
- Unauthenticated access handling
- Bearings configuration
- Related lists configuration

## Running Tests

### Prerequisites

1. Ensure you have Go 1.22+ installed
2. Navigate to the backend directory:
   ```bash
   cd fastcrm/backend
   ```

### Run All Tests

```bash
go test ./tests/... -v
```

### Run Specific Test File

```bash
# Run auth tests only
go test ./tests/... -v -run TestAuth

# Run contact tests only
go test ./tests/... -v -run TestContact

# Run account tests only
go test ./tests/... -v -run TestAccount

# Run task tests only
go test ./tests/... -v -run TestTask

# Run validation tests only
go test ./tests/... -v -run TestValidation

# Run tripwire tests only
go test ./tests/... -v -run TestTripwire

# Run bulk tests only
go test ./tests/... -v -run TestBulk

# Run admin/authorization tests only
go test ./tests/... -v -run TestAdmin
go test ./tests/... -v -run TestAuthorization
```

### Run Specific Test

```bash
go test ./tests/... -v -run TestAuth_Login
go test ./tests/... -v -run TestContact_Create
```

### Run with Race Detection

```bash
go test ./tests/... -v -race
```

### Run with Coverage

```bash
go test ./tests/... -v -cover -coverprofile=coverage.out

# View coverage report in browser
go tool cover -html=coverage.out
```

### Run Tests in Short Mode (skip long-running tests)

```bash
go test ./tests/... -v -short
```

## Test Architecture

### Test Setup (`setup_test.go`)

The test infrastructure provides:

- **SetupTestApp**: Creates a complete test application with:
  - In-memory SQLite database
  - All handlers and services initialized
  - All routes registered
  - Fresh database migrations applied

- **TestUser**: Helper struct containing user credentials and tokens

- **Helper Functions**:
  - `CreateTestUser`: Register a new user and get their tokens
  - `LoginUser`: Login an existing user
  - `MakeRequest`: Make HTTP requests to the test app
  - `MakeRequestWithResponse`: Make requests and decode JSON responses
  - `AssertStatus`: Assert HTTP status codes
  - `AssertJSON`: Assert JSON response fields

### Test Isolation

- Each test function uses a fresh database
- Tests are independent and can run in parallel
- Organization isolation is tested explicitly

## Adding New Tests

1. Create a new file in the `tests` directory (e.g., `newfeature_test.go`)
2. Use the package name `tests`
3. Import the test helpers from `setup_test.go`
4. Follow the existing patterns:

```go
func TestNewFeature_SomeOperation(t *testing.T) {
    app := SetupTestApp(t)
    defer app.Cleanup()

    user := app.CreateTestUser(t, "test@example.com", "password123", "Test Org")

    t.Run("test case description", func(t *testing.T) {
        // Arrange
        body := map[string]interface{}{
            "field": "value",
        }

        // Act
        var response struct {
            Field string `json:"field"`
        }
        resp := app.MakeRequestWithResponse(t, "POST", "/api/v1/endpoint", body, user.AccessToken, &response)

        // Assert
        AssertStatus(t, resp, http.StatusOK)
        if response.Field != "expected" {
            t.Errorf("Expected field 'expected', got %s", response.Field)
        }
    })
}
```

## Pre-Go-Live Checklist

Run the full test suite and ensure all tests pass:

```bash
go test ./tests/... -v 2>&1 | tee test-results.txt
```

Review the output and fix any failing tests before going live.

## Troubleshooting

### Tests fail with "migration not found"
Ensure the migrations directory exists at the correct relative path from the tests directory.

### Tests fail with database errors
Check that SQLite is properly installed and the test has write permissions for temp directories.

### Tests timeout
Increase the test timeout: `go test ./tests/... -v -timeout 5m`

### Race conditions
Run with `-race` flag to detect race conditions: `go test ./tests/... -v -race`
