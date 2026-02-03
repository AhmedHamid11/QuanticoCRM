package entity

import "time"

// APIToken represents a bearer token for API access
type APIToken struct {
	ID          string     `json:"id" db:"id"`
	OrgID       string     `json:"orgId" db:"org_id"`
	CreatedBy   string     `json:"createdBy" db:"created_by"`
	Name        string     `json:"name" db:"name"`
	TokenHash   string     `json:"-" db:"token_hash"` // Never expose hash
	TokenPrefix string     `json:"tokenPrefix" db:"token_prefix"`
	Scopes      []string   `json:"scopes" db:"scopes"`
	LastUsedAt  *time.Time `json:"lastUsedAt" db:"last_used_at"`
	ExpiresAt   *time.Time `json:"expiresAt" db:"expires_at"`
	IsActive    bool       `json:"isActive" db:"is_active"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
}

// APITokenCreateInput represents input for creating a new API token
type APITokenCreateInput struct {
	Name      string   `json:"name" validate:"required,min=1,max=100"`
	Scopes    []string `json:"scopes"`           // Defaults to ["read", "write"] if empty
	ExpiresIn *int     `json:"expiresIn"`        // Days until expiration, nil = never
}

// APITokenCreateResponse includes the full token (only shown once at creation)
type APITokenCreateResponse struct {
	Token    string   `json:"token"`    // Full token (only shown once!)
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Scopes   []string `json:"scopes"`
	ExpiresAt *time.Time `json:"expiresAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

// APITokenListItem represents a token in list view (no sensitive data)
type APITokenListItem struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"tokenPrefix"`
	Scopes      []string   `json:"scopes"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	IsActive    bool       `json:"isActive"`
	CreatedAt   time.Time  `json:"createdAt"`
	CreatedBy   string     `json:"createdBy"`
}

// APITokenClaims represents the validated claims from an API token
// Used similarly to TokenClaims for JWT
type APITokenClaims struct {
	TokenID string   `json:"tokenId"`
	OrgID   string   `json:"orgId"`
	Scopes  []string `json:"scopes"`
}

// Scope constants
const (
	ScopeRead  = "read"
	ScopeWrite = "write"
)

// ValidScopes returns all valid scope values
func ValidScopes() []string {
	return []string{ScopeRead, ScopeWrite}
}

// HasScope checks if the token has a specific scope
func (c *APITokenClaims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}
