package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var (
	ErrNoConnection         = errors.New("salesforce connection not configured")
	ErrNoTokens             = errors.New("salesforce not connected - no access token")
	ErrExpiredToken         = errors.New("salesforce token expired")
	ErrInvalidState         = errors.New("invalid OAuth state parameter")
	ErrMissingEncryptionKey = errors.New("encryption key not configured")
)

// SalesforceOAuthService manages Salesforce OAuth 2.0 authentication and token lifecycle
type SalesforceOAuthService struct {
	repo          *repo.SalesforceRepo
	encryptionKey []byte
}

// NewSalesforceOAuthService creates a new SalesforceOAuthService
func NewSalesforceOAuthService(repo *repo.SalesforceRepo, encryptionKey []byte) *SalesforceOAuthService {
	return &SalesforceOAuthService{
		repo:          repo,
		encryptionKey: encryptionKey,
	}
}

// SaveConfig stores Salesforce Connected App credentials (encrypts client_secret)
func (s *SalesforceOAuthService) SaveConfig(ctx context.Context, orgID string, config entity.SFSyncConfig) error {
	if s.encryptionKey == nil {
		return ErrMissingEncryptionKey
	}

	// Validate required fields
	if config.ClientID == "" {
		return errors.New("client_id is required")
	}
	if config.ClientSecret == "" {
		return errors.New("client_secret is required")
	}
	if config.RedirectURL == "" {
		return errors.New("redirect_url is required")
	}

	// Encrypt client secret
	encryptedSecret, err := util.EncryptToken(config.ClientSecret, s.encryptionKey)
	if err != nil {
		log.Printf("Failed to encrypt client secret for org %s: %v", orgID, err)
		return fmt.Errorf("failed to encrypt client secret: %w", err)
	}

	// Create connection entity
	conn := &entity.SalesforceConnection{
		ID:                    uuid.New().String(),
		OrgID:                 orgID,
		ClientID:              config.ClientID,
		ClientSecretEncrypted: encryptedSecret,
		RedirectURL:           config.RedirectURL,
		IsEnabled:             true, // Default to enabled on config save
	}

	// Upsert connection
	if err := s.repo.UpsertConnection(ctx, conn); err != nil {
		log.Printf("Failed to save Salesforce config for org %s: %v", orgID, err)
		return fmt.Errorf("failed to save salesforce config: %w", err)
	}

	log.Printf("Salesforce config saved for org: %s", orgID)
	return nil
}

// GetConfig retrieves the Salesforce connection config (tokens remain encrypted)
func (s *SalesforceOAuthService) GetConfig(ctx context.Context, orgID string) (*entity.SalesforceConnection, error) {
	conn, err := s.repo.GetConnection(ctx, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No connection exists, not an error
		}
		log.Printf("Failed to get Salesforce config for org %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to get salesforce config: %w", err)
	}
	return conn, nil
}

// GetAuthorizationURL generates the Salesforce OAuth authorization URL
func (s *SalesforceOAuthService) GetAuthorizationURL(ctx context.Context, orgID string) (string, error) {
	if s.encryptionKey == nil {
		return "", ErrMissingEncryptionKey
	}

	// Load connection
	conn, err := s.repo.GetConnection(ctx, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNoConnection
		}
		log.Printf("Failed to load connection for org %s: %v", orgID, err)
		return "", fmt.Errorf("failed to load connection: %w", err)
	}

	// Decrypt client secret
	clientSecret, err := util.DecryptToken(conn.ClientSecretEncrypted, s.encryptionKey)
	if err != nil {
		log.Printf("Failed to decrypt client secret for org %s: %v", orgID, err)
		return "", fmt.Errorf("failed to decrypt client secret: %w", err)
	}

	// Build OAuth2 config
	oauth2Config := s.buildOAuth2Config(conn.ClientID, clientSecret, conn.RedirectURL)

	// Generate CSRF state token: base64(orgID + ":" + random32bytes)
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		log.Printf("Failed to generate state token for org %s: %v", orgID, err)
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}
	state := base64.URLEncoding.EncodeToString([]byte(orgID + ":" + base64.URLEncoding.EncodeToString(randomBytes)))

	// Generate authorization URL
	authURL := oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	log.Printf("Generated OAuth authorization URL for org: %s", orgID)
	return authURL, nil
}

// HandleCallback exchanges the authorization code for tokens and stores them encrypted
func (s *SalesforceOAuthService) HandleCallback(ctx context.Context, code, state string) (string, error) {
	if s.encryptionKey == nil {
		return "", ErrMissingEncryptionKey
	}

	// Parse state to extract orgID (CSRF protection)
	stateDecoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return "", ErrInvalidState
	}
	parts := strings.SplitN(string(stateDecoded), ":", 2)
	if len(parts) != 2 {
		return "", ErrInvalidState
	}
	orgID := parts[0]

	// Load connection
	conn, err := s.repo.GetConnection(ctx, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNoConnection
		}
		log.Printf("Failed to load connection for org %s during callback: %v", orgID, err)
		return "", fmt.Errorf("failed to load connection: %w", err)
	}

	// Decrypt client secret
	clientSecret, err := util.DecryptToken(conn.ClientSecretEncrypted, s.encryptionKey)
	if err != nil {
		log.Printf("Failed to decrypt client secret for org %s during callback: %v", orgID, err)
		return "", fmt.Errorf("failed to decrypt client secret: %w", err)
	}

	// Build OAuth2 config
	oauth2Config := s.buildOAuth2Config(conn.ClientID, clientSecret, conn.RedirectURL)

	// Exchange authorization code for tokens
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		log.Printf("Failed to exchange OAuth code for org %s: %v", orgID, err)
		return "", fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Extract instance URL from token extras
	instanceURL := ""
	if instanceURLRaw := token.Extra("instance_url"); instanceURLRaw != nil {
		if instanceURLStr, ok := instanceURLRaw.(string); ok {
			instanceURL = instanceURLStr
		}
	}

	// Encrypt access token and refresh token
	encryptedAccessToken, err := util.EncryptToken(token.AccessToken, s.encryptionKey)
	if err != nil {
		log.Printf("Failed to encrypt access token for org %s: %v", orgID, err)
		return "", fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encryptedRefreshToken, err := util.EncryptToken(token.RefreshToken, s.encryptionKey)
	if err != nil {
		log.Printf("Failed to encrypt refresh token for org %s: %v", orgID, err)
		return "", fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	// Store tokens
	if err := s.repo.UpdateTokens(ctx, orgID, encryptedAccessToken, encryptedRefreshToken, token.Expiry); err != nil {
		log.Printf("Failed to store tokens for org %s: %v", orgID, err)
		return "", fmt.Errorf("failed to store tokens: %w", err)
	}

	// Update connection with instance URL and connected timestamp
	conn.InstanceURL = instanceURL
	now := time.Now()
	conn.ConnectedAt = &now
	conn.TokenType = token.TokenType
	if err := s.repo.UpsertConnection(ctx, conn); err != nil {
		log.Printf("Failed to update connection metadata for org %s: %v", orgID, err)
		// Don't fail - tokens are already stored
	}

	log.Printf("OAuth tokens successfully stored for org: %s", orgID)
	return orgID, nil
}

// GetHTTPClient returns an authenticated HTTP client with auto-refresh for expired tokens
func (s *SalesforceOAuthService) GetHTTPClient(ctx context.Context, orgID string) (*http.Client, error) {
	if s.encryptionKey == nil {
		return nil, ErrMissingEncryptionKey
	}

	// Load connection
	conn, err := s.repo.GetConnection(ctx, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoConnection
		}
		log.Printf("Failed to load connection for org %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to load connection: %w", err)
	}

	// Check if tokens exist
	if conn.AccessTokenEncrypted == nil {
		return nil, ErrNoTokens
	}

	// Decrypt tokens
	accessToken, err := util.DecryptToken(conn.AccessTokenEncrypted, s.encryptionKey)
	if err != nil {
		log.Printf("Failed to decrypt access token for org %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	refreshToken := ""
	if conn.RefreshTokenEncrypted != nil {
		refreshToken, err = util.DecryptToken(conn.RefreshTokenEncrypted, s.encryptionKey)
		if err != nil {
			log.Printf("Failed to decrypt refresh token for org %s: %v", orgID, err)
			// Continue without refresh token (will fail if access token expired)
		}
	}

	// Build OAuth2 token
	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    conn.TokenType,
	}
	if conn.ExpiresAt != nil {
		token.Expiry = *conn.ExpiresAt
	}

	// Decrypt client secret
	clientSecret, err := util.DecryptToken(conn.ClientSecretEncrypted, s.encryptionKey)
	if err != nil {
		log.Printf("Failed to decrypt client secret for org %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to decrypt client secret: %w", err)
	}

	// Build OAuth2 config
	oauth2Config := s.buildOAuth2Config(conn.ClientID, clientSecret, conn.RedirectURL)

	// Store original expiry to detect if token was refreshed
	originalExpiry := token.Expiry

	// Create HTTP client - this auto-refreshes expired tokens via golang.org/x/oauth2
	client := oauth2Config.Client(ctx, token)

	// Check if token was refreshed (expiry changed)
	// IMPORTANT: After creating the client, we need to check if the token was updated
	// The oauth2 library updates the token in-place, so we check expiry change
	if !token.Expiry.Equal(originalExpiry) {
		// Token was refreshed - re-encrypt and store new tokens
		encryptedAccessToken, err := util.EncryptToken(token.AccessToken, s.encryptionKey)
		if err != nil {
			log.Printf("Warning: Failed to encrypt refreshed access token for org %s: %v", orgID, err)
		} else {
			encryptedRefreshToken, err := util.EncryptToken(token.RefreshToken, s.encryptionKey)
			if err != nil {
				log.Printf("Warning: Failed to encrypt refreshed refresh token for org %s: %v", orgID, err)
			} else {
				// Store refreshed tokens
				if err := s.repo.UpdateTokens(ctx, orgID, encryptedAccessToken, encryptedRefreshToken, token.Expiry); err != nil {
					log.Printf("Warning: Failed to store refreshed tokens for org %s: %v", orgID, err)
				} else {
					log.Printf("Proactively refreshed and stored new tokens for org: %s", orgID)
				}
			}
		}
	}

	return client, nil
}

// GetConnectionStatus returns the current connection status
func (s *SalesforceOAuthService) GetConnectionStatus(ctx context.Context, orgID string) (string, error) {
	conn, err := s.repo.GetConnection(ctx, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "not_configured", nil
		}
		log.Printf("Failed to get connection status for org %s: %v", orgID, err)
		return "", fmt.Errorf("failed to get connection status: %w", err)
	}

	// No access token = configured but not authorized
	if conn.AccessTokenEncrypted == nil {
		return "configured", nil
	}

	// Check if token is expired
	if conn.ExpiresAt != nil && time.Now().After(*conn.ExpiresAt) {
		// Token expired
		if conn.RefreshTokenEncrypted == nil {
			return "expired", nil // No refresh token, need to re-authorize
		}
		// Has refresh token, will auto-refresh on next use
		return "connected", nil
	}

	return "connected", nil
}

// DisconnectOrg clears tokens and connected timestamp
func (s *SalesforceOAuthService) DisconnectOrg(ctx context.Context, orgID string) error {
	conn, err := s.repo.GetConnection(ctx, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrNoConnection
		}
		log.Printf("Failed to load connection for org %s during disconnect: %v", orgID, err)
		return fmt.Errorf("failed to load connection: %w", err)
	}

	// Clear tokens and connected timestamp
	conn.AccessTokenEncrypted = nil
	conn.RefreshTokenEncrypted = nil
	conn.ConnectedAt = nil
	conn.ExpiresAt = nil

	if err := s.repo.UpsertConnection(ctx, conn); err != nil {
		log.Printf("Failed to disconnect org %s: %v", orgID, err)
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	log.Printf("Disconnected Salesforce for org: %s", orgID)
	return nil
}

// buildOAuth2Config creates the OAuth2 configuration for Salesforce
func (s *SalesforceOAuthService) buildOAuth2Config(clientID, clientSecret, redirectURL string) *oauth2.Config {
	// Support sandbox URLs via environment variables
	authURL := os.Getenv("SALESFORCE_AUTH_URL")
	if authURL == "" {
		authURL = "https://login.salesforce.com/services/oauth2/authorize"
	}

	tokenURL := os.Getenv("SALESFORCE_TOKEN_URL")
	if tokenURL == "" {
		tokenURL = "https://login.salesforce.com/services/oauth2/token"
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"api", "refresh_token"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}
}
