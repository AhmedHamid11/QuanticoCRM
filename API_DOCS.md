# Quantico CRM API Documentation

**Base URL:** `http://localhost:8080/api/v1`

**Authentication:** All endpoints (except public auth) require a Bearer token:
```bash
-H "Authorization: Bearer <access_token>"
```

---

## Table of Contents

1. [Authentication](#authentication)
2. [Generic Entity CRUD](#generic-entity-crud)
3. [Standard Entities](#standard-entities)
4. [Bulk Operations](#bulk-operations)
5. [CSV Import](#csv-import)
6. [Lookups](#lookups)
7. [Related Lists](#related-lists)
8. [Admin - Entities & Fields](#admin---entities--fields)
9. [Admin - Layouts](#admin---layouts)
10. [Admin - Navigation](#admin---navigation)
11. [Admin - Tripwires (Webhooks)](#admin---tripwires-webhooks)
12. [Admin - Validation Rules](#admin---validation-rules)
13. [Users](#users)
14. [API Tokens](#api-tokens)
15. [Screen Flows](#screen-flows)

---

## Authentication

### Register (Create Org + User)

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "SecurePass123!",
    "orgName": "Acme Corp",
    "firstName": "John",
    "lastName": "Doe"
  }'
```

**Response (201):**
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 900,
  "user": {
    "id": "usr_ABC123",
    "email": "admin@acme.com",
    "firstName": "John",
    "lastName": "Doe"
  },
  "org": {
    "id": "org_XYZ789",
    "name": "Acme Corp",
    "slug": "acme-corp"
  }
}
```

---

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "SecurePass123!"
  }'
```

**Response (200):**
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 900,
  "user": {
    "id": "usr_ABC123",
    "email": "admin@acme.com",
    "firstName": "John",
    "lastName": "Doe"
  },
  "org": {
    "id": "org_XYZ789",
    "name": "Acme Corp",
    "slug": "acme-corp"
  }
}
```

---

### Refresh Token

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }'
```

**Response (200):**
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 900
}
```

---

### Get Current User

```bash
curl -X GET http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "usr_ABC123",
  "email": "admin@acme.com",
  "firstName": "John",
  "lastName": "Doe",
  "role": "owner",
  "orgId": "org_XYZ789",
  "orgName": "Acme Corp"
}
```

---

### Get User Organizations

```bash
curl -X GET http://localhost:8080/api/v1/auth/orgs \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "organizations": [
    {
      "id": "org_XYZ789",
      "name": "Acme Corp",
      "slug": "acme-corp",
      "role": "owner"
    },
    {
      "id": "org_DEF456",
      "name": "Other Company",
      "slug": "other-company",
      "role": "user"
    }
  ]
}
```

---

### Switch Organization

```bash
curl -X POST http://localhost:8080/api/v1/auth/switch-org \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "orgId": "org_DEF456"
  }'
```

**Response (200):**
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 900,
  "org": {
    "id": "org_DEF456",
    "name": "Other Company",
    "slug": "other-company"
  }
}
```

---

### Invite User (Admin Only)

```bash
curl -X POST http://localhost:8080/api/v1/auth/invite \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@acme.com",
    "role": "user"
  }'
```

**Response (201):**
```json
{
  "id": "inv_ABC123",
  "email": "newuser@acme.com",
  "role": "user",
  "expiresAt": "2024-01-22T10:30:00Z",
  "inviteUrl": "http://localhost:5173/accept-invite?token=abc123..."
}
```

---

### Accept Invitation

```bash
curl -X POST http://localhost:8080/api/v1/auth/accept-invite \
  -H "Content-Type: application/json" \
  -d '{
    "token": "abc123...",
    "password": "NewUserPass123!",
    "firstName": "Jane",
    "lastName": "Smith"
  }'
```

**Response (200):**
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 900,
  "user": {
    "id": "usr_NEW456",
    "email": "newuser@acme.com",
    "firstName": "Jane",
    "lastName": "Smith"
  },
  "org": {
    "id": "org_XYZ789",
    "name": "Acme Corp",
    "slug": "acme-corp"
  }
}
```

---

### List Pending Invitations (Admin Only)

```bash
curl -X GET http://localhost:8080/api/v1/auth/invitations \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "invitations": [
    {
      "id": "inv_ABC123",
      "email": "pending@acme.com",
      "role": "user",
      "createdAt": "2024-01-15T10:30:00Z",
      "expiresAt": "2024-01-22T10:30:00Z"
    }
  ]
}
```

---

### Cancel Invitation (Admin Only)

```bash
curl -X DELETE http://localhost:8080/api/v1/auth/invitations/inv_ABC123 \
  -H "Authorization: Bearer <access_token>"
```

**Response (204):** No content

---

### Change Password

```bash
curl -X POST http://localhost:8080/api/v1/auth/change-password \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "currentPassword": "OldPass123!",
    "newPassword": "NewPass456!"
  }'
```

**Response (200):**
```json
{
  "message": "Password changed successfully"
}
```

---

### Logout

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }'
```

**Response (200):**
```json
{
  "message": "Logged out successfully"
}
```

---

## Generic Entity CRUD

All custom entities use the same endpoint pattern. Replace `{entity}` with entity name (e.g., `Contact`, `Job`, `Candidate`).

### List Records

```bash
curl -X GET "http://localhost:8080/api/v1/entities/Contact/records?page=1&pageSize=20&sortBy=createdAt&sortDir=desc" \
  -H "Authorization: Bearer <access_token>"
```

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| page | int | 1 | Page number |
| pageSize | int | 20 | Records per page (max 100) |
| sortBy | string | created_at | Field to sort by |
| sortDir | string | desc | asc or desc |
| search | string | | Search by name field |

**Response (200):**
```json
{
  "data": [
    {
      "id": "rec_ABC123",
      "name": "John Doe",
      "email": "john@example.com",
      "phone": "555-1234",
      "accountId": "rec_ACC001",
      "accountName": "Acme Corp",
      "createdAt": "2024-01-15T10:30:00Z",
      "modifiedAt": "2024-01-15T10:30:00Z",
      "createdById": "usr_ABC123",
      "modifiedById": "usr_ABC123"
    },
    {
      "id": "rec_DEF456",
      "name": "Jane Smith",
      "email": "jane@example.com",
      "phone": "555-5678",
      "accountId": null,
      "accountName": "",
      "createdAt": "2024-01-14T09:00:00Z",
      "modifiedAt": "2024-01-14T09:00:00Z",
      "createdById": "usr_ABC123",
      "modifiedById": "usr_ABC123"
    }
  ],
  "total": 45,
  "totalPages": 3,
  "page": 1,
  "pageSize": 20
}
```

---

### Get Single Record

```bash
curl -X GET http://localhost:8080/api/v1/entities/Contact/records/rec_ABC123 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "rec_ABC123",
  "name": "John Doe",
  "email": "john@example.com",
  "phone": "555-1234",
  "accountId": "rec_ACC001",
  "accountName": "Acme Corp",
  "customField": "Custom Value",
  "createdAt": "2024-01-15T10:30:00Z",
  "modifiedAt": "2024-01-15T10:30:00Z",
  "createdById": "usr_ABC123",
  "modifiedById": "usr_ABC123"
}
```

**Error Response (404):**
```json
{
  "error": "Record not found"
}
```

---

### Create Record

```bash
curl -X POST http://localhost:8080/api/v1/entities/Contact/records \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "New Contact",
    "email": "new@example.com",
    "phone": "555-9999",
    "accountId": "rec_ACC001",
    "accountName": "Acme Corp"
  }'
```

**Response (201):**
```json
{
  "id": "rec_NEW789",
  "name": "New Contact",
  "email": "new@example.com",
  "phone": "555-9999",
  "accountId": "rec_ACC001",
  "accountName": "Acme Corp",
  "createdAt": "2024-01-16T14:00:00Z",
  "modifiedAt": "2024-01-16T14:00:00Z"
}
```

**Validation Error (422):**
```json
{
  "error": "Validation failed",
  "fieldErrors": [
    {
      "field": "email",
      "message": "Email is required"
    }
  ]
}
```

---

### Update Record

```bash
curl -X PUT http://localhost:8080/api/v1/entities/Contact/records/rec_ABC123 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Doe Updated",
    "phone": "555-0000"
  }'
```

**Response (200):**
```json
{
  "id": "rec_ABC123",
  "name": "John Doe Updated",
  "email": "john@example.com",
  "phone": "555-0000",
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

### Delete Record

```bash
curl -X DELETE http://localhost:8080/api/v1/entities/Contact/records/rec_ABC123 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "deleted": true
}
```

---

### Upsert Record (Create or Update)

Creates a new record if no match exists, otherwise updates the existing record.

```bash
curl -X POST "http://localhost:8080/api/v1/entities/Contact/records/upsert?matchField=email" \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "name": "John Doe",
    "phone": "555-1234"
  }'
```

**Query Parameters:**
| Param | Required | Description |
|-------|----------|-------------|
| matchField | Yes | Field to match on (e.g., "email", "externalId") |

**Response - Created (201):**
```json
{
  "id": "rec_NEW789",
  "email": "john@example.com",
  "name": "John Doe",
  "phone": "555-1234",
  "createdAt": "2024-01-16T14:00:00Z",
  "modifiedAt": "2024-01-16T14:00:00Z",
  "_upsertAction": "created"
}
```

**Response - Updated (200):**
```json
{
  "id": "rec_ABC123",
  "email": "john@example.com",
  "name": "John Doe",
  "phone": "555-1234",
  "modifiedAt": "2024-01-16T15:00:00Z",
  "_upsertAction": "updated"
}
```

**Error - Missing matchField (400):**
```json
{
  "error": "matchField query parameter is required (e.g., ?matchField=email)"
}
```

---

### Get Entity Fields

```bash
curl -X GET http://localhost:8080/api/v1/entities/Contact/fields \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
[
  {
    "name": "name",
    "label": "Name",
    "type": "varchar",
    "isRequired": true,
    "isReadOnly": false,
    "sortOrder": 0
  },
  {
    "name": "email",
    "label": "Email",
    "type": "email",
    "isRequired": false,
    "isReadOnly": false,
    "sortOrder": 1
  },
  {
    "name": "account",
    "label": "Account",
    "type": "link",
    "linkEntity": "Account",
    "isRequired": false,
    "sortOrder": 2
  }
]
```

---

### Get Entity Definition

```bash
curl -X GET http://localhost:8080/api/v1/entities/Contact/def \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "name": "Contact",
  "label": "Contact",
  "labelPlural": "Contacts",
  "icon": "user",
  "description": "People you do business with",
  "isCustom": false,
  "createdAt": "2024-01-01T00:00:00Z"
}
```

---

## Standard Entities

### Contacts

#### List Contacts
```bash
curl -X GET "http://localhost:8080/api/v1/contacts?page=1&pageSize=20&search=john" \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "data": [
    {
      "id": "con_ABC123",
      "firstName": "John",
      "lastName": "Doe",
      "emailAddress": "john@example.com",
      "phoneNumber": "555-1234",
      "accountId": "acc_XYZ789",
      "accountName": "Acme Corp",
      "deleted": false,
      "createdAt": "2024-01-15T10:30:00Z",
      "modifiedAt": "2024-01-15T10:30:00Z"
    }
  ],
  "total": 1,
  "totalPages": 1,
  "page": 1,
  "pageSize": 20
}
```

#### Create Contact
```bash
curl -X POST http://localhost:8080/api/v1/contacts \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "firstName": "Jane",
    "lastName": "Smith",
    "emailAddress": "jane@example.com",
    "phoneNumber": "555-5678",
    "accountId": "acc_XYZ789"
  }'
```

**Response (201):**
```json
{
  "id": "con_NEW456",
  "firstName": "Jane",
  "lastName": "Smith",
  "emailAddress": "jane@example.com",
  "phoneNumber": "555-5678",
  "accountId": "acc_XYZ789",
  "accountName": "Acme Corp",
  "createdAt": "2024-01-16T14:00:00Z",
  "modifiedAt": "2024-01-16T14:00:00Z"
}
```

---

### Accounts

#### List Accounts
```bash
curl -X GET "http://localhost:8080/api/v1/accounts?page=1&pageSize=20" \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "data": [
    {
      "id": "acc_XYZ789",
      "name": "Acme Corp",
      "website": "https://acme.com",
      "emailAddress": "contact@acme.com",
      "phoneNumber": "555-0000",
      "billingAddressStreet": "123 Main St",
      "billingAddressCity": "San Francisco",
      "billingAddressState": "CA",
      "billingAddressPostalCode": "94105",
      "billingAddressCountry": "USA",
      "deleted": false,
      "createdAt": "2024-01-10T09:00:00Z",
      "modifiedAt": "2024-01-10T09:00:00Z"
    }
  ],
  "total": 1,
  "totalPages": 1,
  "page": 1,
  "pageSize": 20
}
```

#### Create Account
```bash
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "New Company",
    "website": "https://newco.com",
    "emailAddress": "info@newco.com",
    "phoneNumber": "555-1111"
  }'
```

**Response (201):**
```json
{
  "id": "acc_NEW123",
  "name": "New Company",
  "website": "https://newco.com",
  "emailAddress": "info@newco.com",
  "phoneNumber": "555-1111",
  "createdAt": "2024-01-16T14:00:00Z",
  "modifiedAt": "2024-01-16T14:00:00Z"
}
```

---

### Tasks

#### List Tasks
```bash
curl -X GET "http://localhost:8080/api/v1/tasks?status=Open&type=Call" \
  -H "Authorization: Bearer <access_token>"
```

**Query Parameters:**
| Param | Description |
|-------|-------------|
| status | Open, In Progress, Completed, Deferred, Cancelled |
| type | Call, Email, Meeting, Todo |
| parentType | Parent entity type (e.g., Contact, Account) |
| parentId | Parent record ID |
| dueBefore | ISO date string |
| dueAfter | ISO date string |

**Response (200):**
```json
{
  "data": [
    {
      "id": "tsk_ABC123",
      "subject": "Follow up call",
      "description": "Discuss contract renewal",
      "status": "Open",
      "type": "Call",
      "priority": "Normal",
      "dueDate": "2024-01-20T15:00:00Z",
      "parentType": "Contact",
      "parentId": "con_ABC123",
      "parentName": "John Doe",
      "assignedUserId": "usr_ABC123",
      "assignedUserName": "Admin User",
      "createdAt": "2024-01-15T10:30:00Z",
      "modifiedAt": "2024-01-15T10:30:00Z"
    }
  ],
  "total": 1,
  "totalPages": 1,
  "page": 1,
  "pageSize": 20
}
```

#### Create Task
```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "subject": "Send proposal",
    "description": "Prepare and send Q1 proposal",
    "status": "Open",
    "type": "Todo",
    "priority": "High",
    "dueDate": "2024-01-25T17:00:00Z",
    "parentType": "Account",
    "parentId": "acc_XYZ789",
    "assignedUserId": "usr_ABC123"
  }'
```

**Response (201):**
```json
{
  "id": "tsk_NEW456",
  "subject": "Send proposal",
  "description": "Prepare and send Q1 proposal",
  "status": "Open",
  "type": "Todo",
  "priority": "High",
  "dueDate": "2024-01-25T17:00:00Z",
  "parentType": "Account",
  "parentId": "acc_XYZ789",
  "parentName": "Acme Corp",
  "assignedUserId": "usr_ABC123",
  "assignedUserName": "Admin User",
  "createdAt": "2024-01-16T14:00:00Z",
  "modifiedAt": "2024-01-16T14:00:00Z"
}
```

---

## Bulk Operations

### Bulk Create

```bash
curl -X POST http://localhost:8080/api/v1/entities/Contact/bulk \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "records": [
      {"name": "Contact 1", "email": "c1@example.com"},
      {"name": "Contact 2", "email": "c2@example.com"},
      {"name": "Contact 3", "email": "c3@example.com"}
    ],
    "options": {
      "skipErrors": false,
      "returnErrors": true,
      "fireTripwires": true,
      "validateOnly": false,
      "batchSize": 1000
    }
  }'
```

**Options:**
| Option | Type | Default | Description |
|--------|------|---------|-------------|
| skipErrors | bool | false | Continue on individual failures |
| returnErrors | bool | true | Include error details in response |
| fireTripwires | bool | true | Trigger webhooks for each record |
| validateOnly | bool | false | Validate without saving |
| batchSize | int | 1000 | Records per transaction batch |

**Response (201):**
```json
{
  "created": 3,
  "failed": 0,
  "ids": ["rec_001", "rec_002", "rec_003"],
  "errors": []
}
```

**Response with errors (skipErrors=true):**
```json
{
  "created": 2,
  "failed": 1,
  "ids": ["rec_001", "rec_003"],
  "errors": [
    {
      "index": 1,
      "error": "Email is required",
      "fieldErrors": [{"field": "email", "message": "Email is required"}]
    }
  ]
}
```

---

### Bulk Update

```bash
curl -X PATCH http://localhost:8080/api/v1/entities/Contact/bulk \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "records": [
      {"id": "rec_001", "status": "Active"},
      {"id": "rec_002", "status": "Active"},
      {"id": "rec_003", "status": "Inactive"}
    ],
    "options": {
      "skipErrors": false
    }
  }'
```

**Response (200):**
```json
{
  "updated": 3,
  "failed": 0,
  "ids": ["rec_001", "rec_002", "rec_003"],
  "errors": []
}
```

---

## CSV Import

### Preview CSV

Preview before importing to verify column mappings.

```bash
curl -X POST http://localhost:8080/api/v1/entities/Contact/import/csv/preview \
  -H "Authorization: Bearer <access_token>" \
  -F "file=@contacts.csv"
```

**Response (200):**
```json
{
  "headers": ["Name", "Email", "Phone", "Company"],
  "mappedHeaders": ["name", "email", "phone", ""],
  "sampleRows": [
    {"Name": "John Doe", "Email": "john@example.com", "Phone": "555-1234", "Company": "Acme"},
    {"Name": "Jane Smith", "Email": "jane@example.com", "Phone": "555-5678", "Company": "Beta Inc"}
  ],
  "totalRows": 150,
  "unmappedColumns": ["Company"],
  "fields": [
    {"csvHeader": "Name", "fieldName": "name", "fieldLabel": "Name", "fieldType": "varchar", "mapped": true},
    {"csvHeader": "Email", "fieldName": "email", "fieldLabel": "Email", "fieldType": "email", "mapped": true},
    {"csvHeader": "Phone", "fieldName": "phone", "fieldLabel": "Phone", "fieldType": "phone", "mapped": true},
    {"csvHeader": "Company", "fieldName": "", "fieldLabel": "", "fieldType": "", "mapped": false}
  ]
}
```

---

### Import CSV

```bash
curl -X POST http://localhost:8080/api/v1/entities/Contact/import/csv \
  -H "Authorization: Bearer <access_token>" \
  -F "file=@contacts.csv" \
  -F 'options={"skipErrors": true, "fireTripwires": false}'
```

**Options (JSON in form field):**
```json
{
  "columnMapping": {"Company": "accountName"},
  "skipErrors": true,
  "fireTripwires": false,
  "validateOnly": false
}
```

**Response (201):**
```json
{
  "created": 148,
  "failed": 2,
  "totalRows": 150,
  "headers": ["Name", "Email", "Phone", "Company"],
  "mappedHeaders": ["name", "email", "phone", "accountName"],
  "ids": ["rec_001", "rec_002", "..."],
  "errors": [
    {"index": 45, "error": "Invalid email format"},
    {"index": 89, "error": "Name is required"}
  ]
}
```

---

## Lookups

### Search for Lookup Autocomplete

```bash
curl -X GET "http://localhost:8080/api/v1/lookup/Account?search=acme&limit=10" \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "records": [
    {"id": "acc_001", "name": "Acme Corp"},
    {"id": "acc_002", "name": "Acme Industries"}
  ],
  "total": 2
}
```

---

### Get Single Lookup Record

```bash
curl -X GET http://localhost:8080/api/v1/lookup/Account/acc_001 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "acc_001",
  "name": "Acme Corp"
}
```

---

### Batch Get Multiple Lookup Records

```bash
curl -X POST http://localhost:8080/api/v1/lookup/Account/batch \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "ids": ["acc_001", "acc_002", "acc_003"]
  }'
```

**Response (200):**
```json
{
  "records": [
    {"id": "acc_001", "name": "Acme Corp"},
    {"id": "acc_002", "name": "Beta Inc"},
    {"id": "acc_003", "name": "Gamma LLC"}
  ],
  "total": 3
}
```

---

## Related Lists

### Get Related Records

```bash
curl -X GET "http://localhost:8080/api/v1/accounts/acc_001/related/Contact?page=1&pageSize=10&sort=createdAt&dir=desc" \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "records": [
    {
      "id": "con_001",
      "firstName": "John",
      "lastName": "Doe",
      "emailAddress": "john@example.com",
      "createdAt": "2024-01-15T10:30:00Z"
    },
    {
      "id": "con_002",
      "firstName": "Jane",
      "lastName": "Smith",
      "emailAddress": "jane@example.com",
      "createdAt": "2024-01-14T09:00:00Z"
    }
  ],
  "total": 5,
  "page": 1,
  "pageSize": 10,
  "totalPages": 1
}
```

---

### Get Related List Configs

```bash
curl -X GET http://localhost:8080/api/v1/entities/Account/related-list-configs \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
[
  {
    "id": "rlc_001",
    "entityType": "Account",
    "relatedEntity": "Contact",
    "lookupField": "accountId",
    "label": "Contacts",
    "enabled": true,
    "displayFields": ["firstName", "lastName", "emailAddress"],
    "sortOrder": 0,
    "defaultSort": "createdAt",
    "defaultSortDir": "desc",
    "pageSize": 5
  },
  {
    "id": "rlc_002",
    "entityType": "Account",
    "relatedEntity": "Task",
    "lookupField": "parentId",
    "label": "Tasks",
    "enabled": true,
    "displayFields": ["subject", "status", "dueDate"],
    "sortOrder": 1,
    "defaultSort": "dueDate",
    "defaultSortDir": "asc",
    "pageSize": 5
  }
]
```

---

### Discover Related List Options (Admin)

```bash
curl -X GET http://localhost:8080/api/v1/entities/Account/related-list-options \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
[
  {
    "relatedEntity": "Contact",
    "lookupField": "accountId",
    "suggestedLabel": "Contacts",
    "isConfigured": true
  },
  {
    "relatedEntity": "Opportunity",
    "lookupField": "accountId",
    "suggestedLabel": "Opportunities",
    "isConfigured": false
  },
  {
    "relatedEntity": "Task",
    "lookupField": "parentId",
    "suggestedLabel": "Tasks",
    "isConfigured": true
  }
]
```

---

### Save Related List Configs (Admin)

```bash
curl -X PUT http://localhost:8080/api/v1/entities/Account/related-list-configs \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "configs": [
      {
        "relatedEntity": "Contact",
        "lookupField": "accountId",
        "label": "Contacts",
        "enabled": true,
        "displayFields": ["firstName", "lastName", "emailAddress", "phoneNumber"],
        "sortOrder": 0,
        "defaultSort": "createdAt",
        "defaultSortDir": "desc",
        "pageSize": 10
      }
    ]
  }'
```

**Response (200):**
```json
[
  {
    "id": "rlc_001",
    "entityType": "Account",
    "relatedEntity": "Contact",
    "lookupField": "accountId",
    "label": "Contacts",
    "enabled": true,
    "displayFields": ["firstName", "lastName", "emailAddress", "phoneNumber"],
    "sortOrder": 0,
    "defaultSort": "createdAt",
    "defaultSortDir": "desc",
    "pageSize": 10
  }
]
```

---

## Admin - Entities & Fields

### List All Entities

```bash
curl -X GET http://localhost:8080/api/v1/admin/entities \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
[
  {
    "name": "Contact",
    "label": "Contact",
    "labelPlural": "Contacts",
    "icon": "user",
    "isCustom": false
  },
  {
    "name": "Account",
    "label": "Account",
    "labelPlural": "Accounts",
    "icon": "building",
    "isCustom": false
  },
  {
    "name": "Job",
    "label": "Job",
    "labelPlural": "Jobs",
    "icon": "briefcase",
    "isCustom": true
  }
]
```

---

### Create Entity

```bash
curl -X POST http://localhost:8080/api/v1/admin/entities \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Project",
    "label": "Project",
    "labelPlural": "Projects",
    "icon": "folder",
    "description": "Track client projects"
  }'
```

**Response (201):**
```json
{
  "name": "Project",
  "label": "Project",
  "labelPlural": "Projects",
  "icon": "folder",
  "description": "Track client projects",
  "isCustom": true,
  "createdAt": "2024-01-16T14:00:00Z"
}
```

---

### Get Entity

```bash
curl -X GET http://localhost:8080/api/v1/admin/entities/Project \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "name": "Project",
  "label": "Project",
  "labelPlural": "Projects",
  "icon": "folder",
  "description": "Track client projects",
  "isCustom": true,
  "createdAt": "2024-01-16T14:00:00Z",
  "modifiedAt": "2024-01-16T14:00:00Z"
}
```

---

### Update Entity

```bash
curl -X PATCH http://localhost:8080/api/v1/admin/entities/Project \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "Client Project",
    "description": "Track all client projects and milestones"
  }'
```

**Response (200):**
```json
{
  "name": "Project",
  "label": "Client Project",
  "labelPlural": "Projects",
  "icon": "folder",
  "description": "Track all client projects and milestones",
  "isCustom": true,
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

### List Field Types

```bash
curl -X GET http://localhost:8080/api/v1/admin/field-types \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
[
  {"type": "varchar", "label": "Text", "description": "Short text up to 255 characters"},
  {"type": "text", "label": "Text Area", "description": "Long text"},
  {"type": "int", "label": "Integer", "description": "Whole number"},
  {"type": "float", "label": "Decimal", "description": "Decimal number"},
  {"type": "currency", "label": "Currency", "description": "Money amount"},
  {"type": "bool", "label": "Checkbox", "description": "True/false"},
  {"type": "date", "label": "Date", "description": "Date only"},
  {"type": "datetime", "label": "Date/Time", "description": "Date and time"},
  {"type": "email", "label": "Email", "description": "Email address"},
  {"type": "phone", "label": "Phone", "description": "Phone number"},
  {"type": "url", "label": "URL", "description": "Web address"},
  {"type": "enum", "label": "Picklist", "description": "Single select from options"},
  {"type": "multiEnum", "label": "Multi-Select", "description": "Multiple select from options"},
  {"type": "link", "label": "Lookup", "description": "Link to another entity"},
  {"type": "linkMultiple", "label": "Multi-Lookup", "description": "Link to multiple records"},
  {"type": "rollup", "label": "Rollup", "description": "Calculated from related records"}
]
```

---

### List Fields

```bash
curl -X GET http://localhost:8080/api/v1/admin/entities/Contact/fields \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
[
  {
    "name": "firstName",
    "label": "First Name",
    "type": "varchar",
    "isRequired": true,
    "isReadOnly": false,
    "sortOrder": 0
  },
  {
    "name": "lastName",
    "label": "Last Name",
    "type": "varchar",
    "isRequired": true,
    "isReadOnly": false,
    "sortOrder": 1
  },
  {
    "name": "account",
    "label": "Account",
    "type": "link",
    "linkEntity": "Account",
    "isRequired": false,
    "isReadOnly": false,
    "sortOrder": 5
  },
  {
    "name": "status",
    "label": "Status",
    "type": "enum",
    "options": ["Active", "Inactive", "Lead"],
    "defaultValue": "Active",
    "isRequired": false,
    "sortOrder": 6
  }
]
```

---

### Create Field

```bash
curl -X POST http://localhost:8080/api/v1/admin/entities/Contact/fields \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "leadSource",
    "label": "Lead Source",
    "type": "enum",
    "options": ["Website", "Referral", "Cold Call", "Event", "Other"],
    "defaultValue": "Website",
    "isRequired": false
  }'
```

**Response (201):**
```json
{
  "name": "leadSource",
  "label": "Lead Source",
  "type": "enum",
  "options": ["Website", "Referral", "Cold Call", "Event", "Other"],
  "defaultValue": "Website",
  "isRequired": false,
  "isReadOnly": false,
  "sortOrder": 10
}
```

---

### Create Lookup Field

```bash
curl -X POST http://localhost:8080/api/v1/admin/entities/Contact/fields \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "referredBy",
    "label": "Referred By",
    "type": "link",
    "linkEntity": "Contact",
    "isRequired": false
  }'
```

**Response (201):**
```json
{
  "name": "referredBy",
  "label": "Referred By",
  "type": "link",
  "linkEntity": "Contact",
  "isRequired": false,
  "sortOrder": 11
}
```

---

### Update Field

```bash
curl -X PUT http://localhost:8080/api/v1/admin/entities/Contact/fields/leadSource \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "Lead Source Channel",
    "options": ["Website", "Referral", "Cold Call", "Event", "Social Media", "Other"]
  }'
```

**Response (200):**
```json
{
  "name": "leadSource",
  "label": "Lead Source Channel",
  "type": "enum",
  "options": ["Website", "Referral", "Cold Call", "Event", "Social Media", "Other"],
  "defaultValue": "Website",
  "isRequired": false
}
```

---

### Delete Field

```bash
curl -X DELETE http://localhost:8080/api/v1/admin/entities/Contact/fields/leadSource \
  -H "Authorization: Bearer <access_token>"
```

**Response (204):** No content

---

### Reorder Fields

```bash
curl -X POST http://localhost:8080/api/v1/admin/entities/Contact/fields/reorder \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "fieldOrder": ["firstName", "lastName", "emailAddress", "phoneNumber", "account", "status"]
  }'
```

**Response (200):**
```json
{
  "success": true
}
```

---

## Admin - Layouts

### Get Layout

```bash
curl -X GET http://localhost:8080/api/v1/admin/entities/Contact/layouts/detail \
  -H "Authorization: Bearer <access_token>"
```

**Layout types:** `list`, `detail`, `create`

**Response (200):**
```json
{
  "id": "lay_001",
  "entityName": "Contact",
  "layoutType": "detail",
  "layoutData": "[{\"name\":\"firstName\",\"width\":200},{\"name\":\"lastName\",\"width\":200}]",
  "exists": true,
  "createdAt": "2024-01-10T09:00:00Z",
  "modifiedAt": "2024-01-15T10:30:00Z"
}
```

---

### Save Layout

```bash
curl -X PUT http://localhost:8080/api/v1/admin/entities/Contact/layouts/list \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "layoutData": "[{\"name\":\"firstName\",\"width\":150},{\"name\":\"lastName\",\"width\":150},{\"name\":\"emailAddress\",\"width\":250},{\"name\":\"account\",\"width\":200}]"
  }'
```

**Response (200):**
```json
{
  "id": "lay_002",
  "entityName": "Contact",
  "layoutType": "list",
  "layoutData": "[{\"name\":\"firstName\",\"width\":150},{\"name\":\"lastName\",\"width\":150},{\"name\":\"emailAddress\",\"width\":250},{\"name\":\"account\",\"width\":200}]",
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

### Get Layout V2 (Sections)

```bash
curl -X GET http://localhost:8080/api/v1/admin/entities/Contact/layouts/detail/v2 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "entityName": "Contact",
  "layoutType": "detail",
  "layout": {
    "version": "v2",
    "sections": [
      {
        "id": "sec_001",
        "label": "Basic Information",
        "columns": 2,
        "isCollapsible": false,
        "isCollapsed": false,
        "fields": ["firstName", "lastName", "emailAddress", "phoneNumber"]
      },
      {
        "id": "sec_002",
        "label": "Additional Details",
        "columns": 2,
        "isCollapsible": true,
        "isCollapsed": true,
        "fields": ["account", "status", "leadSource"]
      }
    ]
  },
  "exists": true
}
```

---

### Save Layout V2

```bash
curl -X PUT http://localhost:8080/api/v1/admin/entities/Contact/layouts/detail/v2 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "v2",
    "sections": [
      {
        "id": "sec_001",
        "label": "Contact Info",
        "columns": 2,
        "fields": ["firstName", "lastName", "emailAddress", "phoneNumber"]
      },
      {
        "id": "sec_002",
        "label": "Company",
        "columns": 1,
        "fields": ["account"]
      }
    ]
  }'
```

**Response (200):**
```json
{
  "id": "lay_001",
  "entityName": "Contact",
  "layoutType": "detail",
  "layout": {
    "version": "v2",
    "sections": [...]
  },
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

## Admin - Navigation

### List All Tabs (Admin)

```bash
curl -X GET http://localhost:8080/api/v1/admin/navigation \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
[
  {
    "id": "nav_001",
    "label": "Contacts",
    "href": "/contacts",
    "icon": "user",
    "entityName": "Contact",
    "isVisible": true,
    "isSystem": true,
    "sortOrder": 0
  },
  {
    "id": "nav_002",
    "label": "Accounts",
    "href": "/accounts",
    "icon": "building",
    "entityName": "Account",
    "isVisible": true,
    "isSystem": true,
    "sortOrder": 1
  },
  {
    "id": "nav_003",
    "label": "Projects",
    "href": "/projects",
    "icon": "folder",
    "entityName": "Project",
    "isVisible": true,
    "isSystem": false,
    "sortOrder": 2
  }
]
```

---

### List Visible Tabs (All Users)

```bash
curl -X GET http://localhost:8080/api/v1/navigation \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
[
  {
    "id": "nav_001",
    "label": "Contacts",
    "href": "/contacts",
    "icon": "user"
  },
  {
    "id": "nav_002",
    "label": "Accounts",
    "href": "/accounts",
    "icon": "building"
  }
]
```

---

### Create Navigation Tab

```bash
curl -X POST http://localhost:8080/api/v1/admin/navigation \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "Reports",
    "href": "/reports",
    "icon": "chart-bar",
    "isVisible": true
  }'
```

**Response (201):**
```json
{
  "id": "nav_004",
  "label": "Reports",
  "href": "/reports",
  "icon": "chart-bar",
  "isVisible": true,
  "isSystem": false,
  "sortOrder": 3
}
```

---

### Update Navigation Tab

```bash
curl -X PUT http://localhost:8080/api/v1/admin/navigation/nav_003 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "Client Projects",
    "icon": "briefcase"
  }'
```

**Response (200):**
```json
{
  "id": "nav_003",
  "label": "Client Projects",
  "href": "/projects",
  "icon": "briefcase",
  "isVisible": true,
  "isSystem": false,
  "sortOrder": 2
}
```

---

### Delete Navigation Tab

```bash
curl -X DELETE http://localhost:8080/api/v1/admin/navigation/nav_004 \
  -H "Authorization: Bearer <access_token>"
```

**Response (204):** No content

**Error (403) - Cannot delete system tab:**
```json
{
  "error": "cannot delete system navigation tab"
}
```

---

### Reorder Navigation Tabs

```bash
curl -X POST http://localhost:8080/api/v1/admin/navigation/reorder \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "tabIds": ["nav_002", "nav_001", "nav_003"]
  }'
```

**Response (200):**
```json
[
  {"id": "nav_002", "label": "Accounts", "sortOrder": 0},
  {"id": "nav_001", "label": "Contacts", "sortOrder": 1},
  {"id": "nav_003", "label": "Projects", "sortOrder": 2}
]
```

---

## Admin - Tripwires (Webhooks)

### List Tripwires

```bash
curl -X GET "http://localhost:8080/api/v1/tripwires?entityType=Contact&enabled=true" \
  -H "Authorization: Bearer <access_token>"
```

**Query Parameters:**
| Param | Description |
|-------|-------------|
| search | Search by name |
| entityType | Filter by entity |
| enabled | true/false |
| sortBy | created_at (default) |
| sortDir | desc (default) |
| page | Page number |
| pageSize | Records per page |

**Response (200):**
```json
{
  "data": [
    {
      "id": "tw_001",
      "name": "Notify on Contact Create",
      "entityType": "Contact",
      "endpointURL": "https://hooks.example.com/contact-created",
      "enabled": true,
      "conditions": [
        {"eventType": "CREATE", "field": null, "operator": null, "value": null}
      ],
      "createdAt": "2024-01-10T09:00:00Z",
      "modifiedAt": "2024-01-10T09:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 20
}
```

---

### Create Tripwire

```bash
curl -X POST http://localhost:8080/api/v1/tripwires \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "High Value Deal Alert",
    "entityType": "Opportunity",
    "endpointURL": "https://hooks.example.com/high-value-deal",
    "conditions": [
      {
        "eventType": "CREATE",
        "field": "amount",
        "operator": "greaterThan",
        "value": "100000"
      },
      {
        "eventType": "UPDATE",
        "field": "amount",
        "operator": "greaterThan",
        "value": "100000"
      }
    ]
  }'
```

**Condition Operators:**
- `equals`, `notEquals`
- `contains`, `notContains`
- `startsWith`, `endsWith`
- `greaterThan`, `lessThan`, `greaterThanOrEqual`, `lessThanOrEqual`
- `isBlank`, `isNotBlank`
- `changed`, `changedTo`, `changedFrom`

**Response (201):**
```json
{
  "id": "tw_002",
  "name": "High Value Deal Alert",
  "entityType": "Opportunity",
  "endpointURL": "https://hooks.example.com/high-value-deal",
  "enabled": true,
  "conditions": [...],
  "createdAt": "2024-01-16T14:00:00Z"
}
```

---

### Get Tripwire

```bash
curl -X GET http://localhost:8080/api/v1/tripwires/tw_001 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "tw_001",
  "name": "Notify on Contact Create",
  "entityType": "Contact",
  "endpointURL": "https://hooks.example.com/contact-created",
  "enabled": true,
  "conditions": [
    {"eventType": "CREATE", "field": null, "operator": null, "value": null}
  ],
  "createdAt": "2024-01-10T09:00:00Z",
  "modifiedAt": "2024-01-10T09:00:00Z",
  "createdById": "usr_ABC123",
  "modifiedById": "usr_ABC123"
}
```

---

### Update Tripwire

```bash
curl -X PUT http://localhost:8080/api/v1/tripwires/tw_001 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Notify on Contact Create or Update",
    "conditions": [
      {"eventType": "CREATE"},
      {"eventType": "UPDATE"}
    ]
  }'
```

**Response (200):**
```json
{
  "id": "tw_001",
  "name": "Notify on Contact Create or Update",
  "conditions": [
    {"eventType": "CREATE"},
    {"eventType": "UPDATE"}
  ],
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

### Delete Tripwire

```bash
curl -X DELETE http://localhost:8080/api/v1/tripwires/tw_001 \
  -H "Authorization: Bearer <access_token>"
```

**Response (204):** No content

---

### Toggle Tripwire

```bash
curl -X POST http://localhost:8080/api/v1/tripwires/tw_001/toggle \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "tw_001",
  "enabled": false,
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

### Test Tripwire

```bash
curl -X POST http://localhost:8080/api/v1/tripwires/tw_001/test \
  -H "Authorization: Bearer <access_token>"
```

**Response (200) - Success:**
```json
{
  "success": true,
  "statusCode": 200,
  "statusText": "200 OK",
  "durationMs": 156,
  "responseBody": "{\"received\":true}"
}
```

**Response (200) - Failure:**
```json
{
  "success": false,
  "statusCode": 500,
  "statusText": "500 Internal Server Error",
  "durationMs": 2034,
  "error": "connection refused"
}
```

---

### Get Tripwire Execution Logs

```bash
curl -X GET "http://localhost:8080/api/v1/tripwires/tw_001/logs?page=1&pageSize=20" \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "data": [
    {
      "id": "log_001",
      "tripwireId": "tw_001",
      "eventType": "CREATE",
      "recordId": "con_ABC123",
      "status": "success",
      "statusCode": 200,
      "durationMs": 145,
      "executedAt": "2024-01-16T14:30:00Z"
    },
    {
      "id": "log_002",
      "tripwireId": "tw_001",
      "eventType": "CREATE",
      "recordId": "con_DEF456",
      "status": "failed",
      "statusCode": 500,
      "errorMessage": "Internal Server Error",
      "durationMs": 3012,
      "executedAt": "2024-01-16T14:25:00Z"
    }
  ],
  "total": 2,
  "page": 1,
  "pageSize": 20
}
```

---

### Get Webhook Settings

```bash
curl -X GET http://localhost:8080/api/v1/settings/webhooks \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "authType": "apiKey",
  "apiKey": "sk_****1234",
  "timeoutMs": 10000,
  "retryCount": 3,
  "retryDelayMs": 1000
}
```

---

### Save Webhook Settings

```bash
curl -X PUT http://localhost:8080/api/v1/settings/webhooks \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "authType": "bearer",
    "bearerToken": "my-secret-token",
    "timeoutMs": 15000,
    "retryCount": 5
  }'
```

**Auth Types:**
- `none` - No authentication
- `apiKey` - X-API-Key header
- `bearer` - Authorization: Bearer header
- `customHeader` - Custom header name/value

**Response (200):**
```json
{
  "authType": "bearer",
  "timeoutMs": 15000,
  "retryCount": 5
}
```

---

## Admin - Validation Rules

### List Validation Rules

```bash
curl -X GET http://localhost:8080/api/v1/admin/entities/Contact/validation-rules \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "data": [
    {
      "id": "vr_001",
      "name": "Require Email for Active Contacts",
      "entityType": "Contact",
      "enabled": true,
      "conditions": [
        {"field": "status", "operator": "equals", "value": "Active"},
        {"field": "email", "operator": "isBlank"}
      ],
      "actions": [
        {"type": "blockSave", "message": "Email is required for active contacts"}
      ],
      "runOn": ["CREATE", "UPDATE"],
      "createdAt": "2024-01-10T09:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 20
}
```

---

### Create Validation Rule

```bash
curl -X POST http://localhost:8080/api/v1/admin/entities/Contact/validation-rules \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Prevent Duplicate Emails",
    "conditions": [
      {
        "field": "email",
        "operator": "isDuplicate"
      }
    ],
    "actions": [
      {
        "type": "blockSave",
        "message": "A contact with this email already exists",
        "field": "email"
      }
    ],
    "runOn": ["CREATE", "UPDATE"]
  }'
```

**Condition Operators:**
- `equals`, `notEquals`, `contains`, `notContains`
- `startsWith`, `endsWith`
- `isBlank`, `isNotBlank`
- `greaterThan`, `lessThan`, `greaterThanOrEqual`, `lessThanOrEqual`
- `isDuplicate` - Check for duplicate values
- `changed`, `changedTo`, `changedFrom`
- `matches` - Regex pattern

**Action Types:**
- `blockSave` - Prevent save with error message
- `showWarning` - Show warning but allow save
- `setFieldValue` - Auto-populate a field
- `clearFieldValue` - Clear a field

**Response (201):**
```json
{
  "id": "vr_002",
  "name": "Prevent Duplicate Emails",
  "entityType": "Contact",
  "enabled": true,
  "conditions": [...],
  "actions": [...],
  "runOn": ["CREATE", "UPDATE"],
  "createdAt": "2024-01-16T14:00:00Z"
}
```

---

### Get Validation Rule

```bash
curl -X GET http://localhost:8080/api/v1/admin/entities/Contact/validation-rules/vr_001 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "vr_001",
  "name": "Require Email for Active Contacts",
  "entityType": "Contact",
  "enabled": true,
  "conditions": [...],
  "actions": [...],
  "runOn": ["CREATE", "UPDATE"],
  "createdAt": "2024-01-10T09:00:00Z",
  "modifiedAt": "2024-01-10T09:00:00Z"
}
```

---

### Update Validation Rule

```bash
curl -X PUT http://localhost:8080/api/v1/admin/entities/Contact/validation-rules/vr_001 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Require Email and Phone for Active Contacts",
    "conditions": [
      {"field": "status", "operator": "equals", "value": "Active"},
      {"field": "email", "operator": "isBlank"},
      {"field": "phone", "operator": "isBlank"}
    ],
    "actions": [
      {"type": "blockSave", "message": "Email and phone are required for active contacts"}
    ]
  }'
```

**Response (200):**
```json
{
  "id": "vr_001",
  "name": "Require Email and Phone for Active Contacts",
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

### Delete Validation Rule

```bash
curl -X DELETE http://localhost:8080/api/v1/admin/entities/Contact/validation-rules/vr_001 \
  -H "Authorization: Bearer <access_token>"
```

**Response (204):** No content

---

### Toggle Validation Rule

```bash
curl -X POST http://localhost:8080/api/v1/admin/entities/Contact/validation-rules/vr_001/toggle \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "vr_001",
  "enabled": false,
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

### Test Validation Rule

```bash
curl -X POST http://localhost:8080/api/v1/admin/entities/Contact/validation-rules/test \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "rule": {
      "conditions": [
        {"field": "email", "operator": "isBlank"}
      ],
      "actions": [
        {"type": "blockSave", "message": "Email is required"}
      ]
    },
    "operation": "CREATE",
    "oldRecord": null,
    "newRecord": {
      "name": "Test Contact",
      "email": ""
    }
  }'
```

**Response (200) - Rule triggered:**
```json
{
  "valid": false,
  "message": "Email is required",
  "fieldErrors": [
    {"field": "email", "message": "Email is required"}
  ],
  "conditionsMatched": true
}
```

**Response (200) - Rule not triggered:**
```json
{
  "valid": true,
  "conditionsMatched": false
}
```

---

## Users

### List Users

```bash
curl -X GET "http://localhost:8080/api/v1/users?page=1&pageSize=20" \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "data": [
    {
      "id": "usr_ABC123",
      "email": "admin@acme.com",
      "firstName": "John",
      "lastName": "Doe",
      "role": "owner",
      "isActive": true,
      "lastLoginAt": "2024-01-16T10:00:00Z"
    },
    {
      "id": "usr_DEF456",
      "email": "user@acme.com",
      "firstName": "Jane",
      "lastName": "Smith",
      "role": "user",
      "isActive": true,
      "lastLoginAt": "2024-01-15T14:30:00Z"
    }
  ],
  "total": 2,
  "page": 1,
  "pageSize": 20
}
```

---

### Get User

```bash
curl -X GET http://localhost:8080/api/v1/users/usr_DEF456 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "usr_DEF456",
  "email": "user@acme.com",
  "firstName": "Jane",
  "lastName": "Smith",
  "role": "user",
  "isActive": true,
  "lastLoginAt": "2024-01-15T14:30:00Z",
  "createdAt": "2024-01-05T09:00:00Z"
}
```

---

### Update User Role (Admin Only)

```bash
curl -X PUT http://localhost:8080/api/v1/users/usr_DEF456/role \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "role": "admin"
  }'
```

**Roles:** `owner`, `admin`, `user`

**Response (200):**
```json
{
  "id": "usr_DEF456",
  "email": "user@acme.com",
  "firstName": "Jane",
  "lastName": "Smith",
  "role": "admin"
}
```

**Error (403) - Admin trying to promote to owner:**
```json
{
  "error": "Only owners can promote users to owner"
}
```

---

### Remove User from Organization (Admin Only)

```bash
curl -X DELETE http://localhost:8080/api/v1/users/usr_DEF456 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "message": "User removed from organization"
}
```

**Error (400) - Last owner:**
```json
{
  "error": "Cannot remove the last owner. Transfer ownership first."
}
```

---

## API Tokens

### Create API Token (Admin Only)

```bash
curl -X POST http://localhost:8080/api/v1/api-tokens \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "n8n Integration",
    "scopes": ["read", "write"],
    "expiresIn": 31536000
  }'
```

**Scopes:** `read`, `write`, `admin`

**Response (201):**
```json
{
  "id": "tok_ABC123",
  "name": "n8n Integration",
  "token": "qcrm_xxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "scopes": ["read", "write"],
  "expiresAt": "2025-01-16T14:00:00Z",
  "createdAt": "2024-01-16T14:00:00Z"
}
```

> **Important:** The `token` value is only shown once at creation time. Store it securely.

---

### List API Tokens

```bash
curl -X GET http://localhost:8080/api/v1/api-tokens \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "tokens": [
    {
      "id": "tok_ABC123",
      "name": "n8n Integration",
      "scopes": ["read", "write"],
      "lastUsedAt": "2024-01-16T15:30:00Z",
      "expiresAt": "2025-01-16T14:00:00Z",
      "createdAt": "2024-01-16T14:00:00Z",
      "isActive": true
    }
  ]
}
```

---

### Revoke API Token

```bash
curl -X POST http://localhost:8080/api/v1/api-tokens/tok_ABC123/revoke \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "message": "Token revoked successfully"
}
```

---

### Delete API Token

```bash
curl -X DELETE http://localhost:8080/api/v1/api-tokens/tok_ABC123 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "message": "Token deleted successfully"
}
```

---

## Screen Flows

### List Flows

```bash
curl -X GET http://localhost:8080/api/v1/flows \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "data": [
    {
      "id": "flow_001",
      "name": "Case Escalation",
      "description": "Escalate high priority cases",
      "isActive": true,
      "createdAt": "2024-01-10T09:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 20
}
```

---

### Get Flow

```bash
curl -X GET http://localhost:8080/api/v1/flows/flow_001 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "flow_001",
  "name": "Case Escalation",
  "description": "Escalate high priority cases",
  "isActive": true,
  "definition": {
    "trigger": {
      "type": "manual",
      "entityType": "Case",
      "buttonLabel": "Escalate"
    },
    "steps": [...]
  },
  "createdAt": "2024-01-10T09:00:00Z"
}
```

---

### Get Flows for Entity (for UI buttons)

```bash
curl -X GET http://localhost:8080/api/v1/flows/entity/Case \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "flows": [
    {
      "id": "flow_001",
      "name": "Case Escalation",
      "buttonLabel": "Escalate",
      "showOn": ["detail"]
    }
  ]
}
```

---

### Start Flow Execution

```bash
curl -X POST http://localhost:8080/api/v1/flows/flow_001/start \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "entity": "Case",
    "recordId": "case_ABC123"
  }'
```

**Response (200):**
```json
{
  "id": "exec_XYZ789",
  "flowId": "flow_001",
  "status": "running",
  "currentStep": "screen_assessment",
  "variables": {
    "$record": {"id": "case_ABC123", "subject": "Issue with login"}
  },
  "screen": {
    "title": "Escalation Assessment",
    "fields": [
      {"name": "impact", "label": "Business Impact", "type": "select", "options": ["Low", "Medium", "High", "Critical"]},
      {"name": "urgency", "label": "Urgency", "type": "select", "options": ["Low", "Medium", "High", "Critical"]},
      {"name": "notes", "label": "Escalation Notes", "type": "textarea"}
    ]
  },
  "startedAt": "2024-01-16T14:00:00Z"
}
```

---

### Get Execution State

```bash
curl -X GET http://localhost:8080/api/v1/flows/executions/exec_XYZ789 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "id": "exec_XYZ789",
  "flowId": "flow_001",
  "status": "waiting",
  "currentStep": "screen_assessment",
  "variables": {...},
  "screen": {...}
}
```

---

### Submit Screen Data

```bash
curl -X POST http://localhost:8080/api/v1/flows/executions/exec_XYZ789/submit \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "impact": "High",
    "urgency": "Critical",
    "notes": "Customer cannot access their account, blocking production deployment"
  }'
```

**Response (200) - Flow continues:**
```json
{
  "id": "exec_XYZ789",
  "status": "running",
  "currentStep": "screen_confirm",
  "screen": {
    "title": "Confirm Escalation",
    "fields": [...]
  }
}
```

**Response (200) - Flow completed:**
```json
{
  "id": "exec_XYZ789",
  "status": "completed",
  "result": {
    "message": "Case has been escalated to Tier 2 Support",
    "redirectUrl": "/cases/case_ABC123"
  },
  "completedAt": "2024-01-16T14:05:00Z"
}
```

---

### Create Flow (Admin Only)

```bash
curl -X POST http://localhost:8080/api/v1/flows \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "New Customer Onboarding",
    "description": "Guide users through customer setup",
    "isActive": true,
    "definition": {
      "trigger": {
        "type": "manual",
        "entityType": "Account",
        "buttonLabel": "Start Onboarding",
        "showOn": ["detail"]
      },
      "steps": [
        {
          "id": "screen_welcome",
          "type": "screen",
          "name": "Welcome",
          "screen": {
            "title": "Customer Onboarding",
            "fields": [
              {"name": "primaryContact", "label": "Primary Contact Name", "type": "text", "required": true}
            ]
          },
          "next": "end_complete"
        },
        {
          "id": "end_complete",
          "type": "end",
          "name": "Complete",
          "end": {
            "message": "Onboarding started successfully"
          }
        }
      ]
    }
  }'
```

**Response (201):**
```json
{
  "id": "flow_002",
  "name": "New Customer Onboarding",
  "isActive": true,
  "createdAt": "2024-01-16T14:00:00Z"
}
```

---

### Update Flow (Admin Only)

```bash
curl -X PUT http://localhost:8080/api/v1/flows/flow_002 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Customer Onboarding v2",
    "isActive": false
  }'
```

**Response (200):**
```json
{
  "id": "flow_002",
  "name": "Customer Onboarding v2",
  "isActive": false,
  "modifiedAt": "2024-01-16T15:00:00Z"
}
```

---

### Delete Flow (Admin Only)

```bash
curl -X DELETE http://localhost:8080/api/v1/flows/flow_002 \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "deleted": true
}
```

---

### List Flow Executions (Admin Only)

```bash
curl -X GET "http://localhost:8080/api/v1/flows/flow_001/executions?status=completed&limit=50" \
  -H "Authorization: Bearer <access_token>"
```

**Response (200):**
```json
{
  "data": [
    {
      "id": "exec_001",
      "flowId": "flow_001",
      "status": "completed",
      "startedBy": "usr_ABC123",
      "entityType": "Case",
      "recordId": "case_ABC123",
      "startedAt": "2024-01-16T14:00:00Z",
      "completedAt": "2024-01-16T14:05:00Z"
    }
  ]
}
```

---

## Error Responses

All errors return JSON with an `error` field:

```json
{
  "error": "Error message description"
}
```

**HTTP Status Codes:**

| Code | Meaning |
|------|---------|
| 400 | Bad Request - Invalid input or missing required fields |
| 401 | Unauthorized - Invalid or missing authentication token |
| 403 | Forbidden - Insufficient permissions for this action |
| 404 | Not Found - Resource doesn't exist |
| 409 | Conflict - Resource already exists (e.g., duplicate email) |
| 422 | Unprocessable Entity - Validation failed |
| 500 | Internal Server Error |

**Validation Error Response (422):**
```json
{
  "error": "Validation failed",
  "fieldErrors": [
    {"field": "email", "message": "Email is required"},
    {"field": "phone", "message": "Invalid phone format"}
  ]
}
```

---

## Webhook Payloads (Outgoing)

When tripwires fire, they send POST requests to configured endpoints:

```json
{
  "tripwireId": "tw_001",
  "tripwireName": "Notify on Contact Create",
  "event": "CREATE",
  "entityType": "Contact",
  "recordId": "con_ABC123",
  "timestamp": "2024-01-16T14:00:00Z",
  "oldRecord": null,
  "newRecord": {
    "id": "con_ABC123",
    "firstName": "John",
    "lastName": "Doe",
    "email": "john@example.com",
    "createdAt": "2024-01-16T14:00:00Z"
  }
}
```

**Headers sent with webhook:**
```
Content-Type: application/json
User-Agent: FastCRM-Webhook/1.0
X-FastCRM-Event: CREATE
X-API-Key: <if configured>
Authorization: Bearer <if configured>
```

**Update event includes both old and new:**
```json
{
  "event": "UPDATE",
  "oldRecord": {
    "id": "con_ABC123",
    "firstName": "John",
    "status": "Lead"
  },
  "newRecord": {
    "id": "con_ABC123",
    "firstName": "John",
    "status": "Active"
  }
}
```
