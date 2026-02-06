# Phase 11: Detection Foundation - Context

**Gathered:** 2026-02-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Core deduplication infrastructure: similarity algorithms (Jaro-Winkler), field normalization (email, phone), configurable matching rules, blocking strategies. This phase builds the detection engine — merge, UI, import integration are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Matching Rule Design
- **Weighted scoring model**: Each field contributes to total score based on admin-assigned weight, threshold determines match
- **Cross-entity rules supported**: Rules can match across entity types (e.g., Contact-Lead dedup)
- **Rule priority**: Rules have priority order; first matching rule determines the score (no aggregation)
- **Multi-value field matching**: Support matching primary email vs primary, primary vs secondary, etc. for fields with multiple values

### Confidence Scoring
- **Admin sets field weights**: Admin assigns 0-100 weight per field in rule configuration
- **Per-rule thresholds**: Each rule defines its own minimum score to be considered a match
- **Per-rule confidence tiers**: Each rule can define its own High/Medium/Low tier boundaries
- **Exact match boost**: Admin can mark specific fields as "exact match = auto-high" to override scoring

### Blocking Strategy
- **All strategies available**: Soundex, prefix, exact, n-gram — admin picks per rule
- **Speed preset as default**: Aggressive blocking for faster performance (acceptable to miss edge cases)
- **Pre-computed indexes**: Soundex/prefixes stored in DB columns, updated on record save
- **Soft limit with warning**: Process large blocks but warn admin in results (no hard cutoff)

### Entity Support
- **All standard entities**: Contact, Account, Lead, and other standard entities support dedup out of the box
- **Custom entity opt-in**: Admin explicitly enables dedup for each custom entity
- **Type-based comparison defaults**: Email fields get email normalization, phone gets E.164, text gets fuzzy — automatic per field type
- **Explicit cross-entity mapping**: Admin maps fields between entities in rule config (e.g., Contact.email → Lead.emailAddress)

### Claude's Discretion
- Exact blocking index schema design
- Soundex implementation (standard vs double metaphone)
- Rule caching strategy
- Normalization edge cases (international phone formats, email subaddressing)

</decisions>

<specifics>
## Specific Ideas

- Multi-value email matching: "I would want to match the primary email to primary and secondary" — support comparing all email values, not just first one
- Rules should be flexible enough to handle CRM-specific patterns (Contact-Lead conversion scenarios)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 11-detection-foundation*
*Context gathered: 2026-02-06*
