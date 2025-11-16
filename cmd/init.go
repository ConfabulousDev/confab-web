package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/db"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize confab (install hook, create database)",
	Long: `Installs the SessionEnd hook in ~/.claude/settings.json and creates
the local SQLite database for storing and querying your Claude Code sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Init()
		defer logger.Close()

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

		// Create database
		fmt.Println("Creating database...")
		database, err := db.Open()
		if err != nil {
			logger.Error("Failed to create database: %v", err)
			return fmt.Errorf("failed to create database: %w", err)
		}
		defer database.Close()

		logger.Info("Database created at %s", database.Path())
		fmt.Printf("✓ Database created at %s\n", database.Path())
		fmt.Println()

		logger.Info("Initialization complete")
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
