# Technology Stack

**Analysis Date:** 2026-02-03

## Languages

**Primary:**
- Go 1.22.0 - Backend API server and CLI tools
- TypeScript 5.0+ - Frontend development
- Svelte 5.0.0 - UI component framework

**Secondary:**
- SQL - Database schema and migrations
- Bash - Build and utility scripts

## Runtime

**Environment:**
- Go 1.22.0 - Backend server runtime
- Node.js 18+ (assumed, no .nvmrc file) - Frontend build tooling

**Package Managers:**
- Go modules (`go.mod`) - Go dependency management
- npm - JavaScript package management
- Lockfile: `go.sum` present, `package-lock.json` present

## Frameworks

**Core:**
- Fiber v2.52.0 - HTTP web framework (Go) - REST API server with routing, middleware, and CORS support
- SvelteKit 2.0.0 - Meta-framework for Svelte - Server-side rendering, routing, API proxy
- Vite 5.0.0 - Build tool and dev server - Frontend build bundling and HMR

**UI/Styling:**
- Tailwind CSS 3.4.0 - Utility-first CSS framework
- PostCSS 8.4.32 - CSS transformation
- Autoprefixer 10.4.16 - Browser prefix automation

**Testing:**
- None configured in package.json or go.mod

**Build/Dev:**
- SvelteKit Vite plugin 4.0.0 - Svelte integration with Vite
- Adapter-auto 3.0.0 - Auto-adapter for platform detection
- Adapter-vercel 5.0.0 - Deployment adapter for Vercel
- svelte-check 4.0.0 - Type checker for Svelte

## Key Dependencies

**Critical - Backend:**
- `github.com/tursodatabase/libsql-client-go v0.0.0-20240902231107-85af5b9d094d` - Turso database client (SQLite edge)
- `github.com/mattn/go-sqlite3 v1.14.22` - SQLite driver for Go
- `github.com/golang-jwt/jwt/v5 v5.2.1` - JWT token generation and validation
- `golang.org/x/crypto v0.31.0` - Bcrypt password hashing and crypto utilities

**Critical - Frontend:**
- `marked ^11.2.0 - Markdown parsing for rich text rendering

**Infrastructure:**
- `github.com/gofiber/fiber/v2 v2.52.0` - HTTP routing and middleware
- `github.com/google/uuid v1.6.0` - UUID generation for IDs
- `github.com/joho/godotenv v1.5.1` - .env file loading for local development
- `golang.org/x/mod v0.17.0` - Go module utilities

**Middleware/Utilities:**
- `github.com/valyala/fasthttp v1.51.0` - High-performance HTTP library (used by Fiber)
- `github.com/klauspost/compress v1.17.0` - Compression algorithms (gzip, brotli)

## Configuration

**Environment:**
- `.env` files for configuration (development and production)
- Backend expects: `TURSO_URL`, `TURSO_AUTH_TOKEN`, `JWT_SECRET`, `DATABASE_PATH`, `ENVIRONMENT`, `PORT`, `ALLOWED_ORIGINS`
- Frontend expects: `PUBLIC_API_URL` (in SvelteKit .env format)
- Configuration reads in this order: `.env`, `../.env`, `../../.env` (local dev support)

**Build:**
- `vite.config.ts` - Frontend build and dev server config
- `tsconfig.json` - TypeScript compiler settings
- `Dockerfile` - Multi-stage build for production deployment
- `go.mod` / `go.sum` - Go module versioning

## Platform Requirements

**Development:**
- Go 1.22+
- Node.js 18+ (for npm and Vite)
- SQLite (local development database)
- wkhtmltopdf binary (for PDF generation)

**Production:**
- Go 1.22+ runtime
- Turso (SQLite edge) or local SQLite database
- wkhtmltopdf system package
- Node.js not required (frontend built to static assets)

**Deployment Targets:**
- Backend: Railway (Git-based deployment)
- Frontend: Vercel (SvelteKit adapter-vercel)
- Database: Turso (managed SQLite edge) or self-hosted SQLite

## Development Commands

**Backend:**
```bash
cd backend && air                           # Run with auto-reload (requires air installed)
go run cmd/api/main.go                      # Run API server directly
go run cmd/migrate/main.go                  # Run migrations
```

**Frontend:**
```bash
cd frontend && npm run dev                  # Start Vite dev server on port 5173
npm run build                               # Build for production
npm run preview                             # Preview production build
npm run check                               # Type-check Svelte components
```

## Database Schema

**Master Database:**
- Stores: User accounts, organizations, invitations, metadata (field definitions, entity definitions, layouts)
- Access: All API endpoints read/write to master for auth and config
- Default location (dev): `../fastcrm.db` (SQLite)
- Production: Turso database specified by `TURSO_URL` and `TURSO_AUTH_TOKEN`

**Tenant Databases (per-organization):**
- Stores: Contact, Account, Task, Quote, and custom entity data
- Provisioned per organization in multi-tenant mode (Turso)
- Shared master database in local mode (development)

---

*Stack analysis: 2026-02-03*
