package dedup

import (
	"context"
	"fmt"
	"log"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
)

// Detector orchestrates duplicate detection
type Detector struct {
	normalizer *Normalizer
	blocker    *Blocker
	scorer     *Scorer
	ruleRepo   *repo.MatchingRuleRepo
}

// NewDetector creates a new detector
func NewDetector(ruleRepo *repo.MatchingRuleRepo, defaultRegion string) *Detector {
	normalizer := NewNormalizer(defaultRegion)
	return &Detector{
		normalizer: normalizer,
		blocker:    NewBlocker(normalizer),
		scorer:     NewScorer(normalizer),
		ruleRepo:   ruleRepo,
	}
}

// DuplicateMatch represents a detected duplicate
type DuplicateMatch struct {
	RecordID    string              `json:"recordId"`
	RecordName  string              `json:"recordName,omitempty"`
	MatchResult *entity.MatchResult `json:"matchResult"`
}

// CheckForDuplicates finds duplicates for a single record (e.g., during create)
// Returns matches above threshold, sorted by score descending
func (d *Detector) CheckForDuplicates(ctx context.Context, conn db.DBConn, orgID, entityType string,
	record map[string]interface{}, excludeID string) ([]DuplicateMatch, error) {

	// Get enabled rules for this entity
	rules, err := d.ruleRepo.WithDB(conn).ListEnabledRules(ctx, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching rules: %w", err)
	}

	if len(rules) == 0 {
		return []DuplicateMatch{}, nil // No rules configured
	}

	var allMatches []DuplicateMatch
	seenRecords := make(map[string]bool) // Dedupe results across rules

	// Process rules in priority order (first match wins per CONTEXT.md)
	for _, rule := range rules {
		log.Printf("[DETECTOR] Processing rule %s (strategy=%s) for %s/%s, excludeID=%s",
			rule.Name, rule.BlockingStrategy, entityType, getStringValue(record, "id"), excludeID)

		// Find candidate records using blocking
		candidates, err := d.blocker.FindCandidates(ctx, conn, orgID, entityType, record, excludeID, &rule)
		if err != nil {
			return nil, fmt.Errorf("failed to find candidates: %w", err)
		}

		log.Printf("[DETECTOR] Rule %s found %d candidates", rule.Name, len(candidates))

		// Score each candidate
		for _, candidateID := range candidates {
			if seenRecords[candidateID] {
				continue // Already matched by higher priority rule
			}

			// Fetch candidate record
			candidateRecord, err := d.fetchRecord(ctx, conn, entityType, candidateID, orgID)
			if err != nil {
				log.Printf("[DETECTOR] Failed to fetch candidate %s: %v", candidateID, err)
				continue // Skip on error
			}

			// Calculate score
			result, isMatch := d.scorer.CompareRecords(record, candidateRecord, &rule)
			log.Printf("[DETECTOR] Scored %s vs %s: score=%.2f, isMatch=%v",
				excludeID, candidateID, result.Score, isMatch)
			if isMatch {
				allMatches = append(allMatches, DuplicateMatch{
					RecordID:    candidateID,
					MatchResult: result,
				})
				seenRecords[candidateID] = true
			}
		}
	}

	// Sort by score descending
	sortMatchesByScore(allMatches)

	return allMatches, nil
}

// ScoreRecord scores two records against a matching rule. Exposed for bulk
// operations where candidates are already pre-fetched into memory.
func (d *Detector) ScoreRecord(recordA, recordB map[string]interface{}, rule *entity.MatchingRule) (*entity.MatchResult, bool) {
	return d.scorer.CompareRecords(recordA, recordB, rule)
}

// DetectDuplicatesInBatch scans entity for all duplicates (background job use)
// Returns pairs grouped by confidence tier
func (d *Detector) DetectDuplicatesInBatch(ctx context.Context, conn db.DBConn, orgID, entityType string,
	limit int, offset int) ([]DuplicateMatch, int, error) {

	tableName := util.GetTableName(entityType)

	// Get all records (paginated)
	query := fmt.Sprintf(`SELECT * FROM %s WHERE org_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, tableName)
	rows, err := conn.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query records: %w", err)
	}
	defer rows.Close()

	// Scan all records into maps
	records, err := util.ScanRowsToMaps(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to scan records: %w", err)
	}

	var allMatches []DuplicateMatch
	processedPairs := make(map[string]bool) // "id1:id2" to avoid duplicate pairs

	// For each record, find duplicates
	for _, record := range records {
		recordID := getStringValue(record, "id")
		if recordID == "" {
			continue
		}

		matches, err := d.CheckForDuplicates(ctx, conn, orgID, entityType, record, recordID)
		if err != nil {
			continue // Log and continue on individual errors
		}

		for _, match := range matches {
			// Create canonical pair key (smaller ID first)
			pairKey := recordID + ":" + match.RecordID
			if match.RecordID < recordID {
				pairKey = match.RecordID + ":" + recordID
			}

			if processedPairs[pairKey] {
				continue
			}
			processedPairs[pairKey] = true

			allMatches = append(allMatches, DuplicateMatch{
				RecordID:    match.RecordID,
				MatchResult: match.MatchResult,
			})
		}
	}

	return allMatches, len(records), nil
}

// fetchRecord retrieves a single record as a map
func (d *Detector) fetchRecord(ctx context.Context, conn db.DBConn, entityType, recordID, orgID string) (map[string]interface{}, error) {
	return util.FetchRecordAsMap(ctx, db.GetRawDB(conn), util.GetTableName(entityType), recordID, orgID)
}

// GetNormalizer returns the normalizer for external use
func (d *Detector) GetNormalizer() *Normalizer {
	return d.normalizer
}

// GetBlocker returns the blocker for external use (e.g., updating blocking keys)
func (d *Detector) GetBlocker() *Blocker {
	return d.blocker
}

// sortMatchesByScore sorts matches by score descending
func sortMatchesByScore(matches []DuplicateMatch) {
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].MatchResult.Score > matches[i].MatchResult.Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}
