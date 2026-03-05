package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/driversti/keyforge/internal/keys"
)

func newEnrollCmd() *cobra.Command {
	var (
		name       string
		token      string
		acceptsSSH bool
		keyPath    string
	)

	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "Enroll this device with a KeyForge server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Determine key path (default: ~/.ssh/id_ed25519).
			if keyPath == "" {
				keyPath = keys.DefaultKeyPath()
			}

			// 2. Generate key if needed.
			fmt.Printf("Checking for SSH key at %s...\n", keyPath)
			if err := keys.GenerateED25519Key(keyPath); err != nil {
				return fmt.Errorf("generate key: %w", err)
			}

			// 3. Read public key.
			pubKey, err := keys.ReadPublicKey(keyPath + ".pub")
			if err != nil {
				return fmt.Errorf("read public key: %w", err)
			}

			// 4. Register with server.
			body := map[string]any{
				"name":             name,
				"public_key":       pubKey,
				"accepts_ssh":      acceptsSSH,
				"enrollment_token": token,
			}
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshal request: %w", err)
			}

			resp, err := apiRequest("POST", "/api/v1/devices", strings.NewReader(string(jsonBody)))
			if err != nil {
				return fmt.Errorf("register device: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("registration failed (%d): %s", resp.StatusCode, string(b))
			}

			var device map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			fmt.Printf("✓ Device %q enrolled successfully\n", name)
			fmt.Printf("  Fingerprint: %s\n", device["fingerprint"])

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name (required)")
	cmd.Flags().StringVar(&token, "token", "", "Enrollment token (required)")
	cmd.Flags().BoolVar(&acceptsSSH, "accept-ssh", false, "This device accepts SSH connections")
	cmd.Flags().StringVar(&keyPath, "key", "", "Path to SSH key (default: ~/.ssh/id_ed25519)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("token")

	return cmd
}
