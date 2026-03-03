package util

import (
	"encoding/json"
	"fmt"

	"github.com/fastcrm/backend/internal/entity"
)

// FieldValues holds the columns, placeholders, and values for SQL operations
type FieldValues struct {
	Columns      []string
	Placeholders []string
	Values       []interface{}
}

// BuildInsertFieldValues builds columns, placeholders, and values for INSERT operations
func BuildInsertFieldValues(fields []entity.FieldDef, body map[string]interface{}) *FieldValues {
	fv := &FieldValues{}

	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Handle lookup fields specially - they have {fieldName}Id and {fieldName}Name in body
		if field.Type == entity.FieldTypeLink {
			snakeName := CamelToSnake(field.Name)
			idCol, nameCol := GetLinkColumnNames(snakeName)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := body[idKey]; ok {
				fv.Columns = append(fv.Columns, QuoteIdentifier(idCol))
				fv.Placeholders = append(fv.Placeholders, "?")
				fv.Values = append(fv.Values, idVal)
			}
			if nameVal, ok := body[nameKey]; ok {
				fv.Columns = append(fv.Columns, QuoteIdentifier(nameCol))
				fv.Placeholders = append(fv.Placeholders, "?")
				fv.Values = append(fv.Values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields - they have {fieldName}Ids and {fieldName}Names (JSON arrays) in body
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := body[idsKey]; ok {
				fv.Columns = append(fv.Columns, QuoteIdentifier(snakeName+"_ids"))
				fv.Placeholders = append(fv.Placeholders, "?")
				fv.Values = append(fv.Values, idsVal)
			}
			if namesVal, ok := body[namesKey]; ok {
				fv.Columns = append(fv.Columns, QuoteIdentifier(snakeName+"_names"))
				fv.Placeholders = append(fv.Placeholders, "?")
				fv.Values = append(fv.Values, namesVal)
			}
			continue
		}

		// Regular fields - check if value provided or use default
		if val, ok := body[field.Name]; ok {
			fv.Columns = append(fv.Columns, QuoteIdentifier(CamelToSnake(field.Name)))
			fv.Placeholders = append(fv.Placeholders, "?")
			fv.Values = append(fv.Values, val)
		} else if field.DefaultValue != nil && *field.DefaultValue != "" {
			fv.Columns = append(fv.Columns, QuoteIdentifier(CamelToSnake(field.Name)))
			fv.Placeholders = append(fv.Placeholders, "?")
			fv.Values = append(fv.Values, *field.DefaultValue)
			// Also add to body so it's returned in the response
			body[field.Name] = *field.DefaultValue
		}
	}

	return fv
}

// BuildUpdateSetClauses builds SET clauses and values for UPDATE operations
func BuildUpdateSetClauses(fields []entity.FieldDef, body map[string]interface{}) (setClauses []string, values []interface{}) {
	for _, field := range fields {
		if field.Name == "id" {
			continue
		}

		// Handle lookup fields specially
		if field.Type == entity.FieldTypeLink {
			snakeName := CamelToSnake(field.Name)
			idCol, nameCol := GetLinkColumnNames(snakeName)
			idKey := field.Name + "Id"
			nameKey := field.Name + "Name"

			if idVal, ok := body[idKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", QuoteIdentifier(idCol)))
				values = append(values, idVal)
			}
			if nameVal, ok := body[nameKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", QuoteIdentifier(nameCol)))
				values = append(values, nameVal)
			}
			continue
		}

		// Handle multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			snakeName := CamelToSnake(field.Name)
			idsKey := field.Name + "Ids"
			namesKey := field.Name + "Names"

			if idsVal, ok := body[idsKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", QuoteIdentifier(snakeName+"_ids")))
				values = append(values, idsVal)
			}
			if namesVal, ok := body[namesKey]; ok {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", QuoteIdentifier(snakeName+"_names")))
				values = append(values, namesVal)
			}
			continue
		}

		// Regular fields
		if val, ok := body[field.Name]; ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", QuoteIdentifier(CamelToSnake(field.Name))))
			values = append(values, val)
		}
	}

	return setClauses, values
}

// AddDerivedLookupFields adds derived fields for lookups to a camelCase record
func AddDerivedLookupFields(record, camelRecord map[string]interface{}, lookupFields, multiLookupFields map[string]*entity.FieldDef) {
	// Add derived fields for lookups: {fieldName}Id, {fieldName}Name, {fieldName}Link
	for fieldName, fieldDef := range lookupFields {
		snakeName := CamelToSnake(fieldName)
		idCol, nameCol := GetLinkColumnNames(snakeName)

		// Get the ID and Name values from the record
		idVal := record[idCol]
		nameVal := record[nameCol]

		// Set the derived fields in camelCase
		camelRecord[fieldName+"Id"] = idVal
		camelRecord[fieldName+"Name"] = nameVal

		// Generate the link URL if we have an ID and linked entity
		if idVal != nil && idVal != "" && fieldDef.LinkEntity != nil {
			linkedEntityPlural := GetTableName(*fieldDef.LinkEntity)
			camelRecord[fieldName+"Link"] = "/" + linkedEntityPlural + "/" + fmt.Sprintf("%v", idVal)
		} else {
			camelRecord[fieldName+"Link"] = nil
		}
	}

	// Add derived fields for multi-lookups: {fieldName}Ids, {fieldName}Names, {fieldName}Links
	for fieldName, fieldDef := range multiLookupFields {
		snakeName := CamelToSnake(fieldName)
		idsCol := snakeName + "_ids"
		namesCol := snakeName + "_names"

		// Get the IDs and Names values from the record
		idsVal := record[idsCol]
		namesVal := record[namesCol]

		// Set the derived fields in camelCase
		camelRecord[fieldName+"Ids"] = idsVal
		camelRecord[fieldName+"Names"] = namesVal

		// Generate the links array if we have IDs and linked entity
		if idsVal != nil && idsVal != "" && idsVal != "[]" && fieldDef.LinkEntity != nil {
			linkedEntityPlural := GetTableName(*fieldDef.LinkEntity)
			// Parse the IDs JSON array and create links
			var ids []string
			if idsStr, ok := idsVal.(string); ok {
				json.Unmarshal([]byte(idsStr), &ids)
			}
			var links []string
			for _, idItem := range ids {
				links = append(links, "/"+linkedEntityPlural+"/"+idItem)
			}
			linksJSON, _ := json.Marshal(links)
			camelRecord[fieldName+"Links"] = string(linksJSON)
		} else {
			camelRecord[fieldName+"Links"] = "[]"
		}
	}
}
