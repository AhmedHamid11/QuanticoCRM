package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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

// ============================================================
// Sentinel errors
// ============================================================

var (
	ErrGmailNotConnected     = errors.New("gmail: not connected")
	ErrGmailNoTokens         = errors.New("gmail: no access token")
	ErrGmailInvalidState     = errors.New("gmail: invalid oauth state parameter")
	ErrGmailMissingClientCfg = errors.New("GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET not configured")
)

// ============================================================
// GmailConnectionStatus is the response shape for GET /gmail/status
// ============================================================

// GmailConnectionStatus describes the current state of a user's Gmail connection.
type GmailConnectionStatus struct {
	Connected    bool       `json:"connected"`
	GmailAddress string     `json:"gmailAddress,omitempty"`
	ConnectedAt  *time.Time `json:"connectedAt,omitempty"`
	DNSSPFValid  int        `json:"dnsspfValid"`
	DNSDKIMValid int        `json:"dnsdkimValid"`
	DNSDMARCValid int       `json:"dnsdmarcValid"`
}

// ============================================================
// GmailOAuthService manages Gmail OAuth and token lifecycle
// ============================================================

// GmailOAuthService mirrors GoogleCalendarService exactly, adapted for Gmail scopes
// and using EngagementRepo instead of SchedulingRepo.
type GmailOAuthService struct {
	repo          *repo.EngagementRepo
	encryptionKey []byte
}

// NewGmailOAuthService creates a new GmailOAuthService.
func NewGmailOAuthService(r *repo.EngagementRepo, encryptionKey []byte) *GmailOAuthService {
	return &GmailOAuthService{
		repo:          r,
		encryptionKey: encryptionKey,
	}
}

// GetEncryptionKey returns the encryption key (for creating tenant-scoped instances).
func (s *GmailOAuthService) GetEncryptionKey() []byte {
	return s.encryptionKey
}

// ============================================================
// OAuth config
// ============================================================

func (s *GmailOAuthService) buildOAuth2Config(redirectBase string) (*oauth2.Config, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return nil, ErrGmailMissingClientCfg
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectBase + "/api/v1/gmail/callback",
		Scopes: []string{
			"https://www.googleapis.com/auth/gmail.send",
			"https://www.googleapis.com/auth/gmail.metadata",
		},
		Endpoint: googleOAuth2Endpoint, // Reuse the same endpoint constant from google_calendar.go
	}, nil
}

// ============================================================
// State encoding / decoding (CSRF protection)
// ============================================================

// encodeState encodes orgID + userID + random bytes as a base64 state token.
// The format is: base64(orgID + ":" + userID + ":" + base64(randomBytes))
func (s *GmailOAuthService) encodeState(orgID, userID string) (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("gmail: failed to generate state token: %w", err)
	}
	stateData := orgID + ":" + userID + ":" + base64.URLEncoding.EncodeToString(randomBytes)
	return base64.URLEncoding.EncodeToString([]byte(stateData)), nil
}

// decodeState extracts orgID and userID from the OAuth state token.
func (s *GmailOAuthService) decodeState(state string) (orgID, userID string, err error) {
	decoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return "", "", ErrGmailInvalidState
	}
	parts := strings.SplitN(string(decoded), ":", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ErrGmailInvalidState
	}
	return parts[0], parts[1], nil
}

// ============================================================
// Public API
// ============================================================

// GetAuthorizationURL builds the Google OAuth authorization URL for Gmail.
func (s *GmailOAuthService) GetAuthorizationURL(ctx context.Context, orgID, userID, redirectBase string) (string, error) {
	if s.encryptionKey == nil {
		return "", ErrMissingEncryptionKey
	}

	oauth2Config, err := s.buildOAuth2Config(redirectBase)
	if err != nil {
		return "", err
	}

	state, err := s.encodeState(orgID, userID)
	if err != nil {
		return "", err
	}

	authURL := oauth2Config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	log.Printf("[GmailOAuth] Generated auth URL for user %s in org %s", userID, orgID)
	return authURL, nil
}

// HandleCallback exchanges the OAuth authorization code for tokens, fetches the
// Gmail address from Google's userinfo endpoint, runs DNS validation, and persists
// the encrypted token record in the tenant DB.
// Returns orgID, userID, error.
func (s *GmailOAuthService) HandleCallback(ctx context.Context, code, state, redirectBase string) (string, string, error) {
	if s.encryptionKey == nil {
		return "", "", ErrMissingEncryptionKey
	}

	orgID, userID, err := s.decodeState(state)
	if err != nil {
		return "", "", err
	}

	oauth2Config, err := s.buildOAuth2Config(redirectBase)
	if err != nil {
		return "", "", err
	}

	// Exchange code for tokens
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		log.Printf("[GmailOAuth] Failed to exchange code for org %s user %s: %v", orgID, userID, err)
		return "", "", fmt.Errorf("gmail: failed to exchange authorization code: %w", err)
	}

	// Encrypt access token
	encryptedAccess, err := util.EncryptToken(token.AccessToken, s.encryptionKey)
	if err != nil {
		return "", "", fmt.Errorf("gmail: failed to encrypt access token: %w", err)
	}

	// Encrypt refresh token (may be absent on repeat grants, but prompt=consent forces it)
	var encryptedRefresh []byte
	if token.RefreshToken != "" {
		encryptedRefresh, err = util.EncryptToken(token.RefreshToken, s.encryptionKey)
		if err != nil {
			return "", "", fmt.Errorf("gmail: failed to encrypt refresh token: %w", err)
		}
	}

	// Fetch the authenticated Gmail address from Google's userinfo endpoint
	httpClient := oauth2Config.Client(ctx, token)
	gmailAddress, err := s.fetchGmailAddress(ctx, httpClient)
	if err != nil {
		log.Printf("[GmailOAuth] Warning: failed to fetch gmail address for org %s user %s: %v", orgID, userID, err)
		gmailAddress = "" // Non-fatal; store empty and continue
	}

	// DNS validation — runs synchronously, errors are non-fatal (stored as 0)
	spf, dkim, dmarc := s.validateDNS(ctx, gmailAddress)

	now := time.Now()
	tok := &entity.GmailOAuthToken{
		ID:                    uuid.New().String(),
		OrgID:                 orgID,
		UserID:                userID,
		AccessTokenEncrypted:  encryptedAccess,
		RefreshTokenEncrypted: encryptedRefresh,
		GmailAddress:          gmailAddress,
		DNSSPFValid:           spf,
		DNSDKIMValid:          dkim,
		DNSDMARCValid:         dmarc,
		ConnectedAt:           &now,
	}
	if !token.Expiry.IsZero() {
		tok.TokenExpiry = &token.Expiry
	}

	if err := s.repo.UpsertGmailOAuthToken(ctx, tok); err != nil {
		log.Printf("[GmailOAuth] Failed to store token for org %s user %s: %v", orgID, userID, err)
		return "", "", fmt.Errorf("gmail: failed to store token: %w", err)
	}

	log.Printf("[GmailOAuth] Token stored for user %s in org %s (address=%s, spf=%d, dkim=%d, dmarc=%d)",
		userID, orgID, gmailAddress, spf, dkim, dmarc)
	return orgID, userID, nil
}

// GetConnectionStatus returns the connection status for a user's Gmail account.
func (s *GmailOAuthService) GetConnectionStatus(ctx context.Context, orgID, userID string) (GmailConnectionStatus, error) {
	tok, err := s.repo.GetGmailOAuthToken(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GmailConnectionStatus{Connected: false}, nil
		}
		return GmailConnectionStatus{}, fmt.Errorf("gmail: failed to get token: %w", err)
	}
	if tok == nil {
		return GmailConnectionStatus{Connected: false}, nil
	}
	if tok.AccessTokenEncrypted == nil {
		return GmailConnectionStatus{Connected: false}, nil
	}

	return GmailConnectionStatus{
		Connected:     true,
		GmailAddress:  tok.GmailAddress,
		ConnectedAt:   tok.ConnectedAt,
		DNSSPFValid:   tok.DNSSPFValid,
		DNSDKIMValid:  tok.DNSDKIMValid,
		DNSDMARCValid: tok.DNSDMARCValid,
	}, nil
}

// Disconnect removes the Gmail OAuth token for a user within an org.
func (s *GmailOAuthService) Disconnect(ctx context.Context, orgID, userID string) error {
	if err := s.repo.DeleteGmailOAuthToken(ctx, orgID, userID); err != nil {
		return fmt.Errorf("gmail: failed to disconnect: %w", err)
	}
	log.Printf("[GmailOAuth] Disconnected user %s in org %s", userID, orgID)
	return nil
}

// GetHTTPClient returns an authenticated HTTP client for Gmail API calls.
// The oauth2 library auto-refreshes the access token using the stored refresh token.
func (s *GmailOAuthService) GetHTTPClient(ctx context.Context, orgID, userID string) (*http.Client, *entity.GmailOAuthToken, error) {
	if s.encryptionKey == nil {
		return nil, nil, ErrMissingEncryptionKey
	}

	tok, err := s.repo.GetGmailOAuthToken(ctx, orgID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("gmail: failed to load token: %w", err)
	}
	if tok == nil {
		return nil, nil, ErrGmailNotConnected
	}
	if tok.AccessTokenEncrypted == nil {
		return nil, nil, ErrGmailNoTokens
	}

	accessToken, err := util.DecryptToken(tok.AccessTokenEncrypted, s.encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("gmail: failed to decrypt access token: %w", err)
	}

	var refreshToken string
	if tok.RefreshTokenEncrypted != nil {
		refreshToken, _ = util.DecryptToken(tok.RefreshTokenEncrypted, s.encryptionKey)
	}

	oauthToken := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	}
	if tok.TokenExpiry != nil {
		oauthToken.Expiry = *tok.TokenExpiry
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     googleOAuth2Endpoint,
	}

	client := cfg.Client(ctx, oauthToken)
	return client, tok, nil
}

// ============================================================
// Internal helpers
// ============================================================

// fetchGmailAddress retrieves the authenticated user's email via Google's userinfo API.
func (s *GmailOAuthService) fetchGmailAddress(ctx context.Context, client *http.Client) (string, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return "", fmt.Errorf("gmail: userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gmail: userinfo returned %d: %s", resp.StatusCode, body)
	}

	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("gmail: failed to decode userinfo: %w", err)
	}
	return info.Email, nil
}

// validateDNS checks SPF, DKIM, and DMARC DNS records for the domain of the
// given Gmail address. All errors are handled gracefully — a DNS failure results
// in 0 (unknown) rather than blocking the OAuth callback.
//
// Returns (spf, dkim, dmarc) where 1 = found / 0 = not found or error.
func (s *GmailOAuthService) validateDNS(ctx context.Context, gmailAddress string) (spf, dkim, dmarc int) {
	if gmailAddress == "" {
		return 0, 0, 0
	}

	atIdx := strings.LastIndex(gmailAddress, "@")
	if atIdx < 0 || atIdx == len(gmailAddress)-1 {
		return 0, 0, 0
	}
	domain := gmailAddress[atIdx+1:]

	// SPF: TXT record on domain starting with "v=spf1"
	records, err := net.LookupTXT(domain)
	if err == nil {
		for _, r := range records {
			if strings.HasPrefix(r, "v=spf1") {
				spf = 1
				break
			}
		}
	}

	// DMARC: TXT record on _dmarc.<domain> starting with "v=DMARC1"
	records, err = net.LookupTXT("_dmarc." + domain)
	if err == nil {
		for _, r := range records {
			if strings.HasPrefix(r, "v=DMARC1") {
				dmarc = 1
				break
			}
		}
	}

	// DKIM: check "google._domainkey.<domain>" (covers @gmail.com and Google Workspace).
	// If the DKIM selector is not "google", this returns 0 (unknown). The UI shows
	// "Unknown" rather than "Fail" to avoid false alarms for custom selectors.
	records, err = net.LookupTXT("google._domainkey." + domain)
	if err == nil {
		for _, r := range records {
			if strings.Contains(r, "v=DKIM1") {
				dkim = 1
				break
			}
		}
	}

	return spf, dkim, dmarc
}
