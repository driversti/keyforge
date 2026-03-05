package db

import (
	"regexp"
	"testing"
	"time"
)

func TestCreateQuickEnroll(t *testing.T) {
	db := newTestDB(t)

	expires := time.Now().Add(1 * time.Hour).UTC()
	token, err := db.CreateQuickEnroll("my-laptop", true, "5m", expires)
	if err != nil {
		t.Fatalf("CreateQuickEnroll: %v", err)
	}

	if token.ID == "" {
		t.Error("expected ID to be non-empty")
	}
	if token.Token == "" {
		t.Error("expected Token value to be non-empty")
	}
	if token.Label != "quick-enroll" {
		t.Errorf("expected label 'quick-enroll', got %q", token.Label)
	}

	// Verify code is exactly 8 digits.
	matched, _ := regexp.MatchString(`^\d{8}$`, token.Code)
	if !matched {
		t.Errorf("expected 8-digit numeric code, got %q", token.Code)
	}

	if token.DeviceName != "my-laptop" {
		t.Errorf("expected device_name 'my-laptop', got %q", token.DeviceName)
	}
	if !token.AcceptSSH {
		t.Error("expected AcceptSSH to be true")
	}
	if token.SyncInterval != "5m" {
		t.Errorf("expected sync_interval '5m', got %q", token.SyncInterval)
	}
	if token.Used {
		t.Error("expected Used to be false")
	}
}

func TestGetTokenByCode(t *testing.T) {
	db := newTestDB(t)

	expires := time.Now().Add(1 * time.Hour).UTC()
	created, err := db.CreateQuickEnroll("round-trip-device", false, "10m", expires)
	if err != nil {
		t.Fatalf("CreateQuickEnroll: %v", err)
	}

	got, err := db.GetTokenByCode(created.Code)
	if err != nil {
		t.Fatalf("GetTokenByCode: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, got.ID)
	}
	if got.Token != created.Token {
		t.Errorf("expected token %q, got %q", created.Token, got.Token)
	}
	if got.Code != created.Code {
		t.Errorf("expected code %q, got %q", created.Code, got.Code)
	}
	if got.DeviceName != "round-trip-device" {
		t.Errorf("expected device_name 'round-trip-device', got %q", got.DeviceName)
	}
	if got.AcceptSSH != false {
		t.Error("expected AcceptSSH to be false")
	}
	if got.SyncInterval != "10m" {
		t.Errorf("expected sync_interval '10m', got %q", got.SyncInterval)
	}
}

func TestGetTokenByCode_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := db.GetTokenByCode("99999999")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateQuickEnroll_UniqueCode(t *testing.T) {
	db := newTestDB(t)

	expires := time.Now().Add(1 * time.Hour).UTC()
	codes := make(map[string]bool)

	for i := 0; i < 20; i++ {
		token, err := db.CreateQuickEnroll("device", false, "", expires)
		if err != nil {
			t.Fatalf("CreateQuickEnroll #%d: %v", i, err)
		}
		if codes[token.Code] {
			t.Fatalf("duplicate code %q at iteration %d", token.Code, i)
		}
		codes[token.Code] = true
	}

	if len(codes) != 20 {
		t.Errorf("expected 20 unique codes, got %d", len(codes))
	}
}
