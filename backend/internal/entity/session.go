package entity

import "time"

// Session represents an active user session
type Session struct {
	ID               string    `json:"id" db:"id"`
	UserID           string    `json:"userId" db:"user_id"`
	OrgID            string    `json:"orgId" db:"org_id"`
	RefreshTokenHash string    `json:"-" db:"refresh_token_hash"`
	UserAgent        string    `json:"userAgent" db:"user_agent"`
	IPAddress        string    `json:"ipAddress" db:"ip_address"`
	IsImpersonation  bool      `json:"isImpersonation" db:"is_impersonation"`
	ImpersonatedBy   *string   `json:"impersonatedBy" db:"impersonated_by"`
	ExpiresAt        time.Time `json:"expiresAt" db:"expires_at"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
}

// AuthResponse represents the response after successful authentication
type AuthResponse struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	ExpiresAt    time.Time    `json:"expiresAt"`
	User         UserWithOrgs `json:"user"`
}

// RefreshInput represents input for refreshing tokens
type RefreshInput struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

// SwitchOrgInput represents input for switching active organization
type SwitchOrgInput struct {
	OrgID string `json:"orgId" validate:"required"`
}

// ImpersonateInput represents input for admin impersonation
type ImpersonateInput struct {
	OrgID  string `json:"orgId" validate:"required"`
	UserID string `json:"userId"` // Optional - if not provided, impersonate as org owner
}

// TokenClaims represents the JWT token claims
type TokenClaims struct {
	UserID          string `json:"userId"`
	OrgID           string `json:"orgId"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	IsPlatformAdmin bool   `json:"isPlatformAdmin"`
	IsImpersonation bool   `json:"isImpersonation"`
	ImpersonatedBy  string `json:"impersonatedBy,omitempty"`
}

// CurrentUser represents the authenticated user context
type CurrentUser struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	OrgID           string `json:"orgId"`
	OrgName         string `json:"orgName"`
	OrgSlug         string `json:"orgSlug"`
	Role            string `json:"role"`
	IsPlatformAdmin bool   `json:"isPlatformAdmin"`
	IsImpersonation bool   `json:"isImpersonation"`
	ImpersonatedBy  string `json:"impersonatedBy,omitempty"`
}
