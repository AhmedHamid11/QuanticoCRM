package util

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// FetchRecordAsMap fetches a record by ID and returns it as a map (for tripwire evaluation)
// CRITICAL: Always filters by org_id to ensure multi-tenant data isolation
func FetchRecordAsMap(ctx context.Context, db *sql.DB, tableName, id, orgID string) (map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE id = ? AND org_id = ?", tableName)
	rows, err := db.QueryContext(ctx, query, id, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		return nil, nil
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	record := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]
		if b, ok := val.([]byte); ok {
			record[col] = string(b)
		} else {
			record[col] = val
		}
	}

	// Convert snake_case to camelCase
	camelRecord := make(map[string]interface{})
	for col, val := range record {
		camelCol := SnakeToCamel(col)
		camelRecord[camelCol] = val
	}

	return camelRecord, nil
}

// GetRecordDisplayName extracts a human-readable display name from a record map.
// For contacts/leads it uses firstName + lastName, for other entities it uses the name field.
func GetRecordDisplayName(entityType string, record map[string]interface{}) string {
	et := strings.ToLower(entityType)
	if et == "contact" || et == "lead" {
		first, _ := record["firstName"].(string)
		last, _ := record["lastName"].(string)
		name := strings.TrimSpace(first + " " + last)
		if name != "" {
			return name
		}
	}
	// Try "name" field (accounts, opportunities, tasks, etc.)
	if name, ok := record["name"].(string); ok && name != "" {
		return name
	}
	// Try snake_case variants (raw DB rows use snake_case)
	if et == "contact" || et == "lead" {
		first, _ := record["first_name"].(string)
		last, _ := record["last_name"].(string)
		name := strings.TrimSpace(first + " " + last)
		if name != "" {
			return name
		}
	}
	return ""
}

// StructToMap converts any struct to a map using reflection.
// It uses json tags for field names, falling back to the field name in camelCase.
// This ensures all fields (including newly added ones) are automatically included.
func StructToMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	result := make(map[string]interface{})
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get the json tag name, or use field name
		jsonTag := field.Tag.Get("json")
		fieldName := field.Name
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			} else if parts[0] == "-" {
				continue // Skip fields with json:"-"
			}
		} else {
			// Convert to camelCase if no json tag
			if len(fieldName) > 0 {
				fieldName = strings.ToLower(fieldName[:1]) + fieldName[1:]
			}
		}

		// Get the actual value
		var value interface{}
		if fieldVal.Kind() == reflect.Ptr {
			if fieldVal.IsNil() {
				value = nil
			} else {
				value = fieldVal.Elem().Interface()
			}
		} else {
			value = fieldVal.Interface()
		}

		result[fieldName] = value
	}

	return result
}

// ScanRowsToMaps scans all rows into a slice of maps with camelCase keys
func ScanRowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var records []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		record := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				record[col] = string(b)
			} else {
				record[col] = val
			}
		}

		// Convert snake_case columns to camelCase
		camelRecord := make(map[string]interface{})
		for col, val := range record {
			camelCol := SnakeToCamel(col)
			camelRecord[camelCol] = val
		}

		records = append(records, camelRecord)
	}

	if records == nil {
		records = []map[string]interface{}{}
	}

	return records, nil
}
