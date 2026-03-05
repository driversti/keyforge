package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"driversti.dev/keyforge/internal/db"
	"driversti.dev/keyforge/internal/keys"
	"driversti.dev/keyforge/internal/models"
	"driversti.dev/keyforge/internal/server"
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
	srv, err := server.New(database, testAPIKey, "http://localhost:9315")
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
	const testKeyLaptop = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKbda9fDvF5RsoqRdX4EqZREGdC0qaS4LGb+rGuyQeEN test@laptop"
	const testKeyServer = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFqAG13g7sbzvqitxQpTElf3QC7Izo/qTqYvsxEaqgB3 root@server"

	var firstDeviceID string
	t.Run("create first device", func(t *testing.T) {
		body := `{"name":"test-laptop","public_key":"` + testKeyLaptop + `","accepts_ssh":false,"tags":["test"]}`
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
		body := `{"name":"test-server","public_key":"` + testKeyServer + `","accepts_ssh":true,"tags":["linux"]}`
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

		if !strings.Contains(body, "test@laptop") {
			t.Fatal("expected authorized_keys to contain test@laptop key")
		}
		if !strings.Contains(body, "root@server") {
			t.Fatal("expected authorized_keys to contain root@server key")
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

		if strings.Contains(body, "test@laptop") {
			t.Fatal("expected authorized_keys to NOT contain test@laptop key after revoke")
		}
		if !strings.Contains(body, "root@server") {
			t.Fatal("expected authorized_keys to still contain root@server key")
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

	// i. Web UI pages require auth — GET / without session → redirect to /login
	t.Run("web UI root redirects to login", func(t *testing.T) {
		// Disable redirects to capture the 303.
		noRedirectClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp := doRequest(t, noRedirectClient, "GET", ts.URL+"/", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("expected 303 redirect for /, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/login" {
			t.Fatalf("expected redirect to /login, got %q", loc)
		}
	})

	// j. Web UI pages load with API key query param
	t.Run("web UI root page with key param", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/?key="+testAPIKey, "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for /?key=..., got %d", resp.StatusCode)
		}
	})

	t.Run("web UI authorized-keys page with key param", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/authorized-keys?key="+testAPIKey, "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for /authorized-keys?key=..., got %d", resp.StatusCode)
		}
	})

	// k. Login page is accessible without auth.
	t.Run("login page accessible", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/login", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for /login, got %d", resp.StatusCode)
		}
	})
}

func TestIntegration_EnrollmentFlow(t *testing.T) {
	// Unique SSH keys for this test (different from FullWorkflow keys).
	const enrolledKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHUzjCL5Mf1GGjEV5wIXfLGb85kP8cBHdAqh2dqf9HEe enrolled@device"
	const reusedKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMr9Q7pGXdBrFJFc3gv3FPwXlRJ7Rq7BGvhwfWVbKJ5Y reuse@device"
	const expiredKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJqMG5RvPaGpKsh3LDjBg85qjDbxFnxP7fGCvPqHbKRZ expired@device"
	const noAuthKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDeWGCvPy3FJfvKFVUqDZnLwzKfPegdbYJVeMXihlhb6 noauth@device"

	// 1. Create in-memory DB and server.
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	defer database.Close()

	srv, err := server.New(database, testAPIKey, "http://localhost:9315")
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}

	// 2. Start httptest server.
	ts := httptest.NewServer(srv)
	defer ts.Close()

	client := ts.Client()

	// a. Create enrollment token.
	var tokenValue string
	t.Run("create enrollment token", func(t *testing.T) {
		body := `{"label":"test-token","expires_in":"1h"}`
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/tokens", body, true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
		}

		var token models.EnrollmentToken
		if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if token.Token == "" {
			t.Fatal("expected token value to be set")
		}
		if token.Label != "test-token" {
			t.Fatalf("expected label %q, got %q", "test-token", token.Label)
		}
		tokenValue = token.Token
	})

	// b. Enroll device with token (no API key).
	t.Run("enroll device with token", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"enrolled-device","public_key":"%s","accepts_ssh":false,"enrollment_token":"%s"}`, enrolledKey, tokenValue)
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices", body, false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
		}

		var device models.Device
		if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if device.Name != "enrolled-device" {
			t.Fatalf("expected name %q, got %q", "enrolled-device", device.Name)
		}
	})

	// c. Verify device in list.
	t.Run("verify device in list", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/devices", "", true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var devices []models.Device
		if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		found := false
		for _, d := range devices {
			if d.Name == "enrolled-device" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected enrolled-device to appear in device list")
		}
	})

	// d. Token is burned.
	t.Run("token is burned", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/tokens", "", true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var tokens []models.EnrollmentToken
		if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		found := false
		for _, tok := range tokens {
			if tok.Token == tokenValue {
				found = true
				if !tok.Used {
					t.Fatal("expected token to be marked as used")
				}
				break
			}
		}
		if !found {
			t.Fatal("expected to find the enrollment token in the list")
		}
	})

	// e. Reuse token fails.
	t.Run("reuse token fails", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"reuse-device","public_key":"%s","accepts_ssh":false,"enrollment_token":"%s"}`, reusedKey, tokenValue)
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices", body, false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 401, got %d: %s", resp.StatusCode, string(b))
		}
	})

	// f. Expired token fails.
	t.Run("expired token fails", func(t *testing.T) {
		// Create a token that is already expired.
		body := `{"label":"expired-token","expires_in":"-1s"}`
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/tokens", body, true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201 creating expired token, got %d: %s", resp.StatusCode, string(b))
		}

		var expiredToken models.EnrollmentToken
		if err := json.NewDecoder(resp.Body).Decode(&expiredToken); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		// Try to enroll with the expired token.
		enrollBody := fmt.Sprintf(`{"name":"expired-device","public_key":"%s","accepts_ssh":false,"enrollment_token":"%s"}`, expiredKey, expiredToken.Token)
		enrollResp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices", enrollBody, false)
		defer enrollResp.Body.Close()

		if enrollResp.StatusCode != http.StatusUnauthorized {
			b, _ := io.ReadAll(enrollResp.Body)
			t.Fatalf("expected 401 for expired token, got %d: %s", enrollResp.StatusCode, string(b))
		}
	})

	// g. No auth fails.
	t.Run("no auth fails", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"noauth-device","public_key":"%s","accepts_ssh":false}`, noAuthKey)
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices", body, false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 401, got %d: %s", resp.StatusCode, string(b))
		}
	})
}

func TestIntegration_AuditLogAPI(t *testing.T) {
	const auditKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKbda9fDvF5RsoqRdX4EqZREGdC0qaS4LGb+rGuyQeEN audit@test"

	// 1. Create in-memory DB and server.
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	defer database.Close()

	srv, err := server.New(database, testAPIKey, "http://localhost:9315")
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}

	ts := httptest.NewServer(srv)
	defer ts.Close()

	client := ts.Client()

	// 2. Create a device to generate an audit entry.
	t.Run("create device for audit", func(t *testing.T) {
		body := `{"name":"audit-device","public_key":"` + auditKey + `","accepts_ssh":false,"tags":["audit-test"]}`
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices", body, true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
		}
	})

	// 3. GET /api/v1/audit with auth — verify entries exist.
	t.Run("list audit entries", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/audit", "", true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result struct {
			Entries []map[string]any `json:"entries"`
			Total   int              `json:"total"`
			Limit   int              `json:"limit"`
			Offset  int              `json:"offset"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		if len(result.Entries) < 1 {
			t.Fatal("expected at least 1 audit entry")
		}
		if result.Total < 1 {
			t.Fatalf("expected total >= 1, got %d", result.Total)
		}
		if result.Limit != 50 {
			t.Fatalf("expected default limit 50, got %d", result.Limit)
		}
		if result.Offset != 0 {
			t.Fatalf("expected default offset 0, got %d", result.Offset)
		}
	})

	// 4. Pagination: ?limit=1 returns exactly 1 entry.
	t.Run("audit pagination limit", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/audit?limit=1&offset=0", "", true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result struct {
			Entries []map[string]any `json:"entries"`
			Total   int              `json:"total"`
			Limit   int              `json:"limit"`
			Offset  int              `json:"offset"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Fatalf("expected exactly 1 entry with limit=1, got %d", len(result.Entries))
		}
		if result.Limit != 1 {
			t.Fatalf("expected limit 1, got %d", result.Limit)
		}
		if result.Offset != 0 {
			t.Fatalf("expected offset 0, got %d", result.Offset)
		}
	})

	// 5. Unauthorized access — GET /api/v1/audit without auth → 401.
	t.Run("audit unauthorized", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/audit", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestIntegration_QuickEnrollFlow(t *testing.T) {
	const serverURL = "http://localhost:9315"

	// 1. Create in-memory DB and server.
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	defer database.Close()

	srv, err := server.New(database, testAPIKey, serverURL)
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}

	// 2. Create a quick enroll token directly via the DB.
	expiresAt := time.Now().Add(1 * time.Hour)
	token, err := database.CreateQuickEnroll("my-laptop", true, "5m", expiresAt)
	if err != nil {
		t.Fatalf("creating quick enroll token: %v", err)
	}

	// 3. GET /e/{code} with Accept: text/html → HTML response.
	t.Run("browser gets HTML page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/e/"+token.Code, nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if !strings.Contains(body, token.DeviceName) {
			t.Fatalf("expected HTML to contain device name %q", token.DeviceName)
		}
		expectedCurl := fmt.Sprintf("curl -sSL %s/e/%s | sh", serverURL, token.Code)
		if !strings.Contains(body, expectedCurl) {
			t.Fatalf("expected HTML to contain curl command %q", expectedCurl)
		}
	})

	// 4. GET /e/{code} without Accept header → shell script.
	t.Run("curl gets shell script", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/e/"+token.Code, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if !strings.Contains(body, fmt.Sprintf("NAME=%q", token.DeviceName)) {
			t.Fatal("expected script to contain NAME variable")
		}
		if !strings.Contains(body, fmt.Sprintf("TOKEN=%q", token.Token)) {
			t.Fatal("expected script to contain TOKEN variable")
		}
		if !strings.Contains(body, "SERVER_URL=") {
			t.Fatal("expected script to contain SERVER_URL variable")
		}
		if !strings.Contains(body, `ACCEPT_SSH="true"`) {
			t.Fatal("expected script to contain ACCEPT_SSH=\"true\"")
		}
	})

	// 5. GET /e/9999 → 404 not found.
	t.Run("unknown code returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/e/9999", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	// 6. Burn the token, then GET /e/{code} → 410 Gone.
	t.Run("burned token returns 410", func(t *testing.T) {
		_, err := database.ValidateAndBurnToken(token.Token)
		if err != nil {
			t.Fatalf("burning token: %v", err)
		}

		req := httptest.NewRequest("GET", "/e/"+token.Code, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusGone {
			t.Fatalf("expected 410, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestIntegration_KeysCacheFallback(t *testing.T) {
	const cacheTestKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKbda9fDvF5RsoqRdX4EqZREGdC0qaS4LGb+rGuyQeEN cache@test"

	// 1. Create in-memory DB and server.
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	defer database.Close()

	srv, err := server.New(database, testAPIKey, "http://localhost:9315")
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}

	ts := httptest.NewServer(srv)
	defer ts.Close()

	client := ts.Client()

	// 2. Create a device via POST /api/v1/devices with API key auth.
	t.Run("create device", func(t *testing.T) {
		body := `{"name":"cache-device","public_key":"` + cacheTestKey + `","accepts_ssh":false,"tags":["cache-test"]}`
		resp := doRequest(t, client, "POST", ts.URL+"/api/v1/devices", body, true)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
		}
	})

	// 3. Fetch keys via GET /api/v1/authorized_keys — verify the device's public key is present.
	var keysContent string
	t.Run("fetch authorized keys", func(t *testing.T) {
		resp := doRequest(t, client, "GET", ts.URL+"/api/v1/authorized_keys", "", false)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading body: %v", err)
		}
		keysContent = string(b)

		if !strings.Contains(keysContent, "cache@test") {
			t.Fatal("expected authorized_keys to contain cache@test key")
		}
	})

	// 4. Write the keys to a temp cache file via keys.WriteCache.
	cachePath := filepath.Join(t.TempDir(), "authorized_keys.cache")
	t.Run("write cache", func(t *testing.T) {
		if err := keys.WriteCache(cachePath, keysContent); err != nil {
			t.Fatalf("writing cache: %v", err)
		}
	})

	// 5. Read back from cache via keys.ReadCache — verify content matches.
	t.Run("read cache", func(t *testing.T) {
		cached, modTime, err := keys.ReadCache(cachePath)
		if err != nil {
			t.Fatalf("reading cache: %v", err)
		}

		if cached != keysContent {
			t.Fatalf("cache content mismatch:\n  got:  %q\n  want: %q", cached, keysContent)
		}

		// 6. Verify modtime is recent (within last minute).
		if time.Since(modTime) > time.Minute {
			t.Fatalf("cache modtime too old: %v (now: %v)", modTime, time.Now())
		}
	})
}
