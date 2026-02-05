---
phase: 010-extension-setup-page
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - fastcrm/frontend/src/routes/settings/profile/+page.svelte
autonomous: true

must_haves:
  truths:
    - "User can see Gmail Extension section on Profile Settings page"
    - "User can see installation instructions for the extension"
    - "Extension info shows version 1.0.1 and key features"
  artifacts:
    - path: "fastcrm/frontend/src/routes/settings/profile/+page.svelte"
      provides: "Gmail Extension section with install instructions"
      contains: "Gmail Extension"
  key_links: []
---

<objective>
Add a "Gmail Extension" section to the Profile Settings page that displays information about the QuanticoCRM Gmail Extension and provides installation instructions.

Purpose: Allow users to discover and install the Gmail extension from within QuanticoCRM settings.
Output: Updated profile settings page with extension information section.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@fastcrm/frontend/src/routes/settings/profile/+page.svelte
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add Gmail Extension section to Profile Settings</name>
  <files>fastcrm/frontend/src/routes/settings/profile/+page.svelte</files>
  <action>
Add a new card section after the "Change Password" section (before the Platform Admin Badge) that displays:

1. **Header**: "Integrations" with subtitle "Connect QuanticoCRM with other tools"

2. **Gmail Extension Card** inside:
   - Icon: Gmail/email icon (SVG)
   - Title: "Quantico CRM for Gmail"
   - Version badge: "v1.0.1"
   - Description: "Access your Quantico CRM data directly in Gmail. Log emails, view contacts, and manage deals without leaving your inbox."

3. **Features list** (compact, inline):
   - Log emails to CRM
   - View contact info in Gmail
   - Create tasks from emails

4. **Installation section**:
   - Primary action: "Install from Chrome Web Store" button (styled as blue button, links to Chrome Web Store - use placeholder URL `https://chrome.google.com/webstore` for now)
   - Secondary text: "Or install manually: Download the extension files and load as unpacked extension in Chrome's developer mode"

5. **Styling**:
   - Match existing card styling (bg-white shadow rounded-lg overflow-hidden)
   - Use consistent px-6 py-4 spacing
   - Features as small gray text with bullet points or checkmarks
  </action>
  <verify>
Run frontend dev server and navigate to /settings/profile. Verify:
1. New "Integrations" section appears after Change Password
2. Gmail Extension card displays with all info
3. Install button is clickable and styled correctly
4. Layout matches other cards on the page
  </verify>
  <done>
Gmail Extension section visible on Profile Settings page with:
- Extension name, version, and description displayed
- Feature highlights visible
- Chrome Web Store install button present
- Manual installation instructions shown
  </done>
</task>

</tasks>

<verification>
- Profile Settings page loads without errors
- Integrations section renders with Gmail Extension info
- Install button links to Chrome Web Store
- Styling is consistent with existing page sections
</verification>

<success_criteria>
Users visiting /settings/profile can see the Gmail Extension section and understand how to install it.
</success_criteria>

<output>
After completion, create `.planning/quick/010-extension-setup-page/010-01-SUMMARY.md`
</output>
