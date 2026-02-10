# Quantico CRM Roadmap

## Milestones

- **v1.0 Platform Update System** - Phases 01-05 (shipped 2026-02-01) -> [archive](milestones/v1.0-ROADMAP.md)
- **v2.0 Security Hardening** - Phases 06-10 (shipped 2026-02-04) -> [archive](milestones/v2.0-ROADMAP.md)
- **v3.0 Deduplication System** - Phases 11-16 (shipped 2026-02-09) -> [archive](milestones/v3.0-ROADMAP.md)
- **v4.0 Salesforce Merge Integration** - Phases 17-19 (in progress)

---

## Phases

<details>
<summary>v1.0 Platform Update System (Phases 1-5) — SHIPPED 2026-02-01</summary>

- [x] Phase 1-5: Platform version tracking, changelog, admin UI, org migration, version-aware provisioning (9/9 plans)

</details>

<details>
<summary>v2.0 Security Hardening (Phases 6-10) — SHIPPED 2026-02-04</summary>

- [x] Phase 6-10: OWASP compliance, token security, CSRF, audit logging, CI scanning (22/22 plans)

</details>

<details>
<summary>v3.0 Deduplication System (Phases 11-16) — SHIPPED 2026-02-09</summary>

- [x] Phase 11: Detection Foundation (3/3 plans) — completed 2026-02-06
- [x] Phase 12: Real-Time Detection (4/4 plans) — completed 2026-02-07
- [x] Phase 13: Merge Engine (4/4 plans) — completed 2026-02-07
- [x] Phase 14: Import Integration (3/3 plans) — completed 2026-02-08
- [x] Phase 15: Background Scanning (3/3 plans) — completed 2026-02-08
- [x] Phase 16: Admin UI (5/5 plans) — completed 2026-02-08

</details>

---

### v4.0 Salesforce Merge Integration (In Progress)

**Milestone Goal:** Send merge instructions from Quantico to Salesforce so customers can use Quantico as a standalone dedup/merge tool that syncs results back to their Salesforce org.

#### Phase 17: Core Integration
**Goal**: Quantico can authenticate with Salesforce, generate merge instruction payloads, and deliver them to Salesforce staging object
**Depends on**: Phase 16 (v3.0 complete)
**Requirements**: SFI-01, SFI-02, SFI-03, SFI-04, SFI-05, SFI-06, SFI-07, SFI-08, SFI-09, SFI-10, SFI-11, SFI-12, SFI-13, SFI-14
**Success Criteria** (what must be TRUE):
  1. Admin can configure Salesforce Connected App (Client ID, Client Secret, Redirect URL) in Quantico admin panel
  2. Admin can authorize Quantico to access Salesforce via OAuth 2.0 authorization flow
  3. Quantico generates valid merge instruction JSON from dedup resolution results (winner_id, loser_id, field_values using 18-char Salesforce IDs and API field names)
  4. Quantico batches up to 200 merge instructions per API call with unique batch_id (QTC-YYYYMMDD-NNN) and instruction_ids (MI-NNNN)
  5. Quantico POSTs batched merge instructions to Salesforce staging object via REST API and handles API errors gracefully
  6. Quantico proactively refreshes OAuth access tokens before expiry (avoids mid-batch token expiration)
**Plans:** 5 plans

Plans:
- [ ] 17-01-PLAN.md -- Foundation: database schema, entity types, SFID prefixes, repository, encryption utility
- [ ] 17-02-PLAN.md -- OAuth 2.0: service with token management, handler with config/connect endpoints
- [ ] 17-03-PLAN.md -- Payload: merge instruction builder, batch assembler with unique IDs
- [ ] 17-04-PLAN.md -- Delivery: async batch delivery service, queue/trigger/status endpoints
- [ ] 17-05-PLAN.md -- Admin UI: integrations hub, Salesforce config page, connection flow, job table

#### Phase 18: Rate Limiting & Error Handling
**Goal**: Quantico respects Salesforce API limits and handles errors intelligently
**Depends on**: Phase 17
**Requirements**: SFI-15, SFI-16, SFI-17, SFI-18, SFI-19
**Success Criteria** (what must be TRUE):
  1. Quantico tracks API usage per org over 24-hour rolling windows and displays current usage percentage
  2. Quantico does not exceed 100,000 Salesforce API calls per 24-hour period per org (Enterprise Edition limit)
  3. Quantico pauses batch delivery automatically when org reaches 80% API capacity (80,000 calls) to prevent hitting hard limit
  4. Quantico implements exponential backoff for 429 Too Many Requests errors (5s, 10s, 20s, 40s delays with max 5 retries)
  5. Admin can manually trigger merge delivery to override rate limiting pauses (for testing or recovery scenarios)
**Plans**: TBD

Plans:
- [ ] 18-01: TBD

#### Phase 19: Audit Logging & Admin Configuration
**Goal**: Quantico logs all merge instruction delivery for compliance and provides admin UI for configuration and monitoring
**Depends on**: Phase 18
**Requirements**: SFI-20, SFI-21, SFI-22, SFI-23, SFI-24, SFI-25, SFI-26, SFI-27
**Success Criteria** (what must be TRUE):
  1. Quantico logs every merge instruction sent with batch_id, instruction_id, timestamp, winner_id, loser_id, delivery status, and Salesforce response details
  2. Audit logs are stored with 7-year retention (SOX compliance requirement) and are tamper-evident
  3. Admin can query audit logs by batch_id, date range, org, and result status (success/error/retry) via admin UI
  4. Admin page shows Salesforce connection status (connected/disconnected/token expired) and allows testing connection
  5. Admin can enable or disable Salesforce sync for an org via toggle control
  6. Admin can manually trigger immediate merge instruction delivery via admin UI button
**Plans**: TBD

Plans:
- [ ] 19-01: TBD

---

## Progress

**Execution Order:**
Phases execute in numeric order: 17 → 18 → 19

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 01-05 | v1.0 | 9/9 | Complete | 2026-02-01 |
| 06-10 | v2.0 | 22/22 | Complete | 2026-02-04 |
| 11-16 | v3.0 | 22/22 | Complete | 2026-02-09 |
| 17 | v4.0 | 0/5 | Planned | - |
| 18 | v4.0 | 0/TBD | Not started | - |
| 19 | v4.0 | 0/TBD | Not started | - |

**Totals:** 3 milestones shipped (53 plans), v4.0 in progress (3 phases, 5 plans planned for Phase 17)

---

*Last updated: 2026-02-09 — Phase 17 planned (5 plans in 4 waves)*
