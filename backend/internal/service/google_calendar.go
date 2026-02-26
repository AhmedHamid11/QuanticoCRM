package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	ErrGoogleNotConnected     = errors.New("google calendar not connected")
	ErrGoogleNoTokens         = errors.New("google calendar: no access token")
	ErrGoogleInvalidState     = errors.New("invalid google oauth state parameter")
	ErrGoogleMissingClientCfg = errors.New("GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET not configured")
)

// googleOAuth2Endpoint defines Google's OAuth2 endpoints
var googleOAuth2Endpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// BusySlot represents a time period when the user is busy in Google Calendar
type BusySlot struct {
	Start time.Time
	End   time.Time
}

// CalendarEvent represents an event to create in Google Calendar
type CalendarEvent struct {
	Summary     string
	Description string
	StartTime   time.Time
	EndTime     time.Time
	GuestEmail  string
	GuestName   string
	Timezone    string
}

// GoogleCalendarService manages Google Calendar OAuth and API interactions
type GoogleCalendarService struct {
	repo          *repo.SchedulingRepo
	encryptionKey []byte
}

// NewGoogleCalendarService creates a new GoogleCalendarService
func NewGoogleCalendarService(r *repo.SchedulingRepo, encryptionKey []byte) *GoogleCalendarService {
	return &GoogleCalendarService{
		repo:          r,
		encryptionKey: encryptionKey,
	}
}

// buildOAuth2Config creates the OAuth2 configuration for Google Calendar
func (s *GoogleCalendarService) buildOAuth2Config(redirectBase string) (*oauth2.Config, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return nil, ErrGoogleMissingClientCfg
	}

	redirectURI := redirectBase + "/api/v1/scheduling/google/callback"

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes: []string{
			"https://www.googleapis.com/auth/calendar.readonly",
			"https://www.googleapis.com/auth/calendar.events",
		},
		Endpoint: googleOAuth2Endpoint,
	}, nil
}

// GetAuthorizationURL builds the Google OAuth authorization URL
func (s *GoogleCalendarService) GetAuthorizationURL(ctx context.Context, orgID, userID, redirectBase string) (string, error) {
	if s.encryptionKey == nil {
		return "", ErrMissingEncryptionKey
	}

	oauth2Config, err := s.buildOAuth2Config(redirectBase)
	if err != nil {
		return "", err
	}

	// Generate CSRF state: base64(orgID + ":" + userID + ":" + random)
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}
	stateData := orgID + ":" + userID + ":" + base64.URLEncoding.EncodeToString(randomBytes)
	state := base64.URLEncoding.EncodeToString([]byte(stateData))

	authURL := oauth2Config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	log.Printf("[GoogleCalendar] Generated auth URL for user %s in org %s", userID, orgID)
	return authURL, nil
}

// HandleCallback exchanges the OAuth code for tokens and stores them encrypted
// Returns orgID, userID, error
func (s *GoogleCalendarService) HandleCallback(ctx context.Context, code, state, redirectBase string) (string, string, error) {
	if s.encryptionKey == nil {
		return "", "", ErrMissingEncryptionKey
	}

	// Parse state to extract orgID and userID
	stateDecoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return "", "", ErrGoogleInvalidState
	}
	parts := strings.SplitN(string(stateDecoded), ":", 3)
	if len(parts) < 2 {
		return "", "", ErrGoogleInvalidState
	}
	orgID := parts[0]
	userID := parts[1]

	oauth2Config, err := s.buildOAuth2Config(redirectBase)
	if err != nil {
		return "", "", err
	}

	// Exchange code for tokens
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		log.Printf("[GoogleCalendar] Failed to exchange code for org %s user %s: %v", orgID, userID, err)
		return "", "", fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Encrypt tokens
	encryptedAccess, err := util.EncryptToken(token.AccessToken, s.encryptionKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to encrypt access token: %w", err)
	}

	var encryptedRefresh []byte
	if token.RefreshToken != "" {
		encryptedRefresh, err = util.EncryptToken(token.RefreshToken, s.encryptionKey)
		if err != nil {
			return "", "", fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	}

	now := time.Now()
	conn := &entity.GoogleCalendarConnection{
		ID:                    uuid.New().String(),
		OrgID:                 orgID,
		UserID:                userID,
		AccessTokenEncrypted:  encryptedAccess,
		RefreshTokenEncrypted: encryptedRefresh,
		CalendarID:            "primary",
		ConnectedAt:           &now,
	}
	if !token.Expiry.IsZero() {
		conn.TokenExpiry = &token.Expiry
	}

	if err := s.repo.UpsertGoogleConnection(ctx, conn); err != nil {
		log.Printf("[GoogleCalendar] Failed to store tokens for org %s user %s: %v", orgID, userID, err)
		return "", "", fmt.Errorf("failed to store tokens: %w", err)
	}

	log.Printf("[GoogleCalendar] Tokens stored for user %s in org %s", userID, orgID)
	return orgID, userID, nil
}

// getHTTPClient returns an authenticated HTTP client for Google Calendar API calls
func (s *GoogleCalendarService) getHTTPClient(ctx context.Context, orgID, userID string) (*http.Client, *entity.GoogleCalendarConnection, error) {
	if s.encryptionKey == nil {
		return nil, nil, ErrMissingEncryptionKey
	}

	conn, err := s.repo.GetGoogleConnection(ctx, orgID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, ErrGoogleNotConnected
		}
		return nil, nil, fmt.Errorf("failed to load google connection: %w", err)
	}

	if conn.AccessTokenEncrypted == nil {
		return nil, nil, ErrGoogleNoTokens
	}

	accessToken, err := util.DecryptToken(conn.AccessTokenEncrypted, s.encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	var refreshToken string
	if conn.RefreshTokenEncrypted != nil {
		refreshToken, _ = util.DecryptToken(conn.RefreshTokenEncrypted, s.encryptionKey)
	}

	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	}
	if conn.TokenExpiry != nil {
		token.Expiry = *conn.TokenExpiry
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	oauth2Config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     googleOAuth2Endpoint,
	}

	client := oauth2Config.Client(ctx, token)
	return client, conn, nil
}

// GetFreeBusySlots fetches busy time slots from Google Calendar
func (s *GoogleCalendarService) GetFreeBusySlots(ctx context.Context, orgID, userID string, start, end time.Time) ([]BusySlot, error) {
	client, _, err := s.getHTTPClient(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, ErrGoogleNotConnected) || errors.Is(err, ErrGoogleNoTokens) {
			return nil, nil // Not connected, return empty (no busy times)
		}
		return nil, err
	}

	requestBody := map[string]interface{}{
		"timeMin": start.UTC().Format(time.RFC3339),
		"timeMax": end.UTC().Format(time.RFC3339),
		"items":   []map[string]string{{"id": "primary"}},
	}
	bodyBytes, _ := json.Marshal(requestBody)

	resp, err := client.Post(
		"https://www.googleapis.com/calendar/v3/freeBusy",
		"application/json",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call FreeBusy API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("FreeBusy API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Calendars map[string]struct {
			Busy []struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"busy"`
		} `json:"calendars"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode FreeBusy response: %w", err)
	}

	var slots []BusySlot
	if cal, ok := result.Calendars["primary"]; ok {
		for _, b := range cal.Busy {
			startTime, err1 := time.Parse(time.RFC3339, b.Start)
			endTime, err2 := time.Parse(time.RFC3339, b.End)
			if err1 == nil && err2 == nil {
				slots = append(slots, BusySlot{Start: startTime, End: endTime})
			}
		}
	}

	return slots, nil
}

// CreateEvent creates a Google Calendar event
func (s *GoogleCalendarService) CreateEvent(ctx context.Context, orgID, userID string, event CalendarEvent) (string, error) {
	client, _, err := s.getHTTPClient(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, ErrGoogleNotConnected) {
			return "", nil // Not connected, skip event creation
		}
		return "", err
	}

	tz := event.Timezone
	if tz == "" {
		tz = "UTC"
	}

	eventBody := map[string]interface{}{
		"summary":     event.Summary,
		"description": event.Description,
		"start": map[string]string{
			"dateTime": event.StartTime.UTC().Format(time.RFC3339),
			"timeZone": tz,
		},
		"end": map[string]string{
			"dateTime": event.EndTime.UTC().Format(time.RFC3339),
			"timeZone": tz,
		},
		"attendees": []map[string]string{
			{"email": event.GuestEmail, "displayName": event.GuestName},
		},
	}

	bodyBytes, _ := json.Marshal(eventBody)
	resp, err := client.Post(
		"https://www.googleapis.com/calendar/v3/calendars/primary/events",
		"application/json",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create calendar event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create event API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode create event response: %w", err)
	}

	log.Printf("[GoogleCalendar] Created event %s for user %s in org %s", result.ID, userID, orgID)
	return result.ID, nil
}

// GetConnectionStatus returns the Google Calendar connection status
func (s *GoogleCalendarService) GetConnectionStatus(ctx context.Context, orgID, userID string) (string, error) {
	conn, err := s.repo.GetGoogleConnection(ctx, orgID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "not_connected", nil
		}
		return "", fmt.Errorf("failed to get google connection: %w", err)
	}

	if conn.AccessTokenEncrypted == nil {
		return "not_connected", nil
	}

	if conn.TokenExpiry != nil && time.Now().After(*conn.TokenExpiry) {
		// Token expired but may have refresh token
		if conn.RefreshTokenEncrypted == nil {
			return "expired", nil
		}
		// Has refresh token — will auto-refresh
		return "connected", nil
	}

	return "connected", nil
}

// GetEncryptionKey returns the encryption key (for creating derived service instances)
func (s *GoogleCalendarService) GetEncryptionKey() []byte {
	return s.encryptionKey
}

// Disconnect removes the Google Calendar connection
func (s *GoogleCalendarService) Disconnect(ctx context.Context, orgID, userID string) error {
	err := s.repo.DeleteGoogleConnection(ctx, orgID, userID)
	if err != nil {
		return fmt.Errorf("failed to disconnect google calendar: %w", err)
	}
	log.Printf("[GoogleCalendar] Disconnected user %s in org %s", userID, orgID)
	return nil
}
