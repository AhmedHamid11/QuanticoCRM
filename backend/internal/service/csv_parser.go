package service

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/fastcrm/backend/internal/entity"
)

// CSVParseResult contains the parsed CSV data and any errors
type CSVParseResult struct {
	Records       []map[string]interface{} `json:"records"`
	Headers       []string                 `json:"headers"`
	MappedHeaders []string                 `json:"mappedHeaders"` // Headers mapped to field names
	RowCount      int                      `json:"rowCount"`
	Errors        []CSVParseError          `json:"errors,omitempty"`
}

// CSVParseError represents an error during CSV parsing
type CSVParseError struct {
	Row     int    `json:"row"`
	Column  string `json:"column,omitempty"`
	Message string `json:"message"`
}

// CSVParser handles CSV file parsing and field mapping
type CSVParser struct{}

// NewCSVParser creates a new CSVParser
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// MaxCSVRows is the maximum number of rows allowed in a CSV import
const MaxCSVRows = 10000

// Parse parses a CSV file and maps headers to entity fields
func (p *CSVParser) Parse(reader io.Reader, fields []entity.FieldDef) (*CSVParseResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	csvReader.LazyQuotes = true

	// Read header row
	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	if len(headers) == 0 {
		return nil, fmt.Errorf("CSV file has no columns")
	}

	// Trim whitespace from headers
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}

	// Build field mapping (case-insensitive)
	fieldMap := p.buildFieldMap(fields)
	mappedHeaders := p.mapHeaders(headers, fieldMap)

	result := &CSVParseResult{
		Headers:       headers,
		MappedHeaders: mappedHeaders,
		Records:       make([]map[string]interface{}, 0),
	}

	// Read data rows
	rowNum := 1 // 0 was header
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, CSVParseError{
				Row:     rowNum,
				Message: fmt.Sprintf("Failed to parse row: %v", err),
			})
			rowNum++
			continue
		}

		// Check row limit
		if rowNum > MaxCSVRows {
			result.Errors = append(result.Errors, CSVParseError{
				Row:     rowNum,
				Message: fmt.Sprintf("CSV exceeds maximum of %d rows", MaxCSVRows),
			})
			break
		}

		// Convert row to map
		record := p.rowToMap(row, headers, mappedHeaders, fields, rowNum, &result.Errors)
		if record != nil {
			result.Records = append(result.Records, record)
		}
		rowNum++
	}

	result.RowCount = len(result.Records)
	return result, nil
}

// buildFieldMap creates a case-insensitive map of field names and labels to field definitions
func (p *CSVParser) buildFieldMap(fields []entity.FieldDef) map[string]*entity.FieldDef {
	fieldMap := make(map[string]*entity.FieldDef)

	for i := range fields {
		field := &fields[i]

		// Map by field name (camelCase)
		fieldMap[strings.ToLower(field.Name)] = field

		// Map by label
		fieldMap[strings.ToLower(field.Label)] = field

		// Map by snake_case version
		fieldMap[strings.ToLower(p.camelToSnake(field.Name))] = field

		// For lookup fields, also map the ID and Name variants
		if field.Type == entity.FieldTypeLink {
			// fieldNameId, fieldName_id, etc.
			fieldMap[strings.ToLower(field.Name+"id")] = field
			fieldMap[strings.ToLower(field.Name+"_id")] = field
			fieldMap[strings.ToLower(p.camelToSnake(field.Name)+"_id")] = field
		}

		// For multi-lookup fields
		if field.Type == entity.FieldTypeLinkMultiple {
			fieldMap[strings.ToLower(field.Name+"ids")] = field
			fieldMap[strings.ToLower(field.Name+"_ids")] = field
			fieldMap[strings.ToLower(p.camelToSnake(field.Name)+"_ids")] = field
		}
	}

	return fieldMap
}

// mapHeaders maps CSV headers to field names
func (p *CSVParser) mapHeaders(headers []string, fieldMap map[string]*entity.FieldDef) []string {
	mapped := make([]string, len(headers))

	for i, header := range headers {
		// Clean the header
		cleanHeader := strings.ToLower(strings.TrimSpace(header))
		cleanHeader = strings.ReplaceAll(cleanHeader, " ", "_")
		cleanHeader = strings.ReplaceAll(cleanHeader, "-", "_")

		if field, ok := fieldMap[cleanHeader]; ok {
			// Determine the correct mapped name based on field type
			if field.Type == entity.FieldTypeLink {
				// Check if header is for ID or Name
				if strings.HasSuffix(cleanHeader, "id") || strings.HasSuffix(cleanHeader, "_id") {
					mapped[i] = field.Name + "Id"
				} else if strings.HasSuffix(cleanHeader, "name") || strings.HasSuffix(cleanHeader, "_name") {
					mapped[i] = field.Name + "Name"
				} else {
					// Default to ID for lookup fields
					mapped[i] = field.Name + "Id"
				}
			} else if field.Type == entity.FieldTypeLinkMultiple {
				if strings.HasSuffix(cleanHeader, "ids") || strings.HasSuffix(cleanHeader, "_ids") {
					mapped[i] = field.Name + "Ids"
				} else if strings.HasSuffix(cleanHeader, "names") || strings.HasSuffix(cleanHeader, "_names") {
					mapped[i] = field.Name + "Names"
				} else {
					mapped[i] = field.Name + "Ids"
				}
			} else {
				mapped[i] = field.Name
			}
		} else {
			// Header doesn't map to any field - keep original for error reporting
			mapped[i] = ""
		}
	}

	return mapped
}

// rowToMap converts a CSV row to a map using the mapped headers
func (p *CSVParser) rowToMap(
	row []string,
	headers []string,
	mappedHeaders []string,
	fields []entity.FieldDef,
	rowNum int,
	errors *[]CSVParseError,
) map[string]interface{} {
	record := make(map[string]interface{})

	// Build a quick lookup for field types
	fieldTypes := make(map[string]entity.FieldType)
	for _, field := range fields {
		fieldTypes[field.Name] = field.Type
	}

	for i, value := range row {
		if i >= len(mappedHeaders) {
			break
		}

		fieldName := mappedHeaders[i]
		if fieldName == "" {
			// Unmapped column - skip
			continue
		}

		// Trim whitespace
		value = strings.TrimSpace(value)

		// Handle empty values
		if value == "" {
			continue
		}

		// Store the value (type conversion will happen during validation/insert)
		record[fieldName] = value
	}

	// Skip completely empty rows
	if len(record) == 0 {
		return nil
	}

	return record
}

// ParseWithMapping parses CSV with explicit column mapping
func (p *CSVParser) ParseWithMapping(reader io.Reader, columnMapping map[string]string) (*CSVParseResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	csvReader.LazyQuotes = true

	// Read header row
	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Apply explicit mapping
	mappedHeaders := make([]string, len(headers))
	for i, header := range headers {
		header = strings.TrimSpace(header)
		headers[i] = header
		if mapped, ok := columnMapping[header]; ok {
			mappedHeaders[i] = mapped
		} else if mapped, ok := columnMapping[strings.ToLower(header)]; ok {
			mappedHeaders[i] = mapped
		}
	}

	result := &CSVParseResult{
		Headers:       headers,
		MappedHeaders: mappedHeaders,
		Records:       make([]map[string]interface{}, 0),
	}

	// Read data rows
	rowNum := 1
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, CSVParseError{
				Row:     rowNum,
				Message: fmt.Sprintf("Failed to parse row: %v", err),
			})
			rowNum++
			continue
		}

		if rowNum > MaxCSVRows {
			result.Errors = append(result.Errors, CSVParseError{
				Row:     rowNum,
				Message: fmt.Sprintf("CSV exceeds maximum of %d rows", MaxCSVRows),
			})
			break
		}

		record := make(map[string]interface{})
		for i, value := range row {
			if i >= len(mappedHeaders) || mappedHeaders[i] == "" {
				continue
			}
			value = strings.TrimSpace(value)
			if value != "" {
				record[mappedHeaders[i]] = value
			}
		}

		if len(record) > 0 {
			result.Records = append(result.Records, record)
		}
		rowNum++
	}

	result.RowCount = len(result.Records)
	return result, nil
}

// GetSampleRows returns the first N rows for preview
func (p *CSVParser) GetSampleRows(reader io.Reader, count int) ([]string, [][]string, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	csvReader.LazyQuotes = true

	// Read header row
	headers, err := csvReader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Trim headers
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}

	// Read sample rows
	var rows [][]string
	for i := 0; i < count; i++ {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip bad rows in preview
		}
		rows = append(rows, row)
	}

	return headers, rows, nil
}

// camelToSnake converts camelCase to snake_case
func (p *CSVParser) camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteByte(byte(r + 32))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
