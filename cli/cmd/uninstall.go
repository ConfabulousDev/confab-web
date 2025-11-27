package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove confab hooks from Claude Code settings",
	Long:  `Removes confab sync hooks from ~/.claude/settings.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running uninstall command")

		fmt.Println("=== Confab: Uninstall ===")
		fmt.Println()

		// Remove hooks from settings.json
		fmt.Println("Removing sync hooks...")
		if err := config.UninstallSyncHooks(); err != nil {
			logger.Error("Failed to uninstall hooks: %v", err)
			return fmt.Errorf("failed to uninstall hooks: %w", err)
		}

		settingsPath, _ := config.GetSettingsPath()
		logger.Info("Hooks removed from %s", settingsPath)
		fmt.Printf("✓ Hooks removed from %s\n", settingsPath)
		fmt.Println()
		fmt.Println("Hooks removed. Confab will no longer sync sessions.")
		fmt.Println("Your sessions remain accessible in the cloud backend.")
		fmt.Println("To completely remove confab, delete the confab binary and run 'confab logout'.")

		logger.Info("Uninstall complete")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
