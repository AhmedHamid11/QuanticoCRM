# Research Summary: Salesforce Merge Instruction Delivery

**Domain:** Integration platform for delivering deduplication merge instructions to Salesforce
**Researched:** 2026-02-09
**Overall confidence:** MEDIUM

WebSearch findings verified against official Salesforce documentation patterns (Integration Patterns and Practices v66.0 Spring '26), Salesforce API limits documentation, and established integration best practices. No Context7 coverage for this domain (integration-specific, not library-specific). Medium confidence reflects reliance on verified WebSearch sources rather than official Quantico-to-Salesforce integration docs (which don't exist yet).

## Executive Summary

Salesforce merge instruction delivery is a well-established integration pattern with clear industry best practices. The v4.0 spec aligns closely with these standards: OAuth 2.0 authentication, staging object pattern for complex processing, batch delivery with rate limiting, and audit logging are all table stakes for production integrations.

Key findings from research:

1. **Staging Object Pattern is Standard:** Salesforce integration best practices recommend using staging custom objects where external systems insert data with lightweight ETL logic, while all processing is handled by Salesforce Apex. This avoids row locks, transaction timeouts, and invalid data issues. The v4.0 spec correctly adopts this pattern.

2. **Rate Limiting is Critical:** Salesforce enforces strict API limits (100,000 daily calls for Enterprise Edition, 10,000 records per batch, 10MB payload limit). Integrations must track usage, implement exponential backoff for 429 responses, and pause at 80% capacity to avoid cascading failures.

3. **Error Handling Requires Intelligence:** Not all errors warrant the same response. Transient errors (429 Too Many Requests, 503 Service Unavailable) require exponential backoff and retry. Permanent errors (400 Bad Request, 401 Unauthorized) should not retry and require admin intervention. Industry best practice: max 3-5 retries, 5-second minimum delay, exponential backoff with jitter.

4. **Audit Trail is Compliance Requirement:** SOX requires 7-year retention for financial data, GDPR requires audit trails for duration of lawful processing. Salesforce native logging expires after 180 days (Setup Audit Trail) or 18 months (Field History). External audit logs are mandatory for compliance, especially for merge operations that modify customer data.

5. **Admin Visibility is Non-Negotiable:** Integration troubleshooting requires surfacing Salesforce API error messages, showing delivery queue status, providing manual retry controls, and displaying API usage metrics. Black-box integrations fail in production because admins can't debug issues.

6. **Custom Object Merge Limitations:** Salesforce merge API only supports Contact, Account, and Lead. Custom objects require Apex triggers to process merge logic. Staging object pattern abstracts this limitation: Quantico sends instructions, Salesforce Apex decides how to merge.

## Key Findings

**Stack:** OAuth 2.0 + Salesforce REST API + Staging Custom Object + Apex Trigger (Salesforce-side)

**Architecture:** One-way batch delivery with staging object pattern. Quantico queues merge instructions → batches them → POSTs to Salesforce REST API → inserts into staging object → Salesforce Apex trigger processes merges asynchronously.

**Critical pitfall:** Fire-and-forget delivery with no admin visibility. Leads to black-box failures, impossible troubleshooting, and loss of customer trust.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Core Integration (4-5 days)
**Addresses:** TS-02 OAuth, TS-01 Payload Builder, TS-03 Batch Delivery
**Avoids:** Premature optimization. Build basic end-to-end flow first.
**Rationale:** Can't test anything without OAuth + payload + delivery. Get happy path working before adding error handling sophistication.

### Phase 2: Error Handling & Reliability (3-4 days)
**Addresses:** TS-04 Rate Limiting, Error Handling, Retry Logic, DIFF-04 Advanced Retry
**Avoids:** Hitting rate limits in production, cascading failures from network blips
**Rationale:** Production reliability depends on intelligent error handling. Phase 1 proves concept, Phase 2 makes it production-ready.

### Phase 3: Audit & Compliance (2-3 days)
**Addresses:** TS-05 Audit Logging, compliance retention
**Avoids:** Compliance violations, impossible troubleshooting
**Rationale:** Audit trail is table stakes for compliance and debugging. Relatively simple to implement (insert to audit table on delivery).

### Phase 4: Admin UI & Visibility (5-7 days)
**Addresses:** TS-06 Admin Configuration UI, DIFF-02 Status Tracking, DIFF-03 Analytics
**Avoids:** Black-box integration, admin frustration
**Rationale:** Makes integration usable for non-technical admins. Most complex phase (multiple UI pages, real-time updates, charts).

### Phase 5: Quality & Validation (2-3 days)
**Addresses:** DIFF-01 Pre-Flight Validation, DIFF-05 Field Mapping Presets
**Avoids:** Wasting API calls on invalid payloads, slow onboarding
**Rationale:** Polish. Catches errors before sending, speeds up setup time.

**Phase ordering rationale:**
- **1 → 2 → 3** forms critical path: integration → reliability → compliance. Can't skip any.
- **Phase 4** depends on 1-3 being complete (UI displays data from audit logs, delivery queue, etc.)
- **Phase 5** is optional polish but high ROI (validation saves API calls, presets reduce setup time)

**Research flags for phases:**
- **Phase 1:** Likely needs deeper research on Salesforce Connected App setup (OAuth flow specifics, callback URL configuration)
- **Phase 2:** Standard patterns, unlikely to need additional research
- **Phase 3:** Standard patterns, unlikely to need additional research
- **Phase 4:** May need UI/UX research on admin dashboard best practices (polling vs SSE for real-time updates, chart library selection)
- **Phase 5:** Standard patterns, unlikely to need additional research

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | OAuth 2.0 + REST API + staging object is documented Salesforce pattern |
| Features | MEDIUM | WebSearch verified with official Salesforce docs, but no direct Quantico-to-Salesforce integration examples |
| Architecture | HIGH | Staging object pattern is official Salesforce best practice (Integration Patterns v66.0) |
| Pitfalls | MEDIUM | Industry best practices documented, but Quantico-specific pitfalls unknown until implementation |

**HIGH confidence areas:** Based on official Salesforce documentation (Integration Patterns and Practices, API Limits, OAuth flows)

**MEDIUM confidence areas:** Based on verified WebSearch findings (integration troubleshooting guides, deduplication tools, webhook patterns) but not official Salesforce-to-Quantico integration docs

**No LOW confidence findings:** All research verified against multiple sources

## Gaps to Address

### Gaps Resolved During Research
- ✓ Batch size optimization (200 per spec, aligns with Salesforce limits)
- ✓ Rate limiting strategy (exponential backoff confirmed as best practice)
- ✓ Audit retention requirements (SOX 7 years, GDPR variable)
- ✓ Custom object support (staging object pattern handles it)

### Gaps Requiring Phase-Specific Research

**Phase 1 (Core Integration):**
- Salesforce Connected App configuration specifics (OAuth callback URL, scopes required)
- Staging object schema design (fields needed, indexes for performance)
- Field mapping storage schema (how to store Quantico field → Salesforce field mappings)

**Phase 4 (Admin UI):**
- Real-time update mechanism (polling vs Server-Sent Events for delivery queue status)
- Chart library selection (lightweight, SvelteKit-compatible, supports time series)
- UI/UX patterns for OAuth connection flow (modal vs full page, error states)

**Future (out of v4.0 scope):**
- Webhook extensibility pattern (defer to v4.1 per spec)
- Bidirectional sync (Salesforce → Quantico, complex conflict resolution)
- Multi-CRM support (abstract Salesforce-specific logic)

### Open Questions for Implementation

1. **Salesforce staging object ownership:** Should Quantico provide Apex package to customers, or do customers build their own staging object and Apex trigger?
   - **Recommendation:** Provide reference Apex code as documentation, but don't manage it. Too much Salesforce-side complexity.

2. **Token refresh strategy:** Refresh on expiry (reactive) or before expiry (proactive)?
   - **Recommendation:** Proactive. Refresh when token has <5 minutes remaining to avoid mid-batch expiry.

3. **Batch delivery timing:** Manual trigger only, or scheduled?
   - **Recommendation:** Both. Manual for testing, scheduled for production (hourly/daily).

4. **Failed batch retry:** Automatic or manual?
   - **Recommendation:** Automatic for transient errors (429, 503), manual for permanent errors (400, 401).

5. **Multi-tenant credential storage:** Per-org OAuth tokens in Turso DB?
   - **Recommendation:** Yes, encrypted at rest. Use Turso DB encryption or application-level encryption.

---

**Next Steps:** Use this research to populate requirements definitions (R-01 through R-NN) in v4.0-REQUIREMENTS.md. Map table stakes (TS-01 through TS-06) to Phase 1-3 requirements, differentiators (DIFF-01 through DIFF-05) to Phase 4-5 requirements.
