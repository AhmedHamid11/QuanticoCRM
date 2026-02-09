# Wulf Recruiting CRM Setup

This document describes the custom recruiting CRM setup created based on the Wulf Recruiting requirements documents.

## Overview

The Wulf Recruiting CRM is designed for executive recruiting with the following core entities:

| Entity | Purpose | Key Fields |
|--------|---------|------------|
| **Client** | Recruiting client companies | Name, Industry, Contract Terms, Status |
| **ClientContact** | Contacts at client companies | Name, Role, Email, Phone, linked to Client |
| **Candidate** | People being recruited | Name, Location, Salary, Relocation prefs, Resume |
| **JobOpening** | Job orders (JO's) | JO#, Title, Client, Priority (AA-D), Salary Range |
| **Submittal** | Pipeline tracking | Candidate → Job, Stage progression, Fees |
| **Activity** | Interaction log | Type, Date, Notes, linked to any entity |
| **Invoice** | Billing tracking | Client, Candidate, Fees, Payment status |

## Pipeline Stages

The Submittal entity tracks candidates through the recruiting pipeline:

```
Submitted → PI_1 → PI_2 → PI_3 → Onsite_1 → Onsite_2 → Offer → Accepted → Started → Placed
```

(PI = Phone Interview, Onsite = On-site Interview)

## Priority Categories

Job Openings use a priority system:
- **AA** - Urgent/Hot priority
- **A** - High priority
- **B** - Normal priority (default)
- **C** - Low priority
- **D** - Inactive/backburner

## Files Created

### SQL Schema
- `/wulf_recruiting_setup.sql` - Complete table definitions

### Go Provisioning
- `/FastCRM/fastcrm/backend/internal/service/provisioning_wulf_recruiting.go` - Metadata provisioning service
- `/FastCRM/fastcrm/backend/cmd/provision-wulf-recruiting/main.go` - Command-line provisioning tool

## How to Provision

### Option 1: Use Existing Org

If you want to add Wulf Recruiting to an existing org:

```bash
cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/backend
go run cmd/provision-wulf-recruiting/main.go --org-id=00DKFC4GC5S000CA70
```

Replace `00DKFC4GC5S000CA70` with your org ID.

### Option 2: Create New Org First

1. Create a new organization through the app's signup flow
2. Note the org ID
3. Run the provisioning command with that org ID

### Command Options

```bash
go run cmd/provision-wulf-recruiting/main.go \
  --org-id=YOUR_ORG_ID \        # Required: Organization ID
  --db=/path/to/fastcrm.db \    # Optional: Database path
  --create-tables=true \        # Optional: Create data tables (default: true)
  --sample-data=true            # Optional: Import sample data (default: true)
```

## Sample Data

The provisioning includes sample data from the original spreadsheets:

### Clients
- Andritz (LDX), Vezer, Cidan, Techtron, AVC, RIE Coatings, etc.

### Job Openings
- JO# 1249-1254: Various positions including APC Sales Manager, Mobile Lab Tech, Veterinarian, Service Technician

### Candidates
- 20+ placeable candidates with their position types, salary expectations, and location preferences

### Pipeline/Submittals
- Historical placement data including commissions and pipeline days

## Entity Relationships

```
                    ┌─────────────────┐
                    │     Client      │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
              ▼              ▼              ▼
    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │ClientContact│  │ JobOpening  │  │   Invoice   │
    └─────────────┘  └──────┬──────┘  └─────────────┘
                            │
                            │
              ┌─────────────┼─────────────┐
              │             │             │
              ▼             ▼             │
    ┌─────────────┐  ┌─────────────┐      │
    │  Candidate  │──│  Submittal  │──────┘
    └──────┬──────┘  └─────────────┘
           │
           ▼
    ┌─────────────┐
    │  Activity   │
    └─────────────┘
```

## Field Types Used

- `varchar` - Text fields
- `text` - Long text/notes
- `date` - Date fields
- `datetime` - Timestamp fields
- `enum` - Dropdown selections (Status, Priority, Stage, etc.)
- `link` - Foreign key relationships
- `bool` - Yes/No fields
- `int` - Integer numbers
- `float` - Decimal numbers
- `currency` - Money amounts
- `email` - Email addresses
- `phone` - Phone numbers
- `url` - Web URLs

## Navigation Tabs

The provisioning creates these navigation tabs:

1. Home
2. Clients
3. Candidates
4. Job Openings
5. Pipeline (Submittals)
6. Invoices
7. Activities

## Bearings (Stage Progress Indicators)

Visual progress bars are created for:
- Submittal pipeline stages
- Job Opening status
- Invoice status
- Client status
- Candidate status

## Next Steps

After provisioning, you may want to:

1. **Import historical data** - Use the sample data as a template for importing your actual data
2. **Configure related lists** - Set up related lists to show Submittals on Candidate/JobOpening pages
3. **Create reports** - Build reports for pipeline metrics, placements, revenue
4. **Set up automation** - Configure tripwires for stage changes

## Troubleshooting

### Tables not showing data
- Verify the org_id matches your authenticated org
- Check the database has the tables: `sqlite3 fastcrm.db ".tables"`

### Provisioning fails
- Ensure you're running from the backend directory
- Check the database path is correct
- Verify the org_id exists in the organizations table

### Fields not appearing
- Run the Entity Manager repair tool
- Check field_defs table for the org_id
