package keys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateED25519Key(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test_key")

	if err := GenerateED25519Key(keyPath); err != nil {
		t.Fatalf("GenerateED25519Key() error = %v", err)
	}

	// Verify private key file exists.
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("private key file not found: %v", err)
	}

	// Verify private key permissions are 0600.
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("private key permissions = %o, want 0600", perm)
	}

	// Verify public key file exists.
	if _, err := os.Stat(keyPath + ".pub"); err != nil {
		t.Fatalf("public key file not found: %v", err)
	}

	// Verify public key starts with "ssh-ed25519".
	pubData, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}
	if !strings.HasPrefix(string(pubData), "ssh-ed25519") {
		t.Errorf("public key does not start with ssh-ed25519: %s", string(pubData)[:40])
	}
}

func TestGenerateED25519Key_ExistingKeySkipped(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test_key")

	// Create a dummy file at the key path.
	dummy := []byte("dummy key content")
	if err := os.WriteFile(keyPath, dummy, 0o600); err != nil {
		t.Fatalf("write dummy file: %v", err)
	}

	// GenerateED25519Key should return nil without overwriting.
	if err := GenerateED25519Key(keyPath); err != nil {
		t.Fatalf("GenerateED25519Key() error = %v", err)
	}

	// Verify the file was not overwritten.
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	if string(data) != string(dummy) {
		t.Error("existing key file was overwritten")
	}
}

func TestReadPublicKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test_key")

	if err := GenerateED25519Key(keyPath); err != nil {
		t.Fatalf("GenerateED25519Key() error = %v", err)
	}

	pubKey, err := ReadPublicKey(keyPath + ".pub")
	if err != nil {
		t.Fatalf("ReadPublicKey() error = %v", err)
	}

	if pubKey == "" {
		t.Error("ReadPublicKey() returned empty string")
	}

	if !strings.HasPrefix(pubKey, "ssh-ed25519") {
		t.Errorf("public key does not start with ssh-ed25519: %s", pubKey[:40])
	}
}
