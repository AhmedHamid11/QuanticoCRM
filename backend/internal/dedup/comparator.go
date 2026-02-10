package dedup

import (
	"github.com/adrg/strutil/metrics"
	"github.com/fastcrm/backend/internal/entity"
)

// Comparator interface for field comparison
type Comparator interface {
	Compare(a, b string) float64
}

// FuzzyTextComparator uses Jaro-Winkler for name/text comparison
type FuzzyTextComparator struct {
	jw         *metrics.JaroWinkler
	normalizer *Normalizer
}

// NewFuzzyTextComparator creates a Jaro-Winkler based comparator
func NewFuzzyTextComparator(normalizer *Normalizer) *FuzzyTextComparator {
	jw := metrics.NewJaroWinkler()
	jw.CaseSensitive = false
	return &FuzzyTextComparator{jw: jw, normalizer: normalizer}
}

// Compare returns Jaro-Winkler similarity (0.0 - 1.0)
func (c *FuzzyTextComparator) Compare(a, b string) float64 {
	a = c.normalizer.NormalizeText(a)
	b = c.normalizer.NormalizeText(b)

	if a == "" || b == "" {
		return 0.0
	}

	// Exact match
	if a == b {
		return 1.0
	}

	return c.jw.Compare(a, b)
}

// EmailComparator handles email-specific comparison
type EmailComparator struct {
	jw         *metrics.JaroWinkler
	normalizer *Normalizer
}

// NewEmailComparator creates an email comparator
func NewEmailComparator(normalizer *Normalizer) *EmailComparator {
	jw := metrics.NewJaroWinkler()
	jw.CaseSensitive = false
	return &EmailComparator{jw: jw, normalizer: normalizer}
}

// Compare handles email comparison with local/domain weighting
// Per SCORING_DEEP_DIVE.md: 80% local part, 20% domain
func (c *EmailComparator) Compare(a, b string) float64 {
	normA := c.normalizer.NormalizeEmail(a)
	normB := c.normalizer.NormalizeEmail(b)

	if normA == "" || normB == "" {
		return 0.0
	}

	// Exact match
	if normA == normB {
		return 1.0
	}

	localA := c.normalizer.ExtractEmailLocal(normA)
	localB := c.normalizer.ExtractEmailLocal(normB)
	domainA := c.normalizer.ExtractEmailDomain(normA)
	domainB := c.normalizer.ExtractEmailDomain(normB)

	// Exact local part match (same person, different provider)
	if localA == localB {
		return 0.85
	}

	// Fuzzy comparison
	localSim := c.jw.Compare(localA, localB)
	domainSim := 0.5 // Different domains
	if domainA == domainB {
		domainSim = 1.0
	}

	// Weighted: 80% local, 20% domain
	return (localSim * 0.8) + (domainSim * 0.2)
}

// PhoneComparator handles phone number comparison with E.164 normalization
type PhoneComparator struct {
	normalizer *Normalizer
}

// NewPhoneComparator creates a phone comparator
func NewPhoneComparator(normalizer *Normalizer) *PhoneComparator {
	return &PhoneComparator{normalizer: normalizer}
}

// Compare normalizes to E.164 and does exact match
// Phone numbers are binary - they match or they don't
func (c *PhoneComparator) Compare(a, b string) float64 {
	normA := c.normalizer.NormalizePhone(a)
	normB := c.normalizer.NormalizePhone(b)

	if normA == "" || normB == "" {
		return 0.0 // Can't compare invalid phones
	}

	if normA == normB {
		return 1.0 // Exact match after normalization
	}

	return 0.0 // Phone numbers don't fuzzy match
}

// ExactComparator for exact string matching
type ExactComparator struct {
	normalizer *Normalizer
}

// NewExactComparator creates an exact match comparator
func NewExactComparator(normalizer *Normalizer) *ExactComparator {
	return &ExactComparator{normalizer: normalizer}
}

// Compare returns 1.0 for exact match, 0.0 otherwise
func (c *ExactComparator) Compare(a, b string) float64 {
	a = c.normalizer.NormalizeText(a)
	b = c.normalizer.NormalizeText(b)

	if a == "" || b == "" {
		return 0.0
	}

	if a == b {
		return 1.0
	}
	return 0.0
}

// GetComparatorForFieldType returns appropriate comparator based on field type
func GetComparatorForFieldType(fieldType entity.FieldType, normalizer *Normalizer) Comparator {
	switch fieldType {
	case entity.FieldTypeEmail:
		return NewEmailComparator(normalizer)
	case entity.FieldTypePhone:
		return NewPhoneComparator(normalizer)
	case entity.FieldTypeVarchar, entity.FieldTypeText:
		return NewFuzzyTextComparator(normalizer)
	default:
		return NewExactComparator(normalizer)
	}
}

// GetComparatorForAlgorithm returns comparator based on algorithm name
func GetComparatorForAlgorithm(algorithm string, normalizer *Normalizer) Comparator {
	switch algorithm {
	case entity.AlgorithmEmail:
		return NewEmailComparator(normalizer)
	case entity.AlgorithmPhone:
		return NewPhoneComparator(normalizer)
	case entity.AlgorithmJaroWinkler:
		return NewFuzzyTextComparator(normalizer)
	case entity.AlgorithmExact:
		return NewExactComparator(normalizer)
	default:
		return NewFuzzyTextComparator(normalizer)
	}
}
