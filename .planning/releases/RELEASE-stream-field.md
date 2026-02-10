# Release Notes: Stream Field Type

**Version:** 1.0
**Release Date:** 2026-02-03
**Type:** New Feature

---

## Overview

Introducing the **Stream** field type - a Twitter/journal-style field that enables lightweight, timestamped note-taking directly on any entity record. Perfect for activity logs, communication history, internal notes, and audit trails.

---

## Features

### Add Stream Fields to Any Entity
- Available in Entity Manager under field types
- Creates two database columns automatically: entry field + log field
- Works with all custom entities

### Inline Entry Submission
- Add entries directly from the detail page - no need to navigate to edit mode
- Text area with "Add Entry" button
- Keyboard shortcut: **Ctrl+Enter** (or **Cmd+Enter** on Mac) to submit
- Entries automatically timestamped (YYYY-MM-DD HH:MM format)
- Input clears after successful submission

### Entry History
- Entries displayed in reverse chronological order (newest first)
- Scrollable history panel (max 256px height)
- Entry count badge shows total number of entries
- Timestamps displayed in a subtle, readable format

### Delete Entries
- Hover over any entry to reveal delete button
- Confirmation dialog prevents accidental deletion
- Loading state while deletion is processing
- History updates immediately after deletion

---

## How to Use

### 1. Add a Stream Field to an Entity

1. Navigate to **Setup → Entity Manager**
2. Select your entity (e.g., Product, Contact, Account)
3. Go to the **Fields** tab
4. Click **Add Field**
5. Select **Stream** as the field type
6. Enter a label (e.g., "Activity Log", "Notes", "Communication History")
7. Save

### 2. Add Entries

**From the Detail Page:**
1. Navigate to any record of that entity
2. Find the Stream field in the form
3. Type your entry in the text area
4. Click **Add Entry** or press **Ctrl+Enter**
5. Entry appears at the top of the history with timestamp

**From the Edit Page:**
1. Click **Edit** on the record
2. Type your entry in the Stream field
3. Click **Save**
4. Entry is added to the log

### 3. Delete Entries

1. Hover over the entry you want to delete
2. Click the trash icon that appears
3. Confirm deletion in the dialog
4. Entry is removed from the history

---

## Technical Details

### Database Schema
When a Stream field is created, two columns are added to the entity table:
- `{field_name}` (VARCHAR) - Temporary entry field, cleared after save
- `{field_name}_log` (TEXT) - Persistent log of all entries

### Log Format
Entries are stored as newline-separated text:
```
2026-02-03 16:45 - Latest entry text here
2026-02-03 14:30 - Previous entry text
2026-02-03 09:15 - Oldest entry in view
```

### API Endpoints

**Add Entry (via record update):**
```
PUT /api/v1/entities/{entity}/records/{id}
Body: { "fieldName": "Entry text here" }
```

**Delete Entry:**
```
DELETE /api/v1/entities/{entity}/records/{id}/stream/{fieldName}?index={entryIndex}
```

---

## Use Cases

| Use Case | Field Label | Example Entries |
|----------|-------------|-----------------|
| Sales Activity | Activity Log | "Called to discuss Q1 pricing", "Sent proposal via email" |
| Support Tickets | Resolution Notes | "Escalated to engineering", "Customer confirmed fix works" |
| Project Management | Status Updates | "Milestone 1 complete", "Waiting on client approval" |
| HR Records | Interview Notes | "Strong technical skills", "Schedule follow-up" |
| Inventory | Stock Notes | "Reordered 50 units", "Quality check passed" |

---

## Commits

- `5bbae6d` - feat: add Stream field type for journal/Twitter-style entries
- `10e0a2f` - fix(stream): correct key mapping for stream field log retrieval
- `b05b66a` - feat(stream): add inline entry submission on detail pages
- `27eb880` - feat(stream): add ability to delete log entries

---

## Known Limitations

- Entries cannot be edited after creation (append-only by design)
- No rich text formatting (plain text only)
- No file attachments within entries
- No character limit on entries

---

## Future Enhancements (Out of Scope for v1)

- Rich text formatting
- @mentions and notifications
- File attachments per entry
- Entry search/filter
- Export log to CSV/PDF

---

*Released by the Quantico CRM Team*
