package entity

import "time"

// User represents an authenticated user
type User struct {
	ID              string     `json:"id" db:"id"`
	Email           string     `json:"email" db:"email"`
	PasswordHash    string     `json:"-" db:"password_hash"`
	FirstName       string     `json:"firstName" db:"first_name"`
	LastName        string     `json:"lastName" db:"last_name"`
	IsActive        bool       `json:"isActive" db:"is_active"`
	IsPlatformAdmin bool       `json:"isPlatformAdmin" db:"is_platform_admin"`
	EmailVerified   bool       `json:"emailVerified" db:"email_verified"`
	LastLoginAt     *time.Time `json:"lastLoginAt" db:"last_login_at"`
	CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
	ModifiedAt      time.Time  `json:"modifiedAt" db:"modified_at"`
}

// Name returns the full name of the user
func (u *User) Name() string {
	if u.FirstName == "" {
		return u.LastName
	}
	if u.LastName == "" {
		return u.FirstName
	}
	return u.FirstName + " " + u.LastName
}

// UserWithOrgs represents a user with their organization memberships
type UserWithOrgs struct {
	User
	Memberships []MembershipWithOrg `json:"memberships"`
}

// RegisterInput represents input for user registration
type RegisterInput struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	OrgName   string `json:"orgName" validate:"required"`
}

// LoginInput represents input for user login
type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// UserUpdateInput represents input for updating a user
type UserUpdateInput struct {
	FirstName     *string `json:"firstName"`
	LastName      *string `json:"lastName"`
	IsActive      *bool   `json:"isActive"`
	EmailVerified *bool   `json:"emailVerified"`
}

// ChangePasswordInput represents input for changing password
type ChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8"`
}

// UserListParams represents query parameters for listing users
type UserListParams struct {
	OrgID    string `query:"orgId"`
	Search   string `query:"search"`
	SortBy   string `query:"sortBy"`
	SortDir  string `query:"sortDir"`
	Page     int    `query:"page"`
	PageSize int    `query:"pageSize"`
}

// UserListResponse represents the response for listing users
type UserListResponse struct {
	Data       []UserWithMembership `json:"data"`
	Total      int                  `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"pageSize"`
	TotalPages int                  `json:"totalPages"`
}

// UserWithMembership represents a user with their membership in a specific org
type UserWithMembership struct {
	User
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joinedAt"`
}

// ForgotPasswordInput represents input for requesting a password reset
type ForgotPasswordInput struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordInput represents input for resetting password with a token
type ResetPasswordInput struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"newPassword" validate:"required,min=8"`
}

// PasswordResetToken represents a password reset token
type PasswordResetToken struct {
	ID        string     `json:"id" db:"id"`
	UserID    string     `json:"userId" db:"user_id"`
	TokenHash string     `json:"-" db:"token_hash"`
	ExpiresAt time.Time  `json:"expiresAt" db:"expires_at"`
	UsedAt    *time.Time `json:"usedAt" db:"used_at"`
	CreatedAt time.Time  `json:"createdAt" db:"created_at"`
}
