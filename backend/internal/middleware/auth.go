package middleware

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// MetadataChecker interface for checking if an org has metadata
type MetadataChecker interface {
	OrgHasMetadata(ctx context.Context, orgID string) (bool, error)
}

// AutoProvisioner interface for auto-provisioning orgs
type AutoProvisioner interface {
	ProvisionDefaultMetadata(ctx context.Context, orgID string) error
}

// AuthMiddleware validates JWT tokens and API tokens, sets user context
type AuthMiddleware struct {
	authService       *service.AuthService
	apiTokenService   *service.APITokenService
	metadataChecker   MetadataChecker
	autoProvisioner   AutoProvisioner
	checkedOrgs       sync.Map // Cache of orgs that have been verified to have metadata
}

// NewAuthMiddleware creates a new AuthMiddleware
func NewAuthMiddleware(authService *service.AuthService, apiTokenService *service.APITokenService) *AuthMiddleware {
	return &AuthMiddleware{
		authService:     authService,
		apiTokenService: apiTokenService,
	}
}

// SetAutoProvisioning enables automatic provisioning for orgs missing metadata
func (m *AuthMiddleware) SetAutoProvisioning(checker MetadataChecker, provisioner AutoProvisioner) {
	m.metadataChecker = checker
	m.autoProvisioner = provisioner
}

// ensureOrgProvisioned checks if an org has metadata and provisions if missing
// This is called after successful authentication to self-heal orgs with missing metadata
func (m *AuthMiddleware) ensureOrgProvisioned(ctx context.Context, orgID string) {
	// Skip if auto-provisioning not configured
	if m.metadataChecker == nil || m.autoProvisioner == nil {
		return
	}

	// Skip if we've already checked this org
	if _, checked := m.checkedOrgs.Load(orgID); checked {
		return
	}

	// Check if org has metadata
	hasMetadata, err := m.metadataChecker.OrgHasMetadata(ctx, orgID)
	if err != nil {
		log.Printf("[AutoProvision] Error checking metadata for org %s: %v", orgID, err)
		return
	}

	if !hasMetadata {
		log.Printf("[AutoProvision] Org %s missing metadata, auto-provisioning...", orgID)
		if err := m.autoProvisioner.ProvisionDefaultMetadata(ctx, orgID); err != nil {
			log.Printf("[AutoProvision] ERROR: Failed to provision org %s: %v", orgID, err)
			// Don't mark as checked so we can retry on next request
			return
		}
		log.Printf("[AutoProvision] Successfully provisioned org %s", orgID)
	}

	// Mark this org as checked
	m.checkedOrgs.Store(orgID, true)
}

// Required returns middleware that requires authentication
// All protected routes should use this
// Supports both JWT tokens and API tokens (prefixed with fcr_)
func (m *AuthMiddleware) Required() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format",
			})
		}

		token := parts[1]
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token required",
			})
		}

		// Check if this is an API token (starts with fcr_)
		if strings.HasPrefix(token, "fcr_") {
			return m.handleAPIToken(c, token)
		}

		// Otherwise, validate as JWT
		claims, err := m.authService.ValidateAccessToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Set user context for downstream handlers
		m.setJWTContext(c, claims)

		// Auto-provision if org is missing metadata (self-healing)
		m.ensureOrgProvisioned(c.Context(), claims.OrgID)

		return c.Next()
	}
}

// handleAPIToken validates an API token and sets context
func (m *AuthMiddleware) handleAPIToken(c *fiber.Ctx, token string) error {
	if m.apiTokenService == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "API tokens not supported",
		})
	}

	claims, err := m.apiTokenService.ValidateToken(c.Context(), token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired API token",
		})
	}

	// Set context for API token - limited info compared to JWT
	c.Locals("orgID", claims.OrgID)
	c.Locals("apiTokenID", claims.TokenID)
	c.Locals("apiTokenScopes", claims.Scopes)
	c.Locals("isAPIToken", true)
	// API tokens don't have a user context - they're org-level
	c.Locals("userID", "")
	c.Locals("email", "")
	c.Locals("role", entity.RoleUser) // Default role for API tokens
	c.Locals("isPlatformAdmin", false)
	c.Locals("isImpersonation", false)

	return c.Next()
}

// setJWTContext sets context values from JWT claims
func (m *AuthMiddleware) setJWTContext(c *fiber.Ctx, claims *entity.TokenClaims) {
	c.Locals("userID", claims.UserID)
	c.Locals("orgID", claims.OrgID)
	c.Locals("email", claims.Email)
	c.Locals("role", claims.Role)
	c.Locals("isPlatformAdmin", claims.IsPlatformAdmin)
	c.Locals("isImpersonation", claims.IsImpersonation)
	c.Locals("isAPIToken", false)
	if claims.ImpersonatedBy != "" {
		c.Locals("impersonatedBy", claims.ImpersonatedBy)
	}
}

// Optional returns middleware that checks authentication but doesn't require it
// Use this for routes that behave differently for authenticated users
// Supports both JWT tokens and API tokens (prefixed with fcr_)
func (m *AuthMiddleware) Optional() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Next()
		}

		token := parts[1]
		if token == "" {
			return c.Next()
		}

		// Check if this is an API token
		if strings.HasPrefix(token, "fcr_") {
			if m.apiTokenService != nil {
				claims, err := m.apiTokenService.ValidateToken(c.Context(), token)
				if err == nil {
					c.Locals("orgID", claims.OrgID)
					c.Locals("apiTokenID", claims.TokenID)
					c.Locals("apiTokenScopes", claims.Scopes)
					c.Locals("isAPIToken", true)
					c.Locals("userID", "")
					c.Locals("email", "")
					c.Locals("role", entity.RoleUser)
					c.Locals("isPlatformAdmin", false)
					c.Locals("isImpersonation", false)
				}
			}
			return c.Next()
		}

		// Validate as JWT
		claims, err := m.authService.ValidateAccessToken(token)
		if err != nil {
			return c.Next()
		}

		// Set user context
		m.setJWTContext(c, claims)
		return c.Next()
	}
}

// RequireScope returns middleware that requires specific API token scope
// Use after Required() middleware for routes that need scope checking
func (m *AuthMiddleware) RequireScope(scope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// If not an API token, allow (JWT tokens have full access)
		isAPIToken, ok := c.Locals("isAPIToken").(bool)
		if !ok || !isAPIToken {
			return c.Next()
		}

		// Check scopes
		scopes, ok := c.Locals("apiTokenScopes").([]string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Insufficient permissions",
			})
		}

		for _, s := range scopes {
			if s == scope {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "API token missing required scope: " + scope,
		})
	}
}

// RequireWriteScope is a convenience middleware for write operations
func (m *AuthMiddleware) RequireWriteScope() fiber.Handler {
	return m.RequireScope(entity.ScopeWrite)
}

// RequireReadScope is a convenience middleware for read operations
func (m *AuthMiddleware) RequireReadScope() fiber.Handler {
	return m.RequireScope(entity.ScopeRead)
}

// PlatformAdminRequired returns middleware that requires platform admin privileges
func (m *AuthMiddleware) PlatformAdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// First, require authentication
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format",
			})
		}

		token := parts[1]
		claims, err := m.authService.ValidateAccessToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Check platform admin status
		if !claims.IsPlatformAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Platform administrator privileges required",
			})
		}

		// Block access during impersonation - platform admin actions should not be performed while impersonating
		if claims.IsImpersonation {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Platform administration not available during impersonation",
			})
		}

		// Set user context
		c.Locals("userID", claims.UserID)
		c.Locals("orgID", claims.OrgID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)
		c.Locals("isPlatformAdmin", claims.IsPlatformAdmin)
		c.Locals("isImpersonation", claims.IsImpersonation)
		if claims.ImpersonatedBy != "" {
			c.Locals("impersonatedBy", claims.ImpersonatedBy)
		}

		return c.Next()
	}
}

// OrgAdminRequired returns middleware that requires org admin or owner role
func (m *AuthMiddleware) OrgAdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// First, require authentication
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format",
			})
		}

		token := parts[1]
		claims, err := m.authService.ValidateAccessToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Platform admins can always access
		if claims.IsPlatformAdmin {
			c.Locals("userID", claims.UserID)
			c.Locals("orgID", claims.OrgID)
			c.Locals("email", claims.Email)
			c.Locals("role", claims.Role)
			c.Locals("isPlatformAdmin", claims.IsPlatformAdmin)
			c.Locals("isImpersonation", claims.IsImpersonation)
			if claims.ImpersonatedBy != "" {
				c.Locals("impersonatedBy", claims.ImpersonatedBy)
			}
			return c.Next()
		}

		// Check org admin/owner role
		if claims.Role != "admin" && claims.Role != "owner" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Organization administrator privileges required",
			})
		}

		// Set user context
		c.Locals("userID", claims.UserID)
		c.Locals("orgID", claims.OrgID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)
		c.Locals("isPlatformAdmin", claims.IsPlatformAdmin)
		c.Locals("isImpersonation", claims.IsImpersonation)
		if claims.ImpersonatedBy != "" {
			c.Locals("impersonatedBy", claims.ImpersonatedBy)
		}

		return c.Next()
	}
}

// OrgOwnerRequired returns middleware that requires org owner role
// Used for destructive operations like deleting the organization or transferring ownership
func (m *AuthMiddleware) OrgOwnerRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// First, require authentication
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format",
			})
		}

		token := parts[1]
		claims, err := m.authService.ValidateAccessToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Platform admins can always access
		if claims.IsPlatformAdmin {
			c.Locals("userID", claims.UserID)
			c.Locals("orgID", claims.OrgID)
			c.Locals("email", claims.Email)
			c.Locals("role", claims.Role)
			c.Locals("isPlatformAdmin", claims.IsPlatformAdmin)
			c.Locals("isImpersonation", claims.IsImpersonation)
			if claims.ImpersonatedBy != "" {
				c.Locals("impersonatedBy", claims.ImpersonatedBy)
			}
			return c.Next()
		}

		// Check owner role only
		if claims.Role != "owner" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Organization owner privileges required",
			})
		}

		// Set user context
		c.Locals("userID", claims.UserID)
		c.Locals("orgID", claims.OrgID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)
		c.Locals("isPlatformAdmin", claims.IsPlatformAdmin)
		c.Locals("isImpersonation", claims.IsImpersonation)
		if claims.ImpersonatedBy != "" {
			c.Locals("impersonatedBy", claims.ImpersonatedBy)
		}

		return c.Next()
	}
}
