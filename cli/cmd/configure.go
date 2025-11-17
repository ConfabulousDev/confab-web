package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure cloud sync settings",
	Long:  `Set backend URL and API key for cloud session sync.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backendURL, _ := cmd.Flags().GetString("backend-url")
		apiKey, _ := cmd.Flags().GetString("api-key")

		// Get current config
		cfg, err := config.GetUploadConfig()
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		// Update fields
		if backendURL != "" {
			cfg.BackendURL = backendURL
		}
		if apiKey != "" {
			cfg.APIKey = apiKey
		}

		// Save config
		if err := config.SaveUploadConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("=== Cloud Configuration Updated ===")
		fmt.Println()
		fmt.Printf("Backend URL: %s\n", cfg.BackendURL)
		if cfg.APIKey != "" {
			fmt.Printf("API Key: %s...%s\n", cfg.APIKey[:8], cfg.APIKey[len(cfg.APIKey)-4:])
			fmt.Println("Status: Cloud sync enabled")
		} else {
			fmt.Println("API Key: (not set)")
			fmt.Println("Status: Cloud sync disabled (no API key)")
		}
		fmt.Println()
		fmt.Println("Cloud sync will take effect on the next session.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configureCmd)

	// Flags for configure command
	configureCmd.Flags().String("backend-url", "", "Backend API URL (e.g., http://localhost:8080)")
	configureCmd.Flags().String("api-key", "", "API key for authentication")
}
