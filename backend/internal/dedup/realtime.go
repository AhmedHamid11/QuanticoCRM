package dedup

import (
	"context"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
)

// RealtimeChecker coordinates async duplicate detection
type RealtimeChecker struct {
	detector  *Detector
	alertRepo *repo.PendingAlertRepo
	ruleRepo  *repo.MatchingRuleRepo
}

// NewRealtimeChecker creates a new realtime checker
func NewRealtimeChecker(detector *Detector, alertRepo *repo.PendingAlertRepo, ruleRepo *repo.MatchingRuleRepo) *RealtimeChecker {
	return &RealtimeChecker{
		detector:  detector,
		alertRepo: alertRepo,
		ruleRepo:  ruleRepo,
	}
}

// CheckAsyncInput contains data needed for async detection
type CheckAsyncInput struct {
	OrgID      string
	UserID     string
	EntityType string
	RecordID   string
	RecordData map[string]interface{}
	RecordName string // For display in alerts
}

// CheckAsync spawns a goroutine to check for duplicates and store alert if found
// CRITICAL: This method MUST be called AFTER the record has been successfully saved to the database.
// The optimistic save pattern guarantees the record exists before async detection begins.
func (r *RealtimeChecker) CheckAsync(conn db.DBConn, input CheckAsyncInput) {
	// Create timeout context for async work (not tied to request lifecycle)
	asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	go func() {
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC in async dedup check for %s/%s: %v", input.EntityType, input.RecordID, rec)
			}
		}()

		r.runCheck(asyncCtx, conn, input)
	}()
}

// runCheck performs the actual detection and stores alert if duplicates found
func (r *RealtimeChecker) runCheck(ctx context.Context, conn db.DBConn, input CheckAsyncInput) {
	// Removed per-record log to reduce noise in production

	// Ensure dedup schema exists (core tables like matching_rules, pending_duplicate_alerts)
	if err := EnsureDedupSchema(ctx, conn); err != nil {
		log.Printf("ERROR: Failed to ensure dedup schema for %s/%s: %v", input.EntityType, input.RecordID, err)
		return
	}

	// Ensure blocking key columns exist on this entity's table (not just contacts)
	if err := EnsureBlockingKeysForEntity(ctx, conn, input.EntityType); err != nil {
		log.Printf("ERROR: Failed to ensure blocking keys for %s: %v", input.EntityType, err)
		// Continue anyway — column provisioning is best-effort
	}

	// Backfill blocking keys for existing records of this entity that predate the dedup feature
	if err := BackfillBlockingKeysForEntity(ctx, conn, input.EntityType, r.detector.GetBlocker()); err != nil {
		log.Printf("ERROR: Failed to backfill blocking keys for %s: %v", input.EntityType, err)
		// Continue anyway — backfill is best-effort
	}

	// Always update blocking keys for this record so it can be found by future checks
	// This must happen regardless of whether rules exist, since rules may be added later
	if err := r.detector.GetBlocker().UpdateBlockingKeys(ctx, conn, input.EntityType, input.RecordID, input.RecordData); err != nil {
		log.Printf("ERROR: Failed to update blocking keys for %s/%s: %v", input.EntityType, input.RecordID, err)
		// Continue anyway - blocking key update is best-effort
	}

	// Check if any matching rules exist for this entity (quick bailout)
	rules, err := r.ruleRepo.WithDB(conn).ListEnabledRules(ctx, input.OrgID, input.EntityType)
	if err != nil {
		log.Printf("ERROR: Failed to get matching rules for %s: %v", input.EntityType, err)
		return
	}
	if len(rules) == 0 {
		return // No rules configured, nothing to check
	}

	// Determine if any rule has block mode enabled
	// NOTE: Current matching_rules schema doesn't have block_mode column
	// For now, default to warn mode (isBlockMode = false)
	// When block_mode column is added to matching_rules table, update this logic
	isBlockMode := false

	// Get merge display fields from the first (highest priority) rule
	// These define which fields to show on the merge screen
	var mergeDisplayFields []string
	if len(rules) > 0 && len(rules[0].MergeDisplayFields) > 0 {
		mergeDisplayFields = rules[0].MergeDisplayFields
	}

	// Run duplicate detection
	matches, err := r.detector.CheckForDuplicates(ctx, conn, input.OrgID, input.EntityType, input.RecordData, input.RecordID)
	if err != nil {
		log.Printf("ERROR: Async dedup check failed for %s/%s: %v", input.EntityType, input.RecordID, err)
		return
	}

	if len(matches) == 0 {
		return // No duplicates found
	}
	log.Printf("[DEDUP] Found %d matches for %s/%s", len(matches), input.EntityType, input.RecordID)

	// Convert matches to alert format (top 3 per CONTEXT.md)
	maxMatches := 3
	if len(matches) < maxMatches {
		maxMatches = len(matches)
	}

	alertMatches := make([]entity.DuplicateAlertMatch, maxMatches)
	highestConfidence := entity.ConfidenceLow

	for i := 0; i < maxMatches; i++ {
		match := matches[i]
		alertMatches[i] = entity.DuplicateAlertMatch{
			RecordID:    match.RecordID,
			MatchResult: match.MatchResult,
		}

		// Look up display name for matched record
		matchedRecord, err := util.FetchRecordAsMap(ctx, db.GetRawDB(conn), util.GetTableName(input.EntityType), match.RecordID, input.OrgID)
		if err == nil && matchedRecord != nil {
			alertMatches[i].RecordName = util.GetRecordDisplayName(input.EntityType, matchedRecord)
		}

		// Track highest confidence
		if match.MatchResult != nil {
			tier := match.MatchResult.ConfidenceTier
			if tier == entity.ConfidenceHigh {
				highestConfidence = entity.ConfidenceHigh
			} else if tier == entity.ConfidenceMedium && highestConfidence != entity.ConfidenceHigh {
				highestConfidence = entity.ConfidenceMedium
			}
		}
	}

	// Store alert for the current record pointing to its matches
	alert := &entity.PendingDuplicateAlert{
		OrgID:              input.OrgID,
		EntityType:         input.EntityType,
		RecordID:           input.RecordID,
		Matches:            alertMatches,
		TotalMatchCount:    len(matches),
		HighestConfidence:  highestConfidence,
		IsBlockMode:        isBlockMode,
		MergeDisplayFields: mergeDisplayFields,
		Status:             entity.AlertStatusPending,
		DetectedAt:         time.Now().UTC(),
	}

	if err := r.alertRepo.WithDB(conn).Upsert(ctx, alert); err != nil {
		log.Printf("ERROR: Failed to store duplicate alert for %s/%s: %v", input.EntityType, input.RecordID, err)
	} else {
		log.Printf("INFO: Created duplicate alert for %s/%s with %d matches (highest: %s, blockMode: %v)",
			input.EntityType, input.RecordID, len(matches), highestConfidence, isBlockMode)
	}

	// Create bidirectional alerts - each matched record should also see this record as a duplicate
	// This ensures both parties in a duplicate pair can see and resolve the alert
	now := time.Now().UTC()
	for _, match := range alertMatches {
		reverseMatch := entity.DuplicateAlertMatch{
			RecordID:    input.RecordID,
			RecordName:  input.RecordName,
			MatchResult: match.MatchResult, // Same match result applies both directions
		}

		reverseAlert := &entity.PendingDuplicateAlert{
			OrgID:              input.OrgID,
			EntityType:         input.EntityType,
			RecordID:           match.RecordID, // The other record
			Matches:            []entity.DuplicateAlertMatch{reverseMatch},
			TotalMatchCount:    1,
			HighestConfidence:  highestConfidence,
			IsBlockMode:        isBlockMode,
			MergeDisplayFields: mergeDisplayFields,
			Status:             entity.AlertStatusPending,
			DetectedAt:         now,
		}

		if err := r.alertRepo.WithDB(conn).Upsert(ctx, reverseAlert); err != nil {
			log.Printf("ERROR: Failed to store reverse duplicate alert for %s/%s: %v", input.EntityType, match.RecordID, err)
		} else {
			log.Printf("INFO: Created reverse duplicate alert for %s/%s pointing to %s",
				input.EntityType, match.RecordID, input.RecordID)
		}
	}
}

// HasRulesForEntity checks if any enabled rules exist for an entity type
// Used for quick bailout before spawning goroutine
func (r *RealtimeChecker) HasRulesForEntity(ctx context.Context, conn db.DBConn, orgID, entityType string) bool {
	rules, err := r.ruleRepo.WithDB(conn).ListEnabledRules(ctx, orgID, entityType)
	return err == nil && len(rules) > 0
}

// CheckAsyncWithMap is the interface-compatible version
func (r *RealtimeChecker) CheckAsyncWithMap(conn db.DBConn, orgID, userID, entityType, recordID, recordName string, recordData map[string]interface{}) {
	r.CheckAsync(conn, CheckAsyncInput{
		OrgID:      orgID,
		UserID:     userID,
		EntityType: entityType,
		RecordID:   recordID,
		RecordData: recordData,
		RecordName: recordName,
	})
}
