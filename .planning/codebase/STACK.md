# Technology Stack

**Analysis Date:** 2026-01-31

## Languages

**Primary:**
- Go 1.22+ - Backend API (Fiber framework)
- TypeScript 5.x - Frontend type safety and tooling
- JavaScript/Svelte 5.x - Frontend UI framework

**Secondary:**
- SQL - Database schema and migrations (SQLite/Turso dialect)
- HTML/CSS - Component markup and styling

## Runtime

**Environment:**
- Go runtime - Linux (Debian bookworm-slim) in Docker
- Node.js 18+ - Frontend development and build
- Docker - Containerization for backend deployment

**Package Manager:**
- go modules - Go dependency management (go.mod, go.sum)
- npm - Node.js package management (package.json, package-lock.json)
- Lockfile: Both present (go.sum ~7KB, package-lock.json ~131KB)

## Frameworks

**Core:**
- Fiber v2.52.0 - Go HTTP framework, lightweight fast web server
- SvelteKit 2.x - Frontend meta-framework built on Svelte 5.0
- Svelte 5.0 - Reactive component framework with runes ($state, $derived)

**Testing:**
- Not explicitly configured in package.json; custom tests in `/backend/tests/` directory

**Build/Dev:**
- Vite 5.x - Frontend bundler and dev server
- SvelteKit adapter-vercel 5.x - Vercel deployment adapter
- SvelteKit adapter-auto 3.x - Auto-detection adapter
- TypeScript - Type checking for frontend
- Tailwind CSS 3.4 - Utility-first CSS framework
- PostCSS 8.4 - CSS processing (for Tailwind)
- Autoprefixer 10.4 - CSS vendor prefix support

## Key Dependencies

**Critical:**
- github.com/gofiber/fiber/v2 v2.52.0 - HTTP server and routing
- github.com/golang-jwt/jwt/v5 v5.2.1 - JWT token generation and validation for auth
- golang.org/x/crypto v0.31.0 - Password hashing (bcrypt), cryptographic primitives
- github.com/tursodatabase/libsql-client-go v0.0.0-20240902231107-85af5b9d094d - Turso edge database client

**Infrastructure:**
- github.com/mattn/go-sqlite3 v1.14.22 - SQLite driver (local dev database)
- github.com/google/uuid v1.6.0 - UUID generation for entity IDs
- github.com/joho/godotenv v1.5.1 - Environment variable loading (.env files)

**Middleware/HTTP:**
- gofiber/middleware/recover - Panic recovery
- gofiber/middleware/logger - Request logging
- gofiber/middleware/cors - Cross-origin request handling
- gofiber/middleware/limiter - Rate limiting (100 req/min per IP)
- coder/websocket v1.8.12 - WebSocket support (for flows/real-time)

**Frontend:**
- marked v11.2.0 - Markdown parsing and rendering
- (Svelte/Vite ecosystem: brotli, compression, etc. all standard)

## Configuration

**Environment:**
Backend (.env or environment variables):
- `GO_ENV` - Environment mode (development/production), controls rate limiting
- `ENVIRONMENT` - Alternative environment indicator (production/prod triggers security modes)
- `PORT` - Server port (default: 8080)
- `DATABASE_PATH` - Local SQLite path (default: ../fastcrm.db) for development
- `TURSO_URL` - Turso database connection URL (production master database)
- `TURSO_AUTH_TOKEN` - Turso authentication token
- `TURSO_API_TOKEN` - Turso API token for database creation
- `TURSO_ORG` - Turso organization name for provisioning per-org databases
- `JWT_SECRET` - Signing key for JWT tokens (required in production)
- `ALLOWED_ORIGINS` - CORS origin whitelist (default: * in dev, must be set in prod)

Frontend (.env or $env/static/public):
- `PUBLIC_API_URL` - API base URL (default: /api/v1 for local dev proxy, full URL in prod)

**Build:**
- `vite.config.ts` - Vite dev server proxy configuration
- `svelte.config.js` - SvelteKit configuration, Vercel adapter setup
- `tsconfig.json` - TypeScript compiler options (strict mode, ES modules)
- `tailwind.config.js` - Tailwind CSS configuration
- `postcss.config.js` - PostCSS plugins
- `go.mod` / `go.sum` - Go module dependencies
- `Dockerfile` - Multi-stage Docker build for backend API binary

## Platform Requirements

**Development:**
- Go 1.22+ (for backend compilation)
- Node.js 18+ (for frontend build/dev)
- SQLite 3.x (for local database, embedded in go-sqlite3)
- Docker (optional, for containerized backend)
- git (for version control)

**Production:**
- Backend: Railway (Go/Fiber API) - runs Docker container
- Frontend: Vercel (SvelteKit) - serverless deployment
- Master Database: Turso (edge SQLite via libsql HTTP client)
- Per-org Databases: Turso (one database created per organization)

---

*Stack analysis: 2026-01-31*
