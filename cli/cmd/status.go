package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show confab status",
	Long:  `Displays hook installation status and cloud authentication status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Init()
		defer logger.Close()

		logger.Info("Running status command")

		fmt.Println("=== Confab: Status ===")
		fmt.Println()

		// Check hook installation
		hookInstalled, err := config.IsHookInstalled()
		if err != nil {
			logger.Error("Failed to check hook status: %v", err)
			return fmt.Errorf("failed to check hook status: %w", err)
		}

		logger.Info("Hook installed: %v", hookInstalled)
		if hookInstalled {
			fmt.Println("Hook Status: ✓ Installed")
		} else {
			fmt.Println("Hook Status: ✗ Not installed")
			fmt.Println("Run 'confab init' to install the SessionEnd hook.")
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
				fmt.Printf("  API Key: %s...%s\n", cfg.APIKey[:12], cfg.APIKey[len(cfg.APIKey)-4:])

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
	url := backendURL + "/api/v1/auth/validate"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if valid, ok := result["valid"].(bool); !ok || !valid {
		return fmt.Errorf("API key is not valid")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
