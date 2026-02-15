package entity

// ImportJob represents a CSV import job with counts and metadata
type ImportJob struct {
	ID              string `json:"id" db:"id"`
	OrgID           string `json:"orgId" db:"org_id"`
	EntityType      string `json:"entityType" db:"entity_type"`
	ExternalIdField string `json:"externalIdField,omitempty" db:"external_id_field"`
	TotalRows       int    `json:"totalRows" db:"total_rows"`
	CreatedCount    int    `json:"createdCount" db:"created_count"`
	UpdatedCount    int    `json:"updatedCount" db:"updated_count"`
	SkippedCount    int    `json:"skippedCount" db:"skipped_count"`
	MergedCount     int    `json:"mergedCount" db:"merged_count"`
	FailedCount     int    `json:"failedCount" db:"failed_count"`
	CreatedAt       string `json:"createdAt" db:"created_at"`
}

// ImportDedupDecision represents a single dedup decision made during import
type ImportDedupDecision struct {
	ID                  string `json:"id" db:"id"`
	OrgID               string `json:"orgId" db:"org_id"`
	ImportJobID         string `json:"importJobId" db:"import_job_id"`
	DecisionType        string `json:"decisionType" db:"decision_type"`
	Action              string `json:"action" db:"action"`
	KeptExternalID      string `json:"keptExternalId,omitempty" db:"kept_external_id"`
	DiscardedExternalID string `json:"discardedExternalId,omitempty" db:"discarded_external_id"`
	MatchField          string `json:"matchField,omitempty" db:"match_field"`
	MatchValue          string `json:"matchValue,omitempty" db:"match_value"`
	MatchedRecordID     string `json:"matchedRecordId,omitempty" db:"matched_record_id"`
	CreatedAt           string `json:"createdAt" db:"created_at"`
}
