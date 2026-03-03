---
phase: quick-34
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - frontend/src/routes/+layout.svelte
autonomous: true
requirements:
  - QUICK-34
must_haves:
  truths:
    - "Nav bar spans the full viewport width edge-to-edge"
    - "Logo is pinned to the far left with horizontal padding"
    - "Avatar and user menu are pinned to the far right with horizontal padding"
    - "Navigation tabs are centered between logo and right controls"
    - "Content area below nav fills full width with horizontal padding"
    - "No max-w constraint remains on nav or content"
  artifacts:
    - path: "frontend/src/routes/+layout.svelte"
      provides: "Full-width layout with centered nav tabs"
      contains: "justify-between"
  key_links: []
---

<objective>
Rework the layout header so the nav bar is full-width edge-to-edge. Logo pinned left, avatar/user menu pinned right, navigation tabs centered between them. Content area below also full width. Remove all max-w-[75%] constraints.

Purpose: Match the reference Quantix dashboard UI with a clean, professional full-width horizontal nav bar.
Output: Updated +layout.svelte with full-width nav and content.
</objective>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@frontend/src/routes/+layout.svelte
@frontend/src/app.css
</context>

<tasks>

<task type="auto">
  <name>Task 1: Make nav bar and content full-width with centered nav tabs</name>
  <files>frontend/src/routes/+layout.svelte</files>
  <action>
Modify `frontend/src/routes/+layout.svelte` with these specific changes:

**1. Nav bar inner container (line 192):**
Change:
```html
<div class="w-full max-w-[75%] mx-auto px-4 sm:px-6 lg:px-8">
```
To:
```html
<div class="w-full px-6 lg:px-8">
```
Remove `max-w-[75%]` and `mx-auto`. Use `px-6 lg:px-8` for generous horizontal padding.

**2. Restructure the nav flex layout (lines 193-326) to three sections:**

The current structure has two children of the `flex justify-between h-14` div:
- Left div: logo + nav tabs together
- Right div: org switcher + icons + user menu

Restructure into THREE sections for logo-left, tabs-center, controls-right:

```html
<div class="flex items-center justify-between h-14">
  <!-- Left: Logo -->
  <div class="flex items-center flex-shrink-0">
    <a href="/" class="flex-shrink-0 flex items-center">
      <img src="/logo.png" alt="Quantico CRM" class="h-10 w-auto" />
    </a>
  </div>

  <!-- Center: Navigation tabs -->
  <div class="flex items-center space-x-1">
    {#each getNavigationTabs() as tab (tab.id)}
      <a href={tab.href} class="px-3 py-2 text-sm font-medium rounded-md transition-colors {isActive(tab.href) ? 'bg-blue-50 text-blue-700' : 'text-gray-600 hover:text-gray-900 hover:bg-gray-50'}">
        {tab.label}
      </a>
    {/each}
  </div>

  <!-- Right: Org switcher + icons + user menu -->
  <div class="flex items-center space-x-2 flex-shrink-0">
    <!-- Keep ALL existing right-side controls exactly as they are -->
    <!-- Org Switcher, Edit Object icon, Setup icon, User Menu -->
  </div>
</div>
```

The key structural change: move nav tabs OUT of the left div into their own center div. The `justify-between` on the parent will push logo left, controls right, and tabs will sit in the center.

**3. Main content area (line 330):**
Change:
```html
<main class="w-full max-w-[75%] mx-auto px-4 sm:px-6 lg:px-8 py-6">
```
To:
```html
<main class="w-full px-6 lg:px-8 py-6">
```
Remove `max-w-[75%]` and `mx-auto`. Match the same horizontal padding as the nav bar.

**IMPORTANT: Do NOT change any functionality.** All event handlers, conditional rendering, user menu, org switcher, impersonation banner, auth logic, etc. must remain identical. This is purely a layout/CSS restructure.
  </action>
  <verify>
    <automated>cd /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend && npm run build 2>&1 | tail -5</automated>
  </verify>
  <done>
- Nav bar spans full viewport width with px-6/lg:px-8 padding (no max-w-[75%])
- Logo is in its own flex-shrink-0 container on the far left
- Nav tabs are in a centered container between logo and right controls
- Right-side controls (org switcher, setup icons, user menu) are in their own flex-shrink-0 container on the far right
- Content area is full width with matching padding (no max-w-[75%])
- Build succeeds with no errors
- All existing functionality preserved (menus, auth, navigation, impersonation banner)
  </done>
</task>

</tasks>

<verification>
- `npm run build` completes without errors
- No `max-w-[75%]` remains in +layout.svelte
- Nav bar has three-section flex layout (logo | tabs | controls)
- Visual: nav bar spans full width, logo far left, avatar far right, tabs centered
</verification>

<success_criteria>
- Full-width nav bar with logo left, centered tabs, controls right
- Full-width content area below
- Zero regressions in functionality
- Clean build
</success_criteria>

<output>
After completion, create `.planning/quick/34-rework-layout-header-full-width/34-SUMMARY.md`
</output>
