---
phase: quick
plan: 009
type: execute
wave: 1
depends_on: []
files_modified:
  - /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/tailwind.config.js
  - /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/src/app.css
  - /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/src/app.html
autonomous: true

must_haves:
  truths:
    - "New fonts render correctly across the UI"
    - "Orange/silver color scheme is visible"
    - "Dark mode works via class toggle"
    - "Rollback to original styling takes < 1 minute"
  artifacts:
    - path: "frontend/tailwind.config.js"
      provides: "Extended theme with custom colors and fonts"
    - path: "frontend/src/app.css"
      provides: "Font imports and CSS variable definitions"
    - path: "frontend/src/app.html"
      provides: "Google Fonts link tags"
  key_links:
    - from: "app.html"
      to: "Google Fonts CDN"
      via: "link preconnect + stylesheet"
    - from: "tailwind.config.js"
      to: "app.css"
      via: "font-family references"
---

<objective>
Apply experimental styling with new fonts (Space Grotesk, Syncopate, JetBrains Mono) and orange/silver color scheme.

Purpose: Refresh the CRM visual identity with a modern, distinctive look while maintaining easy rollback capability.
Output: Updated styling that can be reverted by restoring 3 files from git.
</objective>

<rollback_strategy>
**EASY ROLLBACK:** All changes are in 3 files. To revert:
```bash
git checkout HEAD~1 -- frontend/tailwind.config.js frontend/src/app.css frontend/src/app.html
```

**Before starting:** The executor should note the current commit hash for reference:
```bash
git rev-parse HEAD  # Save this hash for rollback
```
</rollback_strategy>

<execution_context>
@/Users/ahmedhamid/.claude/get-shit-done/workflows/execute-plan.md
@/Users/ahmedhamid/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
Frontend path: /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend

Current files:
- tailwind.config.js: Minimal config with no customizations
- src/app.css: Just Tailwind directives + system font stack
- src/app.html: Basic HTML template, no font links
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add Google Fonts and Update Tailwind Config</name>
  <files>
    /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/src/app.html
    /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/tailwind.config.js
  </files>
  <action>
1. Update `src/app.html` to load Google Fonts in the head:
   - Add preconnect links for fonts.googleapis.com and fonts.gstatic.com
   - Add stylesheet link for: Space Grotesk (400,500,600,700), Syncopate (400,700), JetBrains Mono (400,500,600)
   - URL: https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&family=Space+Grotesk:wght@400;500;600;700&family=Syncopate:wght@400;700&display=swap

2. Update `tailwind.config.js` to extend theme with:

   Colors:
   - primary: '#FF9145' (princeton orange)
   - 'grey-olive': '#7F898E'
   - silver: '#C4CCC9'
   - 'silver-2': '#C1CAC8'
   - 'bg-light': '#F2F4F3'
   - 'bg-dark': '#0A0B0B'

   Font families:
   - sans: ['Space Grotesk', ...defaultTheme.fontFamily.sans]
   - display: ['Syncopate', 'Space Grotesk', 'sans-serif']
   - mono: ['JetBrains Mono', ...defaultTheme.fontFamily.mono]

   Border radius:
   - DEFAULT: '4px'
   - xl: '24px'

   Enable dark mode via class: darkMode: 'class'

   Import defaultTheme from 'tailwindcss/defaultTheme' at top of config.
  </action>
  <verify>
    - `cat frontend/src/app.html` shows Google Fonts link
    - `cat frontend/tailwind.config.js` shows extended colors, fonts, border radius
    - No syntax errors: `cd frontend && npm run build 2>&1 | head -20`
  </verify>
  <done>Tailwind config extended with custom theme, fonts loading from Google Fonts CDN</done>
</task>

<task type="auto">
  <name>Task 2: Update App CSS with Theme Variables and Base Styles</name>
  <files>
    /Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/frontend/src/app.css
  </files>
  <action>
Update `src/app.css` to use the new fonts and define CSS custom properties for the color scheme.

Replace existing content with:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

/* ===========================================
   EXPERIMENTAL THEME: Orange/Silver
   To revert: git checkout HEAD~1 -- src/app.css
   =========================================== */

@layer base {
  :root {
    /* Primary palette */
    --color-primary: #FF9145;
    --color-primary-hover: #E67D30;
    --color-grey-olive: #7F898E;
    --color-silver: #C4CCC9;
    --color-silver-2: #C1CAC8;

    /* Backgrounds */
    --color-bg: #F2F4F3;
    --color-bg-alt: #FFFFFF;
    --color-text: #0A0B0B;
    --color-text-muted: #7F898E;
  }

  .dark {
    --color-bg: #0A0B0B;
    --color-bg-alt: #1A1B1B;
    --color-text: #F2F4F3;
    --color-text-muted: #C4CCC9;
  }

  body {
    font-family: 'Space Grotesk', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background-color: var(--color-bg);
    color: var(--color-text);
  }

  /* Display headings use Syncopate */
  h1, h2, .font-display {
    font-family: 'Syncopate', 'Space Grotesk', sans-serif;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  /* Code blocks use JetBrains Mono */
  code, pre, .font-mono {
    font-family: 'JetBrains Mono', ui-monospace, monospace;
  }
}

/* Utility class for primary accent */
@layer utilities {
  .text-primary {
    color: var(--color-primary);
  }

  .bg-primary {
    background-color: var(--color-primary);
  }

  .bg-primary:hover {
    background-color: var(--color-primary-hover);
  }

  .border-primary {
    border-color: var(--color-primary);
  }
}
```

Note: Keep the comment at top reminding how to revert.
  </action>
  <verify>
    - `cat frontend/src/app.css` shows CSS variables and font declarations
    - Frontend builds without errors: `cd frontend && npm run build`
  </verify>
  <done>CSS variables defined, body/heading/code fonts applied, dark mode variables ready</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <what-built>
    New experimental styling with:
    - Space Grotesk as body font
    - Syncopate as display font (h1, h2)
    - JetBrains Mono for code
    - Orange (#FF9145) primary color
    - Silver/grey neutral palette
    - Dark mode support (add class="dark" to html)
  </what-built>
  <how-to-verify>
    1. Start frontend dev server: `cd frontend && npm run dev`
    2. Open http://localhost:5173 in browser
    3. Verify fonts:
       - Body text should be Space Grotesk (cleaner, geometric sans-serif)
       - Page titles (h1, h2) should be Syncopate (distinctive uppercase display font)
    4. Check if the styling looks good to you
    5. Optional: Test dark mode by opening DevTools, adding class="dark" to <html> element

    **To rollback if you don't like it:**
    ```bash
    git checkout HEAD~1 -- frontend/tailwind.config.js frontend/src/app.css frontend/src/app.html
    ```
  </how-to-verify>
  <resume-signal>Type "approved" to keep the new styling, or "rollback" to revert</resume-signal>
</task>

</tasks>

<verification>
- [ ] Google Fonts loading in app.html
- [ ] Tailwind config has custom colors, fonts, border radius
- [ ] app.css has CSS variables and font declarations
- [ ] Frontend builds without errors
- [ ] User has verified visual appearance in browser
</verification>

<success_criteria>
- New fonts visibly different from system defaults
- Colors defined and available as Tailwind utilities
- Dark mode variables in place
- User approves the visual appearance OR rollback performed
</success_criteria>

<output>
After completion, create `.planning/quick/009-experimental-styling-fonts-colors/009-SUMMARY.md`
</output>
