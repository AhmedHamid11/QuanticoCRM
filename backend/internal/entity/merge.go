package entity

import "time"

// MergeSnapshot stores pre-merge state for undo capability
type MergeSnapshot struct {
	ID                 string    `json:"id" db:"id"`
	OrgID              string    `json:"orgId" db:"org_id"`
	EntityType         string    `json:"entityType" db:"entity_type"`
	SurvivorID         string    `json:"survivorId" db:"survivor_id"`
	SurvivorBefore     string    `json:"survivorBefore" db:"survivor_before"`         // JSON: full record state before merge
	DuplicateIDs       string    `json:"duplicateIds" db:"duplicate_ids"`             // JSON: ["dup1", "dup2"]
	DuplicateSnapshots string    `json:"duplicateSnapshots" db:"duplicate_snapshots"` // JSON: [{full record}, ...]
	RelatedRecordFKs   string    `json:"relatedRecordFks" db:"related_record_fks"`    // JSON: {"entity": [{"recordId","fkField","oldValue"}]}
	MergedByID         string    `json:"mergedById" db:"merged_by_id"`
	ConsumedAt         *string   `json:"consumedAt,omitempty" db:"consumed_at"` // Set when undo performed
	CreatedAt          time.Time `json:"createdAt" db:"created_at"`
	ExpiresAt          time.Time `json:"expiresAt" db:"expires_at"`
}

// MergeRequest is the API request body for executing a merge
type MergeRequest struct {
	SurvivorID   string                 `json:"survivorId"`
	DuplicateIDs []string               `json:"duplicateIds"`
	MergedFields map[string]interface{} `json:"mergedFields"` // field name -> chosen value
	EntityType   string                 `json:"entityType"`
}

// MergePreviewRequest is the API request body for previewing a merge
type MergePreviewRequest struct {
	RecordIDs  []string `json:"recordIds"`
	EntityType string   `json:"entityType"`
}

// MergeResult is the API response after executing a merge
type MergeResult struct {
	SurvivorID string `json:"survivorId"`
	SnapshotID string `json:"snapshotId"`
	MergedAt   string `json:"mergedAt"`
}

// MergePreview is the API response for previewing a merge
type MergePreview struct {
	Records             []map[string]interface{} `json:"records"`
	SuggestedSurvivorID string                   `json:"suggestedSurvivorId"`
	CompletenessScores  map[string]float64       `json:"completenessScores"` // recordID -> 0.0-1.0
	RelatedRecordCounts []RelatedRecordCount     `json:"relatedRecordCounts"`
	Fields              []FieldDef               `json:"fields"`
}

// RelatedRecordCount shows how many related records each duplicate has
type RelatedRecordCount struct {
	EntityType  string                   `json:"entityType"`
	EntityLabel string                   `json:"entityLabel"`
	RecordID    string                   `json:"recordId"` // which record these belong to
	Count       int                      `json:"count"`
	Records     []map[string]interface{} `json:"records,omitempty"` // actual related records (for expandable preview)
}

// RelatedRecordGroup groups related records by entity type and FK field
type RelatedRecordGroup struct {
	EntityType string          `json:"entityType"`
	FKField    string          `json:"fkField"`
	Records    []RelatedRecord `json:"records"`
}

// RelatedRecord represents a single record with an FK reference to a merge candidate
type RelatedRecord struct {
	ID         string `json:"id"`
	EntityType string `json:"entityType"`
	FKField    string `json:"fkField"`
	FKValue    string `json:"fkValue"` // current FK value (the duplicate's ID)
	OrgID      string `json:"orgId"`
}

// FKChange tracks a single FK change made during merge (for undo)
type FKChange struct {
	RecordID string `json:"recordId"`
	FKField  string `json:"fkField"`
	OldValue string `json:"oldValue"` // original FK value (duplicate's ID)
}

// MergeHistoryEntry represents a merge operation in the history list
type MergeHistoryEntry struct {
	SnapshotID   string    `json:"snapshotId"`
	EntityType   string    `json:"entityType"`
	SurvivorID   string    `json:"survivorId"`
	DuplicateIDs []string  `json:"duplicateIds"`
	MergedByID   string    `json:"mergedById"`
	CanUndo      bool      `json:"canUndo"`
	CreatedAt    time.Time `json:"createdAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
}
