package keys

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndReadCache(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "authorized_keys.cache")
	content := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 user@host\nssh-rsa AAAAB3NzaC1yc2 other@host\n"

	if err := WriteCache(cachePath, content); err != nil {
		t.Fatalf("WriteCache() error = %v", err)
	}

	got, modTime, err := ReadCache(cachePath)
	if err != nil {
		t.Fatalf("ReadCache() error = %v", err)
	}

	if got != content {
		t.Errorf("ReadCache() content = %q, want %q", got, content)
	}

	// Verify modtime is recent (within the last 5 seconds).
	if time.Since(modTime) > 5*time.Second {
		t.Errorf("ReadCache() modTime = %v, expected recent timestamp", modTime)
	}
}

func TestReadCache_NotExists(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "nonexistent", "cache")

	content, modTime, err := ReadCache(cachePath)
	if err != nil {
		t.Fatalf("ReadCache() error = %v, want nil for non-existent file", err)
	}

	if content != "" {
		t.Errorf("ReadCache() content = %q, want empty string", content)
	}

	if !modTime.IsZero() {
		t.Errorf("ReadCache() modTime = %v, want zero time", modTime)
	}
}

func TestWriteCache_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "a", "b", "c", "authorized_keys.cache")
	content := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 user@host\n"

	if err := WriteCache(cachePath, content); err != nil {
		t.Fatalf("WriteCache() error = %v", err)
	}

	// Verify the file exists.
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file not found: %v", err)
	}

	// Verify content is correct.
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(data) != content {
		t.Errorf("cache file content = %q, want %q", string(data), content)
	}
}

func TestDefaultCachePath(t *testing.T) {
	path, err := DefaultCachePath()
	if err != nil {
		t.Fatalf("DefaultCachePath() error = %v", err)
	}

	if path == "" {
		t.Error("DefaultCachePath() returned empty string")
	}
}
