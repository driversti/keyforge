package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

func newKeysCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "keys",
		Short: "Fetch and display all authorized SSH public keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := strings.TrimRight(serverURL, "/") + "/api/v1/authorized_keys"
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("fetch authorized keys: %w", err)
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

			fmt.Print(string(body))
			return nil
		},
	}
}
