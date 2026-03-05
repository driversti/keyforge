# KeyForge Phase 4 — AuthorizedKeysCommand with Cache Fallback

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `keyforge keys` usable as an sshd `AuthorizedKeysCommand` with a local cache fallback, so SSH authentication works even when the KeyForge server is unreachable.

**Architecture:** The existing `keyforge keys` command already fetches keys from the server and prints them to stdout — exactly what sshd's `AuthorizedKeysCommand` expects. We add a transparent caching layer: on every successful fetch, keys are cached to a local file. If the server is unreachable, the cached keys are returned instead. A new `keyforge setup-sshd` command is NOT included (user configures sshd_config manually). The cache file lives at `~/.cache/keyforge/authorized_keys.cache` (or `/var/cache/keyforge/` when running as root).

**Tech Stack:** Go, os/file I/O for cache, net/http for fetching

---

### Task 1: Key Cache Layer

**Files:**
- Create: `internal/keys/cache.go`
- Create: `internal/keys/cache_test.go`

Implement a simple file-based cache for authorized keys.

**`internal/keys/cache.go`:**
```go
package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// CacheResult holds cached key data and metadata.
type CacheResult struct {
	Keys      string
	CachedAt  time.Time
	FromCache bool
}

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
```

**Tests** (`internal/keys/cache_test.go`):
- `TestWriteAndReadCache` — write keys to a temp file, read back, verify content matches
- `TestReadCache_NotExists` — read from non-existent path, verify empty string and no error
- `TestWriteCache_CreatesDirectories` — write to a deeply nested temp path, verify directories created
- `TestDefaultCachePath` — verify it returns a non-empty string without error

---

### Task 2: Integrate Cache into Keys Command

**Files:**
- Modify: `cmd/keyforge/keys.go` — add cache write on success, cache read on failure

**Changes to `newKeysCmd`:**

Update the `RunE` function to:
1. Try to fetch keys from server
2. On success: print/install keys AND write to cache
3. On failure (network error or non-200): try reading from cache
4. If cache hit: print/install cached keys, print a warning to stderr
5. If cache miss too: return the original error

```go
func newKeysCmd() *cobra.Command {
	var (
		install      bool
		cronInterval string
		noCache      bool
	)

	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Fetch and display/install authorized SSH public keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			keysContent, fromCache, err := fetchKeysWithCache(noCache)
			if err != nil {
				return err
			}

			if fromCache {
				fmt.Fprintln(os.Stderr, "WARNING: Using cached keys (server unreachable)")
			}

			if !install {
				fmt.Print(keysContent)
				return nil
			}

			// Install to ~/.ssh/authorized_keys.
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home directory: %w", err)
			}
			authKeysPath := filepath.Join(homeDir, ".ssh", "authorized_keys")

			if err := keys.InstallKeys(keysContent, authKeysPath); err != nil {
				return fmt.Errorf("install keys: %w", err)
			}
			fmt.Printf("Keys installed to %s\n", authKeysPath)

			if cronInterval != "" {
				if err := setupCron(cronInterval); err != nil {
					return fmt.Errorf("setup cron: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&install, "install", false, "Install keys to ~/.ssh/authorized_keys")
	cmd.Flags().StringVar(&cronInterval, "cron", "", "Set up periodic sync (e.g., 15m, 1h)")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable cache fallback")

	return cmd
}

// fetchKeysWithCache fetches keys from the server. On success, caches them.
// On failure, falls back to the cache. Returns (keys, fromCache, error).
func fetchKeysWithCache(noCache bool) (string, bool, error) {
	url := strings.TrimRight(serverURL, "/") + "/api/v1/authorized_keys"

	// Try fetching from server.
	client := &http.Client{Timeout: 10 * time.Second}
	resp, fetchErr := client.Get(url)

	if fetchErr == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", false, fmt.Errorf("read response: %w", err)
		}

		keysContent := string(body)

		// Cache on success (best-effort, don't fail if cache write fails).
		if !noCache {
			if cachePath, err := keys.DefaultCachePath(); err == nil {
				keys.WriteCache(cachePath, keysContent)
			}
		}

		return keysContent, false, nil
	}

	// Server unreachable or error — try cache fallback.
	if resp != nil {
		resp.Body.Close()
	}

	if noCache {
		if fetchErr != nil {
			return "", false, fmt.Errorf("fetch authorized keys: %w", fetchErr)
		}
		return "", false, fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	cachePath, err := keys.DefaultCachePath()
	if err != nil {
		return "", false, fmt.Errorf("fetch failed and cache unavailable: %w", fetchErr)
	}

	cached, _, cacheErr := keys.ReadCache(cachePath)
	if cacheErr != nil || cached == "" {
		if fetchErr != nil {
			return "", false, fmt.Errorf("server unreachable and no cache available: %w", fetchErr)
		}
		return "", false, fmt.Errorf("server error and no cache available")
	}

	return cached, true, nil
}
```

Add `"time"` to imports if not present. Add the `--no-cache` flag for users who want strict behavior (fail if server is down, don't serve stale keys).

---

### Task 3: Documentation & sshd Setup Guide

**Files:**
- Modify: `README.md` — add AuthorizedKeysCommand section

**Add a new section** to README.md after the "How Keys Are Managed" section:

```markdown
## AuthorizedKeysCommand (Real-Time Key Lookup)

Instead of syncing keys via cron, you can configure sshd to query KeyForge on every login attempt:

### Setup

1. Copy the `keyforge` binary to a system-wide location:
   ```bash
   sudo cp keyforge /usr/local/bin/keyforge
   ```

2. Edit `/etc/ssh/sshd_config`:
   ```
   AuthorizedKeysCommand /usr/local/bin/keyforge keys --server http://keyforge:8080
   AuthorizedKeysCommandUser nobody
   ```

3. Restart sshd:
   ```bash
   sudo systemctl restart sshd
   ```

### How It Works

On every SSH login attempt, sshd runs the `keyforge keys` command. It fetches all active public keys from the KeyForge server and returns them to sshd for authentication.

### Cache Fallback

If the KeyForge server is unreachable, the command automatically returns the last successfully fetched keys from a local cache file (`~/.cache/keyforge/authorized_keys.cache` or `/var/cache/keyforge/authorized_keys.cache` for root). This prevents SSH lockout during server outages.

To disable caching: `keyforge keys --server URL --no-cache`

### Comparison: Cron vs AuthorizedKeysCommand

| | Cron Sync | AuthorizedKeysCommand |
|---|---|---|
| Key freshness | Delay (cron interval) | Real-time |
| Revocation speed | Minutes | Instant |
| Server dependency | Only during sync | Every SSH login |
| Offline behavior | Stale file works | Cache fallback |
| Setup complexity | Simple | Requires sshd_config |
```

---

### Task 4: Integration Test for Cache Fallback

**Files:**
- Modify: `test/integration_test.go` — add cache fallback test

**TestIntegration_KeysCacheFallback:**
1. Start a test server
2. Create a device via API
3. Fetch keys (GET /api/v1/authorized_keys) — verify keys returned
4. Manually write those keys to a temp cache file via `keys.WriteCache`
5. Read back from cache via `keys.ReadCache` — verify matches
6. Verify the round-trip: content written = content read, modtime is recent

This tests the cache layer end-to-end. Testing the actual AuthorizedKeysCommand integration with sshd is out of scope (requires root and sshd running).

---

## Summary

After completing all 4 tasks:
- **Cache layer** (`internal/keys/cache.go`) — write/read cached keys to local file
- **Keys command upgraded** — transparent cache on success, fallback on failure, `--no-cache` flag
- **sshd setup documentation** — step-by-step AuthorizedKeysCommand configuration in README
- **Integration test** for cache round-trip
- **Cache path**: `~/.cache/keyforge/authorized_keys.cache` (user) or `/var/cache/keyforge/authorized_keys.cache` (root)
- **Behavior**: fetch → cache → return keys. If server down → return cached keys + stderr warning. If no cache → error.
