package entity

// ImportDuplicateMatch represents an import row that matched an existing database record
type ImportDuplicateMatch struct {
	ImportRowIndex  int                    `json:"importRowIndex"`  // 0-based row index in CSV
	ImportRow       map[string]interface{} `json:"importRow"`       // The import row data
	MatchedRecordID string                 `json:"matchedRecordId"` // ID of the matched existing record
	MatchedRecord   map[string]interface{} `json:"matchedRecord"`   // The matched record data
	ConfidenceScore float64                `json:"confidenceScore"` // 0.0-1.0 overall match score
	ConfidenceTier  string                 `json:"confidenceTier"`  // "high", "medium", "low"
	MatchedFields   []string               `json:"matchedFields"`   // Which fields matched (for UI highlighting)
	RuleName        string                 `json:"ruleName"`        // Which matching rule produced this match
	OtherMatches    []ImportMatchCandidate `json:"otherMatches,omitempty"` // Additional lower-score matches
}

// ImportMatchCandidate represents an alternative match for a row (not the top match)
type ImportMatchCandidate struct {
	RecordID string  `json:"recordId"`
	Name     string  `json:"name"` // Display name for the record
	Score    float64 `json:"score"`
}

// ImportDuplicateGroup represents rows within the CSV that duplicate each other
type ImportDuplicateGroup struct {
	GroupID    string                   `json:"groupId"`    // Unique group identifier (hash)
	RowIndices []int                    `json:"rowIndices"` // 0-based row indices of duplicate rows
	Rows       []map[string]interface{} `json:"rows"`       // The duplicate row data
	KeepIndex  int                      `json:"keepIndex"`  // Default: first row index (auto-selected)
}

// DuplicateCheckResult is the combined result of database + within-file detection
type DuplicateCheckResult struct {
	DatabaseMatches  []ImportDuplicateMatch `json:"databaseMatches"`
	WithinFileGroups []ImportDuplicateGroup `json:"withinFileGroups"`
	TotalRows        int                    `json:"totalRows"`        // Total rows in the CSV
	FlaggedRows      int                    `json:"flaggedRows"`      // Rows that need review
}

// ImportResolution represents the user's decision for a flagged row
type ImportResolution struct {
	Action          string `json:"action"`                    // "skip", "update", "import", "merge"
	SelectedMatchID string `json:"selectedMatchId,omitempty"` // Which match to act on (for update/merge)
}
