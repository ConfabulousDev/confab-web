package cmd

import (
	"fmt"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	confabhttp "github.com/santaclaude2025/confab/pkg/http"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up confab (login + install hooks)",
	Long: `Complete setup for confab in one command.

This command:
1. Authenticates with the cloud backend (if not already logged in)
2. Installs sync hooks (SessionStart + SessionEnd) for incremental upload

If you're already authenticated with a valid API key, the login step is skipped.`,
	RunE: runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	logger.Info("Starting setup")

	backendURL, err := cmd.Flags().GetString("backend-url")
	if err != nil {
		return fmt.Errorf("failed to get backend-url flag: %w", err)
	}

	// Apply default backend URL early so we can compare against saved config
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	fmt.Println("=== Confab Setup ===")
	fmt.Println()

	// Check if already authenticated
	needsLogin := true
	cfg, err := config.GetUploadConfig()
	if err == nil && cfg.APIKey != "" {
		// Check if backend URL matches
		if cfg.BackendURL == backendURL {
			fmt.Println("Checking existing authentication...")
			if err := verifyAPIKey(cfg); err == nil {
				logger.Info("Existing API key is valid, skipping login")
				fmt.Println("✓ Already authenticated")
				fmt.Println()
				needsLogin = false
			} else {
				logger.Info("Existing API key is invalid: %v", err)
				fmt.Println("✗ Existing credentials invalid, need to re-authenticate")
				fmt.Println()
			}
		} else {
			logger.Info("Backend URL changed from %s to %s, need to re-login", cfg.BackendURL, backendURL)
			fmt.Println("Backend URL changed, need to re-authenticate")
			fmt.Println()
		}
	}

	// Login if needed
	if needsLogin {
		fmt.Println("Step 1/2: Authentication")
		fmt.Println()
		if err := doDeviceLogin(backendURL, defaultKeyName()); err != nil {
			return err
		}
		fmt.Println()
	}

	// Install sync hooks
	if needsLogin {
		fmt.Println("Step 2/2: Installing sync hooks")
	} else {
		fmt.Println("Installing sync hooks...")
	}
	fmt.Println()

	if err := config.InstallSyncHooks(); err != nil {
		logger.Error("Failed to install sync hooks: %v", err)
		return fmt.Errorf("failed to install sync hooks: %w", err)
	}

	settingsPath, _ := config.GetSettingsPath()
	logger.Info("Sync hooks installed in %s", settingsPath)
	fmt.Printf("✓ Sync hooks installed in %s\n", settingsPath)
	fmt.Println()

	fmt.Println("=== Setup Complete ===")
	fmt.Println()
	fmt.Println("Confab will now sync your sessions incrementally during active use.")
	fmt.Println("Data uploads every 30 seconds, with a final sync at session end.")
	fmt.Println()
	fmt.Println("Try it out:")
	fmt.Println("  1. Start a new Claude Code session")
	fmt.Println("  2. Your session data will sync in the background")
	fmt.Println("  3. Run 'confab status' to check your setup")

	return nil
}

// verifyAPIKey checks if the API key works by calling the validate endpoint
func verifyAPIKey(cfg *config.UploadConfig) error {
	client := confabhttp.NewClient(cfg, 5*time.Second)

	var result map[string]interface{}
	if err := client.Get("/api/v1/auth/validate", &result); err != nil {
		return err
	}

	if valid, ok := result["valid"].(bool); !ok || !valid {
		return fmt.Errorf("api key is not valid")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(setupCmd)

	setupCmd.Flags().String("backend-url", "", "Backend API URL (default: http://localhost:8080)")
}
