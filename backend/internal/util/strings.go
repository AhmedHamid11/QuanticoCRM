package util

import "strings"

// CamelToSnake converts a camelCase string to snake_case
func CamelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteByte(byte(r + 32)) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// SnakeToCamel converts a snake_case string to camelCase
func SnakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// QuoteIdentifier quotes a SQL identifier (column/table name) for SQLite
func QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// Pluralize returns the English plural of a lowercase word.
// Handles common suffixes: -y → -ies, -s/-x/-z/-ch/-sh → -es.
func Pluralize(word string) string {
	if word == "" {
		return word
	}
	// Words ending in consonant + y → ies
	if strings.HasSuffix(word, "y") && len(word) > 1 {
		prev := word[len(word)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return word[:len(word)-1] + "ies"
		}
	}
	// Words ending in s, x, z, ch, sh → es
	if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "x") || strings.HasSuffix(word, "z") ||
		strings.HasSuffix(word, "ch") || strings.HasSuffix(word, "sh") {
		return word + "es"
	}
	return word + "s"
}

// GetTableName converts entity name to table name (e.g., "Contact" -> "contacts", "Property" -> "properties")
func GetTableName(entityName string) string {
	snake := CamelToSnake(entityName)
	return Pluralize(snake)
}

// GetLinkColumnNames returns the ID and Name column names for a link field.
// Link field names already end with "Id" (e.g., "landlordId" → snake "landlord_id"),
// so the snake name IS the ID column. The name column appends "_name".
func GetLinkColumnNames(snakeFieldName string) (idCol, nameCol string) {
	idCol = snakeFieldName
	nameCol = snakeFieldName + "_name"
	return
}
