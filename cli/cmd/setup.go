package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	confabhttp "github.com/santaclaude2025/confab/pkg/http"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up confab (login + install hook)",
	Long: `Complete setup for confab in one command.

This command:
1. Authenticates with the cloud backend (if not already logged in)
2. Installs the SessionEnd hook to automatically capture sessions

If you're already authenticated with a valid API key, the login step is skipped.`,
	RunE: runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	logger.Info("Starting setup")

	backendURL, err := cmd.Flags().GetString("backend-url")
	if err != nil {
		return fmt.Errorf("failed to get backend-url flag: %w", err)
	}
	backendURLSpecified := backendURL != ""

	// Default key name to hostname
	var keyName string
	hostname, err := os.Hostname()
	if err != nil {
		keyName = "CLI"
	} else {
		keyName = hostname
	}

	fmt.Println("=== Confab Setup ===")
	fmt.Println()

	// Step 1: Check if already authenticated
	needsLogin := true
	cfg, err := config.GetUploadConfig()
	if err == nil && cfg.APIKey != "" {
		// If no backend URL specified on command line, use the saved one
		if !backendURLSpecified && cfg.BackendURL != "" {
			backendURL = cfg.BackendURL
		}

		// Check if backend URL matches (or no URL specified yet)
		if cfg.BackendURL == backendURL || (!backendURLSpecified && cfg.BackendURL != "") {
			// Config exists with matching backend, verify it works
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
			fmt.Printf("Backend URL changed, need to re-authenticate\n")
			fmt.Println()
		}
	}

	// Apply default backend URL if still not set
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	// Step 2: Login if needed
	if needsLogin {
		fmt.Println("Step 1/2: Authentication")
		fmt.Println()
		if err := doLogin(backendURL, keyName); err != nil {
			return err
		}
		fmt.Println()
	}

	// Step 3: Install hook
	if needsLogin {
		fmt.Println("Step 2/2: Installing hook")
	} else {
		fmt.Println("Installing hook...")
	}
	fmt.Println()

	if err := config.InstallHook(); err != nil {
		logger.Error("Failed to install hook: %v", err)
		return fmt.Errorf("failed to install hook: %w", err)
	}

	settingsPath, _ := config.GetSettingsPath()
	logger.Info("Hook installed in %s", settingsPath)
	fmt.Printf("✓ Hook installed in %s\n", settingsPath)
	fmt.Println()

	// Final message
	fmt.Println("=== Setup Complete ===")
	fmt.Println()
	fmt.Println("Confab will now automatically capture your Claude Code sessions.")
	fmt.Println()
	fmt.Println("Try it out:")
	fmt.Println("  1. Start a new Claude Code session")
	fmt.Println("  2. When you end the session, it will be uploaded automatically")
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

// doLogin performs the device code login flow
func doLogin(backendURL, keyName string) error {
	logger.Debug("Login parameters: backend=%s, keyName=%s", backendURL, keyName)

	fmt.Printf("Backend: %s\n", backendURL)
	fmt.Println()

	// Step 1: Request device code
	deviceCode, err := requestDeviceCode(backendURL, keyName)
	if err != nil {
		logger.Error("Failed to get device code: %v", err)
		return fmt.Errorf("failed to initiate login: %w", err)
	}

	// Display instructions
	fmt.Println("To authenticate, visit:")
	fmt.Printf("  %s\n", deviceCode.VerificationURI)
	fmt.Println()
	fmt.Printf("And enter code: %s\n", deviceCode.UserCode)
	fmt.Println()

	// Try to open browser
	if err := openBrowser(deviceCode.VerificationURI + "?code=" + deviceCode.UserCode); err != nil {
		logger.Debug("Failed to open browser: %v", err)
		// Not an error - user can open manually
	}

	fmt.Printf("Waiting for authorization... (expires in %d seconds)\n", deviceCode.ExpiresIn)

	// Step 2: Poll for token
	pollInterval := time.Duration(deviceCode.Interval) * time.Second
	if pollInterval < 5*time.Second {
		pollInterval = 5 * time.Second
	}

	expiresAt := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	var apiKey string
	for {
		if time.Now().After(expiresAt) {
			return fmt.Errorf("authorization timed out - please try again")
		}

		time.Sleep(pollInterval)

		token, err := pollDeviceToken(backendURL, deviceCode.DeviceCode)
		if err != nil {
			logger.Error("Error polling for token: %v", err)
			return fmt.Errorf("failed to complete authorization: %w", err)
		}

		if token.Error == "authorization_pending" {
			// User hasn't authorized yet, keep polling
			continue
		}

		if token.Error == "slow_down" {
			// We're polling too fast, increase interval
			pollInterval += 5 * time.Second
			continue
		}

		if token.Error != "" {
			return fmt.Errorf("authorization failed: %s", token.Error)
		}

		if token.AccessToken != "" {
			apiKey = token.AccessToken
			break
		}
	}

	// Save configuration
	cfg := &config.UploadConfig{
		BackendURL: backendURL,
		APIKey:     apiKey,
	}

	if err := config.SaveUploadConfig(cfg); err != nil {
		logger.Error("Failed to save config: %v", err)
		return fmt.Errorf("failed to save config: %w", err)
	}

	logger.Info("Login successful, config saved")
	fmt.Println()
	fmt.Println("Authentication successful!")

	return nil
}

func init() {
	rootCmd.AddCommand(setupCmd)

	setupCmd.Flags().String("backend-url", "", "Backend API URL (default: http://localhost:8080)")
}
