# QuanticoCRM — Salesforce Merge Instruction Integration

**Technical Specification for Developer Handoff**

| | |
|---|---|
| **Version** | 1.0 |
| **Date** | February 9, 2026 |
| **Author** | Ahmed (CTO/CISO, Nourish) |
| **Status** | Ready for Development |

---

## 1. Overview

QuanticoCRM is an external CRM system with a built-in deduplication engine. When duplicate records are identified and resolved (with human review), Quantico must send merge instructions to Salesforce so that Salesforce can execute the merge on its end. This document defines the payload schema, integration pattern, and Salesforce-side expectations for that handoff.

Quantico is the system of record for dedup decisions and audit logging. Salesforce is the execution layer — it receives trusted instructions and acts on them without second-guessing.

---

## 2. Architecture Summary

### 2.1 Data Flow

1. Data flows INTO Quantico from Salesforce via API or CSV (flexible per situation)
2. Quantico runs deduplication engine on ingested records
3. Human reviewers validate dedup results within Quantico
4. Quantico builds a batched JSON payload with merge instructions
5. Quantico sends payload to Salesforce via its existing API send function
6. Salesforce writes merge instructions to a custom staging object
7. Salesforce Flow processes staging records, executes merges, and reparents child records

### 2.2 Key Design Decisions

| Decision | Choice |
|---|---|
| Objects in Scope | All objects — standard and custom |
| Merge Execution | Custom staging object + Flow (no native merge API) |
| Payload Delivery | Quantico's existing API send function, batched |
| Field Mapping Strategy | Every field explicitly mapped with actual values to write |
| Field Map Format | JSON string (field name → value) |
| Child Record Reparenting | Salesforce auto-discovers and reparents all child relationships |
| Authentication | OAuth 2.0 via Salesforce Connected App (JWT or Web Server flow) |
| Push Cadence | Real-time for hot matches + batch for bulk runs |
| Feedback Loop | Fire and forget — no callback from Salesforce |
| Audit Logging | Quantico only — Salesforce does not maintain merge logs |
| Volume | Hundreds of thousands of records per dedup run |
| Multi-Org | Single org now, architecture should be org-agnostic |
| Schema Discovery | Admin maps objects/fields during Quantico setup |

---

## 3. JSON Payload Schema

This is the exact payload Quantico sends to Salesforce. Each API call contains a batch of merge instructions. The payload is fully self-contained — Salesforce does not need to look up any additional data to execute the merge.

### 3.1 Full Schema Example

```json
{
  "batch_id": "QTC-20260209-001",
  "timestamp": "2026-02-09T14:30:00Z",
  "org_id": "00D5f000000XXXX",
  "merge_instructions": [
    {
      "instruction_id": "MI-0001",
      "object_api_name": "Contact",
      "winner_id": "003XX000004AAAA",
      "loser_id": "003XX000004BBBB",
      "field_values": {
        "FirstName": "Ahmed",
        "LastName": "Smith",
        "Email": "ahmed@nourish.com",
        "Phone": "727-555-1234",
        "MailingCity": "Clearwater",
        "MailingState": "FL",
        "Custom_Field__c": "some value"
      }
    },
    {
      "instruction_id": "MI-0002",
      "object_api_name": "Account",
      "winner_id": "001XX000003CCCC",
      "loser_id": "001XX000003DDDD",
      "field_values": {
        "Name": "Acme Corp",
        "Industry": "Healthcare",
        "BillingCity": "Tampa",
        "Website": "https://acme.com"
      }
    },
    {
      "instruction_id": "MI-0003",
      "object_api_name": "Custom_Patient__c",
      "winner_id": "a01XX000008EEEE",
      "loser_id": "a01XX000008FFFF",
      "field_values": {
        "Patient_Name__c": "Jane Doe",
        "DOB__c": "1990-05-15",
        "MRN__c": "MRN-12345"
      }
    }
  ]
}
```

### 3.2 Field Reference

#### Top-Level Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `batch_id` | String | Yes | Unique ID for this batch run. Format: `QTC-YYYYMMDD-NNN`. Used for tracing in Quantico audit logs. |
| `timestamp` | String | Yes | ISO 8601 timestamp of when the batch was generated. |
| `org_id` | String | Yes | Salesforce Organization ID (18-char). Future-proofs for multi-org support. |
| `merge_instructions` | Array | Yes | Array of merge instruction objects (see below). |

#### Merge Instruction Object Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `instruction_id` | String | Yes | Unique ID for this merge pair. Format: `MI-NNNN`. Quantico logs against this. |
| `object_api_name` | String | Yes | Salesforce API name of the object (e.g., `Contact`, `Account`, `Custom_Patient__c`). |
| `winner_id` | String | Yes | 18-character Salesforce Record ID of the record to KEEP. |
| `loser_id` | String | Yes | 18-character Salesforce Record ID of the record to DELETE after merge. |
| `field_values` | Object | Yes | Key-value pairs: field API name → actual value to write to the winner. Every field is explicitly listed, no defaults. |

---

## 4. What Quantico Must Build

This section defines the exact scope of work for the Quantico side of the integration. Salesforce-side implementation (staging object, Flow, Apex JSON parser) is handled separately.

### 4.1 Merge Instruction Builder

After dedup resolution (human-reviewed), Quantico must construct the JSON payload. This component takes the dedup result (winner, loser, resolved field values) and transforms it into the schema defined in Section 3.

- **Input:** Dedup engine output — two matched records, resolved field values, winner/loser designation
- **Output:** A single merge instruction object conforming to Section 3.2
- Must use Salesforce field API names (not display labels) as keys in `field_values`
- Must use 18-character Salesforce Record IDs for `winner_id` and `loser_id`
- Must include ALL mapped fields in `field_values`, not just changed ones

### 4.2 Batch Assembler

Groups individual merge instructions into batched payloads for efficient API delivery.

- Generates a unique `batch_id` per run (format: `QTC-YYYYMMDD-NNN`)
- Attaches `org_id` from admin configuration
- Determines batch size based on Salesforce API governor limits (recommended: 200 records per batch to align with SF Composite API limits)
- For real-time pushes: single instruction per payload is acceptable
- For batch runs: group up to 200 instructions per API call

### 4.3 API Send Integration

Uses Quantico's existing API send function to deliver payloads to Salesforce.

- **Authentication:** OAuth 2.0 via Salesforce Connected App (JWT or Web Server flow)
- **Endpoint:** Salesforce REST API — sObject create on the custom staging object (`Merge_Instruction__c` or equivalent)
- For batched payloads, use Salesforce Composite API or sObject Collection endpoint to create multiple staging records in one call
- Fire and forget — Quantico does not wait for merge execution confirmation
- Quantico logs the `batch_id` and `instruction_id`s to its own audit system

### 4.4 Quantico Internal Database Update

After dedup resolution and before sending merge instructions to Salesforce, Quantico must update its own database to reflect the merge.

- Mark the loser record as merged/inactive in Quantico's database
- Update the winner record with the resolved field values
- Store the Salesforce Record IDs for both records to maintain cross-system traceability

---

## 5. Salesforce-Side Expectations (For Reference)

This section is provided for context only. The Salesforce implementation is handled separately. However, the developer should understand what Salesforce expects so the Quantico payload aligns correctly.

### 5.1 Custom Staging Object

Salesforce will have a custom object (e.g., `Merge_Instruction__c`) with the following fields that map to the payload:

| SF Field | Type | Maps To |
|---|---|---|
| `Object_API_Name__c` | Text | `object_api_name` |
| `Winner_Id__c` | Text(18) | `winner_id` |
| `Loser_Id__c` | Text(18) | `loser_id` |
| `Field_Map__c` | Long Text | `JSON.stringify(field_values)` |
| `Status__c` | Picklist | Set to "Pending" on creation |
| `Batch_Id__c` | Text | `batch_id` |
| `Instruction_Id__c` | Text | `instruction_id` |

### 5.2 Salesforce Processing

A Record-Triggered Flow fires when a `Merge_Instruction__c` record is created with Status = Pending. The Flow updates the winner record with the field values from `Field_Map__c` (parsed via Invocable Apex), reparents all child records from loser to winner by auto-discovering child relationships, deletes the loser record, and updates `Status__c` to Complete or Failed.

---

## 6. Important Constraints and Notes

### 6.1 Salesforce API Governor Limits

- sObject Collection API: max 200 records per request
- Daily API request limit varies by org edition (typically 100,000+ for Enterprise)
- For hundreds of thousands of records, Quantico must throttle API calls and respect rate limits
- Recommended: implement exponential backoff on 429 (rate limit) responses

### 6.2 Data Integrity

- Always use 18-character Salesforce Record IDs (not 15-character)
- Use Salesforce field API names, not display labels
- `field_values` must contain every mapped field for the object — Salesforce will write exactly what it receives
- Null values should be explicitly passed as `null` if a field should be cleared

### 6.3 JSON Field Map Note

Salesforce Flow cannot natively parse JSON. The Salesforce side will use an Invocable Apex class to parse the `Field_Map__c` JSON. This does not affect Quantico's implementation — just send valid JSON. Ensure the JSON does not exceed Salesforce's Long Text Area field limit of 131,072 characters.

---

## 7. Developer Checklist

| # | Task | Status |
|---|---|---|
| 1 | Build merge instruction object from dedup engine output | To Do |
| 2 | Implement batch assembler with configurable batch size | To Do |
| 3 | Generate unique `batch_id` and `instruction_id` values | To Do |
| 4 | Configure OAuth 2.0 Connected App authentication to Salesforce | To Do |
| 5 | Integrate with existing API send function to POST to SF staging object | To Do |
| 6 | Implement rate limiting / exponential backoff for SF API calls | To Do |
| 7 | Update Quantico internal DB (winner updated, loser marked inactive) | To Do |
| 8 | Audit logging: log `batch_id`, `instruction_id`s, timestamps, outcomes | To Do |
| 9 | Support real-time single-instruction push for high-confidence matches | To Do |
| 10 | Support batch push for scheduled/bulk dedup runs | To Do |
| 11 | Validate `field_values` JSON does not exceed 131,072 characters | To Do |
| 12 | End-to-end test with sandbox Salesforce org | To Do |

---

*— End of Specification —*
