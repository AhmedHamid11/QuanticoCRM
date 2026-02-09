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

// GenerateBlockingKeys computes all blocking keys for a record
func (b *Blocker) GenerateBlockingKeys(record map[string]interface{}) BlockingKeys {
	keys := BlockingKeys{}

	// Last name Soundex and prefix
	if lastName := getStringValue(record, "lastName"); lastName != "" {
		keys.LastNameSoundex = b.GetSoundex(lastName)
		keys.LastNamePrefix = b.normalizer.GetNamePrefix(lastName, 3)
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

// FindCandidates returns record IDs that share blocking keys with the input record
// Uses OR logic across strategies: any shared key = candidate
func (b *Blocker) FindCandidates(ctx context.Context, conn db.DBConn, orgID, entityType string,
	record map[string]interface{}, excludeID string, rule *entity.MatchingRule) ([]string, error) {

	tableName := util.GetTableName(entityType)
	keys := b.GenerateBlockingKeys(record)

	// Build query based on blocking strategy
	var conditions []string
	var args []interface{}

	switch rule.BlockingStrategy {
	case entity.BlockingSoundex:
		if keys.LastNameSoundex != "" {
			conditions = append(conditions, "dedup_last_name_soundex = ?")
			args = append(args, keys.LastNameSoundex)
		}
	case entity.BlockingPrefix:
		if keys.LastNamePrefix != "" {
			conditions = append(conditions, "dedup_last_name_prefix = ?")
			args = append(args, keys.LastNamePrefix)
		}
	case entity.BlockingExact:
		// Exact matching on specific fields - check email and phone
		if keys.EmailDomain != "" {
			conditions = append(conditions, "dedup_email_domain = ?")
			args = append(args, keys.EmailDomain)
		}
		if keys.PhoneE164 != "" {
			conditions = append(conditions, "dedup_phone_e164 = ?")
			args = append(args, keys.PhoneE164)
		}
	case entity.BlockingMulti:
		// Combine multiple strategies with OR
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
	default:
		// Default to multi (combine all strategies) for unknown/empty strategies
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
	}

	// If no blocking conditions, return empty (avoid full table scan)
	if len(conditions) == 0 {
		log.Printf("[BLOCKER] No blocking conditions for %s strategy (keys: soundex=%q prefix=%q domain=%q phone=%q)",
			rule.BlockingStrategy, keys.LastNameSoundex, keys.LastNamePrefix, keys.EmailDomain, keys.PhoneE164)
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

	log.Printf("[BLOCKER] Strategy=%s, query conditions: %s, args: %v, excludeID=%s",
		rule.BlockingStrategy, strings.Join(conditions, " OR "), args, excludeID)

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

	log.Printf("[BLOCKER] Found %d candidates for %s/%s", len(candidates), entityType, excludeID)
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
