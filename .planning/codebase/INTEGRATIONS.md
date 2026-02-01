# External Integrations

**Analysis Date:** 2026-01-31

## APIs & External Services

**PDF Generation:**
- wkhtmltopdf - Converts HTML to PDF for quote rendering
  - SDK/Client: System binary (installed in Docker via apt-get)
  - Usage: `internal/service/pdf_renderer.go` - Quote PDF generation and download
  - Config: Dockerfile installs `wkhtmltopdf` and required fonts (DejaVu, FreeFont)

**None Detected:**
- No Stripe/PayPal payment integration
- No external email service (SMTP not configured)
- No Slack/Twilio messaging integration
- No Sentry/DataDog error tracking
- No third-party authentication providers (Google OAuth, Okta, etc.)

## Data Storage

**Databases:**
- Turso (SQLite edge) - Production master database
  - Connection: Via `TURSO_URL` and `TURSO_AUTH_TOKEN` environment variables
  - Client: `github.com/tursodatabase/libsql-client-go` - HTTP-based SQLite driver
  - Connection pattern: Auto-reconnecting wrapper in `internal/db/turso.go`

- SQLite 3.x - Local development database
  - Connection: `DATABASE_PATH` env var (default: `../fastcrm.db`)
  - Client: `github.com/mattn/go-sqlite3` - CGO SQLite bindings
  - Multi-tenancy: Shared database in local mode, per-org databases in Turso mode

**File Storage:**
- Local filesystem only - PDFs generated on-the-fly, not persisted
- Quote PDF templates stored in database (pdf_templates table)

**Caching:**
- None detected - All data fetched fresh from database
- No Redis, Memcached, or browser caching configured

## Authentication & Identity

**Auth Provider:**
- Custom JWT-based authentication
  - Implementation: `internal/service/auth.go` - Custom user registration and login
  - Token generation: `github.com/golang-jwt/jwt/v5` for JWT creation/validation
  - Token storage: localStorage on frontend (`STORAGE_KEY: 'quantico_auth'`)
  - Token format: Bearer token in Authorization header

**Password Security:**
- Hashing: `golang.org/x/crypto` for bcrypt password hashing
- Validation middleware: `internal/middleware/auth.go` validates JWT and API tokens
- Session management: JWT expiration and automatic 401 redirect on expired session

**API Token Support:**
- Custom API tokens prefixed with `fcr_`
- Service: `internal/service/api_token.go`
- Use case: Non-browser client authentication (scripts, webhooks)

## Monitoring & Observability

**Error Tracking:**
- None detected - No Sentry, DataDog, or similar integration
- Error logging: Standard Go logging with correlation IDs in production
- Production error handling: Sanitized error responses (error ID returned, full details logged)

**Logs:**
- Standard output logging via Go's `log` package
- Fiber middleware logging: Request/response logging via middleware/logger
- Backend logs: Docker container stdout (captured by Railway/hosting platform)
- Frontend errors: Browser console (no centralized log aggregation)

## CI/CD & Deployment

**Hosting:**
- Backend: Railway (Go/Fiber API running Docker container)
- Frontend: Vercel (SvelteKit/Node.js app)
- Database: Turso managed service

**CI Pipeline:**
- None detected - Git push to main branch triggers auto-deployment
- Deployment flow: Push to `main` → Railway/Vercel auto-deploy
- No explicit GitHub Actions or CI service found in codebase

**Build Process:**
- Backend: Docker multi-stage build (Go 1.22 → Debian slim binary)
- Frontend: Vite build producing static assets for Vercel

## Environment Configuration

**Required env vars (Backend):**
- `PORT` - Server port (8080 default)
- `GO_ENV` - Environment mode
- `JWT_SECRET` - **REQUIRED in production** (fails on startup if missing in prod)
- `TURSO_URL` + `TURSO_AUTH_TOKEN` - For Turso connection (production)
- `TURSO_API_TOKEN` - For creating per-org databases
- `TURSO_ORG` - Turso organization name

**Required env vars (Frontend):**
- `PUBLIC_API_URL` - API endpoint (/api/v1 local, full URL production)

**Optional env vars:**
- `DATABASE_PATH` - Local SQLite path (dev only)
- `ENVIRONMENT` - Alternative to GO_ENV for production flag
- `ALLOWED_ORIGINS` - CORS whitelist (default: * in dev, critical to set in prod)

**Secrets location:**
- Backend: Environment variables (Railway, production) or .env file (development)
- Frontend: Vercel environment variables (PUBLIC_API_URL is public)
- Database: Turso auth token stored in backend env only

## Webhooks & Callbacks

**Incoming:**
- None detected - No webhook endpoints for external services
- Flow engine has webhook scaffolding but no active webhook service

**Outgoing:**
- None detected - No outbound webhooks to third parties

## Multi-Tenant Architecture

**Database per Organization:**
- Service: `internal/service/tenant_provisioning.go`
- Turso API: Creates new database per organization using `TURSO_API_TOKEN`
- Local mode: Shared database with `org_id` isolation
- Auto-provisioning: Turso database created on first org user login

**Metadata Provisioning:**
- Service: `internal/service/provisioning.go`
- Includes: Default entities, fields, layouts, navigation tabs, bearings
- Trigger: New organization creation or "Repair Metadata" admin action
- Re-run safety: Uses INSERT OR REPLACE (idempotent)

## Inter-Service Communication

**Backend-to-Frontend:**
- HTTP/REST via Fiber API (`/api/v1` endpoints)
- Vite dev proxy: `localhost:8080` → `/api/v1` during development
- Production: Separate Railway and Vercel deployments, same-origin requests

**Internal Services:**
- All Go services share single `*sql.DB` or Turso connection
- Service composition via dependency injection in `cmd/api/main.go`
- Example: AuthService → AuthRepo → Database

---

*Integration audit: 2026-01-31*
