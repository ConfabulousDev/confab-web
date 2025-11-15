package cmd

import (
	"fmt"

	"github.com/santaclaude/confab/pkg/config"
	"github.com/santaclaude/confab/pkg/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize confab (install hook, create database)",
	Long: `Installs the SessionEnd hook in ~/.claude/settings.json and creates
the local SQLite database for storing and querying your Claude Code sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Confab: Initialize ===")
		fmt.Println()

		// Install SessionEnd hook
		fmt.Println("Installing SessionEnd hook...")
		if err := config.InstallHook(); err != nil {
			return fmt.Errorf("failed to install hook: %w", err)
		}

		settingsPath, _ := config.GetSettingsPath()
		fmt.Printf("✓ Hook installed in %s\n", settingsPath)
		fmt.Println()

		// Create database
		fmt.Println("Creating database...")
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		defer database.Close()

		fmt.Printf("✓ Database created at %s\n", database.Path())
		fmt.Println()

		fmt.Println("=== Initialization Complete ===")
		fmt.Println()
		fmt.Println("Confab will now automatically capture your Claude Code sessions.")
		fmt.Println("Run 'confab status' to view captured sessions.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
