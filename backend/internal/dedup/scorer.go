package dedup

import (
	"github.com/fastcrm/backend/internal/entity"
)

// Scorer calculates overall match scores using weighted field comparison
type Scorer struct {
	normalizer *Normalizer
}

// NewScorer creates a new scorer with the given normalizer
func NewScorer(normalizer *Normalizer) *Scorer {
	return &Scorer{normalizer: normalizer}
}

// CalculateScore computes overall match score for two records against a rule
// Uses weighted additive scoring per SCORING_DEEP_DIVE.md
func (s *Scorer) CalculateScore(recordA, recordB map[string]interface{}, rule *entity.MatchingRule) *entity.MatchResult {
	var totalScore, totalWeight float64
	fieldScores := make(map[string]float64)
	var matchingFields []string

	for _, fc := range rule.FieldConfigs {
		// Get field values from records
		valA := getStringValue(recordA, fc.FieldName)
		valB := getStringValue(recordB, fc.FieldName)

		// For cross-entity rules, use target field name for record B
		if fc.TargetFieldName != "" {
			valB = getStringValue(recordB, fc.TargetFieldName)
		}

		// Skip if either value is empty (don't penalize missing data)
		if valA == "" || valB == "" {
			continue
		}

		// Both values present: this field counts toward the denominator
		totalWeight += fc.Weight

		// Get comparator for this field's algorithm
		comparator := GetComparatorForAlgorithm(fc.Algorithm, s.normalizer)
		score := comparator.Compare(valA, valB)

		fieldScores[fc.FieldName] = score

		// Apply per-field threshold
		threshold := fc.Threshold
		if threshold == 0 {
			threshold = 0.5 // Default: any match above 0.5 contributes
		}

		if score >= threshold {
			totalScore += score * fc.Weight
			matchingFields = append(matchingFields, fc.FieldName)
		}
	}

	// Calculate overall score
	overallScore := 0.0
	if totalWeight > 0 {
		overallScore = totalScore / totalWeight
	}

	return &entity.MatchResult{
		Score:          overallScore,
		ConfidenceTier: s.getConfidenceTier(overallScore, rule),
		FieldScores:    fieldScores,
		MatchingFields: matchingFields,
		RuleID:         rule.ID,
		RuleName:       rule.Name,
	}
}

// getConfidenceTier determines confidence level based on rule thresholds
func (s *Scorer) getConfidenceTier(score float64, rule *entity.MatchingRule) string {
	if score >= rule.HighConfidenceThreshold {
		return entity.ConfidenceHigh
	}
	if score >= rule.MediumConfidenceThreshold {
		return entity.ConfidenceMedium
	}
	if score >= rule.Threshold {
		return entity.ConfidenceLow
	}
	return "" // Below minimum threshold
}

// IsMatch returns true if score meets minimum threshold
func (s *Scorer) IsMatch(result *entity.MatchResult, rule *entity.MatchingRule) bool {
	return result.Score >= rule.Threshold
}

// CheckExactMatchBoost checks if any field with exactMatchBoost has exact match
// Returns true if should auto-high confidence
func (s *Scorer) CheckExactMatchBoost(recordA, recordB map[string]interface{}, rule *entity.MatchingRule) bool {
	for _, fc := range rule.FieldConfigs {
		if !fc.ExactMatchBoost {
			continue
		}

		valA := getStringValue(recordA, fc.FieldName)
		valB := getStringValue(recordB, fc.FieldName)

		if fc.TargetFieldName != "" {
			valB = getStringValue(recordB, fc.TargetFieldName)
		}

		if valA == "" || valB == "" {
			continue
		}

		// Normalize and compare
		normA := s.normalizer.NormalizeText(valA)
		normB := s.normalizer.NormalizeText(valB)

		if normA == normB {
			return true // Exact match on a boost field
		}
	}
	return false
}

// getStringValue extracts string value from record map
func getStringValue(record map[string]interface{}, fieldName string) string {
	val, ok := record[fieldName]
	if !ok || val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	default:
		return ""
	}
}

// CompareRecords is a convenience method that scores and determines if match
func (s *Scorer) CompareRecords(recordA, recordB map[string]interface{}, rule *entity.MatchingRule) (*entity.MatchResult, bool) {
	result := s.CalculateScore(recordA, recordB, rule)

	// Check exact match boost
	if s.CheckExactMatchBoost(recordA, recordB, rule) {
		result.ConfidenceTier = entity.ConfidenceHigh
	}

	isMatch := s.IsMatch(result, rule)
	return result, isMatch
}
