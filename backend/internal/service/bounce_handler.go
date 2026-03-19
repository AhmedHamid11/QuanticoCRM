package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/google/uuid"
)

// BounceHandler processes email bounces and applies the appropriate enrollment
// state transitions and contact suppression.
//
// Hard bounce: set do_not_email=1 on contact, transition enrollment to opted_out.
// Soft bounce: increment soft_bounce_count; pause enrollment after 2 soft bounces.
type BounceHandler struct {
	eventBuffer  *EventBuffer
	trackingRepo *repo.TrackingRepo
	seqRepo      *repo.SequenceRepo
	seqSvc       *SequenceService
}

// NewBounceHandler creates a BounceHandler.
func NewBounceHandler(
	eventBuffer *EventBuffer,
	trackingRepo *repo.TrackingRepo,
	seqRepo *repo.SequenceRepo,
	seqSvc *SequenceService,
) *BounceHandler {
	return &BounceHandler{
		eventBuffer:  eventBuffer,
		trackingRepo: trackingRepo,
		seqRepo:      seqRepo,
		seqSvc:       seqSvc,
	}
}

// ProcessBounce handles a detected bounce for an enrollment.
// subject is the Subject header from the bounce message, used to classify hard vs soft.
func (h *BounceHandler) ProcessBounce(ctx context.Context, orgID, enrollmentID, stepExecID, subject string) error {
	enrollment, err := h.seqRepo.GetEnrollment(ctx, enrollmentID)
	if err != nil {
		return fmt.Errorf("bounce handler: get enrollment %s: %w", enrollmentID, err)
	}
	if enrollment == nil {
		return fmt.Errorf("bounce handler: enrollment %s not found", enrollmentID)
	}

	if isHardBounce(subject) {
		return h.processHardBounce(ctx, orgID, enrollment, stepExecID, subject)
	}
	return h.processSoftBounce(ctx, orgID, enrollment, stepExecID, subject)
}

// processHardBounce sets do_not_email on the contact and opts out the enrollment.
func (h *BounceHandler) processHardBounce(ctx context.Context, orgID string, enrollment *entity.SequenceEnrollment, stepExecID, subject string) error {
	// Set do_not_email on the contact
	if err := h.trackingRepo.SetDoNotEmail(ctx, orgID, enrollment.ContactID); err != nil {
		log.Printf("[BounceHandler] SetDoNotEmail failed for contact %s: %v", enrollment.ContactID, err)
	}

	// Transition enrollment to opted_out
	if transErr := h.seqSvc.TransitionEnrollment(enrollment, entity.EnrollmentStatusOptedOut); transErr == nil {
		if updateErr := h.seqRepo.UpdateEnrollmentStatus(ctx, enrollment.ID, entity.EnrollmentStatusOptedOut); updateErr != nil {
			log.Printf("[BounceHandler] UpdateEnrollmentStatus opted_out failed for %s: %v", enrollment.ID, updateErr)
		}
	} else {
		log.Printf("[BounceHandler] TransitionEnrollment to opted_out failed for %s: %v", enrollment.ID, transErr)
	}

	// Enqueue bounce tracking event
	h.enqueueEvent(orgID, enrollment.ID, stepExecID, map[string]string{
		"bounce_type": "hard",
		"subject":     subject,
	})

	log.Printf("[BounceHandler] Hard bounce for enrollment %s — contact %s set do_not_email", enrollment.ID, enrollment.ContactID)
	return nil
}

// processSoftBounce increments soft_bounce_count and pauses enrollment after 2 bounces.
func (h *BounceHandler) processSoftBounce(ctx context.Context, orgID string, enrollment *entity.SequenceEnrollment, stepExecID, subject string) error {
	newCount, err := h.trackingRepo.IncrementSoftBounceCount(ctx, orgID, enrollment.ID)
	if err != nil {
		return fmt.Errorf("bounce handler: IncrementSoftBounceCount for enrollment %s: %w", enrollment.ID, err)
	}

	// Enqueue bounce tracking event
	meta := map[string]string{
		"bounce_type": "soft",
		"count":       fmt.Sprintf("%d", newCount),
		"subject":     subject,
	}
	h.enqueueEvent(orgID, enrollment.ID, stepExecID, meta)

	if newCount >= 2 {
		// Pause the enrollment after 2 soft bounces
		if transErr := h.seqSvc.TransitionEnrollment(enrollment, entity.EnrollmentStatusPaused); transErr == nil {
			if updateErr := h.seqRepo.UpdateEnrollmentStatus(ctx, enrollment.ID, entity.EnrollmentStatusPaused); updateErr != nil {
				log.Printf("[BounceHandler] UpdateEnrollmentStatus paused failed for %s: %v", enrollment.ID, updateErr)
			}
		} else {
			log.Printf("[BounceHandler] TransitionEnrollment to paused failed for %s: %v", enrollment.ID, transErr)
		}
		log.Printf("[BounceHandler] Soft bounce count=%d for enrollment %s — pausing", newCount, enrollment.ID)
	} else {
		log.Printf("[BounceHandler] Soft bounce count=%d for enrollment %s — not yet pausing", newCount, enrollment.ID)
	}

	return nil
}

// enqueueEvent enqueues a bounce tracking event via the EventBuffer.
func (h *BounceHandler) enqueueEvent(orgID, enrollmentID, stepExecID string, meta map[string]string) {
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
		EventType:       entity.TrackingEventBounce,
		MetadataJSON:    metaJSON,
		OccurredAt:      now,
		CreatedAt:       now,
	}
	h.eventBuffer.Enqueue(event)
}

// isHardBounce returns true if the bounce subject line indicates a permanent delivery failure.
func isHardBounce(subject string) bool {
	subjectLower := strings.ToLower(subject)
	hardBounceKeywords := []string{
		"delivery status notification (failure)",
		"undeliverable",
		"does not exist",
		"address not found",
		"user unknown",
		"no such user",
		"invalid address",
		"permanent error",
		"550",
		"551",
		"552",
		"553",
		"554",
	}
	for _, kw := range hardBounceKeywords {
		if strings.Contains(subjectLower, kw) {
			return true
		}
	}
	// Check for generic "failure" or "undeliverable" in simpler subjects
	if strings.Contains(subjectLower, "failure") || strings.Contains(subjectLower, "failed") {
		return true
	}
	return false
}
