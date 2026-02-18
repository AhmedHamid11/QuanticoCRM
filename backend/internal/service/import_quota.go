package service

import (
	"context"
	"fmt"
	"log"

	"github.com/fastcrm/backend/internal/turso"
)

// ImportQuotaService estimates import cost and checks Turso usage quota
type ImportQuotaService struct {
	tursoClient *turso.Client
}

// NewImportQuotaService creates a new quota service.
// tursoClient can be nil if Turso API is not configured (quota checks will be skipped).
func NewImportQuotaService(tursoClient *turso.Client) *ImportQuotaService {
	return &ImportQuotaService{tursoClient: tursoClient}
}

// PreflightResult contains the import cost estimate and usage info
type PreflightResult struct {
	EstimatedReads   int64   `json:"estimatedReads"`
	CurrentUsage     int64   `json:"currentUsage"`
	MonthlyLimit     int64   `json:"monthlyLimit"`
	RemainingBudget  int64   `json:"remainingBudget"`
	UsagePercent     float64 `json:"usagePercent"`
	WouldExceed      bool    `json:"wouldExceed"`
	Warning          string  `json:"warning,omitempty"`
	QuotaUnavailable bool    `json:"quotaUnavailable,omitempty"`
}

const (
	// TursoFreeRowReads is the free-tier monthly row read limit (512M since Feb 2025)
	TursoFreeRowReads int64 = 512_000_000

	// readsPerLookup estimates rows read per lookup field resolution (with indexes)
	readsPerLookupWithIndex int64 = 5
	// readsPerLookup estimates rows read per lookup without index (full scan)
	readsPerLookupWithoutIndex int64 = 10_000

	// readsPerBatchQuery estimates overhead per batch query
	readsPerBatchQuery int64 = 50

	// readsPerInsert estimates rows read per insert (index updates, etc.)
	readsPerInsert int64 = 2
)

// EstimateImportCost estimates the Turso row reads for an import operation.
// lookupFieldCount is the number of lookup fields that need resolution.
// hasIndexes indicates whether COLLATE NOCASE indexes are present.
func EstimateImportCost(recordCount int, lookupFieldCount int, hasIndexes bool) int64 {
	rc := int64(recordCount)
	lfc := int64(lookupFieldCount)

	var lookupReads int64
	if hasIndexes {
		// With batching + indexes: ~(records/500) batch queries, each reading a few rows
		batchCount := (rc + 499) / 500
		lookupReads = batchCount * readsPerBatchQuery * lfc
	} else {
		// Without indexes: each batch query scans the full table
		lookupReads = rc * readsPerLookupWithoutIndex * lfc
	}

	// Insert reads (index maintenance)
	insertReads := rc * readsPerInsert

	return lookupReads + insertReads
}

// CheckQuota performs a pre-flight usage check against Turso's API.
func (s *ImportQuotaService) CheckQuota(ctx context.Context, recordCount int, lookupFieldCount int) (*PreflightResult, error) {
	estimated := EstimateImportCost(recordCount, lookupFieldCount, true) // assume indexes after migration

	result := &PreflightResult{
		EstimatedReads: estimated,
		MonthlyLimit:   TursoFreeRowReads,
	}

	if s.tursoClient == nil {
		result.QuotaUnavailable = true
		result.Warning = "Turso API not configured — cannot verify usage quota"
		result.RemainingBudget = TursoFreeRowReads
		return result, nil
	}

	usage, err := s.tursoClient.GetUsage(ctx)
	if err != nil {
		log.Printf("[PREFLIGHT] Turso usage fetch failed: %v", err)
		result.QuotaUnavailable = true
		result.Warning = "Could not verify Turso quota — import will proceed without quota check"
		result.RemainingBudget = TursoFreeRowReads
		return result, nil
	}

	result.CurrentUsage = usage.Usage.RowsRead
	result.RemainingBudget = TursoFreeRowReads - usage.Usage.RowsRead
	if result.RemainingBudget < 0 {
		result.RemainingBudget = 0
	}
	result.UsagePercent = float64(usage.Usage.RowsRead) / float64(TursoFreeRowReads) * 100

	if estimated > result.RemainingBudget {
		result.WouldExceed = true
		result.Warning = fmt.Sprintf(
			"This import may use ~%dM row reads, but only %dM remain in your monthly Turso budget (%.0f%% used). Consider splitting the import into smaller batches.",
			estimated/1_000_000,
			result.RemainingBudget/1_000_000,
			result.UsagePercent,
		)
	} else if result.UsagePercent > 80 {
		result.Warning = fmt.Sprintf(
			"Your Turso row read usage is at %.0f%%. This import will use an estimated %dK additional reads.",
			result.UsagePercent,
			estimated/1_000,
		)
	}

	return result, nil
}
