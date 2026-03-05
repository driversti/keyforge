package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"driversti.dev/keyforge/internal/auth"
	"driversti.dev/keyforge/internal/db"
	"driversti.dev/keyforge/internal/server"
)

var version = "dev"

var (
	serverURL string
	apiKey    string
)

var rootCmd = &cobra.Command{
	Use:   "keyforge",
	Short: "KeyForge — SSH public key registry",
	Long:  "KeyForge is an SSH public key registry that allows you to store and retrieve SSH public keys via a simple API.",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the KeyForge HTTP server",
	RunE:  runServe,
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:9315", "KeyForge server URL")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	serveCmd.Flags().Int("port", 9315, "Port to listen on")
	serveCmd.Flags().String("data", "./keyforge-data", "Data directory path")
	serveCmd.Flags().String("url", "", "Public URL of this server (shown in web UI, e.g. https://keyforge.example.com)")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(newDeviceCmd())
	rootCmd.AddCommand(newKeysCmd())
	rootCmd.AddCommand(newTokenCmd())
	rootCmd.AddCommand(newEnrollCmd())
	rootCmd.AddCommand(newPushCmd())
	rootCmd.AddCommand(newImportCmd())
}

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	dataDir, _ := cmd.Flags().GetString("data")

	// Ensure data directory exists.
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Open database.
	dsn := filepath.Join(dataDir, "keyforge.db")
	database, err := db.New(dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	// Load or generate API key.
	key, err := loadOrGenerateAPIKey(database)
	if err != nil {
		return fmt.Errorf("api key setup: %w", err)
	}

	publicURL, _ := cmd.Flags().GetString("url")
	if publicURL == "" {
		publicURL = fmt.Sprintf("http://localhost:%d", port)
	}

	srv, err := server.New(database, key, publicURL)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return srv.ListenAndServe(port)
}

// loadOrGenerateAPIKey retrieves the API key from the settings table. If none
// exists, it generates a new one, stores it, and prints it to stdout.
func loadOrGenerateAPIKey(database *db.DB) (string, error) {
	key, err := database.GetSetting("api_key")
	if err == nil {
		return key, nil
	}
	// If the error is not "no rows", it's a real error.
	if !strings.Contains(err.Error(), "sql: no rows") {
		return "", fmt.Errorf("query api_key: %w", err)
	}

	// Generate and store a new key.
	key, err = auth.GenerateAPIKey()
	if err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}

	if err := database.SetSetting("api_key", key); err != nil {
		return "", fmt.Errorf("store api key: %w", err)
	}

	fmt.Println("=== Generated API Key (save this!) ===")
	fmt.Println(key)
	fmt.Println("=======================================")

	return key, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
