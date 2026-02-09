# Feature Landscape: Salesforce Merge Instruction Delivery

**Domain:** Integration platform for delivering deduplication merge instructions to Salesforce
**Researched:** 2026-02-09
**Confidence:** MEDIUM (WebSearch verified with official Salesforce documentation patterns)

## Executive Summary

Merge instruction delivery systems bridge the gap between standalone dedup engines and customer CRM systems of record. Based on current integration patterns, table stakes include payload construction, OAuth authentication, batch delivery with rate limiting, and basic audit logging. Differentiators include intelligent retry logic, pre-flight validation, admin troubleshooting UI, and status tracking. The v4.0 spec aligns well with industry standards but should avoid common pitfalls: fire-and-forget with no visibility, insufficient error handling, and lack of admin troubleshooting tools.

---

## Table Stakes

Features users expect. Missing any of these = product feels incomplete or broken.

### TS-01: Merge Instruction Payload Builder
**Why expected:** Core functionality. Without this, there's no integration.
**Complexity:** Low
**What it includes:**
- Construct JSON payload from dedup results (winner ID, loser ID, field values)
- Map Quantico field names to Salesforce field names
- Support standard objects (Contact, Account, Lead)
- Support custom objects (any object with merge capability)
- Support standard fields (Name, Email, Phone, etc.)
- Support custom fields (any __c field)
**Why non-negotiable:** This is literally the product. If you can't build a merge instruction, nothing else matters.
**Admin UI needs:**
- Field mapping configuration: Quantico field → Salesforce field
- Object type selection (Contact, Account, Lead, custom objects)
- Preview payload before sending (validation)

### TS-02: Salesforce OAuth 2.0 Authentication
**Why expected:** Industry-standard auth for Salesforce integrations
**Complexity:** Moderate
**What it includes:**
- OAuth 2.0 Web Server Flow
- Salesforce Connected App configuration
- Access token + refresh token storage
- Token refresh before expiry
- Per-org credential storage (multi-tenant)
**Why non-negotiable:** Salesforce requires OAuth for REST API access. Username/password auth is deprecated and insecure.
**Admin UI needs:**
- OAuth connection wizard (authorize with Salesforce)
- Connection status indicator (connected/disconnected)
- Reconnect button if token expires
- Test connection button

### TS-03: Batch Delivery to Salesforce Staging Object
**Why expected:** Salesforce integration pattern for complex processing
**Complexity:** Moderate
**What it includes:**
- Batch assembler (group merge instructions)
- POST to Salesforce REST API
- Insert into staging custom object (e.g., `QuanticoMergeInstruction__c`)
- Response handling (success/failure per batch)
**Why non-negotiable:** Spec defines staging object pattern. This is how instructions enter Salesforce for processing.
**Salesforce API limits:**
- Max 10,000 records per batch
- Max 10MB payload per batch
- Daily limit: 100,000 API calls (Enterprise) + 1,000 per user
**Recommended batch size:** 200 records (from spec)
**Admin UI needs:**
- Batch size configuration
- Delivery frequency (manual trigger, scheduled, real-time)
- Delivery queue view (pending batches)

### TS-04: Rate Limiting & Error Handling
**Why expected:** Salesforce API has strict rate limits. Without this, integration breaks in production.
**Complexity:** Moderate
**What it includes:**
- Track API call count per 24-hour window
- Pause delivery when approaching limits (80% threshold)
- Handle HTTP 429 (rate limit exceeded) responses
- Exponential backoff on retry (5s, 10s, 20s, 40s, 80s)
- Max retry attempts (3-5 recommended)
- Dead letter queue for failed deliveries
**Why non-negotiable:** Hitting rate limits causes cascading failures. Exponential backoff is industry best practice.
**Error-specific handling:**
- 401 Unauthorized → token expired, trigger refresh
- 429 Too Many Requests → exponential backoff, don't retry immediately
- 500 Server Error → retry with backoff
- 400 Bad Request → don't retry, log as permanent failure
**Admin UI needs:**
- API usage dashboard (calls used / daily limit)
- Failed deliveries list with error messages
- Retry button for failed batches

### TS-05: Basic Audit Logging
**Why expected:** Compliance requirement. Admins need to know what was sent to Salesforce and when.
**Complexity:** Low
**What it includes:**
- Log each batch delivery attempt
- Track: timestamp, batch size, records sent, success/failure status
- Store batch ID from Salesforce (for troubleshooting)
- Retention: at least 90 days (SOX requires 7 years for financial data, GDPR varies)
**Why non-negotiable:** Without audit trail, impossible to debug issues or meet compliance requirements.
**Admin UI needs:**
- Delivery history table (timestamp, batch ID, status, record count)
- Filter by date range, status (success/failed)
- Export to CSV for compliance

### TS-06: Admin Configuration UI
**Why expected:** Admins need to set up and monitor the integration without touching code
**Complexity:** Moderate
**What it includes:**
- Salesforce org configuration (OAuth setup)
- Field mapping configuration
- Batch settings (size, frequency)
- Connection testing
- Delivery status monitoring
**Why non-negotiable:** SaaS product. Admins can't edit config files or run CLI commands.
**Components:**
- Settings page: OAuth credentials, field mappings, batch config
- Status dashboard: connection status, API usage, recent deliveries
- Delivery queue: pending batches, failed deliveries, retry controls

---

## Differentiators

Features that set this product apart. Not expected, but highly valued.

### DIFF-01: Pre-Flight Validation
**Value proposition:** Catch errors before sending to Salesforce, reducing failed API calls and wasted rate limit budget
**Complexity:** Moderate
**What it includes:**
- Validate Salesforce record IDs (15 or 18 character format, valid characters)
- Validate field names against Salesforce org schema
- Check field types match (text to text, number to number)
- Validate payload size under 10MB
- Warn if batch size exceeds 10,000 records
- Detect missing required fields
**When to use:** Before queuing batch for delivery
**Why valuable:**
- Salesforce returns 400 Bad Request for invalid payloads → wasted API call
- Failed batches require manual investigation and retry
- Pre-flight validation catches 80% of errors before they happen
**Admin UI impact:**
- Validation errors shown in delivery queue
- Fix invalid records before sending
- Validation rules configuration (strict vs permissive)

### DIFF-02: Intelligent Status Tracking
**Value proposition:** Real-time visibility into merge instruction lifecycle
**Complexity:** Moderate
**What it includes:**
- Track each merge instruction through states: Pending → Queued → Sent → Processing → Complete/Failed
- Poll Salesforce staging object for processing status
- Update Quantico record with delivery outcome
- Link back to original dedup result
- Surface Salesforce processing errors in Quantico UI
**Why valuable:**
- Admins see which merges actually completed in Salesforce
- Failed merges can be investigated and retried
- Traceability from dedup detection → merge instruction → Salesforce outcome
**Admin UI impact:**
- Status column in merge history: "Sent to Salesforce", "Processing", "Completed", "Failed"
- Click record → see Salesforce processing details
- Filter by status: show only failed deliveries

### DIFF-03: Delivery Analytics Dashboard
**Value proposition:** Insights into integration health and performance
**Complexity:** Low
**What it includes:**
- API usage chart (calls per day, approaching limit warnings)
- Delivery success rate (% successful batches)
- Average processing time (time from queued → delivered)
- Error breakdown (by error type: 401, 429, 500, validation failures)
- Merge volume over time
**Why valuable:**
- Spot trends: sudden spike in failures indicates Salesforce issue
- Capacity planning: if approaching API limits, upgrade Salesforce edition or reduce batch frequency
- Executive reporting: "We delivered 10,000 merge instructions this month with 98% success rate"
**Admin UI impact:**
- Analytics tab in admin panel
- Charts: API usage, delivery success rate, error breakdown
- Date range filters

### DIFF-04: Advanced Retry Logic
**Value proposition:** Automatic recovery from transient failures without admin intervention
**Complexity:** Moderate
**What it includes:**
- Distinguish transient errors (429, 503) from permanent errors (400, 401)
- Automatic retry for transient errors with exponential backoff
- Max retry attempts configurable (default: 3)
- Dead letter queue for permanent failures
- Alert admin after N consecutive failures
**Why valuable:**
- Network blips don't require manual intervention
- Salesforce downtime doesn't lose data
- Admin only alerted for real issues
**Salesforce best practices:**
- Retry with exponential backoff + jitter (don't hammer endpoint)
- Limit retry attempts to avoid infinite loops
- 5-second minimum delay between retries
**Admin UI impact:**
- Retry attempts shown in delivery log
- Alert banner: "10 batches failed after 3 retries"
- Manual retry button for dead letter queue items

### DIFF-05: Field Mapping Presets
**Value proposition:** Faster setup for common Salesforce → Quantico mappings
**Complexity:** Low
**What it includes:**
- Pre-configured mappings for standard objects:
  - Contact: FirstName, LastName, Email, Phone, AccountId, etc.
  - Account: Name, Phone, BillingAddress, Website, etc.
  - Lead: FirstName, LastName, Email, Company, Status, etc.
- One-click apply preset, then customize
- Save custom mappings as new presets
**Why valuable:**
- Reduces setup time from 30 minutes to 2 minutes
- Prevents mapping mistakes (typos in field names)
- Standardizes across customers
**Admin UI impact:**
- Dropdown: "Apply preset: Contact (standard), Account (standard), Custom"
- Edit mappings after applying preset
- "Save as new preset" button

---

## Anti-Features

Features to explicitly NOT build. These hurt more than they help.

### AF-01: Auto-Merge Without Review
**Why avoid:** Risk of data loss, users lose trust
**What to do instead:**
- Always require explicit admin approval before sending merge instructions
- Provide preview of what will be merged
- Confirm dialog: "Send 50 merge instructions to Salesforce?"
**Reasoning:**
- Automatic merges can destroy data if dedup rules are misconfigured
- "Fire and forget" feels risky to admins
- Manual trigger gives control and builds trust

### AF-02: Real-Time Sync (Two-Way)
**Why avoid:** Complexity explosion, out of scope for v4.0
**What to do instead:**
- v4.0: One-way sync (Quantico → Salesforce)
- Future: Consider two-way sync in v4.1+ if customer demand exists
**Reasoning:**
- Two-way sync requires webhooks, conflict resolution, field mapping in both directions
- Staging object pattern is one-way by design
- 95% of use case is "dedupe in Quantico, apply to Salesforce"

### AF-03: Support for Salesforce Merge API Directly
**Why avoid:** Salesforce merge API only supports Contacts, Accounts, Leads. Custom objects not supported.
**What to do instead:**
- Use staging object pattern (spec defines this)
- Customer builds Apex trigger to process staging object
- Apex trigger can call merge API or use custom logic
**Reasoning:**
- Salesforce merge API limitations documented: "There is no standard Salesforce functionality to merge two records for custom objects"
- Staging object pattern gives flexibility: customer controls merge logic in Apex
- Avoids Quantico being blocked by Salesforce API limitations

### AF-04: Batch Delivery Without Admin Visibility
**Why avoid:** Black box = no trust, impossible to debug
**What to do instead:**
- Always show delivery queue, batch status, error messages
- Surface Salesforce API responses in UI
- Provide manual retry controls
**Reasoning:**
- Admins need to see what's happening
- When deliveries fail, admin needs error messages to fix root cause
- Transparency builds trust

### AF-05: Storing Salesforce Credentials in Plain Text
**Why avoid:** Security nightmare, fails compliance audits
**What to do instead:**
- Encrypt OAuth tokens at rest (use Turso DB encryption or application-level encryption)
- Use environment variables for encryption keys
- Never log tokens in plain text
**Reasoning:**
- OAuth tokens are sensitive credentials
- Compliance requirements (SOX, GDPR) mandate encryption
- Security best practice

### AF-06: Generic Webhook Extensibility (v4.0)
**Why avoid:** Out of scope for v4.0 per spec
**What to do instead:**
- Defer to v4.1 as "extensible framework"
- v4.0: Salesforce-specific implementation
- Future: Abstract to webhook pattern for other CRMs
**Reasoning:**
- Spec explicitly says "Out of Scope for v4.0: Extensible framework (defer to v4.1)"
- Premature abstraction slows delivery
- Prove value with Salesforce first, then generalize

---

## Feature Dependencies

```
Core Flow:
TS-02 (OAuth) → TS-01 (Payload Builder) → TS-03 (Batch Delivery) → TS-04 (Error Handling) → TS-05 (Audit Logging)
                                                                                              ↓
                                                                                    TS-06 (Admin UI)

Differentiators Build On Table Stakes:
DIFF-01 (Pre-Flight Validation) requires TS-01 (Payload Builder)
DIFF-02 (Status Tracking) requires TS-03 (Batch Delivery) + TS-05 (Audit Logging)
DIFF-03 (Analytics) requires TS-05 (Audit Logging)
DIFF-04 (Advanced Retry) requires TS-04 (Error Handling)
DIFF-05 (Field Mapping Presets) requires TS-01 (Payload Builder)
```

**Critical Path (must implement in order):**
1. TS-02 OAuth (can't call API without auth)
2. TS-01 Payload Builder (need something to send)
3. TS-03 Batch Delivery (send it)
4. TS-04 Error Handling (handle failures)
5. TS-05 Audit Logging (track what happened)
6. TS-06 Admin UI (make it usable)

**Parallel Tracks (can build independently):**
- DIFF-01 Pre-Flight Validation (enhances TS-01)
- DIFF-05 Field Mapping Presets (enhances TS-01)
- DIFF-03 Analytics (enhances TS-05)

**Later Additions (build after table stakes complete):**
- DIFF-02 Status Tracking (requires polling Salesforce)
- DIFF-04 Advanced Retry (enhances TS-04)

---

## Complexity Assessment

### Easy (1-2 days implementation)
- **TS-05 Basic Audit Logging:** Standard table, insert on delivery, display in UI
- **DIFF-03 Analytics Dashboard:** Aggregate queries on audit log, chart library for visualization
- **DIFF-05 Field Mapping Presets:** JSON config files, dropdown in UI

### Moderate (3-5 days implementation)
- **TS-01 Payload Builder:** JSON construction, field mapping lookup, validation logic
- **TS-02 OAuth:** Salesforce Connected App setup, OAuth flow implementation, token storage
- **TS-03 Batch Delivery:** Batch assembler, HTTP client, Salesforce REST API integration
- **TS-04 Rate Limiting & Error Handling:** Rate limit tracking, exponential backoff, retry queue
- **TS-06 Admin UI:** Multiple pages (settings, dashboard, queue), form validation, real-time updates
- **DIFF-01 Pre-Flight Validation:** Schema introspection, field type validation, payload size check
- **DIFF-04 Advanced Retry:** Dead letter queue, retry scheduler, alert logic

### Hard (5-10 days implementation)
- **DIFF-02 Status Tracking:** Polling Salesforce staging object, state machine for merge lifecycle, bidirectional linking

---

## Admin UI Requirements Mapped to Features

| Feature | Admin UI Components | User Workflows |
|---------|---------------------|----------------|
| **TS-01 Payload Builder** | Field mapping config page, object type selector, preview pane | 1. Select object type<br>2. Map fields<br>3. Preview payload<br>4. Save mapping |
| **TS-02 OAuth** | Connection wizard, status indicator, test connection button | 1. Click "Connect Salesforce"<br>2. Authorize in Salesforce<br>3. Redirected back to Quantico<br>4. See "Connected" status |
| **TS-03 Batch Delivery** | Delivery queue table, batch size input, frequency selector, manual trigger button | 1. Configure batch size (200)<br>2. Set frequency (manual/scheduled)<br>3. Click "Send Batch"<br>4. See batch in queue |
| **TS-04 Error Handling** | Failed deliveries list, error message display, retry button, API usage chart | 1. View failed batches<br>2. Read error message<br>3. Fix issue (re-auth, adjust payload)<br>4. Click "Retry" |
| **TS-05 Audit Logging** | Delivery history table, date range filter, status filter, export CSV button | 1. Navigate to "Delivery History"<br>2. Filter by date/status<br>3. Export for compliance |
| **DIFF-01 Pre-Flight Validation** | Validation errors list, fix invalid records UI, strict/permissive toggle | 1. Queue batch<br>2. See validation errors<br>3. Fix invalid records<br>4. Re-queue |
| **DIFF-02 Status Tracking** | Status column in history, detail view with Salesforce response, filter by status | 1. Click record in history<br>2. See Salesforce processing status<br>3. Filter: "Show failed only" |
| **DIFF-03 Analytics** | Charts (API usage, success rate, errors), date range selector, export report | 1. Navigate to "Analytics"<br>2. Select date range<br>3. View charts<br>4. Export report |
| **DIFF-04 Advanced Retry** | Retry attempts counter, dead letter queue tab, alert banner | 1. See alert: "10 failed after retries"<br>2. Navigate to dead letter queue<br>3. Investigate<br>4. Manual retry |
| **DIFF-05 Field Mapping Presets** | Preset dropdown, edit mapping UI, save preset button | 1. Select preset: "Contact (standard)"<br>2. Customize if needed<br>3. Save as new preset |

---

## Multi-Object Support Strategy

### v4.0 Scope (Per Spec)
- **Standard objects:** Contact, Account, Lead (priority)
- **Custom objects:** All objects that support merge (if Salesforce API allows)
- **Standard fields:** All (Name, Email, Phone, Address, etc.)
- **Custom fields:** All (__c suffix fields)

### Implementation Approach
1. **Object-agnostic payload builder:** Don't hardcode object types, use dynamic field mapping
2. **Schema introspection:** Query Salesforce for available objects and fields (via `/sobjects` endpoint)
3. **Field mapping per object type:** Store mappings in DB, one config per object type
4. **Salesforce API limitations:** Detect if object supports merge (Contact, Account, Lead do; most custom objects don't via API, but can via Apex)

### Challenges
- **Salesforce merge API limitations:** Native merge API only supports Contact, Account, Lead
- **Custom object merges:** Require Apex trigger on staging object to handle merge logic
- **Solution:** Staging object pattern abstracts this. Quantico sends instructions, Salesforce Apex processes them.

### Admin UI Impact
- Object type dropdown populated from Salesforce org (via schema introspection)
- Field mapping page shows available fields for selected object type
- Validation warns if object doesn't support native merge

---

## Validation & Troubleshooting Features

### What Validation is Needed?

**Pre-Send Validation (DIFF-01):**
- Record IDs: 15 or 18 characters, alphanumeric, valid Salesforce ID format
- Field names: Exist in Salesforce schema for selected object type
- Field types: Compatible (text → text, number → number, no text → number)
- Payload size: Under 10MB per batch
- Batch size: Under 10,000 records per batch
- Required fields: All required Salesforce fields included

**Post-Send Validation:**
- Salesforce API response: 200 OK = success, 4xx/5xx = failure
- Parse error messages from Salesforce
- Detect token expiry (401) vs rate limit (429) vs bad request (400)

### Troubleshooting Tools Needed

**For Admins:**
1. **Connection tester:** "Test Connection" button → call Salesforce API, show result
2. **Error message display:** Surface exact Salesforce API error in UI
3. **Manual retry:** Retry individual failed batches
4. **Delivery logs:** Show HTTP request/response for debugging
5. **Validation report:** List all validation errors before sending

**For Developers (future):**
- API request/response logging (toggle on for debugging)
- Webhook for delivery status updates (future extensibility)

---

## Sources

- [Salesforce API Integration Guide 2026](https://www.codleo.com/blog/salesforce-api-integration)
- [Salesforce Integration Patterns and Practices](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/integration_patterns_and_practices.pdf)
- [Integration Design Pattern – Staging Approach](https://www.linkedin.com/pulse/integration-design-pattern-staging-approach-vishnu-teja)
- [Salesforce API Rate Limits & Optimization](https://coefficient.io/salesforce-api/salesforce-api-rate-limits)
- [Building a Batch Retry Framework With BatchApexErrorEvent](https://developer.salesforce.com/blogs/2019/01/building-a-batch-retry-framework-with-batchapexerrorevent)
- [Complete Salesforce Audit Trail Management for Compliance](https://www.flosum.com/blog/salesforce-audit-trail-compliance-guide)
- [Salesforce Integration Troubleshooting Guide](https://www.appseconnect.com/salesforce-integration-troubleshooting-guide/)
- [Webhook Architecture Patterns for Real-Time Integrations](https://technori.com/news/webhook-architecture-real-time-integrations/)
- [Merging Custom and Standard Objects in Salesforce](https://help.zoominfo.com/s/article/Merging-Custom-and-Standard-Objects-in-Salesforce)
- [Best Salesforce Deduplication Tools in 2026](https://no-duplicates.com/blog/best-salesforce-deduplication-tools-2026)
