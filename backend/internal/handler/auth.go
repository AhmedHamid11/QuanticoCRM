package handler

import (
	"errors"
	"log"
	"strings"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// AuthHandler handles HTTP requests for authentication
type AuthHandler struct {
	authService  *service.AuthService
	auditLogger  *service.AuditLogger
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		auditLogger: service.NewAuditLogger(),
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

	return c.Status(fiber.StatusCreated).JSON(response)
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
		return h.handleAuthError(c, err)
	}

	return c.JSON(response)
}

// RefreshToken handles token refresh
// POST /auth/refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var input entity.RefreshInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Refresh token is required",
		})
	}

	response, err := h.authService.RefreshTokens(c.Context(), input.RefreshToken)
	if err != nil {
		return h.handleAuthError(c, err)
	}

	return c.JSON(response)
}

// Logout handles user logout
// POST /auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var input entity.RefreshInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Refresh token is required",
		})
	}

	if err := h.authService.Logout(c.Context(), input.RefreshToken); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to logout",
		})
	}

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

	return c.JSON(response)
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

	return c.JSON(response)
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

	return c.JSON(response)
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

	return c.JSON(response)
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

	if err := h.authService.ChangePassword(c.Context(), userID, input); err != nil {
		return h.handleAuthError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "Password changed successfully",
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
	case errors.Is(err, service.ErrPasswordTooWeak):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be at least 8 characters",
		})
	case errors.Is(err, service.ErrInvalidEmail):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid email address",
		})
	default:
		// Check for permission errors
		if strings.Contains(err.Error(), "only owners and admins") {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "An unexpected error occurred",
		})
	}
}
