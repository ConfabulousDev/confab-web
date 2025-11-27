package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize confab (install sync hooks)",
	Long: `Installs sync hooks in ~/.claude/settings.json for incremental session capture.

Installs SessionStart + SessionEnd hooks that run a background sync daemon
during active sessions (uploads data every 30 seconds).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running init command")

		fmt.Println("=== Confab: Initialize ===")
		fmt.Println()

		// Install sync daemon hooks
		fmt.Println("Installing sync hooks (SessionStart + SessionEnd)...")
		if err := config.InstallSyncHooks(); err != nil {
			logger.Error("Failed to install sync hooks: %v", err)
			return fmt.Errorf("failed to install sync hooks: %w", err)
		}

		settingsPath, _ := config.GetSettingsPath()
		logger.Info("Sync hooks installed in %s", settingsPath)
		fmt.Printf("✓ Sync hooks installed in %s\n", settingsPath)
		fmt.Println()

		logger.Info("Initialization complete")
		fmt.Println("=== Initialization Complete ===")
		fmt.Println()
		fmt.Println("Confab will now sync your sessions incrementally during active use.")
		fmt.Println("Data uploads every 30 seconds, with a final sync at session end.")

		fmt.Println()
		fmt.Println("If not logged in yet, run 'confab login' to authenticate.")
		fmt.Println()
		fmt.Println("Tip: Use 'confab setup' next time to do login + init in one step.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
