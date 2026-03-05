package db

import (
	"strings"
	"testing"
	"time"
)

func TestCreateToken(t *testing.T) {
	db := newTestDB(t)

	expires := time.Now().Add(1 * time.Hour).UTC()
	token, err := db.CreateToken("test-token", expires)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if token.ID == "" {
		t.Error("expected ID to be non-empty")
	}
	if token.Token == "" {
		t.Error("expected Token value to be non-empty")
	}
	if token.Label != "test-token" {
		t.Errorf("expected label 'test-token', got %q", token.Label)
	}
	if token.Used {
		t.Error("expected Used to be false")
	}
	if token.UsedBy != nil {
		t.Error("expected UsedBy to be nil")
	}

	// Verify it's retrievable.
	got, err := db.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.Token != token.Token {
		t.Errorf("expected token value %q, got %q", token.Token, got.Token)
	}
}

func TestValidateAndBurnToken(t *testing.T) {
	db := newTestDB(t)

	expires := time.Now().Add(1 * time.Hour).UTC()
	created, err := db.CreateToken("burn-me", expires)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	burned, err := db.ValidateAndBurnToken(created.Token)
	if err != nil {
		t.Fatalf("ValidateAndBurnToken: %v", err)
	}

	if !burned.Used {
		t.Error("expected Used to be true after burning")
	}
	if burned.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, burned.ID)
	}

	// Verify it's marked as used in the database.
	got, err := db.GetToken(created.ID)
	if err != nil {
		t.Fatalf("GetToken after burn: %v", err)
	}
	if !got.Used {
		t.Error("expected Used to be true in database")
	}
}

func TestValidateAndBurnToken_Expired(t *testing.T) {
	db := newTestDB(t)

	// Create a token that already expired.
	expires := time.Now().Add(-1 * time.Hour).UTC()
	created, err := db.CreateToken("expired-token", expires)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	_, err = db.ValidateAndBurnToken(created.Token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if !strings.Contains(err.Error(), "token expired") {
		t.Errorf("expected 'token expired' error, got: %v", err)
	}
}

func TestValidateAndBurnToken_AlreadyUsed(t *testing.T) {
	db := newTestDB(t)

	expires := time.Now().Add(1 * time.Hour).UTC()
	created, err := db.CreateToken("use-twice", expires)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// Burn it once.
	_, err = db.ValidateAndBurnToken(created.Token)
	if err != nil {
		t.Fatalf("first ValidateAndBurnToken: %v", err)
	}

	// Try to burn it again.
	_, err = db.ValidateAndBurnToken(created.Token)
	if err == nil {
		t.Fatal("expected error for already-used token, got nil")
	}
	if !strings.Contains(err.Error(), "token already used") {
		t.Errorf("expected 'token already used' error, got: %v", err)
	}
}

func TestValidateAndBurnToken_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := db.ValidateAndBurnToken("nonexistent-token-value")
	if err == nil {
		t.Fatal("expected error for nonexistent token, got nil")
	}
	if !strings.Contains(err.Error(), "token not found") {
		t.Errorf("expected 'token not found' error, got: %v", err)
	}
}

func TestListTokens(t *testing.T) {
	db := newTestDB(t)

	// Start with empty list.
	tokens, err := db.ListTokens()
	if err != nil {
		t.Fatalf("ListTokens (empty): %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}

	// Create two tokens.
	expires := time.Now().Add(1 * time.Hour).UTC()
	_, err = db.CreateToken("token-1", expires)
	if err != nil {
		t.Fatalf("CreateToken(token-1): %v", err)
	}
	_, err = db.CreateToken("token-2", expires)
	if err != nil {
		t.Fatalf("CreateToken(token-2): %v", err)
	}

	tokens, err = db.ListTokens()
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestDeleteToken(t *testing.T) {
	db := newTestDB(t)

	expires := time.Now().Add(1 * time.Hour).UTC()
	created, err := db.CreateToken("delete-me", expires)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if err := db.DeleteToken(created.ID); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	_, err = db.GetToken(created.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// Deleting again should return ErrNotFound.
	err = db.DeleteToken(created.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for second delete, got %v", err)
	}
}
