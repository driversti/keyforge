package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"driversti.dev/keyforge/internal/db"
	"driversti.dev/keyforge/internal/models"
)

// Handler holds the database dependency for all API handlers.
type Handler struct {
	db     *db.DB
	apiKey string
}

// NewHandler creates a new Handler with the given database and API key.
func NewHandler(database *db.DB, apiKey string) *Handler {
	return &Handler{db: database, apiKey: apiKey}
}

// isAPIKeyValid checks whether the request carries a valid Bearer token.
func (h *Handler) isAPIKeyValid(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(h.apiKey)) == 1
}

// HealthCheck returns a simple health status.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetAuthorizedKeys returns all active public keys.
// With ?format=json it returns a JSON array; otherwise plain text (one key per line).
func (h *Handler) GetAuthorizedKeys(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetActivePublicKeys()
	if err != nil {
		http.Error(w, "failed to get keys", http.StatusInternalServerError)
		return
	}

	if r.URL.Query().Get("format") == "json" {
		type keyEntry struct {
			Name        string `json:"name"`
			PublicKey   string `json:"public_key"`
			Fingerprint string `json:"fingerprint"`
		}
		var entries []keyEntry
		for _, d := range devices {
			entries = append(entries, keyEntry{
				Name:        d.Name,
				PublicKey:   d.PublicKey,
				Fingerprint: d.Fingerprint,
			})
		}
		if entries == nil {
			entries = []keyEntry{}
		}
		writeJSON(w, http.StatusOK, entries)
		return
	}

	// Default: plain text.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, d := range devices {
		fmt.Fprintln(w, d.PublicKey)
	}
}

// ListDevices returns all devices as a JSON array.
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.ListDevices()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list devices"})
		return
	}

	// Ensure empty array instead of null in JSON output.
	if devices == nil {
		devices = []models.Device{}
	}

	writeJSON(w, http.StatusOK, devices)
}

// GetDevice returns a single device by ID.
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request, id string) {
	device, err := h.db.GetDevice(id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get device"})
		return
	}

	writeJSON(w, http.StatusOK, device)
}

// CreateDevice creates a new device from the JSON request body.
// Auth: accepts either a valid API key (Bearer token) or an enrollment token in the body.
func (h *Handler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var req models.CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	// Auth: check API key OR enrollment token.
	if !h.isAPIKeyValid(r) {
		if req.EnrollmentToken == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "API key or enrollment token required"})
			return
		}
		_, err := h.db.ValidateAndBurnToken(req.EnrollmentToken)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
	}

	if req.Name == "" || req.PublicKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and public_key are required"})
		return
	}

	device, err := h.db.CreateDevice(req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "already registered") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "invalid SSH public key") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create device"})
		return
	}

	// Log audit.
	devID := device.ID
	h.db.LogAudit("device.created", &devID, fmt.Sprintf("device %q registered", device.Name), r.RemoteAddr)

	writeJSON(w, http.StatusCreated, device)
}

// UpdateDevice updates an existing device.
func (h *Handler) UpdateDevice(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var req models.UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if err := h.db.UpdateDevice(id, req); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update device"})
		return
	}

	h.db.LogAudit("device.updated", &id, "device updated", r.RemoteAddr)

	device, err := h.db.GetDevice(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get updated device"})
		return
	}

	writeJSON(w, http.StatusOK, device)
}

// DeleteDevice removes a device from the database.
func (h *Handler) DeleteDevice(w http.ResponseWriter, r *http.Request, id string) {
	// Get device first for audit log details.
	device, err := h.db.GetDevice(id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get device"})
		return
	}

	if err := h.db.DeleteDevice(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete device"})
		return
	}

	h.db.LogAudit("device.deleted", &id, fmt.Sprintf("device %q deleted", device.Name), r.RemoteAddr)

	w.WriteHeader(http.StatusNoContent)
}

// RevokeDevice sets a device's status to revoked.
func (h *Handler) RevokeDevice(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.RevokeDevice(id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke device"})
		return
	}

	h.db.LogAudit("device.revoked", &id, "device revoked", r.RemoteAddr)

	device, err := h.db.GetDevice(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get device"})
		return
	}

	writeJSON(w, http.StatusOK, device)
}

// ReactivateDevice sets a device's status back to active.
func (h *Handler) ReactivateDevice(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.ReactivateDevice(id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reactivate device"})
		return
	}

	h.db.LogAudit("device.reactivated", &id, "device reactivated", r.RemoteAddr)

	device, err := h.db.GetDevice(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get device"})
		return
	}

	writeJSON(w, http.StatusOK, device)
}

// createTokenRequest is the JSON payload for creating an enrollment token.
type createTokenRequest struct {
	Label     string `json:"label"`
	ExpiresIn string `json:"expires_in"`
}

// CreateToken creates a new enrollment token.
func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.ExpiresIn == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expires_in is required"})
		return
	}

	duration, err := time.ParseDuration(req.ExpiresIn)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expires_in duration"})
		return
	}

	expiresAt := time.Now().Add(duration)
	token, err := h.db.CreateToken(req.Label, expiresAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
		return
	}

	writeJSON(w, http.StatusCreated, token)
}

// ListTokens returns all enrollment tokens as JSON.
func (h *Handler) ListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.db.ListTokens()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tokens"})
		return
	}

	if tokens == nil {
		tokens = []models.EnrollmentToken{}
	}

	writeJSON(w, http.StatusOK, tokens)
}

// DeleteToken removes an enrollment token by ID.
func (h *Handler) DeleteToken(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.DeleteToken(id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "token not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete token"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

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

// Heartbeat updates the last_seen timestamp for a device identified by fingerprint.
func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Fingerprint == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "fingerprint is required"})
		return
	}

	// Find device by fingerprint.
	devices, err := h.db.ListDevices()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list devices"})
		return
	}

	var deviceID string
	for _, d := range devices {
		if d.Fingerprint == req.Fingerprint {
			deviceID = d.ID
			break
		}
	}

	if deviceID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}

	if err := h.db.UpdateLastSeen(deviceID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update last_seen"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeJSON encodes data as JSON and writes it to the response with the given
// status code. If data is nil, only the status code is written.
func writeJSON(w http.ResponseWriter, status int, data any) {
	if data == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
