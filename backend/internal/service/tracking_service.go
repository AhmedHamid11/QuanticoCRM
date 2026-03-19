package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/google/uuid"
)

// trackingPayload is the compact JSON encoded in a tracking token.
// Field names are kept short to minimise URL length.
type trackingPayload struct {
	OrgID           string `json:"o"`
	EnrollmentID    string `json:"e"`
	StepExecutionID string `json:"s"`
	LinkURL         string `json:"u,omitempty"` // only set for click tokens
}

// TrackingService generates and decodes tracking tokens, enqueues events to the
// EventBuffer, and rewrites links in outgoing email HTML bodies.
type TrackingService struct {
	eventBuffer *EventBuffer
	baseURL     string
	// hrefRe matches href="https://..." or href='https://...' in HTML.
	hrefRe *regexp.Regexp
}

// NewTrackingService creates a TrackingService.
// baseURL is the public API base (e.g. "https://api.quanticocrm.com/api/v1").
// Trailing slash is stripped for consistent URL construction.
func NewTrackingService(eventBuffer *EventBuffer, baseURL string) *TrackingService {
	return &TrackingService{
		eventBuffer: eventBuffer,
		baseURL:     strings.TrimRight(baseURL, "/"),
		hrefRe:      regexp.MustCompile(`href="(https?://[^"]+)"`),
	}
}

// GenerateTrackingToken base64url-encodes a trackingPayload for use in tracking URLs.
// The token is URL-safe and contains all data needed to record an event or redirect.
func (s *TrackingService) GenerateTrackingToken(orgID, enrollmentID, stepExecID, linkURL string) string {
	p := trackingPayload{
		OrgID:           orgID,
		EnrollmentID:    enrollmentID,
		StepExecutionID: stepExecID,
		LinkURL:         linkURL,
	}
	data, err := json.Marshal(p)
	if err != nil {
		log.Printf("[TrackingService] GenerateTrackingToken marshal error: %v", err)
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeTrackingToken reverses GenerateTrackingToken.
// Returns an error for malformed tokens; callers should silently fail.
func (s *TrackingService) DecodeTrackingToken(token string) (orgID, enrollmentID, stepExecID, linkURL string, err error) {
	data, decErr := base64.RawURLEncoding.DecodeString(token)
	if decErr != nil {
		return "", "", "", "", fmt.Errorf("base64 decode: %w", decErr)
	}
	var p trackingPayload
	if unmarshalErr := json.Unmarshal(data, &p); unmarshalErr != nil {
		return "", "", "", "", fmt.Errorf("json unmarshal: %w", unmarshalErr)
	}
	return p.OrgID, p.EnrollmentID, p.StepExecutionID, p.LinkURL, nil
}

// RecordOpen decodes the token and enqueues an open event to the EventBuffer.
// Errors are logged but not returned — open tracking is best-effort.
func (s *TrackingService) RecordOpen(token string) {
	orgID, enrollmentID, stepExecID, _, err := s.DecodeTrackingToken(token)
	if err != nil {
		log.Printf("[TrackingService] RecordOpen bad token: %v", err)
		return
	}
	now := time.Now().UTC()
	e := entity.TrackingEvent{
		ID:              uuid.NewString(),
		OrgID:           orgID,
		EnrollmentID:    enrollmentID,
		StepExecutionID: stepExecID,
		EventType:       entity.TrackingEventOpen,
		OccurredAt:      now,
		CreatedAt:       now,
	}
	if !s.eventBuffer.Enqueue(e) {
		log.Printf("[TrackingService] RecordOpen: EventBuffer full, event dropped for enrollment %s", enrollmentID)
	}
}

// RecordClick decodes the token, enqueues a click event, and returns the original URL
// for the 302 redirect. Returns an error only for genuinely bad tokens.
func (s *TrackingService) RecordClick(token string) (redirectURL string, err error) {
	orgID, enrollmentID, stepExecID, linkURL, decErr := s.DecodeTrackingToken(token)
	if decErr != nil {
		return "", fmt.Errorf("RecordClick decode: %w", decErr)
	}
	if linkURL == "" {
		return "", fmt.Errorf("RecordClick: token has no link URL")
	}

	now := time.Now().UTC()
	e := entity.TrackingEvent{
		ID:              uuid.NewString(),
		OrgID:           orgID,
		EnrollmentID:    enrollmentID,
		StepExecutionID: stepExecID,
		EventType:       entity.TrackingEventClick,
		LinkURL:         &linkURL,
		OccurredAt:      now,
		CreatedAt:       now,
	}
	if !s.eventBuffer.Enqueue(e) {
		log.Printf("[TrackingService] RecordClick: EventBuffer full, event dropped for enrollment %s", enrollmentID)
	}

	return linkURL, nil
}

// GeneratePixelURL returns the tracking pixel URL for an email send.
func (s *TrackingService) GeneratePixelURL(orgID, enrollmentID, stepExecID string) string {
	token := s.GenerateTrackingToken(orgID, enrollmentID, stepExecID, "")
	return fmt.Sprintf("%s/t/p/%s", s.baseURL, token)
}

// RewriteLinks replaces all href="https://..." occurrences in bodyHTML with
// tracking redirect URLs. Links pointing to mailto: are left untouched.
// Only double-quoted href attributes are matched (standard HTML).
func (s *TrackingService) RewriteLinks(bodyHTML, orgID, enrollmentID, stepExecID string) string {
	return s.hrefRe.ReplaceAllStringFunc(bodyHTML, func(match string) string {
		// Extract URL from href="..."
		sub := s.hrefRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		originalURL := sub[1]

		// Skip mailto links
		if strings.HasPrefix(originalURL, "mailto:") {
			return match
		}

		// Skip unsubscribe / opt-out links that point back to our own app
		// (to avoid double-wrapping compliance links)
		if strings.Contains(originalURL, "/unsubscribe") || strings.Contains(originalURL, "/opt-out") {
			return match
		}

		token := s.GenerateTrackingToken(orgID, enrollmentID, stepExecID, originalURL)
		redirectURL := fmt.Sprintf("%s/t/c/%s", s.baseURL, token)
		return fmt.Sprintf(`href="%s"`, redirectURL)
	})
}

// InjectTracking rewrites all links in bodyHTML and appends a 1x1 tracking pixel
// before the closing </body> tag (or at the end if </body> is absent).
func (s *TrackingService) InjectTracking(bodyHTML, orgID, enrollmentID, stepExecID string) string {
	// Rewrite links
	result := s.RewriteLinks(bodyHTML, orgID, enrollmentID, stepExecID)

	// Append pixel
	pixelURL := s.GeneratePixelURL(orgID, enrollmentID, stepExecID)
	pixel := fmt.Sprintf(`<img src="%s" width="1" height="1" style="display:none" alt="" />`, pixelURL)

	if idx := strings.LastIndex(result, "</body>"); idx != -1 {
		return result[:idx] + pixel + result[idx:]
	}
	return result + pixel
}
