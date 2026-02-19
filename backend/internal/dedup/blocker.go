package dedup

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/util"
	"github.com/tilotech/go-phonetics"
)

// Blocker generates blocking keys and finds candidate pairs
type Blocker struct {
	normalizer *Normalizer
}

// NewBlocker creates a new blocker
func NewBlocker(normalizer *Normalizer) *Blocker {
	return &Blocker{normalizer: normalizer}
}

// BlockingKeys holds pre-computed blocking index values for a record
type BlockingKeys struct {
	LastNameSoundex string
	LastNamePrefix  string
	EmailDomain     string
	PhoneE164       string
}

// GenerateBlockingKeys computes all blocking keys for a record.
// Handles both Contact-style fields (lastName) and Account-style fields (name).
func (b *Blocker) GenerateBlockingKeys(record map[string]interface{}) BlockingKeys {
	keys := BlockingKeys{}

	// Name-based keys: try lastName (Contacts), fall back to name (Accounts, etc.)
	nameForBlocking := getStringValue(record, "lastName")
	if nameForBlocking == "" {
		nameForBlocking = getStringValue(record, "name")
	}
	if nameForBlocking != "" {
		keys.LastNameSoundex = b.GetSoundex(nameForBlocking)
		keys.LastNamePrefix = b.normalizer.GetNamePrefix(nameForBlocking, 3)
	}

	// Email domain - check both "email" and "emailAddress" field names
	email := getStringValue(record, "email")
	if email == "" {
		email = getStringValue(record, "emailAddress")
	}
	if email != "" {
		keys.EmailDomain = b.normalizer.ExtractEmailDomain(email)
	}

	// Phone E.164 - check both "phone" and "phoneNumber" field names
	phone := getStringValue(record, "phone")
	if phone == "" {
		phone = getStringValue(record, "phoneNumber")
	}
	if phone != "" {
		keys.PhoneE164 = b.normalizer.NormalizePhone(phone)
	}

	return keys
}

// GetSoundex returns Soundex encoding for a name
func (b *Blocker) GetSoundex(name string) string {
	name = b.normalizer.NormalizeText(name)
	if name == "" {
		return ""
	}
	return phonetics.EncodeSoundex(name)
}

// FindCandidates returns record IDs that share blocking keys with the input record.
// Always uses all available blocking keys with OR logic for maximum recall.
// The blocking strategy field is ignored — candidate narrowing is automatic.
func (b *Blocker) FindCandidates(ctx context.Context, conn db.DBConn, orgID, entityType string,
	record map[string]interface{}, excludeID string, rule *entity.MatchingRule) ([]string, error) {

	tableName := util.GetTableName(entityType)
	keys := b.GenerateBlockingKeys(record)

	// Use all available blocking keys with OR logic
	var conditions []string
	var args []interface{}

	if keys.LastNameSoundex != "" {
		conditions = append(conditions, "dedup_last_name_soundex = ?")
		args = append(args, keys.LastNameSoundex)
	}
	if keys.LastNamePrefix != "" {
		conditions = append(conditions, "dedup_last_name_prefix = ?")
		args = append(args, keys.LastNamePrefix)
	}
	if keys.EmailDomain != "" {
		conditions = append(conditions, "dedup_email_domain = ?")
		args = append(args, keys.EmailDomain)
	}
	if keys.PhoneE164 != "" {
		conditions = append(conditions, "dedup_phone_e164 = ?")
		args = append(args, keys.PhoneE164)
	}

	// If no blocking conditions, return empty (avoid full table scan)
	if len(conditions) == 0 {
		return []string{}, nil
	}

	// Build query
	query := fmt.Sprintf(`SELECT id FROM %s WHERE org_id = ?`, tableName)
	queryArgs := []interface{}{orgID}

	// Add blocking conditions with OR
	query += " AND (" + strings.Join(conditions, " OR ") + ")"
	queryArgs = append(queryArgs, args...)

	// Exclude self
	if excludeID != "" {
		query += " AND id != ?"
		queryArgs = append(queryArgs, excludeID)
	}

	// Limit to prevent huge result sets (soft limit per CONTEXT.md)
	query += " LIMIT 1000"

	rows, err := conn.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to find candidates: %w", err)
	}
	defer rows.Close()

	var candidates []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan candidate: %w", err)
		}
		candidates = append(candidates, id)
	}

	return candidates, nil
}

// CandidateRecord holds a pre-fetched candidate record with its blocking keys
type CandidateRecord struct {
	ID     string
	Record map[string]interface{}
	Keys   BlockingKeys
}

// BatchFindCandidates retrieves all existing records matching any blocking key
// from the import rows in a single query. This replaces N individual FindCandidates
// calls with one bulk query, reducing database round-trips from O(N) to O(1).
func (b *Blocker) BatchFindCandidates(ctx context.Context, conn db.DBConn, orgID, entityType string,
	importKeys []BlockingKeys, rule *entity.MatchingRule) ([]CandidateRecord, error) {

	tableName := util.GetTableName(entityType)

	// Collect unique blocking key values across all import rows
	soundexVals := make(map[string]bool)
	prefixVals := make(map[string]bool)
	domainVals := make(map[string]bool)
	phoneVals := make(map[string]bool)

	for _, keys := range importKeys {
		if keys.LastNameSoundex != "" {
			soundexVals[keys.LastNameSoundex] = true
		}
		if keys.LastNamePrefix != "" {
			prefixVals[keys.LastNamePrefix] = true
		}
		if keys.EmailDomain != "" {
			domainVals[keys.EmailDomain] = true
		}
		if keys.PhoneE164 != "" {
			phoneVals[keys.PhoneE164] = true
		}
	}

	// Use all available blocking keys (strategy is ignored)
	var conditions []string
	var args []interface{}

	addInCondition := func(col string, vals map[string]bool) {
		if len(vals) == 0 {
			return
		}
		placeholders := make([]string, 0, len(vals))
		for v := range vals {
			placeholders = append(placeholders, "?")
			args = append(args, v)
		}
		conditions = append(conditions, fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ",")))
	}

	addInCondition("dedup_last_name_soundex", soundexVals)
	addInCondition("dedup_last_name_prefix", prefixVals)
	addInCondition("dedup_email_domain", domainVals)
	addInCondition("dedup_phone_e164", phoneVals)

	if len(conditions) == 0 {
		return nil, nil
	}

	// One query for ALL candidates instead of N individual queries
	query := fmt.Sprintf(`SELECT * FROM %s WHERE org_id = ? AND (%s) LIMIT 10000`,
		tableName, strings.Join(conditions, " OR "))
	queryArgs := append([]interface{}{orgID}, args...)

	rows, err := conn.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("batch find candidates: %w", err)
	}
	defer rows.Close()

	records, err := util.ScanRowsToMaps(rows)
	if err != nil {
		return nil, fmt.Errorf("batch scan candidates: %w", err)
	}

	// Build CandidateRecords with blocking keys read directly from dedup columns
	candidates := make([]CandidateRecord, 0, len(records))
	for _, rec := range records {
		id := getStringValue(rec, "id")
		if id == "" {
			continue
		}
		candidates = append(candidates, CandidateRecord{
			ID:     id,
			Record: rec,
			Keys: BlockingKeys{
				LastNameSoundex: getStringValue(rec, "dedupLastNameSoundex"),
				LastNamePrefix:  getStringValue(rec, "dedupLastNamePrefix"),
				EmailDomain:     getStringValue(rec, "dedupEmailDomain"),
				PhoneE164:       getStringValue(rec, "dedupPhoneE164"),
			},
		})
	}

	log.Printf("[BLOCKER] BatchFindCandidates: strategy=%s, %d conditions -> %d candidates for %s",
		rule.BlockingStrategy, len(conditions), len(candidates), entityType)

	return candidates, nil
}

// UpdateBlockingKeys updates the blocking index columns for a record
// Call this after record creation/update
func (b *Blocker) UpdateBlockingKeys(ctx context.Context, conn db.DBConn, entityType, recordID string,
	record map[string]interface{}) error {

	tableName := util.GetTableName(entityType)
	keys := b.GenerateBlockingKeys(record)

	query := fmt.Sprintf(`UPDATE %s SET
		dedup_last_name_soundex = ?,
		dedup_last_name_prefix = ?,
		dedup_email_domain = ?,
		dedup_phone_e164 = ?
		WHERE id = ?`, tableName)

	_, err := conn.ExecContext(ctx, query,
		keys.LastNameSoundex, keys.LastNamePrefix, keys.EmailDomain, keys.PhoneE164, recordID)

	if err != nil {
		return fmt.Errorf("failed to update blocking keys: %w", err)
	}
	return nil
}
