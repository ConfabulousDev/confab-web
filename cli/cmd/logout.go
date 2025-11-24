package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear API key and disable cloud sync",
	Long:  `Removes the stored API key and disables cloud sync.`,
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, args []string) error {
	logger.Info("Starting logout")

	// Get current config
	cfg, err := config.GetUploadConfig()
	if err != nil {
		logger.Error("Failed to get config: %v", err)
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Check if already logged out
	if cfg.APIKey == "" {
		logger.Info("Already logged out, no API key found")
		fmt.Println("Already logged out. No API key found.")
		return nil
	}

	// Clear API key
	cfg.APIKey = ""

	// Save config
	if err := config.SaveUploadConfig(cfg); err != nil {
		logger.Error("Failed to save config: %v", err)
		return fmt.Errorf("failed to save config: %w", err)
	}

	logger.Info("Logout successful, API key removed")
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
