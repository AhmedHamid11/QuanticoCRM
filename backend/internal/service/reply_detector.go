package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/google/uuid"
)

// ReplyDetector polls Gmail threads to detect prospect replies, OOO auto-replies,
// and bounces for completed sequence email step executions.
type ReplyDetector struct {
	oauthSvc    *GmailOAuthService
	eventBuffer *EventBuffer
	seqSvc      *SequenceService
	seqRepo     *repo.SequenceRepo
	trackingRepo *repo.TrackingRepo
	bounceHandler *BounceHandler
}

// NewReplyDetector creates a ReplyDetector.
func NewReplyDetector(
	oauthSvc *GmailOAuthService,
	eventBuffer *EventBuffer,
	seqSvc *SequenceService,
	seqRepo *repo.SequenceRepo,
	trackingRepo *repo.TrackingRepo,
	bounceHandler *BounceHandler,
) *ReplyDetector {
	return &ReplyDetector{
		oauthSvc:     oauthSvc,
		eventBuffer:  eventBuffer,
		seqSvc:       seqSvc,
		seqRepo:      seqRepo,
		trackingRepo: trackingRepo,
		bounceHandler: bounceHandler,
	}
}

// gmailThreadResponse is the partial structure of the Gmail threads.get API response.
type gmailThreadResponse struct {
	ID       string         `json:"id"`
	Messages []gmailMessage `json:"messages"`
}

type gmailMessage struct {
	ID      string        `json:"id"`
	Payload gmailPayload  `json:"payload"`
}

type gmailPayload struct {
	Headers []gmailHeader `json:"headers"`
}

type gmailHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CheckThreadForReplies polls a Gmail thread and detects reply, ooo, or bounce.
// Returns the reply type ("reply", "ooo", "bounce", "none") and any error.
func (d *ReplyDetector) CheckThreadForReplies(
	ctx context.Context,
	orgID, userID, threadID, enrollmentID, stepExecID string,
) (replyType string, err error) {
	client, _, err := d.oauthSvc.GetHTTPClient(ctx, orgID, userID)
	if err != nil {
		return "", fmt.Errorf("reply detector: get HTTP client: %w", err)
	}

	url := fmt.Sprintf(
		"https://gmail.googleapis.com/gmail/v1/users/me/threads/%s?format=metadata&metadataHeaders=X-Auto-Reply&metadataHeaders=Auto-Submitted&metadataHeaders=From&metadataHeaders=Subject",
		threadID,
	)
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("reply detector: thread fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("reply detector: Gmail API returned %d: %s", resp.StatusCode, string(body))
	}

	var thread gmailThreadResponse
	if err := json.Unmarshal(body, &thread); err != nil {
		return "", fmt.Errorf("reply detector: parse thread response: %w", err)
	}

	// Only the original outbound message — no replies yet
	if len(thread.Messages) <= 1 {
		return "none", nil
	}

	// Check messages after the first (which is our outbound message)
	for _, msg := range thread.Messages[1:] {
		headers := msg.Payload.Headers

		// Check for bounce (mailer-daemon or postmaster in From header)
		fromHeader := getHeader(headers, "From")
		fromLower := strings.ToLower(fromHeader)
		if strings.Contains(fromLower, "mailer-daemon") || strings.Contains(fromLower, "postmaster") {
			subjectHeader := getHeader(headers, "Subject")
			d.enqueueEvent(orgID, enrollmentID, stepExecID, entity.TrackingEventBounce, map[string]string{
				"from":    fromHeader,
				"subject": subjectHeader,
			})
			// Delegate to BounceHandler for state transitions
			if d.bounceHandler != nil {
				if bounceErr := d.bounceHandler.ProcessBounce(ctx, orgID, enrollmentID, stepExecID, subjectHeader); bounceErr != nil {
					log.Printf("[ReplyDetector] BounceHandler.ProcessBounce failed for exec %s: %v", stepExecID, bounceErr)
				}
			}
			return "bounce", nil
		}

		// Check for OOO auto-reply
		if isOOOReply(headers) {
			d.enqueueEvent(orgID, enrollmentID, stepExecID, entity.TrackingEventOOO, nil)
			// OOO does NOT pause enrollment per product decision
			log.Printf("[ReplyDetector] OOO reply detected for enrollment %s, exec %s — not pausing", enrollmentID, stepExecID)
			return "ooo", nil
		}

		// Genuine reply — pause enrollment
		d.enqueueEvent(orgID, enrollmentID, stepExecID, entity.TrackingEventReply, nil)
		enrollment, repoErr := d.seqRepo.GetEnrollment(ctx, enrollmentID)
		if repoErr != nil || enrollment == nil {
			log.Printf("[ReplyDetector] enrollment %s not found for reply transition: %v", enrollmentID, repoErr)
			return "reply", nil
		}
		if transErr := d.seqSvc.TransitionEnrollment(enrollment, entity.EnrollmentStatusReplied); transErr != nil {
			log.Printf("[ReplyDetector] TransitionEnrollment to replied failed for %s: %v", enrollmentID, transErr)
		} else {
			if updateErr := d.seqRepo.UpdateEnrollmentStatus(ctx, enrollment.ID, entity.EnrollmentStatusReplied); updateErr != nil {
				log.Printf("[ReplyDetector] UpdateEnrollmentStatus replied failed for %s: %v", enrollmentID, updateErr)
			}
		}
		return "reply", nil
	}

	return "none", nil
}

// enqueueEvent builds and enqueues a tracking event via the EventBuffer.
func (d *ReplyDetector) enqueueEvent(orgID, enrollmentID, stepExecID, eventType string, meta map[string]string) {
	var metaJSON *string
	if len(meta) > 0 {
		b, _ := json.Marshal(meta)
		s := string(b)
		metaJSON = &s
	}
	now := time.Now().UTC()
	event := entity.TrackingEvent{
		ID:              uuid.New().String(),
		OrgID:           orgID,
		EnrollmentID:    enrollmentID,
		StepExecutionID: stepExecID,
		EventType:       eventType,
		MetadataJSON:    metaJSON,
		OccurredAt:      now,
		CreatedAt:       now,
	}
	d.eventBuffer.Enqueue(event)
}

// isOOOReply returns true if the message headers indicate an auto-reply / OOO.
func isOOOReply(headers []gmailHeader) bool {
	for _, h := range headers {
		nameLower := strings.ToLower(h.Name)
		valLower := strings.ToLower(h.Value)

		if nameLower == "x-auto-reply" {
			return true
		}
		if nameLower == "auto-submitted" && valLower == "auto-replied" {
			return true
		}
		if nameLower == "subject" {
			if strings.Contains(valLower, "out of office") ||
				strings.Contains(valLower, "automatic reply") ||
				strings.Contains(valLower, "auto-reply") ||
				strings.Contains(valLower, " ooo ") ||
				strings.HasPrefix(valLower, "ooo ") ||
				strings.HasSuffix(valLower, " ooo") {
				return true
			}
		}
	}
	return false
}

// getHeader retrieves the value of the first matching header (case-insensitive).
func getHeader(headers []gmailHeader, name string) string {
	nameLower := strings.ToLower(name)
	for _, h := range headers {
		if strings.ToLower(h.Name) == nameLower {
			return h.Value
		}
	}
	return ""
}
