package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var useSyncDaemon bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize confab (install hook)",
	Long: `Installs hooks in ~/.claude/settings.json to automatically capture sessions.

By default, installs the SessionEnd hook to upload complete sessions at end.
With --sync flag, installs SessionStart + SessionEnd hooks for incremental sync
during active sessions (uploads data every 30 seconds).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running init command (sync=%v)", useSyncDaemon)

		fmt.Println("=== Confab: Initialize ===")
		fmt.Println()

		if useSyncDaemon {
			// Install sync daemon hooks
			fmt.Println("Installing sync daemon hooks (SessionStart + SessionEnd)...")
			if err := config.InstallSyncHooks(); err != nil {
				logger.Error("Failed to install sync hooks: %v", err)
				return fmt.Errorf("failed to install sync hooks: %w", err)
			}

			settingsPath, _ := config.GetSettingsPath()
			logger.Info("Sync hooks installed in %s", settingsPath)
			fmt.Printf("✓ Sync hooks installed in %s\n", settingsPath)
			fmt.Println()

			logger.Info("Initialization complete (sync mode)")
			fmt.Println("=== Initialization Complete ===")
			fmt.Println()
			fmt.Println("Confab will now sync your sessions incrementally during active use.")
			fmt.Println("Data uploads every 30 seconds, with a final sync at session end.")
		} else {
			// Install SessionEnd hook (original behavior)
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
			fmt.Println("Tip: Use 'confab init --sync' for incremental uploads during sessions.")
		}

		fmt.Println()
		fmt.Println("If not logged in yet, run 'confab login' to authenticate.")
		fmt.Println()
		fmt.Println("Tip: Use 'confab setup' next time to do login + init in one step.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&useSyncDaemon, "sync", false, "Use incremental sync daemon instead of session-end upload")
}
