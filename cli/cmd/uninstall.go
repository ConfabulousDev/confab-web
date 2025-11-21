package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove SessionEnd hook from Claude Code settings",
	Long:  `Removes the confab SessionEnd hook from ~/.claude/settings.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running uninstall command")

		fmt.Println("=== Confab: Uninstall ===")
		fmt.Println()

		// Remove hook from settings.json
		fmt.Println("Removing SessionEnd hook...")
		if err := config.UninstallHook(); err != nil {
			logger.Error("Failed to uninstall hook: %v", err)
			return fmt.Errorf("failed to uninstall hook: %w", err)
		}

		settingsPath, _ := config.GetSettingsPath()
		logger.Info("Hook removed from %s", settingsPath)
		fmt.Printf("✓ Hook removed from %s\n", settingsPath)
		fmt.Println()
		fmt.Println("Hook removed. Confab will no longer capture sessions.")
		fmt.Println("Your sessions remain accessible in the cloud backend.")
		fmt.Println("To completely remove confab, delete the confab binary and run 'confab logout'.")

		logger.Info("Uninstall complete")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
