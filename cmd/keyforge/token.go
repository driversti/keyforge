package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// enrollmentToken represents the JSON structure returned by the API.
type enrollmentToken struct {
	ID        string  `json:"id"`
	Token     string  `json:"token"`
	Label     string  `json:"label"`
	ExpiresAt string  `json:"expires_at"`
	Used      bool    `json:"used"`
	UsedBy    *string `json:"used_by,omitempty"`
	CreatedAt string  `json:"created_at"`
}

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage enrollment tokens",
	}

	cmd.AddCommand(newTokenCreateCmd())
	cmd.AddCommand(newTokenListCmd())
	cmd.AddCommand(newTokenDeleteCmd())

	return cmd
}

func newTokenCreateCmd() *cobra.Command {
	var (
		label   string
		expires string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new enrollment token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if expires == "" {
				return fmt.Errorf("--expires is required")
			}

			reqBody := map[string]string{
				"label":      label,
				"expires_in": expires,
			}

			data, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("marshal request: %w", err)
			}

			resp, err := apiRequest(http.MethodPost, "/api/v1/tokens", bytes.NewReader(data))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			var token enrollmentToken
			if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			fmt.Printf("Token created successfully.\n")
			fmt.Printf("Token: %s\n", token.Token)
			fmt.Printf("Label: %s\n", token.Label)
			fmt.Printf("Expires: %s\n", token.ExpiresAt)
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Token label")
	cmd.Flags().StringVar(&expires, "expires", "", "Expiry duration (e.g. 1h, 24h, 168h)")

	return cmd
}

func newTokenListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all enrollment tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest(http.MethodGet, "/api/v1/tokens", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			var tokens []enrollmentToken
			if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tLABEL\tUSED\tEXPIRES\tCREATED")
			for _, t := range tokens {
				used := "no"
				if t.Used {
					used = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.ID, t.Label, used, t.ExpiresAt, t.CreatedAt)
			}
			return w.Flush()
		},
	}
}

func newTokenDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an enrollment token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest(http.MethodDelete, "/api/v1/tokens/"+args[0], nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			fmt.Println("Token deleted successfully.")
			return nil
		},
	}
}
