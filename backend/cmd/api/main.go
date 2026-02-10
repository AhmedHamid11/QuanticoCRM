package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/fastcrm/backend/internal/config"
	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/dedup"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/flow"
	"github.com/fastcrm/backend/internal/handler"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func init() {
	// Load .env file if it exists (development only)
	// In production, environment variables should be set directly
	// Try multiple locations since app may run from different directories
	envPaths := []string{".env", "../.env", "../../.env"}
	loaded := false
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			log.Printf("Loaded environment from %s", path)
			loaded = true
			break
		}
	}
	if !loaded {
		log.Println("No .env file found, using environment variables")
	}
}

func main() {
	// SECURITY: Load and validate configuration FIRST (fails fast on missing required config)
	cfg := config.Load()

	var masterDB *sql.DB
	var masterDBConn db.DBConn // Interface for repos that support reconnection
	var tursoDB *db.TursoDB   // TursoDB wrapper for Turso connections
	var err error

	// Check for Turso connection (production master database)
	tursoURL := cfg.TursoURL
	tursoToken := cfg.TursoToken

	if tursoURL != "" && tursoToken != "" {
		// Production: Connect to Turso master database using TursoDB wrapper
		// TursoDB provides automatic reconnection for the libsql HTTP driver
		tursoDB, err = db.NewTursoDB(tursoURL, tursoToken)
		if err != nil {
			log.Fatalf("Failed to connect to Turso: %v", err)
		}
		defer tursoDB.Close()

		// Get the underlying *sql.DB for repos that still need it
		masterDB = tursoDB.GetDB()
		// Use TursoDB (with auto-reconnect) for repos that support DBConn interface
		masterDBConn = tursoDB

		log.Printf("Connected to Turso master database with auto-reconnect: %s (v4)", tursoURL)
	} else {
		// Development: Use local SQLite as master database
		dbPath := os.Getenv("DATABASE_PATH")
		if dbPath == "" {
			dbPath = "../fastcrm.db"
		}
		masterDB, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		defer masterDB.Close()

		// For local SQLite, use *sql.DB directly (it satisfies DBConn interface)
		masterDBConn = masterDB

		log.Printf("Using local database: %s", dbPath)

		// Local SQLite settings
		masterDB.SetMaxOpenConns(25)
		masterDB.SetMaxIdleConns(10)
		masterDB.SetConnMaxLifetime(5 * time.Minute)
		log.Println("Connection pool configured: MaxOpen=25, MaxIdle=10, MaxLifetime=5m")
	}

	// Get JWT secret from centralized config (validated at startup)
	jwtSecret := cfg.GetJWTSecret()

	// Initialize repositories
	// Note: Repos that access master DB metadata use DBConn interface for auto-reconnect support with Turso
	// Data repos (contact, account, task, quote) use masterDB as they get tenant DB via middleware
	contactRepo := repo.NewContactRepo(masterDB)
	accountRepo := repo.NewAccountRepo(masterDB)
	taskRepo := repo.NewTaskRepo(masterDB)
	// Metadata repos use masterDBConn (with retry logic) since they read from master DB
	metadataRepo := repo.NewMetadataRepo(masterDBConn)
	// Ensure metadata schema has all required columns (handles older databases)
	if err := metadataRepo.EnsureSchema(context.Background()); err != nil {
		log.Printf("Warning: Failed to ensure metadata schema: %v", err)
	}

	// Ensure submittals table has client_id column (migration 040)
	ensureSubmittalsClientID(masterDB)
	// Ensure sessions table has timeout columns (migration 046, 047)
	ensureSessionsColumns(masterDB)
	navigationRepo := repo.NewNavigationRepo(masterDBConn)
	relatedListRepo := repo.NewRelatedListRepo(masterDBConn, metadataRepo) // Uses DBConn for auto-reconnect
	tripwireRepo := repo.NewTripwireRepo(masterDB)
	bearingRepo := repo.NewBearingRepo(masterDBConn, metadataRepo)
	validationRepo := repo.NewValidationRepo(masterDB)
	authRepo := repo.NewAuthRepo(masterDBConn)      // Uses DBConn for auto-reconnect
	customPageRepo := repo.NewCustomPageRepo(masterDB)
	apiTokenRepo := repo.NewAPITokenRepo(masterDBConn) // Uses DBConn for auto-reconnect
	flowRepo := repo.NewFlowRepo(masterDB)
	listViewRepo := repo.NewListViewRepo(masterDB)
	quoteRepo := repo.NewQuoteRepo(masterDB)
	pdfTemplateRepo := repo.NewPdfTemplateRepo(masterDB)
	versionRepo := repo.NewVersionRepo(masterDBConn)
	migrationRepo := repo.NewMigrationRepo(masterDBConn)
	orgSettingsRepo := repo.NewOrgSettingsRepo(masterDBConn)
	auditRepo := repo.NewAuditRepo(masterDBConn)
	matchingRuleRepo := repo.NewMatchingRuleRepo(masterDBConn)
	pendingAlertRepo := repo.NewPendingAlertRepo(masterDBConn)
	mergeRepo := repo.NewMergeRepo(masterDBConn)
	scanJobRepo := repo.NewScanJobRepo(masterDBConn)
	notificationRepo := repo.NewNotificationRepo(masterDBConn)
	salesforceRepo := repo.NewSalesforceRepo(masterDBConn)
	ingestAPIKeyRepo := repo.NewIngestAPIKeyRepo(masterDBConn)
	mirrorRepo := repo.NewMirrorRepo(masterDBConn)
	ingestJobRepo := repo.NewIngestJobRepo(masterDBConn)
	deltaKeyRepo := repo.NewDeltaKeyRepo(masterDBConn)

	// Initialize dedup services
	defaultRegion := "US" // Default region for phone normalization
	detector := dedup.NewDetector(matchingRuleRepo, defaultRegion)
	realtimeChecker := dedup.NewRealtimeChecker(detector, pendingAlertRepo, matchingRuleRepo)
	importDuplicateService := service.NewImportDuplicateService(detector, matchingRuleRepo)

	// Initialize services
	auditLogger := service.NewAuditLogger(auditRepo)
	tripwireService := service.NewTripwireService(masterDB, tripwireRepo)
	validationService := service.NewValidationService(masterDB, validationRepo)

	// Initialize merge services (requires auditLogger)
	mergeDiscoveryService := service.NewMergeDiscoveryService(metadataRepo)
	mergeService := service.NewMergeService(mergeRepo, metadataRepo, mergeDiscoveryService, auditLogger)

	// Initialize notification service
	notificationService := service.NewNotificationService(notificationRepo, authRepo)

	// Initialize scan job service
	scanJobService := service.NewScanJobService(detector, scanJobRepo, pendingAlertRepo, matchingRuleRepo, authRepo)
	scanJobService.SetNotificationService(notificationService)

	// Use masterDBConn (with retry logic) for provisioning to handle connection errors
	provisioningService := service.NewProvisioningService(masterDBConn)
	authConfig := service.DefaultAuthConfig(jwtSecret)
	authService := service.NewAuthService(authRepo, authConfig, provisioningService)
	apiTokenService := service.NewAPITokenService(apiTokenRepo)
	ingestAPIKeyService := service.NewIngestAPIKeyService(ingestAPIKeyRepo)
	ingestService := service.NewIngestService(mirrorRepo, ingestJobRepo, deltaKeyRepo, metadataRepo, masterDBConn)
	versionService := service.NewVersionService()

	// Initialize Salesforce OAuth service
	sfEncryptionKey, err := util.GetEncryptionKey()
	if err != nil {
		log.Printf("Warning: Salesforce encryption key not configured: %v", err)
		sfEncryptionKey = nil // Service will error on token operations
	}
	salesforceOAuthService := service.NewSalesforceOAuthService(salesforceRepo, sfEncryptionKey)

	// Initialize Salesforce delivery service components (Plan 03)
	payloadBuilder := service.NewMergeInstructionBuilder(salesforceRepo, metadataRepo)
	batchAssembler := service.NewBatchAssembler()

	// Initialize tenant provisioning service for per-org databases
	// This creates dedicated Turso databases for each new organization
	// Using masterDBConn (with retry logic) for metadata provisioning
	tenantProvisioningService := service.NewTenantProvisioningService(masterDBConn)
	authService.SetTenantProvisioning(tenantProvisioningService)
	authService.SetVersionRepo(versionRepo)
	if tenantProvisioningService.IsLocalMode() {
		log.Println("Tenant provisioning: LOCAL MODE (shared database)")
	} else {
		log.Println("Tenant provisioning: TURSO MODE (per-org databases)")
	}

	// Initialize DB manager for multi-tenant database routing
	dbManagerConfig := db.DefaultManagerConfig()
	var dbManager *db.Manager
	if tursoDB != nil {
		// Use TursoDB manager for auto-reconnect support
		dbManager = db.NewManagerWithTurso(tursoDB, dbManagerConfig)
	} else {
		// Local mode with regular *sql.DB
		dbManager = db.NewManager(masterDB, dbManagerConfig)
	}
	defer dbManager.Close()

	// Initialize Salesforce rate limiting service (Plan 18-01)
	rateLimitService := service.NewRateLimitService(dbManager, authRepo)

	// Initialize Salesforce delivery service (Plan 04, updated Plan 18-02, Plan 19-01)
	sfDeliveryService := service.NewSFDeliveryService(
		salesforceOAuthService,
		payloadBuilder,
		batchAssembler,
		salesforceRepo,
		dbManager,
		authRepo,
		rateLimitService,
		auditLogger,
	)

	// Initialize migration propagator for multi-tenant updates
	migrationPropagator := service.NewMigrationPropagator(
		masterDBConn,
		dbManager,
		versionRepo,
		migrationRepo,
		versionService,
	)

	// Run migration propagation BEFORE accepting requests
	// This blocks until all orgs are migrated (or fail with logging)
	log.Println("[STARTUP] Running migration propagation for all organizations...")
	propagationResult := migrationPropagator.PropagateAll(context.Background())
	log.Printf("[STARTUP] Migration propagation complete: %d success, %d failed, %d skipped",
		propagationResult.SuccessCount, propagationResult.FailedCount, propagationResult.SkippedCount)

	// Initialize scan scheduler
	scanScheduler, err := service.NewScanScheduler(scanJobService, scanJobRepo)
	if err != nil {
		log.Printf("Warning: Failed to initialize scan scheduler: %v", err)
	} else {
		// Start scheduler (loads all enabled schedules, begins gocron)
		if err := scanScheduler.Start(context.Background()); err != nil {
			log.Printf("Warning: Failed to start scan scheduler: %v", err)
		} else {
			log.Println("Scan scheduler started")
		}
		defer func() {
			if err := scanScheduler.Shutdown(); err != nil {
				log.Printf("Warning: Scan scheduler shutdown error: %v", err)
			}
			log.Println("Scan scheduler stopped")
		}()
	}

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService, apiTokenService)
	authMiddleware.SetAutoProvisioning(metadataRepo, provisioningService) // Enable auto-provisioning for orgs missing metadata

	// Session timeout middleware (SESS-01, SESS-02)
	sessionTimeoutMiddleware := middleware.NewSessionTimeoutMiddleware(middleware.SessionTimeoutConfig{
		AuthService: authService,
		// Skip activity tracking for refresh and extend-session endpoints
		SkipActivityUpdate: []string{"/auth/refresh", "/auth/extend-session"},
	})

	tenantMiddleware := middleware.NewTenantMiddleware(dbManager, authRepo)
	ingestAuthMiddleware := middleware.NewIngestAuthMiddleware(ingestAPIKeyService, dbManager, authRepo)

	// Initialize handlers
	contactHandler := handler.NewContactHandler(contactRepo, taskRepo, authRepo, tripwireService, validationService, realtimeChecker, masterDBConn)
	accountHandler := handler.NewAccountHandler(accountRepo, taskRepo, masterDB, metadataRepo, authRepo, tripwireService, validationService)
	taskHandler := handler.NewTaskHandler(taskRepo, authRepo, tripwireService, validationService)
	adminHandler := handler.NewAdminHandlerWithManager(masterDB, dbManager, metadataRepo, navigationRepo)
	adminHandler.SetProvisioningService(provisioningService) // Enable re-provisioning endpoint
	navigationHandler := handler.NewNavigationHandler(navigationRepo)
	lookupHandler := handler.NewLookupHandler(masterDB, metadataRepo)
	relatedHandler := handler.NewRelatedHandler(masterDB)
	relatedListHandler := handler.NewRelatedListHandler(relatedListRepo, metadataRepo, masterDB)
	genericEntityHandler := handler.NewGenericEntityHandler(masterDB, metadataRepo, authRepo, tripwireService, validationService, realtimeChecker)
	genericEntityHandler.SetProvisioningService(provisioningService)
	dataExplorerHandler := handler.NewDataExplorerHandler(masterDB)
	tripwireHandler := handler.NewTripwireHandler(tripwireRepo)
	bearingHandler := handler.NewBearingHandler(bearingRepo)
	validationHandler := handler.NewValidationHandler(validationRepo, validationService)
	cookieConfig := middleware.CookieConfig{
		IsProduction: cfg.IsProduction(),
		Domain:       cfg.CookieDomain, // e.g., ".quanticocrm.com" for subdomain sharing
	}
	authHandler := handler.NewAuthHandler(authService, auditLogger, cookieConfig)
	userHandler := handler.NewUserHandler(authRepo, auditLogger)
	apiTokenHandler := handler.NewAPITokenHandler(apiTokenService)
	bulkHandler := handler.NewBulkHandler(masterDB, metadataRepo, tripwireService, validationService)
	importHandler := handler.NewImportHandler(masterDB, metadataRepo, tripwireService, validationService, importDuplicateService)
	metadataHandler := handler.NewMetadataHandler(metadataRepo)
	customPageHandler := handler.NewCustomPageHandler(customPageRepo)
	listViewHandler := handler.NewListViewHandler(listViewRepo)
	quoteHandler := handler.NewQuoteHandler(quoteRepo, authRepo, tripwireService, validationService)
	pdfTemplateHandler := handler.NewPdfTemplateHandler(pdfTemplateRepo)
	versionHandler := handler.NewVersionHandler(versionRepo, versionService, migrationRepo, authRepo)
	schemaHandler := handler.NewSchemaHandler(masterDB, metadataRepo)
	orgSettingsHandler := handler.NewOrgSettingsHandler(orgSettingsRepo, auditLogger)
	auditHandler := handler.NewAuditHandler(auditRepo)
	dedupHandler := handler.NewDedupHandler(masterDBConn, matchingRuleRepo, pendingAlertRepo)
	mergeHandler := handler.NewMergeHandler(masterDB, mergeRepo, mergeService, mergeDiscoveryService, metadataRepo)
	scanJobHandler := handler.NewScanJobHandler(masterDB, scanJobRepo, notificationRepo, scanScheduler, scanJobService)
	salesforceHandler := handler.NewSalesforceHandler(salesforceOAuthService, sfDeliveryService, rateLimitService, salesforceRepo)
	ingestRateLimiter := service.NewIngestRateLimiter()
	ingestHandler := handler.NewIngestHandler(ingestService, mirrorRepo, ingestJobRepo, deltaKeyRepo, ingestRateLimiter)
	ingestKeyHandler := handler.NewIngestAPIKeyHandler(ingestAPIKeyService)
	mirrorHandler := handler.NewMirrorHandler(mirrorRepo, ingestJobRepo)

	// Wire migration propagator to version handler (created earlier in startup)
	versionHandler.SetMigrationPropagator(migrationPropagator)

	// Initialize PDF services
	pdfTemplateService := service.NewPdfTemplateService(quoteRepo, pdfTemplateRepo)
	pdfRenderer := service.NewWkhtmltopdfRenderer()
	quoteHandler.SetPdfServices(pdfTemplateRepo, pdfTemplateService, pdfRenderer)

	// Initialize flow engine with entity service adapter
	flowEntityService := service.NewFlowEntityService(masterDB, metadataRepo)
	flowEngine := flow.NewEngine(flowRepo, flowEntityService, nil) // No webhook service for now
	flowHandler := handler.NewFlowHandler(flowEngine, flowRepo)

	// Create Fiber app with environment-aware error handling
	app := fiber.New(fiber.Config{
		// SECURITY: Maximum request body size (10MB for upload endpoints)
		// Individual routes can apply stricter limits via BodyLimit middleware
		BodyLimit: middleware.UploadBodyLimit,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Check if it's already an APIError (from handlers using NewAPIError)
			if apiErr, ok := err.(*util.APIError); ok {
				return c.Status(apiErr.StatusCode).JSON(apiErr)
			}

			// Classify and sanitize unknown errors
			category := util.ClassifyError(err)
			requestID := util.GenerateErrorID()

			// Log full error for debugging with request_id for support correlation
			log.Printf("[ERROR %s] [%s] %v", requestID, category, err)

			// SECURITY: Sanitize error messages in production to prevent information disclosure
			if cfg.IsProduction() {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":      util.GetCategoryMessage(category),
					"request_id": requestID,
				})
			}

			// In development, include full error details for debugging
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":      util.GetCategoryMessage(category),
				"details":    err.Error(),
				"request_id": requestID,
			})
		},
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New())

	// SECURITY: HSTS header on all responses (CRIT-05)
	// Forces browsers to use HTTPS for 1 year
	app.Use(middleware.HSTS())

	// SECURITY: Security headers on all responses (HARD-01)
	// Protects against clickjacking, MIME sniffing, and other common attacks
	app.Use(middleware.SecurityHeaders())

	// SECURITY: Custom CORS middleware with origin validation (CRIT-01)
	// In production, non-allowlisted origins receive NO CORS headers (silent reject)
	app.Use(middleware.NewCORS(cfg.AllowedOrigins, cfg.IsDevelopment()))

	// SECURITY: CSRF protection for state-changing requests (SESS-03)
	// Protects against cross-site request forgery using double-submit cookie pattern
	app.Use(middleware.NewCSRFMiddleware(middleware.CSRFConfig{
		IsProduction: cfg.IsProduction(),
	}))

	// Rate limiting to prevent abuse and DDoS
	// 100 requests per minute per IP address in production
	// Disabled in development (GO_ENV=development) to avoid issues during rapid testing
	if os.Getenv("GO_ENV") != "development" {
		app.Use(limiter.New(limiter.Config{
			Max:        100,
			Expiration: 1 * time.Minute,
			KeyGenerator: func(c *fiber.Ctx) string {
				// Use X-Forwarded-For if behind a proxy, otherwise use remote IP
				forwarded := c.Get("X-Forwarded-For")
				if forwarded != "" {
					return forwarded
				}
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"error": "Rate limit exceeded. Please try again later.",
				})
			},
			SkipSuccessfulRequests: false,
			SkipFailedRequests:     false,
			Next: func(c *fiber.Ctx) bool {
				// Skip rate limiting for:
				// - Health checks (monitoring systems)
				// - OPTIONS preflight requests (CORS preflight should never be rate limited)
				return c.Path() == "/api/v1/health" || c.Method() == "OPTIONS"
			},
		}))
		log.Println("Rate limiting enabled: 100 requests/minute per IP")
	} else {
		log.Println("Rate limiting disabled (development mode)")
	}

	// API routes
	api := app.Group("/api/v1")

	// Health check (public)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "version": "v13"})
	})

	// Ingest API (external system auth via X-API-Key header, bypasses JWT)
	// This endpoint is separate from JWT auth chain - uses its own middleware
	ingest := api.Group("/ingest", ingestAuthMiddleware.Authenticate())
	ingestHandler.RegisterRoutes(ingest)

	// Salesforce OAuth callback (public - user redirected back from Salesforce)
	// State parameter provides CSRF protection (verified by service)
	salesforceHandler.RegisterCallbackRoute(api)

	// CRITICAL: Register stop-impersonate BEFORE the /auth group to ensure Fiber matches it first.
	// This route must use Required() middleware (not PlatformAdminRequired) because during
	// impersonation, isPlatformAdmin is false. The handler validates impersonatedBy claim.
	api.Post("/auth/stop-impersonate", authMiddleware.Required(), authHandler.StopImpersonate)

	// ==========================================
	// Public auth routes (no authentication required)
	// ==========================================
	// SECURITY: Apply strict rate limiting to ALL auth endpoints to prevent brute force attacks
	// 5 attempts per minute per IP for: register, login, forgot-password, reset-password, etc.
	authRateLimiter := middleware.NewAuthRateLimiter(middleware.AuthRateLimiterConfig{
		Max:    5,
		Window: 1 * time.Minute,
	})
	auth := api.Group("/auth", authRateLimiter)
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.RefreshToken)
	auth.Post("/accept-invite", authHandler.AcceptInvitation)
	auth.Post("/forgot-password", authHandler.ForgotPassword)
	auth.Post("/reset-password", authHandler.ResetPassword)

	// ==========================================
	// Protected auth routes (authentication required)
	// ==========================================
	// Session timeout middleware applied after auth to enforce idle and absolute timeouts
	// 403 audit middleware captures authorization failures for security monitoring
	authProtected := auth.Group("", authMiddleware.Required(), sessionTimeoutMiddleware, middleware.AuditAuthorizationFailures(auditLogger))
	authProtected.Post("/logout", authHandler.Logout)
	authProtected.Post("/logout-all", authHandler.LogoutAll)
	authProtected.Get("/me", authHandler.Me)
	authProtected.Get("/orgs", authHandler.GetUserOrgs)
	authProtected.Post("/switch-org", authHandler.SwitchOrg)
	authProtected.Post("/change-password", authHandler.ChangePassword)
	authProtected.Post("/extend-session", authHandler.ExtendSession)

	// Org admin routes (requires admin/owner role)
	authAdmin := auth.Group("", authMiddleware.OrgAdminRequired(), sessionTimeoutMiddleware, middleware.AuditAuthorizationFailures(auditLogger))
	authAdmin.Post("/invite", authHandler.InviteUser)
	authAdmin.Get("/invitations", authHandler.ListInvitations)
	authAdmin.Delete("/invitations/:id", authHandler.DeleteInvitation)

	// Platform admin routes (requires platform admin)
	authPlatformAdmin := auth.Group("", authMiddleware.PlatformAdminRequired(), sessionTimeoutMiddleware, middleware.AuditAuthorizationFailures(auditLogger))
	authPlatformAdmin.Post("/impersonate", authHandler.Impersonate)

	// ==========================================
	// Protected API routes (authentication required - all users)
	// ==========================================
	// Chain: auth (JWT validation) -> session timeout (idle/absolute) -> 403 audit -> password change -> body limit -> tenant (resolve DB)
	// SECURITY: Apply 1MB body limit to most routes (HARD-06)
	protected := api.Group("", authMiddleware.Required(), sessionTimeoutMiddleware, middleware.AuditAuthorizationFailures(auditLogger), middleware.RequirePasswordChange(), middleware.BodyLimit(middleware.DefaultBodyLimit), tenantMiddleware.ResolveTenant())

	// CRM entity routes (accessible to all authenticated users)
	contactHandler.RegisterRoutes(protected)
	accountHandler.RegisterRoutes(protected)
	taskHandler.RegisterRoutes(protected)
	quoteHandler.RegisterRoutes(protected)
	lookupHandler.RegisterRoutes(protected)
	relatedHandler.RegisterRelatedRoutes(protected)
	genericEntityHandler.RegisterRoutes(protected)
	bulkHandler.RegisterRoutes(protected)
	mergeHandler.RegisterRoutes(protected)

	// Import routes need larger body limit for file uploads (10MB)
	// Create separate group without the 1MB body limit restriction
	importProtected := api.Group("", authMiddleware.Required(), sessionTimeoutMiddleware, middleware.AuditAuthorizationFailures(auditLogger), middleware.RequirePasswordChange(), tenantMiddleware.ResolveTenant())
	importHandler.RegisterRoutes(importProtected)

	// User management - read-only for all authenticated users
	userHandler.RegisterRoutes(protected)

	// Navigation - visible tabs for all authenticated users
	navigationHandler.RegisterPublicRoutes(protected)

	// Org settings - read-only for all authenticated users
	orgSettingsHandler.RegisterPublicRoutes(protected)

	// Read-only metadata - accessible to all authenticated users for rendering layouts
	metadataHandler.RegisterRoutes(protected)

	// Schema API - for Gmail extension to discover entities and search records
	schemaHandler.RegisterRoutes(protected)

	// Version info - accessible to all authenticated users
	versionHandler.RegisterRoutes(protected)

	// Read-only related list and bearing routes for detail pages
	relatedListHandler.RegisterPublicRoutes(protected)
	bearingHandler.RegisterPublicRoutes(protected)

	// Custom pages - public routes for viewing pages
	customPageHandler.RegisterPublicRoutes(protected)

	// List views - users can view, create, update, and delete their own list views
	listViews := protected.Group("/list-views")
	listViews.Get("/:entity", listViewHandler.List)
	listViews.Get("/:entity/default", listViewHandler.GetDefault)
	listViews.Get("/:entity/:id", listViewHandler.Get)
	listViews.Post("/:entity", listViewHandler.Create)
	listViews.Put("/:entity/:id", listViewHandler.Update)
	listViews.Delete("/:entity/:id", listViewHandler.Delete)

	// Screen flows - all users can view and execute flows
	flowHandler.RegisterRoutes(protected)

	// Static downloads - extension zip files for authenticated users
	protected.Get("/downloads/extension", func(c *fiber.Ctx) error {
		// Serve the Chrome extension zip file
		c.Set("Content-Disposition", "attachment; filename=quantico-capture-extension.zip")
		c.Set("Content-Type", "application/zip")
		return c.SendFile("./static/quantico-capture-extension.zip")
	})

	// ==========================================
	// Admin routes (requires admin or owner role)
	// ==========================================
	adminProtected := api.Group("", authMiddleware.OrgAdminRequired(), sessionTimeoutMiddleware, middleware.AuditAuthorizationFailures(auditLogger), middleware.RequirePasswordChange(), tenantMiddleware.ResolveTenant())

	// API Token management (admin only - creating/revoking tokens)
	apiTokens := adminProtected.Group("/api-tokens")
	apiTokens.Post("", apiTokenHandler.Create)
	apiTokens.Get("", apiTokenHandler.List)
	apiTokens.Post("/:id/revoke", apiTokenHandler.Revoke)
	apiTokens.Delete("/:id", apiTokenHandler.Delete)

	// Ingest API Key management (admin only)
	ingestKeys := adminProtected.Group("/ingest-keys")
	ingestKeys.Post("", ingestKeyHandler.Create)
	ingestKeys.Get("", ingestKeyHandler.List)
	ingestKeys.Post("/:id/deactivate", ingestKeyHandler.Deactivate)
	ingestKeys.Delete("/:id", ingestKeyHandler.Delete)

	// Admin-only functionality
	adminHandler.RegisterRoutes(adminProtected)
	navigationHandler.RegisterAdminRoutes(adminProtected)
	relatedListHandler.RegisterRoutes(adminProtected)
	tripwireHandler.RegisterRoutes(adminProtected)
	bearingHandler.RegisterRoutes(adminProtected)
	validationHandler.RegisterRoutes(adminProtected)
	versionHandler.RegisterAdminRoutes(adminProtected)
	// Data Explorer for org admins (filtered to their org's data only)
	dataExplorerHandler.RegisterOrgRoutes(adminProtected)

	// User management - write operations for admins/owners only
	userHandler.RegisterAdminRoutes(adminProtected)

	// Custom pages - admin management
	customPageHandler.RegisterAdminRoutes(adminProtected)

	// Screen flows - admin can create/edit/delete flow definitions
	flowHandler.RegisterAdminRoutes(adminProtected)

	// PDF Template management (admin only)
	pdfTemplateHandler.RegisterRoutes(adminProtected)

	// Org settings - admin can update homepage and other settings
	orgSettingsHandler.RegisterAdminRoutes(adminProtected)

	// Audit logs - admin can view, export, and verify
	adminProtected.Get("/audit-logs", auditHandler.List)
	adminProtected.Get("/audit-logs/export", auditHandler.Export)
	adminProtected.Get("/audit-logs/verify", auditHandler.VerifyChain)
	adminProtected.Get("/audit-logs/event-types", auditHandler.GetEventTypes)

	// Deduplication - admin can manage rules and check for duplicates
	dedupHandler.RegisterRoutes(adminProtected)

	// Background scanning - admin can manage schedules and jobs
	scanJobHandler.RegisterAdminRoutes(adminProtected)

	// Salesforce integration - admin can configure OAuth and manage sync
	salesforceHandler.RegisterRoutes(adminProtected)

	// Mirror management (admin only - create/configure schema contracts)
	mirrorHandler.RegisterRoutes(adminProtected)

	// PDF Template public routes (read-only for all authenticated users)
	pdfTemplateHandler.RegisterPublicRoutes(protected)

	// Notifications - all authenticated users can view their notifications
	scanJobHandler.RegisterPublicRoutes(protected)

	// ==========================================
	// Platform Admin routes (SUPER ADMIN ONLY)
	// NOTE: These routes do NOT use ResolveTenant middleware - they work with master DB only
	// ==========================================
	platformAdmin := api.Group("/platform", authMiddleware.PlatformAdminRequired(), sessionTimeoutMiddleware, middleware.AuditAuthorizationFailures(auditLogger))

	// Debug endpoint to verify platform routes and tenant provisioning config
	platformAdmin.Get("/debug", func(c *fiber.Ctx) error {
		// Check env vars (redacted for security)
		hasAPIToken := os.Getenv("TURSO_API_TOKEN") != "" || os.Getenv("TURSO_AUTH_TOKEN") != ""
		hasOrg := os.Getenv("TURSO_ORG") != ""
		tursoOrg := os.Getenv("TURSO_ORG")
		if len(tursoOrg) > 3 {
			tursoOrg = tursoOrg[:3] + "***"
		}

		return c.JSON(fiber.Map{
			"status":              "ok",
			"route":               "platform-debug",
			"version":             "v13",
			"tenantProvisioning":  map[string]interface{}{
				"isLocalMode":    tenantProvisioningService.IsLocalMode(),
				"hasAPIToken":    hasAPIToken,
				"hasOrg":         hasOrg,
				"orgPrefix":      tursoOrg,
			},
		})
	})

	platformAdmin.Post("/orgs", func(c *fiber.Ctx) error {
		var input entity.OrganizationCreateInput
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if input.Name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Organization name is required",
			})
		}

		org, err := authService.CreateOrganization(c.Context(), input)
		if err != nil {
			return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ErrCategoryDatabase)
		}

		return c.Status(fiber.StatusCreated).JSON(org)
	})

	platformAdmin.Get("/orgs", func(c *fiber.Ctx) error {
		log.Printf("[v6] Platform orgs handler called")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("pageSize", 20)
		orgs, err := authRepo.ListOrganizations(c.Context(), page, pageSize)
		if err != nil {
			log.Printf("[v6] Error listing organizations: %v", err)
			return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ErrCategoryDatabase)
		}
		return c.JSON(orgs)
	})

	platformAdmin.Patch("/orgs/:id", func(c *fiber.Ctx) error {
		orgID := c.Params("id")
		if orgID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Organization ID is required",
			})
		}

		var input entity.OrganizationUpdateInput
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		org, err := authRepo.UpdateOrganization(c.Context(), orgID, input)
		if err != nil {
			return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ErrCategoryDatabase)
		}

		return c.JSON(org)
	})

	platformAdmin.Delete("/orgs/:id", func(c *fiber.Ctx) error {
		orgID := c.Params("id")
		if orgID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Organization ID is required",
			})
		}

		// Delete organization and all related data
		if err := authRepo.DeleteOrganization(c.Context(), orgID); err != nil {
			return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ErrCategoryDatabase)
		}

		return c.Status(fiber.StatusNoContent).Send(nil)
	})

	// SECURITY: Data Explorer is only available to Platform Admin (super admin)
	// This allows viewing ALL data across ALL organizations - only for debugging/support
	dataExplorerHandler.RegisterRoutes(platformAdmin)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting FastCRM API on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// ensureSubmittalsClientID ensures the submittals table has client_id column
// and the clientId field definition exists for related list discovery
func ensureSubmittalsClientID(db *sql.DB) {
	// First check if submittals table exists at all
	var tableExists int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='submittals'").Scan(&tableExists); err == nil && tableExists > 0 {
		// Table exists - check if client_id column exists
		var colCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('submittals') WHERE name = 'client_id'").Scan(&colCount); err != nil {
			log.Printf("Note: submittals column check failed: %v", err)
		}

		if colCount == 0 {
			// Add the client_id column
			if _, err := db.Exec("ALTER TABLE submittals ADD COLUMN client_id TEXT"); err != nil {
				log.Printf("Warning: failed to add client_id to submittals: %v", err)
			} else {
				log.Println("Added client_id column to submittals table")

				// Create index
				_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_submittals_client ON submittals(client_id)")

				// Backfill from job_openings
				result, err := db.Exec(`
					UPDATE submittals
					SET client_id = (
						SELECT j.client_id
						FROM job_openings j
						WHERE j.id = submittals.job_opening_id
					)
					WHERE client_id IS NULL AND job_opening_id IS NOT NULL
				`)
				if err != nil {
					log.Printf("Warning: failed to backfill client_id: %v", err)
				} else {
					rows, _ := result.RowsAffected()
					log.Printf("Backfilled client_id for %d submittals", rows)
				}
			}
		}
	}
	// If table doesn't exist, skip silently - it will be created when an org provisions the Submittal entity

	// ALWAYS ensure clientId field definition exists for all orgs that have Submittal entity
	// This is critical for related list discovery to work - runs even if column already exists
	result, err := db.Exec(`
		INSERT OR REPLACE INTO field_defs (id, org_id, entity_name, name, label, type, is_required, sort_order, link_entity, created_at, modified_at)
		SELECT
			'fld_sub_clientid_' || org_id,
			org_id,
			'Submittal',
			'clientId',
			'Client',
			'link',
			0,
			3,
			'Client',
			datetime('now'),
			datetime('now')
		FROM entity_defs
		WHERE name = 'Submittal'
	`)
	if err != nil {
		log.Printf("Warning: failed to add clientId field definition: %v", err)
	} else {
		rows, _ := result.RowsAffected()
		log.Printf("Ensured clientId field definition for %d Submittal entities", rows)
	}

	// Fix related_list_configs to use clientId instead of clientName/client_name
	// This is needed because the Placements related list may have been created with the old field name
	// The value might be stored as 'clientName' (camelCase) or 'client_name' (snake_case)
	result, err = db.Exec(`
		UPDATE related_list_configs
		SET lookup_field = 'clientId', modified_at = datetime('now')
		WHERE related_entity = 'Submittal' AND (lookup_field = 'clientName' OR lookup_field = 'client_name')
	`)
	if err != nil {
		log.Printf("Warning: failed to update related_list_configs: %v", err)
	} else {
		rows, _ := result.RowsAffected()
		if rows > 0 {
			log.Printf("Updated %d related_list_configs from clientName to clientId", rows)
		}
	}

	// Also delete the old clientName field definition if it exists (to clean up)
	_, _ = db.Exec(`
		DELETE FROM field_defs
		WHERE entity_name = 'Submittal' AND name = 'clientName' AND type = 'varchar'
	`)
}

// ensureSessionsColumns ensures the sessions table has all required columns
// for token family support (migration 046) and session timeouts (migration 047)
func ensureSessionsColumns(db *sql.DB) {
	// Check if sessions table exists
	var tableExists int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&tableExists); err != nil || tableExists == 0 {
		return // Table doesn't exist yet, will be created by migrations
	}

	// Columns to ensure exist with their default values
	columns := []struct {
		name         string
		definition   string
		defaultValue string
	}{
		{"family_id", "TEXT", ""},
		{"is_revoked", "INTEGER DEFAULT 0", "0"},
		{"last_activity_at", "TEXT", "CURRENT_TIMESTAMP"},
		{"idle_timeout_minutes", "INTEGER DEFAULT 30", "30"},
		{"absolute_timeout_minutes", "INTEGER DEFAULT 1440", "1440"},
	}

	for _, col := range columns {
		var colCount int
		query := "SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name = ?"
		if err := db.QueryRow(query, col.name).Scan(&colCount); err != nil {
			log.Printf("Note: sessions column check failed for %s: %v", col.name, err)
			continue
		}

		if colCount == 0 {
			// Add the column
			alterSQL := "ALTER TABLE sessions ADD COLUMN " + col.name + " " + col.definition
			if _, err := db.Exec(alterSQL); err != nil {
				log.Printf("Warning: failed to add %s to sessions: %v", col.name, err)
			} else {
				log.Printf("Added %s column to sessions table", col.name)

				// Backfill existing rows if needed
				if col.defaultValue != "" {
					updateSQL := "UPDATE sessions SET " + col.name + " = " + col.defaultValue + " WHERE " + col.name + " IS NULL"
					if _, err := db.Exec(updateSQL); err != nil {
						log.Printf("Warning: failed to backfill %s: %v", col.name, err)
					}
				}
			}
		}
	}

	// Backfill family_id with session's own ID if NULL
	if _, err := db.Exec("UPDATE sessions SET family_id = id WHERE family_id IS NULL"); err != nil {
		log.Printf("Warning: failed to backfill family_id: %v", err)
	}

	// Create indexes if they don't exist
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_sessions_family ON sessions(family_id)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_family_revoked ON sessions(family_id, is_revoked)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_last_activity ON sessions(last_activity_at)",
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			log.Printf("Warning: failed to create index: %v", err)
		}
	}
}
