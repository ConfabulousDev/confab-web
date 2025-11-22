package cmd

import (
	"fmt"
	"os"

	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/redactor"
	"github.com/spf13/cobra"
)

var redactionCmd = &cobra.Command{
	Use:   "redaction",
	Short: "Manage sensitive data redaction",
	Long: `Manage redaction of sensitive data (API keys, passwords, secrets) before uploading to the cloud.

Redaction is configured via ~/.confab/redaction.json and can be enabled/disabled
by renaming the file (redaction.json = enabled, redaction.json.disabled = disabled).

Users can edit the redaction.json file directly to add custom patterns or modify
the default patterns.`,
}

var redactionEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable redaction",
	Long:  `Enable redaction by activating the redaction configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running redaction enable command")

		// Check if disabled config exists
		if _, err := os.Stat(redactor.GetDisabledConfigPath()); os.IsNotExist(err) {
			// No disabled config - need to initialize
			fmt.Println("No redaction config found. Initializing with default patterns...")
			if err := redactor.InitializeDefaultConfig(); err != nil {
				logger.Error("Failed to initialize redaction config: %v", err)
				return fmt.Errorf("failed to initialize redaction config: %w", err)
			}
			fmt.Println("✓ Created default redaction config at:", redactor.GetDisabledConfigPath())
			fmt.Println()
		}

		// Enable redaction
		if err := redactor.Enable(); err != nil {
			logger.Error("Failed to enable redaction: %v", err)
			return fmt.Errorf("failed to enable redaction: %w", err)
		}

		fmt.Println("✓ Redaction enabled")
		fmt.Println()
		fmt.Println("Sensitive data will now be redacted before upload.")
		fmt.Println("Config file:", redactor.GetConfigPath())
		fmt.Println()
		fmt.Println("To customize patterns, edit the config file directly:")
		fmt.Printf("  vim %s\n", redactor.GetConfigPath())
		fmt.Println()

		return nil
	},
}

var redactionDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable redaction",
	Long:  `Disable redaction by deactivating the redaction configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running redaction disable command")

		if err := redactor.Disable(); err != nil {
			logger.Error("Failed to disable redaction: %v", err)
			return fmt.Errorf("failed to disable redaction: %w", err)
		}

		fmt.Println("✓ Redaction disabled")
		fmt.Println()
		fmt.Println("Data will be uploaded without redaction.")
		fmt.Println("To re-enable, run: confab redaction enable")
		fmt.Println()

		return nil
	},
}

var redactionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show redaction status",
	Long:  `Display current redaction configuration and status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running redaction status command")

		fmt.Println("=== Redaction Status ===")
		fmt.Println()

		enabled := redactor.IsEnabled()
		if enabled {
			fmt.Println("Status: ✓ Enabled")
			fmt.Println("Config: ", redactor.GetConfigPath())
			fmt.Println()

			// Load and display config
			cfg, err := redactor.LoadConfig()
			if err != nil {
				logger.Error("Failed to load redaction config: %v", err)
				fmt.Println("Error: Failed to load configuration")
				fmt.Printf("  %v\n", err)
				return nil
			}

			fmt.Printf("Patterns: %d configured\n", len(cfg.Patterns))
			fmt.Println()

			// Show pattern summary
			if len(cfg.Patterns) > 0 {
				fmt.Println("Pattern Types:")
				typeCounts := make(map[string]int)
				for _, p := range cfg.Patterns {
					typeCounts[p.Type]++
				}
				for ptype, count := range typeCounts {
					fmt.Printf("  - %s: %d pattern(s)\n", ptype, count)
				}
				fmt.Println()
			}

			fmt.Println("To customize patterns:")
			fmt.Printf("  vim %s\n", redactor.GetConfigPath())
			fmt.Println()
			fmt.Println("To disable redaction:")
			fmt.Println("  confab redaction disable")

		} else {
			fmt.Println("Status: ✗ Disabled")
			fmt.Println()

			// Check if disabled config exists
			if _, err := os.Stat(redactor.GetDisabledConfigPath()); err == nil {
				fmt.Println("Config: ", redactor.GetDisabledConfigPath())
				fmt.Println()
				fmt.Println("To enable redaction:")
				fmt.Println("  confab redaction enable")
			} else {
				fmt.Println("No redaction config found.")
				fmt.Println()
				fmt.Println("To initialize and enable redaction:")
				fmt.Println("  confab redaction enable")
			}
		}

		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(redactionCmd)
	redactionCmd.AddCommand(redactionEnableCmd)
	redactionCmd.AddCommand(redactionDisableCmd)
	redactionCmd.AddCommand(redactionStatusCmd)
}
