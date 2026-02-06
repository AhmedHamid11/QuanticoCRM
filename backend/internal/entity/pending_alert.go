package entity

import "time"

// PendingDuplicateAlert stores async detection results for user review
type PendingDuplicateAlert struct {
	ID                     string                `json:"id" db:"id"`
	OrgID                  string                `json:"orgId" db:"org_id"`
	EntityType             string                `json:"entityType" db:"entity_type"`
	RecordID               string                `json:"recordId" db:"record_id"`
	Matches                []DuplicateAlertMatch `json:"matches" db:"-"`
	MatchesJSON            string                `json:"-" db:"matches_json"`
	TotalMatchCount        int                   `json:"totalMatchCount" db:"total_match_count"`
	HighestConfidence      string                `json:"highestConfidence" db:"highest_confidence"`
	IsBlockMode            bool                  `json:"isBlockMode" db:"is_block_mode"`
	MergeDisplayFields     []string              `json:"mergeDisplayFields" db:"-"`     // Fields to show on merge screen
	MergeDisplayFieldsJSON string                `json:"-" db:"merge_display_fields"`   // Stored as JSON
	Status                 string                `json:"status" db:"status"`
	DetectedAt             time.Time             `json:"detectedAt" db:"detected_at"`
	ResolvedAt             *time.Time            `json:"resolvedAt,omitempty" db:"resolved_at"`
	ResolvedByID           *string               `json:"resolvedById,omitempty" db:"resolved_by_id"`
	OverrideText           *string               `json:"overrideText,omitempty" db:"override_text"`
}

// DuplicateAlertMatch is a match stored in an alert (serialized to JSON)
type DuplicateAlertMatch struct {
	RecordID    string       `json:"recordId"`
	RecordName  string       `json:"recordName,omitempty"`
	MatchResult *MatchResult `json:"matchResult"`
}

// AlertStatus constants
const (
	AlertStatusPending       = "pending"
	AlertStatusDismissed     = "dismissed"
	AlertStatusMerged        = "merged"
	AlertStatusCreatedAnyway = "created_anyway"
)
