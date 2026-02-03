# External Integrations

**Analysis Date:** 2026-02-03

## APIs & External Services

**PDF Generation:**
- wkhtmltopdf - System command for rendering HTML to PDF
  - SDK/Client: System binary (called via exec)
  - Location: `/backend/internal/service/pdf_wkhtmltopdf.go`
  - Usage: Quote PDF templates, custom PDF rendering
  - Configuration: Must be installed in system (`apt-get install wkhtmltopdf`)

**Turso Platform API:**
- Service: Turso database provisioning and management
  - SDK/Client: `github.com/tursodatabase/libsql-client-go`
  - Auth: `TURSO_API_TOKEN` environment variable
  - Org: `TURSO_ORG` environment variable
  - Location: `/backend/internal/turso/` (custom client wrapper)
  - Usage: Creating per-organization tenant databases, multi-tenant isolation
  - Endpoint: Turso API (details in `turso/client.go`)

## Data Storage

**Primary Database:**
- Type: SQLite (via Turso in production, local file in development)
- Provider: Turso (managed SQLite edge) or self-hosted
- Connection:
  - Development: `TURSO_URL` empty, uses local file at `DATABASE_PATH`
  - Production: `TURSO_URL` and `TURSO_AUTH_TOKEN` for Turso connection
- Client: `github.com/mattn/go-sqlite3` (Go driver)
- ORM: None - raw SQL with `database/sql` package
- Location: `/backend/internal/db/` (database connection management)

**Master Database Tables:**
- `users` - User accounts with password hashes
- `organizations` - Tenant organizations
- `user_org_memberships` - Multi-org user assignments
- `entity_defs` - Custom entity definitions (configurable per-org)
- `field_defs` - Custom field definitions (configurable per-org)
- `layouts` - UI layout configurations for entities
- `navigation_configs` - Custom navigation menus
- `related_list_configs` - Related record list configurations
- `migrations` - Schema migration tracking

**Tenant Databases (Multi-Tenant):**
- One dedicated SQLite database per organization (via Turso)
- Tables: `contacts`, `accounts`, `tasks`, `quotes`, `custom_*` (dynamic entities)
- Master DB is shared; tenant DBs are isolated

**File Storage:**
- Local filesystem only - No S3 or cloud storage
- PDF templates and generated PDFs handled in-memory or temp files

**Caching:**
- None - No Redis, Memcached, or distributed cache
- In-memory caching via Go map types in service layer

## Authentication & Identity

**Auth Provider:**
- Custom implementation (no third-party OAuth)
  - Location: `/backend/internal/service/auth.go`
  - `/backend/internal/handler/auth.go`

**Implementation Approach:**
- Email/password authentication with bcrypt hashing
- JWT tokens (HS256 algorithm)
  - Access token: 24 hour expiry
  - Refresh token: 7 day expiry
- Invitation tokens for user onboarding
- API tokens for programmatic access (read-only for now)

**Token Validation:**
- JWT validation on protected routes via middleware
- Location: `/backend/internal/middleware/auth_middleware.go`
- Platform admin impersonation support (for debugging/support)

## Monitoring & Observability

**Error Tracking:**
- None configured - Errors logged to stdout/stderr only

**Logs:**
- Fiber logger middleware logs all HTTP requests
- Custom log statements throughout codebase
- Environment-aware error response (production vs development)
- Security: Errors sanitized in production (no stack traces returned to clients)

## CI/CD & Deployment

**Hosting:**
- Backend: Railway (Git-based auto-deployment)
- Frontend: Vercel (SvelteKit adapter-vercel for deployment)
- Database: Turso (managed SQLite) or self-hosted

**CI Pipeline:**
- None configured - Deployment via git push to main branch
- Railway and Vercel watch main branch automatically

**Deployment Flow:**
1. Push to `main` branch in Git
2. Railway automatically pulls backend changes and runs `go build`
3. Vercel automatically pulls frontend changes and runs `npm run build`
4. Production database migrations run via Railway deployment hooks

## Environment Configuration

**Required env vars (Production):**
- `JWT_SECRET` - Secret key for signing JWT tokens (CRITICAL)
- `TURSO_URL` - Turso database connection URL
- `TURSO_AUTH_TOKEN` - Turso API authentication token
- `TURSO_ORG` - Turso organization name
- `TURSO_API_TOKEN` - Turso Platform API token (for provisioning)
- `ENVIRONMENT` - Set to "production" to enable strict error handling

**Optional env vars:**
- `DATABASE_PATH` - Local SQLite file path (dev only, default: `../fastcrm.db`)
- `PORT` - API server port (default: 8080)
- `ALLOWED_ORIGINS` - CORS allowed origins (default: `*` in dev, must be set in prod)
- `GO_ENV` - Set to "development" to disable rate limiting
- `PUBLIC_API_URL` - Frontend API base URL (default: `/api/v1`)

**Secrets Location:**
- `.env` files (local development only, NOT committed)
- Environment variables on deployment platform (Railway/Vercel)
- Backend: `/backend/.env` (git-ignored)
- Frontend: `/frontend/.env` (git-ignored)

## Webhooks & Callbacks

**Incoming Webhooks:**
- None configured

**Outgoing Webhooks:**
- None configured
- Tripwires (trigger-based automation) are internal only

---

*Integration audit: 2026-02-03*
