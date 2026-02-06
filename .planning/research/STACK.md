# Stack Research: Deduplication System

**Project:** Quantico CRM - Deduplication Milestone
**Researched:** 2026-02-05
**Focus:** Record deduplication and fuzzy matching for Go/Turso backend

---

## Executive Summary

For scoring-based duplicate detection in a Go 1.22/Fiber backend with Turso (SQLite), I recommend:

1. **String Similarity:** `github.com/adrg/strutil` - Clean API, all needed algorithms, actively maintained
2. **Phone Normalization:** `github.com/nyaruka/phonenumbers` - Google libphonenumber port, production-tested
3. **Phonetic Matching:** Turso's built-in SQLean Fuzzy extension for Soundex in SQL; Go-side with strutil for Jaro-Winkler
4. **Background Jobs:** In-process goroutine workers (no Redis dependency needed for this scale)

Key insight: Turso already includes SQLean Fuzzy v0.24.1, providing `fuzzy_soundex()`, `fuzzy_leven()`, and `fuzzy_jaro_winkler()` directly in SQL. This eliminates the need to pull all data into Go for comparison.

---

## Recommended Libraries

### Core String Similarity

| Library | Version | Purpose | Why This One |
|---------|---------|---------|--------------|
| `github.com/adrg/strutil` | v0.3.1 | String similarity metrics | Clean interface, 8 algorithms, configurable costs, MIT license. Best API design of Go options. |
| `github.com/nyaruka/phonenumbers` | v1.6.8 | Phone normalization | Direct port of Google's libphonenumber, used in production daily, E.164 format support |

### Installation

```bash
cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend
go get github.com/adrg/strutil@v0.3.1
go get github.com/nyaruka/phonenumbers@v1.6.8
```

### Alternatives Considered

| Category | Recommended | Alternative | Why Not Alternative |
|----------|-------------|-------------|---------------------|
| String similarity | strutil | go-edlib | go-edlib has more algorithms but strutil's interface is cleaner; both work |
| String similarity | strutil | smetrics | smetrics has fewer algorithms; strutil is more complete |
| Phone parsing | phonenumbers | dongri/phonenumber | phonenumbers is the full libphonenumber port; dongri is simpler but less accurate |

---

## String Similarity Algorithms

### Which Algorithm for Which Field

| Field Type | Primary Algorithm | Threshold | Rationale |
|------------|-------------------|-----------|-----------|
| **Names** | Jaro-Winkler | 0.85+ | Favors prefix matches ("John Smith" vs "Jon Smith"), handles typos well |
| **Emails** | Exact match on domain, Levenshtein on local part | 2 edits max | Domains must match exactly; local parts can have typos |
| **Company names** | Jaro-Winkler + token sort | 0.80+ | Word order may vary ("Acme Corp" vs "Corp Acme") |
| **Addresses** | Jaccard on tokens | 0.70+ | Address components may be abbreviated or reordered |

### Algorithm Reference

**Jaro-Winkler** (PRIMARY for names):
- Returns 0-1 similarity score
- Weights prefix matches heavily (first 4 chars)
- Best for: Person names, company names
- Threshold: 0.85+ is likely duplicate, 0.90+ is high confidence

```go
import (
    "github.com/adrg/strutil"
    "github.com/adrg/strutil/metrics"
)

func NameSimilarity(a, b string) float64 {
    jw := metrics.NewJaroWinkler()
    jw.CaseSensitive = false
    return strutil.Similarity(a, b, jw)
}
```

**Levenshtein** (for exact-ish matches):
- Returns edit distance (number of changes needed)
- Configurable insert/delete/replace costs
- Best for: Email local parts, short strings
- Threshold: 0-2 edits for likely duplicate

```go
func EmailLocalPartSimilarity(a, b string) int {
    lev := metrics.NewLevenshtein()
    // Returns raw distance, not similarity
    dist := lev.Distance(a, b)
    return dist
}
```

**Jaccard** (for tokenized data):
- Compares sets of n-grams or tokens
- Best for: Addresses, descriptions, multi-word fields
- Threshold: 0.70+ overlap suggests similarity

```go
func AddressSimilarity(a, b string) float64 {
    jacc := metrics.NewJaccard()
    jacc.NgramSize = 2 // bigrams
    return strutil.Similarity(a, b, jacc)
}
```

### Scoring Strategy

Combine multiple field scores with weights:

```go
type DuplicateScore struct {
    Name    float64 // Weight: 0.35
    Email   float64 // Weight: 0.30
    Phone   float64 // Weight: 0.25
    Address float64 // Weight: 0.10
    Total   float64 // Weighted sum
}

func CalculateDuplicateScore(record1, record2 Record) DuplicateScore {
    score := DuplicateScore{
        Name:    NameSimilarity(record1.Name, record2.Name),
        Email:   EmailSimilarity(record1.Email, record2.Email),
        Phone:   PhoneSimilarity(record1.Phone, record2.Phone),
        Address: AddressSimilarity(record1.Address, record2.Address),
    }

    score.Total = (score.Name * 0.35) +
                  (score.Email * 0.30) +
                  (score.Phone * 0.25) +
                  (score.Address * 0.10)
    return score
}
```

---

## Phonetic Matching

### Turso Built-in Functions (SQLean Fuzzy)

Turso includes SQLean Fuzzy v0.24.1 which provides:

| Function | Purpose | Example |
|----------|---------|---------|
| `fuzzy_soundex(text)` | 4-char phonetic code | `fuzzy_soundex('Smith')` -> 'S530' |
| `fuzzy_leven(a, b)` | Levenshtein distance | `fuzzy_leven('john', 'jon')` -> 1 |
| `fuzzy_jaro_winkler(a, b)` | Similarity 0-1 | `fuzzy_jaro_winkler('john', 'jon')` -> 0.93 |
| `fuzzy_translit(text)` | Unicode to ASCII | `fuzzy_translit('Muller')` -> 'Muller' |

**Use Case:** Pre-filter candidates in SQL before detailed Go-side comparison:

```sql
-- Find contacts with similar-sounding names
SELECT id, name, email
FROM contacts
WHERE fuzzy_soundex(name) = fuzzy_soundex('John Smith')
   OR fuzzy_jaro_winkler(name, 'John Smith') > 0.8
```

### When to Use Phonetic vs String Distance

| Scenario | Use Phonetic (Soundex) | Use String Distance (Jaro-Winkler) |
|----------|------------------------|-----------------------------------|
| "Jon" vs "John" | Yes - same sound | Also works |
| "Smith" vs "Smyth" | Yes - same sound | Also works |
| "Mike" vs "Michael" | No - different sounds | No - different strings |
| "IBM" vs "I.B.M." | No - acronyms | Yes - remove punctuation first |
| Typos | No | Yes - this is the strength |

**Recommendation:** Use Soundex as a **blocking** strategy (find candidates), then Jaro-Winkler for **scoring** (rank candidates).

---

## Phone Number Normalization

### Library: nyaruka/phonenumbers

This is the Go port of Google's libphonenumber, providing:
- Parsing from any format ("(555) 123-4567", "555.123.4567", "+1 555 123 4567")
- Normalization to E.164 ("+15551234567")
- Validation per country
- Carrier lookup, timezone mapping

```go
import "github.com/nyaruka/phonenumbers"

func NormalizePhone(raw, defaultCountry string) (string, error) {
    num, err := phonenumbers.Parse(raw, defaultCountry)
    if err != nil {
        return "", err
    }

    if !phonenumbers.IsValidNumber(num) {
        return "", fmt.Errorf("invalid phone number: %s", raw)
    }

    // E.164 format: +15551234567
    return phonenumbers.Format(num, phonenumbers.E164), nil
}
```

### Phone Comparison Strategy

1. **Normalize both numbers** to E.164 at ingest time
2. **Store normalized** in a separate column (`phone_normalized`)
3. **Compare normalized** values for exact match
4. **Handle partial matches** (last 7 digits) for legacy data

```go
// Phone matching is binary after normalization
func PhonesMatch(a, b string) bool {
    normA, errA := NormalizePhone(a, "US")
    normB, errB := NormalizePhone(b, "US")

    if errA != nil || errB != nil {
        // Fall back to last 7 digits comparison
        return Last7Digits(a) == Last7Digits(b)
    }

    return normA == normB
}

func Last7Digits(phone string) string {
    digits := regexp.MustCompile(`\d`).FindAllString(phone, -1)
    if len(digits) >= 7 {
        return strings.Join(digits[len(digits)-7:], "")
    }
    return strings.Join(digits, "")
}
```

---

## SQLite/Turso Considerations

### What Turso Provides

Turso (libSQL fork of SQLite) includes these relevant extensions:

| Extension | Version | Relevant Functions |
|-----------|---------|-------------------|
| SQLean Fuzzy | v0.24.1 | `fuzzy_soundex`, `fuzzy_leven`, `fuzzy_jaro_winkler`, `fuzzy_translit` |
| FTS5 | Built-in | Full-text search (not fuzzy, but useful for blocking) |

### Architecture: SQL Blocking + Go Scoring

**Problem:** You cannot efficiently compare every record to every other record in Go (O(n^2) comparisons).

**Solution:** Use SQL for "blocking" (finding candidates), Go for detailed scoring.

```
                                    Blocking (SQL)                    Scoring (Go)

                 +-----------+      +-------------------+      +------------------+
New Record  ---> | Normalize | ---> | Find Candidates   | ---> | Calculate Scores |
                 +-----------+      | (Soundex, prefix) |      | (Jaro-Winkler)   |
                                    +-------------------+      +------------------+
                                           |                          |
                                           v                          v
                                    ~50-100 candidates         Ranked duplicates
                                    (fast, approximate)        (accurate, detailed)
```

### Blocking Strategies (SQL)

**1. Soundex Blocking:**
```sql
-- Find contacts with same Soundex code
SELECT id, name, email, phone
FROM contacts
WHERE fuzzy_soundex(first_name) = fuzzy_soundex(?)
  AND org_id = ?
LIMIT 100
```

**2. Prefix Blocking:**
```sql
-- Find contacts with same name prefix
SELECT id, name, email, phone
FROM contacts
WHERE LOWER(SUBSTR(first_name, 1, 3)) = LOWER(SUBSTR(?, 1, 3))
  AND org_id = ?
LIMIT 100
```

**3. Email Domain Blocking:**
```sql
-- Find contacts with same email domain
SELECT id, name, email, phone
FROM contacts
WHERE SUBSTR(email, INSTR(email, '@')) = SUBSTR(?, INSTR(?, '@'))
  AND org_id = ?
LIMIT 100
```

**4. Phone Suffix Blocking:**
```sql
-- Find contacts with same last 4 digits
SELECT id, name, email, phone
FROM contacts
WHERE phone_normalized LIKE ?
  AND org_id = ?
LIMIT 100
```

### Index Strategy

```sql
-- Create indexes for blocking queries
CREATE INDEX idx_contacts_soundex_first ON contacts(org_id, fuzzy_soundex(first_name));
CREATE INDEX idx_contacts_name_prefix ON contacts(org_id, LOWER(SUBSTR(first_name, 1, 3)));
CREATE INDEX idx_contacts_phone_norm ON contacts(org_id, phone_normalized);
CREATE INDEX idx_contacts_email_domain ON contacts(org_id, SUBSTR(email, INSTR(email, '@')));
```

**Note on Expression Indexes:** SQLite supports indexes on expressions (like `fuzzy_soundex(first_name)`), but verify Turso supports this in production. If not, compute and store the Soundex value in a separate column.

---

## Performance Architecture

### Background Scanning Pattern

For duplicate detection, you need two modes:

1. **Real-time on ingest:** Check new/updated records against existing
2. **Background full scan:** Periodic scan of entire entity table

### In-Process Worker (Recommended for Turso)

**Why not Redis-based queues (Asynq)?**
- Adds operational complexity (Redis dependency)
- Turso databases are per-tenant; job queue would need to know which DB to connect to
- For duplicate scanning, in-process workers are sufficient

**Pattern: Goroutine Worker Pool**

```go
// internal/dedup/scanner.go

type DuplicateScanner struct {
    db          *sql.DB
    concurrency int
    batchSize   int
}

func NewDuplicateScanner(db *sql.DB) *DuplicateScanner {
    return &DuplicateScanner{
        db:          db,
        concurrency: 4,  // Workers processing batches
        batchSize:   100, // Records per batch
    }
}

func (s *DuplicateScanner) ScanEntity(ctx context.Context, entityType string) error {
    // Get total count
    total, err := s.countRecords(ctx, entityType)
    if err != nil {
        return err
    }

    // Create job channel
    jobs := make(chan ScanBatch, s.concurrency*2)
    results := make(chan []DuplicatePair, s.concurrency*2)

    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < s.concurrency; i++ {
        wg.Add(1)
        go s.worker(ctx, jobs, results, &wg)
    }

    // Feed batches
    go func() {
        for offset := 0; offset < total; offset += s.batchSize {
            jobs <- ScanBatch{
                EntityType: entityType,
                Offset:     offset,
                Limit:      s.batchSize,
            }
        }
        close(jobs)
    }()

    // Collect results
    go func() {
        wg.Wait()
        close(results)
    }()

    // Process results
    for pairs := range results {
        if err := s.saveDuplicatePairs(ctx, pairs); err != nil {
            log.Printf("Error saving duplicate pairs: %v", err)
        }
    }

    return nil
}
```

### Real-Time Checking (On Ingest)

```go
// Called when creating/updating a record
func (s *DuplicateService) CheckForDuplicates(ctx context.Context, record Record) ([]DuplicateCandidate, error) {
    // 1. Block: Find candidates via SQL
    candidates, err := s.findCandidates(ctx, record)
    if err != nil {
        return nil, err
    }

    // 2. Score: Calculate similarity for each candidate
    var duplicates []DuplicateCandidate
    for _, candidate := range candidates {
        score := s.calculateScore(record, candidate)
        if score.Total >= 0.7 { // Configurable threshold
            duplicates = append(duplicates, DuplicateCandidate{
                Record: candidate,
                Score:  score,
            })
        }
    }

    // 3. Sort by score descending
    sort.Slice(duplicates, func(i, j int) bool {
        return duplicates[i].Score.Total > duplicates[j].Score.Total
    })

    return duplicates, nil
}
```

### Performance Targets

| Operation | Target | Notes |
|-----------|--------|-------|
| Real-time check (on ingest) | <200ms | Block via SQL, score top 50 candidates |
| Background batch (100 records) | <5s | Parallel scoring across 4 workers |
| Full entity scan (10K records) | <30 min | Run nightly, interruptible |

---

## Configuration Schema

```go
// internal/entity/dedup_config.go

type DuplicateRuleConfig struct {
    ID           string              `json:"id"`
    OrgID        string              `json:"orgId"`
    EntityType   string              `json:"entityType"`
    Name         string              `json:"name"`
    Description  string              `json:"description"`
    Active       bool                `json:"active"`

    // Field matching rules
    Rules        []FieldMatchRule    `json:"rules"`

    // Thresholds
    AutoMergeThreshold   float64     `json:"autoMergeThreshold"`   // e.g., 0.95
    ReviewThreshold      float64     `json:"reviewThreshold"`      // e.g., 0.70

    // Blocking strategy
    BlockingFields       []string    `json:"blockingFields"`       // Fields for SQL blocking

    CreatedAt    time.Time           `json:"createdAt"`
    UpdatedAt    time.Time           `json:"updatedAt"`
}

type FieldMatchRule struct {
    FieldName    string              `json:"fieldName"`
    MatchType    MatchType           `json:"matchType"`    // exact, fuzzy, phonetic
    Weight       float64             `json:"weight"`       // 0.0 - 1.0
    Threshold    float64             `json:"threshold"`    // Minimum score to count as match
    Algorithm    string              `json:"algorithm"`    // jaro_winkler, levenshtein, soundex
    Options      map[string]any      `json:"options"`      // Algorithm-specific options
}

type MatchType string

const (
    MatchTypeExact    MatchType = "exact"
    MatchTypeFuzzy    MatchType = "fuzzy"
    MatchTypePhonetic MatchType = "phonetic"
    MatchTypeNone     MatchType = "none"
)
```

---

## Migration Schema

```sql
-- Duplicate detection rules
CREATE TABLE duplicate_rules (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    active INTEGER DEFAULT 1,
    rules TEXT NOT NULL,  -- JSON array of FieldMatchRule
    auto_merge_threshold REAL DEFAULT 0.95,
    review_threshold REAL DEFAULT 0.70,
    blocking_fields TEXT,  -- JSON array of field names
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, entity_type, name)
);

-- Detected duplicate pairs
CREATE TABLE duplicate_pairs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    record1_id TEXT NOT NULL,
    record2_id TEXT NOT NULL,
    score REAL NOT NULL,
    score_breakdown TEXT,  -- JSON of per-field scores
    status TEXT DEFAULT 'pending',  -- pending, merged, dismissed
    rule_id TEXT,
    detected_at TEXT DEFAULT CURRENT_TIMESTAMP,
    resolved_at TEXT,
    resolved_by TEXT,
    UNIQUE(org_id, entity_type, record1_id, record2_id)
);

CREATE INDEX idx_duplicate_pairs_status ON duplicate_pairs(org_id, entity_type, status);
CREATE INDEX idx_duplicate_pairs_record ON duplicate_pairs(org_id, record1_id);
```

---

## Summary of Recommendations

| Category | Choice | Rationale |
|----------|--------|-----------|
| **String Similarity** | `github.com/adrg/strutil` v0.3.1 | Clean API, all needed algorithms, actively maintained |
| **Phone Normalization** | `github.com/nyaruka/phonenumbers` v1.6.8 | Google libphonenumber port, E.164 support |
| **Phonetic (SQL)** | Turso SQLean Fuzzy (built-in) | Already available, no extra setup |
| **Background Jobs** | In-process goroutine workers | Simpler than Redis queue, sufficient for per-tenant DBs |
| **Primary Algorithm** | Jaro-Winkler | Best for name matching, handles typos and variations |
| **Blocking Strategy** | SQL Soundex + prefix | Reduces candidates before expensive Go-side scoring |

---

## Sources

### HIGH Confidence (Official Documentation)

- [strutil GitHub Repository](https://github.com/adrg/strutil) - String similarity metrics library
- [phonenumbers GitHub Repository](https://github.com/nyaruka/phonenumbers) - Go port of libphonenumber
- [Turso SQLite Extensions Documentation](https://docs.turso.tech/features/sqlite-extensions) - Built-in SQLean Fuzzy

### MEDIUM Confidence (Verified with Multiple Sources)

- [go-edlib GitHub Repository](https://github.com/hbollon/go-edlib) - Alternative string comparison library
- [SQLean Fuzzy Documentation](https://github.com/nalgeon/sqlean/blob/main/docs/fuzzy.md) - Fuzzy matching functions
- [Asynq GitHub Repository](https://github.com/hibiken/asynq) - Redis-based task queue (considered but not recommended)

### LOW Confidence (Community Sources)

- [Task Queues in Go Comparison](https://medium.com/@geisonfgfg/task-queues-in-go-asynq-vs-machinery-vs-work-powering-background-jobs-in-high-throughput-systems-45066a207aa7) - Asynq vs Machinery comparison
- [Fuzzy Matching 101 Guide](https://matchdatapro.com/fuzzy-matching-101-a-complete-guide-for-2025/) - General fuzzy matching best practices
- [CRM Deduplication Guide 2025](https://www.rtdynamic.com/blog/crm-deduplication-guide-2025/) - CRM-specific deduplication patterns
