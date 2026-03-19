package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/google/uuid"
)

// ABService implements A/B test variant assignment, stats aggregation, and winner promotion.
// Variant assignment is per-enrollment (sequence-level), not per-step: one variant is assigned
// at enrollment time and applies to all email steps in the sequence.
type ABService struct {
	getDB       func(orgID string) *sql.DB
	trackingRepo *repo.TrackingRepo
}

// NewABService creates an ABService. getDB should return the tenant DB for a given orgID.
func NewABService(getDB func(orgID string) *sql.DB, trackingRepo *repo.TrackingRepo) *ABService {
	return &ABService{
		getDB:        getDB,
		trackingRepo: trackingRepo,
	}
}

// AssignVariant assigns an A/B variant at enrollment time using round-robin across active variants.
// Returns nil if no variants are configured for this sequence's first email step.
// If a winner has been promoted, only the winner is considered (100% traffic to winner).
func (s *ABService) AssignVariant(ctx context.Context, orgID, sequenceID string, tenantRepo *repo.SequenceRepo) (*string, error) {
	// Get all steps for this sequence, find the first email step
	steps, err := tenantRepo.ListStepsBySequence(ctx, sequenceID)
	if err != nil {
		return nil, fmt.Errorf("ABService.AssignVariant: list steps: %w", err)
	}

	var firstEmailStepID string
	for _, step := range steps {
		if step.StepType == entity.StepTypeEmail {
			firstEmailStepID = step.ID
			break
		}
	}
	if firstEmailStepID == "" {
		// No email steps — no variant to assign
		return nil, nil
	}

	tenantDB := s.getDB(orgID)
	if tenantDB == nil {
		return nil, nil
	}
	tRepo := s.trackingRepo.WithDB(tenantDB)

	variants, err := tRepo.ListABVariantsForStep(ctx, firstEmailStepID)
	if err != nil {
		return nil, fmt.Errorf("ABService.AssignVariant: list variants: %w", err)
	}
	if len(variants) == 0 {
		return nil, nil
	}

	// If a winner is set, use only the winner
	var winner *entity.ABTestVariant
	for i := range variants {
		if variants[i].IsWinner == 1 {
			winner = &variants[i]
			break
		}
	}
	if winner != nil {
		return &winner.ID, nil
	}

	// Filter to active variants (traffic_pct > 0)
	var active []entity.ABTestVariant
	for _, v := range variants {
		if v.TrafficPct > 0 {
			active = append(active, v)
		}
	}
	if len(active) == 0 {
		return nil, nil
	}

	// Count existing enrollments for round-robin
	var count int
	row := tenantDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sequence_enrollments WHERE sequence_id = ?`, sequenceID)
	if scanErr := row.Scan(&count); scanErr != nil {
		count = 0
	}

	assigned := active[count%len(active)]
	return &assigned.ID, nil
}

// GetVariantsWithStats returns variant definitions combined with their tracking stats for a step.
func (s *ABService) GetVariantsWithStats(ctx context.Context, orgID, stepID string) ([]ABVariantWithStats, error) {
	tenantDB := s.getDB(orgID)
	if tenantDB == nil {
		return nil, fmt.Errorf("ABService: no tenant DB for org %s", orgID)
	}
	tRepo := s.trackingRepo.WithDB(tenantDB)

	variants, err := tRepo.ListABVariantsForStep(ctx, stepID)
	if err != nil {
		return nil, fmt.Errorf("ABService.GetVariantsWithStats: list variants: %w", err)
	}

	stats, err := tRepo.GetABStatsForStep(ctx, stepID)
	if err != nil {
		return nil, fmt.Errorf("ABService.GetVariantsWithStats: get stats: %w", err)
	}

	// Index stats by variant_id for O(1) lookup
	statsMap := make(map[string]entity.ABTrackingStats, len(stats))
	for _, stat := range stats {
		statsMap[stat.VariantID] = stat
	}

	result := make([]ABVariantWithStats, 0, len(variants))
	for _, v := range variants {
		entry := ABVariantWithStats{Variant: v}
		if stat, ok := statsMap[v.ID]; ok {
			entry.Stats = &stat
		}
		result = append(result, entry)
	}
	return result, nil
}

// ABVariantWithStats combines a variant definition with its aggregated tracking stats.
type ABVariantWithStats struct {
	Variant entity.ABTestVariant  `json:"variant"`
	Stats   *entity.ABTrackingStats `json:"stats,omitempty"`
}

// CreateVariant creates a new A/B test variant for a step.
func (s *ABService) CreateVariant(ctx context.Context, orgID string, variant *entity.ABTestVariant) error {
	tenantDB := s.getDB(orgID)
	if tenantDB == nil {
		return fmt.Errorf("ABService: no tenant DB for org %s", orgID)
	}
	return s.trackingRepo.WithDB(tenantDB).CreateABVariant(ctx, variant)
}

// UpdateVariant updates an existing A/B test variant.
func (s *ABService) UpdateVariant(ctx context.Context, orgID string, variant *entity.ABTestVariant) error {
	tenantDB := s.getDB(orgID)
	if tenantDB == nil {
		return fmt.Errorf("ABService: no tenant DB for org %s", orgID)
	}
	return s.trackingRepo.WithDB(tenantDB).UpdateABVariant(ctx, variant)
}

// DeleteVariant deletes an A/B test variant by ID.
func (s *ABService) DeleteVariant(ctx context.Context, orgID, variantID string) error {
	tenantDB := s.getDB(orgID)
	if tenantDB == nil {
		return fmt.Errorf("ABService: no tenant DB for org %s", orgID)
	}
	return s.trackingRepo.WithDB(tenantDB).DeleteABVariant(ctx, variantID)
}

// PromoteWinner promotes a variant to winner status.
// Sets is_winner=1, traffic_pct=100 on the winner and is_winner=0, traffic_pct=0 on all other variants for the same step.
func (s *ABService) PromoteWinner(ctx context.Context, orgID, variantID string) error {
	tenantDB := s.getDB(orgID)
	if tenantDB == nil {
		return fmt.Errorf("ABService: no tenant DB for org %s", orgID)
	}

	tRepo := s.trackingRepo.WithDB(tenantDB)

	// Fetch the variant to get its step_id
	variant, err := tRepo.GetABVariant(ctx, variantID)
	if err != nil {
		return fmt.Errorf("ABService.PromoteWinner: fetch variant: %w", err)
	}
	if variant == nil {
		return fmt.Errorf("ABService.PromoteWinner: variant %s not found", variantID)
	}

	return tRepo.SetABWinner(ctx, variant.StepID, variantID)
}

// IncrementABStats increments the specified counter in ab_tracking_stats for a variant.
// This is called after the EventBuffer flushes open/click/reply events.
func (s *ABService) IncrementABStats(ctx context.Context, orgID, variantID string, sends, opens, clicks, replies int) error {
	tenantDB := s.getDB(orgID)
	if tenantDB == nil {
		return nil // Degrade gracefully — don't fail the flush
	}
	statID := uuid.New().String()
	return s.trackingRepo.WithDB(tenantDB).UpsertABTrackingStats(ctx, statID, variantID, orgID, sends, opens, clicks, replies)
}

// GetVariantByID fetches a single variant (for use by dispatchEmailStep).
func (s *ABService) GetVariantByID(ctx context.Context, orgID, variantID string) (*entity.ABTestVariant, error) {
	tenantDB := s.getDB(orgID)
	if tenantDB == nil {
		return nil, nil
	}
	return s.trackingRepo.WithDB(tenantDB).GetABVariant(ctx, variantID)
}

// ========== Helpers ==========

// makeABTestVariant is a convenience constructor used by the handler.
func makeABTestVariant(stepID, label string, subjectOverride, bodyHTMLOverride *string, trafficPct int) *entity.ABTestVariant {
	now := time.Now().UTC()
	return &entity.ABTestVariant{
		ID:               uuid.New().String(),
		StepID:           stepID,
		VariantLabel:     label,
		SubjectOverride:  subjectOverride,
		BodyHTMLOverride: bodyHTMLOverride,
		TrafficPct:       trafficPct,
		IsWinner:         0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
