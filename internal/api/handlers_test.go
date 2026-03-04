package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/models"
)

func setup(t *testing.T) *Handler {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewHandler(database)
}

func TestHealthCheck(t *testing.T) {
	h := setup(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.HealthCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestGetAuthorizedKeys(t *testing.T) {
	h := setup(t)

	// Create two devices.
	h.db.CreateDevice(models.CreateDeviceRequest{
		Name:      "laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA laptop@example",
	})
	h.db.CreateDevice(models.CreateDeviceRequest{
		Name:      "desktop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5BBBBB desktop@example",
	})

	req := httptest.NewRequest(http.MethodGet, "/authorized_keys", nil)
	rec := httptest.NewRecorder()

	h.GetAuthorizedKeys(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/plain" {
		t.Errorf("expected Content-Type 'text/plain', got %q", ct)
	}

	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte("laptop@example")) {
		t.Error("expected body to contain laptop key")
	}
	if !bytes.Contains([]byte(body), []byte("desktop@example")) {
		t.Error("expected body to contain desktop key")
	}
}

func TestListDevices(t *testing.T) {
	h := setup(t)

	h.db.CreateDevice(models.CreateDeviceRequest{
		Name:      "laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA laptop@example",
	})

	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	rec := httptest.NewRecorder()

	h.ListDevices(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var devices []models.Device
	if err := json.NewDecoder(rec.Body).Decode(&devices); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}
}

func TestListDevices_EmptyReturnsArray(t *testing.T) {
	h := setup(t)

	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	rec := httptest.NewRecorder()

	h.ListDevices(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Must be [] not null.
	body := rec.Body.String()
	if body != "[]\n" {
		t.Errorf("expected empty JSON array, got %q", body)
	}
}

func TestCreateDevice(t *testing.T) {
	h := setup(t)

	payload := `{"name":"laptop","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA laptop@example"}`
	req := httptest.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.CreateDevice(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}

	var device models.Device
	if err := json.NewDecoder(rec.Body).Decode(&device); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if device.Name != "laptop" {
		t.Errorf("expected name 'laptop', got %q", device.Name)
	}
	if device.ID == "" {
		t.Error("expected non-empty device ID")
	}
}

func TestRevokeDevice(t *testing.T) {
	h := setup(t)

	dev, err := h.db.CreateDevice(models.CreateDeviceRequest{
		Name:      "laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA laptop@example",
	})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/devices/"+dev.ID+"/revoke", nil)
	rec := httptest.NewRecorder()

	h.RevokeDevice(rec, req, dev.ID)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var revoked models.Device
	if err := json.NewDecoder(rec.Body).Decode(&revoked); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if revoked.Status != models.StatusRevoked {
		t.Errorf("expected status 'revoked', got %q", revoked.Status)
	}
}
