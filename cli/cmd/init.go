package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize confab (install hook)",
	Long:  `Installs the SessionEnd hook in ~/.claude/settings.json to automatically capture sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running init command")

		fmt.Println("=== Confab: Initialize ===")
		fmt.Println()

		// Install SessionEnd hook
		fmt.Println("Installing SessionEnd hook...")
		if err := config.InstallHook(); err != nil {
			logger.Error("Failed to install hook: %v", err)
			return fmt.Errorf("failed to install hook: %w", err)
		}

		settingsPath, _ := config.GetSettingsPath()
		logger.Info("Hook installed in %s", settingsPath)
		fmt.Printf("✓ Hook installed in %s\n", settingsPath)
		fmt.Println()

		logger.Info("Initialization complete")
		fmt.Println("=== Initialization Complete ===")
		fmt.Println()
		fmt.Println("Confab will now automatically capture your Claude Code sessions.")
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
