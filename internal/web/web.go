package web

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"driversti.dev/keyforge/internal/db"
	"driversti.dev/keyforge/internal/models"
)

//go:embed templates static
var Content embed.FS

// SessionStore holds active session tokens in memory.
type SessionStore struct {
	mu     sync.RWMutex
	tokens map[string]time.Time // token -> expiry
}

// NewSessionStore creates a new in-memory session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{tokens: make(map[string]time.Time)}
}

// Create generates a new session token and stores it with a 24-hour expiry.
func (s *SessionStore) Create() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	s.mu.Lock()
	s.tokens[token] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	return token, nil
}

// Valid checks whether a session token is valid and not expired.
func (s *SessionStore) Valid(token string) bool {
	s.mu.RLock()
	expiry, ok := s.tokens[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		s.Delete(token)
		return false
	}
	return true
}

// Delete removes a session token.
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.tokens, token)
	s.mu.Unlock()
}

// SessionAuth returns middleware that checks for a valid session cookie or API key query param.
func SessionAuth(apiKey string, sessions *SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check session cookie.
			if cookie, err := r.Cookie("keyforge_session"); err == nil {
				if sessions.Valid(cookie.Value) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check API key in query parameter.
			if key := r.URL.Query().Get("key"); key != "" {
				if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Redirect to login page.
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		})
	}
}

// Handler serves the web UI pages.
type Handler struct {
	db              *db.DB
	funcMap         template.FuncMap
	serverURL       string
	apiKey          string
	sessions        *SessionStore
	enrollScriptBody string
}

// NewHandler creates a new web UI handler.
func NewHandler(database *db.DB, serverURL string, apiKey string, sessions *SessionStore, enrollScriptBody string) *Handler {
	return &Handler{
		db:               database,
		serverURL:        serverURL,
		apiKey:           apiKey,
		sessions:         sessions,
		enrollScriptBody: enrollScriptBody,
		funcMap: template.FuncMap{
			"truncate": func(s string, n int) string {
				if len(s) <= n {
					return s
				}
				return s[:n] + "..."
			},
			"formatDate": func(t time.Time) string {
				return t.Format("2006-01-02")
			},
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
		},
	}
}

// renderPage parses layout.html together with the specific page template and
// executes it. Parsing per request avoids shared template set issues.
func (h *Handler) renderPage(w http.ResponseWriter, page string, data any) {
	tmpl, err := template.New("").Funcs(h.funcMap).ParseFS(Content, "templates/layout.html", "templates/"+page)
	if err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
	}
}

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

// DevicesPage lists all devices with optional search and status filter.
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

// AddDevicePage renders the add device form.
func (h *Handler) AddDevicePage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, "add_device.html", map[string]any{
		"Name":       "",
		"PublicKey":  "",
		"Tags":       "",
		"AcceptsSSH": false,
		"Error":      "",
	})
}

// AddDeviceSubmit handles the add device form submission.
func (h *Handler) AddDeviceSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	publicKey := strings.TrimSpace(r.FormValue("public_key"))
	tagsRaw := strings.TrimSpace(r.FormValue("tags"))
	acceptsSSH := r.FormValue("accepts_ssh") == "true"

	// Parse tags.
	var tags []string
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	// Validate.
	if name == "" || publicKey == "" {
		h.renderPage(w, "add_device.html", map[string]any{
			"Name":       name,
			"PublicKey":  publicKey,
			"Tags":       tagsRaw,
			"AcceptsSSH": acceptsSSH,
			"Error":      "Name and public key are required.",
		})
		return
	}

	req := models.CreateDeviceRequest{
		Name:       name,
		PublicKey:  publicKey,
		AcceptsSSH: acceptsSSH,
		Tags:       tags,
	}

	device, err := h.db.CreateDevice(req)
	if err != nil {
		h.renderPage(w, "add_device.html", map[string]any{
			"Name":       name,
			"PublicKey":  publicKey,
			"Tags":       tagsRaw,
			"AcceptsSSH": acceptsSSH,
			"Error":      err.Error(),
		})
		return
	}

	devID := device.ID
	h.db.LogAudit("device.created", &devID, fmt.Sprintf("device %q registered via web UI", device.Name), r.RemoteAddr)

	http.Redirect(w, r, "/devices?flash="+url.QueryEscape(fmt.Sprintf("Device %q registered successfully.", device.Name)), http.StatusSeeOther)
}

// AuthorizedKeysPage shows all active public keys.
func (h *Handler) AuthorizedKeysPage(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetActivePublicKeys()
	if err != nil {
		http.Error(w, "failed to get keys", http.StatusInternalServerError)
		return
	}

	var keys []string
	for _, d := range devices {
		keys = append(keys, d.PublicKey)
	}

	h.renderPage(w, "authorized_keys.html", map[string]any{
		"Keys":      strings.Join(keys, "\n"),
		"KeyCount":  len(keys),
		"ServerURL": h.serverURL,
	})
}

// DownloadPage renders the public download/install page.
func (h *Handler) DownloadPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, "download.html", map[string]any{
		"ServerURL": h.serverURL,
	})
}

// RevokeDeviceAction revokes a device and redirects to the device list.
func (h *Handler) RevokeDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.RevokeDevice(id); err != nil {
		http.Redirect(w, r, "/devices?flash="+url.QueryEscape("Failed to revoke device."), http.StatusSeeOther)
		return
	}
	h.db.LogAudit("device.revoked", &id, "device revoked via web UI", r.RemoteAddr)
	http.Redirect(w, r, "/devices?flash="+url.QueryEscape("Device revoked."), http.StatusSeeOther)
}

// ReactivateDeviceAction reactivates a device and redirects to the device list.
func (h *Handler) ReactivateDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.ReactivateDevice(id); err != nil {
		http.Redirect(w, r, "/devices?flash="+url.QueryEscape("Failed to reactivate device."), http.StatusSeeOther)
		return
	}
	h.db.LogAudit("device.reactivated", &id, "device reactivated via web UI", r.RemoteAddr)
	http.Redirect(w, r, "/devices?flash="+url.QueryEscape("Device reactivated."), http.StatusSeeOther)
}

// DeleteDeviceAction deletes a device and redirects to the device list.
func (h *Handler) DeleteDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	// Get device first for audit log details.
	device, _ := h.db.GetDevice(id)

	if err := h.db.DeleteDevice(id); err != nil {
		http.Redirect(w, r, "/devices?flash="+url.QueryEscape("Failed to delete device."), http.StatusSeeOther)
		return
	}

	details := "device deleted via web UI"
	if device != nil {
		details = fmt.Sprintf("device %q deleted via web UI", device.Name)
	}
	h.db.LogAudit("device.deleted", &id, details, r.RemoteAddr)
	http.Redirect(w, r, "/devices?flash="+url.QueryEscape("Device deleted."), http.StatusSeeOther)
}

// EditDevicePage renders the edit device form.
func (h *Handler) EditDevicePage(w http.ResponseWriter, r *http.Request, id string) {
	device, err := h.db.GetDevice(id)
	if err != nil {
		http.Redirect(w, r, "/devices?flash="+url.QueryEscape("Device not found."), http.StatusSeeOther)
		return
	}

	h.renderPage(w, "edit_device.html", map[string]any{
		"Device":     device,
		"TagsString": strings.Join(device.Tags, ", "),
		"Error":      "",
	})
}

// EditDeviceSubmit handles the edit device form submission.
func (h *Handler) EditDeviceSubmit(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	device, err := h.db.GetDevice(id)
	if err != nil {
		http.Redirect(w, r, "/devices?flash="+url.QueryEscape("Device not found."), http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	tagsRaw := strings.TrimSpace(r.FormValue("tags"))
	acceptsSSH := r.FormValue("accepts_ssh") == "true"

	if name == "" {
		h.renderPage(w, "edit_device.html", map[string]any{
			"Device":     device,
			"TagsString": tagsRaw,
			"Error":      "Name is required.",
		})
		return
	}

	// Parse tags.
	var tags []string
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}
	if tags == nil {
		tags = []string{}
	}

	req := models.UpdateDeviceRequest{
		Name:       &name,
		AcceptsSSH: &acceptsSSH,
		Tags:       tags,
	}

	if err := h.db.UpdateDevice(id, req); err != nil {
		h.renderPage(w, "edit_device.html", map[string]any{
			"Device":     device,
			"TagsString": tagsRaw,
			"Error":      err.Error(),
		})
		return
	}

	h.db.LogAudit("device.updated", &id, fmt.Sprintf("device %q updated via web UI", name), r.RemoteAddr)

	http.Redirect(w, r, "/devices?flash="+url.QueryEscape(fmt.Sprintf("Device %q updated.", name)), http.StatusSeeOther)
}

// TokensPage lists all enrollment tokens.
func (h *Handler) TokensPage(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.db.ListTokens()
	if err != nil {
		http.Error(w, "failed to list tokens", http.StatusInternalServerError)
		return
	}

	flash := r.URL.Query().Get("flash")
	createdToken := r.URL.Query().Get("created_token")
	createdEnrollCmd := r.URL.Query().Get("created_enroll_cmd")

	h.renderPage(w, "tokens.html", map[string]any{
		"Tokens":          tokens,
		"Flash":           flash,
		"CreatedToken":    createdToken,
		"CreatedEnrollCmd": createdEnrollCmd,
		"Now":             time.Now().UTC(),
	})
}

// CreateTokenSubmit handles the create token form submission.
func (h *Handler) CreateTokenSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	label := strings.TrimSpace(r.FormValue("label"))
	expiresIn := strings.TrimSpace(r.FormValue("expires_in"))

	if expiresIn == "" {
		http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Expiry duration is required."), http.StatusSeeOther)
		return
	}

	duration, err := time.ParseDuration(expiresIn)
	if err != nil {
		http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Invalid expiry duration."), http.StatusSeeOther)
		return
	}

	expiresAt := time.Now().Add(duration)
	token, err := h.db.CreateToken(label, expiresAt)
	if err != nil {
		http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Failed to create token."), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Token created successfully.")+"&created_token="+url.QueryEscape(token.Token), http.StatusSeeOther)
}

// DeleteTokenAction deletes an enrollment token and redirects to the token list.
func (h *Handler) DeleteTokenAction(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.DeleteToken(id); err != nil {
		http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Failed to delete token."), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Token deleted."), http.StatusSeeOther)
}

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
		"APIKey":        h.apiKey,
		"MaskedAPIKey":  masked,
		"ServerURL":     h.serverURL,
		"TotalDevices":  len(devices),
		"ActiveDevices": active,
		"SSHAccepting":  sshAccepting,
		"TotalTokens":   len(tokens),
		"AuditCount":    auditCount,
	})
}

// LoginPage renders the login form.
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to home.
	if cookie, err := r.Cookie("keyforge_session"); err == nil {
		if h.sessions.Valid(cookie.Value) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	h.renderPage(w, "login.html", map[string]any{
		"Error": r.URL.Query().Get("error"),
	})
}

// LoginSubmit handles the login form submission.
func (h *Handler) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	key := strings.TrimSpace(r.FormValue("api_key"))
	if subtle.ConstantTimeCompare([]byte(key), []byte(h.apiKey)) != 1 {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Invalid API key."), http.StatusSeeOther)
		return
	}

	token, err := h.sessions.Create()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "keyforge_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// QuickEnrollPage serves either a customized enrollment script (for curl)
// or an info page (for browsers) based on the Accept header.
func (h *Handler) QuickEnrollPage(w http.ResponseWriter, r *http.Request, code string) {
	token, err := h.db.GetTokenByCode(code)
	if err != nil {
		http.Error(w, "Enrollment link not found.", http.StatusNotFound)
		return
	}
	if token.Used {
		http.Error(w, "This enrollment link has already been used.", http.StatusGone)
		return
	}
	if time.Now().UTC().After(token.ExpiresAt) {
		http.Error(w, "This enrollment link has expired.", http.StatusGone)
		return
	}

	// Content negotiation: browsers get HTML, curl/wget get the script.
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		h.renderQuickEnrollPage(w, token)
		return
	}
	h.renderQuickEnrollScript(w, token)
}

// renderQuickEnrollScript writes a shell script with baked-in variables followed
// by the shared enrollment body.
func (h *Handler) renderQuickEnrollScript(w http.ResponseWriter, token *models.EnrollmentToken) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	acceptSSH := "false"
	if token.AcceptSSH {
		acceptSSH = "true"
	}

	fmt.Fprintf(w, "#!/bin/sh\nset -e\n\n")
	fmt.Fprintf(w, "NAME=%q\n", token.DeviceName)
	fmt.Fprintf(w, "TOKEN=%q\n", token.Token)
	fmt.Fprintf(w, "SERVER_URL=%q\n", h.serverURL)
	fmt.Fprintf(w, "ACCEPT_SSH=%q\n", acceptSSH)
	fmt.Fprintf(w, "SYNC_INTERVAL=%q\n", token.SyncInterval)
	fmt.Fprintf(w, "KEY_PATH=\"$HOME/.ssh/id_ed25519\"\n\n")
	fmt.Fprint(w, h.enrollScriptBody)
}

// renderQuickEnrollPage renders the browser-friendly quick enroll info page.
func (h *Handler) renderQuickEnrollPage(w http.ResponseWriter, token *models.EnrollmentToken) {
	curlCmd := fmt.Sprintf("curl -sSL %s/e/%s | sh", h.serverURL, token.Code)
	h.renderPage(w, "quick_enroll.html", map[string]any{
		"Token":   token,
		"CurlCmd": curlCmd,
	})
}

// CreateQuickEnrollSubmit handles the quick enroll form submission.
func (h *Handler) CreateQuickEnrollSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	deviceName := strings.TrimSpace(r.FormValue("device_name"))
	acceptSSH := r.FormValue("accept_ssh") == "true"
	syncInterval := strings.TrimSpace(r.FormValue("sync_interval"))
	expiresIn := strings.TrimSpace(r.FormValue("expires_in"))

	if deviceName == "" || expiresIn == "" {
		http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Device name and expiry are required."), http.StatusSeeOther)
		return
	}

	duration, err := time.ParseDuration(expiresIn)
	if err != nil {
		http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Invalid expiry duration."), http.StatusSeeOther)
		return
	}

	expiresAt := time.Now().Add(duration)
	token, err := h.db.CreateQuickEnroll(deviceName, acceptSSH, syncInterval, expiresAt)
	if err != nil {
		http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Failed to create enrollment link."), http.StatusSeeOther)
		return
	}

	enrollCmd := fmt.Sprintf("curl -sSL %s/e/%s | sh", h.serverURL, token.Code)
	http.Redirect(w, r, "/tokens?flash="+url.QueryEscape("Enrollment link created!")+"&created_enroll_cmd="+url.QueryEscape(enrollCmd), http.StatusSeeOther)
}

// LogoutHandler clears the session and redirects to login.
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("keyforge_session"); err == nil {
		h.sessions.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "keyforge_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
