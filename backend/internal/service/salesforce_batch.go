package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/fastcrm/backend/internal/entity"
)

// BatchAssembler groups merge instructions into batches with unique IDs
type BatchAssembler struct {
	maxBatchSize int
}

// NewBatchAssembler creates a new BatchAssembler with default batch size (200)
func NewBatchAssembler() *BatchAssembler {
	return &BatchAssembler{maxBatchSize: 200}
}

// NewBatchAssemblerWithSize creates a new BatchAssembler with custom batch size
func NewBatchAssemblerWithSize(maxSize int) *BatchAssembler {
	return &BatchAssembler{maxBatchSize: maxSize}
}

// AssembleBatches splits instructions into batches with unique batch IDs
func (a *BatchAssembler) AssembleBatches(
	sfOrgID string,
	instructions []entity.MergeInstruction,
) ([]entity.MergeInstructionBatch, error) {
	if len(instructions) == 0 {
		return nil, fmt.Errorf("cannot assemble batches: instructions array is empty")
	}

	if sfOrgID == "" {
		return nil, fmt.Errorf("sfOrgID cannot be empty")
	}

	// 1. Generate batch_id prefix
	prefix := fmt.Sprintf("QTC-%s", time.Now().UTC().Format("20060102"))

	// 2. Split instructions into chunks of maxBatchSize
	batches := make([]entity.MergeInstructionBatch, 0)
	for i := 0; i < len(instructions); i += a.maxBatchSize {
		end := i + a.maxBatchSize
		if end > len(instructions) {
			end = len(instructions)
		}

		chunk := instructions[i:end]
		batchIndex := i / a.maxBatchSize

		// 3. Generate unique batch_id
		batchID := fmt.Sprintf("%s-%03d", prefix, batchIndex+1)

		// 4. Generate timestamp
		timestamp := time.Now().UTC().Format(time.RFC3339)

		// 5. Create batch
		batch := entity.MergeInstructionBatch{
			BatchID:           batchID,
			Timestamp:         timestamp,
			OrgID:             sfOrgID,
			MergeInstructions: chunk,
		}

		batches = append(batches, batch)
	}

	return batches, nil
}

// AssembleSingle wraps a single instruction in a batch for real-time pushes
func (a *BatchAssembler) AssembleSingle(
	sfOrgID string,
	instruction entity.MergeInstruction,
) entity.MergeInstructionBatch {
	// Generate real-time batch_id: QTC-YYYYMMDD-RT-001
	timestamp := time.Now().UTC()
	batchID := fmt.Sprintf("QTC-%s-RT-%03d", timestamp.Format("20060102"), 1)

	return entity.MergeInstructionBatch{
		BatchID:           batchID,
		Timestamp:         timestamp.Format(time.RFC3339),
		OrgID:             sfOrgID,
		MergeInstructions: []entity.MergeInstruction{instruction},
	}
}

// SerializeBatch converts a batch to JSON
func (a *BatchAssembler) SerializeBatch(batch entity.MergeInstructionBatch) ([]byte, error) {
	return json.Marshal(batch)
}

// ValidateBatch checks if a batch meets all requirements
func (a *BatchAssembler) ValidateBatch(batch entity.MergeInstructionBatch) error {
	// Check batch_id format
	if batch.BatchID == "" {
		return fmt.Errorf("batch_id is empty")
	}

	// Validate batch_id format: QTC-YYYYMMDD-NNN or QTC-YYYYMMDD-RT-NNN
	batchIDPattern := regexp.MustCompile(`^QTC-\d{8}-(RT-)?\d{3}$`)
	if !batchIDPattern.MatchString(batch.BatchID) {
		return fmt.Errorf("batch_id does not match required format QTC-YYYYMMDD-NNN: got %s", batch.BatchID)
	}

	// Check org_id
	if batch.OrgID == "" {
		return fmt.Errorf("org_id is empty")
	}

	// Check instructions array
	if len(batch.MergeInstructions) == 0 {
		return fmt.Errorf("merge_instructions array is empty")
	}

	if len(batch.MergeInstructions) > a.maxBatchSize {
		return fmt.Errorf("merge_instructions array exceeds max batch size %d: got %d", a.maxBatchSize, len(batch.MergeInstructions))
	}

	// Validate each instruction
	for i, instruction := range batch.MergeInstructions {
		// Check instruction_id
		if instruction.InstructionID == "" {
			return fmt.Errorf("instruction %d has empty instruction_id", i)
		}

		// Check object_api_name
		if instruction.ObjectAPIName == "" {
			return fmt.Errorf("instruction %d has empty object_api_name", i)
		}

		// Check winner_id
		if instruction.WinnerID == "" {
			return fmt.Errorf("instruction %d has empty winner_id", i)
		}
		if len(instruction.WinnerID) != 18 {
			return fmt.Errorf("instruction %d has invalid winner_id length (expected 18 chars): got %d chars", i, len(instruction.WinnerID))
		}

		// Check loser_id
		if instruction.LoserID == "" {
			return fmt.Errorf("instruction %d has empty loser_id", i)
		}
		if len(instruction.LoserID) != 18 {
			return fmt.Errorf("instruction %d has invalid loser_id length (expected 18 chars): got %d chars", i, len(instruction.LoserID))
		}

		// Check field_values (can be empty for delete-only, but warn)
		if instruction.FieldValues == nil {
			return fmt.Errorf("instruction %d has nil field_values map", i)
		}
	}

	// Check total serialized JSON size
	serialized, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to serialize batch for size validation: %w", err)
	}

	maxPayloadSize := 10 * 1024 * 1024 // 10MB Salesforce API limit
	if len(serialized) > maxPayloadSize {
		return fmt.Errorf("serialized batch exceeds Salesforce API payload limit (10MB): got %d bytes", len(serialized))
	}

	return nil
}
