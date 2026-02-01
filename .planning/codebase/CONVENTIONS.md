# Coding Conventions

**Analysis Date:** 2026-01-31

## Naming Patterns

**Files:**
- Go packages: lowercase, single-word or compound (e.g., `handler`, `repo`, `service`, `entity`)
- Go files: snake_case descriptive names (e.g., `contact.go`, `contact_test.go`)
- Svelte components: PascalCase (e.g., `LookupField.svelte`, `RelatedList.svelte`)
- TypeScript files: camelCase for utilities, PascalCase for types/interfaces (e.g., `api.ts`, `Contact.ts`)
- Test files: suffix with `_test.go` for Go, `.test.ts` for TypeScript

**Functions:**
- Go: PascalCase for exported, camelCase for unexported (e.g., `NewContactHandler`, `getRepo`)
- TypeScript/Svelte: camelCase for functions and methods (e.g., `loadContacts`, `handleSubmit`)

**Variables:**
- Go: camelCase (e.g., `contactID`, `userEmail`)
- TypeScript: camelCase (e.g., `isLoading`, `sortBy`)
- Svelte state: declared with `$state` rune in Svelte 5 (e.g., `let contacts = $state([])`)

**Types:**
- Go structs: PascalCase (e.g., `Contact`, `ContactCreateInput`, `ContactUpdateInput`)
- Go interfaces: PascalCase with suffix `-Interface` or `-er` convention (e.g., `TripwireServiceInterface`, `DBConn`)
- TypeScript interfaces: PascalCase (e.g., `Contact`, `FieldDef`, `LayoutV2Response`)
- JSON fields: camelCase in API responses and requests (e.g., `firstName`, `lastName`, `emailAddress`)
- Database columns: snake_case (e.g., `first_name`, `last_name`, `email_address`)

## Code Style

**Formatting:**
- No explicit formatter configured; manual adherence to conventions
- Go: standard Go formatting (use `gofmt` implicitly)
- TypeScript/Svelte: TypeScript strict mode enabled in `tsconfig.json`
- Line length: typically 80-100 characters

**Linting:**
- TypeScript strict mode: enabled via `tsconfig.json` with `strict: true`
- ESLint/Prettier: not explicitly configured at project root
- Svelte warnings: some a11y and state warnings suppressed in `svelte.config.js`

## Import Organization

**Order (Go):**
1. Standard library imports (`context`, `database/sql`, `encoding/json`, etc.)
2. Third-party imports (`github.com/gofiber/fiber/v2`, `golang.org/x/crypto`, etc.)
3. Internal imports (`github.com/fastcrm/backend/internal/...`)

Example from `handler/contact.go`:
```go
import (
	"context"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)
```

**Order (TypeScript/Svelte):**
1. Svelte imports (`import { onMount } from 'svelte'`)
2. Library imports (marked, uuid, etc.)
3. Local imports with path aliases (`$lib/...`, `$app/...`)
4. Type imports (`import type { ... } from ...`)

Example from `routes/contacts/new/+page.svelte`:
```typescript
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { get, post } from '$lib/utils/api';
import { toast } from '$lib/stores/toast.svelte';
import type { Contact } from '$lib/types/contact';
import type { FieldDef } from '$lib/types/admin';
```

**Path Aliases:**
- TypeScript: `$lib/` resolves to `src/lib/`, `$app/` for SvelteKit app modules

## Error Handling

**Patterns:**

Go - Explicit error checking:
```go
// From handler/contact.go
result, err := h.getRepo(c).ListByOrg(c.Context(), orgID, params)
if err != nil {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": err.Error(),
	})
}
```

Go - Safe error responses with sanitization:
```go
// From util/errors.go - Production-safe error handling
func SafeErrorResponse(c *fiber.Ctx, statusCode int, err error, publicMessage string) error {
	if IsProduction() {
		errorID := GenerateErrorID()
		log.Printf("[ERROR %s] %v", errorID, err)
		return c.Status(statusCode).JSON(fiber.Map{
			"error":   publicMessage,
			"errorId": errorID,
		})
	}
	return c.Status(statusCode).JSON(fiber.Map{
		"error": err.Error(),
	})
}
```

TypeScript - ApiError custom class with field validation:
```typescript
// From lib/utils/api.ts
export class ApiError extends Error {
	fieldErrors?: FieldValidationError[];
	status?: number;

	constructor(message: string, status?: number, fieldErrors?: FieldValidationError[]) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.fieldErrors = fieldErrors;
	}
}
```

Svelte - Try-catch with toast notifications:
```svelte
// From routes/contacts/+page.svelte
try {
	await loadContacts();
} catch (e) {
	error = e instanceof Error ? e.message : 'Failed to load contacts';
	toast.error(error);
}
```

## Logging

**Framework:** Standard `log` package in Go; `console` in TypeScript/Svelte

**Patterns:**

Go - Printf-style logging with context:
```go
// From handler/auth.go
log.Printf("Login error: %v", err)
log.Printf("[ERROR %s] %v", errorID, err)
```

TypeScript - Console methods in dev, no production logging configured:
```typescript
// From lib/stores/auth.svelte.ts
console.error('Failed to restore auth state:', e);
console.error('Failed to persist auth state:', e);
```

## Comments

**When to Comment:**
- Function/method documentation: standard Go-style comments above exported functions/types
- Complex logic: explain why, not what
- Security notes: explicitly marked (e.g., `// SECURITY: ...`)
- Known issues: TODO comments

Example from `handler/contact.go`:
```go
// getRepo returns the contact repo using the tenant database from context
// Uses GetTenantDBConn for retry-enabled connections
func (h *ContactHandler) getRepo(c *fiber.Ctx) *repo.ContactRepo {
```

**JSDoc/TSDoc:**
- Not systematically used; function exports in TypeScript have inline type annotations
- Svelte components use TypeScript script blocks with inline types

## Function Design

**Size:** Functions kept to 20-50 lines typically; handlers delegate to services

**Parameters:**
- Go handlers: receive `*fiber.Ctx`, extract context variables explicitly
- Services: accept `context.Context` as first parameter for cancellation
- TypeScript: use destructuring for options objects

**Return Values:**
- Go: explicit `(result, error)` tuple pattern
- TypeScript: return types explicitly annotated (e.g., `Promise<Contact>`)
- Svelte: functions return promises for async operations

Example from `repo/contact.go`:
```go
func (r *ContactRepo) Create(ctx context.Context, orgID string, input entity.ContactCreateInput, userID string) (*entity.Contact, error) {
	contact := &entity.Contact{
		ID:    sfid.NewContact(),
		OrgID: orgID,
		// ... field initialization
	}
	// ... implementation
	return contact, nil
}
```

## Module Design

**Exports:**

Go - Constructor pattern for all types:
```go
// From handler/contact.go
func NewContactHandler(repo *repo.ContactRepo, taskRepo *repo.TaskRepo, tripwireService TripwireServiceInterface, validationService ValidationServiceInterface) *ContactHandler {
	return &ContactHandler{repo: repo, taskRepo: taskRepo, tripwireService: tripwireService, validationService: validationService}
}
```

TypeScript - Named exports for functions, `export default` or named exports for components:
```typescript
// From lib/utils/api.ts
export class ApiError extends Error { /* ... */ }
export async function api<T>(endpoint: string, options: ApiOptions = {}): Promise<T> { /* ... */ }
export const get = <T>(endpoint: string, signal?: AbortSignal) => api<T>(endpoint, { signal });
```

Svelte - Reactive stores as module-level singletons:
```typescript
// From lib/stores/auth.svelte.ts
let state = $state<AuthState>(getInitialState());
export const auth = {
	get user() { return state.user; },
	get isAuthenticated() { return state.isAuthenticated; }
};
```

**Barrel Files:**
- Used in `lib/components/ui/index.ts` to re-export UI components
- Simplifies imports: `import { TableSkeleton, ErrorDisplay } from '$lib/components/ui'`

## Validation Patterns

**Go - Manual validation in handlers:**
```go
// From handler/contact.go
if input.LastName == "" {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": "lastName is required",
	})
}
```

**Go - Service-level validation:**
```go
// From service/auth.go - Explicit error definitions
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrPasswordTooWeak    = errors.New("password must be at least 8 characters")
)
```

**TypeScript - Form validation helpers:**
```typescript
// From routes/contacts/new/+page.svelte
const formErrors = useFormErrors();
for (const fieldName of layoutFields) {
	const field = fields.find(f => f.name === fieldName);
	if (field?.isRequired && !formData[fieldName]) {
		formErrors.setFieldError(fieldName, `${field.label} is required`);
	}
}
```

## Testing-Related Patterns

**Go - Test helper functions in test files:**
- `SetupTestApp(t *testing.T)` creates test database and Fiber app
- `app.CreateTestUser(t, email, password, orgName)` for auth tests
- `app.MakeRequest(t, method, path, body, token)` for HTTP calls

---

*Convention analysis: 2026-01-31*
