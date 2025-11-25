package cmd

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/utils"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with confab cloud backend",
	Long: `Opens browser to authenticate via GitHub OAuth and obtain an API key.

Note: This requires a browser on the same machine as the CLI. For remote/headless
servers, use the web dashboard to create an API key, then run:
  confab configure --api-key <key>

TODO: Implement device code flow for remote/headless scenarios (similar to 'gh auth login').`,
	RunE: runLogin,
}

func runLogin(cmd *cobra.Command, args []string) error {
	logger.Info("Starting login flow")

	backendURL, err := cmd.Flags().GetString("backend-url")
	if err != nil {
		return fmt.Errorf("failed to get backend-url flag: %w", err)
	}
	keyName, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}

	// Default backend URL
	// TODO: Change default to production (https://confab.fly.dev) once stable
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	// Default key name to hostname
	if keyName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			keyName = "CLI"
		} else {
			keyName = hostname
		}
	}

	logger.Debug("Login parameters: backend=%s, keyName=%s", backendURL, keyName)

	fmt.Println("=== Confab Login ===")
	fmt.Println()
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

		// Redirect to backend success page
		successURL := backendURL + "/auth/login/success"
		http.Redirect(w, r, successURL, http.StatusTemporaryRedirect)

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
	fmt.Println()
	fmt.Printf("Backend: %s\n", backendURL)
	fmt.Println()
	fmt.Println("Cloud sync is now enabled.")
	fmt.Println("Future sessions will be automatically uploaded.")

	return nil
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().String("backend-url", "", "Backend API URL (default: http://localhost:8080)")
	loginCmd.Flags().String("name", "", "Name for this API key (default: hostname)")
}
