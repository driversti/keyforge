package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push authorized keys to servers",
		Long:  "Push authorized keys to a target server via SSH",
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				fmt.Println("Usage: keyforge push --target user@host")
				fmt.Println()
				fmt.Println("Or run this on each server manually:")
				fmt.Printf("  curl -s %s/api/v1/authorized_keys | tee -a ~/.ssh/authorized_keys\n", serverURL)
				return nil
			}

			// Fetch keys from server.
			resp, err := apiRequest("GET", "/api/v1/authorized_keys", nil)
			if err != nil {
				return fmt.Errorf("fetch keys: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("read response: %w", err)
			}
			keysContent := strings.TrimSpace(string(body))

			// Build a shell script that installs the managed section on the remote host.
			script := fmt.Sprintf(`
set -e
mkdir -p ~/.ssh
chmod 700 ~/.ssh
KEYS=%q
FILE=~/.ssh/authorized_keys
HEADER="# --- KeyForge Managed Keys (DO NOT EDIT) ---"
FOOTER="# --- End KeyForge Managed Keys ---"

if [ -f "$FILE" ]; then
    # Remove existing managed section
    sed -i.bak "/$HEADER/,/$FOOTER/d" "$FILE"
    rm -f "$FILE.bak"
fi

# Append managed section
printf "\n%%s\n%%s\n%%s\n" "$HEADER" "$KEYS" "$FOOTER" >> "$FILE"
chmod 600 "$FILE"
echo "Keys installed on $(hostname)"
`, keysContent)

			sshCmd := exec.Command("ssh", target, "bash", "-s")
			sshCmd.Stdin = strings.NewReader(script)
			sshCmd.Stdout = os.Stdout
			sshCmd.Stderr = os.Stderr

			return sshCmd.Run()
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "SSH target (e.g., root@192.168.1.50)")
	return cmd
}
