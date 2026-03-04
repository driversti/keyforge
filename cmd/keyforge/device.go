package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// device represents the JSON structure returned by the API.
type device struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	PublicKey    string   `json:"public_key"`
	Fingerprint  string   `json:"fingerprint"`
	AcceptsSSH   bool     `json:"accepts_ssh"`
	Tags         []string `json:"tags"`
	Status       string   `json:"status"`
	RegisteredAt string   `json:"registered_at"`
}

// apiRequest makes an HTTP request to the KeyForge server using the global
// serverURL and apiKey variables.
func apiRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := strings.TrimRight(serverURL, "/") + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, path, err)
	}
	return resp, nil
}

// resolveDeviceID fetches the device list and returns the ID for the device
// with the given name.
func resolveDeviceID(name string) (string, error) {
	resp, err := apiRequest(http.MethodGet, "/api/v1/devices", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to list devices: HTTP %d", resp.StatusCode)
	}

	var devices []device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return "", fmt.Errorf("decode device list: %w", err)
	}

	for _, d := range devices {
		if d.Name == name {
			return d.ID, nil
		}
	}
	return "", fmt.Errorf("device %q not found", name)
}

func newDeviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "device",
		Short: "Manage devices",
	}

	cmd.AddCommand(newDeviceListCmd())
	cmd.AddCommand(newDeviceAddCmd())
	cmd.AddCommand(newDeviceRevokeCmd())
	cmd.AddCommand(newDeviceReactivateCmd())
	cmd.AddCommand(newDeviceDeleteCmd())

	return cmd
}

func newDeviceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest(http.MethodGet, "/api/v1/devices", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			var devices []device
			if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATUS\tSSH\tFINGERPRINT\tREGISTERED")
			for _, d := range devices {
				fp := d.Fingerprint
				if len(fp) > 16 {
					fp = fp[:16] + "..."
				}
				ssh := "no"
				if d.AcceptsSSH {
					ssh = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", d.Name, d.Status, ssh, fp, d.RegisteredAt)
			}
			return w.Flush()
		},
	}
}

func newDeviceAddCmd() *cobra.Command {
	var (
		name      string
		key       string
		acceptSSH bool
		tags      string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register a new device",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || key == "" {
				return fmt.Errorf("--name and --key are required")
			}

			reqBody := map[string]any{
				"name":       name,
				"public_key": key,
				"accepts_ssh": acceptSSH,
			}
			if tags != "" {
				reqBody["tags"] = strings.Split(tags, ",")
			}

			data, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("marshal request: %w", err)
			}

			resp, err := apiRequest(http.MethodPost, "/api/v1/devices", bytes.NewReader(data))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			fmt.Printf("Device '%s' added successfully\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name (required)")
	cmd.Flags().StringVar(&key, "key", "", "SSH public key (required)")
	cmd.Flags().BoolVar(&acceptSSH, "accept-ssh", false, "Whether the device accepts SSH connections")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags")

	return cmd
}

func newDeviceRevokeCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a device",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			id, err := resolveDeviceID(name)
			if err != nil {
				return err
			}

			resp, err := apiRequest(http.MethodPost, "/api/v1/devices/"+id+"/revoke", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			fmt.Printf("Device '%s' revoked successfully\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name (required)")
	return cmd
}

func newDeviceReactivateCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "reactivate",
		Short: "Reactivate a revoked device",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			id, err := resolveDeviceID(name)
			if err != nil {
				return err
			}

			resp, err := apiRequest(http.MethodPost, "/api/v1/devices/"+id+"/reactivate", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			fmt.Printf("Device '%s' reactivated successfully\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name (required)")
	return cmd
}

func newDeviceDeleteCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a device",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			id, err := resolveDeviceID(name)
			if err != nil {
				return err
			}

			resp, err := apiRequest(http.MethodDelete, "/api/v1/devices/"+id, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			fmt.Printf("Device '%s' deleted successfully\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name (required)")
	return cmd
}
