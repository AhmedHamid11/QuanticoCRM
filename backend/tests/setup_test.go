package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fastcrm/backend/internal/handler"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"

	_ "github.com/mattn/go-sqlite3"
)

// testDBConn wraps *sql.DB to implement db.DBConn without io.Closer.
// Prevents fasthttp from closing the shared test DB between requests.
type testDBConn struct {
	db *sql.DB
}

func (w *testDBConn) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return w.db.QueryContext(ctx, query, args...)
}
func (w *testDBConn) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return w.db.QueryRowContext(ctx, query, args...)
}
func (w *testDBConn) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return w.db.ExecContext(ctx, query, args...)
}
func (w *testDBConn) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return w.db.PrepareContext(ctx, query)
}
func (w *testDBConn) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return w.db.BeginTx(ctx, opts)
}

// TestApp holds all components needed for testing
type TestApp struct {
	App                 *fiber.App
	DB                  *sql.DB
	AuthService         *service.AuthService
	ContactRepo         *repo.ContactRepo
	AccountRepo         *repo.AccountRepo
	TaskRepo            *repo.TaskRepo
	MetadataRepo        *repo.MetadataRepo
	TripwireRepo        *repo.TripwireRepo
	ValidationRepo      *repo.ValidationRepo
	TripwireService     *service.TripwireService
	ValidationService   *service.ValidationService
	IngestAPIKeyService *service.IngestAPIKeyService
	IngestAPIKeyRepo    *repo.IngestAPIKeyRepo
	MirrorRepo          *repo.MirrorRepo
	IngestJobRepo       *repo.IngestJobRepo
	DeltaKeyRepo        *repo.DeltaKeyRepo
	IngestService       *service.IngestService
	RateLimiter         *service.IngestRateLimiter
	AuthRepo            *repo.AuthRepo
}

// TestUser represents a test user with tokens
type TestUser struct {
	UserID       string
	Email        string
	OrgID        string
	OrgName      string
	Role         string
	AccessToken  string
	RefreshToken string
}

// SetupTestApp creates a new test application with an in-memory database
func SetupTestApp(t *testing.T) *TestApp {
	t.Helper()

	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	contactRepo := repo.NewContactRepo(db)
	accountRepo := repo.NewAccountRepo(db)
	taskRepo := repo.NewTaskRepo(db)
	metadataRepo := repo.NewMetadataRepo(db)
	navigationRepo := repo.NewNavigationRepo(db)
	relatedListRepo := repo.NewRelatedListRepo(db, metadataRepo)
	tripwireRepo := repo.NewTripwireRepo(db)
	bearingRepo := repo.NewBearingRepo(db, metadataRepo)
	validationRepo := repo.NewValidationRepo(db)
	authRepo := repo.NewAuthRepo(db)
	auditRepo := repo.NewAuditRepo(db)

	// Ingest infrastructure repositories
	ingestAPIKeyRepo := repo.NewIngestAPIKeyRepo(db)
	mirrorRepo := repo.NewMirrorRepo(db)
	ingestJobRepo := repo.NewIngestJobRepo(db)
	deltaKeyRepo := repo.NewDeltaKeyRepo(db)

	// Initialize services
	tripwireService := service.NewTripwireService(db, tripwireRepo)
	validationService := service.NewValidationService(db, validationRepo)
	provisioningService := service.NewProvisioningService(db)
	provisioningService.SkipSampleData = true // Skip sample data for cleaner test assertions
	authConfig := service.DefaultAuthConfig("test-jwt-secret-for-testing")
	authService := service.NewAuthService(authRepo, authConfig, provisioningService)
	apiTokenService := service.NewAPITokenService(repo.NewAPITokenRepo(db))
	auditLogger := service.NewAuditLogger(auditRepo)

	// Ingest infrastructure services
	ingestAPIKeyService := service.NewIngestAPIKeyService(ingestAPIKeyRepo)
	ingestService := service.NewIngestService(mirrorRepo, ingestJobRepo, deltaKeyRepo, metadataRepo, db)
	ingestRateLimiter := service.NewIngestRateLimiter()

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService, apiTokenService)

	// Wrap DB to prevent fasthttp from calling Close() between requests.
	// Production middleware uses unexported dbConnWrapper for the same reason.
	wrappedDB := &testDBConn{db: db}

	// Test tenant middleware - sets DB for all authenticated requests
	testTenantMiddleware := func(c *fiber.Ctx) error {
		// Only apply if orgID is set (by auth middleware) and DB not yet set
		if orgID := c.Locals("orgID"); orgID != nil && c.Locals("dbConn") == nil {
			c.Locals("db", wrappedDB)
			c.Locals("dbConn", wrappedDB)
		}
		return c.Next()
	}

	// Initialize handlers
	contactHandler := handler.NewContactHandler(contactRepo, taskRepo, nil, tripwireService, validationService, nil, nil)
	accountHandler := handler.NewAccountHandler(accountRepo, taskRepo, db, metadataRepo, nil, tripwireService, validationService)
	taskHandler := handler.NewTaskHandler(taskRepo, nil, tripwireService, validationService)
	adminHandler := handler.NewAdminHandler(db, metadataRepo, navigationRepo)
	navigationHandler := handler.NewNavigationHandler(navigationRepo)
	lookupHandler := handler.NewLookupHandler(db, metadataRepo)
	relatedHandler := handler.NewRelatedHandler(db)
	relatedListHandler := handler.NewRelatedListHandler(relatedListRepo, metadataRepo, db)
	genericEntityHandler := handler.NewGenericEntityHandler(db, metadataRepo, authRepo, tripwireService, validationService, nil)
	tripwireHandler := handler.NewTripwireHandler(tripwireRepo)
	bearingHandler := handler.NewBearingHandler(bearingRepo)
	validationHandler := handler.NewValidationHandler(validationRepo, validationService)
	authHandler := handler.NewAuthHandler(authService, auditLogger, middleware.CookieConfig{IsProduction: false})
	userHandler := handler.NewUserHandler(authRepo, auditLogger)
	bulkHandler := handler.NewBulkHandler(db, metadataRepo, tripwireService, validationService)
	importHandler := handler.NewImportHandler(db, metadataRepo, tripwireService, validationService, nil)

	// Ingest handlers
	ingestHandler := handler.NewIngestHandler(ingestService, mirrorRepo, ingestJobRepo, deltaKeyRepo, ingestRateLimiter)
	ingestKeyHandler := handler.NewIngestAPIKeyHandler(ingestAPIKeyService)
	mirrorHandler := handler.NewMirrorHandler(mirrorRepo, ingestJobRepo, provisioningService)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// API routes
	api := app.Group("/api/v1")

	// Health check (public)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Ingest admin routes (JWT auth) — registered FIRST to avoid /ingest group prefix overlap
	orgAdminMW := authMiddleware.OrgAdminRequired()
	ingestKeys := api.Group("/ingest-keys", orgAdminMW, testTenantMiddleware)
	ingestKeys.Post("", ingestKeyHandler.Create)
	ingestKeys.Get("", ingestKeyHandler.List)
	ingestKeys.Post("/:id/deactivate", ingestKeyHandler.Deactivate)
	ingestKeys.Delete("/:id", ingestKeyHandler.Delete)

	// Mirror admin routes (JWT auth) — RegisterRoutes creates its own /admin/mirrors sub-group
	mirrorAdmin := api.Group("/admin/mirrors", orgAdminMW, testTenantMiddleware)
	mirrorAdmin.Post("/", mirrorHandler.Create)
	mirrorAdmin.Get("/", mirrorHandler.List)
	mirrorAdmin.Get("/:id", mirrorHandler.Get)
	mirrorAdmin.Put("/:id", mirrorHandler.Update)
	mirrorAdmin.Delete("/:id", mirrorHandler.Delete)
	mirrorAdmin.Get("/:id/jobs", mirrorHandler.ListJobs)

	// Ingest routes (X-API-Key auth, NOT JWT) — registered BEFORE protected Group("") groups
	testIngestAuth := func(c *fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "X-API-Key header required",
			})
		}

		ingestKey, err := ingestAPIKeyService.ValidateKey(c.Context(), apiKey)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or inactive API key",
			})
		}

		org, err := authRepo.GetOrganizationByID(c.Context(), ingestKey.OrgID)
		if err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Organization not found or inactive",
			})
		}

		if !org.IsActive {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Organization not found or inactive",
			})
		}

		c.Locals("ingestOrgID", ingestKey.OrgID)
		c.Locals("ingestKeyID", ingestKey.ID)
		c.Locals("ingestRateLimit", ingestKey.RateLimit)
		c.Locals("db", wrappedDB)
		c.Locals("dbConn", wrappedDB)

		return c.Next()
	}

	ingestGroup := api.Group("/ingest", testIngestAuth)
	ingestHandler.RegisterRoutes(ingestGroup)

	// Public auth routes
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.RefreshToken)
	auth.Post("/accept-invite", authHandler.AcceptInvitation)

	// Protected auth routes
	authProtected := auth.Group("", authMiddleware.Required())
	authProtected.Post("/logout", authHandler.Logout)
	authProtected.Post("/logout-all", authHandler.LogoutAll)
	authProtected.Get("/me", authHandler.Me)
	authProtected.Get("/orgs", authHandler.GetUserOrgs)
	authProtected.Post("/switch-org", authHandler.SwitchOrg)
	authProtected.Post("/change-password", authHandler.ChangePassword)

	// Org admin routes
	authAdmin := auth.Group("", authMiddleware.OrgAdminRequired())
	authAdmin.Post("/invite", authHandler.InviteUser)
	authAdmin.Get("/invitations", authHandler.ListInvitations)
	authAdmin.Delete("/invitations/:id", authHandler.DeleteInvitation)

	// Platform admin routes (for impersonation tests)
	authPlatformAdmin := auth.Group("", authMiddleware.PlatformAdminRequired())
	authPlatformAdmin.Post("/impersonate", authHandler.Impersonate)
	authPlatformAdmin.Post("/stop-impersonate", authHandler.StopImpersonate)

	// Protected API routes
	protected := api.Group("", authMiddleware.Required(), testTenantMiddleware)

	// CRM entity routes
	contactHandler.RegisterRoutes(protected)
	accountHandler.RegisterRoutes(protected)
	taskHandler.RegisterRoutes(protected)
	lookupHandler.RegisterRoutes(protected)
	relatedHandler.RegisterRelatedRoutes(protected)
	genericEntityHandler.RegisterRoutes(protected)
	bulkHandler.RegisterRoutes(protected)
	importHandler.RegisterRoutes(protected)

	// User management
	userHandler.RegisterRoutes(protected)

	// Navigation
	navigationHandler.RegisterPublicRoutes(protected)

	// Related list public routes (GET configs for detail pages)
	relatedListHandler.RegisterPublicRoutes(protected)

	// Bearing public routes (GET active bearings for detail pages)
	bearingHandler.RegisterPublicRoutes(protected)

	// Admin routes
	adminProtected := api.Group("", authMiddleware.OrgAdminRequired(), testTenantMiddleware)
	adminHandler.RegisterRoutes(adminProtected)
	navigationHandler.RegisterAdminRoutes(adminProtected)
	relatedListHandler.RegisterRoutes(adminProtected)
	tripwireHandler.RegisterRoutes(adminProtected)
	bearingHandler.RegisterRoutes(adminProtected)
	validationHandler.RegisterRoutes(adminProtected)
	userHandler.RegisterAdminRoutes(adminProtected)

	// Platform admin routes (for testing isolation)
	platform := api.Group("/platform", authMiddleware.PlatformAdminRequired())
	platform.Get("/organizations", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"organizations": []string{"org1", "org2"}})
	})

	// Note: ingest-keys and mirror routes registered earlier (before /ingest group) to avoid prefix overlap

	return &TestApp{
		App:                 app,
		DB:                  db,
		AuthService:         authService,
		ContactRepo:         contactRepo,
		AccountRepo:         accountRepo,
		TaskRepo:            taskRepo,
		MetadataRepo:        metadataRepo,
		TripwireRepo:        tripwireRepo,
		ValidationRepo:      validationRepo,
		TripwireService:     tripwireService,
		ValidationService:   validationService,
		IngestAPIKeyService: ingestAPIKeyService,
		IngestAPIKeyRepo:    ingestAPIKeyRepo,
		MirrorRepo:          mirrorRepo,
		IngestJobRepo:       ingestJobRepo,
		DeltaKeyRepo:        deltaKeyRepo,
		IngestService:       ingestService,
		RateLimiter:         ingestRateLimiter,
		AuthRepo:            authRepo,
	}
}

// Cleanup closes the database connection
func (ta *TestApp) Cleanup() {
	if ta.DB != nil {
		ta.DB.Close()
	}
}

// CreateTestUser registers a new user and returns their credentials
func (ta *TestApp) CreateTestUser(t *testing.T, email, password, orgName string) *TestUser {
	t.Helper()

	body := map[string]string{
		"email":    email,
		"password": password,
		"orgName":  orgName,
	}

	resp := ta.MakeRequest(t, "POST", "/api/v1/auth/register", body, "")
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create test user: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	// Extract refresh token from cookie (secure HttpOnly cookie)
	refreshToken := GetRefreshTokenFromCookies(resp)

	// Read the raw response for debugging
	bodyBytes, _ := io.ReadAll(resp.Body)

	var result struct {
		User struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			Memberships []struct {
				OrgID   string `json:"orgId"`
				OrgName string `json:"orgName"`
				Role    string `json:"role"`
			} `json:"memberships"`
		} `json:"user"`
		AccessToken string `json:"accessToken"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		t.Fatalf("Failed to decode response: %v (body: %s)", err, string(bodyBytes))
	}

	// Extract org info from first membership (newly registered user has one membership)
	var resultOrgID, resultOrgName, resultRole string
	if len(result.User.Memberships) > 0 {
		resultOrgID = result.User.Memberships[0].OrgID
		resultOrgName = result.User.Memberships[0].OrgName
		resultRole = result.User.Memberships[0].Role
	}

	return &TestUser{
		UserID:       result.User.ID,
		Email:        result.User.Email,
		OrgID:        resultOrgID,
		OrgName:      resultOrgName,
		Role:         resultRole,
		AccessToken:  result.AccessToken,
		RefreshToken: refreshToken,
	}
}

// LoginUser logs in an existing user
func (ta *TestApp) LoginUser(t *testing.T, email, password string) *TestUser {
	t.Helper()

	body := map[string]string{
		"email":    email,
		"password": password,
	}

	resp := ta.MakeRequest(t, "POST", "/api/v1/auth/login", body, "")
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to login: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	// Extract refresh token from cookie (secure HttpOnly cookie)
	refreshToken := GetRefreshTokenFromCookies(resp)

	// Read body for JSON response
	bodyBytes, _ := io.ReadAll(resp.Body)

	var result struct {
		User struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			Memberships []struct {
				OrgID   string `json:"orgId"`
				OrgName string `json:"orgName"`
				Role    string `json:"role"`
			} `json:"memberships"`
		} `json:"user"`
		AccessToken string `json:"accessToken"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		t.Fatalf("Failed to decode login response: %v (body: %s)", err, string(bodyBytes))
	}

	// Extract org info from first membership
	var orgID, orgName, role string
	if len(result.User.Memberships) > 0 {
		orgID = result.User.Memberships[0].OrgID
		orgName = result.User.Memberships[0].OrgName
		role = result.User.Memberships[0].Role
	}

	return &TestUser{
		UserID:       result.User.ID,
		Email:        result.User.Email,
		OrgID:        orgID,
		OrgName:      orgName,
		Role:         role,
		AccessToken:  result.AccessToken,
		RefreshToken: refreshToken,
	}
}

// MakeRequest makes an HTTP request to the test app
func (ta *TestApp) MakeRequest(t *testing.T, method, path string, body interface{}, token string) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBytes)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := ta.App.Test(req, -1) // -1 means no timeout
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	return resp
}

// MakeRequestWithCookies makes an HTTP request and includes cookies in the request
func (ta *TestApp) MakeRequestWithCookies(t *testing.T, method, path string, body interface{}, token string, cookies []*http.Cookie) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBytes)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := ta.App.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	return resp
}

// GetRefreshTokenFromCookies extracts the refresh token from response cookies
func GetRefreshTokenFromCookies(resp *http.Response) string {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "refresh_token" {
			return cookie.Value
		}
	}
	return ""
}

// MakeRequestWithResponse makes a request and decodes the response into the provided struct
func (ta *TestApp) MakeRequestWithResponse(t *testing.T, method, path string, body interface{}, token string, response interface{}) *http.Response {
	t.Helper()

	resp := ta.MakeRequest(t, method, path, body, token)

	if response != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, response); err != nil {
				t.Fatalf("Failed to decode response: %v (body: %s)", err, string(bodyBytes))
			}
		}
	}

	return resp
}

// AssertStatus checks that the response has the expected status code
func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(bodyBytes))
	}
}

// AssertJSON checks that the response contains the expected JSON fields
func AssertJSON(t *testing.T, resp *http.Response, expected map[string]interface{}) {
	t.Helper()

	var actual map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &actual); err != nil {
		t.Fatalf("Failed to parse response JSON: %v (body: %s)", err, string(bodyBytes))
	}

	for key, expectedVal := range expected {
		actualVal, ok := actual[key]
		if !ok {
			t.Errorf("Expected key %q not found in response", key)
			continue
		}
		if fmt.Sprintf("%v", actualVal) != fmt.Sprintf("%v", expectedVal) {
			t.Errorf("Key %q: expected %v, got %v", key, expectedVal, actualVal)
		}
	}
}

// runMigrations runs all database migrations
func runMigrations(db *sql.DB) error {
	// Get the migrations directory
	migrationsDir := getMigrationsDir()

	// Read all migration files
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file.Name(), err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			errStr := err.Error()
			// Skip errors that are expected in test environments:
			// - "duplicate column": column was already added in an earlier migration
			// - "no such table": migration references tables that only exist in production
			// - "already exists": table/index/constraint already created
			if strings.Contains(errStr, "duplicate column") ||
				strings.Contains(errStr, "no such table") ||
				strings.Contains(errStr, "already exists") {
				continue
			}
			return fmt.Errorf("failed to execute migration %s: %w", file.Name(), err)
		}
	}

	return nil
}

// getMigrationsDir returns the path to the migrations directory
func getMigrationsDir() string {
	// Try relative paths from the test directory
	paths := []string{
		"../../migrations",
		"../../../migrations",
		"../../../../migrations",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fallback to absolute path
	return "/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/migrations"
}

// StringPtr returns a pointer to a string
func StringPtr(s string) *string {
	return &s
}

// BoolPtr returns a pointer to a bool
func BoolPtr(b bool) *bool {
	return &b
}

// IntPtr returns a pointer to an int
func IntPtr(i int) *int {
	return &i
}

// CreatePlatformAdmin creates a platform admin user for testing
func (ta *TestApp) CreatePlatformAdmin(t *testing.T, email, password string) *TestUser {
	t.Helper()

	// First create a regular user with their own org
	user := ta.CreateTestUser(t, email, password, "Platform Admin Org")

	// Mark user as platform admin in database
	_, err := ta.DB.Exec(`UPDATE users SET is_platform_admin = 1 WHERE id = ?`, user.UserID)
	if err != nil {
		t.Fatalf("Failed to make user platform admin: %v", err)
	}

	// Re-login to get updated token with platform admin claim
	return ta.LoginUser(t, email, password)
}

// Impersonate makes a platform admin impersonate a specific org
// Returns the impersonation access token
func (ta *TestApp) Impersonate(t *testing.T, adminToken, targetOrgID string) string {
	t.Helper()

	// Create request body with proper JSON marshaling
	reqBody := fmt.Sprintf(`{"orgId":"%s"}`, targetOrgID)
	req := httptest.NewRequest("POST", "/api/v1/auth/impersonate", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := ta.App.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to make impersonate request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to impersonate (orgID=%s): %d - %s", targetOrgID, resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		AccessToken string `json:"accessToken"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode impersonate response: %v", err)
	}

	return result.AccessToken
}

// CreateContact creates a test contact for isolation testing
func (ta *TestApp) CreateContact(t *testing.T, token string, data map[string]string) map[string]interface{} {
	t.Helper()

	resp := ta.MakeRequest(t, "POST", "/api/v1/contacts/", data, token)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create contact: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode contact response: %v", err)
	}
	return result
}

// CreateAccount creates a test account for isolation testing
func (ta *TestApp) CreateAccount(t *testing.T, token string, data map[string]string) map[string]interface{} {
	t.Helper()

	resp := ta.MakeRequest(t, "POST", "/api/v1/accounts/", data, token)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create account: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode account response: %v", err)
	}
	return result
}

// CreateTask creates a test task for isolation testing
func (ta *TestApp) CreateTask(t *testing.T, token string, data map[string]string) map[string]interface{} {
	t.Helper()

	resp := ta.MakeRequest(t, "POST", "/api/v1/tasks/", data, token)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create task: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode task response: %v", err)
	}
	return result
}

// MakeIngestRequest makes an HTTP request with X-API-Key header
func (ta *TestApp) MakeIngestRequest(t *testing.T, method, path string, body interface{}, apiKey string) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBytes)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := ta.App.Test(req, -1) // -1 means no timeout
	if err != nil {
		t.Fatalf("Failed to make ingest request: %v", err)
	}

	return resp
}

// WaitForJobCompletion polls for job completion with timeout
func (ta *TestApp) WaitForJobCompletion(t *testing.T, apiKey, jobID string, timeoutSecs int) map[string]interface{} {
	t.Helper()

	path := fmt.Sprintf("/api/v1/ingest/jobs/%s", jobID)
	pollInterval := 100 * time.Millisecond
	timeout := time.Duration(timeoutSecs) * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp := ta.MakeIngestRequest(t, "GET", path, nil, apiKey)

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to get job status: %d - %s", resp.StatusCode, string(bodyBytes))
		}

		var job map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &job); err != nil {
			t.Fatalf("Failed to decode job response: %v", err)
		}

		status, ok := job["status"].(string)
		if !ok {
			t.Fatalf("Job status not found in response")
		}

		// Check for terminal states
		if status == "complete" || status == "partial" || status == "failed" {
			return job
		}

		// Sleep before next poll
		time.Sleep(pollInterval)
	}

	t.Fatalf("Job %s did not complete within %d seconds", jobID, timeoutSecs)
	return nil
}
