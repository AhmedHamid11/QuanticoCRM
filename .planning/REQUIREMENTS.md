# Requirements: Quantico CRM v4.0 Salesforce Merge Integration

**Defined:** 2026-02-09
**Core Value:** Fast, secure multi-tenant CRM where customer data is protected and platform updates are transparent

---

## v4.0 Requirements

Focused scope: Build end-to-end Salesforce merge instruction delivery. Table stakes only.

### Merge Instruction Payload

- [ ] **SFI-01**: Quantico builds merge instruction JSON from dedup resolution (winner_id, loser_id, field_values)
- [ ] **SFI-02**: Merge instruction includes all mapped fields for the object (no partial field sets)
- [ ] **SFI-03**: Merge instructions use 18-character Salesforce Record IDs (not 15-char)
- [ ] **SFI-04**: Merge instructions use Salesforce field API names, not display labels

### Salesforce Authentication

- [ ] **SFI-05**: Admin can configure Salesforce Connected App (Client ID, Client Secret, Redirect URL)
- [ ] **SFI-06**: Quantico initiates OAuth 2.0 authorization flow (user grants scope)
- [ ] **SFI-07**: Quantico stores encrypted OAuth tokens in database (refresh token, access token, expiry)
- [ ] **SFI-08**: Quantico refreshes access tokens before expiry (proactive, not reactive)
- [ ] **SFI-09**: Auth failures surface clear error messages (invalid credentials, scope issues, etc.)

### Batch Delivery

- [ ] **SFI-10**: Quantico accepts merge instructions from dedup system (manual trigger or scheduled job)
- [ ] **SFI-11**: Quantico groups merge instructions into batches (up to 200 per batch)
- [ ] **SFI-12**: Quantico generates unique batch_id (format: QTC-YYYYMMDD-NNN) and instruction_ids (MI-NNNN)
- [ ] **SFI-13**: Quantico POSTs merge instructions to Salesforce staging object via REST API
- [ ] **SFI-14**: Quantico handles Salesforce API errors gracefully (invalid syntax, field not found, etc.)

### Rate Limiting

- [ ] **SFI-15**: Quantico tracks API usage per org (calls per 24-hour period)
- [ ] **SFI-16**: Quantico respects Salesforce API limits (100K calls/day for Enterprise)
- [ ] **SFI-17**: Quantico implements exponential backoff (5s, 10s, 20s, 40s) on 429 Too Many Requests
- [ ] **SFI-18**: Quantico pauses batch delivery at 80% capacity (does not attempt 100K+ calls)
- [ ] **SFI-19**: Quantico allows manual trigger to override rate limiting (for testing/recovery)

### Audit Logging

- [ ] **SFI-20**: Quantico logs every merge instruction sent (batch_id, instruction_id, timestamp, winner_id, loser_id)
- [ ] **SFI-21**: Quantico logs delivery status (success, error, retry) with Salesforce response details
- [ ] **SFI-22**: Quantico stores audit logs with 7-year retention (compliance requirement for SOX)
- [ ] **SFI-23**: Admin can query audit logs by batch_id, date range, org, result status

### Admin Configuration UI

- [ ] **SFI-24**: Admin page allows connecting Salesforce org (OAuth flow, test connection)
- [ ] **SFI-25**: Admin page shows connection status (connected, disconnected, token expired)
- [ ] **SFI-26**: Admin page allows enabling/disabling sync for an org
- [ ] **SFI-27**: Admin page displays manual trigger for immediate merge delivery

---

## v4.1+ Requirements

Deferred to future release. Tracked but not in current roadmap.

### Admin Visibility & Monitoring

- **SFI-V2-01**: Delivery status dashboard (pending queue, success rate, failures)
- **SFI-V2-02**: Real-time status updates (polling or SSE)
- **SFI-V2-03**: Analytics dashboard (API usage trends, merge volume, success rates)

### Data Validation & Reliability

- **SFI-V2-04**: Pre-flight validation of merge instructions (ID format, field names, payload size)
- **SFI-V2-05**: Advanced error handling (distinguish transient vs permanent errors)
- **SFI-V2-06**: Manual retry UI for failed batches
- **SFI-V2-07**: Dead letter queue for permanently failed instructions

### Onboarding & Setup

- **SFI-V2-08**: Field mapping presets (common Quantico→Salesforce field mappings)
- **SFI-V2-09**: Setup wizard to guide admins through Salesforce Connected App configuration

### Extensibility

- **SFI-V2-10**: Extensible integration framework (support HubSpot, Pipedrive, webhooks)
- **SFI-V2-11**: Webhook delivery for external systems

---

## Out of Scope

Explicitly excluded. Rationale prevents scope creep.

| Requirement | Reason |
|---|---|
| Bidirectional sync (Salesforce → Quantico) | Complex conflict resolution. Quantico is system of record for merges. |
| Salesforce package deployment | Customers build/deploy own Apex trigger. Too much SF-side complexity. |
| Multi-CRM support in v4.0 | Salesforce only. Framework deferred to v4.1. |
| Webhook delivery | Extensibility deferred to v4.1 per project spec. |
| Real-time UI updates (SSE/WebSocket) | Polling sufficient for MVP. Real-time deferred to v4.1. |
| Field mapping UI | Manual field mapping in code/config. UI presets deferred to v4.1. |

---

## Traceability

Maps v4.0 requirements to roadmap phases. Updated during roadmap creation.

| Requirement | Phase | Status |
|---|---|---|
| SFI-01 to SFI-04 | Phase 17 | Pending |
| SFI-05 to SFI-09 | Phase 17 | Pending |
| SFI-10 to SFI-14 | Phase 17 | Pending |
| SFI-15 to SFI-19 | Phase 18 | Pending |
| SFI-20 to SFI-23 | Phase 19 | Pending |
| SFI-24 to SFI-27 | Phase 19 | Pending |

**Coverage:**
- v4.0 requirements: 27 total
- Mapped to phases: — (pending roadmap)
- Unmapped: — (pending roadmap)

---

*Requirements defined: 2026-02-09*
*Last updated: 2026-02-09 after initial definition*
