package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Manage cloud sync configuration",
	Long:  `Configure cloud backend for syncing Claude Code sessions across devices.`,
}

var cloudConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure cloud sync settings",
	Long:  `Set backend URL and API key for cloud session sync.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backendURL, _ := cmd.Flags().GetString("backend-url")
		apiKey, _ := cmd.Flags().GetString("api-key")
		enabled, _ := cmd.Flags().GetBool("enable")
		disabled, _ := cmd.Flags().GetBool("disable")

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
		if enabled {
			cfg.Enabled = true
		}
		if disabled {
			cfg.Enabled = false
		}

		// Save config
		if err := config.SaveUploadConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("=== Cloud Configuration Updated ===")
		fmt.Println()
		fmt.Printf("Enabled: %v\n", cfg.Enabled)
		fmt.Printf("Backend URL: %s\n", cfg.BackendURL)
		if cfg.APIKey != "" {
			fmt.Printf("API Key: %s...%s\n", cfg.APIKey[:8], cfg.APIKey[len(cfg.APIKey)-4:])
		} else {
			fmt.Println("API Key: (not set)")
		}
		fmt.Println()
		fmt.Println("Cloud sync will take effect on the next session.")

		return nil
	},
}

var cloudStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cloud sync status",
	Long:  `Display current cloud sync configuration and status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.GetUploadConfig()
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		fmt.Println("=== Cloud Sync Status ===")
		fmt.Println()
		fmt.Printf("Enabled: %v\n", cfg.Enabled)
		fmt.Printf("Backend URL: %s\n", cfg.BackendURL)
		if cfg.APIKey != "" {
			fmt.Printf("API Key: %s...%s\n", cfg.APIKey[:8], cfg.APIKey[len(cfg.APIKey)-4:])
		} else {
			fmt.Println("API Key: (not set)")
		}

		if !cfg.Enabled {
			fmt.Println()
			fmt.Println("To enable cloud sync, run:")
			fmt.Println("  confab cloud configure --enable --backend-url <url> --api-key <key>")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cloudCmd)
	cloudCmd.AddCommand(cloudConfigureCmd)
	cloudCmd.AddCommand(cloudStatusCmd)

	// Flags for configure command
	cloudConfigureCmd.Flags().String("backend-url", "", "Backend API URL (e.g., http://localhost:8080)")
	cloudConfigureCmd.Flags().String("api-key", "", "API key for authentication")
	cloudConfigureCmd.Flags().Bool("enable", false, "Enable cloud sync")
	cloudConfigureCmd.Flags().Bool("disable", false, "Disable cloud sync")
}
