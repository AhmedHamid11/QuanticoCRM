package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/util"
)

// MergeDiscoveryService handles related record discovery for merge operations
type MergeDiscoveryService struct {
	metadataRepo *repo.MetadataRepo
}

// NewMergeDiscoveryService creates a new MergeDiscoveryService
func NewMergeDiscoveryService(metadataRepo *repo.MetadataRepo) *MergeDiscoveryService {
	return &MergeDiscoveryService{
		metadataRepo: metadataRepo,
	}
}

// DiscoverRelatedRecords finds all records that reference the given recordID via lookup fields
// This dynamically scans all entities and their fields to find foreign key references
func (s *MergeDiscoveryService) DiscoverRelatedRecords(ctx context.Context, dbConn db.DBConn, orgID, entityType, recordID string) ([]entity.RelatedRecordGroup, error) {
	// Get all entities for this org
	entities, err := s.metadataRepo.ListEntities(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}

	var groups []entity.RelatedRecordGroup

	// For each entity, check its fields for lookup references to our entityType
	for _, ent := range entities {
		// Skip the entity we're merging (no self-references)
		if ent.Name == entityType {
			continue
		}

		// Get fields for this entity
		fields, err := s.metadataRepo.ListFields(ctx, orgID, ent.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to list fields for %s: %w", ent.Name, err)
		}

		// Find lookup fields that reference our entityType
		for _, field := range fields {
			if field.Type != entity.FieldTypeLink {
				continue
			}

			// Check if this lookup points to our entityType
			if field.LinkEntity == nil || *field.LinkEntity != entityType {
				continue
			}

			// Found a lookup field! Query for records that reference our recordID
			tableName := util.GetTableName(ent.Name)
			fkColumn := util.CamelToSnake(field.Name) + "_id"

			// Check if table exists and has archived_at column
			var tableExists int
			err := dbConn.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
				tableName,
			).Scan(&tableExists)
			if err != nil || tableExists == 0 {
				continue // Table doesn't exist, skip
			}

			// Check if archived_at column exists
			hasArchivedAt := false
			pragmaRows, err := dbConn.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
			if err == nil {
				defer pragmaRows.Close()
				for pragmaRows.Next() {
					var cid int
					var name, colType string
					var notNull int
					var dfltValue interface{}
					var pk int
					if err := pragmaRows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
						continue
					}
					if name == "archived_at" {
						hasArchivedAt = true
						break
					}
				}
			}

			// Build query to find related records
			query := fmt.Sprintf(
				"SELECT id, %s FROM %s WHERE org_id = ? AND %s = ?",
				util.QuoteIdentifier(fkColumn),
				tableName,
				util.QuoteIdentifier(fkColumn),
			)

			// Exclude archived records if column exists
			if hasArchivedAt {
				query += " AND (archived_at IS NULL OR archived_at = '')"
			}

			rows, err := dbConn.QueryContext(ctx, query, orgID, recordID)
			if err != nil {
				// Column might not exist, skip this field
				continue
			}
			defer rows.Close()

			var relatedRecords []entity.RelatedRecord
			for rows.Next() {
				var id, fkValue string
				if err := rows.Scan(&id, &fkValue); err != nil {
					continue
				}

				relatedRecords = append(relatedRecords, entity.RelatedRecord{
					ID:         id,
					EntityType: ent.Name,
					FKField:    field.Name,
					FKValue:    fkValue,
					OrgID:      orgID,
				})
			}

			// Add group if we found related records
			if len(relatedRecords) > 0 {
				groups = append(groups, entity.RelatedRecordGroup{
					EntityType: ent.Name,
					FKField:    field.Name,
					Records:    relatedRecords,
				})
			}
		}
	}

	if groups == nil {
		groups = []entity.RelatedRecordGroup{}
	}

	return groups, nil
}

// CountRelatedRecords counts related records for multiple record IDs
// This is used in merge preview to show how many related records each duplicate has
func (s *MergeDiscoveryService) CountRelatedRecords(ctx context.Context, dbConn db.DBConn, orgID, entityType string, recordIDs []string) ([]entity.RelatedRecordCount, error) {
	if len(recordIDs) == 0 {
		return []entity.RelatedRecordCount{}, nil
	}

	// Get all entities for this org
	entities, err := s.metadataRepo.ListEntities(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}

	var counts []entity.RelatedRecordCount

	// For each entity, check its fields for lookup references to our entityType
	for _, ent := range entities {
		// Skip the entity we're merging (no self-references)
		if ent.Name == entityType {
			continue
		}

		// Get fields for this entity
		fields, err := s.metadataRepo.ListFields(ctx, orgID, ent.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to list fields for %s: %w", ent.Name, err)
		}

		// Find lookup fields that reference our entityType
		for _, field := range fields {
			if field.Type != entity.FieldTypeLink {
				continue
			}

			// Check if this lookup points to our entityType
			if field.LinkEntity == nil || *field.LinkEntity != entityType {
				continue
			}

			// Found a lookup field! Count records for each recordID
			tableName := util.GetTableName(ent.Name)
			fkColumn := util.CamelToSnake(field.Name) + "_id"

			// Check if table exists
			var tableExists int
			err := dbConn.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
				tableName,
			).Scan(&tableExists)
			if err != nil || tableExists == 0 {
				continue // Table doesn't exist, skip
			}

			// Check if archived_at column exists
			hasArchivedAt := false
			pragmaRows, err := dbConn.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
			if err == nil {
				defer pragmaRows.Close()
				for pragmaRows.Next() {
					var cid int
					var name, colType string
					var notNull int
					var dfltValue interface{}
					var pk int
					if err := pragmaRows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
						continue
					}
					if name == "archived_at" {
						hasArchivedAt = true
						break
					}
				}
			}

			// For each recordID, count and fetch related records
			for _, recordID := range recordIDs {
				query := fmt.Sprintf(
					"SELECT id FROM %s WHERE org_id = ? AND %s = ?",
					tableName,
					util.QuoteIdentifier(fkColumn),
				)

				// Exclude archived records if column exists
				if hasArchivedAt {
					query += " AND (archived_at IS NULL OR archived_at = '')"
				}

				rows, err := dbConn.QueryContext(ctx, query, orgID, recordID)
				if err != nil {
					// Column might not exist, skip this field
					continue
				}

				var relatedRecords []map[string]interface{}
				for rows.Next() {
					var id string
					if err := rows.Scan(&id); err != nil {
						continue
					}
					relatedRecords = append(relatedRecords, map[string]interface{}{
						"id": id,
					})
				}
				rows.Close()

				// Add count if we found related records
				if len(relatedRecords) > 0 {
					counts = append(counts, entity.RelatedRecordCount{
						EntityType:  ent.Name,
						EntityLabel: ent.Label,
						RecordID:    recordID,
						Count:       len(relatedRecords),
						Records:     relatedRecords,
					})
				}
			}
		}
	}

	if counts == nil {
		counts = []entity.RelatedRecordCount{}
	}

	return counts, nil
}

// CalculateCompleteness calculates a 0.0-1.0 completeness score for a record
// Score = (number of non-empty fields) / (total number of fields)
func (s *MergeDiscoveryService) CalculateCompleteness(record map[string]interface{}) float64 {
	if record == nil || len(record) == 0 {
		return 0.0
	}

	// Count total fields and non-empty fields
	totalFields := 0
	filledFields := 0

	for key, value := range record {
		// Skip system fields that don't affect completeness
		if key == "id" || key == "orgId" || key == "createdAt" || key == "modifiedAt" ||
			key == "createdById" || key == "modifiedById" {
			continue
		}

		totalFields++

		// Check if field is filled
		if !isEmpty(value) {
			filledFields++
		}
	}

	if totalFields == 0 {
		return 0.0
	}

	return float64(filledFields) / float64(totalFields)
}

// isEmpty checks if a value is considered empty
func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case int, int64:
		return v == 0
	case float32, float64:
		return v == 0.0
	case bool:
		return false // Booleans are never "empty" - false is a valid value
	default:
		return false
	}
}

// SuggestSurvivor analyzes records and suggests which should be the survivor
// Returns the ID of the record with the highest completeness score
func (s *MergeDiscoveryService) SuggestSurvivor(records []map[string]interface{}) string {
	if len(records) == 0 {
		return ""
	}

	var bestID string
	var bestScore float64 = -1.0

	for _, record := range records {
		score := s.CalculateCompleteness(record)

		// Get record ID
		id, ok := record["id"].(string)
		if !ok {
			continue
		}

		if score > bestScore {
			bestScore = score
			bestID = id
		}
	}

	return bestID
}
