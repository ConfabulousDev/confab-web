package cmd

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/config"
	confabhttp "github.com/santaclaude2025/confab/pkg/http"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/utils"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show confab status",
	Long:  `Displays hook installation status and cloud authentication status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running status command")

		fmt.Println("=== Confab: Status ===")
		fmt.Println()

		// Check sync hooks installation
		hooksInstalled, err := config.IsSyncHooksInstalled()
		if err != nil {
			logger.Error("Failed to check hook status: %v", err)
			return fmt.Errorf("failed to check hook status: %w", err)
		}

		logger.Info("Sync hooks installed: %v", hooksInstalled)
		if hooksInstalled {
			fmt.Println("Sync Hooks: ✓ Installed")
		} else {
			fmt.Println("Sync Hooks: ✗ Not installed")
			fmt.Println("Run 'confab init' to install sync hooks.")
		}

		fmt.Println()

		// Check cloud sync status
		cfg, err := config.GetUploadConfig()
		if err != nil {
			logger.Error("Failed to get cloud config: %v", err)
			fmt.Println("Cloud Sync: ✗ Configuration error")
		} else {
			fmt.Println("Cloud Sync:")
			if cfg.APIKey != "" {
				fmt.Printf("  Backend: %s\n", cfg.BackendURL)

				// Validate API key
				fmt.Print("  Validating API key... ")
				if err := validateAPIKey(cfg.BackendURL, cfg.APIKey); err != nil {
					logger.Error("API key validation failed: %v", err)
					fmt.Println("✗ Invalid")
					fmt.Printf("  Error: %v\n", err)
					fmt.Println("  Run 'confab login' to re-authenticate")
				} else {
					logger.Info("API key is valid")
					fmt.Println("✓ Valid")
					fmt.Println("  Status: ✓ Authenticated and ready")
				}
			} else {
				fmt.Println("  Status: ✗ Not configured")
				fmt.Println("  Run 'confab login' to authenticate")
			}
		}

		fmt.Println()

		return nil
	},
}

// validateAPIKey checks if the API key is valid by calling the backend
func validateAPIKey(backendURL, apiKey string) error {
	// Create a temporary config for the HTTP client
	cfg := &config.UploadConfig{
		BackendURL: backendURL,
		APIKey:     apiKey,
	}

	client := confabhttp.NewClient(cfg, utils.DefaultHTTPTimeout)
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
	rootCmd.AddCommand(statusCmd)
}
