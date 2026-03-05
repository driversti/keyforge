package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// DefaultCachePath returns the default cache file path.
// Root users: /var/cache/keyforge/authorized_keys.cache
// Regular users: ~/.cache/keyforge/authorized_keys.cache
func DefaultCachePath() (string, error) {
	if runtime.GOOS != "windows" && os.Getuid() == 0 {
		return "/var/cache/keyforge/authorized_keys.cache", nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".cache", "keyforge", "authorized_keys.cache"), nil
}

// WriteCache writes the keys content to the cache file.
func WriteCache(cachePath string, content string) error {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	return os.WriteFile(cachePath, []byte(content), 0o600)
}

// ReadCache reads the cached keys. Returns empty string and no error if cache doesn't exist.
func ReadCache(cachePath string) (string, time.Time, error) {
	info, err := os.Stat(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", time.Time{}, nil
		}
		return "", time.Time{}, fmt.Errorf("stat cache: %w", err)
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read cache: %w", err)
	}

	return string(data), info.ModTime(), nil
}
