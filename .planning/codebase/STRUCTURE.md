# Codebase Structure

**Analysis Date:** 2026-02-03

## Directory Layout

```
fastcrm/
├── backend/                    # Go/Fiber API server
│   ├── cmd/
│   │   ├── api/               # Main HTTP server
│   │   ├── migrate/           # Database migrations runner
│   │   ├── seed/              # Test data seeding
│   │   └── [other-commands]/  # Admin tools
│   ├── internal/
│   │   ├── handler/           # HTTP request handlers
│   │   ├── service/           # Business logic
│   │   ├── repo/              # Data access layer
│   │   ├── db/                # Database connections and routing
│   │   ├── middleware/        # Auth, tenant, recovery middleware
│   │   ├── entity/            # Domain types
│   │   ├── flow/              # Workflow engine
│   │   ├── migrations/        # Migration definitions (Go)
│   │   ├── util/              # Shared utilities
│   │   └── sfid/              # ID generation
│   ├── tests/                 # Integration tests
│   ├── go.mod, go.sum         # Go dependencies
│   ├── Dockerfile             # Container image
│   └── migrations/            # SQL migration files (adjacent to cmd/migrate)
│
├── frontend/                   # SvelteKit UI application
│   ├── src/
│   │   ├── routes/            # SvelteKit pages (file-based routing)
│   │   │   ├── (auth)/        # Auth pages (login, register, etc.)
│   │   │   ├── contacts/      # Contact list and detail pages
│   │   │   ├── accounts/      # Account pages
│   │   │   ├── tasks/         # Task pages
│   │   │   ├── quotes/        # Quote pages
│   │   │   ├── admin/         # Admin pages
│   │   │   └── settings/      # User settings
│   │   ├── lib/
│   │   │   ├── components/    # Reusable Svelte components
│   │   │   │   ├── ui/        # Low-level UI components (Button, Skeleton, etc.)
│   │   │   │   ├── *.svelte   # Feature components (RelatedList, Toast, etc.)
│   │   │   ├── types/         # TypeScript type definitions
│   │   │   ├── stores/        # Svelte reactive stores (auth, navigation, toast)
│   │   │   ├── utils/         # Utilities (api.ts for HTTP, fieldMapping.ts)
│   │   │   └── params/        # SvelteKit param matchers
│   │   ├── app.css            # Global styles
│   │   └── app.html           # HTML template
│   ├── svelte.config.js        # SvelteKit configuration
│   ├── package.json            # Node dependencies
│   └── tailwind.config.js      # Tailwind CSS configuration
│
├── migrations/                 # SQL migration files (shared location)
│   ├── 001_create_contacts.sql
│   ├── 002_create_metadata_tables.sql
│   └── [...]
│
├── API_DOCS.md                # API endpoint documentation
├── progress.txt               # Development progress tracker
└── .planning/                 # GSD planning documents
    └── codebase/              # This directory

```

## Directory Purposes

**backend/cmd/api:**
- Purpose: HTTP server entrypoint
- Contains: Fiber app setup, middleware chain, route registration, service initialization
- Key files: `main.go` (685 lines - sets up entire API with 29+ handlers and 20+ services)

**backend/internal/handler:**
- Purpose: HTTP request handling, input validation, response serialization
- Contains: 29 handler files (contact, account, task, quote, auth, admin, etc.)
- Key files: `contact.go`, `account.go`, `auth.go`, `admin.go` - each follows Handler struct + RegisterRoutes pattern

**backend/internal/service:**
- Purpose: Business logic, orchestration, external integrations
- Contains: 21 service files including auth, provisioning, validation, PDF rendering, workflow engine
- Key files: `auth.go` (JWT, signup, org creation), `provisioning.go` (metadata setup), `validation.go` (field validation)

**backend/internal/repo:**
- Purpose: Database queries and transactions
- Contains: 22 repository files for each entity type and concern
- Key files: `contact.go`, `account.go`, `metadata.go`, `auth.go` - each provides Create/Read/Update/Delete/List operations

**backend/internal/db:**
- Purpose: Connection pooling, multi-tenant database routing, Turso integration
- Contains: `manager.go` (connection lifecycle), `turso.go` (Turso HTTP client with auto-reconnect)
- Key pattern: `DBConn` interface allows repos to work with both local SQLite and Turso

**backend/internal/middleware:**
- Purpose: Cross-cutting HTTP concerns
- Contains: Auth middleware (JWT/API token validation, org context), tenant middleware (DB routing)
- Pattern: Middleware.Required(), .OrgAdminRequired(), .PlatformAdminRequired() for role-based access control

**backend/internal/entity:**
- Purpose: Go struct definitions for domain models
- Contains: 23 entity files (contact.go, account.go, user.go, organization.go, etc.)
- Pattern: Each struct has JSON tags for API serialization, nested types for custom fields and relationships

**backend/internal/flow:**
- Purpose: Workflow/automation engine
- Contains: Engine, trigger evaluation, action execution
- Used by: Flow handler to execute business process automation

**backend/internal/migrations:**
- Purpose: Go code for applying SQL migrations
- Contains: Migration definitions as Go structs
- Called by: `cmd/migrate/main.go` to run database setup

**migrations/:**
- Purpose: SQL schema definitions
- Contains: 40+ numbered SQL files (001_create_contacts.sql, etc.)
- Pattern: Each file is idempotent (CREATE TABLE IF NOT EXISTS) and versioned

**backend/tests:**
- Purpose: Integration tests
- Contains: 13 test files (auth_test.go, contact_test.go, etc.) testing across layers
- Key files: `setup_test.go` (test database setup), individual entity tests

**frontend/src/routes:**
- Purpose: SvelteKit file-based routing
- Contains: Page files (+page.svelte) and layout files (+layout.svelte)
- Pattern: Routes match directory structure: `/routes/contacts/[id]/+page.svelte` → `/contacts/123`

**frontend/src/lib/components:**
- Purpose: Reusable UI components
- Contains: 40+ Svelte components organized by concern
- Pattern: `ui/` subdir for primitives (Button, Skeleton); feature components at root level

**frontend/src/lib/types:**
- Purpose: TypeScript type definitions
- Contains: 15+ type definition files matching backend entity structure
- Key files: `contact.ts`, `account.ts`, `layout.ts` (layout system types)

**frontend/src/lib/stores:**
- Purpose: Reactive state management using Svelte 5 runes
- Contains: `auth.svelte.ts` (user/org/tokens), `navigation.svelte.ts` (nav tabs), `toast.svelte.ts` (notifications)
- Pattern: Stores are `.svelte.ts` files with getter/setter functions exported

**frontend/src/lib/utils:**
- Purpose: Shared utility functions
- Contains: `api.ts` (HTTP client), `fieldMapping.ts` (entity field type mapping)
- Key pattern: `api.ts` handles auth token refresh, error parsing, session expiry

## Key File Locations

**Entry Points:**
- Backend: `backend/cmd/api/main.go` - Fiber app, 685 lines, all middleware and route registration
- Frontend: `frontend/src/routes/+layout.svelte` - Main layout, navigation, auth initialization
- Frontend: `frontend/src/routes/+page.svelte` - Home/dashboard redirect
- Migrations: `backend/cmd/migrate/main.go` - Runs SQL migrations from `migrations/` directory

**Configuration:**
- Backend: `backend/.env` (DATABASE_PATH, JWT_SECRET, TURSO_URL, ALLOWED_ORIGINS)
- Frontend: `frontend/.env` (PUBLIC_API_URL)
- Backend: `backend/go.mod` - Go dependency versions (Fiber v2, Turso libsql)
- Frontend: `frontend/package.json` - Node dependencies (SvelteKit, Tailwind)

**Core Logic - Backend:**
- Entity definitions: `backend/internal/entity/*.go` (23 files)
- HTTP handlers: `backend/internal/handler/*.go` (29 files)
- Business logic: `backend/internal/service/*.go` (21 files)
- Data queries: `backend/internal/repo/*.go` (22 files)
- Auth system: `backend/internal/service/auth.go`, `backend/internal/handler/auth.go`, `backend/internal/middleware/auth.go`
- Database routing: `backend/internal/db/manager.go`, `backend/internal/middleware/tenant.go`

**Core Logic - Frontend:**
- Auth flow: `frontend/src/lib/stores/auth.svelte.ts`, `frontend/src/routes/(auth)/`
- List pages: `frontend/src/routes/contacts/+page.svelte`, `frontend/src/routes/accounts/+page.svelte`
- Detail pages: `frontend/src/routes/contacts/[id]/+page.svelte` (uses SectionRenderer with dynamic layouts)
- Layout system: `frontend/src/lib/components/SectionRenderer.svelte`, `frontend/src/lib/types/layout.ts`

**Testing:**
- Backend: `backend/tests/` directory with test files per entity type
- Setup: `backend/tests/setup_test.go` - Database initialization, helpers
- Frontend: No test files (component testing via browser verification)

## Naming Conventions

**Files:**
- Handler files: entity name + `.go` (e.g., `contact.go`, `account.go`)
- Entity files: entity name + `.go` (e.g., `contact.go`)
- Service files: service name + `.go` (e.g., `auth.go`, `validation.go`)
- Repository files: entity name + `.go` (e.g., `contact.go`)
- Test files: function name + `_test.go` (e.g., `contact_test.go`)
- Frontend routes: entity name pluralized (e.g., `contacts`, `accounts`, `tasks`)

**Directories:**
- Handler: `handler/` (singular, contains all HTTP handlers)
- Service: `service/` (singular, contains all business logic)
- Repository: `repo/` (abbreviated, contains all data access)
- Entity: `entity/` (singular, contains all domain types)
- Frontend routes: plural entity names (e.g., `contacts`, `accounts`)
- Frontend components: PascalCase Svelte files (e.g., `RelatedList.svelte`, `SectionRenderer.svelte`)

**Variables and Types:**
- Receivers: 1-2 letter abbreviations (h for handler, r for repo, s for service)
- Contexts: Always named `ctx context.Context` (using Go standard naming)
- Database connections: `db` (local variable), `DB()` method (getter)
- Errors: `err` (Go convention)

**Routes:**
- List: `/api/v1/{entity}` (GET = list, POST = create)
- Detail: `/api/v1/{entity}/{id}` (GET = read, PUT/PATCH = update, DELETE = delete)
- Nested: `/api/v1/{entity}/{id}/{relation}` (e.g., `/contacts/123/related-lists`)
- Admin: `/api/v1/admin/...` (metadata, validation rules, etc.)
- Auth: `/api/v1/auth/...` (login, register, refresh, invite)
- Platform: `/api/v1/platform/...` (org management, super-admin only)

## Where to Add New Code

**New Feature (same entity):**
- Backend logic: `backend/internal/service/[feature_name].go`
- Data access: Add methods to existing `backend/internal/repo/[entity].go`
- HTTP endpoint: Add handler method to `backend/internal/handler/[entity].go`
- Route registration: Add route in handler's `RegisterRoutes()` method

**New Entity/Module:**
- Entity definition: `backend/internal/entity/[name].go` with struct + input types
- Handler: `backend/internal/handler/[name].go` with CRUD endpoints and RegisterRoutes()
- Service: `backend/internal/service/[name].go` for business logic (if needed)
- Repository: `backend/internal/repo/[name].go` with CRUD queries
- Frontend page: `frontend/src/routes/[names]/+page.svelte` (list) and `+page.svelte` in `[id]/` directory (detail)
- Frontend type: `frontend/src/lib/types/[name].ts` with TypeScript interfaces

**Utilities:**
- Shared backend helpers: `backend/internal/util/`
- Frontend helpers: `frontend/src/lib/utils/` (api.ts for HTTP, fieldMapping.ts for types)
- Domain models: `backend/internal/entity/`

**Database Schema:**
- New tables: Create migration file `migrations/NNN_description.sql` (use next number)
- Table modifications: Create new migration (never modify existing migrations)
- Indexes: Add in same migration file as table creation
- Pattern: Use `CREATE TABLE IF NOT EXISTS` for idempotency

**Tests:**
- Unit/integration tests: `backend/tests/[entity]_test.go`
- Setup helpers: Extend `setup_test.go`
- Frontend testing: Use browser DevTools verification (no automated tests currently)

## Special Directories

**backend/internal/sfid:**
- Purpose: Salesforce-like ID generation (e.g., con_abc123def456)
- Generated: No (source code)
- Committed: Yes
- Used by: All repos to generate IDs for new records

**backend/internal/migrations:**
- Purpose: Go code to load and run SQL migrations
- Generated: No (source code)
- Committed: Yes
- Related to: `migrations/` directory (SQL files)

**migrations/:**
- Purpose: Database schema as SQL
- Generated: No (source code)
- Committed: Yes
- Pattern: Numbered files, executed in order by migrate command

**frontend/.svelte-kit/:**
- Purpose: Generated SvelteKit build artifacts
- Generated: Yes (auto-generated during `npm run dev` or `npm run build`)
- Committed: No (in .gitignore)

**frontend/.vercel/:**
- Purpose: Vercel deployment build output
- Generated: Yes (auto-generated by Vercel CI/CD)
- Committed: No

**backend/tests/:**
- Purpose: Integration tests with real database
- Generated: No (source code)
- Committed: Yes
- Run: `cd backend && go test ./tests/...`

---

*Structure analysis: 2026-02-03*
