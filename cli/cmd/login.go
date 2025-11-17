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
	backendURL, _ := cmd.Flags().GetString("backend-url")
	keyName, _ := cmd.Flags().GetString("name")

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

	fmt.Println("=== Confab Login ===")
	fmt.Println()
	fmt.Printf("Backend: %s\n", backendURL)
	fmt.Printf("Key name: %s\n", keyName)
	fmt.Println()

	// Start localhost callback server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://localhost:%d", port)

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
	server := &http.Server{Handler: mux}
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

	fmt.Println("Opening browser for authentication...")
	fmt.Println()
	if err := openBrowser(authorizeURL); err != nil {
		fmt.Println("Failed to open browser automatically.")
		fmt.Println("Please open this URL manually:")
		fmt.Println()
		fmt.Println(authorizeURL)
		fmt.Println()
	}

	fmt.Println("Waiting for authentication... (timeout: 5 minutes)")
	fmt.Println()

	// Wait for API key or timeout
	timeout := time.After(5 * time.Minute)
	var apiKey string

	select {
	case apiKey = <-apiKeyChan:
		// Success!
	case err := <-errorChan:
		server.Close()
		return fmt.Errorf("authentication failed: %w", err)
	case <-timeout:
		server.Close()
		return fmt.Errorf("authentication timeout after 5 minutes")
	}

	// Shutdown callback server
	server.Close()

	// Save configuration
	cfg := &config.UploadConfig{
		BackendURL: backendURL,
		APIKey:     apiKey,
	}

	if err := config.SaveUploadConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✓ Authentication successful!")
	fmt.Println()
	fmt.Printf("API Key: %s...%s\n", apiKey[:12], apiKey[len(apiKey)-4:])
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
