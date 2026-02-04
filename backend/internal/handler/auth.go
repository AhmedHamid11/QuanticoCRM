package handler

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/service"
	"github.com/fastcrm/backend/internal/util"
	"github.com/gofiber/fiber/v2"
)

// AuthHandler handles HTTP requests for authentication
type AuthHandler struct {
	authService  *service.AuthService
	auditLogger  *service.AuditLogger
	isProduction bool
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(authService *service.AuthService, auditLogger *service.AuditLogger, isProduction bool) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		auditLogger:  auditLogger,
		isProduction: isProduction,
	}
}

// Register handles user registration with a new organization
// POST /auth/register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var input entity.RegisterInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if input.Email == "" || input.Password == "" || input.OrgName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email, password, and organization name are required",
		})
	}

	// Normalize email to lowercase
	input.Email = strings.ToLower(input.Email)

	response, err := h.authService.Register(c.Context(), input)
	if err != nil {
		log.Printf("Register error: %v", err)
		return h.handleAuthError(c, err)
	}

	// SECURITY: Set refresh token as HttpOnly cookie
	middleware.SetRefreshTokenCookie(c, response.RefreshToken, h.isProduction)

	// Return access token in body only (refresh token is in cookie)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"accessToken": response.AccessToken,
		"expiresAt":   response.ExpiresAt,
		"user":        response.User,
	})
}

// Login handles user login
// POST /auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var input entity.LoginInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Email == "" || input.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	// Normalize email to lowercase for case-insensitive login
	input.Email = strings.ToLower(input.Email)

	userAgent := c.Get("User-Agent")
	ipAddress := c.IP()

	response, err := h.authService.Login(c.Context(), input, userAgent, ipAddress)
	if err != nil {
		log.Printf("Login error: %v", err)
		// AUDIT: Log failed login attempt
		go h.auditLogger.LogLoginAttempt(c.Context(), input.Email, ipAddress, userAgent, false, err.Error())
		return h.handleAuthError(c, err)
	}

	// AUDIT: Log successful login
	go h.auditLogger.LogLoginAttempt(c.Context(), input.Email, ipAddress, userAgent, true, "")

	// SECURITY: Set refresh token as HttpOnly cookie
	middleware.SetRefreshTokenCookie(c, response.RefreshToken, h.isProduction)

	// Return access token in body only (refresh token is in cookie)
	return c.JSON(fiber.Map{
		"accessToken": response.AccessToken,
		"expiresAt":   response.ExpiresAt,
		"user":        response.User,
	})
}

// RefreshToken handles token refresh
// POST /auth/refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	// Read refresh token from HttpOnly cookie (not body)
	refreshToken := middleware.GetRefreshTokenFromCookie(c)
	if refreshToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Refresh token not found",
		})
	}

	response, err := h.authService.RefreshTokens(c.Context(), refreshToken)
	if err != nil {
		// On any auth error, clear the cookie
		middleware.ClearRefreshTokenCookie(c, h.isProduction)
		return h.handleAuthError(c, err)
	}

	// Set new refresh token cookie (rotation)
	middleware.SetRefreshTokenCookie(c, response.RefreshToken, h.isProduction)

	// Return new access token in body
	return c.JSON(fiber.Map{
		"accessToken": response.AccessToken,
		"expiresAt":   response.ExpiresAt,
		"user":        response.User,
	})
}

// Logout handles user logout
// POST /auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// Read refresh token from cookie
	refreshToken := middleware.GetRefreshTokenFromCookie(c)

	if refreshToken != "" {
		if err := h.authService.Logout(c.Context(), refreshToken); err != nil {
			log.Printf("Logout error: %v", err)
			// Continue to clear cookie even if server-side logout fails
		}
	}

	// AUDIT: Log logout event
	if userID, ok := c.Locals("userID").(string); ok {
		email, _ := c.Locals("email").(string)
		orgID, _ := c.Locals("orgID").(string)
		go h.auditLogger.LogLogout(c.Context(), userID, email, orgID, c.IP(), c.Get("User-Agent"))
	}

	// Always clear the cookie
	middleware.ClearRefreshTokenCookie(c, h.isProduction)

	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
	})
}

// LogoutAll handles logging out all sessions for a user
// POST /auth/logout-all
func (h *AuthHandler) LogoutAll(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	if err := h.authService.LogoutAll(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to logout all sessions",
		})
	}

	return c.JSON(fiber.Map{
		"message": "All sessions logged out successfully",
	})
}

// Me returns the current authenticated user
// GET /auth/me
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	orgID := c.Locals("orgID").(string)

	currentUser, err := h.authService.GetCurrentUser(c.Context(), userID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user info",
		})
	}

	// Add impersonation info from context if present
	if isImpersonation, ok := c.Locals("isImpersonation").(bool); ok && isImpersonation {
		currentUser.IsImpersonation = true
		if impersonatedBy, ok := c.Locals("impersonatedBy").(string); ok {
			currentUser.ImpersonatedBy = impersonatedBy
		}
	}

	return c.JSON(currentUser)
}

// GetUserOrgs returns all organizations the user belongs to
// GET /auth/orgs
func (h *AuthHandler) GetUserOrgs(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	userWithOrgs, err := h.authService.GetUserWithOrgs(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get organizations",
		})
	}

	return c.JSON(fiber.Map{
		"memberships": userWithOrgs.Memberships,
	})
}

// SwitchOrg switches the user's active organization
// POST /auth/switch-org
func (h *AuthHandler) SwitchOrg(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var input entity.SwitchOrgInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.OrgID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Organization ID is required",
		})
	}

	response, err := h.authService.SwitchOrganization(c.Context(), userID, input.OrgID)
	if err != nil {
		return h.handleAuthError(c, err)
	}

	// SECURITY: Set refresh token as HttpOnly cookie
	middleware.SetRefreshTokenCookie(c, response.RefreshToken, h.isProduction)

	// Return access token in body only
	return c.JSON(fiber.Map{
		"accessToken": response.AccessToken,
		"expiresAt":   response.ExpiresAt,
		"user":        response.User,
	})
}

// Impersonate allows platform admins to impersonate organizations/users
// POST /auth/impersonate
func (h *AuthHandler) Impersonate(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	userEmail, _ := c.Locals("email").(string)

	var input entity.ImpersonateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.OrgID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Organization ID is required",
		})
	}

	response, err := h.authService.Impersonate(c.Context(), userID, input)
	if err != nil {
		log.Printf("Impersonate error: %v", err)
		return h.handleAuthError(c, err)
	}

	// SECURITY: Audit log impersonation start
	h.auditLogger.LogImpersonationStart(
		c.Context(),
		userID,
		userEmail,
		input.OrgID,
		input.UserID,
		c.IP(),
		c.Get("User-Agent"),
	)

	// SECURITY: Set refresh token as HttpOnly cookie
	middleware.SetRefreshTokenCookie(c, response.RefreshToken, h.isProduction)

	// Return access token in body only
	return c.JSON(fiber.Map{
		"accessToken": response.AccessToken,
		"expiresAt":   response.ExpiresAt,
		"user":        response.User,
	})
}

// StopImpersonate stops impersonation and returns to admin's own session
// POST /auth/stop-impersonate
func (h *AuthHandler) StopImpersonate(c *fiber.Ctx) error {
	log.Printf("[v9] StopImpersonate handler called")
	// Get the original admin user ID from the impersonation context
	impersonatedBy := c.Locals("impersonatedBy")
	if impersonatedBy == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "[v9] Not currently impersonating",
		})
	}

	adminUserID := impersonatedBy.(string)

	response, err := h.authService.StopImpersonation(c.Context(), adminUserID)
	if err != nil {
		return h.handleAuthError(c, err)
	}

	// SECURITY: Audit log impersonation stop
	h.auditLogger.LogImpersonationStop(
		c.Context(),
		adminUserID,
		response.User.Email,
		c.IP(),
		c.Get("User-Agent"),
	)

	// SECURITY: Set refresh token as HttpOnly cookie
	middleware.SetRefreshTokenCookie(c, response.RefreshToken, h.isProduction)

	// Return access token in body only
	return c.JSON(fiber.Map{
		"accessToken": response.AccessToken,
		"expiresAt":   response.ExpiresAt,
		"user":        response.User,
	})
}

// InviteUser invites a user to the current organization
// POST /auth/invite
func (h *AuthHandler) InviteUser(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	orgID := c.Locals("orgID").(string)

	var input entity.InvitationInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}

	// Normalize email to lowercase
	input.Email = strings.ToLower(input.Email)

	invitation, err := h.authService.InviteUser(c.Context(), userID, orgID, input)
	if err != nil {
		return h.handleAuthError(c, err)
	}

	// Include token in response (needed for sending invitation email)
	// Token is excluded from Invitation JSON by default for security
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":    "Invitation sent successfully",
		"invitation": invitation,
		"token":      invitation.Token,
	})
}

// ListInvitations returns pending invitations for the current organization
// GET /auth/invitations
func (h *AuthHandler) ListInvitations(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	invitations, err := h.authService.ListPendingInvitations(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list invitations",
		})
	}

	return c.JSON(fiber.Map{
		"invitations": invitations,
	})
}

// DeleteInvitation cancels a pending invitation
// DELETE /auth/invitations/:id
func (h *AuthHandler) DeleteInvitation(c *fiber.Ctx) error {
	invitationID := c.Params("id")

	if err := h.authService.DeleteInvitation(c.Context(), invitationID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete invitation",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Invitation cancelled",
	})
}

// AcceptInvitation accepts an organization invitation
// POST /auth/accept-invite
func (h *AuthHandler) AcceptInvitation(c *fiber.Ctx) error {
	var input entity.AcceptInvitationInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invitation token is required",
		})
	}

	response, err := h.authService.AcceptInvitation(c.Context(), input)
	if err != nil {
		return h.handleAuthError(c, err)
	}

	// SECURITY: Set refresh token as HttpOnly cookie
	middleware.SetRefreshTokenCookie(c, response.RefreshToken, h.isProduction)

	// Return access token in body only
	return c.JSON(fiber.Map{
		"accessToken": response.AccessToken,
		"expiresAt":   response.ExpiresAt,
		"user":        response.User,
	})
}

// ChangePassword changes the current user's password
// POST /auth/change-password
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var input entity.ChangePasswordInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.CurrentPassword == "" || input.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Current password and new password are required",
		})
	}

	// Change password (invalidates all existing sessions)
	if err := h.authService.ChangePassword(c.Context(), userID, input); err != nil {
		return h.handleAuthError(c, err)
	}

	// AUDIT: Log password change
	email, _ := c.Locals("email").(string)
	orgID, _ := c.Locals("orgID").(string)
	go h.auditLogger.LogPasswordChange(c.Context(), userID, email, orgID, c.IP())

	// Generate new auth response with fresh tokens (mustChangePassword=false)
	// Use Login to get fresh credentials after password change
	loginInput := entity.LoginInput{
		Email:    email,
		Password: input.NewPassword,
	}
	response, err := h.authService.Login(c.Context(), loginInput, c.Get("User-Agent"), c.IP())
	if err != nil {
		// Password was changed but re-login failed
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Password changed but failed to generate new session",
			"message": "Please log in again with your new password",
		})
	}

	// SECURITY: Set refresh token as HttpOnly cookie
	middleware.SetRefreshTokenCookie(c, response.RefreshToken, h.isProduction)

	// Return access token and user info
	return c.JSON(fiber.Map{
		"message":     "Password changed successfully",
		"accessToken": response.AccessToken,
		"expiresAt":   response.ExpiresAt,
		"user":        response.User,
	})
}

// ForgotPassword initiates a password reset
// POST /auth/forgot-password
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var input entity.ForgotPasswordInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}

	// Normalize email to lowercase
	input.Email = strings.ToLower(input.Email)

	token, err := h.authService.ForgotPassword(c.Context(), input)
	if err != nil {
		log.Printf("Forgot password error: %v", err)
		// Always return success to prevent email enumeration
	}

	// SECURITY: Never expose reset token in response
	// In production, token should be sent via email
	// Log token in development for testing (check server logs, not HTTP response)
	if token != "" {
		log.Printf("[DEV ONLY] Password reset token for %s: %s", input.Email, token)
	}

	return c.JSON(fiber.Map{
		"message": "If an account exists with this email, a password reset link has been sent.",
	})
}

// ResetPassword resets password using a token
// POST /auth/reset-password
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var input entity.ResetPasswordInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Reset token is required",
		})
	}

	if input.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "New password is required",
		})
	}

	if err := h.authService.ResetPassword(c.Context(), input); err != nil {
		log.Printf("Reset password error: %v", err)
		return h.handleAuthError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "Password has been reset successfully. You can now log in with your new password.",
	})
}

// ExtendSession explicitly extends the session when user clicks "Stay logged in"
// POST /auth/extend-session
func (h *AuthHandler) ExtendSession(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	orgID := c.Locals("orgID").(string)

	err := h.authService.ExtendSession(c.Context(), userID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to extend session",
		})
	}

	// Calculate new idle expiry time for frontend countdown reset
	// Default idle timeout is 30 minutes
	session, err := h.authService.GetSessionWithTimeouts(c.Context(), userID, orgID)
	if err != nil {
		// Session extended but couldn't fetch details
		return c.JSON(fiber.Map{
			"message": "Session extended",
		})
	}

	// Return the new idle expiry time
	idleExpiresAt := session.LastActivityAt.Add(time.Duration(session.IdleTimeoutMinutes) * time.Minute)

	return c.JSON(fiber.Map{
		"message":       "Session extended",
		"idleExpiresAt": idleExpiresAt,
	})
}

// handleAuthError converts service errors to appropriate HTTP responses
func (h *AuthHandler) handleAuthError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	case errors.Is(err, service.ErrUserNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	case errors.Is(err, service.ErrUserInactive):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "User account is inactive",
		})
	case errors.Is(err, service.ErrOrgNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Organization not found",
		})
	case errors.Is(err, service.ErrOrgInactive):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Organization is inactive",
		})
	case errors.Is(err, service.ErrEmailExists):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Email already registered",
		})
	case errors.Is(err, service.ErrSlugExists):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Organization slug already exists",
		})
	case errors.Is(err, service.ErrInvalidToken):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	case errors.Is(err, service.ErrTokenReuse):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Session invalidated for security reasons. Please log in again.",
		})
	case errors.Is(err, service.ErrNotMember):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "User is not a member of this organization",
		})
	case errors.Is(err, service.ErrNotPlatformAdmin):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "This action requires platform administrator privileges",
		})
	case errors.Is(err, service.ErrInvalidInvitation):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid or expired invitation",
		})
	case errors.Is(err, service.ErrPasswordTooWeak), errors.Is(err, service.ErrPasswordTooShort):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be at least 8 characters",
		})
	case errors.Is(err, service.ErrPasswordTooLong):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be 128 characters or less",
		})
	case errors.Is(err, service.ErrPasswordCommon):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "This password is too common, please choose a different one",
		})
	case errors.Is(err, service.ErrInvalidEmail):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid email address",
		})
	default:
		// Check for permission errors - these are safe to expose as they contain no internal details
		if strings.Contains(err.Error(), "only owners and admins") {
			return util.NewAPIErrorWithMessage(c, fiber.StatusForbidden, err.Error(), util.ErrCategoryPermission)
		}
		return util.NewAPIError(c, fiber.StatusInternalServerError, err, util.ErrCategoryInternal)
	}
}
