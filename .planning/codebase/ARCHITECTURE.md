# Architecture

**Analysis Date:** 2026-01-31

## Pattern Overview

**Overall:** Multi-tenant SaaS CRM with layered backend (Go/Fiber) and reactive frontend (SvelteKit). Data isolation per organization using separate per-tenant databases in production (Turso) or shared local SQLite in development.

**Key Characteristics:**
- Multi-tenant architecture with per-org database isolation (Turso in production, local SQLite in dev)
- Layered backend with clear separation: handlers → services → repositories → database
- Fiber web framework with middleware for auth, tenancy, and error handling
- Reactive SvelteKit frontend with optimistic UI updates and built-in form validation
- Generic entity handler for extensible entity management without code changes
- Metadata-driven layouts loaded from backend (no hardcoded UI field layouts)

## Layers

**Handlers (HTTP Request Layer):**
- Purpose: Accept HTTP requests, validate input, delegate to services/repos, return JSON responses
- Location: `backend/internal/handler/`
- Contains: Entity-specific handlers (ContactHandler, AccountHandler, etc.), generic handler (GenericEntityHandler), admin handlers
- Depends on: Services, repositories, middleware context (orgID, tenantDB)
- Used by: Fiber routing, middleware chain
- Pattern: Each handler has `RegisterRoutes()` method that attaches endpoints to Fiber groups

**Services (Business Logic Layer):**
- Purpose: Complex business operations, multi-step workflows, external integrations
- Location: `backend/internal/service/`
- Contains: AuthService, ProvisioningService, TripwireService, ValidationService, FlowEngine, PDFService
- Depends on: Repositories, database connections
- Used by: Handlers
- Examples: `TripwireService` evaluates automation rules, `ProvisioningService` creates org metadata, `AuthService` manages authentication

**Repositories (Data Access Layer):**
- Purpose: Encapsulate database queries for specific entities/domains
- Location: `backend/internal/repo/`
- Contains: ContactRepo, AccountRepo, TaskRepo, MetadataRepo, AuthRepo, etc.
- Depends on: Database connections (DBConn interface or *sql.DB)
- Used by: Handlers, services
- Pattern: Each repo has `WithDB()` method for tenant database switching

**Database Layer:**
- Purpose: Connection management, tenant routing, Turso auto-reconnect
- Location: `backend/internal/db/`
- Contains: Manager (routes to tenant DBs), TursoDB (auto-reconnect wrapper), DBConn interface
- Depends on: SQL driver (sqlite3 for local, libsql for Turso)
- Used by: Middleware, repos

**Frontend Components (SvelteKit):**
- Purpose: Reactive UI, optimistic updates, client-side validation
- Location: `frontend/src/routes/` (pages), `frontend/src/lib/` (reusable components)
- Contains: Page components, form components, detail/list views, layout renderer
- Depends on: API client, stores (auth, navigation, toast), type definitions
- Used by: Browser navigation

## Data Flow

**Create/Update/Delete Flow:**

1. Frontend form submission (optimistic update applied immediately)
2. API request sent to backend with auth token
3. AuthMiddleware validates token, sets orgID in context
4. TenantMiddleware resolves tenant database, sets in context
5. Handler receives request, extracts orgID and tenantDB from context
6. Handler calls repository with tenant database
7. Repository executes query on tenant database
8. Service layer evaluates tripwires/validations if applicable
9. Response returned to frontend
10. Frontend either confirms optimistic update or rolls back on error

**List/Detail View Flow:**

1. Page component loads on route navigation
2. Fetch layout definition from `/api/v1/entities/{Entity}/layouts/detail`
3. Fetch field metadata from `/api/v1/entities/{Entity}/fields`
4. Fetch record data from `/api/v1/entities/{Entity}/{id}`
5. Frontend SectionRenderer renders layout sections dynamically based on metadata
6. Lookup fields trigger auto-complete from `/api/v1/lookup/{entity}/{field}`

**Multi-Tenant Isolation:**

1. Request arrives with JWT token containing orgID
2. AuthMiddleware validates token, extracts and sets orgID in Fiber context
3. TenantMiddleware uses orgID to get tenant database:
   - Development: Shared SQLite database with org_id column filtering
   - Production (Turso): Separate per-org database with automatic connection caching
4. All queries executed on isolated tenant database
5. No cross-tenant data visible due to database separation (not just filtering)

**Metadata-Driven Layout:**

1. Admin configures entity layouts in Layout Editor
2. Layouts stored in `layout_defs` table (master database)
3. Frontend fetches layout and field defs on page load
4. Layout defines sections, field display order, visibility rules
5. Frontend SectionRenderer dynamically renders UI without hardcoded layouts

## Key Abstractions

**DBConn Interface:**
- Purpose: Abstract database connection for repos that need retry logic (Turso auto-reconnect)
- Examples: `backend/internal/repo/metadata.go`, `backend/internal/repo/auth.go`
- Pattern: Repos that frequently reconnect use DBConn; data repos use *sql.DB (already get tenant DB via middleware)

**GenericEntityHandler:**
- Purpose: Single handler for all custom/dynamic entities without per-entity code
- Examples: `backend/internal/handler/generic_entity.go` (1472 lines)
- Pattern: Reads entity metadata at runtime, dynamically builds queries, supports CRUD for any entity

**TripwireService:**
- Purpose: Automation engine evaluating business rules on record changes
- Examples: Auto-create tasks on opportunity creation, update account on contact change
- Pattern: Listens to Create/Update/Delete events, evaluates rule conditions, executes actions

**ValidationService:**
- Purpose: Field-level and record-level validation rules
- Examples: Check email format, ensure required fields, custom validators
- Pattern: Evaluated before save, returns field-level error map

**ProvisioningService:**
- Purpose: Create default metadata (entities, fields, layouts) when new org signs up
- Examples: `backend/internal/service/provisioning.go`
- Pattern: Runs once per org, creates default Contact/Account/Task/Quote entities with standard fields

**TenantProvisioningService:**
- Purpose: Create dedicated Turso databases for each organization in production
- Examples: `backend/internal/service/tenant_provisioning.go`
- Pattern: On new org creation, provision new Turso database if not in local mode

## Entry Points

**Backend API Server:**
- Location: `backend/cmd/api/main.go`
- Triggers: `go run ./cmd/api/main.go` or deployment startup
- Responsibilities: Initialize repositories, services, middleware, register routes, start Fiber server

**Frontend SvelteKit App:**
- Location: `frontend/src/routes/+layout.svelte` (root layout)
- Triggers: `npm run dev` or Vercel deployment
- Responsibilities: Set up auth store, load user profile, render navigation, handle routing

**Database Migrations:**
- Location: `backend/cmd/migrate/main.go`
- Triggers: `go run ./cmd/migrate/main.go`
- Responsibilities: Apply SQL migrations to master database schema

**Seed/Provision Commands:**
- Location: `backend/cmd/seed/main.go`, `backend/cmd/provision-recruiting/main.go`
- Triggers: Manual invocation in development
- Responsibilities: Populate test data, provision org metadata

## Error Handling

**Strategy:** Layered error responses with sanitization in production

**Patterns:**
- Handlers catch errors from repos/services, return HTTP status + JSON error
- Production: Generic error messages with error ID for support correlation (prevent info disclosure)
- Development: Full error messages for debugging
- Frontend: Custom ApiError class with optional field-level validation errors for form handling
- Tripwires/Validations: Run in background, don't block save (tripwires post-fire)

**Error Recovery:**
- Frontend optimistic UI with rollback on API error
- Retry logic in API client for network failures (3 attempts with exponential backoff)
- Database connection retry via TursoDB wrapper for transient failures

## Cross-Cutting Concerns

**Logging:**
- Backend: Fiber logger middleware + log.Printf calls in handlers/services
- Frontend: Browser console (can use devtools)
- No structured logging framework (uses stdlib log)

**Validation:**
- Backend: ValidationService evaluates rules, returns field-level error map
- Frontend: Form component validation (client-side) + API error field mapping (server-side)
- Tripwires: Evaluated asynchronously post-save

**Authentication:**
- JWT tokens issued by `/auth/login`, stored in localStorage
- AuthMiddleware validates token on every request (except public routes)
- API tokens for programmatic access (separate token_auth table)
- Platform admin impersonation support with `impersonatedBy` JWT claim

**Authorization:**
- OrgAdminRequired() middleware checks user role in org
- PlatformAdminRequired() middleware checks platform admin flag
- Required() middleware checks any authenticated user
- Fine-grained: Admin-only routes group at `/admin` and `/platform` prefixes

**Multi-Tenancy:**
- Every table has org_id column (even in dev)
- Production: Separate Turso database per org (hard isolation)
- Development: Shared SQLite with org_id column filtering (soft isolation)
- Tenant database resolved from JWT orgID via TenantMiddleware

---

*Architecture analysis: 2026-01-31*
