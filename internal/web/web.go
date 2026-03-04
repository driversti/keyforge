package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/models"
)

//go:embed templates static
var Content embed.FS

// Handler serves the web UI pages.
type Handler struct {
	db        *db.DB
	funcMap   template.FuncMap
	serverURL string
}

// NewHandler creates a new web UI handler.
func NewHandler(database *db.DB, serverURL string) *Handler {
	return &Handler{
		db:        database,
		serverURL: serverURL,
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

// DevicesPage lists all devices.
func (h *Handler) DevicesPage(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.ListDevices()
	if err != nil {
		http.Error(w, "failed to list devices", http.StatusInternalServerError)
		return
	}

	flash := r.URL.Query().Get("flash")

	h.renderPage(w, "devices.html", map[string]any{
		"Devices": devices,
		"Flash":   flash,
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

	http.Redirect(w, r, "/?flash="+url.QueryEscape(fmt.Sprintf("Device %q registered successfully.", device.Name)), http.StatusSeeOther)
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

// RevokeDeviceAction revokes a device and redirects to the device list.
func (h *Handler) RevokeDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.RevokeDevice(id); err != nil {
		http.Redirect(w, r, "/?flash="+url.QueryEscape("Failed to revoke device."), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?flash="+url.QueryEscape("Device revoked."), http.StatusSeeOther)
}

// ReactivateDeviceAction reactivates a device and redirects to the device list.
func (h *Handler) ReactivateDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.ReactivateDevice(id); err != nil {
		http.Redirect(w, r, "/?flash="+url.QueryEscape("Failed to reactivate device."), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?flash="+url.QueryEscape("Device reactivated."), http.StatusSeeOther)
}

// DeleteDeviceAction deletes a device and redirects to the device list.
func (h *Handler) DeleteDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.DeleteDevice(id); err != nil {
		http.Redirect(w, r, "/?flash="+url.QueryEscape("Failed to delete device."), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?flash="+url.QueryEscape("Device deleted."), http.StatusSeeOther)
}
