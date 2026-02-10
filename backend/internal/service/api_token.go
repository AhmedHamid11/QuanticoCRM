package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

var (
	ErrTokenNotFound     = errors.New("API token not found")
	ErrTokenExpired      = errors.New("API token has expired")
	ErrTokenRevoked      = errors.New("API token has been revoked")
	ErrInvalidTokenName  = errors.New("token name must be between 1 and 100 characters")
	ErrInvalidScope      = errors.New("invalid scope specified")
)

// APITokenService handles API token business logic
type APITokenService struct {
	repo *repo.APITokenRepo
}

// NewAPITokenService creates a new APITokenService
func NewAPITokenService(repo *repo.APITokenRepo) *APITokenService {
	return &APITokenService{repo: repo}
}

// Create generates a new API token for an organization
func (s *APITokenService) Create(ctx context.Context, orgID, createdBy string, input entity.APITokenCreateInput) (*entity.APITokenCreateResponse, error) {
	// Validate name
	if len(input.Name) == 0 || len(input.Name) > 100 {
		return nil, ErrInvalidTokenName
	}

	// Set default scopes if none provided
	scopes := input.Scopes
	if len(scopes) == 0 {
		scopes = []string{entity.ScopeRead, entity.ScopeWrite}
	}

	// Validate scopes
	validScopes := entity.ValidScopes()
	for _, scope := range scopes {
		isValid := false
		for _, v := range validScopes {
			if scope == v {
				isValid = true
				break
			}
		}
		if !isValid {
			return nil, fmt.Errorf("%w: %s", ErrInvalidScope, scope)
		}
	}

	// Generate the token: fcr_<random 32 bytes base64>
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}
	token := "fcr_" + hex.EncodeToString(tokenBytes)

	// Extract prefix for display (first 12 chars: fcr_xxxxxxxx)
	tokenPrefix := token[:12]

	// Hash the token for storage
	tokenHash := s.hashToken(token)

	// Calculate expiration
	var expiresAt *time.Time
	if input.ExpiresIn != nil && *input.ExpiresIn > 0 {
		exp := time.Now().UTC().AddDate(0, 0, *input.ExpiresIn)
		expiresAt = &exp
	}

	// Create the token in database
	apiToken, err := s.repo.Create(ctx, orgID, createdBy, input.Name, tokenHash, tokenPrefix, scopes, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("creating token: %w", err)
	}

	// Return response with the full token (only shown once!)
	return &entity.APITokenCreateResponse{
		Token:     token,
		ID:        apiToken.ID,
		Name:      apiToken.Name,
		Scopes:    apiToken.Scopes,
		ExpiresAt: apiToken.ExpiresAt,
		CreatedAt: apiToken.CreatedAt,
	}, nil
}

// ValidateToken validates an API token and returns its claims
func (s *APITokenService) ValidateToken(ctx context.Context, token string) (*entity.APITokenClaims, error) {
	// Verify token format
	if len(token) < 12 || token[:4] != "fcr_" {
		return nil, ErrTokenNotFound
	}

	// Hash the token
	tokenHash := s.hashToken(token)

	// Look up the token
	apiToken, err := s.repo.GetByHash(ctx, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("looking up token: %w", err)
	}

	// Check if active
	if !apiToken.IsActive {
		return nil, ErrTokenRevoked
	}

	// Check expiration (double-check even though query filters)
	if apiToken.ExpiresAt != nil && apiToken.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrTokenExpired
	}

	// Update last used timestamp (async, don't block on errors)
	go func() {
		_ = s.repo.UpdateLastUsed(context.Background(), apiToken.ID)
	}()

	return &entity.APITokenClaims{
		TokenID: apiToken.ID,
		OrgID:   apiToken.OrgID,
		Scopes:  apiToken.Scopes,
	}, nil
}

// List returns all tokens for an organization
func (s *APITokenService) List(ctx context.Context, orgID string) ([]entity.APITokenListItem, error) {
	tokens, err := s.repo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing tokens: %w", err)
	}
	if tokens == nil {
		tokens = []entity.APITokenListItem{}
	}
	return tokens, nil
}

// Revoke deactivates a token (keeps it in DB for audit trail)
func (s *APITokenService) Revoke(ctx context.Context, tokenID, orgID string) error {
	err := s.repo.Revoke(ctx, tokenID, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrTokenNotFound
		}
		return fmt.Errorf("revoking token: %w", err)
	}
	return nil
}

// Delete permanently removes a token
func (s *APITokenService) Delete(ctx context.Context, tokenID, orgID string) error {
	err := s.repo.Delete(ctx, tokenID, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrTokenNotFound
		}
		return fmt.Errorf("deleting token: %w", err)
	}
	return nil
}

// hashToken creates a SHA256 hash of the token
func (s *APITokenService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
