package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	var (
		githubUser string
		filePath   string
		name       string
		acceptSSH  bool
		tags       []string
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import SSH public keys from GitHub or a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if githubUser == "" && filePath == "" {
				return fmt.Errorf("specify --github <username> or --file <path>")
			}
			if githubUser != "" && filePath != "" {
				return fmt.Errorf("use either --github or --file, not both")
			}

			if githubUser != "" {
				return importFromGitHub(githubUser, acceptSSH, tags)
			}
			return importFromFile(filePath, name, acceptSSH, tags)
		},
	}

	cmd.Flags().StringVar(&githubUser, "github", "", "GitHub username to import keys from")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to SSH public key file")
	cmd.Flags().StringVar(&name, "name", "", "Device name (required with --file)")
	cmd.Flags().BoolVar(&acceptSSH, "accept-ssh", false, "Mark imported devices as SSH-accepting")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags for imported devices (comma-separated)")

	return cmd
}

func importFromGitHub(username string, acceptSSH bool, tags []string) error {
	url := fmt.Sprintf("https://github.com/%s.keys", username)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetch GitHub keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub returned status %d (user %q may not exist)", resp.StatusCode, username)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	keys := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(keys) == 0 || (len(keys) == 1 && keys[0] == "") {
		fmt.Printf("No public keys found for GitHub user %q.\n", username)
		return nil
	}

	imported := 0
	for i, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		deviceName := fmt.Sprintf("%s-github-%d", username, i+1)
		if len(keys) == 1 {
			deviceName = fmt.Sprintf("%s-github", username)
		}

		err := registerDevice(deviceName, key, acceptSSH, tags)
		if err != nil {
			fmt.Printf("  [skip] %s: %s\n", deviceName, err)
			continue
		}
		fmt.Printf("  [ok]   %s imported\n", deviceName)
		imported++
	}

	fmt.Printf("\nImported %d of %d keys from GitHub user %q.\n", imported, len(keys), username)
	return nil
}

func importFromFile(path, name string, acceptSSH bool, tags []string) error {
	if name == "" {
		// Default name from filename: ~/.ssh/id_ed25519.pub -> id_ed25519
		base := filepath.Base(path)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read key file: %w", err)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return fmt.Errorf("key file is empty")
	}

	if err := registerDevice(name, key, acceptSSH, tags); err != nil {
		return err
	}

	fmt.Printf("Imported key from %s as device %q.\n", path, name)
	return nil
}

func registerDevice(name, publicKey string, acceptSSH bool, tags []string) error {
	payload := map[string]any{
		"name":        name,
		"public_key":  publicKey,
		"accepts_ssh": acceptSSH,
	}
	if len(tags) > 0 {
		payload["tags"] = tags
	}

	body, _ := json.Marshal(payload)
	resp, err := apiRequest("POST", "/api/v1/devices", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("already registered")
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}
