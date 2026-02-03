# Coding Conventions

**Analysis Date:** 2026-02-03

## Naming Patterns

**Files:**
- Go handlers: `{entity}.go` (e.g., `contact.go`, `account.go`) - PascalCase for package names (`handler`, `repo`, `entity`)
- TypeScript types: kebab-case filenames with descriptive names (e.g., `contact.ts`, `validation.ts`)
- Svelte components: PascalCase (e.g., `Button.svelte`, `LookupField.svelte`)
- Test files: `{entity}_test.go` in Go; no test files in TypeScript/Svelte (tests are minimal)

**Functions:**
- Go: PascalCase for exported functions (e.g., `NewContactHandler`, `Create`, `List`, `GetByID`)
- Private Go functions: camelCase (e.g., `getRepo`, `getTaskRepo`)
- TypeScript: camelCase for functions (e.g., `loadContacts`, `selectListView`, `handleSessionExpired`)
- Svelte methods: camelCase (e.g., `addToast`, `deleteContact`, `loadListViews`)

**Variables:**
- Go: camelCase (e.g., `contactRepo`, `userID`, `orgID`, `taskRepo`)
- TypeScript/Svelte: camelCase (e.g., `selectedListView`, `filterQuery`, `lastError`, `pageSize`)
- Constants: UPPER_SNAKE_CASE in Go (e.g., `STORAGE_KEY`)
- State in Svelte: camelCase using `$state` (e.g., `let contacts = $state([])`)

**Types:**
- Go structs: PascalCase (e.g., `Contact`, `ContactHandler`, `ContactRepo`)
- Go struct fields: camelCase with JSON tags in snake_case (e.g., `FirstName` → `json:"firstName" db:"first_name"`)
- TypeScript interfaces: PascalCase (e.g., `Contact`, `ContactListResponse`, `ContactCreateInput`)
- Type suffixes: `Input` for creation/update input types, `Response` for API responses

**Entity naming:**
- Go entity models use both JSON camelCase and database snake_case tags
- Example: `FirstName string json:"firstName" db:"first_name"`
- Computed fields (not in DB) use `db:"-"` tag

## Code Style

**Formatting:**
- TypeScript: strict mode enabled (`strict: true` in tsconfig.json)
- Frontend: SvelteKit with Vite, no explicit prettier/ESLint config found (uses defaults)
- Go: standard Go conventions (gofmt equivalent)
- TypeScript: `allowJs: true`, `checkJs: true` for type safety

**Linting:**
- No ESLint config detected in root - relies on TypeScript strict mode
- Go: no explicit linter config, uses standard conventions
- Svelte config suppresses a11y warnings and certain state warnings

**Line length:** Not enforced explicitly, but Go handlers and Svelte files appear to follow ~80-100 character lines

## Import Organization

**Go order:**
1. Standard library (e.g., `context`, `database/sql`, `time`)
2. External packages (e.g., `github.com/gofiber/fiber/v2`)
3. Internal packages (e.g., `github.com/fastcrm/backend/internal/entity`)

Example from `cmd/api/main.go`:
```go
import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	...
)
```

**TypeScript order:**
1. SvelteKit imports (`$lib`, `$env`)
2. Relative imports (`./lib`)
3. Type imports (`type { ... }`)

Example from `api.ts`:
```typescript
import { PUBLIC_API_URL } from '$env/static/public';
import type { FieldValidationError } from '$lib/types/validation';
```

**Path Aliases:**
- TypeScript: `$lib` → `/src/lib`
- `$env` → SvelteKit environment variables
- No explicit barrel exports (imports use full paths)

## Error Handling

**Go patterns:**
- Handlers return `error` from context handler functions
- Return fiber HTTP status codes with JSON error objects
- Example pattern: `return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})`
- Custom `ApiError` class in frontend for HTTP errors with field validation support
- Session expiration detected on 401 status with specific token validation message

**TypeScript patterns:**
- `ApiError` class extends `Error` with optional `status` and `fieldErrors` properties
- Distinction between retry-able errors (network/CORS) vs non-retryable (validation, auth)
- Validation errors (422 status) passed with `fieldErrors` array
- Session expired handler clears auth state and redirects to login

**Svelte patterns:**
- Try/catch blocks with fallback to empty state on error
- Toast notifications for user-facing errors
- Console.error for non-blocking failures (e.g., lookup search failures)

## Logging

**Framework:**
- Go: standard `log` package with `log.Printf` for structured logging
- Frontend: `console.error` for client-side errors only (no logging library)

**Patterns:**
- Go: Use `log.Printf` for debug info, `log.Println` for major lifecycle events
- Prefix logs with context: `[STARTUP]`, `[ERROR]`, `[v13]` for version tracking
- Environment-aware: Production errors are sanitized with error IDs; development shows full messages
- Frontend: Only log errors to console (e.g., `console.error('Failed to refresh token:', e)`)

**Examples from code:**
```go
log.Printf("Loaded environment from %s", path)
log.Println("Connection pool configured: MaxOpen=25, MaxIdle=10, MaxLifetime=5m")
log.Printf("[STARTUP] Running migration propagation for all organizations...")
```

## Comments

**When to Comment:**
- Document non-obvious business logic (e.g., "Only treat as abort if it's specifically an AbortError")
- Explain WHY code exists (security rationale, workarounds)
- Mark critical sections (e.g., "SECURITY:", "CRITICAL:")
- Explain complex type transformations (JSON/struct mapping)

**JSDoc/TSDoc:**
- Not widely used in codebase
- Go uses simple comment lines above structs and functions
- Example: `// Contact represents a CRM contact (person)`

**Security comments:**
```go
// SECURITY: Fail fast in production if JWT_SECRET is not set
// SECURITY: Sanitize error messages in production to prevent information disclosure
// SECURITY: Configure CORS based on environment
```

## Function Design

**Size:**
- Go handlers typically 20-50 lines
- TypeScript utilities vary from 10-40 lines
- No explicit function length rules observed

**Parameters:**
- Go handlers: Always receive `*fiber.Ctx` as first param
- Extract values from context: `orgID := c.Locals("orgID").(string)`
- Pass context to repo methods: `h.repo.GetByID(c.Context(), orgID, id)`

**Return Values:**
- Go handlers: Always return `error`
- Wrap results in Fiber response: `return c.JSON(result)` or `c.Status(...).JSON(...)`
- TypeScript functions: Return typed results or throw errors
- Async functions marked with `async`

**Dependency Injection pattern:**
- Handler constructors accept all dependencies: `NewContactHandler(repo, taskRepo, tripwireService, validationService)`
- Stored as struct fields
- Repos support `WithDB()` method for tenant-specific database routing

## Module Design

**Exports:**
- Go: Export package-level functions only (avoid excessive exports)
- Package structure: `internal/{handler,repo,service,entity,middleware,db}`
- TypeScript: Named exports for types and functions
- Svelte stores: Exported as module-level functions

**Example patterns:**

Go repo (from `contact.go`):
```go
type ContactRepo struct {
	db db.DBConn
}

func NewContactRepo(conn db.DBConn) *ContactRepo {
	return &ContactRepo{db: conn}
}

func (r *ContactRepo) WithDB(conn db.DBConn) *ContactRepo {
	if conn == nil {
		return r
	}
	return &ContactRepo{db: conn}
}
```

TypeScript type definitions:
```typescript
export interface Contact {
	id: string;
	orgId: string;
	// ... other fields
}

export interface ContactCreateInput {
	lastName: string; // required
	firstName?: string; // optional
}
```

Svelte store (from `toast.svelte.ts`):
```typescript
export function addToast(message: string, type: Toast['type'], duration = 3000) {
	const id = nextId++;
	toasts = [...toasts, { id, message, type }];
	setTimeout(() => { /* dismiss */ }, duration);
}

export const toast = {
	success: (message: string) => addToast(message, 'success'),
	error: (message: string) => addToast(message, 'error', 5000),
};
```

**Barrel Files:** Not used - imports specify full paths to modules

## Multi-Tenant Architecture

**Org Isolation:**
- All handlers receive `orgID` from context locals: `orgID := c.Locals("orgID").(string)`
- Passed to all repo methods as first parameter after context
- Middleware extracts and validates org context

**Database Routing:**
- Handlers get tenant-specific repo via `h.getRepo(c)` which uses `middleware.GetTenantDBConn(c)`
- Repos support `WithDB()` to swap connection without creating new instance
- Pattern: `h.getRepo(c).ListByOrg(c.Context(), orgID, params)`

**Connection Types:**
- `db.DBConn` interface for repos supporting retry-enabled connections (Turso)
- Regular `*sql.DB` for local development
- TursoDB wrapper provides automatic reconnection

---

*Convention analysis: 2026-02-03*
