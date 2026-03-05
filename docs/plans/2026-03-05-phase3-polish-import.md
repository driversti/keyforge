# KeyForge Phase 3 — Polish & Import Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add dashboard, audit log (API + Web UI), import commands (GitHub + file), device search/filter, settings page, and responsive improvements.

**Architecture:** Phase 3 builds on the existing Web UI, API, and DB layers. The audit log DB layer (`ListAudit`) already supports limit/offset pagination — it needs an API endpoint and Web UI page. The dashboard replaces the device list as the home page, showing stats and recent activity. Import commands are standalone CLI additions that call the existing `POST /api/v1/devices` endpoint. Device filtering is done client-side with a search input (the device count is small enough). Settings page shows server info and API key.

**Tech Stack:** Go, cobra CLI, html/template + htmx, SQLite, `net/http` for GitHub key fetching

**Spec:** `/Users/driversti/Projects/SPEC.md`

---

### Task 1: Audit Log API Endpoint

**Files:**
- Modify: `internal/db/audit_repo.go` — add `CountAudit()` method
- Create: `internal/db/audit_repo_test.go` — tests for CountAudit
- Modify: `internal/api/handlers.go` — add `ListAudit` handler
- Modify: `internal/server/server.go` — register `GET /api/v1/audit` route

**DB method — `CountAudit`:**

Add to `internal/db/audit_repo.go`:
```go
// CountAudit returns the total number of audit log entries.
func (d *DB) CountAudit() (int, error) {
	var count int
	err := d.DB.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count audit entries: %w", err)
	}
	return count, nil
}
```

**Tests** (`internal/db/audit_repo_test.go`):
- `TestLogAudit` — insert entry, verify it appears in ListAudit
- `TestListAudit_Pagination` — insert 5 entries, list with limit=2,offset=0, verify 2 returned; list with limit=2,offset=2, verify next 2
- `TestCountAudit` — insert 3 entries, verify count is 3
- `TestListAudit_Empty` — list from empty table, verify empty slice (not nil)

**API handler** — add to `internal/api/handlers.go`:
```go
// ListAudit returns paginated audit log entries.
func (h *Handler) ListAudit(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	entries, err := h.db.ListAudit(limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list audit log"})
		return
	}

	total, err := h.db.CountAudit()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to count audit entries"})
		return
	}

	if entries == nil {
		entries = []models.AuditEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
```

Add `"strconv"` to imports in handlers.go if not already present.

**Route** — add to `internal/server/server.go` in the protected API section:
```go
s.mux.Handle("GET /api/v1/audit", requireKey(http.HandlerFunc(s.apiHandler.ListAudit)))
```

---

### Task 2: Audit Log Web UI Page

**Files:**
- Create: `internal/web/templates/audit.html`
- Modify: `internal/web/web.go` — add `AuditPage` handler
- Modify: `internal/server/server.go` — register `GET /audit` web route
- Modify: `internal/web/templates/layout.html` — add "Audit Log" nav link

**Template** (`internal/web/templates/audit.html`):
```html
{{define "title"}}Audit Log{{end}}
{{define "content"}}
<div class="top-bar">
    <h1>Audit Log</h1>
</div>

{{if .Entries}}
<table>
    <thead>
        <tr>
            <th>Time</th>
            <th>Action</th>
            <th>Details</th>
            <th>Source IP</th>
        </tr>
    </thead>
    <tbody>
        {{range .Entries}}
        <tr>
            <td style="white-space:nowrap">{{formatDate .CreatedAt}}</td>
            <td><span class="badge {{actionBadgeClass .Action}}">{{.Action}}</span></td>
            <td>{{.Details}}</td>
            <td style="font-family:monospace;font-size:0.85rem;color:var(--text-muted)">{{.SourceIP}}</td>
        </tr>
        {{end}}
    </tbody>
</table>

<div class="pagination">
    {{if gt .Offset 0}}
    <a href="/audit?page={{prevPage .Page}}" class="btn btn-outline btn-sm">&larr; Newer</a>
    {{end}}
    <span class="page-info">Page {{.Page}} of {{.TotalPages}}</span>
    {{if lt .Page .TotalPages}}
    <a href="/audit?page={{nextPage .Page}}" class="btn btn-outline btn-sm">Older &rarr;</a>
    {{end}}
</div>
{{else}}
<p style="color:var(--text-muted)">No audit log entries yet.</p>
{{end}}
{{end}}
```

**Handler** — add to `internal/web/web.go`:
```go
// AuditPage lists audit log entries with pagination.
func (h *Handler) AuditPage(w http.ResponseWriter, r *http.Request) {
	const perPage = 50

	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}

	offset := (page - 1) * perPage

	entries, err := h.db.ListAudit(perPage, offset)
	if err != nil {
		http.Error(w, "failed to list audit log", http.StatusInternalServerError)
		return
	}

	total, err := h.db.CountAudit()
	if err != nil {
		http.Error(w, "failed to count audit entries", http.StatusInternalServerError)
		return
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}

	h.renderPage(w, "audit.html", map[string]any{
		"Entries":    entries,
		"Page":       page,
		"TotalPages": totalPages,
		"Offset":     offset,
		"Total":      total,
	})
}
```

**Template functions** — add to the `funcMap` in `NewHandler`:
```go
"actionBadgeClass": func(action string) string {
	switch {
	case strings.Contains(action, "created"):
		return "active"
	case strings.Contains(action, "revoked"), strings.Contains(action, "deleted"):
		return "revoked"
	default:
		return ""
	}
},
"prevPage": func(page int) int {
	if page > 1 {
		return page - 1
	}
	return 1
},
"nextPage": func(page int) int {
	return page + 1
},
```

Add `"strconv"` to web.go imports if not present.

**CSS** — add to `internal/web/static/style.css`:
```css
/* Pagination */
.pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    margin-top: 1.5rem;
    padding: 1rem 0;
}

.page-info {
    font-size: 0.9rem;
    color: var(--text-muted);
}
```

**Route** — add to `internal/server/server.go` in the web UI section:
```go
s.mux.Handle("GET /audit", requireSession(http.HandlerFunc(s.webHandler.AuditPage)))
```

**Nav link** — in `internal/web/templates/layout.html`, add before the Logout link:
```html
<a href="/audit">Audit Log</a>
```

---

### Task 3: Dashboard Page

**Files:**
- Create: `internal/web/templates/dashboard.html`
- Modify: `internal/web/web.go` — add `DashboardPage` handler, update `DevicesPage` route awareness
- Modify: `internal/server/server.go` — change `GET /{$}` to dashboard, add `GET /devices` for device list
- Modify: `internal/web/templates/layout.html` — update nav (Dashboard link, Devices link)

The dashboard becomes the new home page (`/`). The device list moves to `/devices`.

**Template** (`internal/web/templates/dashboard.html`):
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
    <h2>Recent Activity</h2>
    {{if .RecentActivity}}
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
    <div style="text-align:center;margin-top:1rem">
        <a href="/audit" class="btn btn-outline btn-sm">View Full Audit Log</a>
    </div>
    {{else}}
    <p style="color:var(--text-muted)">No activity yet.</p>
    {{end}}
</div>
{{end}}
```

**Handler** — add to `internal/web/web.go`:
```go
// DashboardPage renders the dashboard with stats and recent activity.
func (h *Handler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.ListDevices()
	if err != nil {
		http.Error(w, "failed to list devices", http.StatusInternalServerError)
		return
	}

	var active, revoked, sshAccepting int
	for _, d := range devices {
		if d.Status == models.StatusActive {
			active++
			if d.AcceptsSSH {
				sshAccepting++
			}
		} else {
			revoked++
		}
	}

	recentActivity, err := h.db.ListAudit(10, 0)
	if err != nil {
		recentActivity = nil // non-fatal
	}

	h.renderPage(w, "dashboard.html", map[string]any{
		"TotalDevices":   len(devices),
		"ActiveDevices":  active,
		"RevokedDevices": revoked,
		"SSHAccepting":   sshAccepting,
		"RecentActivity": recentActivity,
	})
}
```

**CSS** — add to `internal/web/static/style.css`:
```css
/* Stats grid */
.stats-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1rem;
    margin-bottom: 1.5rem;
}

.stat-card {
    background: var(--surface);
    border-radius: var(--radius);
    padding: 1.25rem;
    text-align: center;
}

.stat-number {
    font-size: 2rem;
    font-weight: 700;
    color: var(--primary);
}

.stat-number.stat-success {
    color: var(--success);
}

.stat-number.stat-danger {
    color: var(--danger);
}
```

**Route changes** in `internal/server/server.go`:
```go
// Replace: s.mux.Handle("GET /{$}", requireSession(http.HandlerFunc(s.webHandler.DevicesPage)))
// With:
s.mux.Handle("GET /{$}", requireSession(http.HandlerFunc(s.webHandler.DashboardPage)))
s.mux.Handle("GET /devices", requireSession(http.HandlerFunc(s.webHandler.DevicesPage)))
```

**Nav updates** in `internal/web/templates/layout.html`:
- Change the first nav link from `<a href="/">Devices</a>` or equivalent to:
```html
<a href="/">Dashboard</a>
<a href="/devices">Devices</a>
```

**Also update `DevicesPage` redirects** — any redirects in `web.go` that go to `/?flash=...` should now go to `/devices?flash=...` since the device list moved to `/devices`. These are in:
- `AddDeviceSubmit` — redirect to `/devices?flash=...`
- `RevokeDeviceAction` — redirect to `/devices?flash=...`
- `ReactivateDeviceAction` — redirect to `/devices?flash=...`
- `DeleteDeviceAction` — redirect to `/devices?flash=...`

---

### Task 4: Import CLI Commands

**Files:**
- Create: `cmd/keyforge/import.go`
- Modify: `cmd/keyforge/main.go` — register import command

**Command:** `keyforge import --github <username>` and `keyforge import --file <path> --name <name>`

**`cmd/keyforge/import.go`:**
```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	var (
		githubUser string
		filePath   string
		name       string
		acceptSSH  bool
		tags       []string
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import SSH public keys from GitHub or a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if githubUser == "" && filePath == "" {
				return fmt.Errorf("specify --github <username> or --file <path>")
			}
			if githubUser != "" && filePath != "" {
				return fmt.Errorf("use either --github or --file, not both")
			}

			if githubUser != "" {
				return importFromGitHub(githubUser, acceptSSH, tags)
			}
			return importFromFile(filePath, name, acceptSSH, tags)
		},
	}

	cmd.Flags().StringVar(&githubUser, "github", "", "GitHub username to import keys from")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to SSH public key file")
	cmd.Flags().StringVar(&name, "name", "", "Device name (required with --file)")
	cmd.Flags().BoolVar(&acceptSSH, "accept-ssh", false, "Mark imported devices as SSH-accepting")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags for imported devices (comma-separated)")

	return cmd
}

func importFromGitHub(username string, acceptSSH bool, tags []string) error {
	url := fmt.Sprintf("https://github.com/%s.keys", username)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetch GitHub keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub returned status %d (user %q may not exist)", resp.StatusCode, username)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	keys := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(keys) == 0 || (len(keys) == 1 && keys[0] == "") {
		fmt.Printf("No public keys found for GitHub user %q.\n", username)
		return nil
	}

	imported := 0
	for i, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		deviceName := fmt.Sprintf("%s-github-%d", username, i+1)
		if len(keys) == 1 {
			deviceName = fmt.Sprintf("%s-github", username)
		}

		err := registerDevice(deviceName, key, acceptSSH, tags)
		if err != nil {
			fmt.Printf("  [skip] %s: %s\n", deviceName, err)
			continue
		}
		fmt.Printf("  [ok]   %s imported\n", deviceName)
		imported++
	}

	fmt.Printf("\nImported %d of %d keys from GitHub user %q.\n", imported, len(keys), username)
	return nil
}

func importFromFile(path, name string, acceptSSH bool, tags []string) error {
	if name == "" {
		// Default name from filename: ~/.ssh/id_ed25519.pub -> id_ed25519
		base := filepath.Base(path)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read key file: %w", err)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return fmt.Errorf("key file is empty")
	}

	if err := registerDevice(name, key, acceptSSH, tags); err != nil {
		return err
	}

	fmt.Printf("Imported key from %s as device %q.\n", path, name)
	return nil
}

func registerDevice(name, publicKey string, acceptSSH bool, tags []string) error {
	payload := map[string]any{
		"name":       name,
		"public_key": publicKey,
		"accepts_ssh": acceptSSH,
	}
	if len(tags) > 0 {
		payload["tags"] = tags
	}

	body, _ := json.Marshal(payload)
	resp, err := apiRequest("POST", "/api/v1/devices", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("already registered")
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}
```

**Register** — in `cmd/keyforge/main.go` `init()`:
```go
rootCmd.AddCommand(newImportCmd())
```

---

### Task 5: Device Search & Filter in Web UI

**Files:**
- Modify: `internal/web/templates/devices.html` — add search input and status filter
- Modify: `internal/web/web.go` — update `DevicesPage` to accept query params
- Modify: `internal/web/static/style.css` — add filter bar styles

This uses server-side filtering (query params `?q=search&status=active`).

**DB method** — add to `internal/db/device_repo.go`:
```go
// SearchDevices returns devices matching the search query and optional status filter.
func (d *DB) SearchDevices(query string, status string) ([]models.Device, error) {
	var args []any
	sql := `SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen
	        FROM devices WHERE 1=1`

	if query != "" {
		sql += ` AND (name LIKE ? OR fingerprint LIKE ? OR tags LIKE ?)`
		like := "%" + query + "%"
		args = append(args, like, like, like)
	}

	if status != "" {
		sql += ` AND status = ?`
		args = append(args, status)
	}

	sql += ` ORDER BY registered_at DESC`

	rows, err := d.DB.Query(sql, args...)
	// ... same scan logic as ListDevices ...
}
```

Actually, to avoid duplicating scan logic, refactor `ListDevices` and `SearchDevices` to share a `scanDevices(rows)` helper.

**Test** (`internal/db/device_repo_test.go` — add):
- `TestSearchDevices_ByName` — create 3 devices, search by partial name, verify filtered results
- `TestSearchDevices_ByStatus` — create active + revoked devices, filter by status

**Update devices.html** — add filter bar above the table:
```html
<form method="GET" action="/devices" class="filter-bar">
    <input type="text" name="q" value="{{.Query}}" placeholder="Search by name, fingerprint, or tag..." class="filter-input">
    <select name="status" class="filter-select">
        <option value="">All Statuses</option>
        <option value="active" {{if eq .StatusFilter "active"}}selected{{end}}>Active</option>
        <option value="revoked" {{if eq .StatusFilter "revoked"}}selected{{end}}>Revoked</option>
    </select>
    <button type="submit" class="btn btn-outline btn-sm">Filter</button>
    {{if or .Query .StatusFilter}}
    <a href="/devices" class="btn btn-sm" style="color:var(--text-muted)">Clear</a>
    {{end}}
</form>
```

**Update DevicesPage handler** in `web.go`:
```go
func (h *Handler) DevicesPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	statusFilter := r.URL.Query().Get("status")

	var devices []models.Device
	var err error

	if query != "" || statusFilter != "" {
		devices, err = h.db.SearchDevices(query, statusFilter)
	} else {
		devices, err = h.db.ListDevices()
	}

	if err != nil {
		http.Error(w, "failed to list devices", http.StatusInternalServerError)
		return
	}

	flash := r.URL.Query().Get("flash")

	h.renderPage(w, "devices.html", map[string]any{
		"Devices":      devices,
		"Flash":        flash,
		"Query":        query,
		"StatusFilter": statusFilter,
	})
}
```

**CSS** — add to `internal/web/static/style.css`:
```css
/* Filter bar */
.filter-bar {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    margin-bottom: 1.5rem;
    flex-wrap: wrap;
}

.filter-input {
    flex: 1;
    min-width: 200px;
    padding: 0.5rem 0.8rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    color: var(--text);
    font-size: 0.9rem;
}

.filter-input:focus {
    outline: none;
    border-color: var(--primary);
    box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
}

.filter-select {
    padding: 0.5rem 0.8rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    color: var(--text);
    font-size: 0.9rem;
}

.filter-select:focus {
    outline: none;
    border-color: var(--primary);
}
```

**Responsive** — add to the `@media (max-width: 768px)` section:
```css
.filter-bar {
    flex-direction: column;
}

.filter-input {
    min-width: 100%;
}

.stats-grid {
    grid-template-columns: repeat(2, 1fr);
}
```

---

### Task 6: Settings Page

**Files:**
- Create: `internal/web/templates/settings.html`
- Modify: `internal/web/web.go` — add `SettingsPage` handler
- Modify: `internal/server/server.go` — register `GET /settings` route
- Modify: `internal/web/templates/layout.html` — add Settings nav link

**Template** (`internal/web/templates/settings.html`):
```html
{{define "title"}}Settings{{end}}
{{define "content"}}
<div class="top-bar">
    <h1>Settings</h1>
</div>

<div class="card">
    <h2>API Key</h2>
    <p style="color:var(--text-muted);margin-bottom:1rem">Use this key for CLI commands and API access. Keep it secret.</p>
    <div style="display:flex;gap:0.5rem;align-items:center">
        <code id="api-key-display" style="flex:1;background:var(--bg);padding:0.6rem 0.8rem;border-radius:var(--radius);border:1px solid var(--border);font-size:0.85rem;overflow:hidden;text-overflow:ellipsis">{{.MaskedAPIKey}}</code>
        <button class="btn btn-outline btn-sm" onclick="toggleAPIKey()" id="toggle-key-btn">Show</button>
        <button class="btn btn-outline btn-sm" onclick="copyAPIKey()">Copy</button>
    </div>
</div>

<div class="card">
    <h2>Server Info</h2>
    <table>
        <tbody>
            <tr><td style="font-weight:500;width:200px">Server URL</td><td style="font-family:monospace">{{.ServerURL}}</td></tr>
            <tr><td style="font-weight:500">Total Devices</td><td>{{.TotalDevices}}</td></tr>
            <tr><td style="font-weight:500">Active Devices</td><td>{{.ActiveDevices}}</td></tr>
            <tr><td style="font-weight:500">SSH-Accepting</td><td>{{.SSHAccepting}}</td></tr>
            <tr><td style="font-weight:500">Enrollment Tokens</td><td>{{.TotalTokens}}</td></tr>
            <tr><td style="font-weight:500">Audit Log Entries</td><td>{{.AuditCount}}</td></tr>
        </tbody>
    </table>
</div>

<div class="card">
    <h2>Quick Reference</h2>
    <p style="color:var(--text-muted);margin-bottom:0.75rem">Common CLI commands:</p>
    <div class="keys-output">keyforge serve --port 8080 --data ./keyforge-data
keyforge enroll --name "device" --token TOKEN --server {{.ServerURL}}
keyforge keys --install --server {{.ServerURL}}
keyforge token create --label "new-device" --expires 1h --server {{.ServerURL}} --api-key API_KEY</div>
</div>

<script>
var keyHidden = true;
var fullKey = "{{.APIKey}}";
var maskedKey = "{{.MaskedAPIKey}}";
function toggleAPIKey() {
    var el = document.getElementById('api-key-display');
    var btn = document.getElementById('toggle-key-btn');
    keyHidden = !keyHidden;
    el.textContent = keyHidden ? maskedKey : fullKey;
    btn.textContent = keyHidden ? 'Show' : 'Hide';
}
function copyAPIKey() {
    navigator.clipboard.writeText(fullKey);
}
</script>
{{end}}
```

**Handler** — add to `internal/web/web.go`:
```go
// SettingsPage shows server settings and info.
func (h *Handler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	devices, _ := h.db.ListDevices()
	tokens, _ := h.db.ListTokens()
	auditCount, _ := h.db.CountAudit()

	var active, sshAccepting int
	for _, d := range devices {
		if d.Status == models.StatusActive {
			active++
			if d.AcceptsSSH {
				sshAccepting++
			}
		}
	}

	masked := h.apiKey[:8] + strings.Repeat("*", len(h.apiKey)-8)

	h.renderPage(w, "settings.html", map[string]any{
		"APIKey":       h.apiKey,
		"MaskedAPIKey": masked,
		"ServerURL":    h.serverURL,
		"TotalDevices": len(devices),
		"ActiveDevices": active,
		"SSHAccepting": sshAccepting,
		"TotalTokens":  len(tokens),
		"AuditCount":   auditCount,
	})
}
```

**Route** — in `internal/server/server.go`:
```go
s.mux.Handle("GET /settings", requireSession(http.HandlerFunc(s.webHandler.SettingsPage)))
```

**Nav link** — in `layout.html`, add before Logout:
```html
<a href="/settings">Settings</a>
```

---

### Task 7: Responsive CSS Improvements

**Files:**
- Modify: `internal/web/static/style.css` — enhance mobile styles

Enhance the existing `@media (max-width: 768px)` section and add a smaller breakpoint:

**Update existing responsive section** — replace the current `@media (max-width: 768px)` block with:
```css
@media (max-width: 768px) {
    nav .container {
        flex-wrap: wrap;
        gap: 0.5rem;
    }

    nav .logo {
        margin-right: 0;
        width: 100%;
    }

    nav a {
        font-size: 0.85rem;
    }

    .top-bar {
        flex-direction: column;
        align-items: flex-start;
        gap: 0.75rem;
    }

    table {
        display: block;
        overflow-x: auto;
    }

    .actions {
        flex-direction: column;
    }

    .fingerprint {
        max-width: 120px;
    }

    .stats-grid {
        grid-template-columns: repeat(2, 1fr);
    }

    .filter-bar {
        flex-direction: column;
    }

    .filter-input {
        min-width: 100%;
    }

    .card {
        padding: 1rem;
    }

    .pagination {
        flex-wrap: wrap;
    }
}

@media (max-width: 480px) {
    .container {
        padding: 0 0.75rem;
    }

    .stats-grid {
        grid-template-columns: 1fr 1fr;
        gap: 0.5rem;
    }

    .stat-card {
        padding: 0.75rem;
    }

    .stat-number {
        font-size: 1.5rem;
    }

    h1 {
        font-size: 1.25rem;
    }
}
```

---

### Task 8: Integration Tests for Phase 3

**Files:**
- Modify: `test/integration_test.go` — add Phase 3 tests

**Tests to add:**

**TestIntegration_AuditLogAPI:**
1. Start server, create API key
2. Create a device (generates audit entry)
3. `GET /api/v1/audit` with API key
4. Verify response contains `entries` array with at least one entry
5. Verify `total` field is correct
6. Test pagination: `?limit=1&offset=0` returns 1 entry, `?limit=1&offset=1` returns next
7. Test auth enforcement: `GET /api/v1/audit` without API key returns 401

**TestIntegration_ImportFlow:**
1. Start server
2. Register a device via API (simulating import)
3. Verify device appears in list
4. Try registering with duplicate key — verify 409

**TestIntegration_DeviceSearch:**
1. Start server
2. Create 3 devices with different names/tags
3. Access web handler `DevicesPage` with `?q=searchterm`
4. Verify only matching devices returned

---

## Summary

After completing all 8 tasks:
- **Audit log API** (`GET /api/v1/audit`) with pagination (limit/offset + total count)
- **Audit log Web UI page** with pagination (page-based navigation)
- **Dashboard** as new home page — device stats + recent activity feed
- **Device list** moved to `/devices` with search/filter (text search + status filter)
- **Import commands** — `keyforge import --github <user>` and `keyforge import --file <path>`
- **Settings page** — API key (show/copy), server info, quick reference
- **Responsive CSS** — improved mobile layout for all new and existing pages
- **Integration tests** for audit API, import flow, and device search

Nav order: Dashboard | Devices | Add Device | Authorized Keys | Tokens | Audit Log | Settings | Logout
