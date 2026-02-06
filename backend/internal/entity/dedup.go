package entity

import "time"

// FieldConfig defines how a single field is compared in duplicate detection
type FieldConfig struct {
	FieldName       string  `json:"fieldName"`                 // Field on source entity
	TargetFieldName string  `json:"targetFieldName,omitempty"` // Field on target entity (cross-entity)
	Weight          float64 `json:"weight"`                    // 0-100, contribution to total score
	Algorithm       string  `json:"algorithm"`                 // "exact", "jaro_winkler", "email", "phone", "phonetic"
	Threshold       float64 `json:"threshold,omitempty"`       // Per-field threshold (default: 0.88 for fuzzy)
	ExactMatchBoost bool    `json:"exactMatchBoost,omitempty"` // Auto-high confidence on exact match
}

// MatchingRule defines criteria for detecting duplicates
type MatchingRule struct {
	ID                        string        `json:"id" db:"id"`
	OrgID                     string        `json:"orgId" db:"org_id"`
	Name                      string        `json:"name" db:"name"`
	Description               string        `json:"description,omitempty" db:"description"`
	EntityType                string        `json:"entityType" db:"entity_type"`
	TargetEntityType          *string       `json:"targetEntityType,omitempty" db:"target_entity_type"`
	IsEnabled                 bool          `json:"isEnabled" db:"is_enabled"`
	Priority                  int           `json:"priority" db:"priority"`
	Threshold                 float64       `json:"threshold" db:"threshold"`
	HighConfidenceThreshold   float64       `json:"highConfidenceThreshold" db:"high_confidence_threshold"`
	MediumConfidenceThreshold float64       `json:"mediumConfidenceThreshold" db:"medium_confidence_threshold"`
	BlockingStrategy          string        `json:"blockingStrategy" db:"blocking_strategy"`
	FieldConfigs              []FieldConfig `json:"fieldConfigs" db:"-"`            // Loaded from JSON
	FieldConfigsJSON          string        `json:"-" db:"field_configs"`           // Stored as JSON
	CreatedAt                 time.Time     `json:"createdAt" db:"created_at"`
	ModifiedAt                time.Time     `json:"modifiedAt" db:"modified_at"`
}

// MatchResult represents the outcome of comparing two records
type MatchResult struct {
	Score          float64            `json:"score"`          // 0.0-1.0 overall score
	ConfidenceTier string             `json:"confidenceTier"` // "high", "medium", "low"
	FieldScores    map[string]float64 `json:"fieldScores"`    // Per-field breakdown
	MatchingFields []string           `json:"matchingFields"` // Fields that contributed
	RuleID         string             `json:"ruleId"`         // Which rule matched
	RuleName       string             `json:"ruleName"`
}

// DuplicatePair represents a detected duplicate pair pending review
type DuplicatePair struct {
	ID              string      `json:"id" db:"id"`
	OrgID           string      `json:"orgId" db:"org_id"`
	EntityType      string      `json:"entityType" db:"entity_type"`
	RecordID1       string      `json:"recordId1" db:"record_id_1"`
	RecordID2       string      `json:"recordId2" db:"record_id_2"`
	MatchResult     MatchResult `json:"matchResult" db:"-"`
	MatchResultJSON string      `json:"-" db:"match_result"`
	Status          string      `json:"status" db:"status"` // "pending", "merged", "dismissed"
	DetectedAt      time.Time   `json:"detectedAt" db:"detected_at"`
	ResolvedAt      *time.Time  `json:"resolvedAt,omitempty" db:"resolved_at"`
	ResolvedByID    *string     `json:"resolvedById,omitempty" db:"resolved_by_id"`
}

// MatchingRuleCreateInput for creating new rules
type MatchingRuleCreateInput struct {
	Name                      string        `json:"name" validate:"required"`
	Description               string        `json:"description,omitempty"`
	EntityType                string        `json:"entityType" validate:"required"`
	TargetEntityType          *string       `json:"targetEntityType,omitempty"`
	IsEnabled                 bool          `json:"isEnabled"`
	Priority                  int           `json:"priority"`
	Threshold                 float64       `json:"threshold" validate:"required,min=0,max=1"`
	HighConfidenceThreshold   float64       `json:"highConfidenceThreshold"`
	MediumConfidenceThreshold float64       `json:"mediumConfidenceThreshold"`
	BlockingStrategy          string        `json:"blockingStrategy" validate:"required"`
	FieldConfigs              []FieldConfig `json:"fieldConfigs" validate:"required,min=1"`
}

// MatchingRuleUpdateInput for updating existing rules
type MatchingRuleUpdateInput struct {
	Name                      *string        `json:"name,omitempty"`
	Description               *string        `json:"description,omitempty"`
	IsEnabled                 *bool          `json:"isEnabled,omitempty"`
	Priority                  *int           `json:"priority,omitempty"`
	Threshold                 *float64       `json:"threshold,omitempty"`
	HighConfidenceThreshold   *float64       `json:"highConfidenceThreshold,omitempty"`
	MediumConfidenceThreshold *float64       `json:"mediumConfidenceThreshold,omitempty"`
	BlockingStrategy          *string        `json:"blockingStrategy,omitempty"`
	FieldConfigs              []FieldConfig  `json:"fieldConfigs,omitempty"`
}

// BlockingStrategy constants
const (
	BlockingSoundex = "soundex"
	BlockingPrefix  = "prefix"
	BlockingExact   = "exact"
	BlockingNgram   = "ngram"
	BlockingMulti   = "multi" // Combines multiple strategies
)

// ComparisonAlgorithm constants
const (
	AlgorithmExact       = "exact"
	AlgorithmJaroWinkler = "jaro_winkler"
	AlgorithmEmail       = "email"
	AlgorithmPhone       = "phone"
	AlgorithmPhonetic    = "phonetic"
)

// ConfidenceTier constants
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)
