package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fastcrm/backend/internal/data"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserInactive       = errors.New("user account is inactive")
	ErrOrgNotFound        = errors.New("organization not found")
	ErrOrgInactive        = errors.New("organization is inactive")
	ErrEmailExists        = errors.New("email already registered")
	ErrSlugExists         = errors.New("organization slug already exists")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrTokenReuse         = errors.New("refresh token reuse detected")
	ErrNotMember          = errors.New("user is not a member of this organization")
	ErrNotPlatformAdmin   = errors.New("user is not a platform administrator")
	ErrInvalidInvitation  = errors.New("invalid or expired invitation")
	ErrPasswordTooWeak    = errors.New("password must be at least 8 characters")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong    = errors.New("password must be 128 characters or less")
	ErrPasswordCommon     = errors.New("this password is too common, please choose a different one")
	ErrInvalidEmail       = errors.New("invalid email address")
)

// AuthConfig holds configuration for the auth service
type AuthConfig struct {
	JWTSecret          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	InvitationExpiry   time.Duration
	BcryptCost         int
}

// DefaultAuthConfig returns sensible default configuration
func DefaultAuthConfig(jwtSecret string) AuthConfig {
	return AuthConfig{
		JWTSecret:          jwtSecret,
		AccessTokenExpiry:  24 * time.Hour,     // 24 hours
		RefreshTokenExpiry: 7 * 24 * time.Hour, // 7 days
		InvitationExpiry:   7 * 24 * time.Hour, // 7 days
		BcryptCost:         12,
	}
}

// AuthService handles authentication business logic
type AuthService struct {
	repo               *repo.AuthRepo
	config             AuthConfig
	provisioning       *ProvisioningService
	tenantProvisioning *TenantProvisioningService
	versionRepo        *repo.VersionRepo // For platform version lookups during org creation
}

// NewAuthService creates a new AuthService
func NewAuthService(repo *repo.AuthRepo, config AuthConfig, provisioning *ProvisioningService) *AuthService {
	return &AuthService{
		repo:         repo,
		config:       config,
		provisioning: provisioning,
	}
}

// SetTenantProvisioning sets the tenant provisioning service
// This is called after initialization to avoid circular dependencies
func (s *AuthService) SetTenantProvisioning(tp *TenantProvisioningService) {
	s.tenantProvisioning = tp
}

// SetVersionRepo sets the version repository for platform version lookups
// This is called after initialization to avoid circular dependencies
func (s *AuthService) SetVersionRepo(vr *repo.VersionRepo) {
	s.versionRepo = vr
}

// --- Registration ---

// Register creates a new user and organization
func (s *AuthService) Register(ctx context.Context, input entity.RegisterInput) (*entity.AuthResponse, error) {
	// Validate input
	if err := s.validateEmail(input.Email); err != nil {
		return nil, err
	}
	if err := s.validatePassword(input.Password); err != nil {
		return nil, err
	}

	// Check if email already exists
	_, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err == nil {
		return nil, ErrEmailExists
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("checking email: %w", err)
	}

	// Generate org slug from name
	slug := s.generateSlug(input.OrgName)

	// Check if slug exists and make unique if needed
	slug, err = s.ensureUniqueSlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("generating slug: %w", err)
	}

	// Hash password
	passwordHash, err := s.hashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	// Get current platform version for new org
	platformVersion := "v0.1.0" // Default fallback
	if s.versionRepo != nil {
		pv, err := s.versionRepo.GetPlatformVersion(ctx)
		if err != nil {
			log.Printf("[Register] Warning: failed to get platform version: %v, using default", err)
		} else {
			platformVersion = pv.Version
			log.Printf("[Register] New org will be created at platform version %s", platformVersion)
		}
	}

	// Create organization with tenant database
	var org *entity.Organization
	if s.tenantProvisioning != nil {
		// First create the org record (without database info)
		org, err = s.repo.CreateOrganization(ctx, entity.OrganizationCreateInput{
			Name:           input.OrgName,
			Slug:           slug,
			CurrentVersion: platformVersion,
		})
		if err != nil {
			return nil, fmt.Errorf("creating organization: %w", err)
		}

		// Provision tenant database and metadata
		tenantDB, err := s.tenantProvisioning.ProvisionTenant(ctx, org.ID, slug)
		if err != nil {
			// Log but don't fail registration if provisioning fails
			fmt.Printf("Warning: failed to provision tenant database: %v\n", err)
		} else if tenantDB.URL != "" {
			// Update org with database credentials
			if err := s.repo.UpdateOrganizationDatabase(ctx, org.ID, tenantDB.URL, tenantDB.Token, tenantDB.Name); err != nil {
				fmt.Printf("Warning: failed to save database credentials: %v\n", err)
			}
		}
	} else {
		// Legacy path: create org and provision metadata to shared DB
		org, err = s.repo.CreateOrganization(ctx, entity.OrganizationCreateInput{
			Name:           input.OrgName,
			Slug:           slug,
			CurrentVersion: platformVersion,
		})
		if err != nil {
			return nil, fmt.Errorf("creating organization: %w", err)
		}

		if s.provisioning != nil {
			if err := s.provisioning.ProvisionDefaultMetadata(ctx, org.ID); err != nil {
				fmt.Printf("Warning: failed to provision default metadata: %v\n", err)
			}
		}
	}

	// Create user
	user, err := s.repo.CreateUser(ctx, input.Email, passwordHash, input.FirstName, input.LastName)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	// Create membership (owner role, default org)
	_, err = s.repo.CreateMembership(ctx, user.ID, org.ID, entity.RoleOwner, true)
	if err != nil {
		return nil, fmt.Errorf("creating membership: %w", err)
	}

	// Generate tokens and return auth response
	return s.createAuthResponse(ctx, user, org.ID, false, nil)
}

// --- Login ---

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, input entity.LoginInput, userAgent, ipAddress string) (*entity.AuthResponse, error) {
	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("getting user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Verify password
	if !s.verifyPassword(input.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Check if password is weak (after successful authentication)
	mustChangePassword := s.isPasswordWeak(input.Password)

	// Get user's default organization
	membership, err := s.repo.GetDefaultMembership(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("getting default membership: %w", err)
	}

	// Update last login
	_ = s.repo.UpdateUserLastLogin(ctx, user.ID)

	// Generate tokens and return auth response with password change flag
	return s.createAuthResponseWithPasswordFlag(ctx, user, membership.OrgID, false, nil, mustChangePassword)
}

// --- Token Refresh ---

// RefreshTokens generates new access and refresh tokens with rotation and reuse detection
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (*entity.AuthResponse, error) {
	// Hash the refresh token to look it up
	tokenHash := s.hashToken(refreshToken)

	// Get session by refresh token - includes revoked sessions for reuse detection
	session, err := s.repo.GetSessionByRefreshToken(ctx, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("getting session: %w", err)
	}

	// SECURITY: Token reuse detection
	// If the token has already been used (is_revoked = true), an attacker may have stolen it
	// Invalidate the entire token family to kick out both legitimate user and attacker
	if session.IsRevoked {
		log.Printf("[SECURITY] Token reuse detected for family %s, user %s", session.FamilyID, session.UserID)
		// Revoke entire family - this invalidates all tokens from this login session
		_ = s.repo.RevokeTokenFamily(ctx, session.FamilyID)
		return nil, ErrTokenReuse
	}

	// Mark current token as revoked (it's been used for rotation)
	_ = s.repo.RevokeSession(ctx, session.ID)

	// Get user
	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Create new auth response with SAME family ID (rotation within family)
	return s.createAuthResponseWithFamily(ctx, user, session.OrgID, session.FamilyID, session.IsImpersonation, session.ImpersonatedBy)
}

// --- Logout ---

// Logout invalidates the refresh token
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := s.hashToken(refreshToken)
	return s.repo.DeleteSessionByRefreshToken(ctx, tokenHash)
}

// LogoutAll invalidates all sessions for a user
func (s *AuthService) LogoutAll(ctx context.Context, userID string) error {
	return s.repo.DeleteUserSessions(ctx, userID)
}

// --- Organization Switching ---

// SwitchOrganization switches the user's active organization
func (s *AuthService) SwitchOrganization(ctx context.Context, userID, orgID string) (*entity.AuthResponse, error) {
	// Verify user is a member of the organization
	_, err := s.repo.GetMembership(ctx, userID, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotMember
		}
		return nil, fmt.Errorf("getting membership: %w", err)
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	// Generate new tokens for the new org
	return s.createAuthResponse(ctx, user, orgID, false, nil)
}

// --- Impersonation (Platform Admin) ---

// Impersonate allows a platform admin to impersonate a user in an organization
func (s *AuthService) Impersonate(ctx context.Context, adminUserID string, input entity.ImpersonateInput) (*entity.AuthResponse, error) {
	// Get admin user
	adminUser, err := s.repo.GetUserByID(ctx, adminUserID)
	if err != nil {
		return nil, fmt.Errorf("getting admin user: %w", err)
	}

	// Verify admin is a platform admin
	if !adminUser.IsPlatformAdmin {
		return nil, ErrNotPlatformAdmin
	}

	// Get target organization
	org, err := s.repo.GetOrganizationByID(ctx, input.OrgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrOrgNotFound
		}
		return nil, fmt.Errorf("getting organization: %w", err)
	}

	var targetUser *entity.User

	if input.UserID != "" {
		// Impersonate specific user
		targetUser, err = s.repo.GetUserByID(ctx, input.UserID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrUserNotFound
			}
			return nil, fmt.Errorf("getting target user: %w", err)
		}

		// Verify target user is member of org
		_, err = s.repo.GetMembership(ctx, input.UserID, input.OrgID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrNotMember
			}
			return nil, fmt.Errorf("getting membership: %w", err)
		}
	} else {
		// Create a virtual user for impersonation (without platform admin privileges)
		// This allows platform admins to see exactly what regular org users see
		targetUser = &entity.User{
			ID:              adminUser.ID,
			Email:           adminUser.Email,
			FirstName:       adminUser.FirstName,
			LastName:        adminUser.LastName,
			IsActive:        true,
			IsPlatformAdmin: false,
		}
	}

	// Generate tokens with impersonation flag
	return s.createAuthResponse(ctx, targetUser, org.ID, true, &adminUser.ID)
}

// StopImpersonation returns to the admin's own session
func (s *AuthService) StopImpersonation(ctx context.Context, adminUserID string) (*entity.AuthResponse, error) {
	// Get admin user
	adminUser, err := s.repo.GetUserByID(ctx, adminUserID)
	if err != nil {
		return nil, fmt.Errorf("getting admin user: %w", err)
	}

	// Get admin's default org
	membership, err := s.repo.GetDefaultMembership(ctx, adminUserID)
	if err != nil {
		// If no membership, this is a platform admin without an org
		// They can still use the platform but won't have a default org
		return nil, fmt.Errorf("getting default membership: %w", err)
	}

	return s.createAuthResponse(ctx, adminUser, membership.OrgID, false, nil)
}

// --- Invitations ---

// InviteUser creates an invitation for a user to join an organization
func (s *AuthService) InviteUser(ctx context.Context, inviterID, orgID string, input entity.InvitationInput) (*entity.Invitation, error) {
	// Verify inviter has permission (admin or owner)
	membership, err := s.repo.GetMembership(ctx, inviterID, orgID)
	if err != nil {
		return nil, fmt.Errorf("getting inviter membership: %w", err)
	}

	if membership.Role != entity.RoleOwner && membership.Role != entity.RoleAdmin {
		return nil, errors.New("only owners and admins can invite users")
	}

	// Admins cannot invite users as owners
	if membership.Role == entity.RoleAdmin && input.Role == entity.RoleOwner {
		return nil, errors.New("only owners can invite users as owners")
	}

	// Check tier limits
	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("getting organization: %w", err)
	}

	maxUsers := entity.GetMaxUsers(org.Plan)
	if maxUsers > 0 { // 0 means unlimited
		currentMembers, err := s.repo.CountOrgMembers(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("counting members: %w", err)
		}

		pendingInvites, err := s.repo.CountPendingInvitations(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("counting invitations: %w", err)
		}

		totalUsers := currentMembers + pendingInvites
		if totalUsers >= maxUsers {
			return nil, fmt.Errorf("user limit reached: your %s plan allows %d users (upgrade to Pro for unlimited users)", org.Plan, maxUsers)
		}
	}

	// Validate email
	if err := s.validateEmail(input.Email); err != nil {
		return nil, err
	}

	// Generate invitation token
	token, err := s.generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}

	// SECURITY: Hash the token before storing to protect against DB leakage
	tokenHash := s.hashToken(token)

	// Set default role
	role := input.Role
	if role == "" {
		role = entity.RoleUser
	}

	// Create invitation with hashed token
	expiresAt := time.Now().Add(s.config.InvitationExpiry)
	invitation, err := s.repo.CreateInvitation(ctx, orgID, input.Email, role, tokenHash, inviterID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("creating invitation: %w", err)
	}

	// Return original token (not hash) so it can be sent to the user
	// The stored hash is only for lookup
	invitation.Token = token

	return invitation, nil
}

// AcceptInvitation accepts an invitation and creates/adds the user
func (s *AuthService) AcceptInvitation(ctx context.Context, input entity.AcceptInvitationInput) (*entity.AuthResponse, error) {
	// SECURITY: Hash the token before lookup (tokens are stored as hashes)
	tokenHash := s.hashToken(input.Token)

	// Get invitation by token hash
	invitation, err := s.repo.GetInvitationByToken(ctx, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidInvitation
		}
		return nil, fmt.Errorf("getting invitation: %w", err)
	}

	// Check if user already exists
	user, err := s.repo.GetUserByEmail(ctx, invitation.Email)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("checking user: %w", err)
	}

	if user == nil {
		// Create new user
		if err := s.validatePassword(input.Password); err != nil {
			return nil, err
		}

		passwordHash, err := s.hashPassword(input.Password)
		if err != nil {
			return nil, fmt.Errorf("hashing password: %w", err)
		}

		user, err = s.repo.CreateUser(ctx, invitation.Email, passwordHash, input.FirstName, input.LastName)
		if err != nil {
			return nil, fmt.Errorf("creating user: %w", err)
		}
	}

	// Check if already a member
	_, err = s.repo.GetMembership(ctx, user.ID, invitation.OrgID)
	if err == nil {
		// Already a member, just mark invitation as accepted
		_ = s.repo.MarkInvitationAccepted(ctx, invitation.ID)
		return s.createAuthResponse(ctx, user, invitation.OrgID, false, nil)
	}

	// Create membership
	// For new users (no existing memberships), make this their default org
	memberships, _ := s.repo.GetUserMemberships(ctx, user.ID)
	isDefault := len(memberships) == 0
	_, err = s.repo.CreateMembership(ctx, user.ID, invitation.OrgID, invitation.Role, isDefault)
	if err != nil {
		return nil, fmt.Errorf("creating membership: %w", err)
	}

	// Mark invitation as accepted
	_ = s.repo.MarkInvitationAccepted(ctx, invitation.ID)

	// Return auth response for the new org
	return s.createAuthResponse(ctx, user, invitation.OrgID, false, nil)
}

// ListPendingInvitations returns all pending invitations for an organization
func (s *AuthService) ListPendingInvitations(ctx context.Context, orgID string) ([]entity.InvitationWithDetails, error) {
	return s.repo.ListPendingInvitations(ctx, orgID)
}

// DeleteInvitation cancels a pending invitation
func (s *AuthService) DeleteInvitation(ctx context.Context, invitationID string) error {
	return s.repo.DeleteInvitation(ctx, invitationID)
}

// --- Password Management ---

// ChangePassword changes a user's password
func (s *AuthService) ChangePassword(ctx context.Context, userID string, input entity.ChangePasswordInput) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("getting user: %w", err)
	}

	// Verify current password
	if !s.verifyPassword(input.CurrentPassword, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	// Validate new password
	if err := s.validatePassword(input.NewPassword); err != nil {
		return err
	}

	// Hash new password
	newHash, err := s.hashPassword(input.NewPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	// Update password
	if err := s.repo.UpdateUserPassword(ctx, userID, newHash); err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	// Invalidate all existing sessions
	_ = s.repo.DeleteUserSessions(ctx, userID)

	return nil
}

// ForgotPassword creates a password reset token for a user
// Returns the token (in production, this would be sent via email)
func (s *AuthService) ForgotPassword(ctx context.Context, input entity.ForgotPasswordInput) (string, error) {
	// Validate email format
	if err := s.validateEmail(input.Email); err != nil {
		return "", err
	}

	// Get user by email - but don't reveal if user exists or not
	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return success even if user doesn't exist (security best practice)
			return "", nil
		}
		return "", fmt.Errorf("checking user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		// Return success even if user is inactive (security best practice)
		return "", nil
	}

	// Generate reset token
	token, err := s.generateSecureToken(32)
	if err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}

	// Hash the token before storing
	tokenHash := s.hashToken(token)

	// Set expiry (1 hour)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Delete any existing reset tokens for this user
	_ = s.repo.DeletePasswordResetTokens(ctx, user.ID)

	// Create new reset token
	if err := s.repo.CreatePasswordResetToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return "", fmt.Errorf("creating reset token: %w", err)
	}

	// In production, send email here
	// For now, return the token (to be displayed in UI or logged)
	return token, nil
}

// ResetPassword resets a user's password using a reset token
func (s *AuthService) ResetPassword(ctx context.Context, input entity.ResetPasswordInput) error {
	// Validate new password
	if err := s.validatePassword(input.NewPassword); err != nil {
		return err
	}

	// Hash the token to look it up
	tokenHash := s.hashToken(input.Token)

	// Get reset token
	resetToken, err := s.repo.GetPasswordResetToken(ctx, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrInvalidToken
		}
		return fmt.Errorf("getting reset token: %w", err)
	}

	// Check if token has been used
	if resetToken.UsedAt != nil {
		return ErrInvalidToken
	}

	// Check if token has expired
	if time.Now().After(resetToken.ExpiresAt) {
		return ErrInvalidToken
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, resetToken.UserID)
	if err != nil {
		return fmt.Errorf("getting user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return ErrUserInactive
	}

	// Hash new password
	newHash, err := s.hashPassword(input.NewPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	// Update password
	if err := s.repo.UpdateUserPassword(ctx, resetToken.UserID, newHash); err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	// Mark token as used
	if err := s.repo.MarkPasswordResetTokenUsed(ctx, resetToken.ID); err != nil {
		return fmt.Errorf("marking token used: %w", err)
	}

	// Invalidate all existing sessions
	_ = s.repo.DeleteUserSessions(ctx, resetToken.UserID)

	return nil
}

// --- Token Validation ---

// ValidateAccessToken validates an access token and returns the claims
func (s *AuthService) ValidateAccessToken(tokenString string) (*entity.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return &entity.TokenClaims{
		UserID:             claims["userId"].(string),
		OrgID:              claims["orgId"].(string),
		Email:              claims["email"].(string),
		Role:               claims["role"].(string),
		IsPlatformAdmin:    claims["isPlatformAdmin"].(bool),
		IsImpersonation:    claims["isImpersonation"].(bool),
		ImpersonatedBy:     getStringClaim(claims, "impersonatedBy"),
		MustChangePassword: getBoolClaim(claims, "mustChangePassword"),
	}, nil
}

// --- Helper Functions ---

// createAuthResponse creates a new auth response with a NEW token family
// Used for login, register, org switch, stop impersonation (security: new context = new family)
func (s *AuthService) createAuthResponse(ctx context.Context, user *entity.User, orgID string, isImpersonation bool, impersonatedBy *string) (*entity.AuthResponse, error) {
	// Empty familyID means create a new family, mustChangePassword defaults to false
	return s.createAuthResponseWithFamilyAndPasswordFlag(ctx, user, orgID, "", isImpersonation, impersonatedBy, false)
}

// createAuthResponseWithPasswordFlag creates a new auth response with password change flag
// Used for login when we need to flag users with weak passwords
func (s *AuthService) createAuthResponseWithPasswordFlag(ctx context.Context, user *entity.User, orgID string, isImpersonation bool, impersonatedBy *string, mustChangePassword bool) (*entity.AuthResponse, error) {
	// Empty familyID means create a new family
	return s.createAuthResponseWithFamilyAndPasswordFlag(ctx, user, orgID, "", isImpersonation, impersonatedBy, mustChangePassword)
}

// createAuthResponseWithFamily creates an auth response with an explicit family ID
// If familyID is empty, a new family is created (for login/register/org switch)
// If familyID is provided, it's used for token rotation within the same family
func (s *AuthService) createAuthResponseWithFamily(ctx context.Context, user *entity.User, orgID string, familyID string, isImpersonation bool, impersonatedBy *string) (*entity.AuthResponse, error) {
	// Default mustChangePassword to false for backward compatibility
	return s.createAuthResponseWithFamilyAndPasswordFlag(ctx, user, orgID, familyID, isImpersonation, impersonatedBy, false)
}

// createAuthResponseWithFamilyAndPasswordFlag creates an auth response with an explicit family ID and password change flag
// If familyID is empty, a new family is created (for login/register/org switch)
// If familyID is provided, it's used for token rotation within the same family
func (s *AuthService) createAuthResponseWithFamilyAndPasswordFlag(ctx context.Context, user *entity.User, orgID string, familyID string, isImpersonation bool, impersonatedBy *string, mustChangePassword bool) (*entity.AuthResponse, error) {
	// Get organization and verify it exists and is active
	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("getting organization: %w", err)
	}
	if !org.IsActive {
		return nil, ErrOrgInactive
	}

	// Get membership for role
	var role string
	if isImpersonation {
		// Platform admins impersonating get admin role in the target org
		// This allows them to access admin pages, but isPlatformAdmin=false
		// prevents them from seeing platform-wide data like org counts
		role = entity.RoleAdmin
	} else {
		membership, err := s.repo.GetMembership(ctx, user.ID, orgID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("getting membership: %w", err)
		}
		if membership != nil {
			role = membership.Role
		} else {
			role = entity.RoleUser
		}
	}

	// Generate access token
	expiresAt := time.Now().Add(s.config.AccessTokenExpiry)
	accessToken, err := s.generateAccessToken(user, orgID, role, isImpersonation, impersonatedBy, mustChangePassword, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := s.generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	// Store session with hashed refresh token and family ID
	refreshTokenHash := s.hashToken(refreshToken)
	refreshExpiresAt := time.Now().Add(s.config.RefreshTokenExpiry)
	_, err = s.repo.CreateSessionWithFamily(ctx, user.ID, orgID, refreshTokenHash, familyID, "", "", isImpersonation, impersonatedBy, refreshExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	// Get user's memberships
	memberships, err := s.repo.GetUserMemberships(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("getting memberships: %w", err)
	}

	// When impersonating, ensure the impersonated org appears in memberships
	// and is marked as the current (default) org
	if isImpersonation {
		// Check if user already has this org in their memberships
		hasOrg := false
		for i := range memberships {
			if memberships[i].OrgID == orgID {
				hasOrg = true
				// Mark this as the default for the impersonation session
				memberships[i].IsDefault = true
			} else {
				// Clear default from other orgs
				memberships[i].IsDefault = false
			}
		}

		// If user doesn't have this org, add a virtual membership
		if !hasOrg {
			virtualMembership := entity.MembershipWithOrg{
				Membership: entity.Membership{
					ID:        "impersonation",
					UserID:    user.ID,
					OrgID:     orgID,
					Role:      role,
					IsDefault: true,
					JoinedAt:  time.Now(),
				},
				OrgName: org.Name,
				OrgSlug: org.Slug,
			}
			// Prepend the impersonated org so it appears first
			memberships = append([]entity.MembershipWithOrg{virtualMembership}, memberships...)
		}
	}

	return &entity.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User: entity.UserWithOrgs{
			User:        *user,
			Memberships: memberships,
		},
	}, nil
}

func (s *AuthService) generateAccessToken(user *entity.User, orgID, role string, isImpersonation bool, impersonatedBy *string, mustChangePassword bool, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"userId":             user.ID,
		"orgId":              orgID,
		"email":              user.Email,
		"role":               role,
		"isPlatformAdmin":    user.IsPlatformAdmin,
		"isImpersonation":    isImpersonation,
		"mustChangePassword": mustChangePassword,
		"exp":                expiresAt.Unix(),
		"iat":                time.Now().Unix(),
	}

	if impersonatedBy != nil {
		claims["impersonatedBy"] = *impersonatedBy
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *AuthService) hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.config.BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CreateOrganization creates a new organization (for platform admin use)
// It generates a unique slug, provisions a tenant database, and creates default metadata.
func (s *AuthService) CreateOrganization(ctx context.Context, input entity.OrganizationCreateInput) (*entity.Organization, error) {
	if input.Name == "" {
		return nil, errors.New("organization name is required")
	}

	slug := input.Slug
	if slug == "" {
		slug = s.generateSlug(input.Name)
	}

	slug, err := s.ensureUniqueSlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("generating slug: %w", err)
	}
	input.Slug = slug

	// Get current platform version for new org
	platformVersion := "v0.1.0" // Default fallback
	if s.versionRepo != nil {
		pv, err := s.versionRepo.GetPlatformVersion(ctx)
		if err != nil {
			log.Printf("[CreateOrganization] Warning: failed to get platform version: %v, using default", err)
		} else {
			platformVersion = pv.Version
			log.Printf("[CreateOrganization] New org will be created at platform version %s", platformVersion)
		}
	}
	input.CurrentVersion = platformVersion

	// Create organization record
	org, err := s.repo.CreateOrganization(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}

	// Provision tenant database and metadata
	if s.tenantProvisioning != nil {
		log.Printf("[TenantProvisioning] Starting provisioning for org %s (slug: %s)", org.ID, slug)
		tenantDB, err := s.tenantProvisioning.ProvisionTenant(ctx, org.ID, slug)
		if err != nil {
			log.Printf("[TenantProvisioning] ERROR: Failed to provision tenant database for org %s: %v", org.ID, err)
			// Store error in org metadata for debugging
			org.ProvisioningError = err.Error()
		} else if tenantDB.URL != "" {
			log.Printf("[TenantProvisioning] Database created: %s for org %s", tenantDB.Name, org.ID)
			// Update org with database credentials
			if err := s.repo.UpdateOrganizationDatabase(ctx, org.ID, tenantDB.URL, tenantDB.Token, tenantDB.Name); err != nil {
				log.Printf("[TenantProvisioning] ERROR: Failed to save database credentials for org %s: %v", org.ID, err)
				org.ProvisioningError = "DB created but failed to save credentials: " + err.Error()
			}
			// Update the returned org object
			org.DatabaseURL = tenantDB.URL
			org.DatabaseToken = tenantDB.Token
			org.DatabaseName = tenantDB.Name
			log.Printf("[TenantProvisioning] SUCCESS: Org %s provisioned with database %s", org.ID, tenantDB.Name)
		} else {
			log.Printf("[TenantProvisioning] Local mode: org %s using shared database", org.ID)
		}
	} else if s.provisioning != nil {
		// Legacy path: provision metadata to shared DB
		if err := s.provisioning.ProvisionDefaultMetadata(ctx, org.ID); err != nil {
			log.Printf("[Provisioning] ERROR: Failed to provision default metadata for org %s: %v", org.ID, err)
		}
	}

	return org, nil
}

func (s *AuthService) verifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *AuthService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (s *AuthService) generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func (s *AuthService) generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and special chars with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
	}

	return slug
}

func (s *AuthService) ensureUniqueSlug(ctx context.Context, baseSlug string) (string, error) {
	slug := baseSlug
	counter := 1

	for {
		_, err := s.repo.GetOrganizationBySlug(ctx, slug)
		if err == sql.ErrNoRows {
			return slug, nil
		}
		if err != nil {
			return "", err
		}

		// Slug exists, try with counter
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++

		if counter > 100 {
			return "", errors.New("unable to generate unique slug")
		}
	}
}

func (s *AuthService) validateEmail(email string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}
	return nil
}

func (s *AuthService) validatePassword(password string) error {
	// NIST SP 800-63B: Use character count, not byte count for Unicode support
	length := utf8.RuneCountInString(password)

	// NIST: Minimum 8 characters
	if length < 8 {
		return ErrPasswordTooShort
	}

	// NIST: Maximum 128 characters (allow long passphrases)
	if length > 128 {
		return ErrPasswordTooLong
	}

	// NIST: Check against blocklist of common passwords
	if data.IsCommonPassword(password) {
		return ErrPasswordCommon
	}

	return nil
}

// isPasswordWeak checks if a password fails current security requirements
// Used to flag existing users who need to update their password
func (s *AuthService) isPasswordWeak(password string) bool {
	// Length check (NIST: 8-128 characters)
	length := utf8.RuneCountInString(password)
	if length < 8 || length > 128 {
		return true
	}

	// Blocklist check
	if data.IsCommonPassword(password) {
		return true
	}

	return false
}

func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getBoolClaim(claims jwt.MapClaims, key string) bool {
	if val, ok := claims[key].(bool); ok {
		return val
	}
	return false
}

// --- User Retrieval (for handlers) ---

// GetCurrentUser returns the full current user details
func (s *AuthService) GetCurrentUser(ctx context.Context, userID, orgID string) (*entity.CurrentUser, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("getting organization: %w", err)
	}

	membership, err := s.repo.GetMembership(ctx, userID, orgID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting membership: %w", err)
	}

	role := entity.RoleUser
	if membership != nil {
		role = membership.Role
	}

	return &entity.CurrentUser{
		ID:              user.ID,
		Email:           user.Email,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		OrgID:           org.ID,
		OrgName:         org.Name,
		OrgSlug:         org.Slug,
		Role:            role,
		IsPlatformAdmin: user.IsPlatformAdmin,
	}, nil
}

// GetUserWithOrgs returns a user with all their organization memberships
func (s *AuthService) GetUserWithOrgs(ctx context.Context, userID string) (*entity.UserWithOrgs, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	memberships, err := s.repo.GetUserMemberships(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting memberships: %w", err)
	}

	return &entity.UserWithOrgs{
		User:        *user,
		Memberships: memberships,
	}, nil
}

// --- Session Timeout Management ---

// GetSessionWithTimeouts retrieves an active session with timeout configuration
func (s *AuthService) GetSessionWithTimeouts(ctx context.Context, userID, orgID string) (*entity.Session, error) {
	session, err := s.repo.GetSessionByUserAndOrg(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	return session, nil
}

// UpdateSessionActivity updates the last activity timestamp for a session
func (s *AuthService) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	return s.repo.UpdateLastActivity(ctx, sessionID)
}

// ExtendSession explicitly extends a session by updating last_activity_at
// This is called when the user clicks "Stay logged in" in the session warning
func (s *AuthService) ExtendSession(ctx context.Context, userID, orgID string) error {
	session, err := s.repo.GetSessionByUserAndOrg(ctx, userID, orgID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	return s.repo.UpdateLastActivity(ctx, session.ID)
}
