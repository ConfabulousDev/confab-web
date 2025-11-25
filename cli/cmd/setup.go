package cmd

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	confabhttp "github.com/santaclaude2025/confab/pkg/http"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/utils"
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

// doLogin performs the OAuth login flow
func doLogin(backendURL, keyName string) error {
	logger.Debug("Login parameters: backend=%s, keyName=%s", backendURL, keyName)

	fmt.Printf("Backend: %s\n", backendURL)
	fmt.Printf("Key name: %s\n", keyName)
	fmt.Println()

	// Start localhost callback server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Error("Failed to start callback server: %v", err)
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://localhost:%d", port)

	logger.Info("Started callback server on port %d", port)
	fmt.Printf("Starting callback server on port %d...\n", port)

	// Channel to receive API key
	apiKeyChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	// HTTP handler for callback
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.URL.Query().Get("key")
		if apiKey == "" {
			http.Error(w, "Missing API key", http.StatusBadRequest)
			errorChan <- fmt.Errorf("missing API key in callback")
			return
		}

		// Send success page
		html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Confab Login Success</title>
    <style>
        body {
            font-family: system-ui, -apple-system, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .container {
            background: white;
            padding: 3rem;
            border-radius: 1rem;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            text-align: center;
            max-width: 400px;
        }
        h1 { color: #333; margin: 0 0 1rem 0; }
        p { color: #666; margin: 0.5rem 0; }
        .success { color: #10b981; font-size: 3rem; }
    </style>
</head>
<body>
    <div class="container">
        <div class="success">✓</div>
        <h1>Login Successful!</h1>
        <p>Your API key has been saved.</p>
        <p>You can close this window and return to your terminal.</p>
    </div>
    <script>
        setTimeout(() => window.close(), 3000);
    </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))

		apiKeyChan <- apiKey
	})

	// Start server in background
	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  utils.ServerReadTimeout,
		WriteTimeout: utils.ServerWriteTimeout,
		IdleTimeout:  utils.ServerIdleTimeout,
	}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errorChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	// Build authorize URL
	authorizeURL := fmt.Sprintf("%s/auth/cli/authorize?callback=%s&name=%s",
		backendURL,
		url.QueryEscape(callbackURL),
		url.QueryEscape(keyName),
	)

	logger.Debug("Opening browser: %s", authorizeURL)
	fmt.Println("Opening browser for authentication...")
	fmt.Println()
	if err := openBrowser(authorizeURL); err != nil {
		logger.Warn("Failed to open browser: %v", err)
		fmt.Println("Failed to open browser automatically.")
		fmt.Println("Please open this URL manually:")
		fmt.Println()
		fmt.Println(authorizeURL)
		fmt.Println()
	}

	fmt.Printf("Waiting for authentication... (timeout: %v)\n", utils.OAuthFlowTimeout)
	fmt.Println()

	// Wait for API key or timeout
	timeout := time.After(utils.OAuthFlowTimeout)
	var apiKey string

	select {
	case apiKey = <-apiKeyChan:
		logger.Info("Received API key from callback")
	case err := <-errorChan:
		logger.Error("Authentication failed: %v", err)
		server.Close()
		return fmt.Errorf("authentication failed: %w", err)
	case <-timeout:
		logger.Error("Authentication timeout after %v", utils.OAuthFlowTimeout)
		server.Close()
		return fmt.Errorf("authentication timeout after %v", utils.OAuthFlowTimeout)
	}

	// Shutdown callback server
	server.Close()

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
	fmt.Println("✓ Authentication successful!")

	return nil
}

func init() {
	rootCmd.AddCommand(setupCmd)

	setupCmd.Flags().String("backend-url", "", "Backend API URL (default: http://localhost:8080)")
}
