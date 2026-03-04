package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

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
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "KeyForge server URL")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	serveCmd.Flags().Int("port", 8080, "Port to listen on")
	serveCmd.Flags().String("data", "./keyforge-data", "Data directory path")

	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	data, _ := cmd.Flags().GetString("data")
	fmt.Printf("Starting KeyForge server on port %d with data dir %s\n", port, data)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
