# Duplicate Scoring Deep Dive

**Domain:** CRM Duplicate Detection Scoring Algorithms
**Researched:** 2026-02-05
**Overall Confidence:** HIGH (multiple authoritative sources cross-referenced)

---

## Executive Summary

This research provides specific algorithms, thresholds, and formulas for implementing CRM duplicate detection scoring. The findings are based on industry standards (Fellegi-Sunter probabilistic matching), real-world CRM implementations (Salesforce, HubSpot, Delpha), and open-source libraries (Splink, fuzzy-matcher).

**Key Recommendations:**
- Use Jaro-Winkler with **0.88 threshold** for names (balance between 0.85-0.90 range)
- Match first/last names **separately**, not combined
- Use **weighted additive scoring** (not multiplicative)
- Implement **negative signals** for high-discriminating fields
- Use **multi-level thresholds** rather than single cutoffs

---

## 1. Field-Specific Algorithm Recommendations

### 1.1 Personal Names

**Recommended Algorithm:** Jaro-Winkler + Phonetic Fallback

**Why Jaro-Winkler for Names:**
- Optimized for short strings
- Gives weight to matching prefixes (names often share prefixes like "John/Jonathan")
- Handles transpositions well ("Richar" vs "Richard")
- Better than Levenshtein for typical name typos

**Thresholds (based on Splink documentation):**

| Level | Jaro-Winkler Score | Interpretation | Weight Contribution |
|-------|-------------------|----------------|---------------------|
| Exact | 1.0 | Identical | Full weight |
| High | >= 0.92 | Minor typo ("Micheal" vs "Michael") | 90% weight |
| Medium | >= 0.88 | Common variation ("Rick" vs "Richard") | 70% weight |
| Low | >= 0.80 | Possible match, needs verification | 40% weight |
| None | < 0.80 | No match | 0% or negative |

**Critical: Match First/Last Separately**

Research indicates matching full names combined produces inflated scores when one component is long. Always compare:
- `first_name_a` vs `first_name_b` (Jaro-Winkler)
- `last_name_a` vs `last_name_b` (Jaro-Winkler)
- Combine scores with equal weight (or slight preference to last name)

**Phonetic Matching (Secondary):**

Use Double Metaphone as fallback when Jaro-Winkler fails but names "sound" similar:
- "Philip" and "Phillip" => Same Metaphone code "FLP"
- "Stephen" and "Steven" => Same Metaphone code "STFN"

**Recommended Approach:**
```
function nameScore(name_a, name_b):
    jw = jaroWinkler(name_a, name_b)
    if jw >= 0.88:
        return jw

    // Phonetic fallback
    if doubleMetaphone(name_a) == doubleMetaphone(name_b):
        return 0.75  // Phonetic match bonus

    return jw
```

**Sources:**
- [Splink: Choosing String Comparators](https://moj-analytical-services.github.io/splink/topic_guides/comparisons/choosing_comparators.html)
- [Datablist: Jaro-Winkler Distance](https://www.datablist.com/learn/data-cleaning/fuzzy-matching-jaro-winkler-distance)

---

### 1.2 Email Addresses

**Recommended Algorithm:** Domain-Aware Split Matching

**Key Insight:** The local part (before @) is highly discriminating; the domain is not.

**Scoring Strategy:**

```
function emailScore(email_a, email_b):
    if email_a == email_b:
        return 1.0  // Exact match

    local_a, domain_a = split(email_a, '@')
    local_b, domain_b = split(email_b, '@')

    // Exact local match with different domain (common)
    if local_a == local_b:
        return 0.85  // Same person, different email provider

    // Fuzzy local match
    local_sim = jaroWinkler(local_a, local_b)

    // Domain matching (lower weight)
    domain_sim = domain_a == domain_b ? 1.0 : 0.5

    // Weighted combination: 80% local, 20% domain
    return (local_sim * 0.8) + (domain_sim * 0.2)
```

**Special Cases:**
- **Catch-all corporate emails:** "john@company.com" and "john.smith@company.com" may be same person
- **Common providers:** Gmail, Yahoo, Outlook domains should not boost score significantly
- **Typos in domain:** "gmail.com" vs "gmial.com" should still match

**Thresholds:**

| Match Type | Score | Confidence |
|------------|-------|------------|
| Exact | 1.0 | Definite match |
| Same local, different domain | 0.85 | High confidence |
| Fuzzy local >= 0.90, same domain | 0.80 | High confidence |
| Fuzzy local >= 0.85, same domain | 0.65 | Medium confidence |
| Different | < 0.50 | Low/no match |

**Sources:**
- [Intuit fuzzy-matcher](https://github.com/intuit/fuzzy-matcher) - Email handling strips domain
- [Data Ladder: Fuzzy Matching Guide](https://dataladder.com/fuzzy-matching-101/)

---

### 1.3 Phone Numbers

**Recommended Algorithm:** E.164 Normalization + Exact Match

**Critical First Step:** Normalize to E.164 format before any comparison.

```
// All these should normalize to: +13125551234
"(312) 555-1234"
"312-555-1234"
"+1 312 555 1234"
"001 312 555 1234"
"1-312-555-1234"
```

**Recommended Library:** Google's libphonenumber (or ports: phonenumbers for Python, google-libphonenumber for JS)

**Scoring Strategy:**

```
function phoneScore(phone_a, phone_b):
    norm_a = normalizeE164(phone_a)
    norm_b = normalizeE164(phone_b)

    if !norm_a or !norm_b:
        return null  // Invalid phone, skip

    if norm_a == norm_b:
        return 1.0  // Exact match

    // Check for country code variations
    national_a = extractNationalNumber(norm_a)
    national_b = extractNationalNumber(norm_b)

    if national_a == national_b:
        return 0.90  // Same number, different/missing country code

    // Partial match (last 7 digits) - risky but sometimes useful
    if last7(norm_a) == last7(norm_b):
        return 0.60  // Possible match, needs verification

    return 0.0  // No match
```

**Important Considerations:**
- Phone numbers are **binary** (match or don't) after normalization
- Fuzzy matching on phones is dangerous (one digit off = different person)
- Missing country codes are common - default to org's primary country
- Mobile vs landline distinction can be informative

**Sources:**
- [Google libphonenumber](https://github.com/google/libphonenumber)
- [Insycle: Phone Number Formatting](https://blog.insycle.com/phone-number-formatting-crm)

---

### 1.4 Addresses

**Recommended Algorithm:** Multi-Component Weighted Matching

Addresses are complex and require decomposition:

```
Address Components:
- Street number (exact or close numeric match)
- Street name (fuzzy string match)
- Unit/Apt (exact match if present)
- City (fuzzy or exact)
- State/Province (exact)
- Postal code (exact or prefix match)
- Country (exact)
```

**Component-Specific Algorithms:**

| Component | Algorithm | Threshold | Weight |
|-----------|-----------|-----------|--------|
| Street Number | Damerau-Levenshtein | 1 edit | 0.20 |
| Street Name | Jaro-Winkler | 0.85 | 0.35 |
| Unit/Apt | Exact (normalized) | 1.0 | 0.10 |
| City | Jaro-Winkler | 0.90 | 0.15 |
| Postal Code | Prefix match (3+ chars) | 0.85 | 0.15 |
| Country | Exact | 1.0 | 0.05 |

**Normalization Requirements (before matching):**
```
- Lowercase
- "Street" => "St", "Avenue" => "Ave", "Road" => "Rd"
- "Apartment" => "Apt", "Suite" => "Ste", "Unit" => "#"
- Remove punctuation
- Trim whitespace
```

**Scoring Formula:**
```
function addressScore(addr_a, addr_b):
    components = ['street_num', 'street_name', 'unit', 'city', 'postal', 'country']
    weights = [0.20, 0.35, 0.10, 0.15, 0.15, 0.05]

    total_score = 0
    total_weight = 0

    for i, comp in enumerate(components):
        val_a = normalize(addr_a[comp])
        val_b = normalize(addr_b[comp])

        if val_a and val_b:
            score = compareComponent(comp, val_a, val_b)
            total_score += score * weights[i]
            total_weight += weights[i]

    return total_score / total_weight if total_weight > 0 else null
```

**Sources:**
- [Placekey: Address Matching](https://www.placekey.io/blog/address-matching)
- [O'Reilly: Fuzzy Data Matching](https://www.oreilly.com/library/view/fuzzy-data-matching/9781098152260/ch04.html)

---

### 1.5 Company Names

**Recommended Algorithm:** Token-Based Jaccard + Suffix Stripping

Company names require special handling due to:
- Legal suffixes (Inc, LLC, Corp, Ltd)
- Common abbreviations
- Word order variations
- DBA names

**Pre-processing:**
```
function normalizeCompany(name):
    name = lowercase(name)
    name = removePunctuation(name)
    name = stripSuffixes(name, ['inc', 'llc', 'corp', 'ltd', 'co', 'company',
                                 'incorporated', 'corporation', 'limited'])
    name = expandAbbreviations(name)  // "intl" => "international"
    return name
```

**Scoring Strategy:**

```
function companyScore(name_a, name_b):
    norm_a = normalizeCompany(name_a)
    norm_b = normalizeCompany(name_b)

    // Exact match after normalization
    if norm_a == norm_b:
        return 1.0

    // Token-based Jaccard similarity
    tokens_a = tokenize(norm_a)
    tokens_b = tokenize(norm_b)

    jaccard = len(intersection(tokens_a, tokens_b)) / len(union(tokens_a, tokens_b))

    // Also compute Jaro-Winkler for short names
    jw = jaroWinkler(norm_a, norm_b)

    // Take the higher score (Jaccard better for long names, JW for short)
    return max(jaccard, jw)
```

**Thresholds:**

| Score | Interpretation |
|-------|----------------|
| >= 0.90 | Very likely same company |
| >= 0.75 | Possible match, verify |
| >= 0.60 | Weak match, needs review |
| < 0.60 | Different companies |

**Sources:**
- [Data Ladder: Handling Abbreviations](https://dataladder.com/managing-nicknames-abbreviations-variants-in-entity-matching/)
- [Baeldung: Token-Based Matching](https://www.baeldung.com/cs/string-similarity-token-methods)

---

## 2. Overall Scoring Formula

### 2.1 Recommended: Weighted Additive Scoring

Based on the Fellegi-Sunter model and real-world CRM implementations (Delpha, Dedupely), use weighted additive scoring:

```
Overall Score = (Σ field_score_i * field_weight_i) / (Σ field_weight_i)
```

**Why Not Multiplicative?**
- One weak field would tank the entire score
- Missing fields would multiply to zero
- Additive allows graceful degradation

### 2.2 Recommended Field Weights for CRM

| Field | Weight | Rationale |
|-------|--------|-----------|
| Email | 100 | Highly unique, rarely shared |
| Phone | 80 | Unique per person, some sharing |
| Last Name | 60 | Discriminating, but common names exist |
| First Name | 40 | Less discriminating |
| Company | 50 | Important for B2B, varies |
| Address (full) | 40 | Can be shared (family, roommates) |
| Date of Birth | 70 | Highly discriminating if available |
| LinkedIn URL | 90 | Unique identifier |

### 2.3 Handling Missing Fields

**Critical:** Do not penalize for missing fields unless marked mandatory.

```
function calculateOverallScore(record_a, record_b, field_configs):
    total_score = 0
    total_weight = 0

    for field in field_configs:
        val_a = record_a[field.name]
        val_b = record_b[field.name]

        // Skip if either is empty (unless mandatory)
        if isEmpty(val_a) or isEmpty(val_b):
            if field.mandatory:
                // Penalize: treat as mismatch
                total_score += 0
                total_weight += field.weight
            continue  // Skip optional empty fields

        score = field.compare(val_a, val_b)
        total_score += score * field.weight
        total_weight += field.weight

    return total_score / total_weight if total_weight > 0 else 0
```

**Example Calculation:**

```
Record A: {email: "john@acme.com", first: "John", last: "Smith", phone: null}
Record B: {email: "john@acme.com", first: "Jon", last: "Smith", phone: null}

Field Scores:
- Email: 1.0 * 100 = 100
- First Name: 0.85 (JW) * 40 = 34
- Last Name: 1.0 * 60 = 60
- Phone: skipped (both null)

Overall = (100 + 34 + 60) / (100 + 40 + 60) = 194/200 = 0.97 = 97%
```

---

## 3. Negative Signals

### 3.1 Concept: Counter-Weights

Some fields should **reduce** the score when they differ significantly. These are "discriminating" fields where a mismatch is strong evidence of different entities.

### 3.2 Recommended Negative Signals

| Field | Trigger | Penalty | Rationale |
|-------|---------|---------|-----------|
| Company Name | Different (< 0.5 match) | -30% | Same person rarely at different companies simultaneously |
| Date of Birth | Different | -50% | Unique to individual |
| Country | Different | -20% | Same person unlikely in multiple countries |
| Gender | Different | -40% | Fundamental identifier |
| Industry | Different | -10% | Weak signal but informative |

### 3.3 Implementation

```
function calculateWithNegativeSignals(record_a, record_b, field_configs):
    base_score = calculateOverallScore(record_a, record_b, field_configs)

    // Apply negative signals
    for field in negative_signal_fields:
        val_a = record_a[field.name]
        val_b = record_b[field.name]

        if isEmpty(val_a) or isEmpty(val_b):
            continue  // Can't penalize if data missing

        sim = field.compare(val_a, val_b)
        if sim < field.mismatch_threshold:
            base_score = base_score * (1 - field.penalty)

    return max(0, base_score)
```

**Example:**
```
Base score: 0.85 (85%)
Company mismatch detected: "Acme Inc" vs "Beta Corp" (sim = 0.15)
Penalty: 30%

Final score: 0.85 * (1 - 0.30) = 0.85 * 0.70 = 0.595 = 59.5%
```

### 3.4 When NOT to Apply Negative Signals

- When data is frequently stale (company field for job changers)
- When field is optional and often empty
- In B2C contexts where company may not apply

---

## 4. Threshold Tuning

### 4.1 The Precision-Recall Tradeoff

| Threshold | Effect on Precision | Effect on Recall |
|-----------|---------------------|------------------|
| Higher (e.g., 90%) | More precision (fewer false positives) | Less recall (more missed duplicates) |
| Lower (e.g., 70%) | Less precision (more false positives) | More recall (catch more duplicates) |

### 4.2 Recommended Thresholds by Use Case

**Conservative (Minimize False Positives):**
- Auto-merge threshold: **95%**
- Review threshold: **80%**
- Ignore below: **70%**

**Balanced (General CRM Use):**
- Auto-merge threshold: **90%**
- Review threshold: **75%**
- Ignore below: **60%**

**Aggressive (Maximize Duplicate Detection):**
- Auto-merge threshold: **85%**
- Review threshold: **65%**
- Ignore below: **50%**

### 4.3 Multi-Tier Threshold Strategy

Recommended approach from Splink and enterprise tools:

```
thresholds = {
    'definite_match': 0.95,    // Auto-merge safe
    'probable_match': 0.85,    // High confidence, quick review
    'possible_match': 0.70,    // Needs human review
    'unlikely_match': 0.55,    // Show only if requested
    'no_match': 0.55           // Below this, ignore
}
```

### 4.4 Converting to Probability (Fellegi-Sunter)

Match weights can be converted to probability:

```
probability = 2^match_weight / (1 + 2^match_weight)

| Match Weight | Probability |
|--------------|-------------|
| 0 | 50% |
| 2 | 80% |
| 4 | 94% |
| 7 | 99.2% |
| 10 | 99.9% |
```

---

## 5. Performance Optimization

### 5.1 The Scale Problem

Comparing every record to every other record:
- 10,000 records = 50 million comparisons
- 100,000 records = 5 billion comparisons
- 1 million records = 500 billion comparisons

### 5.2 Blocking Strategy

**Blocking reduces comparisons by only comparing records that share a "blocking key."**

**Recommended Blocking Keys for CRM:**

| Blocking Key | Example | Reduction |
|--------------|---------|-----------|
| Email domain | "acme.com" | 90-99% |
| First 3 chars of last name | "SMI" | 95%+ |
| Postal code | "60601" | 99%+ |
| Phone area code | "312" | 95%+ |
| Soundex of last name | "S530" | 90%+ |

**Multi-Block Strategy:**
```
Run comparisons for records sharing ANY of:
1. Same email domain
2. Same first 3 chars of normalized last name
3. Same postal code
4. Same phone area code

This catches duplicates even if one blocking field is missing/different.
```

### 5.3 Early Termination

Stop scoring early if match is impossible:

```
function scoreWithEarlyTermination(record_a, record_b, min_threshold):
    max_possible_score = 1.0
    current_score = 0
    current_weight = 0
    total_weight = sum(all_weights)

    for field in fields_by_weight_descending:
        val_a = record_a[field.name]
        val_b = record_b[field.name]

        if isEmpty(val_a) or isEmpty(val_b):
            max_possible_score -= field.weight / total_weight
            continue

        score = field.compare(val_a, val_b)
        current_score += score * field.weight
        current_weight += field.weight

        // Early termination: can we still reach threshold?
        remaining_weight = total_weight - current_weight
        best_possible = (current_score + remaining_weight) / total_weight

        if best_possible < min_threshold:
            return null  // Can't possibly match

    return current_score / current_weight
```

### 5.4 Indexing Strategy

**Pre-compute blocking keys:**
```sql
CREATE INDEX idx_contacts_blocking ON contacts(
    org_id,
    LOWER(SUBSTR(last_name, 1, 3)),
    email_domain
);
```

**Pre-compute phonetic codes:**
```sql
ALTER TABLE contacts ADD COLUMN last_name_soundex TEXT;
UPDATE contacts SET last_name_soundex = soundex(last_name);
CREATE INDEX idx_contacts_soundex ON contacts(org_id, last_name_soundex);
```

---

## 6. Recommended Implementation

### 6.1 Go Scoring Engine Structure

```go
type FieldConfig struct {
    Name             string
    Weight           float64
    Algorithm        string  // "jaro_winkler", "exact", "phone", "email", "jaccard"
    Threshold        float64
    Mandatory        bool
    NegativeSignal   bool
    MismatchPenalty  float64
}

type ScoringConfig struct {
    Fields              []FieldConfig
    AutoMergeThreshold  float64
    ReviewThreshold     float64
    MinimumThreshold    float64
}

var DefaultCRMConfig = ScoringConfig{
    Fields: []FieldConfig{
        {Name: "email", Weight: 100, Algorithm: "email", Mandatory: false},
        {Name: "phone", Weight: 80, Algorithm: "phone", Mandatory: false},
        {Name: "lastName", Weight: 60, Algorithm: "jaro_winkler", Threshold: 0.88},
        {Name: "firstName", Weight: 40, Algorithm: "jaro_winkler", Threshold: 0.88},
        {Name: "company", Weight: 50, Algorithm: "company", NegativeSignal: true, MismatchPenalty: 0.30},
    },
    AutoMergeThreshold: 0.90,
    ReviewThreshold:    0.75,
    MinimumThreshold:   0.60,
}
```

### 6.2 Score Calculation Flow

```
1. BLOCKING: Find candidate pairs using blocking keys
2. NORMALIZE: Clean all field values
3. COMPARE: Calculate per-field similarity scores
4. COMBINE: Weight and sum field scores
5. PENALIZE: Apply negative signals for mismatches
6. CLASSIFY: Bucket into definite/probable/possible/no match
7. RETURN: Sorted list of duplicate candidates
```

---

## 7. Summary: Quick Reference

### Field Algorithm Cheat Sheet

| Field Type | Algorithm | Threshold | Notes |
|------------|-----------|-----------|-------|
| First Name | Jaro-Winkler | 0.88 | + Metaphone fallback |
| Last Name | Jaro-Winkler | 0.88 | + Metaphone fallback |
| Email | Split (local/domain) | Local: 0.90 | Weight local 80%, domain 20% |
| Phone | E.164 + Exact | 1.0 | Binary after normalization |
| Company | Jaccard + suffix strip | 0.75 | Token-based for long names |
| Address | Multi-component | Varies | Street name most important |

### Weight Recommendations

| Field | Suggested Weight | Negative Signal? |
|-------|------------------|------------------|
| Email | 100 | No |
| LinkedIn | 90 | No |
| Phone | 80 | No |
| DOB | 70 | Yes (-50% penalty) |
| Last Name | 60 | No |
| Company | 50 | Yes (-30% penalty) |
| First Name | 40 | No |
| Address | 40 | No |

### Threshold Recommendations

| Category | Threshold | Action |
|----------|-----------|--------|
| Definite Match | >= 95% | Safe to auto-merge |
| Probable Match | >= 85% | Quick review, likely merge |
| Possible Match | >= 70% | Needs human review |
| Unlikely Match | >= 55% | Show only on request |
| No Match | < 55% | Ignore |

---

## Sources

### Authoritative (HIGH Confidence)
- [Splink: Fellegi-Sunter Model](https://moj-analytical-services.github.io/splink/topic_guides/theory/fellegi_sunter.html)
- [Splink: Choosing String Comparators](https://moj-analytical-services.github.io/splink/topic_guides/comparisons/choosing_comparators.html)
- [Intuit fuzzy-matcher](https://github.com/intuit/fuzzy-matcher)
- [Google libphonenumber](https://github.com/google/libphonenumber)

### Industry Practice (MEDIUM Confidence)
- [Delpha: Duplicate Scoring](https://help.delpha.io/delpha-for-salesforce/how-to-faq/delpha-duplicate/how-does-the-duplicate-scoring-work)
- [Salesforce: Matching Criteria](https://help.salesforce.com/s/articleView?id=sales.matching_rule_matching_criteria.htm)
- [HubSpot: AI Duplication Management](https://blog.hubspot.com/customers/behind-the-buzzwords-whats-under-the-hood-of-your-ai-powered-duplication-management-tool)
- [Data Ladder: Fuzzy Matching Guide](https://dataladder.com/fuzzy-matching-101/)

### Research Papers (HIGH Confidence for Theory)
- [Fellegi-Sunter Mathematics](https://www.robinlinacre.com/maths_of_fellegi_sunter/)
- [NCBI: Record Linkage Overview](https://www.ncbi.nlm.nih.gov/books/NBK253312/)
- [Jaro-Winkler Distance](https://en.wikipedia.org/wiki/Jaro%E2%80%93Winkler_distance)
- [Phonetic Algorithm Overview](https://en.wikipedia.org/wiki/Phonetic_algorithm)
