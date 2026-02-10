package entity

import "time"

// Role constants for organization membership
const (
	RoleOwner = "owner"
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// IsAdminRole returns true if the role has admin privileges (owner or admin)
func IsAdminRole(role string) bool {
	return role == RoleOwner || role == RoleAdmin
}

// IsOwnerRole returns true if the role is owner
func IsOwnerRole(role string) bool {
	return role == RoleOwner
}

// Membership represents a user's membership in an organization
type Membership struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"userId" db:"user_id"`
	OrgID     string    `json:"orgId" db:"org_id"`
	Role      string    `json:"role" db:"role"`
	IsDefault bool      `json:"isDefault" db:"is_default"`
	JoinedAt  time.Time `json:"joinedAt" db:"joined_at"`
}

// MembershipWithOrg includes the organization details
type MembershipWithOrg struct {
	Membership
	OrgName string `json:"orgName"`
	OrgSlug string `json:"orgSlug"`
}

// MembershipWithUser includes the user details
type MembershipWithUser struct {
	Membership
	UserEmail     string `json:"userEmail"`
	UserFirstName string `json:"userFirstName"`
	UserLastName  string `json:"userLastName"`
}

// InvitationInput represents input for inviting a user to an organization
type InvitationInput struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role"`
}

// Invitation represents a pending organization invitation
type Invitation struct {
	ID         string     `json:"id" db:"id"`
	OrgID      string     `json:"orgId" db:"org_id"`
	Email      string     `json:"email" db:"email"`
	Role       string     `json:"role" db:"role"`
	Token      string     `json:"-" db:"token"`
	InvitedBy  string     `json:"invitedBy" db:"invited_by"`
	ExpiresAt  time.Time  `json:"expiresAt" db:"expires_at"`
	AcceptedAt *time.Time `json:"acceptedAt" db:"accepted_at"`
	CreatedAt  time.Time  `json:"createdAt" db:"created_at"`
}

// InvitationWithDetails includes organization and inviter details
type InvitationWithDetails struct {
	Invitation
	Token           string `json:"token"` // Shadow embedded field to include in JSON
	OrgName         string `json:"orgName"`
	InviterName     string `json:"inviterName"`
	InviterEmail    string `json:"inviterEmail"`
}

// AcceptInvitationInput represents input for accepting an invitation
type AcceptInvitationInput struct {
	Token     string `json:"token" validate:"required"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}
