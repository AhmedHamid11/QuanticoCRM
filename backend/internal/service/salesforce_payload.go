package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

// MergeInstructionBuilder transforms dedup results into Salesforce merge instruction JSON
type MergeInstructionBuilder struct {
	sfRepo       *repo.SalesforceRepo
	metadataRepo *repo.MetadataRepo
}

// NewMergeInstructionBuilder creates a new MergeInstructionBuilder
func NewMergeInstructionBuilder(sfRepo *repo.SalesforceRepo, metadataRepo *repo.MetadataRepo) *MergeInstructionBuilder {
	return &MergeInstructionBuilder{
		sfRepo:       sfRepo,
		metadataRepo: metadataRepo,
	}
}

// MergeInstructionInput is the input struct for building merge instructions
type MergeInstructionInput struct {
	EntityType   string
	SurvivorID   string
	DuplicateID  string
	MergedFields map[string]interface{}
}

// BuildInstruction creates a single merge instruction from dedup results
func (b *MergeInstructionBuilder) BuildInstruction(
	ctx context.Context,
	orgID string,
	entityType string,
	survivorID string,
	duplicateID string,
	mergedFields map[string]interface{},
	instructionCounter int,
) (*entity.MergeInstruction, error) {
	// 1. Generate instruction_id
	instructionID := fmt.Sprintf("MI-%04d", instructionCounter)

	// 2. Load field mappings
	mappings, err := b.sfRepo.ListFieldMappings(ctx, orgID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to load field mappings: %w", err)
	}

	// 3. Determine Salesforce object API name
	salesforceObject := entityType // Default to entity type (Contact, Account, Lead are standard)
	if len(mappings) > 0 {
		salesforceObject = mappings[0].SalesforceObject
	}

	// 4. Convert Quantico field names to Salesforce API names
	salesforceFields := make(map[string]interface{})

	// Create a lookup map for faster field name translation
	fieldLookup := make(map[string]string) // quantico_field -> salesforce_field
	for _, mapping := range mappings {
		fieldLookup[mapping.QuanticoField] = mapping.SalesforceField
	}

	// Translate field names
	for quanticoField, value := range mergedFields {
		salesforceField := quanticoField // Default: use same name if no mapping exists
		if mappedField, exists := fieldLookup[quanticoField]; exists {
			salesforceField = mappedField
		}
		salesforceFields[salesforceField] = value
	}

	// 5. Convert Salesforce Record IDs to 18-character format
	survivorID18, err := ensureSalesforceID18(survivorID)
	if err != nil {
		return nil, fmt.Errorf("invalid survivor ID: %w", err)
	}

	duplicateID18, err := ensureSalesforceID18(duplicateID)
	if err != nil {
		return nil, fmt.Errorf("invalid duplicate ID: %w", err)
	}

	// 6. Build MergeInstruction entity
	instruction := &entity.MergeInstruction{
		InstructionID: instructionID,
		ObjectAPIName: salesforceObject,
		WinnerID:      survivorID18,
		LoserID:       duplicateID18,
		FieldValues:   salesforceFields,
	}

	// 7. Validate field_values JSON size
	fieldValuesJSON, err := json.Marshal(salesforceFields)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize field_values: %w", err)
	}

	if len(fieldValuesJSON) > 131072 {
		return nil, fmt.Errorf("field_values JSON exceeds Salesforce Long Text Area limit (131,072 chars): got %d chars", len(fieldValuesJSON))
	}

	return instruction, nil
}

// BuildInstructions creates multiple merge instructions from a batch of dedup results
func (b *MergeInstructionBuilder) BuildInstructions(
	ctx context.Context,
	orgID string,
	mergeRequests []MergeInstructionInput,
) ([]entity.MergeInstruction, error) {
	instructions := make([]entity.MergeInstruction, 0, len(mergeRequests))

	for i, request := range mergeRequests {
		instruction, err := b.BuildInstruction(
			ctx,
			orgID,
			request.EntityType,
			request.SurvivorID,
			request.DuplicateID,
			request.MergedFields,
			i+1, // instructionCounter starts at 1
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build instruction %d: %w", i+1, err)
		}
		instructions = append(instructions, *instruction)
	}

	return instructions, nil
}

// ensureSalesforceID18 converts a Salesforce ID to 18-character format
func ensureSalesforceID18(id string) (string, error) {
	if len(id) == 18 {
		return id, nil
	}
	if len(id) != 15 {
		return "", fmt.Errorf("invalid Salesforce ID length %d (expected 15 or 18): %s", len(id), id)
	}

	// Salesforce checksum: split into 3 groups of 5 chars
	// For each group, create a 5-bit number where bit N is 1 if char N is uppercase
	// Map each 5-bit number to base32 char (A-Z, 0-5)
	suffix := ""
	base32Chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"

	for i := 0; i < 3; i++ {
		flags := 0
		for j := 0; j < 5; j++ {
			c := id[i*5+j]
			if c >= 'A' && c <= 'Z' {
				flags |= 1 << j
			}
		}
		suffix += string(base32Chars[flags])
	}

	return id + suffix, nil
}
