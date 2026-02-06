package dedup

import (
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// Normalizer provides field value normalization for consistent comparison
type Normalizer struct {
	defaultRegion string
}

// NewNormalizer creates a normalizer with default phone region
func NewNormalizer(defaultRegion string) *Normalizer {
	if defaultRegion == "" {
		defaultRegion = "US"
	}
	return &Normalizer{defaultRegion: defaultRegion}
}

// NormalizeText lowercases and trims whitespace
func (n *Normalizer) NormalizeText(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

// NormalizeEmail lowercases, trims, and validates email format
// Returns empty string if invalid email
func (n *Normalizer) NormalizeEmail(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || !strings.Contains(value, "@") {
		return ""
	}
	return value
}

// NormalizePhone converts phone to E.164 format
// Returns empty string if invalid/unparseable
func (n *Normalizer) NormalizePhone(value string) string {
	if value == "" {
		return ""
	}

	// Try to parse with default region
	num, err := phonenumbers.Parse(value, n.defaultRegion)
	if err != nil {
		return ""
	}

	// Validate the number
	if !phonenumbers.IsValidNumber(num) {
		return ""
	}

	// Format to E.164 (e.g., +15551234567)
	return phonenumbers.Format(num, phonenumbers.E164)
}

// ExtractEmailDomain returns the domain portion of an email
func (n *Normalizer) ExtractEmailDomain(email string) string {
	email = n.NormalizeEmail(email)
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// ExtractEmailLocal returns the local part (before @) of an email
func (n *Normalizer) ExtractEmailLocal(email string) string {
	email = n.NormalizeEmail(email)
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

// GetNamePrefix returns first N characters of a name, lowercased
func (n *Normalizer) GetNamePrefix(name string, length int) string {
	name = n.NormalizeText(name)
	if len(name) < length {
		return name
	}
	return name[:length]
}
