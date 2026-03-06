# KeyForge UI Redesign - Design Document

**Date**: 2026-03-06
**Status**: Approved
**Approach**: CSS Variables Rewrite (no build pipeline)

## Overview

Complete modern redesign of the KeyForge web UI with:
- Light and dark theme support
- Clean, minimal aesthetic (Linear/Apple inspired)
- Monochrome palette with emerald/teal accent
- System preference detection with manual override

## Design Principles

1. **Minimal & Clean**: Generous whitespace, subtle borders, refined typography
2. **Professional**: Cohesive color system, consistent spacing
3. **Accessible**: Good contrast ratios, clear focus states
4. **Simple Architecture**: No build pipeline, pure CSS + minimal JS for theme toggle

## Color System

### Light Theme

| Token | Value | Usage |
|-------|-------|-------|
| `--bg` | `#ffffff` | Page background |
| `--bg-subtle` | `#f9fafb` | Secondary backgrounds |
| `--surface` | `#ffffff` | Cards, modals |
| `--border` | `#e5e7eb` | Borders, dividers |
| `--text` | `#111827` | Primary text |
| `--text-muted` | `#6b7280` | Secondary text |
| `--accent` | `#10b981` | Primary actions (emerald-500) |
| `--accent-hover` | `#059669` | Hover state (emerald-600) |
| `--success` | `#10b981` | Success states |
| `--danger` | `#ef4444` | Destructive actions |
| `--danger-bg` | `#fef2f2` | Danger background |

### Dark Theme

| Token | Value | Usage |
|-------|-------|-------|
| `--bg` | `#0f172a` | Page background |
| `--bg-subtle` | `#020617` | Secondary backgrounds |
| `--surface` | `#1e293b` | Cards, modals |
| `--border` | `#334155` | Borders, dividers |
| `--text` | `#f1f5f9` | Primary text |
| `--text-muted` | `#94a3b8` | Secondary text |
| `--accent` | `#34d399` | Primary actions (emerald-400) |
| `--accent-hover` | `#10b981` | Hover state (emerald-500) |
| `--success` | `#34d399` | Success states |
| `--danger` | `#f87171` | Destructive actions |
| `--danger-bg` | `rgba(248,113,113,0.1)` | Danger background |

## Typography

- **Font Family**: Inter (Google Fonts)
- **Load**: `<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap">`
- **Base Size**: 16px (1rem)

### Scale

| Name | Size | Weight | Usage |
|------|------|--------|-------|
| xs | 0.75rem (12px) | 500 | Labels, badges |
| sm | 0.875rem (14px) | 400-500 | Table cells, small text |
| base | 1rem (16px) | 400 | Body text |
| lg | 1.125rem (18px) | 500 | Card titles |
| xl | 1.25rem (20px) | 600 | Page subtitles |
| 2xl | 1.5rem (24px) | 700 | Page titles |

## Design Tokens

```css
:root {
  /* Radii */
  --radius-sm: 6px;
  --radius-md: 10px;
  --radius-lg: 16px;
  --radius-full: 9999px;

  /* Shadows */
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.05);
  --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
  --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.1);

  /* Transitions */
  --transition-fast: 0.1s ease;
  --transition-base: 0.15s ease;
  --transition-slow: 0.2s ease;

  /* Spacing */
  --space-1: 0.25rem;
  --space-2: 0.5rem;
  --space-3: 0.75rem;
  --space-4: 1rem;
  --space-5: 1.25rem;
  --space-6: 1.5rem;
  --space-8: 2rem;
}
```

## Component Specifications

### Navigation Bar

- **Position**: Sticky top
- **Height**: 64px
- **Background**: Surface color with subtle blur (`backdrop-filter: blur(12px)`)
- **Border**: 1px bottom border
- **Layout**: Flexbox
  - Left: Logo (KeyForge)
  - Center: Nav links (Dashboard, Devices, Tokens, etc.)
  - Right: Theme toggle, Logout

**Mobile**:
- Hamburger menu
- Slide-out drawer from right
- Full-height overlay

### Theme Toggle

- **Style**: Icon button (sun/moon)
- **Position**: Navigation bar, right side
- **Behavior**:
  1. Check `prefers-color-scheme` on load
  2. Check `localStorage.getItem('theme')` (overrides system)
  3. Apply theme via `data-theme` attribute on `<html>`
  4. Toggle updates localStorage and DOM

```javascript
// Theme toggle logic
function initTheme() {
  const stored = localStorage.getItem('theme');
  const system = window.matchMedia('(prefers-color-scheme: dark)').matches;
  const theme = stored || (system ? 'dark' : 'light');
  document.documentElement.setAttribute('data-theme', theme);
}

function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  const next = current === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('theme', next);
}
```

### Buttons

| Variant | Light Theme | Dark Theme | Usage |
|---------|-------------|------------|-------|
| Primary | Emerald bg, white text | Emerald bg, white text | Main actions |
| Secondary | Ghost, border on hover | Ghost, border on hover | Secondary actions |
| Danger | Red bg, white text | Red bg, white text | Destructive |
| Ghost | Transparent, subtle hover | Transparent, subtle hover | Tertiary |

**Sizes**:
- `sm`: height 32px, padding 6px 12px
- `md`: height 40px, padding 8px 16px
- `lg`: height 48px, padding 12px 24px

### Cards

```css
.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg); /* 16px */
  padding: var(--space-6); /* 1.5rem */
  box-shadow: var(--shadow-sm);
}

.card:hover {
  box-shadow: var(--shadow-md);
}
```

### Tables

- **Style**: Clean, minimal borders
- **Header**: Sticky, muted background
- **Rows**: Hover highlight
- **Actions**: Icon buttons (pencil, ban, trash)

```css
table {
  width: 100%;
  border-collapse: collapse;
}

th {
  text-align: left;
  font-weight: 600;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

td {
  padding: 16px;
  border-bottom: 1px solid var(--border);
}

tr:last-child td {
  border-bottom: none;
}

tr:hover td {
  background: var(--bg-subtle);
}
```

### Form Inputs

```css
input[type="text"],
input[type="password"],
textarea,
select {
  width: 100%;
  padding: 10px 14px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  color: var(--text);
  font-size: 0.9375rem;
  transition: border-color var(--transition-fast),
              box-shadow var(--transition-fast);
}

input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.15);
}
```

### Badges

```css
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 10px;
  border-radius: var(--radius-full);
  font-size: 0.75rem;
  font-weight: 500;
}

.badge.active {
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent);
}

.badge.revoked {
  background: var(--danger-bg);
  color: var(--danger);
}
```

## Page Layouts

### Base Layout

```
┌─────────────────────────────────────────────────────────┐
│ [Logo]     [Dashboard] [Devices] [Tokens]    [☀️] [Logout]│
├─────────────────────────────────────────────────────────┤
│                                                         │
│   ┌─────────────────────────────────────────────────┐   │
│   │                  Page Content                   │   │
│   │              (max-width: 1200px)                │   │
│   └─────────────────────────────────────────────────┘   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Dashboard

```
┌─────────────────────────────────────────────────────────┐
│ Dashboard                                               │
├─────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │
│  │   12    │  │   10    │  │    2    │  │    5    │    │
│  │ Total   │  │ Active  │  │ Revoked │  │ SSH     │    │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘    │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Recent Activity                                  │   │
│  │ ─────────────────────────────────────────────── │   │
│  │ 2 min ago    [ENROLLED]    pixel-8-pro           │   │
│  │ 1 hour ago   [REVOKED]     old-laptop            │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### Login Page

- Centered card (max 400px width)
- Subtle gradient background
- Lock icon at top
- Single password input
- Full-width login button

## Files to Modify

| File | Changes |
|------|---------|
| `internal/web/static/style.css` | Complete rewrite with new design system |
| `internal/web/templates/layout.html` | Add Inter font, theme toggle, nav redesign |
| `internal/web/templates/*.html` | Minor class updates for new components |
| `internal/web/static/theme.js` | New file for theme toggle logic |

## Implementation Notes

1. **CSS Variables**: All colors defined in `:root` with overrides in `[data-theme="dark"]`
2. **Theme Script**: Inline `<script>` in layout head to prevent flash
3. **Icons**: Use inline SVGs for sun/moon, action icons
4. **No Dependencies**: Pure CSS, no Tailwind or other frameworks
5. **Backward Compatible**: Existing htmx interactions unchanged

## Success Criteria

- [ ] Light theme looks clean and professional
- [ ] Dark theme maintains good contrast
- [ ] Theme toggle works instantly without flicker
- [ ] Mobile responsive (works on phone browsers)
- [ ] All existing functionality preserved
- [ ] Page load time not significantly increased
