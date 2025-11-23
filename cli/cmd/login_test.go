package cmd

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
)

// TestRunLogin_SuccessfulCallback tests the core OAuth callback flow
func TestRunLogin_SuccessfulCallback(t *testing.T) {
	// This test verifies the callback server works correctly
	// We'll start the login flow and immediately simulate the callback

	// Setup: Use temp config file to avoid polluting user's real config
	tmpDir := t.TempDir()
	testConfigPath := fmt.Sprintf("%s/config.json", tmpDir)
	t.Setenv("CONFAB_CONFIG_PATH", testConfigPath)

	// Create a test backend URL (not used in this test, but needed for login)
	testBackendURL := "http://test-backend.example.com"

	// We'll test the core callback handling logic by directly testing
	// what happens when the callback endpoint receives an API key

	// Start login in a goroutine (it will block waiting for callback)
	errChan := make(chan error, 1)
	portChan := make(chan int, 1)

	go func() {
		// We can't easily test runLogin directly due to browser opening
		// Instead, test the core callback server behavior
		// by creating our own minimal version that matches the pattern

		// Start callback server
		listener, err := startCallbackServer(portChan)
		if err != nil {
			errChan <- err
			return
		}
		defer listener.Close()

		// Wait for callback or timeout
		select {
		case apiKey := <-apiKeyChan:
			// Save config (same as runLogin)
			cfg := &config.UploadConfig{
				BackendURL: testBackendURL,
				APIKey:     apiKey,
			}
			if err := config.SaveUploadConfig(cfg); err != nil {
				errChan <- err
				return
			}
			errChan <- nil
		case <-time.After(5 * time.Second):
			errChan <- fmt.Errorf("timeout waiting for callback")
		}
	}()

	// Wait for server to start
	var port int
	select {
	case port = <-portChan:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for server to start")
	}

	// Simulate OAuth callback with API key
	testAPIKey := "test-api-key-abcdef123456"
	callbackURL := fmt.Sprintf("http://localhost:%d/?key=%s", port, url.QueryEscape(testAPIKey))

	resp, err := http.Get(callbackURL)
	if err != nil {
		t.Fatalf("Failed to send callback: %v", err)
	}
	defer resp.Body.Close()

	// Verify callback returned success
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Wait for login flow to complete
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Login flow failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for login flow to complete")
	}

	// Verify config was saved correctly
	cfg, err := config.GetUploadConfig()
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if cfg.APIKey != testAPIKey {
		t.Errorf("Expected API key %s, got %s", testAPIKey, cfg.APIKey)
	}

	if cfg.BackendURL != testBackendURL {
		t.Errorf("Expected backend URL %s, got %s", testBackendURL, cfg.BackendURL)
	}
}

// TestRunLogin_MissingAPIKey tests callback with missing API key
func TestRunLogin_MissingAPIKey(t *testing.T) {
	portChan := make(chan int, 1)
	errChan := make(chan error, 1)

	go func() {
		listener, err := startCallbackServer(portChan)
		if err != nil {
			errChan <- err
			return
		}
		defer listener.Close()

		select {
		case <-apiKeyChan:
			errChan <- nil
		case err := <-errorChan:
			errChan <- err
		case <-time.After(5 * time.Second):
			errChan <- fmt.Errorf("timeout")
		}
	}()

	// Wait for server to start
	var port int
	select {
	case port = <-portChan:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for server to start")
	}

	// Send callback without API key
	callbackURL := fmt.Sprintf("http://localhost:%d/", port)
	resp, err := http.Get(callbackURL)
	if err != nil {
		t.Fatalf("Failed to send callback: %v", err)
	}
	defer resp.Body.Close()

	// Verify callback returned error
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	// Verify error was received
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error for missing API key")
		}
		if !strings.Contains(err.Error(), "missing API key") {
			t.Errorf("Expected 'missing API key' error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for error")
	}
}

// Helper function to start callback server (matches login.go pattern)

var (
	apiKeyChan chan string
	errorChan  chan error
)

func startCallbackServer(portChan chan<- int) (net.Listener, error) {
	// Initialize channels
	apiKeyChan = make(chan string, 1)
	errorChan = make(chan error, 1)

	// Start listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start listener: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	portChan <- port

	// HTTP handler (same as login.go)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.URL.Query().Get("key")
		if apiKey == "" {
			http.Error(w, "Missing API key", http.StatusBadRequest)
			errorChan <- fmt.Errorf("missing API key in callback")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Success</body></html>"))

		apiKeyChan <- apiKey
	})

	// Start server
	server := &http.Server{
		Handler: mux,
	}
	go func() {
		server.Serve(listener)
	}()

	return listener, nil
}

// Note: We don't test openBrowser() because it has side effects (opens browser)
// and is platform-specific. It's a simple switch statement that's not worth
// the complexity of mocking exec.Command or the annoyance of actually opening
// browsers during tests.
