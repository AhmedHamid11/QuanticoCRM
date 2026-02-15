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

// DuplicateCheckProgress represents the current state of a duplicate check operation
type DuplicateCheckProgress struct {
	Phase           string `json:"phase"`           // "preparing" or "checking"
	ProcessedRows   int    `json:"processedRows"`
	TotalRows       int    `json:"totalRows"`
	DuplicatesFound int    `json:"duplicatesFound"`
}

// CheckDuplicates checks for duplicates both against database and within the CSV file
func (s *ImportDuplicateService) CheckDuplicates(
	ctx context.Context,
	dbConn db.DBConn,
	orgID, entityType string,
	importRows []map[string]interface{},
) (*entity.DuplicateCheckResult, error) {
	return s.CheckDuplicatesWithProgress(ctx, dbConn, orgID, entityType, importRows, nil)
}

// CheckDuplicatesWithProgress checks for duplicates with a progress callback.
// If onProgress is nil, no progress events are emitted.
func (s *ImportDuplicateService) CheckDuplicatesWithProgress(
	ctx context.Context,
	dbConn db.DBConn,
	orgID, entityType string,
	importRows []map[string]interface{},
	onProgress func(DuplicateCheckProgress),
) (*entity.DuplicateCheckResult, error) {
	// Detect database duplicates (import rows matching existing records)
	databaseMatches, err := s.detectDatabaseDuplicatesWithProgress(ctx, dbConn, orgID, entityType, importRows, onProgress)
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

// detectDatabaseDuplicates finds import rows that match existing database records (no progress)
func (s *ImportDuplicateService) detectDatabaseDuplicates(
	ctx context.Context,
	dbConn db.DBConn,
	orgID, entityType string,
	importRows []map[string]interface{},
) ([]entity.ImportDuplicateMatch, error) {
	return s.detectDatabaseDuplicatesWithProgress(ctx, dbConn, orgID, entityType, importRows, nil)
}

// detectDatabaseDuplicatesWithProgress finds import rows that match existing database records
// and emits progress events via the onProgress callback.
//
// Performance: Uses a batch approach that replaces N individual database queries with
// a small fixed number of bulk queries, then does all scoring in memory.
// For 8724 rows this reduces from ~17,000+ DB round-trips to ~3-5.
func (s *ImportDuplicateService) detectDatabaseDuplicatesWithProgress(
	ctx context.Context,
	dbConn db.DBConn,
	orgID, entityType string,
	importRows []map[string]interface{},
	onProgress func(DuplicateCheckProgress),
) ([]entity.ImportDuplicateMatch, error) {
	var matches []entity.ImportDuplicateMatch
	totalRows := len(importRows)

	emit := func(p DuplicateCheckProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	// Emit "preparing" phase
	emit(DuplicateCheckProgress{
		Phase:     "preparing",
		TotalRows: totalRows,
	})

	// Ensure blocking key columns exist (fast after first run due to sync.Map cache).
	if err := dedup.EnsureBlockingKeysForEntity(ctx, dbConn, entityType); err != nil {
		log.Printf("[IMPORT-DEDUP] Failed to ensure blocking keys for %s: %v", entityType, err)
	}
	// NOTE: We intentionally skip BackfillBlockingKeysForEntity here. It does row-by-row
	// UPDATEs for up to 5000 records, which blocks the UI on "Preparing..." for large tables.
	// Instead, we fetch un-indexed records after the batch query and compute their blocking
	// keys in memory (see below).

	// FIX 1: Fetch matching rules ONCE (previously fetched per-row inside detector)
	tenantRepo := s.matchingRuleRepo.WithDB(dbConn)
	rules, err := tenantRepo.ListEnabledRules(ctx, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching rules: %w", err)
	}
	if len(rules) == 0 {
		emit(DuplicateCheckProgress{
			Phase: "checking", ProcessedRows: totalRows, TotalRows: totalRows,
		})
		return nil, nil
	}

	// FIX 2: Generate blocking keys for ALL import rows upfront
	blocker := s.detector.GetBlocker()
	importKeys := make([]dedup.BlockingKeys, totalRows)
	for i, row := range importRows {
		importKeys[i] = blocker.GenerateBlockingKeys(row)
	}

	// FIX 2+3: Batch-find all candidate records (one query per rule instead of N per row)
	candidateRecords := make(map[string]map[string]interface{}) // id -> record
	candidateKeys := make(map[string]dedup.BlockingKeys)        // id -> blocking keys

	for _, rule := range rules {
		candidates, err := blocker.BatchFindCandidates(ctx, dbConn, orgID, entityType, importKeys, &rule)
		if err != nil {
			log.Printf("[IMPORT-DEDUP] Batch find candidates failed for rule %s: %v", rule.Name, err)
			continue
		}
		for _, c := range candidates {
			candidateRecords[c.ID] = c.Record
			candidateKeys[c.ID] = c.Keys
		}
	}

	log.Printf("[IMPORT-DEDUP] Loaded %d indexed candidate records for %d import rows across %d rules",
		len(candidateRecords), totalRows, len(rules))

	// Fetch un-indexed records (NULL blocking keys) — these are records created before the
	// dedup feature or after a server restart before backfill ran. Instead of doing slow
	// row-by-row UPDATEs (BackfillBlockingKeysForEntity), we compute keys in-memory.
	tableName := util.GetTableName(entityType)
	unindexedRows, err := dbConn.QueryContext(ctx, fmt.Sprintf(
		`SELECT * FROM %s WHERE org_id = ? AND dedup_email_domain IS NULL LIMIT 10000`, tableName), orgID)
	if err != nil {
		if !dedup.IsSchemaError(err) {
			log.Printf("[IMPORT-DEDUP] Failed to fetch un-indexed records: %v", err)
		}
	} else {
		unindexedRecords, scanErr := util.ScanRowsToMaps(unindexedRows)
		unindexedRows.Close()
		if scanErr == nil && len(unindexedRecords) > 0 {
			log.Printf("[IMPORT-DEDUP] Found %d un-indexed records, computing blocking keys in memory", len(unindexedRecords))
			for _, rec := range unindexedRecords {
				id, _ := rec["id"].(string)
				if id == "" || candidateRecords[id] != nil {
					continue
				}
				candidateRecords[id] = rec
				candidateKeys[id] = blocker.GenerateBlockingKeys(rec)
			}
		}
	}

	log.Printf("[IMPORT-DEDUP] Total candidate pool: %d records (indexed + un-indexed)", len(candidateRecords))

	// Build blocking key index for O(1) candidate lookup per import row
	type keyIndex struct {
		soundex map[string][]string
		prefix  map[string][]string
		domain  map[string][]string
		phone   map[string][]string
	}
	idx := keyIndex{
		soundex: make(map[string][]string),
		prefix:  make(map[string][]string),
		domain:  make(map[string][]string),
		phone:   make(map[string][]string),
	}
	for id, keys := range candidateKeys {
		if keys.LastNameSoundex != "" {
			idx.soundex[keys.LastNameSoundex] = append(idx.soundex[keys.LastNameSoundex], id)
		}
		if keys.LastNamePrefix != "" {
			idx.prefix[keys.LastNamePrefix] = append(idx.prefix[keys.LastNamePrefix], id)
		}
		if keys.EmailDomain != "" {
			idx.domain[keys.EmailDomain] = append(idx.domain[keys.EmailDomain], id)
		}
		if keys.PhoneE164 != "" {
			idx.phone[keys.PhoneE164] = append(idx.phone[keys.PhoneE164], id)
		}
	}

	// findCandidateIDs returns candidate IDs matching a row's blocking keys for a rule
	findCandidateIDs := func(keys dedup.BlockingKeys, rule *entity.MatchingRule) []string {
		seen := make(map[string]bool)
		var result []string
		addFrom := func(m map[string][]string, key string) {
			if key == "" {
				return
			}
			for _, id := range m[key] {
				if !seen[id] {
					seen[id] = true
					result = append(result, id)
				}
			}
		}
		switch rule.BlockingStrategy {
		case entity.BlockingSoundex:
			addFrom(idx.soundex, keys.LastNameSoundex)
		case entity.BlockingPrefix:
			addFrom(idx.prefix, keys.LastNamePrefix)
		case entity.BlockingExact:
			addFrom(idx.domain, keys.EmailDomain)
			addFrom(idx.phone, keys.PhoneE164)
		case entity.BlockingMulti:
			addFrom(idx.soundex, keys.LastNameSoundex)
			addFrom(idx.prefix, keys.LastNamePrefix)
			addFrom(idx.domain, keys.EmailDomain)
		default:
			addFrom(idx.soundex, keys.LastNameSoundex)
			addFrom(idx.prefix, keys.LastNamePrefix)
			addFrom(idx.domain, keys.EmailDomain)
		}
		return result
	}

	// Throttle: emit progress every N rows to cap at ~100 events
	emitEvery := totalRows / 100
	if emitEvery < 1 {
		emitEvery = 1
	}

	// Score each import row against relevant candidates (in-memory, no DB queries)
	type scoredMatch struct {
		recordID    string
		record      map[string]interface{}
		matchResult *entity.MatchResult
	}

	for rowIdx, importRow := range importRows {
		// Emit progress (throttled)
		if rowIdx%emitEvery == 0 || rowIdx == totalRows-1 {
			emit(DuplicateCheckProgress{
				Phase:           "checking",
				ProcessedRows:   rowIdx,
				TotalRows:       totalRows,
				DuplicatesFound: len(matches),
			})
		}

		rowKeys := importKeys[rowIdx]
		seenCandidates := make(map[string]bool)
		var rowMatches []scoredMatch

		for ruleIdx := range rules {
			rule := &rules[ruleIdx]
			candidateIDs := findCandidateIDs(rowKeys, rule)

			for _, candidateID := range candidateIDs {
				if seenCandidates[candidateID] {
					continue
				}
				seenCandidates[candidateID] = true

				candidateRecord := candidateRecords[candidateID]
				result, isMatch := s.detector.ScoreRecord(importRow, candidateRecord, rule)
				if isMatch {
					rowMatches = append(rowMatches, scoredMatch{
						recordID:    candidateID,
						record:      candidateRecord,
						matchResult: result,
					})
				}
			}
		}

		if len(rowMatches) == 0 {
			continue
		}

		// Sort matches by score descending
		sort.Slice(rowMatches, func(i, j int) bool {
			return rowMatches[i].matchResult.Score > rowMatches[j].matchResult.Score
		})

		// Build OtherMatches from non-top matches
		var otherCandidates []entity.ImportMatchCandidate
		for _, m := range rowMatches[1:] {
			otherCandidates = append(otherCandidates, entity.ImportMatchCandidate{
				RecordID: m.recordID,
				Name:     s.extractRecordName(m.record),
				Score:    m.matchResult.Score,
			})
		}

		// FIX 4: Use pre-fetched record directly (eliminates redundant FetchRecordAsMap)
		topMatch := rowMatches[0]
		match := entity.ImportDuplicateMatch{
			ImportRowIndex:  rowIdx,
			ImportRow:       importRow,
			MatchedRecordID: topMatch.recordID,
			MatchedRecord:   topMatch.record,
			ConfidenceScore: topMatch.matchResult.Score,
			ConfidenceTier:  topMatch.matchResult.ConfidenceTier,
			MatchedFields:   topMatch.matchResult.MatchingFields,
			RuleName:        topMatch.matchResult.RuleName,
			OtherMatches:    otherCandidates,
		}

		matches = append(matches, match)
	}

	// Emit final progress
	emit(DuplicateCheckProgress{
		Phase:           "checking",
		ProcessedRows:   totalRows,
		TotalRows:       totalRows,
		DuplicatesFound: len(matches),
	})

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
