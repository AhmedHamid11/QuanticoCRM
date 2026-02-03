package util

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

// EnsureTableExists creates the table for a custom entity if it doesn't exist
// Accepts db.DBConn interface for retry-enabled connections
func EnsureTableExists(ctx context.Context, conn db.DBConn, entityName string, fields []entity.FieldDef) error {
	tableName := GetTableName(entityName)

	// Check if table exists - use db.QueryRowScan for retry support
	var count int
	err := db.QueryRowScan(ctx, conn, []interface{}{&count},
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil // Table exists
	}

	// Build CREATE TABLE statement
	var columns []string
	columns = append(columns, "id TEXT PRIMARY KEY")
	columns = append(columns, "org_id TEXT NOT NULL") // CRITICAL: Multi-tenant isolation

	for _, field := range fields {
		if field.Name == "id" || field.Name == "created_at" || field.Name == "modified_at" {
			continue // Already added or will be added as standard audit columns
		}

		// Skip textBlock fields - they don't need database columns (display-only)
		if field.Type == entity.FieldTypeTextBlock {
			continue
		}

		// For lookup fields, create both _id and _name columns
		if field.Type == entity.FieldTypeLink {
			snakeName := CamelToSnake(field.Name)
			// Create {field_name}_id column for the foreign key
			idColDef := fmt.Sprintf("%s TEXT", QuoteIdentifier(snakeName+"_id"))
			columns = append(columns, idColDef)
			// Create {field_name}_name column for the denormalized display name
			nameColDef := fmt.Sprintf("%s TEXT DEFAULT ''", QuoteIdentifier(snakeName+"_name"))
			columns = append(columns, nameColDef)
			continue
		}

		// For multi-lookup fields, create _ids and _names columns (store as JSON arrays)
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := CamelToSnake(field.Name)
			// Create {field_name}_ids column for the JSON array of foreign keys
			idsColDef := fmt.Sprintf("%s TEXT DEFAULT '[]'", QuoteIdentifier(snakeName+"_ids"))
			columns = append(columns, idsColDef)
			// Create {field_name}_names column for the JSON array of display names
			namesColDef := fmt.Sprintf("%s TEXT DEFAULT '[]'", QuoteIdentifier(snakeName+"_names"))
			columns = append(columns, namesColDef)
			continue
		}

		colType := "TEXT"
		switch field.Type {
		case "int":
			colType = "INTEGER"
		case "float", "currency":
			colType = "REAL"
		case "bool":
			colType = "INTEGER"
		case "date", "datetime":
			colType = "TEXT"
		default:
			colType = "TEXT"
		}

		colDef := fmt.Sprintf("%s %s", QuoteIdentifier(CamelToSnake(field.Name)), colType)
		if field.IsRequired {
			colDef += " NOT NULL"
		}
		columns = append(columns, colDef)
	}

	// Add standard audit columns
	columns = append(columns, "created_at TEXT DEFAULT CURRENT_TIMESTAMP")
	columns = append(columns, "modified_at TEXT DEFAULT CURRENT_TIMESTAMP")
	columns = append(columns, "created_by_id TEXT")
	columns = append(columns, "modified_by_id TEXT")

	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(columns, ", "))

	_, err = conn.ExecContext(ctx, createSQL)
	if err != nil {
		return err
	}

	// Create index on org_id for multi-tenant query performance
	indexSQL := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_org_id ON %s(org_id)", tableName, tableName)
	_, err = conn.ExecContext(ctx, indexSQL)
	return err
}

// EnsureTableExistsRaw is the legacy version that accepts *sql.DB directly
// Deprecated: Use EnsureTableExists with db.DBConn instead
func EnsureTableExistsRaw(ctx context.Context, rawDB *sql.DB, entityName string, fields []entity.FieldDef) error {
	return EnsureTableExists(ctx, rawDB, entityName, fields)
}

// BuildFieldMaps builds lookup and multi-lookup field maps from field definitions
func BuildFieldMaps(fields []entity.FieldDef) (lookups, multiLookups map[string]*entity.FieldDef) {
	lookups = make(map[string]*entity.FieldDef)
	multiLookups = make(map[string]*entity.FieldDef)

	for i := range fields {
		if fields[i].Type == entity.FieldTypeLink {
			lookups[fields[i].Name] = &fields[i]
		}
		if fields[i].Type == entity.FieldTypeLinkMultiple {
			multiLookups[fields[i].Name] = &fields[i]
		}
	}

	return lookups, multiLookups
}

// SyncFieldColumns ensures all field definitions have corresponding columns in the table.
// This is used to fix schema drift where fields exist in metadata but columns are missing.
// Returns the number of columns added and any error encountered.
func SyncFieldColumns(ctx context.Context, conn db.DBConn, entityName string, fields []entity.FieldDef) (int, error) {
	tableName := GetTableName(entityName)
	columnsAdded := 0

	// Check if table exists first
	var count int
	err := db.QueryRowScan(ctx, conn, []interface{}{&count},
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName)
	if err != nil {
		return 0, fmt.Errorf("failed to check if table exists: %w", err)
	}
	if count == 0 {
		return 0, fmt.Errorf("table %s does not exist", tableName)
	}

	// Get existing columns
	existingCols := make(map[string]bool)
	rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return 0, fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
		existingCols[name] = true
	}

	// Check each field and add missing columns
	for _, field := range fields {
		if field.Name == "id" || field.Name == "created_at" || field.Name == "modified_at" {
			continue
		}

		// Skip textBlock fields - they don't need database columns (display-only)
		if field.Type == entity.FieldTypeTextBlock {
			continue
		}

		snakeName := CamelToSnake(field.Name)

		// Handle lookup fields (need _id and _name columns)
		if field.Type == entity.FieldTypeLink {
			idCol := snakeName + "_id"
			nameCol := snakeName + "_name"

			if !existingCols[idCol] {
				sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT", tableName, QuoteIdentifier(idCol))
				if _, err := conn.ExecContext(ctx, sql); err != nil {
					if !strings.Contains(err.Error(), "duplicate column") {
						return columnsAdded, fmt.Errorf("failed to add column %s: %w", idCol, err)
					}
				} else {
					columnsAdded++
				}
			}

			if !existingCols[nameCol] {
				sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT ''", tableName, QuoteIdentifier(nameCol))
				if _, err := conn.ExecContext(ctx, sql); err != nil {
					if !strings.Contains(err.Error(), "duplicate column") {
						return columnsAdded, fmt.Errorf("failed to add column %s: %w", nameCol, err)
					}
				} else {
					columnsAdded++
				}
			}
			continue
		}

		// Handle multi-lookup fields (need _ids and _names columns)
		if field.Type == entity.FieldTypeLinkMultiple {
			idsCol := snakeName + "_ids"
			namesCol := snakeName + "_names"

			if !existingCols[idsCol] {
				sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT '[]'", tableName, QuoteIdentifier(idsCol))
				if _, err := conn.ExecContext(ctx, sql); err != nil {
					if !strings.Contains(err.Error(), "duplicate column") {
						return columnsAdded, fmt.Errorf("failed to add column %s: %w", idsCol, err)
					}
				} else {
					columnsAdded++
				}
			}

			if !existingCols[namesCol] {
				sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT '[]'", tableName, QuoteIdentifier(namesCol))
				if _, err := conn.ExecContext(ctx, sql); err != nil {
					if !strings.Contains(err.Error(), "duplicate column") {
						return columnsAdded, fmt.Errorf("failed to add column %s: %w", namesCol, err)
					}
				} else {
					columnsAdded++
				}
			}
			continue
		}

		// Handle stream fields (need entry and _log columns)
		if field.Type == entity.FieldTypeStream {
			entryCol := snakeName
			logCol := snakeName + "_log"

			if !existingCols[entryCol] {
				sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT ''", tableName, QuoteIdentifier(entryCol))
				if _, err := conn.ExecContext(ctx, sql); err != nil {
					if !strings.Contains(err.Error(), "duplicate column") {
						return columnsAdded, fmt.Errorf("failed to add column %s: %w", entryCol, err)
					}
				} else {
					columnsAdded++
				}
			}

			if !existingCols[logCol] {
				sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT ''", tableName, QuoteIdentifier(logCol))
				if _, err := conn.ExecContext(ctx, sql); err != nil {
					if !strings.Contains(err.Error(), "duplicate column") {
						return columnsAdded, fmt.Errorf("failed to add column %s: %w", logCol, err)
					}
				} else {
					columnsAdded++
				}
			}
			continue
		}

		// Regular fields
		if !existingCols[snakeName] {
			colType := "TEXT"
			switch field.Type {
			case "int":
				colType = "INTEGER"
			case "float", "currency":
				colType = "REAL"
			case "bool":
				colType = "INTEGER"
			}

			sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, QuoteIdentifier(snakeName), colType)
			if _, err := conn.ExecContext(ctx, sql); err != nil {
				if !strings.Contains(err.Error(), "duplicate column") {
					return columnsAdded, fmt.Errorf("failed to add column %s: %w", snakeName, err)
				}
			} else {
				columnsAdded++
			}
		}
	}

	return columnsAdded, nil
}

// BuildValidSortColumns builds a map of valid sort columns from field definitions
func BuildValidSortColumns(fields []entity.FieldDef) (validColumns map[string]bool, linkFieldNames map[string]bool) {
	validColumns = map[string]bool{
		"id": true, "created_at": true, "modified_at": true,
		"created_by_id": true, "modified_by_id": true,
	}
	linkFieldNames = make(map[string]bool)

	for _, field := range fields {
		snakeName := CamelToSnake(field.Name)
		if field.Type == entity.FieldTypeLink {
			linkFieldNames[snakeName] = true
			validColumns[snakeName] = true
			validColumns[snakeName+"_id"] = true
			validColumns[snakeName+"_name"] = true
		} else if field.Type == entity.FieldTypeLinkMultiple {
			validColumns[snakeName+"_ids"] = true
			validColumns[snakeName+"_names"] = true
		} else {
			validColumns[snakeName] = true
		}
	}

	return validColumns, linkFieldNames
}
