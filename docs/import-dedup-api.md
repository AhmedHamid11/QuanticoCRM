# Import Dedup Decisions API

## TL;DR

After importing a CSV through Quantico, the import ID is returned in the response. Salesforce (or any external system) can then call the Ingest API to retrieve which records were kept vs discarded during dedup.

```bash
# 1. List all imports for your org
curl https://your-api.com/api/v1/ingest/imports \
  -H "X-API-Key: qik_your_key_here"

# 2. Get dedup decisions for a specific import
curl https://your-api.com/api/v1/ingest/imports/{importId}/dedup-results \
  -H "X-API-Key: qik_your_key_here"
```

The response tells you exactly which Salesforce external IDs were kept and which were discarded, so Salesforce can merge or delete the losing records.

---

## How It Works

### During CSV Import (Automatic)

When a user imports a CSV through the Import Wizard:

1. Quantico detects duplicates (within-file and against existing DB records)
2. The user resolves duplicates in the wizard UI (pick which row to keep)
3. On import, Quantico persists an **import job** and all **dedup decisions** to the database
4. The import response includes an `importId` field

No extra steps needed from the user -- decisions are saved automatically.

### After Import (API Retrieval)

External systems (like Salesforce) authenticate with an **Ingest API Key** (`X-API-Key` header) and call the endpoints below to retrieve decisions.

---

## Authentication

All endpoints use **X-API-Key** header authentication (not JWT).

Create an Ingest API Key in: **Admin > Settings > Ingest API Keys**

```
X-API-Key: qik_your_key_here
```

The key is scoped to your organization. You only see imports for your own org.

---

## Endpoints

### List Imports

```
GET /api/v1/ingest/imports
```

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `pageSize` | int | 20 | Results per page (max 100) |
| `entityType` | string | - | Filter by entity (e.g. `Contact`, `Account`) |
| `since` | string | - | Only imports after this datetime (ISO 8601) |

**Response:**

```json
{
  "imports": [
    {
      "id": "0IpKHGX0B3A0001ABC",
      "orgId": "00D...",
      "entityType": "Contact",
      "externalIdField": "salesforceId",
      "totalRows": 100,
      "createdCount": 85,
      "updatedCount": 10,
      "skippedCount": 3,
      "mergedCount": 2,
      "failedCount": 0,
      "createdAt": "2026-02-15 15:05:33"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 20
}
```

---

### Get Dedup Results

```
GET /api/v1/ingest/imports/:id/dedup-results
```

**Response:**

```json
{
  "import_id": "0IpKHGX0B3A0001ABC",
  "entity_type": "Contact",
  "total_rows": 100,
  "created": 85,
  "updated": 10,
  "skipped": 3,
  "merged": 2,
  "failed": 0,
  "created_at": "2026-02-15 15:05:33",
  "decisions": [
    {
      "id": "0DdKHGX0B3A0001XYZ",
      "decisionType": "within_file",
      "action": "skip",
      "keptExternalId": "003Vp00000ODIxYIAX",
      "discardedExternalId": "003Vp00000jMwcvIAC",
      "matchField": "emailAddress",
      "matchValue": "eileen.baker@usenourish.com",
      "createdAt": "2026-02-15 15:05:33"
    },
    {
      "id": "0DdKHGX0B3A0002XYZ",
      "decisionType": "db_match",
      "action": "update",
      "keptExternalId": "003Vp00000ABCxYIAX",
      "matchField": "emailAddress",
      "matchValue": "john.doe@example.com",
      "matchedRecordId": "REC001",
      "createdAt": "2026-02-15 15:05:33"
    }
  ]
}
```

---

## Decision Fields

| Field | Description |
|-------|-------------|
| `decisionType` | `within_file` = two CSV rows matched each other; `db_match` = CSV row matched an existing DB record |
| `action` | What happened: `skip` (row discarded), `update` (existing record updated), `import` (new record created), `merge` (sent to merge queue) |
| `keptExternalId` | Salesforce ID of the record that was **kept** |
| `discardedExternalId` | Salesforce ID of the record that was **discarded** (within-file only) |
| `matchField` | The field used to detect the duplicate (e.g. `emailAddress`) |
| `matchValue` | The value that matched (e.g. `eileen.baker@usenourish.com`) |
| `matchedRecordId` | Quantico record ID of the existing record that was matched (db_match only) |

---

## Salesforce Integration Example

After retrieving dedup decisions, Salesforce can use the external IDs to clean up:

```apex
// Apex: Merge discarded contacts into kept contacts
List<Contact> toMerge = [
    SELECT Id FROM Contact
    WHERE Id = :discardedExternalId
];
List<Contact> master = [
    SELECT Id FROM Contact
    WHERE Id = :keptExternalId
];
if (!toMerge.isEmpty() && !master.isEmpty()) {
    Database.merge(master[0], toMerge);
}
```

---

## Error Responses

| Status | Body | Meaning |
|--------|------|---------|
| 401 | `{"error": "X-API-Key header required"}` | Missing or invalid API key |
| 403 | `{"error": "Organization not found or inactive"}` | Key's org is deactivated |
| 404 | `{"error": "Import job not found"}` | No import with that ID (or wrong org) |
| 500 | `{"error": "..."}` | Server error |
