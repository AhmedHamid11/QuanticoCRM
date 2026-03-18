package service

import (
	"context"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/google/uuid"
)

// SuppressionRule defines a field-based rule for blocking enrollment.
// Field is the contact field name, Operator is the comparison operator ("eq"),
// and Value is the value to compare against.
type SuppressionRule struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// EnrollResult is returned by EnrollContact, containing either a successful
// enrollment or a structured warning about an existing active enrollment.
type EnrollResult struct {
	// Warning is true when the contact is already active in another sequence.
	Warning bool `json:"warning"`
	// Enrolled is true when the contact was successfully enrolled.
	Enrolled bool `json:"enrolled,omitempty"`
	// EnrollmentID is set on successful enrollment.
	EnrollmentID string `json:"enrollmentId,omitempty"`
	// ExistingSequence is the name of the conflicting sequence (when Warning=true).
	ExistingSequence string `json:"existingSequence,omitempty"`
	// ExistingEnrollmentID is the ID of the conflicting enrollment (when Warning=true).
	ExistingEnrollmentID string `json:"existingEnrollmentId,omitempty"`
}

// BulkEnrollResult summarises the outcome of a BulkEnroll call.
type BulkEnrollResult struct {
	Enrolled        int      `json:"enrolled"`
	Skipped         int      `json:"skipped"`
	SkippedContacts []string `json:"skippedContacts"`
}

// SuppressionResult is the outcome of a CheckSuppression call.
type SuppressionResult struct {
	Suppressed bool   `json:"suppressed"`
	Reason     string `json:"reason,omitempty"`
}

// SequenceService implements sequence CRUD, enrollment with overlap guard,
// suppression checking, and the enrollment FSM.
type SequenceService struct {
	repo *repo.SequenceRepo
}

// NewSequenceService creates a new SequenceService.
// repo may be nil when the service is used only for FSM validation (e.g., in unit tests).
func NewSequenceService(r *repo.SequenceRepo) *SequenceService {
	return &SequenceService{repo: r}
}

// ========== FSM ==========

// legalTransitions defines the allowed enrollment status transitions.
// Key = from-status, value = set of allowed to-statuses.
var legalTransitions = map[string]map[string]bool{
	entity.EnrollmentStatusEnrolled: {
		entity.EnrollmentStatusActive: true,
	},
	entity.EnrollmentStatusActive: {
		entity.EnrollmentStatusFinished: true,
		entity.EnrollmentStatusPaused:   true,
		entity.EnrollmentStatusReplied:  true,
		entity.EnrollmentStatusBounced:  true,
		entity.EnrollmentStatusOptedOut: true,
	},
	entity.EnrollmentStatusPaused: {
		entity.EnrollmentStatusActive:   true,
		entity.EnrollmentStatusFinished: true,
	},
}

// isLegalTransition returns true if transitioning from -> to is allowed.
func isLegalTransition(from, to string) bool {
	allowed, ok := legalTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// TransitionEnrollment applies a state machine transition to an enrollment.
// It mutates the enrollment's Status field on success.
// Returns an error if the transition is not legal.
func (s *SequenceService) TransitionEnrollment(enrollment *entity.SequenceEnrollment, targetStatus string) error {
	if !isLegalTransition(enrollment.Status, targetStatus) {
		return fmt.Errorf("illegal enrollment transition: %s -> %s", enrollment.Status, targetStatus)
	}
	enrollment.Status = targetStatus
	return nil
}

// ========== Enrollment ==========

// EnrollContact enrolls a contact in a sequence.
//
// Logic:
//  1. Check suppression (opt-out list + rules if provided).
//  2. Check if contact is already enrolled in THIS sequence.
//  3. Check overlap: any active enrollment in ANY other sequence.
//     - If overlap and forceEnroll=false: return warning (not error).
//     - If overlap and forceEnroll=true: proceed.
//  4. Create enrollment (status=enrolled).
//  5. Create first StepExecution (status=scheduled, scheduled_at=NOW).
func (s *SequenceService) EnrollContact(ctx context.Context, orgID, sequenceID, contactID, enrolledBy string, forceEnroll bool) (*EnrollResult, error) {
	// 1. Check suppression (no rules in this basic path — caller passes nil)
	suppression, err := s.CheckSuppression(ctx, orgID, contactID, nil)
	if err != nil {
		return nil, fmt.Errorf("suppression check failed: %w", err)
	}
	if suppression.Suppressed {
		return nil, fmt.Errorf("contact is suppressed: %s", suppression.Reason)
	}

	// 2. Check if already enrolled in THIS sequence
	existing, err := s.repo.GetActiveEnrollmentBySequenceAndContact(ctx, sequenceID, contactID)
	if err != nil {
		return nil, fmt.Errorf("enrollment check failed: %w", err)
	}
	if existing != nil {
		return &EnrollResult{
			Warning:              true,
			ExistingSequence:     sequenceID, // will be enriched by caller if needed
			ExistingEnrollmentID: existing.ID,
		}, nil
	}

	// 3. Check overlap in other sequences
	if !forceEnroll {
		overlaps, err := s.repo.GetActiveEnrollmentsByContact(ctx, orgID, contactID)
		if err != nil {
			return nil, fmt.Errorf("overlap check failed: %w", err)
		}
		for _, overlap := range overlaps {
			if overlap.SequenceID != sequenceID {
				// Fetch the sequence name for the warning
				seq, seqErr := s.repo.GetSequence(ctx, orgID, overlap.SequenceID)
				existingName := overlap.SequenceID
				if seqErr == nil && seq != nil {
					existingName = seq.Name
				}
				return &EnrollResult{
					Warning:              true,
					ExistingSequence:     existingName,
					ExistingEnrollmentID: overlap.ID,
				}, nil
			}
		}
	}

	// 4. Create enrollment
	now := time.Now().UTC()
	enrollmentID := uuid.New().String()
	enrollment := &entity.SequenceEnrollment{
		ID:          enrollmentID,
		SequenceID:  sequenceID,
		ContactID:   contactID,
		OrgID:       orgID,
		EnrolledBy:  enrolledBy,
		Status:      entity.EnrollmentStatusEnrolled,
		CurrentStep: 0,
		EnrolledAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateEnrollment(ctx, enrollment); err != nil {
		return nil, fmt.Errorf("failed to create enrollment: %w", err)
	}

	// 5. Create first StepExecution if sequence has steps
	steps, err := s.repo.ListStepsBySequence(ctx, sequenceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list steps: %w", err)
	}
	if len(steps) > 0 {
		execNow := now
		stepExec := &entity.StepExecution{
			ID:           uuid.New().String(),
			EnrollmentID: enrollmentID,
			StepID:       steps[0].ID,
			OrgID:        orgID,
			Status:       entity.ExecutionStatusScheduled,
			ScheduledAt:  &execNow,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.repo.CreateStepExecution(ctx, stepExec); err != nil {
			return nil, fmt.Errorf("failed to create step execution: %w", err)
		}
	}

	return &EnrollResult{
		Enrolled:     true,
		EnrollmentID: enrollmentID,
	}, nil
}

// BulkEnroll enrolls multiple contacts in a sequence.
// Contacts already enrolled in the sequence are skipped.
// Returns a summary of enrolled vs skipped contacts.
func (s *SequenceService) BulkEnroll(ctx context.Context, orgID, sequenceID string, contactIDs []string, enrolledBy string) (*BulkEnrollResult, error) {
	// Query which contacts are already enrolled
	alreadyEnrolled, err := s.repo.GetEnrollmentsBySequenceAndContacts(ctx, sequenceID, contactIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing enrollments: %w", err)
	}

	// Split into enroll vs skip
	var toEnroll []string
	var skipped []string
	for _, id := range contactIDs {
		if alreadyEnrolled[id] {
			skipped = append(skipped, id)
		} else {
			toEnroll = append(toEnroll, id)
		}
	}

	// Fetch first step for scheduling
	steps, err := s.repo.ListStepsBySequence(ctx, sequenceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list steps: %w", err)
	}

	// Build and insert enrollments + step executions
	now := time.Now().UTC()
	var enrollments []*entity.SequenceEnrollment
	var executions []*entity.StepExecution

	for _, contactID := range toEnroll {
		enrollmentID := uuid.New().String()
		enrollments = append(enrollments, &entity.SequenceEnrollment{
			ID:          enrollmentID,
			SequenceID:  sequenceID,
			ContactID:   contactID,
			OrgID:       orgID,
			EnrolledBy:  enrolledBy,
			Status:      entity.EnrollmentStatusEnrolled,
			CurrentStep: 0,
			EnrolledAt:  now,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		if len(steps) > 0 {
			execNow := now
			executions = append(executions, &entity.StepExecution{
				ID:           uuid.New().String(),
				EnrollmentID: enrollmentID,
				StepID:       steps[0].ID,
				OrgID:        orgID,
				Status:       entity.ExecutionStatusScheduled,
				ScheduledAt:  &execNow,
				CreatedAt:    now,
				UpdatedAt:    now,
			})
		}
	}

	if err := s.repo.BulkCreateEnrollments(ctx, enrollments); err != nil {
		return nil, fmt.Errorf("failed to bulk create enrollments: %w", err)
	}

	for _, exec := range executions {
		if err := s.repo.CreateStepExecution(ctx, exec); err != nil {
			return nil, fmt.Errorf("failed to create step execution: %w", err)
		}
	}

	if skipped == nil {
		skipped = []string{}
	}

	return &BulkEnrollResult{
		Enrolled:        len(toEnroll),
		Skipped:         len(skipped),
		SkippedContacts: skipped,
	}, nil
}

// ========== Suppression ==========

// CheckSuppression evaluates whether a contact should be suppressed.
// Two-layer check:
//  1. Opt-out list: contact has channel='email' or channel='all' entry.
//  2. Status-field rules: each rule is evaluated against the contact's field value.
func (s *SequenceService) CheckSuppression(ctx context.Context, orgID, contactID string, rules []SuppressionRule) (*SuppressionResult, error) {
	// Layer 1: Opt-out list
	optedOut, err := s.repo.IsContactOptedOut(ctx, orgID, contactID)
	if err != nil {
		return nil, fmt.Errorf("opt-out check failed: %w", err)
	}
	if optedOut {
		return &SuppressionResult{
			Suppressed: true,
			Reason:     "contact is on the opt-out list",
		}, nil
	}

	// Layer 2: Status-field rules
	for _, rule := range rules {
		fieldValue, err := s.repo.GetContactFieldValue(ctx, orgID, contactID, rule.Field)
		if err != nil {
			return nil, fmt.Errorf("field value check failed for field %s: %w", rule.Field, err)
		}
		if matchesRule(fieldValue, rule.Operator, rule.Value) {
			return &SuppressionResult{
				Suppressed: true,
				Reason:     fmt.Sprintf("contact field %s %s %s", rule.Field, rule.Operator, rule.Value),
			}, nil
		}
	}

	return &SuppressionResult{Suppressed: false}, nil
}

// matchesRule evaluates a single suppression rule against a field value.
func matchesRule(fieldValue, operator, ruleValue string) bool {
	switch operator {
	case "eq", "=", "==":
		return fieldValue == ruleValue
	case "neq", "!=", "<>":
		return fieldValue != ruleValue
	default:
		return false
	}
}
