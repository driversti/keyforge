# UI Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete modern redesign of KeyForge web UI with light/dark theme support, clean minimal aesthetic, and emerald accent color.

**Architecture:** Pure CSS rewrite using CSS custom properties for theming. Single `style.css` file with `[data-theme]` selectors. Minimal JavaScript for theme toggle with localStorage persistence. No build pipeline or external dependencies.

**Tech Stack:** Go + htmx (unchanged), pure CSS with CSS variables, vanilla JS for theme toggle

---

## Task 1: Create Theme Toggle JavaScript

**Files:**
- Create: `internal/web/static/theme.js`

**Step 1: Create the theme toggle script**

```javascript
// Theme management for KeyForge
(function() {
    const STORAGE_KEY = 'keyforge-theme';

    function getSystemTheme() {
        return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }

    function getStoredTheme() {
        return localStorage.getItem(STORAGE_KEY);
    }

    function applyTheme(theme) {
        document.documentElement.setAttribute('data-theme', theme);
        updateToggleIcon(theme);
    }

    function updateToggleIcon(theme) {
        const sunIcon = document.getElementById('sun-icon');
        const moonIcon = document.getElementById('moon-icon');
        if (sunIcon && moonIcon) {
            sunIcon.style.display = theme === 'dark' ? 'block' : 'none';
            moonIcon.style.display = theme === 'light' ? 'block' : 'none';
        }
    }

    function toggleTheme() {
        const current = document.documentElement.getAttribute('data-theme') || getSystemTheme();
        const next = current === 'dark' ? 'light' : 'dark';
        localStorage.setItem(STORAGE_KEY, next);
        applyTheme(next);
    }

    // Initialize on load - run immediately to prevent flash
    const stored = getStoredTheme();
    const system = getSystemTheme();
    const theme = stored || system;
    applyTheme(theme);

    // Listen for system theme changes
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function(e) {
        if (!getStoredTheme()) {
            applyTheme(e.matches ? 'dark' : 'light');
        }
    });

    // Expose toggle function globally
    window.toggleTheme = toggleTheme;
})();
```

**Step 2: Commit**

```bash
git add internal/web/static/theme.js
git commit -m "feat(ui): add theme toggle script with system preference detection"
```

---

## Task 2: Rewrite CSS with Design System

**Files:**
- Modify: `internal/web/static/style.css` (complete rewrite)

**Step 1: Write the new CSS foundation (variables and base styles)**

Replace entire `style.css` content with:

```css
/* KeyForge Design System - Light/Dark Theme */

/* ============================================
   CSS VARIABLES & THEME DEFINITIONS
   ============================================ */

:root {
    /* Typography */
    --font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;

    /* Spacing */
    --space-1: 0.25rem;
    --space-2: 0.5rem;
    --space-3: 0.75rem;
    --space-4: 1rem;
    --space-5: 1.25rem;
    --space-6: 1.5rem;
    --space-8: 2rem;
    --space-10: 2.5rem;
    --space-12: 3rem;

    /* Radii */
    --radius-sm: 6px;
    --radius-md: 10px;
    --radius-lg: 16px;
    --radius-full: 9999px;

    /* Transitions */
    --transition-fast: 0.1s ease;
    --transition-base: 0.15s ease;
    --transition-slow: 0.2s ease;

    /* Shadows */
    --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.04);
    --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.06), 0 2px 4px -2px rgba(0, 0, 0, 0.05);
    --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.08), 0 4px 6px -4px rgba(0, 0, 0, 0.04);

    /* Light theme (default) */
    --bg: #ffffff;
    --bg-subtle: #f9fafb;
    --surface: #ffffff;
    --surface-hover: #f3f4f6;
    --border: #e5e7eb;
    --border-subtle: #f3f4f6;
    --text: #111827;
    --text-muted: #6b7280;
    --text-subtle: #9ca3af;
    --accent: #10b981;
    --accent-hover: #059669;
    --accent-bg: rgba(16, 185, 129, 0.1);
    --success: #10b981;
    --success-bg: rgba(16, 185, 129, 0.1);
    --danger: #ef4444;
    --danger-hover: #dc2626;
    --danger-bg: #fef2f2;
    --warning: #f59e0b;
    --warning-bg: #fffbeb;
    --info: #3b82f6;
    --info-bg: #eff6ff;
}

/* Dark theme */
[data-theme="dark"] {
    --bg: #0f172a;
    --bg-subtle: #020617;
    --surface: #1e293b;
    --surface-hover: #334155;
    --border: #334155;
    --border-subtle: #1e293b;
    --text: #f1f5f9;
    --text-muted: #94a3b8;
    --text-subtle: #64748b;
    --accent: #34d399;
    --accent-hover: #10b981;
    --accent-bg: rgba(52, 211, 153, 0.15);
    --success: #34d399;
    --success-bg: rgba(52, 211, 153, 0.15);
    --danger: #f87171;
    --danger-hover: #ef4444;
    --danger-bg: rgba(248, 113, 113, 0.1);
    --warning: #fbbf24;
    --warning-bg: rgba(251, 191, 36, 0.1);
    --info: #60a5fa;
    --info-bg: rgba(96, 165, 250, 0.1);
    --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.2);
    --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -2px rgba(0, 0, 0, 0.2);
    --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.4), 0 4px 6px -4px rgba(0, 0, 0, 0.3);
}

/* ============================================
   BASE STYLES
   ============================================ */

*, *::before, *::after {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

html {
    font-size: 16px;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
}

body {
    font-family: var(--font-family);
    background: var(--bg);
    color: var(--text);
    line-height: 1.6;
    min-height: 100vh;
}

/* ============================================
   LAYOUT
   ============================================ */

.container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 var(--space-6);
}

/* ============================================
   NAVIGATION
   ============================================ */

.nav {
    position: sticky;
    top: 0;
    z-index: 100;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
}

.nav-inner {
    display: flex;
    align-items: center;
    gap: var(--space-6);
    height: 64px;
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 var(--space-6);
}

.nav-logo {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--accent);
    text-decoration: none;
    margin-right: var(--space-4);
}

.nav-logo:hover {
    color: var(--accent-hover);
}

.nav-links {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex: 1;
}

.nav-link {
    color: var(--text-muted);
    text-decoration: none;
    font-size: 0.875rem;
    font-weight: 500;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-md);
    transition: all var(--transition-fast);
}

.nav-link:hover {
    color: var(--text);
    background: var(--surface-hover);
}

.nav-actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
}

/* Theme toggle button */
.theme-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    cursor: pointer;
    color: var(--text-muted);
    transition: all var(--transition-fast);
}

.theme-toggle:hover {
    background: var(--surface-hover);
    color: var(--text);
    border-color: var(--border);
}

.theme-toggle svg {
    width: 18px;
    height: 18px;
}

/* Mobile nav toggle */
.nav-toggle {
    display: none;
    width: 40px;
    height: 40px;
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--text);
    padding: var(--space-2);
}

.nav-toggle span {
    display: block;
    width: 20px;
    height: 2px;
    background: currentColor;
    margin: 5px 0;
    transition: all var(--transition-fast);
}

/* ============================================
   TYPOGRAPHY
   ============================================ */

h1, h2, h3, h4 {
    font-weight: 600;
    line-height: 1.3;
    color: var(--text);
}

h1 {
    font-size: 1.5rem;
    font-weight: 700;
}

h2 {
    font-size: 1.125rem;
    font-weight: 600;
}

h3 {
    font-size: 1rem;
    font-weight: 600;
}

p {
    color: var(--text-muted);
}

a {
    color: var(--accent);
    text-decoration: none;
    transition: color var(--transition-fast);
}

a:hover {
    color: var(--accent-hover);
}

/* ============================================
   BUTTONS
   ============================================ */

.btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    font-family: inherit;
    font-size: 0.875rem;
    font-weight: 500;
    line-height: 1.4;
    border: none;
    border-radius: var(--radius-md);
    cursor: pointer;
    text-decoration: none;
    transition: all var(--transition-fast);
}

.btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.btn-primary {
    background: var(--accent);
    color: #ffffff;
}

.btn-primary:hover:not(:disabled) {
    background: var(--accent-hover);
}

.btn-secondary {
    background: var(--surface-hover);
    color: var(--text);
    border: 1px solid var(--border);
}

.btn-secondary:hover:not(:disabled) {
    background: var(--border);
}

.btn-danger {
    background: var(--danger);
    color: #ffffff;
}

.btn-danger:hover:not(:disabled) {
    background: var(--danger-hover);
}

.btn-ghost {
    background: transparent;
    color: var(--text-muted);
}

.btn-ghost:hover:not(:disabled) {
    background: var(--surface-hover);
    color: var(--text);
}

.btn-outline {
    background: transparent;
    color: var(--text-muted);
    border: 1px solid var(--border);
}

.btn-outline:hover:not(:disabled) {
    background: var(--surface-hover);
    color: var(--text);
    border-color: var(--text-muted);
}

/* Button sizes */
.btn-sm {
    padding: var(--space-1) var(--space-3);
    font-size: 0.8125rem;
}

.btn-lg {
    padding: var(--space-3) var(--space-6);
    font-size: 1rem;
}

/* Icon button */
.btn-icon {
    padding: var(--space-2);
    width: 36px;
    height: 36px;
}

.btn-icon svg {
    width: 18px;
    height: 18px;
}

/* ============================================
   CARDS
   ============================================ */

.card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: var(--space-6);
    box-shadow: var(--shadow-sm);
}

.card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-4);
}

.card-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text);
}

/* ============================================
   STATS GRID
   ============================================ */

.stats-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: var(--space-4);
    margin-bottom: var(--space-6);
}

.stat-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: var(--space-5);
    text-align: center;
}

.stat-number {
    font-size: 2rem;
    font-weight: 700;
    color: var(--accent);
    line-height: 1.2;
}

.stat-number.stat-success {
    color: var(--success);
}

.stat-number.stat-danger {
    color: var(--danger);
}

.stat-label {
    font-size: 0.875rem;
    color: var(--text-muted);
    margin-top: var(--space-1);
}

/* ============================================
   TABLES
   ============================================ */

.table-wrapper {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    overflow: hidden;
}

table {
    width: 100%;
    border-collapse: collapse;
}

th {
    text-align: left;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    padding: var(--space-3) var(--space-4);
    background: var(--bg-subtle);
    border-bottom: 1px solid var(--border);
}

td {
    padding: var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
    color: var(--text);
    font-size: 0.875rem;
}

tr:last-child td {
    border-bottom: none;
}

tr:hover td {
    background: var(--bg-subtle);
}

/* ============================================
   FORMS
   ============================================ */

.form-group {
    margin-bottom: var(--space-5);
}

.form-label {
    display: block;
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text);
    margin-bottom: var(--space-2);
}

.form-input,
.form-textarea,
.form-select {
    width: 100%;
    padding: var(--space-3) var(--space-4);
    font-family: inherit;
    font-size: 0.9375rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}

.form-input:focus,
.form-textarea:focus,
.form-select:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-bg);
}

.form-input::placeholder,
.form-textarea::placeholder {
    color: var(--text-subtle);
}

.form-textarea {
    min-height: 120px;
    resize: vertical;
    font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
}

.form-checkbox {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    cursor: pointer;
}

.form-checkbox input[type="checkbox"] {
    width: 18px;
    height: 18px;
    accent-color: var(--accent);
    cursor: pointer;
}

/* Inline form */
.inline-form {
    display: flex;
    gap: var(--space-4);
    align-items: flex-end;
    flex-wrap: wrap;
}

.inline-form .form-group {
    margin-bottom: 0;
}

/* ============================================
   BADGES & TAGS
   ============================================ */

.badge {
    display: inline-flex;
    align-items: center;
    padding: var(--space-1) var(--space-3);
    font-size: 0.75rem;
    font-weight: 500;
    border-radius: var(--radius-full);
    text-transform: capitalize;
}

.badge-active,
.badge.active {
    background: var(--success-bg);
    color: var(--success);
}

.badge-revoked,
.badge.revoked {
    background: var(--danger-bg);
    color: var(--danger);
}

.tags {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
}

.tag {
    display: inline-flex;
    padding: var(--space-1) var(--space-2);
    font-size: 0.75rem;
    font-weight: 500;
    background: var(--accent-bg);
    color: var(--accent);
    border-radius: var(--radius-sm);
}

/* ============================================
   FLASH MESSAGES
   ============================================ */

.flash {
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    font-size: 0.875rem;
    margin-bottom: var(--space-4);
}

.flash-success {
    background: var(--success-bg);
    color: var(--success);
    border: 1px solid var(--success);
}

.flash-error {
    background: var(--danger-bg);
    color: var(--danger);
    border: 1px solid var(--danger);
}

.flash-info {
    background: var(--info-bg);
    color: var(--info);
    border: 1px solid var(--info);
}

.flash code {
    display: block;
    margin-top: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-family: 'SF Mono', Monaco, monospace;
    font-size: 0.8125rem;
    color: var(--text);
    word-break: break-all;
}

/* ============================================
   TOP BAR
   ============================================ */

.top-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-6);
    padding: var(--space-4) 0;
}

.top-bar h1 {
    margin: 0;
}

/* ============================================
   FILTER BAR
   ============================================ */

.filter-bar {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    margin-bottom: var(--space-4);
    flex-wrap: wrap;
}

.filter-input {
    flex: 1;
    min-width: 200px;
    padding: var(--space-2) var(--space-3);
    font-size: 0.875rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
}

.filter-input:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-bg);
}

.filter-select {
    padding: var(--space-2) var(--space-3);
    font-size: 0.875rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    cursor: pointer;
}

/* ============================================
   ACTIONS
   ============================================ */

.actions {
    display: flex;
    gap: var(--space-2);
    align-items: center;
}

.actions form {
    display: inline;
}

/* ============================================
   KEYS OUTPUT
   ============================================ */

.keys-output {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-4);
    font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
    font-size: 0.8125rem;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 400px;
    overflow-y: auto;
    line-height: 1.8;
    color: var(--success);
}

/* ============================================
   INSTALL COMMAND
   ============================================ */

.install-cmd {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-4);
}

.install-cmd code {
    flex: 1;
    font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
    font-size: 0.875rem;
    white-space: pre-wrap;
    word-break: break-all;
    line-height: 1.6;
    color: var(--success);
}

/* ============================================
   COPY BUTTON
   ============================================ */

.copy-btn {
    background: var(--accent);
    color: #ffffff;
    border: none;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-md);
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    transition: background var(--transition-fast);
}

.copy-btn:hover {
    background: var(--accent-hover);
}

/* ============================================
   FINGERPRINT
   ============================================ */

.fingerprint {
    font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
    font-size: 0.8125rem;
    color: var(--text-muted);
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

/* ============================================
   DOWNLOAD GRID
   ============================================ */

.download-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-4);
    margin-bottom: var(--space-8);
}

.download-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
}

.download-os {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--accent);
}

.download-arch {
    font-size: 0.9375rem;
    color: var(--text);
}

.download-desc {
    font-size: 0.8125rem;
    color: var(--text-muted);
    margin-bottom: var(--space-2);
}

/* ============================================
   LOGIN PAGE
   ============================================ */

.login-wrapper {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: calc(100vh - 200px);
    padding: var(--space-6);
}

.login-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: var(--space-8);
    width: 100%;
    max-width: 400px;
    box-shadow: var(--shadow-lg);
}

.login-header {
    text-align: center;
    margin-bottom: var(--space-8);
}

.login-icon {
    width: 48px;
    height: 48px;
    color: var(--accent);
    margin-bottom: var(--space-4);
}

.login-header h1 {
    font-size: 1.5rem;
    font-weight: 600;
    margin-bottom: var(--space-2);
}

.login-header p {
    font-size: 0.9375rem;
    color: var(--text-muted);
}

.login-btn {
    width: 100%;
    padding: var(--space-3) var(--space-4);
    font-size: 0.9375rem;
    margin-top: var(--space-2);
}

/* ============================================
   TOKEN REVEAL
   ============================================ */

.token-reveal {
    margin-top: var(--space-2);
    display: flex;
    align-items: center;
    gap: var(--space-2);
}

.token-reveal code {
    flex: 1;
}

/* ============================================
   PAGINATION
   ============================================ */

.pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-4);
    margin-top: var(--space-6);
    padding: var(--space-4) 0;
}

.page-info {
    font-size: 0.875rem;
    color: var(--text-muted);
}

/* ============================================
   EMPTY STATE
   ============================================ */

.empty-state {
    text-align: center;
    color: var(--text-muted);
    padding: var(--space-8);
}

/* ============================================
   RESPONSIVE DESIGN
   ============================================ */

@media (max-width: 1024px) {
    .stats-grid {
        grid-template-columns: repeat(2, 1fr);
    }

    .download-grid {
        grid-template-columns: repeat(2, 1fr);
    }
}

@media (max-width: 768px) {
    .nav-inner {
        height: 56px;
    }

    .nav-links {
        position: fixed;
        top: 56px;
        left: 0;
        right: 0;
        bottom: 0;
        flex-direction: column;
        align-items: stretch;
        gap: 0;
        padding: var(--space-4);
        background: var(--surface);
        border-top: 1px solid var(--border);
        transform: translateX(100%);
        transition: transform var(--transition-slow);
        overflow-y: auto;
    }

    .nav-links.open {
        transform: translateX(0);
    }

    .nav-link {
        padding: var(--space-3) var(--space-4);
        border-radius: var(--radius-md);
    }

    .nav-toggle {
        display: flex;
        flex-direction: column;
        justify-content: center;
        margin-left: auto;
    }

    .nav-actions {
        gap: var(--space-2);
    }

    .top-bar {
        flex-direction: column;
        align-items: flex-start;
        gap: var(--space-3);
    }

    .filter-bar {
        flex-direction: column;
        align-items: stretch;
    }

    .filter-input {
        min-width: 100%;
    }

    table {
        display: block;
        overflow-x: auto;
    }

    .actions {
        flex-direction: column;
        align-items: flex-start;
    }

    .fingerprint {
        max-width: 120px;
    }

    .card {
        padding: var(--space-4);
    }

    .download-grid {
        grid-template-columns: 1fr;
    }

    .install-cmd {
        flex-direction: column;
    }
}

@media (max-width: 480px) {
    .container {
        padding: 0 var(--space-4);
    }

    .stats-grid {
        gap: var(--space-3);
    }

    .stat-card {
        padding: var(--space-4);
    }

    .stat-number {
        font-size: 1.5rem;
    }

    h1 {
        font-size: 1.25rem;
    }

    .login-card {
        padding: var(--space-6);
    }
}
```

**Step 2: Commit**

```bash
git add internal/web/static/style.css
git commit -m "feat(ui): rewrite CSS with modern design system and light/dark themes"
```

---

## Task 3: Update Layout Template

**Files:**
- Modify: `internal/web/templates/layout.html`

**Step 1: Update layout with Inter font, theme toggle, and modern nav**

Replace entire `layout.html` content with:

```html
{{define "layout"}}
<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>KeyForge — {{template "title" .}}</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="/static/style.css">
    <script src="/static/theme.js"></script>
    <script src="/static/htmx.min.js"></script>
</head>
<body>
    <nav class="nav">
        <div class="nav-inner">
            <a href="/" class="nav-logo">KeyForge</a>

            <button class="nav-toggle" onclick="document.querySelector('.nav-links').classList.toggle('open')">
                <span></span>
                <span></span>
                <span></span>
            </button>

            <div class="nav-links">
                <a href="/" class="nav-link">Dashboard</a>
                <a href="/devices" class="nav-link">Devices</a>
                <a href="/add" class="nav-link">Add Device</a>
                <a href="/authorized-keys" class="nav-link">Keys</a>
                <a href="/tokens" class="nav-link">Tokens</a>
                <a href="/audit" class="nav-link">Audit</a>
                <a href="/settings" class="nav-link">Settings</a>
                <a href="/download" class="nav-link">Download</a>
            </div>

            <div class="nav-actions">
                <button class="theme-toggle" onclick="toggleTheme()" title="Toggle theme">
                    <svg id="sun-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <circle cx="12" cy="12" r="5"/>
                        <line x1="12" y1="1" x2="12" y2="3"/>
                        <line x1="12" y1="21" x2="12" y2="23"/>
                        <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
                        <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
                        <line x1="1" y1="12" x2="3" y2="12"/>
                        <line x1="21" y1="12" x2="23" y2="12"/>
                        <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
                        <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
                    </svg>
                    <svg id="moon-icon" style="display:none" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
                    </svg>
                </button>
                <a href="/logout" class="btn btn-ghost btn-sm">Logout</a>
            </div>
        </div>
    </nav>

    <main class="container" style="padding-top: var(--space-6); padding-bottom: var(--space-8);">
        {{template "content" .}}
    </main>

    <script>
        // Close mobile nav when clicking outside
        document.addEventListener('click', function(e) {
            const navLinks = document.querySelector('.nav-links');
            const navToggle = document.querySelector('.nav-toggle');
            if (navLinks && !navLinks.contains(e.target) && !navToggle.contains(e.target)) {
                navLinks.classList.remove('open');
            }
        });
    </script>
</body>
</html>
{{end}}
```

**Step 2: Commit**

```bash
git add internal/web/templates/layout.html
git commit -m "feat(ui): update layout with Inter font and theme toggle"
```

---

## Task 4: Update Dashboard Template

**Files:**
- Modify: `internal/web/templates/dashboard.html`

**Step 1: Update dashboard with new classes**

Replace entire `dashboard.html` content with:

```html
{{define "title"}}Dashboard{{end}}
{{define "content"}}
<div class="top-bar">
    <h1>Dashboard</h1>
</div>

<div class="stats-grid">
    <div class="stat-card">
        <div class="stat-number">{{.TotalDevices}}</div>
        <div class="stat-label">Total Devices</div>
    </div>
    <div class="stat-card">
        <div class="stat-number stat-success">{{.ActiveDevices}}</div>
        <div class="stat-label">Active</div>
    </div>
    <div class="stat-card">
        <div class="stat-number stat-danger">{{.RevokedDevices}}</div>
        <div class="stat-label">Revoked</div>
    </div>
    <div class="stat-card">
        <div class="stat-number">{{.SSHAccepting}}</div>
        <div class="stat-label">SSH Accepting</div>
    </div>
</div>

<div class="card">
    <div class="card-header">
        <h2 class="card-title">Recent Activity</h2>
    </div>
    {{if .RecentActivity}}
    <div class="table-wrapper">
        <table>
            <thead>
                <tr>
                    <th>Time</th>
                    <th>Action</th>
                    <th>Details</th>
                </tr>
            </thead>
            <tbody>
                {{range .RecentActivity}}
                <tr>
                    <td style="white-space:nowrap">{{formatDate .CreatedAt}}</td>
                    <td><span class="badge {{actionBadgeClass .Action}}">{{.Action}}</span></td>
                    <td>{{.Details}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    <div style="text-align:center;margin-top:var(--space-4)">
        <a href="/audit" class="btn btn-ghost btn-sm">View Full Audit Log</a>
    </div>
    {{else}}
    <p style="color:var(--text-muted)">No activity yet.</p>
    {{end}}
</div>
{{end}}
```

**Step 2: Commit**

```bash
git add internal/web/templates/dashboard.html
git commit -m "feat(ui): update dashboard with new design classes"
```

---

## Task 5: Update Devices Template

**Files:**
- Modify: `internal/web/templates/devices.html`

**Step 1: Update devices page with new classes**

Replace entire `devices.html` content with:

```html
{{define "title"}}Devices{{end}}

{{define "content"}}
<div class="top-bar">
    <h1>Devices</h1>
    <a href="/add" class="btn btn-primary">Add Device</a>
</div>

{{if .Flash}}
<div class="flash flash-success">{{.Flash}}</div>
{{end}}

<form method="GET" action="/devices" class="filter-bar">
    <input type="text" name="q" value="{{.Query}}" placeholder="Search by name, fingerprint, or tag..." class="filter-input">
    <select name="status" class="filter-select">
        <option value="">All Statuses</option>
        <option value="active" {{if eq .StatusFilter "active"}}selected{{end}}>Active</option>
        <option value="revoked" {{if eq .StatusFilter "revoked"}}selected{{end}}>Revoked</option>
    </select>
    <button type="submit" class="btn btn-secondary btn-sm">Filter</button>
    {{if or .Query .StatusFilter}}
    <a href="/devices" class="btn btn-ghost btn-sm">Clear</a>
    {{end}}
</form>

<div class="table-wrapper">
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Status</th>
                <th>SSH</th>
                <th>Fingerprint</th>
                <th>Tags</th>
                <th>Registered</th>
                <th>Actions</th>
            </tr>
        </thead>
        <tbody>
            {{if .Devices}}
            {{range .Devices}}
            <tr>
                <td><strong>{{.Name}}</strong></td>
                <td><span class="badge {{if eq (print .Status) "active"}}badge-active{{else}}badge-revoked{{end}}">{{.Status}}</span></td>
                <td>{{if .AcceptsSSH}}<span style="color:var(--success)">Yes</span>{{else}}<span style="color:var(--text-muted)">No</span>{{end}}</td>
                <td><span class="fingerprint" title="{{.Fingerprint}}">{{truncate .Fingerprint 24}}</span></td>
                <td>
                    <div class="tags">
                        {{range .Tags}}<span class="tag">{{.}}</span>{{end}}
                    </div>
                </td>
                <td>{{formatDate .RegisteredAt}}</td>
                <td>
                    <div class="actions">
                        <a href="/devices/{{.ID}}/edit" class="btn btn-ghost btn-sm">Edit</a>
                        {{if eq (print .Status) "active"}}
                        <form method="POST" action="/devices/{{.ID}}/revoke">
                            <button type="submit" class="btn btn-ghost btn-sm">Revoke</button>
                        </form>
                        {{else}}
                        <form method="POST" action="/devices/{{.ID}}/reactivate">
                            <button type="submit" class="btn btn-ghost btn-sm">Reactivate</button>
                        </form>
                        {{end}}
                        <form method="POST" action="/devices/{{.ID}}/delete" onsubmit="return confirm('Delete this device?')">
                            <button type="submit" class="btn btn-danger btn-sm">Delete</button>
                        </form>
                    </div>
                </td>
            </tr>
            {{end}}
            {{else}}
            <tr>
                <td colspan="7" class="empty-state">No devices registered yet.</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{end}}
```

**Step 2: Commit**

```bash
git add internal/web/templates/devices.html
git commit -m "feat(ui): update devices page with new design classes"
```

---

## Task 6: Update Login Template

**Files:**
- Modify: `internal/web/templates/login.html`

**Step 1: Update login page with new design**

Replace entire `login.html` content with:

```html
{{define "title"}}Login{{end}}

{{define "content"}}
<div class="login-wrapper">
    <div class="login-card">
        <div class="login-header">
            <svg class="login-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
                <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
            </svg>
            <h1>Welcome back</h1>
            <p>Enter your password or API key</p>
        </div>

        {{if .Error}}
        <div class="flash flash-error">{{.Error}}</div>
        {{end}}

        <form method="POST" action="/login">
            <div class="form-group">
                <label class="form-label" for="api_key">Password or API Key</label>
                <input type="password" id="api_key" name="api_key" class="form-input" placeholder="Enter password or API key" required autofocus>
            </div>
            <button type="submit" class="btn btn-primary login-btn">Login</button>
        </form>
    </div>
</div>
{{end}}
```

**Step 2: Commit**

```bash
git add internal/web/templates/login.html
git commit -m "feat(ui): update login page with new design"
```

---

## Task 7: Update Remaining Templates

**Files:**
- Modify: `internal/web/templates/tokens.html`
- Modify: `internal/web/templates/authorized_keys.html`
- Modify: `internal/web/templates/add_device.html`
- Modify: `internal/web/templates/edit_device.html`
- Modify: `internal/web/templates/settings.html`
- Modify: `internal/web/templates/audit.html`
- Modify: `internal/web/templates/download.html`
- Modify: `internal/web/templates/quick_enroll.html`

**Step 1: Update tokens.html**

Read the current file and update classes to match new design system:
- `top-bar` → keep
- `btn btn-primary` → keep
- `card` → keep
- `table` → wrap in `table-wrapper`
- `badge active` → `badge badge-active`
- `flash` → `flash flash-success` or `flash flash-error`
- `form-group` labels → add `form-label` class
- `input` → add `form-input` class

**Step 2: Update authorized_keys.html**

Same pattern - update class names to match new design system.

**Step 3: Update add_device.html**

Update form classes:
- `form-group label` → `form-label`
- `input/textarea` → `form-input`, `form-textarea`

**Step 4: Update edit_device.html**

Same pattern as add_device.html.

**Step 5: Update settings.html**

Update form and card classes.

**Step 6: Update audit.html**

Update table wrapper and badges.

**Step 7: Update download.html**

Update card classes and grid.

**Step 8: Update quick_enroll.html**

Update form and card classes.

**Step 9: Commit all template updates**

```bash
git add internal/web/templates/
git commit -m "feat(ui): update remaining templates with new design classes"
```

---

## Task 8: Manual Testing & Verification

**Step 1: Build and run the server**

```bash
cd /Users/driversti/Projects/keyforge
go build -o keyforge ./cmd/keyforge
./keyforge serve --port 9315 --data ./test-data
```

**Step 2: Test light theme**
- Open browser to `http://localhost:9315`
- Verify clean, minimal design with emerald accents
- Check all pages render correctly
- Test forms and buttons

**Step 3: Test dark theme**
- Click theme toggle in nav
- Verify smooth transition to dark theme
- Check all pages render correctly in dark mode
- Verify text contrast is readable

**Step 4: Test theme persistence**
- Refresh page - theme should persist
- Check localStorage has `keyforge-theme` value

**Step 5: Test responsive design**
- Resize browser to mobile width
- Verify hamburger menu works
- Check tables scroll horizontally
- Test forms stack vertically

**Step 6: Test mobile navigation**
- Click hamburger menu
- Verify nav drawer opens/closes
- Test clicking nav links

---

## Task 9: Final Commit & Cleanup

**Step 1: Ensure all changes committed**

```bash
git status
git add -A
git commit -m "feat(ui): complete UI redesign with light/dark themes"
```

**Step 2: Update CHANGELOG**

Add entry to `CHANGELOG.md`:

```markdown
## [Unreleased]

### Changed
- Complete UI redesign with modern, clean aesthetic
- Added light/dark theme support with system preference detection
- New color scheme with emerald accent color
- Improved responsive design for mobile devices
- Added Inter font for better typography
```

**Step 3: Commit changelog**

```bash
git add CHANGELOG.md
git commit -m "docs: add changelog entry for UI redesign"
```

---

## Summary

**Files Created:**
- `internal/web/static/theme.js` - Theme toggle logic

**Files Modified:**
- `internal/web/static/style.css` - Complete rewrite with design system
- `internal/web/templates/layout.html` - New nav, theme toggle, Inter font
- `internal/web/templates/dashboard.html` - Updated classes
- `internal/web/templates/devices.html` - Updated classes
- `internal/web/templates/login.html` - Updated design
- `internal/web/templates/*.html` - All other templates updated

**Commits:**
1. Theme toggle script
2. CSS rewrite with design system
3. Layout with Inter font and nav
4. Dashboard template
5. Devices template
6. Login template
7. Remaining templates
8. Final commit
9. Changelog update
