# Phase 11: Detection Foundation - Research

**Researched:** 2026-02-06
**Domain:** CRM Deduplication Detection Engine - Similarity Algorithms, Normalization, Blocking
**Confidence:** HIGH

## Summary

This research covers the core duplicate detection infrastructure for Quantico CRM: similarity algorithms (Jaro-Winkler, phonetic), field normalization (email, phone E.164), configurable matching rules with weighted scoring, and blocking strategies for performance. The detection foundation enables identifying duplicate records before merge or UI phases.

The project already has comprehensive prior research in `.planning/research/SCORING_DEEP_DIVE.md` covering Fellegi-Sunter probabilistic matching, field-specific algorithms, and threshold recommendations. This phase research focuses on **implementation specifics**: Go libraries, database schema for matching rules, and integration with existing Quantico CRM metadata architecture.

**Key insight:** The existing `FieldDef` metadata already contains field types (`email`, `phone`, `varchar`) that map directly to normalization and comparison strategies. The detection engine can auto-select appropriate algorithms per field type.

**Primary recommendation:** Use `github.com/adrg/strutil` for Jaro-Winkler (well-maintained, simple API, MIT license) and `github.com/nyaruka/phonenumbers` for E.164 phone normalization (Go port of Google libphonenumber, actively maintained with v1.6.8 as of Jan 2026).

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/adrg/strutil | v0.3.1 | Jaro-Winkler, Levenshtein, Jaccard similarity | MIT license, clean API, includes 8+ string metrics, 188k+ downloads |
| github.com/nyaruka/phonenumbers | v1.6.8 | E.164 phone normalization, validation | Official Go port of Google libphonenumber, 346+ importers, actively maintained |
| github.com/tilotech/go-phonetics | latest | Soundex, Metaphone encoding for blocking | BSD-3 license, lightweight, purpose-built for phonetic indexing |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/dlclark/metaphone3 | latest | Double Metaphone (98% accuracy vs 89%) | When standard Metaphone insufficient for name variations |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| adrg/strutil | smashedtoatoms/gofuzz | gofuzz has more algorithms but less mature, strutil is production-proven |
| nyaruka/phonenumbers | ttacon/libphonenumber | Both work; nyaruka more actively maintained and documented |
| tilotech/go-phonetics | dotcypress/phonetics | Similar feature set; tilotech has cleaner API |

**Installation:**
```bash
go get github.com/adrg/strutil@v0.3.1
go get github.com/nyaruka/phonenumbers@v1.6.8
go get github.com/tilotech/go-phonetics
```

## Architecture Patterns

### Recommended Project Structure
```
backend/internal/
├── dedup/                    # NEW: Deduplication engine
│   ├── normalizer.go         # Field normalization (email, phone, text)
│   ├── comparator.go         # Field comparison algorithms
│   ├── scorer.go             # Weighted scoring engine
│   ├── blocker.go            # Blocking strategy implementation
│   ├── detector.go           # Main detection orchestrator
│   └── types.go              # MatchRule, MatchResult, BlockingKey types
├── repo/
│   ├── matching_rule.go      # NEW: CRUD for matching rules
│   └── duplicate.go          # NEW: Duplicate pair storage
├── handler/
│   └── dedup.go              # NEW: Admin API for rules, scan triggers
└── entity/
    └── dedup.go              # NEW: MatchingRule, DuplicatePair entities
```

### Pattern 1: Type-Based Comparison Strategy
**What:** Automatically select comparison algorithm based on field type from metadata
**When to use:** For all field comparisons in duplicate detection

**Example:**
```go
// Source: Quantico CRM existing FieldType + strutil docs
type FieldComparator interface {
    Compare(a, b string) float64
    Normalize(value string) string
}

func GetComparatorForFieldType(fieldType entity.FieldType) FieldComparator {
    switch fieldType {
    case entity.FieldTypeEmail:
        return &EmailComparator{}
    case entity.FieldTypePhone:
        return &PhoneComparator{}
    case entity.FieldTypeVarchar, entity.FieldTypeText:
        return &FuzzyTextComparator{} // Jaro-Winkler
    default:
        return &ExactComparator{}
    }
}
```

### Pattern 2: Weighted Additive Scoring
**What:** Calculate overall match score using weighted sum of field scores
**When to use:** All duplicate pair scoring

**Example:**
```go
// Source: SCORING_DEEP_DIVE.md Fellegi-Sunter model
type MatchResult struct {
    Score          float64            // 0.0-1.0
    ConfidenceTier string             // "high", "medium", "low"
    FieldScores    map[string]float64 // Per-field breakdown
    MatchingFields []string           // Fields that matched
}

func (s *Scorer) CalculateScore(recordA, recordB map[string]any, rule *MatchingRule) *MatchResult {
    var totalScore, totalWeight float64
    fieldScores := make(map[string]float64)

    for _, fc := range rule.FieldConfigs {
        valA, okA := recordA[fc.FieldName].(string)
        valB, okB := recordB[fc.FieldName].(string)

        if !okA || !okB || valA == "" || valB == "" {
            continue // Skip empty fields (don't penalize)
        }

        comparator := GetComparatorForFieldType(fc.FieldType)
        score := comparator.Compare(comparator.Normalize(valA), comparator.Normalize(valB))
        fieldScores[fc.FieldName] = score
        totalScore += score * fc.Weight
        totalWeight += fc.Weight
    }

    if totalWeight == 0 {
        return &MatchResult{Score: 0}
    }

    overallScore := totalScore / totalWeight
    return &MatchResult{
        Score:          overallScore,
        ConfidenceTier: s.getTier(overallScore, rule),
        FieldScores:    fieldScores,
    }
}
```

### Pattern 3: Pre-Computed Blocking Indexes
**What:** Store Soundex/prefix values in dedicated columns, update on record save
**When to use:** For all entities with dedup enabled

**Example:**
```sql
-- Source: CONTEXT.md decision: Pre-computed indexes
ALTER TABLE contacts ADD COLUMN last_name_soundex TEXT;
ALTER TABLE contacts ADD COLUMN email_domain TEXT;
ALTER TABLE contacts ADD COLUMN last_name_prefix TEXT; -- First 3 chars

CREATE INDEX idx_contacts_soundex ON contacts(org_id, last_name_soundex);
CREATE INDEX idx_contacts_domain ON contacts(org_id, email_domain);
CREATE INDEX idx_contacts_prefix ON contacts(org_id, last_name_prefix);
```

### Anti-Patterns to Avoid
- **O(n^2) full comparison:** Always use blocking to reduce candidate pairs
- **Multiplicative scoring:** One zero field tanks entire score; use additive
- **Case-sensitive comparison:** Always normalize to lowercase before compare
- **Ignoring multi-value fields:** Must compare primary vs secondary emails per CONTEXT.md
- **Hard blocking limits:** Use soft limit with warning, not cutoff per CONTEXT.md

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Phone E.164 normalization | Regex-based parser | nyaruka/phonenumbers | Handles 200+ countries, edge cases, carrier detection |
| Jaro-Winkler algorithm | Custom implementation | adrg/strutil | Optimized, tested, handles Unicode properly |
| Soundex encoding | Character mapping function | tilotech/go-phonetics | Handles edge cases (vowels, H/W, etc.) |
| Email domain extraction | strings.Split | Standard + lowercasing | Domain extraction is simple, but add trim/lowercase |

**Key insight:** String similarity algorithms have subtle edge cases (Unicode, empty strings, long strings). Use battle-tested libraries rather than implementing from scratch.

## Common Pitfalls

### Pitfall 1: Phone Number Format Chaos
**What goes wrong:** Phones stored in mixed formats fail to match ("555-1234" vs "+15551234")
**Why it happens:** No normalization at comparison time; relies on input format
**How to avoid:** Always normalize to E.164 before comparison using phonenumbers library
**Warning signs:** Valid duplicates showing 0% phone match score

### Pitfall 2: Case Sensitivity in Email Matching
**What goes wrong:** "John@ACME.com" doesn't match "john@acme.com"
**Why it happens:** Forgot to lowercase before comparison
**How to avoid:** Normalize all text to lowercase in comparator.Normalize()
**Warning signs:** Same person appearing as different records

### Pitfall 3: Blocking Too Aggressively
**What goes wrong:** Miss duplicates that have typos in blocking field (e.g., "Smyth" vs "Smith")
**Why it happens:** Using only exact prefix blocking
**How to avoid:** Use multiple blocking strategies (Soundex + prefix + domain) and OR them
**Warning signs:** Known duplicates not appearing in scan results

### Pitfall 4: Empty Field Handling
**What goes wrong:** Records with sparse data always score low
**Why it happens:** Penalizing missing fields or dividing by total possible weight
**How to avoid:** Only count fields that BOTH records have; divide by actual weight used
**Warning signs:** New records always flagged as unique despite matching on available fields

### Pitfall 5: Cross-Entity Field Mapping
**What goes wrong:** Contact.email doesn't match Lead.emailAddress in cross-entity dedup
**Why it happens:** Field names differ between entity types
**How to avoid:** Store explicit field mappings in rule config per CONTEXT.md decision
**Warning signs:** Cross-entity rules returning 0 matches

## Code Examples

Verified patterns from official sources:

### Jaro-Winkler Comparison
```go
// Source: https://pkg.go.dev/github.com/adrg/strutil/metrics
import (
    "github.com/adrg/strutil/metrics"
    "strings"
)

type FuzzyTextComparator struct {
    jw *metrics.JaroWinkler
}

func NewFuzzyTextComparator() *FuzzyTextComparator {
    jw := metrics.NewJaroWinkler()
    jw.CaseSensitive = false
    return &FuzzyTextComparator{jw: jw}
}

func (c *FuzzyTextComparator) Normalize(value string) string {
    return strings.TrimSpace(strings.ToLower(value))
}

func (c *FuzzyTextComparator) Compare(a, b string) float64 {
    if a == "" || b == "" {
        return 0.0
    }
    return c.jw.Compare(a, b)
}
```

### Phone E.164 Normalization
```go
// Source: https://pkg.go.dev/github.com/nyaruka/phonenumbers
import (
    "github.com/nyaruka/phonenumbers"
    "strings"
)

type PhoneComparator struct {
    defaultRegion string
}

func NewPhoneComparator(region string) *PhoneComparator {
    return &PhoneComparator{defaultRegion: region}
}

func (c *PhoneComparator) Normalize(value string) string {
    num, err := phonenumbers.Parse(value, c.defaultRegion)
    if err != nil {
        return "" // Invalid phone
    }
    if !phonenumbers.IsValidNumber(num) {
        return ""
    }
    return phonenumbers.Format(num, phonenumbers.E164)
}

func (c *PhoneComparator) Compare(a, b string) float64 {
    normA := c.Normalize(a)
    normB := c.Normalize(b)

    if normA == "" || normB == "" {
        return 0.0 // Can't compare invalid phones
    }

    if normA == normB {
        return 1.0 // Exact match after normalization
    }

    // Check national number match (different country codes)
    numA, _ := phonenumbers.Parse(a, c.defaultRegion)
    numB, _ := phonenumbers.Parse(b, c.defaultRegion)

    if *numA.NationalNumber == *numB.NationalNumber {
        return 0.9 // Same number, different country code
    }

    return 0.0 // Different numbers
}
```

### Email Normalization and Comparison
```go
// Source: SCORING_DEEP_DIVE.md + standard Go patterns
import (
    "strings"
    "github.com/adrg/strutil/metrics"
)

type EmailComparator struct {
    jw *metrics.JaroWinkler
}

func NewEmailComparator() *EmailComparator {
    jw := metrics.NewJaroWinkler()
    jw.CaseSensitive = false
    return &EmailComparator{jw: jw}
}

func (c *EmailComparator) Normalize(value string) string {
    return strings.TrimSpace(strings.ToLower(value))
}

func (c *EmailComparator) Compare(a, b string) float64 {
    normA := c.Normalize(a)
    normB := c.Normalize(b)

    if normA == "" || normB == "" {
        return 0.0
    }

    if normA == normB {
        return 1.0 // Exact match
    }

    // Split into local and domain parts
    partsA := strings.Split(normA, "@")
    partsB := strings.Split(normB, "@")

    if len(partsA) != 2 || len(partsB) != 2 {
        return 0.0 // Invalid emails
    }

    localA, domainA := partsA[0], partsA[1]
    localB, domainB := partsB[0], partsB[1]

    // Exact local part match = high confidence (same person, different provider)
    if localA == localB {
        return 0.85
    }

    // Fuzzy local match with domain consideration
    localSim := c.jw.Compare(localA, localB)
    domainSim := 0.5
    if domainA == domainB {
        domainSim = 1.0
    }

    // Weight: 80% local, 20% domain
    return (localSim * 0.8) + (domainSim * 0.2)
}
```

### Soundex Blocking Key Generation
```go
// Source: https://github.com/tilotech/go-phonetics
import (
    "github.com/tilotech/go-phonetics"
    "strings"
)

type BlockingKeyGenerator struct{}

func (g *BlockingKeyGenerator) GetSoundexKey(name string) string {
    if name == "" {
        return ""
    }
    return phonetics.EncodeSoundex(strings.TrimSpace(name))
}

func (g *BlockingKeyGenerator) GetPrefixKey(name string, length int) string {
    name = strings.TrimSpace(strings.ToLower(name))
    if len(name) < length {
        return name
    }
    return name[:length]
}

func (g *BlockingKeyGenerator) GetDomainKey(email string) string {
    email = strings.TrimSpace(strings.ToLower(email))
    parts := strings.Split(email, "@")
    if len(parts) != 2 {
        return ""
    }
    return parts[1]
}

// Generate all blocking keys for a record
func (g *BlockingKeyGenerator) GenerateKeys(record map[string]any) map[string]string {
    keys := make(map[string]string)

    if lastName, ok := record["lastName"].(string); ok && lastName != "" {
        keys["last_name_soundex"] = g.GetSoundexKey(lastName)
        keys["last_name_prefix"] = g.GetPrefixKey(lastName, 3)
    }

    if email, ok := record["email"].(string); ok && email != "" {
        keys["email_domain"] = g.GetDomainKey(email)
    }

    return keys
}
```

### Matching Rule Database Schema
```sql
-- Source: CONTEXT.md decisions + existing Quantico pattern
CREATE TABLE matching_rules (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    entity_type TEXT NOT NULL,          -- "Contact", "Lead", or cross-entity
    target_entity_type TEXT,            -- For cross-entity rules (e.g., Contact-Lead)
    is_enabled INTEGER DEFAULT 1,
    priority INTEGER DEFAULT 0,         -- Lower = higher priority
    threshold REAL NOT NULL,            -- Minimum score to be considered match
    high_confidence_threshold REAL,     -- Auto-merge safe threshold
    medium_confidence_threshold REAL,   -- Needs review threshold
    blocking_strategy TEXT NOT NULL,    -- "soundex", "prefix", "exact", "ngram"
    field_configs TEXT NOT NULL,        -- JSON array of field match configs
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(org_id, entity_type, name)
);

CREATE INDEX idx_matching_rules_org ON matching_rules(org_id, entity_type, is_enabled);

-- Field config JSON structure:
-- [
--   {
--     "fieldName": "email",
--     "targetFieldName": "emailAddress",  -- For cross-entity (optional)
--     "weight": 40,
--     "algorithm": "email",  -- "exact", "jaro_winkler", "email", "phone", "phonetic"
--     "exactMatchBoost": true,  -- Auto-high confidence on exact match
--     "threshold": 0.88
--   }
-- ]
```

### Blocking Index Columns Migration
```sql
-- Source: CONTEXT.md decision: Pre-computed indexes updated on save
-- Add to standard entity tables (Contact, Lead, Account)

ALTER TABLE contacts ADD COLUMN dedup_last_name_soundex TEXT;
ALTER TABLE contacts ADD COLUMN dedup_last_name_prefix TEXT;
ALTER TABLE contacts ADD COLUMN dedup_email_domain TEXT;
ALTER TABLE contacts ADD COLUMN dedup_phone_e164 TEXT;

CREATE INDEX idx_contacts_dedup_soundex ON contacts(org_id, dedup_last_name_soundex);
CREATE INDEX idx_contacts_dedup_prefix ON contacts(org_id, dedup_last_name_prefix);
CREATE INDEX idx_contacts_dedup_domain ON contacts(org_id, dedup_email_domain);

-- Similar for leads, accounts, etc.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Simple Levenshtein | Jaro-Winkler for names | 2010s | Better prefix matching for names |
| Manual phone parsing | libphonenumber E.164 | 2015+ | Handles 200+ countries automatically |
| Single Soundex | Double Metaphone | 2000+ | 98% accuracy vs 89% for phonetic |
| Full scan | Blocking strategies | Always | O(n) blocks vs O(n^2) comparisons |
| Single threshold | Multi-tier confidence | 2020s | Enables auto-merge vs manual review |

**Deprecated/outdated:**
- Simple edit distance for names: Jaro-Winkler is preferred for short strings
- Multiplicative scoring: Additive weighted scoring is now standard
- Single blocking key: Multiple OR'd blocking keys catch more duplicates

## Open Questions

Things that couldn't be fully resolved:

1. **International Phone Number Default Region**
   - What we know: phonenumbers library requires a default region for parsing
   - What's unclear: Should this be org-level config or US default?
   - Recommendation: Make it org-configurable, default to "US"

2. **Multi-Value Email Matching Strategy**
   - What we know: CONTEXT.md requires matching primary vs secondary emails
   - What's unclear: Exact comparison matrix (primary-primary, primary-secondary, secondary-secondary)
   - Recommendation: Compare all combinations, take highest score

3. **Blocking Key Update Performance**
   - What we know: Pre-computed blocking indexes need updating on save
   - What's unclear: Impact on write performance for high-volume imports
   - Recommendation: Batch update during import, individual update on manual edit

## Sources

### Primary (HIGH confidence)
- [adrg/strutil](https://pkg.go.dev/github.com/adrg/strutil/metrics) - Jaro-Winkler API documentation
- [nyaruka/phonenumbers](https://pkg.go.dev/github.com/nyaruka/phonenumbers) - v1.6.8 Go libphonenumber port
- [tilotech/go-phonetics](https://github.com/tilotech/go-phonetics) - Soundex/Metaphone implementation
- `.planning/research/SCORING_DEEP_DIVE.md` - Prior project research on algorithms and thresholds

### Secondary (MEDIUM confidence)
- [Splink: Choosing Comparators](https://moj-analytical-services.github.io/splink/topic_guides/comparisons/choosing_comparators.html) - String comparison research
- [Splink: Phonetic Algorithms](https://moj-analytical-services.github.io/splink/topic_guides/comparisons/phonetic.html) - Blocking strategy guidance
- [Google libphonenumber](https://github.com/google/libphonenumber) - Reference implementation docs

### Tertiary (LOW confidence)
- Community blog posts on CRM deduplication patterns - verified against official sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Verified with pkg.go.dev, active maintainers, production usage
- Architecture: HIGH - Patterns from existing Quantico CRM codebase + prior research
- Pitfalls: MEDIUM - Based on prior research and common industry patterns

**Research date:** 2026-02-06
**Valid until:** 2026-03-06 (30 days - stable algorithms, library versions may update)
