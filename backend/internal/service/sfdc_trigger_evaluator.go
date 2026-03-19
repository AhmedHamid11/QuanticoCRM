package service

import (
	"context"
	"fmt"
	"log"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

// MirrorTriggerEvaluator evaluates sequence enrollment and suppression triggers
// for records promoted by the Mirror ingest pipeline.
//
// It runs after each successful record promotion (not for skipped records) and:
//  1. Looks up all active enrollment triggers for (orgID, targetEntity)
//  2. Checks suppression first — if the contact is on the opt-out list, it skips enrollment
//  3. Enrolls the contact in any sequence whose trigger field matches
//
// The evaluator receives promoted field values directly from the ingest pipeline.
// It does NOT re-query the DB for field values (avoids stale-read pitfall).
type MirrorTriggerEvaluator struct {
	sequenceRepo    *repo.SequenceRepo
	sequenceService *SequenceService
}

// NewMirrorTriggerEvaluator creates a new MirrorTriggerEvaluator.
func NewMirrorTriggerEvaluator(seqRepo *repo.SequenceRepo, seqService *SequenceService) *MirrorTriggerEvaluator {
	return &MirrorTriggerEvaluator{
		sequenceRepo:    seqRepo,
		sequenceService: seqService,
	}
}

// EvaluateRecord evaluates enrollment and suppression triggers for a single promoted record.
//
// Parameters:
//   - tenantDB: the tenant database connection (used to scope all queries)
//   - orgID: the organisation owning the record
//   - targetEntity: the entity type (e.g. "Contact", "Lead")
//   - recordID: the ID of the promoted record (used as contactID for enrollment)
//   - promotedFields: the mapped (target) field values from the promotion pipeline
//
// Suppression takes precedence: if the contact is on the opt-out list, no enrollment
// is attempted for any trigger in this evaluation pass.
func (e *MirrorTriggerEvaluator) EvaluateRecord(
	ctx context.Context,
	tenantDB db.DBConn,
	orgID, targetEntity, recordID string,
	promotedFields map[string]interface{},
) {
	// Use tenant-scoped repo
	tenantRepo := e.sequenceRepo.WithDB(tenantDB)

	// 1. Load active triggers for this entity
	triggers, err := tenantRepo.ListEnrollmentTriggersByEntity(ctx, orgID, targetEntity)
	if err != nil {
		log.Printf("[TRIGGER-EVAL] Failed to list triggers for org=%s entity=%s: %v", orgID, targetEntity, err)
		return
	}
	if len(triggers) == 0 {
		return
	}

	// 2. Check suppression — opt-out list takes full precedence
	optedOut, err := tenantRepo.IsContactOptedOut(ctx, orgID, recordID)
	if err != nil {
		log.Printf("[TRIGGER-EVAL] opt-out check error for record %s: %v", recordID, err)
		// Non-fatal: continue with enrollment if we cannot confirm opt-out
	}
	if optedOut {
		log.Printf("[TRIGGER-EVAL] Record %s is on opt-out list — skipping all triggers", recordID)
		return
	}

	// 3. Find matching triggers
	// Group by sequence_id so we only attempt one enrollment per sequence
	type matchedSequence struct {
		sequenceID string
	}
	seenSequences := make(map[string]bool)
	var matches []entity.EnrollmentTrigger

	for _, trigger := range triggers {
		if seenSequences[trigger.SequenceID] {
			continue // already matched this sequence
		}
		fieldVal, exists := promotedFields[trigger.FieldName]
		if !exists {
			continue // field not in promoted record
		}
		strVal := fmt.Sprintf("%v", fieldVal)
		if matchesTriggerOperator(strVal, trigger.Operator, trigger.Value) {
			matches = append(matches, trigger)
			seenSequences[trigger.SequenceID] = true
		}
	}

	if len(matches) == 0 {
		return
	}

	// 4. Enroll into each matched sequence
	for _, trigger := range matches {
		result, err := e.sequenceService.EnrollContact(ctx, orgID, trigger.SequenceID, recordID, "system:mirror-trigger", false)
		if err != nil {
			// EnrollContact returns error only for hard failures (DB errors, suppression)
			// Log and continue — do not fail the entire evaluation
			log.Printf("[TRIGGER-EVAL] EnrollContact error for record=%s sequence=%s: %v", recordID, trigger.SequenceID, err)
			continue
		}
		if result.Warning {
			log.Printf("[TRIGGER-EVAL] Enrollment overlap for record=%s sequence=%s (existing: %s) — skipped",
				recordID, trigger.SequenceID, result.ExistingEnrollmentID)
		} else if result.Enrolled {
			log.Printf("[TRIGGER-EVAL] Enrolled record=%s into sequence=%s (enrollment=%s)",
				recordID, trigger.SequenceID, result.EnrollmentID)
		}
	}
}

// matchesTriggerOperator evaluates a single trigger operator against a field value.
// Supported operators: "eq" (equals), "neq" (not equals).
func matchesTriggerOperator(fieldValue, operator, ruleValue string) bool {
	switch operator {
	case "eq", "=", "==":
		return fieldValue == ruleValue
	case "neq", "!=", "<>":
		return fieldValue != ruleValue
	default:
		log.Printf("[TRIGGER-EVAL] Unknown operator %q — treating as no-match", operator)
		return false
	}
}
