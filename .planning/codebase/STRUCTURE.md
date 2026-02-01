# Codebase Structure

**Analysis Date:** 2026-01-31

## Directory Layout

```
fastcrm/
├── backend/                    # Go/Fiber API server
│   ├── cmd/                    # Entry point commands
│   │   ├── api/                # Main API server
│   │   ├── migrate/            # Database migration runner
│   │   ├── seed/               # Test data seeding
│   │   └── provision-*/        # Organization provisioning scripts
│   ├── internal/               # Internal packages (not exported)
│   │   ├── db/                 # Database connection management
│   │   ├── entity/             # Type definitions (Contact, Account, etc.)
│   │   ├── handler/            # HTTP request handlers
│   │   ├── middleware/         # Auth, tenant, error handling
│   │   ├── repo/               # Data access layer
│   │   ├── service/            # Business logic (auth, tripwire, validation)
│   │   ├── flow/               # Screen flow engine
│   │   ├── sfid/               # Salesforce ID utilities
│   │   └── util/               # Helper functions
│   ├── migrations/             # SQL migration files
│   └── tests/                  # Test files
├── frontend/                   # SvelteKit app
│   ├── src/
│   │   ├── routes/             # File-based routing
│   │   │   ├── (auth)/         # Auth layout group (login, register)
│   │   │   ├── accounts/       # Account list/detail routes
│   │   │   ├── contacts/       # Contact list/detail routes
│   │   │   ├── tasks/          # Task list/detail routes
│   │   │   ├── quotes/         # Quote list/detail routes
│   │   │   ├── admin/          # Admin panel
│   │   │   ├── settings/       # User settings
│   │   │   └── [entity=customentity]/  # Dynamic entity routes
│   │   ├── lib/
│   │   │   ├── components/     # Reusable Svelte components
│   │   │   │   ├── ui/         # Base UI components (Button, Input, etc.)
│   │   │   │   ├── RelatedList.svelte
│   │   │   │   ├── LookupField.svelte
│   │   │   │   ├── SectionEditor.svelte
│   │   │   │   └── FieldFormModal.svelte
│   │   │   ├── stores/         # Svelte stores (auth, navigation, toast)
│   │   │   ├── types/          # TypeScript type definitions
│   │   │   ├── utils/          # Helper functions (API client, etc.)
│   │   │   └── layouts/        # Layout components (for detail/list views)
│   │   └── params/             # SvelteKit param matchers
│   └── svelte.config.js        # SvelteKit configuration
└── migrations/                 # SQL migration files (duplicate for reference)
```

## Directory Purposes

**backend/cmd/api/:**
- Purpose: Main API server entry point
- Contains: `main.go` which initializes all repos, services, middleware, registers routes, starts Fiber server
- Key files: `main.go` (645 lines - initializes entire stack)

**backend/internal/db/:**
- Purpose: Database connection management and multi-tenant routing
- Contains: `manager.go` (routes to tenant databases), `turso.go` (Turso auto-reconnect wrapper)
- Key abstraction: DBConn interface for retry-enabled connections
- Usage: Resolves correct database (master or tenant) in middleware

**backend/internal/entity/:**
- Purpose: Type definitions for domain entities
- Contains: Structs for Contact, Account, Task, Quote, User, Organization, etc.
- Pattern: Each entity has main struct + CreateInput/UpdateInput variants for request validation
- Examples: `contact.go`, `account.go`, `organization.go`

**backend/internal/handler/:**
- Purpose: HTTP request handlers (API endpoints)
- Contains: Entity-specific handlers (ContactHandler, AccountHandler) + generic handler (GenericEntityHandler)
- Largest files: `generic_entity.go` (1472 lines), `import.go` (1099 lines), `related_list.go` (807 lines)
- Key pattern: Each handler has `RegisterRoutes()` method that attaches endpoints to Fiber router group
- Dependencies: Use middleware helpers to get tenantDB and orgID from context

**backend/internal/middleware/:**
- Purpose: Cross-cutting concerns (auth, tenancy, error handling)
- Contains: `auth.go` (JWT/API token validation), `tenant.go` (database routing)
- Key responsibility: Auth middleware validates token → extracts orgID → sets in context
- Key responsibility: Tenant middleware uses orgID to get correct database connection

**backend/internal/repo/:**
- Purpose: Data access layer for specific entities/domains
- Contains: ContactRepo, AccountRepo, TaskRepo, MetadataRepo, AuthRepo, etc.
- Key pattern: Repos implement `WithDB()` method for tenant database switching
- Query pattern: All queries include org_id filtering for safety
- Examples: `contact.go`, `metadata.go`, `auth.go`

**backend/internal/service/:**
- Purpose: Business logic, multi-step operations, external integrations
- Contains: AuthService, ProvisioningService, TripwireService, ValidationService, FlowEngine
- Key examples:
  - `auth.go`: Login, register, JWT token management, organization creation
  - `provisioning.go`: Create default metadata for new org (entities, fields, layouts)
  - `tripwire.go`: Automation engine evaluating business rules on record changes
  - `validation.go`: Field and record validation rules

**backend/internal/flow/:**
- Purpose: Screen flow engine for automation workflows
- Contains: Flow execution engine, condition evaluators, action executors
- Usage: Admin-created flows that can be triggered on entity changes or user actions

**backend/internal/sfid/:**
- Purpose: Salesforce ID format utilities for import/export
- Contains: ID generation and parsing for Salesforce compatibility

**backend/internal/util/:**
- Purpose: Shared helper functions
- Contains: `filter.go` (filter parsing), `fields.go` (field metadata helpers), `records.go` (record utilities)
- Usage: Used across handlers and repos to avoid duplication

**frontend/src/routes/:**
- Purpose: File-based routing (SvelteKit convention)
- Structure: Each directory = route, `+page.svelte` = page component, `[id]` = dynamic segments
- Key routes: `contacts/`, `accounts/`, `tasks/`, `quotes/`, `admin/`, `settings/`
- Dynamic route: `[entity=customentity]/` allows rendering any custom entity with same detail view

**frontend/src/lib/components/:**
- Purpose: Reusable Svelte components
- UI components: `ui/Button.svelte`, `ui/FormField.svelte`, `ui/Spinner.svelte`
- Entity components: `RelatedList.svelte` (displays related records), `LookupField.svelte` (autocomplete)
- Special: `SectionRenderer.svelte` (renders dynamic layout sections from backend), `FieldFormModal.svelte` (reusable edit modal)

**frontend/src/lib/stores/:**
- Purpose: Reactive state management (Svelte 5 stores)
- Key stores:
  - `auth.svelte.ts`: Current user, organizations, JWT token
  - `navigation.svelte.ts`: Available entities, navigation tabs
  - `toast.svelte.ts`: Toast notifications

**frontend/src/lib/types/:**
- Purpose: TypeScript type definitions for type safety
- Files: Correspond to entities and features (contact.ts, account.ts, layout.ts, etc.)
- Pattern: Mirrors backend entity types for API consistency

**frontend/src/lib/utils/:**
- Purpose: Helper functions
- Key: `api.ts` (HTTP client with retry logic, auth token handling, error mapping)

**backend/migrations/:**
- Purpose: SQL schema definitions applied in order
- Naming: `NNN_description.sql` (e.g., `001_create_contacts.sql`, `002_create_metadata_tables.sql`)
- Execution: Run by `cmd/migrate/main.go`
- Key migrations: metadata tables (entity_defs, field_defs), lookup support, tripwires, organizations

## Key File Locations

**Entry Points:**
- `backend/cmd/api/main.go`: API server startup (initializes stack, registers routes)
- `frontend/src/routes/+layout.svelte`: Root layout (auth check, navigation setup)
- `backend/cmd/migrate/main.go`: Database migration runner

**Configuration:**
- `backend/.env` or environment variables: DB path, JWT secret, Turso credentials
- `frontend/svelte.config.js`: SvelteKit build and adapter config
- `frontend/vite.config.ts`: Vite bundler configuration

**Core Logic:**
- `backend/internal/handler/generic_entity.go`: Extensible entity CRUD (1472 lines)
- `backend/internal/service/auth.go`: Authentication, JWT, organization management
- `backend/internal/service/provisioning.go`: Default metadata creation
- `backend/internal/repo/metadata.go`: Entity/field/layout definitions
- `frontend/src/lib/components/SectionRenderer.svelte`: Dynamic layout rendering

**Testing:**
- `backend/internal/service/rollup_test.go`: Rollup field tests
- `backend/tests/`: Test data and utilities
- Frontend: No formal test files (uses route-based testing)

## Naming Conventions

**Files:**
- Go files: `snake_case.go` (e.g., `generic_entity.go`, `contact_repo.go`)
- Svelte files: `PascalCase.svelte` (e.g., `RelatedList.svelte`, `SectionEditor.svelte`)
- SQL migrations: `NNN_description.sql` (e.g., `001_create_contacts.sql`)
- Routes: Lowercase directory names (e.g., `contacts/`, `accounts/`)

**Functions:**
- Go handlers: PascalCase (e.g., `List()`, `Create()`, `Update()`, `Delete()`)
- Go services: PascalCase (e.g., `EvaluateAndFire()`, `ProvisionDefaultMetadata()`)
- Go repos: PascalCase (e.g., `ListByOrg()`, `GetByID()`, `Create()`)
- SvelteKit pages: `+page.svelte`, `+layout.svelte`, `+error.svelte`

**Variables:**
- Go: `camelCase` for locals, `PascalCase` for exported
- TypeScript: `camelCase` for variables, `PascalCase` for types
- Database columns: `snake_case` (e.g., `first_name`, `email_address`)
- JSON fields: `camelCase` (e.g., `firstName`, `emailAddress`)

**Types:**
- Go structs: PascalCase (e.g., `Contact`, `CreateInput`, `ListParams`)
- Go interfaces: PascalCase ending with "Interface" (e.g., `TripwireServiceInterface`)
- TypeScript types/interfaces: PascalCase (e.g., `Contact`, `CreateContactInput`)

**Database:**
- Tables: Lowercase plural (e.g., `contacts`, `accounts`, `tasks`)
- Columns: `snake_case` (e.g., `first_name`, `email_address`, `org_id`)
- Primary keys: Always `id` (UUID)
- Foreign keys: `{entity}_id` (e.g., `contact_id`, `account_id`)
- Metadata tables: `*_defs` (e.g., `entity_defs`, `field_defs`, `layout_defs`)

## Where to Add New Code

**New Feature (CRUD for existing entity like Contact):**
- Primary code: `backend/internal/handler/contact.go` (extend ContactHandler methods)
- Add repo methods: `backend/internal/repo/contact.go` (add query methods)
- Add service logic: `backend/internal/service/` (create ServiceName if complex)
- Frontend page: `frontend/src/routes/contacts/...` (add routes as needed)
- Frontend components: `frontend/src/lib/components/` (reusable components)
- Type definitions: `backend/internal/entity/contact.go`, `frontend/src/lib/types/contact.ts`

**New Custom Entity (completely new entity type):**
- Metadata migration: `backend/migrations/NNN_create_custom_entity.sql` (create table)
- Entity struct: `backend/internal/entity/custom_entity.go` (define Contact-like type)
- Repository: `backend/internal/repo/custom_entity.go` (copy pattern from contact.go)
- Handler: `backend/internal/handler/custom_entity.go` (copy pattern from contact.go) OR use GenericEntityHandler
- Service (if needed): `backend/internal/service/custom_entity.go`
- Frontend routes: `frontend/src/routes/[entity=customentity]/` (dynamic route handles rendering)
- Frontend types: `frontend/src/lib/types/custom_entity.ts`
- Metadata provisioning: Add to `backend/internal/service/provisioning.go` to create default fields/layouts

**New API/Third-Party Integration:**
- Service: `backend/internal/service/integration_name.go` (handle API calls, retry logic)
- Handler routes: `backend/internal/handler/` (new handler or extend existing)
- Environment config: Add to `.env`, document in startup instructions

**Database Schema Change:**
- Migration file: `backend/migrations/NNN_description.sql`
- Update affected repo methods to handle new column
- Update entity structs to include new field
- Update handler input structs if field is user-settable

**Admin Feature (configuration, settings):**
- Handler: `backend/internal/handler/admin.go` (add admin-only routes)
- Repo: `backend/internal/repo/` (new repo if new table)
- Frontend: `frontend/src/routes/admin/...` (admin panel pages)
- Frontend components: `frontend/src/lib/components/` (reusable admin components)
- Authorization: Protect routes with `authMiddleware.OrgAdminRequired()`

**Utilities (shared helpers):**
- Shared helpers: `backend/internal/util/` (for functions used in multiple packages)
- Frontend utils: `frontend/src/lib/utils/` (client-side helpers)
- Type utilities: Create in same file as related types or in `util/` if generic

## Special Directories

**backend/tmp/:**
- Purpose: Generated files, build artifacts
- Generated: Yes (should be gitignored)
- Committed: No

**frontend/.svelte-kit/:**
- Purpose: SvelteKit build output and type generation
- Generated: Yes (automatically generated during build)
- Committed: No

**backend/tests/:**
- Purpose: Test utilities, seed data, test helpers
- Generated: No (hand-written)
- Committed: Yes

**backend/scripts/:**
- Purpose: Utility scripts for development/operations
- Generated: No
- Committed: Yes

**fastcrm.db* files (in fastcrm/ root):**
- Purpose: Local SQLite development database and WAL files
- Generated: Yes (created on first run)
- Committed: No (should be gitignored)

---

*Structure analysis: 2026-01-31*
