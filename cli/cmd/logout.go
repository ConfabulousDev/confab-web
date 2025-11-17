package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear API key and disable cloud sync",
	Long:  `Removes the stored API key and disables cloud sync.`,
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, args []string) error {
	// Get current config
	cfg, err := config.GetUploadConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Check if already logged out
	if cfg.APIKey == "" {
		fmt.Println("Already logged out. No API key found.")
		return nil
	}

	// Clear API key
	cfg.APIKey = ""

	// Save config
	if err := config.SaveUploadConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✓ Logged out successfully")
	fmt.Println()
	fmt.Println("API key removed. Cloud sync is now disabled.")
	fmt.Println()
	fmt.Println("To login again, run:")
	fmt.Println("  confab login")

	return nil
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
