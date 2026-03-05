package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/driversti/keyforge/internal/keys"
)

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

// parseCronSchedule converts a human-friendly interval like "15m" or "1h"
// into a cron schedule expression.
func parseCronSchedule(interval string) (string, error) {
	interval = strings.TrimSpace(interval)
	if len(interval) < 2 {
		return "", fmt.Errorf("invalid interval %q: must be like 15m or 1h", interval)
	}

	unit := interval[len(interval)-1]
	valueStr := interval[:len(interval)-1]

	value, err := strconv.Atoi(valueStr)
	if err != nil || value <= 0 {
		return "", fmt.Errorf("invalid interval %q: numeric part must be a positive integer", interval)
	}

	switch unit {
	case 'm':
		if value > 59 {
			return "", fmt.Errorf("invalid interval %q: minutes must be 1-59", interval)
		}
		return fmt.Sprintf("*/%d * * * *", value), nil
	case 'h':
		if value > 23 {
			return "", fmt.Errorf("invalid interval %q: hours must be 1-23", interval)
		}
		return fmt.Sprintf("0 */%d * * *", value), nil
	default:
		return "", fmt.Errorf("invalid interval %q: unit must be 'm' (minutes) or 'h' (hours)", interval)
	}
}

// setupCron installs or updates a crontab entry for periodic key sync.
func setupCron(interval string) error {
	schedule, err := parseCronSchedule(interval)
	if err != nil {
		return err
	}

	// Get the absolute path to the keyforge binary.
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	cronLine := fmt.Sprintf("%s %s keys --install --server %s", schedule, binaryPath, serverURL)

	// Read existing crontab.
	existingCrontab := ""
	out, err := exec.Command("crontab", "-l").Output()
	if err == nil {
		existingCrontab = string(out)
	}
	// If crontab -l fails (no crontab), we start fresh.

	// Check if a keyforge entry already exists and replace it.
	lines := strings.Split(existingCrontab, "\n")
	var newLines []string
	replaced := false

	for _, line := range lines {
		if strings.Contains(line, "keyforge keys --install") {
			newLines = append(newLines, cronLine)
			replaced = true
		} else {
			newLines = append(newLines, line)
		}
	}

	if !replaced {
		// Remove trailing empty line before appending.
		for len(newLines) > 0 && newLines[len(newLines)-1] == "" {
			newLines = newLines[:len(newLines)-1]
		}
		newLines = append(newLines, cronLine)
	}

	newCrontab := strings.Join(newLines, "\n") + "\n"

	// Write back via crontab -.
	installCmd := exec.Command("crontab", "-")
	installCmd.Stdin = strings.NewReader(newCrontab)
	if output, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install crontab: %w: %s", err, string(output))
	}

	fmt.Printf("Cron job installed: %s\n", cronLine)
	return nil
}
