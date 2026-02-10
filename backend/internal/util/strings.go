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

// GetTableName converts entity name to table name (e.g., "Contact" -> "contacts", "ClientContact" -> "client_contacts")
func GetTableName(entityName string) string {
	return CamelToSnake(entityName) + "s"
}
