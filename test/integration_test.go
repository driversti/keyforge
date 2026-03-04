package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/models"
	"github.com/driversti/keyforge/internal/server"
)

const testAPIKey = "test-api-key"

// doRequest is a helper that builds and executes an HTTP request, returning the response.
func doRequest(t *testing.T, client *http.Client, method, url, body string, auth bool) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("executing request %s %s: %v", method, url, err)
	}

	return resp
}

func TestIntegration_FullWorkflow(t *testing.T) {
	// 1. Create in-memory DB.
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	defer database.Close()

	// 2. Create server.
	srv, err := server.New(database, testAPIKey, "http://localhost:8080")
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}

	// 3. Start httptest server.
	ts := httptest.NewServer(srv)
	defer ts.Close()

	client := ts.Client()

	// a. Health check (no auth) — GET /api/v1/health → 200
	t.Run("health check", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/health", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if result["status"] != "ok" {
			t.Fatalf("expected status ok, got %q", result["status"])
		}
	})

	// b. Create device (with auth) — POST /api/v1/devices → 201
	var firstDeviceID string
	t.Run("create first device", func(t *testing.T) {
		body := `{"name":"test-laptop","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey test@laptop","accepts_ssh":false,"tags":["test"]}`
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices", body, true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
		}

		var device models.Device
		if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if device.ID == "" {
			t.Fatal("expected device ID to be set")
		}
		firstDeviceID = device.ID
	})

	// c. Create second device (server) — POST /api/v1/devices → 201
	t.Run("create second device", func(t *testing.T) {
		body := `{"name":"test-server","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIServerKey root@server","accepts_ssh":true,"tags":["linux"]}`
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices", body, true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
		}
	})

	// d. List devices (with auth) — GET /api/v1/devices → JSON array with 2 devices
	t.Run("list devices", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/devices", "", true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var devices []models.Device
		if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(devices) != 2 {
			t.Fatalf("expected 2 devices, got %d", len(devices))
		}
	})

	// e. Get authorized_keys (no auth) — GET /api/v1/authorized_keys → plain text with both keys
	t.Run("authorized keys contains both", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/authorized_keys", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading body: %v", err)
		}
		body := string(b)

		if !strings.Contains(body, "TestKey") {
			t.Fatal("expected authorized_keys to contain TestKey")
		}
		if !strings.Contains(body, "ServerKey") {
			t.Fatal("expected authorized_keys to contain ServerKey")
		}
	})

	// f. Revoke device (with auth) — POST /api/v1/devices/{id}/revoke → 200
	t.Run("revoke first device", func(t *testing.T) {
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices/"+firstDeviceID+"/revoke", "", true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
		}
	})

	// g. Authorized keys after revoke — should NOT contain TestKey, should contain ServerKey
	t.Run("authorized keys after revoke", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/authorized_keys", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading body: %v", err)
		}
		body := string(b)

		if strings.Contains(body, "TestKey") {
			t.Fatal("expected authorized_keys to NOT contain TestKey after revoke")
		}
		if !strings.Contains(body, "ServerKey") {
			t.Fatal("expected authorized_keys to still contain ServerKey")
		}
	})

	// h. Unauthorized access — GET /api/v1/devices without auth header → 401
	t.Run("unauthorized access", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/devices", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	// i. Web UI pages load — GET / → 200, GET /authorized-keys → 200
	t.Run("web UI root page", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for /, got %d", resp.StatusCode)
		}
	})

	t.Run("web UI authorized-keys page", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/authorized-keys", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for /authorized-keys, got %d", resp.StatusCode)
		}
	})
}
