package keys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallKeys_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ssh", "authorized_keys")

	keysContent := "ssh-ed25519 AAAAC3... user@host"
	if err := InstallKeys(keysContent, path); err != nil {
		t.Fatalf("InstallKeys() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(data)
	expected := ManagedHeader + "\n" + keysContent + "\n" + ManagedFooter + "\n"
	if got != expected {
		t.Errorf("file content mismatch\ngot:\n%s\nwant:\n%s", got, expected)
	}

	// Check file permissions.
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestInstallKeys_ExistingWithoutSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")

	manualKey := "ssh-rsa AAAAB3... manual@host\n"
	if err := os.WriteFile(path, []byte(manualKey), 0o600); err != nil {
		t.Fatal(err)
	}

	keysContent := "ssh-ed25519 AAAAC3... managed@host"
	if err := InstallKeys(keysContent, path); err != nil {
		t.Fatalf("InstallKeys() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)

	// Manual key must still be present.
	if !strings.Contains(got, "ssh-rsa AAAAB3... manual@host") {
		t.Error("manual key was lost")
	}

	// Managed section must be present.
	if !strings.Contains(got, ManagedHeader) {
		t.Error("managed header missing")
	}
	if !strings.Contains(got, keysContent) {
		t.Error("managed keys missing")
	}
	if !strings.Contains(got, ManagedFooter) {
		t.Error("managed footer missing")
	}
}

func TestInstallKeys_ExistingWithSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")

	oldContent := "ssh-rsa AAAAB3... manual@host\n" +
		ManagedHeader + "\n" +
		"ssh-ed25519 OLD_KEY old@host\n" +
		ManagedFooter + "\n"

	if err := os.WriteFile(path, []byte(oldContent), 0o600); err != nil {
		t.Fatal(err)
	}

	newKeys := "ssh-ed25519 NEW_KEY new@host"
	if err := InstallKeys(newKeys, path); err != nil {
		t.Fatalf("InstallKeys() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)

	// Old managed key must be gone.
	if strings.Contains(got, "OLD_KEY") {
		t.Error("old managed key should have been replaced")
	}

	// New managed key must be present.
	if !strings.Contains(got, "NEW_KEY") {
		t.Error("new managed key missing")
	}

	// Manual key must still be present.
	if !strings.Contains(got, "ssh-rsa AAAAB3... manual@host") {
		t.Error("manual key was lost")
	}
}

func TestInstallKeys_PreservesManualKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")

	// Manual keys above and below the managed section.
	existingContent := "ssh-rsa ABOVE_KEY above@host\n" +
		ManagedHeader + "\n" +
		"ssh-ed25519 OLD_MANAGED old@host\n" +
		ManagedFooter + "\n" +
		"ssh-rsa BELOW_KEY below@host\n"

	if err := os.WriteFile(path, []byte(existingContent), 0o600); err != nil {
		t.Fatal(err)
	}

	newKeys := "ssh-ed25519 FRESH_KEY fresh@host"
	if err := InstallKeys(newKeys, path); err != nil {
		t.Fatalf("InstallKeys() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)

	if !strings.Contains(got, "ABOVE_KEY") {
		t.Error("key above managed section was lost")
	}
	if !strings.Contains(got, "BELOW_KEY") {
		t.Error("key below managed section was lost")
	}
	if !strings.Contains(got, "FRESH_KEY") {
		t.Error("new managed key missing")
	}
	if strings.Contains(got, "OLD_MANAGED") {
		t.Error("old managed key should have been replaced")
	}
}
