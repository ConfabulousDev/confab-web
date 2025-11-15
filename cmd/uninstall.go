package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove SessionEnd hook from Claude Code settings",
	Long: `Removes the confab SessionEnd hook from ~/.claude/settings.json.
Your database and stored sessions are preserved.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Confab: Uninstall ===")
		fmt.Println()

		// Remove hook from settings.json
		fmt.Println("Removing SessionEnd hook...")
		if err := config.UninstallHook(); err != nil {
			return fmt.Errorf("failed to uninstall hook: %w", err)
		}

		settingsPath, _ := config.GetSettingsPath()
		fmt.Printf("✓ Hook removed from %s\n", settingsPath)
		fmt.Println()
		fmt.Println("Database and sessions preserved at ~/.confab/")
		fmt.Println("To completely remove confab, delete ~/.confab/ and the confab binary.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
