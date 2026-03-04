package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/models"
)

// Handler holds the database dependency for all API handlers.
type Handler struct {
	db *db.DB
}

// NewHandler creates a new Handler with the given database.
func NewHandler(database *db.DB) *Handler {
	return &Handler{db: database}
}

// HealthCheck returns a simple health status.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetAuthorizedKeys returns all active public keys as plain text, one per line.
func (h *Handler) GetAuthorizedKeys(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetActivePublicKeys()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get keys"})
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	for _, dev := range devices {
		fmt.Fprintln(w, dev.PublicKey)
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
func (h *Handler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var req models.CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
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
