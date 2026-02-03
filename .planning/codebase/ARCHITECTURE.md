# Architecture

**Analysis Date:** 2026-02-03

## Pattern Overview

**Overall:** Layered multi-tenant SaaS architecture with clear separation between HTTP routing, business logic, data access, and domain models.

**Key Characteristics:**
- Multi-tenant isolation: One database per organization (Turso) or shared database in local mode
- Handler → Service → Repository → Domain pattern
- Master database for auth and metadata; tenant databases for organizational data
- Fiber (Go) backend with SvelteKit frontend communicating via REST API
- Reactive frontend state management using Svelte 5 runes

## Layers

**HTTP Layer (Fiber):**
- Purpose: Accept HTTP requests, validate input, serialize responses
- Location: `backend/cmd/api/main.go` (app setup and routing)
- Contains: Route group definitions, middleware application, error handling
- Depends on: Middleware, Handlers
- Used by: Frontend (SvelteKit), external API clients

**Middleware Layer:**
- Purpose: Authentication, tenant resolution, authorization
- Location: `backend/internal/middleware/`
- Contains: `auth.go` (JWT/API token validation, org context), tenant middleware (DB routing)
- Depends on: Service, DB Manager
- Used by: Fiber route groups (protected, admin, platform-admin)

**Handler Layer:**
- Purpose: Translate HTTP input to domain operations, orchestrate across services
- Location: `backend/internal/handler/`
- Contains: `contact.go`, `account.go`, `task.go`, `quote.go`, `auth.go`, `admin.go`, etc.
- Depends on: Repository, Service
- Used by: Fiber app routing

**Service Layer:**
- Purpose: Business logic, cross-cutting concerns, external integrations
- Location: `backend/internal/service/`
- Contains: `auth.go` (login, JWT, provisioning), `provisioning.go` (metadata setup), `tripwire.go` (automation), `validation.go` (field validation), `pdf_template.go` (PDF generation)
- Depends on: Repository, Entity
- Used by: Handler layer

**Repository Layer:**
- Purpose: Database access, query abstraction, transaction handling
- Location: `backend/internal/repo/`
- Contains: `contact.go`, `account.go`, `metadata.go`, `auth.go`, etc. - one per entity type or concern
- Depends on: Database layer (DB interface), Entity, SFID (ID generation)
- Used by: Service, Handler

**Database Layer:**
- Purpose: Connection pooling, multi-tenant routing, retry logic for Turso
- Location: `backend/internal/db/`
- Contains: `manager.go` (tenant DB routing), `turso.go` (Turso with auto-reconnect), interface definitions
- Depends on: SQLite/Turso drivers
- Used by: Repository layer

**Domain Layer:**
- Purpose: Type definitions, entity schemas
- Location: `backend/internal/entity/`
- Contains: `contact.go`, `account.go`, `user.go`, `organization.go`, etc. - Go structs with JSON tags
- Depends on: Nothing
- Used by: All layers above

**Frontend Layer:**
- Purpose: UI rendering, state management, API communication
- Location: `frontend/src/`
- Contains: Svelte pages, stores, components, types, utilities
- Depends on: API utilities, types
- Used by: Browser

## Data Flow

**CRUD Operation (Create Contact):**

1. Frontend: User fills form at `/contacts/new`
2. Frontend: JavaScript calls `POST /api/v1/contacts` with JSON body
3. Handler: `ContactHandler.Create()` receives request, extracts orgID from context
4. Service: Business logic validates (tripwire checks, custom field validation)
5. Repository: `ContactRepo.Create()` generates SFID, serializes custom fields, executes INSERT
6. Database: Query executed on tenant DB (routed by DB Manager based on orgID)
7. Repository: Returns hydrated Contact entity
8. Handler: Serializes to JSON, returns 201 Created
9. Frontend: Receives response, updates optimistic UI, shows success toast

**Detail Page Load (View Contact):**

1. Frontend: `GET /contacts/[id]` SvelteKit route
2. Frontend: Parallel requests:
   - `GET /api/v1/contacts/{id}` → contact data
   - `GET /api/v1/entities/Contact/fields` → field definitions
   - `GET /api/v1/entities/Contact/layouts/detail` → layout config
   - `GET /api/v1/contacts/{id}/related-lists` → related records configs
3. Handler: Resolves tenant DB from context, executes queries in parallel
4. Frontend: SectionRenderer component renders layout sections based on layout config
5. Frontend: RelatedList components render configured related records

**State Management:**
- Frontend: Svelte 5 runes for reactive state (`$state`, `$derived`)
- Auth store persists to localStorage, auto-refreshes tokens
- Navigation store caches org navigation tabs
- Components use optimistic updates with rollback on error

## Key Abstractions

**Handler Pattern:**
- Purpose: Unified HTTP request handling with tenant isolation
- Examples: `backend/internal/handler/contact.go`, `backend/internal/handler/account.go`
- Pattern: Handler receives Fiber context, extracts orgID/user from middleware-set context locals, routes to repo via tenant DB

**Repository Pattern:**
- Purpose: Data access abstraction, query building, result transformation
- Examples: `backend/internal/repo/contact.go`, `backend/internal/repo/metadata.go`
- Pattern: Repo methods accept `context.Context`, database connection, parameters; return domain entities or errors

**Service Pattern:**
- Purpose: Business logic isolation
- Examples: `backend/internal/service/auth.go`, `backend/internal/service/validation.go`, `backend/internal/service/provisioning.go`
- Pattern: Services coordinate repos and handle cross-cutting concerns

**Multi-Tenant Routing:**
- Purpose: Isolate data between organizations
- Location: `backend/internal/middleware/tenant.go`, `backend/internal/db/manager.go`
- Pattern: Auth middleware sets orgID in context.Locals; tenant middleware resolves to actual DB; repos use tenant DB from context

**Layout System:**
- Purpose: Customizable detail/list page layouts per organization
- Examples: `backend/internal/repo/metadata.go` (stores), `backend/internal/handler/metadata.go` (serves)
- Pattern: Layouts stored as JSON; frontend fetches and renders dynamically (not hardcoded)

**Provisioning:**
- Purpose: Auto-setup of new organizations
- Location: `backend/internal/service/provisioning.go`
- Pattern: On org creation, provision default entities (Contact, Account, Task), field defs, layouts, navigation

## Entry Points

**Backend API Server:**
- Location: `backend/cmd/api/main.go`
- Triggers: `air` (dev) or `./api` (production)
- Responsibilities: Database connection, middleware setup, service initialization, route registration, HTTP server startup

**Frontend SvelteKit App:**
- Location: `frontend/src/routes/`
- Triggers: `npm run dev` (dev) or `npm run build && preview` (build)
- Responsibilities: Client-side routing, page rendering, API communication, authentication flow

**Database Migrations:**
- Location: `backend/cmd/migrate/main.go`
- Triggers: `go run cmd/migrate/main.go`
- Responsibilities: Schema creation and upgrades across master and tenant databases

## Error Handling

**Strategy:** Layered error propagation with context-specific handling

**Patterns:**
- Repository layer: Returns `(*Entity, error)` - database errors propagate up
- Service layer: Wraps repo errors with context, returns validation errors as custom types
- Handler layer: Catches errors, logs with orgID/userID, returns appropriate HTTP status + JSON error
- Middleware: Auth failures → 401, permission failures → 403
- Frontend: Catches API errors, shows toast notifications, field validation errors inline in forms

## Cross-Cutting Concerns

**Logging:**
- Backend: Fiber middleware logs all requests; handlers log with context (orgID, userID, action)
- Frontend: Console logging (no persistent logs)

**Validation:**
- Field validation: `backend/internal/service/validation.go` - rules applied on Create/Update
- API validation: Fiber context.BodyParser validates JSON schema
- Frontend: TypeScript types + custom field validators

**Authentication:**
- Backend: JWT tokens (access + refresh), API tokens for system integrations
- Token validation in `AuthMiddleware` middleware
- Session tracking in auth_sessions table
- Frontend: localStorage persists tokens, auto-refresh on request

**Authorization:**
- Role-based: `orgAdminRequired()`, `platformAdminRequired()` middleware
- Org isolation: All queries filtered by orgID from context
- Resource ownership: Tasks/contacts can have assigned users

---

*Architecture analysis: 2026-02-03*
