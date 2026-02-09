# QuanticoCRM Capture

A Chrome extension that captures screenshots of any page and creates tasks in QuanticoCRM via n8n webhook.

## Features

- 📸 One-click screenshot capture of current tab
- 🔍 Search and select contacts from QuanticoCRM
- 📝 Add optional notes to tasks
- 🔗 Sends data to n8n webhook for processing

## Installation

### Chrome/Edge

1. Open Chrome and go to `chrome://extensions/` (or `edge://extensions/` for Edge)
2. Enable **Developer mode** (toggle in top-right)
3. Click **Load unpacked**
4. Select the `quantico-capture` folder
5. The extension icon should appear in your toolbar

## Configuration

Click the gear icon in the extension popup to configure:

- **n8n Webhook URL**: Your n8n webhook endpoint that receives the capture data
- **QuanticoCRM API URL**: Your CRM API base URL (e.g., `https://crm.yourdomain.com/api/v1`)
- **API Key**: Your QuanticoCRM API key for contact search

## Usage

1. Navigate to any page you want to capture (LinkedIn, email, website, etc.)
2. Click the QuanticoCRM Capture extension icon
3. Click **Capture** to screenshot the current page
4. Search and select a contact
5. Optionally add a note
6. Click **Send to CRM**

## Webhook Payload

The extension sends a JSON payload to your n8n webhook:

```json
{
  "screenshot": "data:image/png;base64,...",
  "source": {
    "url": "https://linkedin.com/in/someone",
    "domain": "linkedin.com",
    "title": "Page Title"
  },
  "contact": {
    "id": "contact_abc123",
    "name": "John Smith",
    "email": "john@example.com"
  },
  "note": "Optional note text",
  "capturedAt": "2024-01-15T10:30:00.000Z"
}
```

## n8n Workflow Setup

Create an n8n workflow with:

1. **Webhook Trigger** - Receives the capture data
2. **Code Node** - Parse the base64 image if needed
3. **QuanticoCRM Node** - Create a Task linked to the contact
4. **Optional**: Store the screenshot in S3/cloud storage

### Example n8n Code Node (Image Handling)

```javascript
// Extract base64 image data
const screenshot = $input.first().json.screenshot;
const base64Data = screenshot.replace(/^data:image\/png;base64,/, '');

// Return for next node
return [{
  json: {
    ...$input.first().json,
    imageBuffer: Buffer.from(base64Data, 'base64')
  }
}];
```

## QuanticoCRM API Requirements

The extension expects the standard EspoCRM-compatible API:

- `GET /Contact?searchQuery={query}` - Contact search
- Returns `{ list: [{ id, name, emailAddress }] }`

## Development

```bash
# Make changes to files
# Reload extension in chrome://extensions/
```

## Permissions

- `activeTab` - Access current tab for screenshot
- `storage` - Save settings locally
- `<all_urls>` - Required for screenshot capture on any site

## Troubleshooting

**Screenshot fails**: Some browser-internal pages (chrome://, about:) cannot be captured.

**Contact search fails**: Verify your API URL and key are correct. Check browser console for errors.

**Webhook fails**: Ensure your n8n webhook is active and the URL is correct.
