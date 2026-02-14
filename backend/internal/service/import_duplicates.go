package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/dedup"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
)

// ImportDuplicateService handles duplicate detection for CSV import
type ImportDuplicateService struct {
	detector         *dedup.Detector
	matchingRuleRepo *repo.MatchingRuleRepo
}

// AuditEntry represents a single row's resolution for the audit report
type AuditEntry struct {
	RowIndex  int
	Action    string
	MatchedID string
	Reason    string
}

// NewImportDuplicateService creates a new ImportDuplicateService
func NewImportDuplicateService(detector *dedup.Detector, matchingRuleRepo *repo.MatchingRuleRepo) *ImportDuplicateService {
	return &ImportDuplicateService{
		detector:         detector,
		matchingRuleRepo: matchingRuleRepo,
	}
}

// CheckDuplicates checks for duplicates both against database and within the CSV file
func (s *ImportDuplicateService) CheckDuplicates(
	ctx context.Context,
	dbConn db.DBConn,
	orgID, entityType string,
	importRows []map[string]interface{},
) (*entity.DuplicateCheckResult, error) {
	// Detect database duplicates (import rows matching existing records)
	databaseMatches, err := s.detectDatabaseDuplicates(ctx, dbConn, orgID, entityType, importRows)
	if err != nil {
		return nil, fmt.Errorf("failed to detect database duplicates: %w", err)
	}

	// Detect within-file duplicates (import rows matching each other)
	withinFileGroups, err := s.detectWithinFileDuplicates(ctx, dbConn, orgID, entityType, importRows)
	if err != nil {
		return nil, fmt.Errorf("failed to detect within-file duplicates: %w", err)
	}

	// Calculate flagged rows count (unique row indices that need review)
	flaggedRows := make(map[int]bool)
	for _, match := range databaseMatches {
		flaggedRows[match.ImportRowIndex] = true
	}
	for _, group := range withinFileGroups {
		for _, rowIdx := range group.RowIndices {
			flaggedRows[rowIdx] = true
		}
	}

	return &entity.DuplicateCheckResult{
		DatabaseMatches:  databaseMatches,
		WithinFileGroups: withinFileGroups,
		TotalRows:        len(importRows),
		FlaggedRows:      len(flaggedRows),
	}, nil
}

// detectDatabaseDuplicates finds import rows that match existing database records
func (s *ImportDuplicateService) detectDatabaseDuplicates(
	ctx context.Context,
	dbConn db.DBConn,
	orgID, entityType string,
	importRows []map[string]interface{},
) ([]entity.ImportDuplicateMatch, error) {
	var matches []entity.ImportDuplicateMatch

	// Ensure blocking key columns exist and are populated for existing records.
	// Without this, the blocker's prefix/soundex/domain queries return 0 candidates
	// because existing records have NULL blocking key values.
	if err := dedup.EnsureBlockingKeysForEntity(ctx, dbConn, entityType); err != nil {
		log.Printf("[IMPORT-DEDUP] Failed to ensure blocking keys for %s: %v", entityType, err)
	}
	if err := dedup.BackfillBlockingKeysForEntity(ctx, dbConn, entityType, s.detector.GetBlocker()); err != nil {
		log.Printf("[IMPORT-DEDUP] Failed to backfill blocking keys for %s: %v", entityType, err)
	}

	// For each import row, check if it matches any existing database records
	for rowIdx, importRow := range importRows {
		// Call the detector for this row (excludeID = "" since import rows have no ID yet)
		duplicates, err := s.detector.CheckForDuplicates(ctx, dbConn, orgID, entityType, importRow, "")
		if err != nil {
			// Log error but continue processing other rows
			continue
		}

		if len(duplicates) == 0 {
			continue
		}

		// Build match candidates from all duplicates found
		var candidates []entity.ImportMatchCandidate
		for _, dup := range duplicates {
			// Fetch the matched record to get display name
			matchedRecord, err := util.FetchRecordAsMap(ctx, db.GetRawDB(dbConn), util.GetTableName(entityType), dup.RecordID, orgID)
			if err != nil {
				continue
			}

			// Extract display name from record (try "name" first, then firstName+lastName)
			displayName := s.extractRecordName(matchedRecord)

			candidates = append(candidates, entity.ImportMatchCandidate{
				RecordID: dup.RecordID,
				Name:     displayName,
				Score:    dup.MatchResult.Score,
			})
		}

		if len(candidates) == 0 {
			continue
		}

		// Sort candidates by score descending (highest score first)
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})

		// Top match is the primary match
		topMatch := duplicates[0]
		matchedRecord, err := util.FetchRecordAsMap(ctx, db.GetRawDB(dbConn), util.GetTableName(entityType), topMatch.RecordID, orgID)
		if err != nil {
			continue
		}

		// Build ImportDuplicateMatch with top match and alternatives
		match := entity.ImportDuplicateMatch{
			ImportRowIndex:  rowIdx,
			ImportRow:       importRow,
			MatchedRecordID: topMatch.RecordID,
			MatchedRecord:   matchedRecord,
			ConfidenceScore: topMatch.MatchResult.Score,
			ConfidenceTier:  topMatch.MatchResult.ConfidenceTier,
			MatchedFields:   topMatch.MatchResult.MatchingFields,
			RuleName:        topMatch.MatchResult.RuleName,
			OtherMatches:    candidates[1:], // Remaining candidates as alternatives
		}

		matches = append(matches, match)
	}

	return matches, nil
}

// detectWithinFileDuplicates finds rows within the CSV that duplicate each other
func (s *ImportDuplicateService) detectWithinFileDuplicates(
	ctx context.Context,
	dbConn db.DBConn,
	orgID, entityType string,
	importRows []map[string]interface{},
) ([]entity.ImportDuplicateGroup, error) {
	// Get enabled matching rules to determine which fields to use for hashing
	tenantRepo := s.matchingRuleRepo.WithDB(dbConn)
	rules, err := tenantRepo.ListEnabledRules(ctx, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching rules: %w", err)
	}

	if len(rules) == 0 {
		// No rules configured - no within-file detection
		return []entity.ImportDuplicateGroup{}, nil
	}

	// Build hash map: hash -> list of row indices
	hashToRows := make(map[string][]int)

	// For each import row, create a hash based on normalized match field values
	for rowIdx, row := range importRows {
		hashKey := s.computeRowHash(row, rules)
		if hashKey == "" {
			continue // Skip rows with no match fields
		}
		hashToRows[hashKey] = append(hashToRows[hashKey], rowIdx)
	}

	// Any hash with 2+ rows is a duplicate group
	var groups []entity.ImportDuplicateGroup
	for hashKey, rowIndices := range hashToRows {
		if len(rowIndices) < 2 {
			continue
		}

		// Build the group
		var rows []map[string]interface{}
		for _, idx := range rowIndices {
			rows = append(rows, importRows[idx])
		}

		group := entity.ImportDuplicateGroup{
			GroupID:    hashKey,
			RowIndices: rowIndices,
			Rows:       rows,
			KeepIndex:  rowIndices[0], // Default to first row as keeper
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// computeRowHash creates a hash key from normalized values of all match fields
func (s *ImportDuplicateService) computeRowHash(row map[string]interface{}, rules []entity.MatchingRule) string {
	// Collect all field names used in matching rules
	fieldSet := make(map[string]bool)
	for _, rule := range rules {
		for _, fieldConfig := range rule.FieldConfigs {
			fieldSet[fieldConfig.FieldName] = true
		}
	}

	// Extract and normalize field values
	var values []string
	for fieldName := range fieldSet {
		val, ok := row[fieldName]
		if !ok {
			continue
		}

		// Convert to string
		strVal := fmt.Sprintf("%v", val)
		if strVal == "" {
			continue
		}

		// Normalize: lowercase, trim whitespace
		normalized := strings.ToLower(strings.TrimSpace(strVal))

		// For phone fields, strip non-digits
		if strings.Contains(strings.ToLower(fieldName), "phone") {
			normalized = s.stripNonDigits(normalized)
		}

		values = append(values, fieldName+":"+normalized)
	}

	if len(values) == 0 {
		return ""
	}

	// Sort for consistent hash regardless of field order
	sort.Strings(values)

	// Create hash
	combined := strings.Join(values, "|")
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash)
}

// stripNonDigits removes all non-digit characters from a string
func (s *ImportDuplicateService) stripNonDigits(str string) string {
	var result strings.Builder
	for _, r := range str {
		if r >= '0' && r <= '9' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// extractRecordName extracts a display name from a record
func (s *ImportDuplicateService) extractRecordName(record map[string]interface{}) string {
	// Try "name" field first
	if name, ok := record["name"].(string); ok && name != "" {
		return name
	}

	// Try firstName + lastName
	firstName, hasFirst := record["first_name"].(string)
	lastName, hasLast := record["last_name"].(string)
	if hasFirst && hasLast {
		return strings.TrimSpace(firstName + " " + lastName)
	}
	if hasFirst {
		return firstName
	}
	if hasLast {
		return lastName
	}

	// Fallback to ID
	if id, ok := record["id"].(string); ok {
		return id
	}

	return "Unknown"
}

// GenerateAuditReport creates a CSV report of all import resolution actions
func (s *ImportDuplicateService) GenerateAuditReport(entries []AuditEntry) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	writer.Write([]string{"Row Number", "Action", "Matched Record ID", "Reason"})

	// Write entries
	for _, entry := range entries {
		writer.Write([]string{
			fmt.Sprintf("%d", entry.RowIndex+1), // Convert to 1-based for user display
			entry.Action,
			entry.MatchedID,
			entry.Reason,
		})
	}

	writer.Flush()
	return buf.Bytes()
}
