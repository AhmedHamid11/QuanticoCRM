# How to Mirror Your Salesforce Data into Quantico CRM

## TL;DR

**Mirroring** copies your data from Salesforce into Quantico CRM automatically. You set it up once, and then whenever Salesforce sends new data, Quantico catches it, checks for duplicates, and adds only the new stuff. Here's the short version:

1. Go to **Admin > Mirrors** and create a new mirror (pick a name, choose which type of record to import, like Contacts or Accounts)
2. Tell the mirror what fields to expect from Salesforce (like First Name, Last Name, Email)
3. Map those Salesforce fields to the matching Quantico fields
4. Go to **Admin > API Tokens** and generate a key (copy it immediately — you only see it once!)
5. Give that key to whoever manages your Salesforce automations so they can connect
6. Watch your data flow in from the **Job History** section on the mirror page

That's it. Quantico handles duplicates, errors, and keeps everything organized.

---

## What Is Mirroring?

Think of mirroring like setting up a mailbox between Salesforce and Quantico CRM.

- **Salesforce** is the sender — it packages up your records (contacts, accounts, etc.) and drops them in the mailbox
- **The Mirror** is the mailbox — it checks what's inside, makes sure everything looks right, and passes it along
- **Quantico CRM** is the receiver — it takes the good stuff and files it away

The mirror is smart: if Salesforce sends the same record twice, Quantico won't create a duplicate. It only adds genuinely new records.

---

## Before You Start

You'll need:

- **Admin access** to your Quantico CRM account
- **Someone who manages your Salesforce automations** (or a tool like RudderStack, Fivetran, etc.) — they'll handle the Salesforce side of the connection
- About **10 minutes** to set everything up on the Quantico side

---

## Step 1: Create a Mirror

A **mirror** is a set of rules that tells Quantico what kind of data to expect from Salesforce and where to put it.

1. Click the **gear icon** in the top-right corner of Quantico
2. Click **Admin**
3. Find the **Mirrors** card and click it
4. Click the **"Create Mirror"** button

You'll see a form asking for:

| Field | What to enter | Example |
|-------|---------------|---------|
| **Mirror Name** | A friendly name so you remember what this is for | `Salesforce Contacts` |
| **Target Entity** | What type of Quantico record should be created from this data. An **entity** is just a category of records — like Contacts, Accounts, or Leads. | `Contact` |
| **Unique Key Field** | The field name from Salesforce that uniquely identifies each record. This is how Quantico knows if it's seen a record before. Salesforce calls this the "Record ID" — it usually looks like `Id`. | `Id` |
| **Unmapped Field Mode** | What to do if Salesforce sends a field you haven't set up yet. Pick **Flexible** to accept it anyway (recommended to start), or **Strict** to reject it. | `Flexible` |
| **Rate Limit** | How many batches per minute this mirror will accept. The default (500) is fine for most setups. | `500` |

Click **"Create Mirror"** and you'll land on the mirror's setup page.

---

## Step 2: Define Your Source Fields

**Source fields** are the fields that Salesforce will send to you. You need to tell Quantico what to expect.

On the mirror setup page, find the **"Source Fields"** section:

1. Click **"Add Field"**
2. Fill in the details for each field Salesforce will send:

| Setting | What it means | Example |
|---------|---------------|---------|
| **Field Name** | The exact name Salesforce uses for this field. It must match exactly. | `FirstName` |
| **Type** | What kind of data this field holds | `text` |
| **Required** | Check this if every record must have this field. If a record is missing a required field, it will be flagged as an error. | Yes for `LastName`, No for `Phone` |
| **Description** | Optional note for your reference | `Contact's first name` |

3. Repeat for each field. Here are common Salesforce Contact fields:

| Field Name | Type | Required? |
|------------|------|-----------|
| `Id` | text | Yes |
| `FirstName` | text | No |
| `LastName` | text | Yes |
| `Email` | email | No |
| `Phone` | phone | No |
| `MailingCity` | text | No |
| `MailingState` | text | No |
| `Title` | text | No |

4. Click **"Save Source Fields"** when done.

> **Tip:** You don't need to add every single Salesforce field — just the ones you actually want in Quantico.

---

## Step 3: Map Salesforce Fields to Quantico Fields

Now you need to tell Quantico which Salesforce field goes where. This is called **field mapping** — it's like matching columns from one spreadsheet to another.

On the mirror setup page, find the **"Field Mapping"** section. You'll see:

- A list of your Salesforce fields on the **left**
- A dropdown for each one on the **right** showing available Quantico fields

For each Salesforce field, pick the matching Quantico field from the dropdown:

| Salesforce Field | Quantico Field |
|------------------|----------------|
| `FirstName` | `firstName` |
| `LastName` | `lastName` |
| `Email` | `emailAddress` |
| `Phone` | `phoneNumber` |
| `MailingCity` | `city` |
| `Title` | `title` |
| `Id` | Leave as "Not Mapped" (the system uses this behind the scenes for duplicate checking — it doesn't need to show up as a visible field) |

Any unmapped fields will show a **yellow warning**. That's okay for fields like `Id` that are only used for duplicate detection.

Click **"Save Mappings"** when done.

> **Progress tip:** The header shows something like "5 of 7 fields mapped" so you can track how many you've connected.

---

## Step 4: Generate an API Key

An **API key** is like a password that lets Salesforce (or whatever tool is sending data) prove it has permission to send data to your Quantico account.

1. Go back to **Admin**
2. Click **"API Tokens"**
3. Click **"Generate Token"**
4. Fill in:
   - **Name:** Something descriptive like `Salesforce Mirror Key`
   - **Permissions:** Check both **Read** and **Write**
   - **Expiration:** Pick how long this key should last (or "Never" if you want it permanent)
5. Click **"Generate Token"**

**IMPORTANT: You will see the key only once.** A screen will pop up showing your key — it looks something like:

```
qik_a3f9e2b1c4d7e6f8a9b0c1d2e3f4a5b6...
```

**Copy it immediately** and store it somewhere safe (like a password manager). If you lose it, you'll need to create a new one.

---

## Step 5: Connect Salesforce

This step happens on the Salesforce side (or in whatever tool sends data to Quantico). You'll need to give your Salesforce admin or integration tool three pieces of information:

| What to share | Where to find it |
|---------------|-----------------|
| **API endpoint** (the address to send data to) | `https://your-quantico-api-url.com/api/v1/ingest` |
| **API key** | The key you copied in Step 4 |
| **Your Organization ID** | Found in Admin > Settings, or ask your Quantico admin |
| **Mirror ID** | Shown on the mirror detail page (looks like `mir_001`) |

Your Salesforce admin will set up an automation (like a Flow, a scheduled job, or a middleware tool) that packages up records and sends them to that address.

The data Salesforce sends should look like this:

```json
{
  "org_id": "your-org-id",
  "mirror_id": "your-mirror-id",
  "records": [
    {
      "Id": "003XX000001ABC",
      "FirstName": "Sarah",
      "LastName": "Johnson",
      "Email": "sarah.johnson@example.com",
      "Phone": "555-0100"
    },
    {
      "Id": "003XX000001DEF",
      "FirstName": "Mike",
      "LastName": "Chen",
      "Email": "mike.chen@example.com"
    }
  ]
}
```

> **Don't worry** — your Salesforce admin or integration tool will handle formatting this. You just need to give them the connection details above.

---

## Step 6: Watch It Work

Once Salesforce starts sending data:

1. Go to **Admin > Mirrors**
2. Click on your mirror
3. Scroll down to **"Job History"**

You'll see a log of every batch of data that came in:

| What you'll see | What it means |
|----------------|---------------|
| **Green "complete" badge** | Everything worked. All records were processed. |
| **Blue "processing" badge** (animated) | Data is being processed right now. |
| **Yellow "partial" badge** | Some records worked, some had problems. Click to see details. |
| **Red "failed" badge** | Something went wrong with the entire batch. Click to see what happened. |
| **Gray "accepted" badge** | Data was received and is waiting to be processed. |

Each job shows a quick summary like: `100 received / 95 ok / 3 skipped / 2 errors`

- **Received:** How many records Salesforce sent
- **Ok:** How many were successfully added to Quantico
- **Skipped:** How many were duplicates (already existed)
- **Errors:** How many had problems (click to see why)

> **The page auto-refreshes** every 30 seconds while jobs are processing, so you can just watch.

---

## How Duplicate Detection Works

Every mirror has a **unique key field** (you set this up in Step 1). For Salesforce, this is usually the `Id` field — every Salesforce record has a unique ID.

Here's what happens:

1. Salesforce sends 100 records
2. Quantico checks each record's unique key against its memory
3. If the key already exists → **skip it** (it's a duplicate)
4. If the key is new → **add it** to Quantico

This means you can safely send the same data multiple times without creating duplicates. The mirror keeps track of everything it's already seen.

**Example:**
- Monday: Salesforce sends 50 contacts → 50 new → all added
- Tuesday: Salesforce sends the same 50 + 10 new contacts → 50 skipped, 10 added
- Wednesday: Salesforce sends just 5 new contacts → 5 added

---

## Troubleshooting

### "I don't see any jobs appearing"

- Make sure your mirror is set to **Active** (check for the green "Active" badge on the mirrors list)
- Confirm the API key is still active (check Admin > API Tokens)
- Ask your Salesforce admin to verify they're using the correct API endpoint, key, and mirror ID

### "Jobs show errors"

Click on the job to expand it. You'll see a table showing exactly which records failed and why. Common reasons:

| Error | What it means | How to fix |
|-------|--------------|------------|
| Required field missing | A record was sent without a field you marked as "Required" | Either un-require the field in your mirror setup, or fix the data in Salesforce |
| Unmapped field (strict mode) | Salesforce sent a field your mirror doesn't know about | Either add the field to your source fields, or switch to "Flexible" mode |
| Rate limit exceeded | Too many batches sent too quickly | Wait a minute and try again, or increase the rate limit in mirror settings |

### "Records appear in jobs but I can't find them in Quantico"

- Check you're looking at the right entity type (if the mirror targets "Contact", look in the Contacts list)
- The records may have been **skipped** as duplicates — check the job details for the skip count

### "I lost my API key"

You'll need to create a new one. Go to Admin > API Tokens, generate a new key, and give the new key to your Salesforce admin. You can deactivate the old key from the same page.

---

## Glossary

| Term | Plain English |
|------|---------------|
| **Mirror** | A set of rules that tells Quantico what data to expect and where to put it |
| **Entity** | A category of records, like Contacts, Accounts, or Leads |
| **Source field** | A field name from Salesforce (the "sender" side) |
| **Target field** | A field name in Quantico (the "receiver" side) |
| **Field mapping** | Connecting a Salesforce field to a Quantico field so data lands in the right place |
| **Unique key** | A value that uniquely identifies each record — used to prevent duplicates |
| **API key** | A secret password that proves the sender has permission to send data |
| **Delta detection** | The process of checking if a record already exists before adding it |
| **Ingest** | The act of receiving and processing incoming data |
| **Job** | A single batch of data that was sent to Quantico — tracked so you can see its status |
| **Rate limit** | A cap on how many batches can be sent per minute — prevents overload |
| **Strict mode** | Mirror rejects data if it contains unexpected fields |
| **Flexible mode** | Mirror accepts data even if it contains unexpected fields (with a warning) |
| **Promotion** | When a validated record gets added to Quantico's actual database |
