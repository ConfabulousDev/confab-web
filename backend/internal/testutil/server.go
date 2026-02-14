package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestServer wraps a real HTTP server for integration testing.
// This allows tests to make actual HTTP requests through the full middleware chain.
type TestServer struct {
	Server   *http.Server
	URL      string           // Base URL (e.g., "http://localhost:54321")
	Env      *TestEnvironment // Database and storage
	listener net.Listener
}

// StartTestServer starts a real HTTP server with the given handler.
// The server listens on a random available port and is automatically
// cleaned up when the test completes.
//
// Usage:
//
//	env := testutil.SetupTestEnvironment(t)
//	apiServer := api.NewServer(env.DB, env.Storage, oauthConfig, nil, "")
//	ts := testutil.StartTestServer(t, env, apiServer.SetupRoutes())
func StartTestServer(t *testing.T, env *TestEnvironment, handler http.Handler) *TestServer {
	t.Helper()

	// Set up required environment variables for the server
	setEnvForTest(t, "INSECURE_DEV_MODE", "true") // Allow non-HTTPS cookies in tests

	// Listen on a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Update BACKEND_URL now that we know the port
	os.Setenv("BACKEND_URL", baseURL)

	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ts := &TestServer{
		Server:   server,
		URL:      baseURL,
		Env:      env,
		listener: listener,
	}

	// Start the server in a goroutine
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Only log if not a normal shutdown
			t.Logf("test server error: %v", err)
		}
	}()

	// Wait for server to be ready
	if err := waitForServer(baseURL, 5*time.Second); err != nil {
		t.Fatalf("server failed to start: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Logf("warning: server shutdown error: %v", err)
		}
	})

	return ts
}

// waitForServer polls the server until it's ready or timeout is reached
func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 100 * time.Millisecond}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("server not ready after %v", timeout)
}

// setEnvForTest sets an environment variable and restores it after the test
func setEnvForTest(t *testing.T, key, value string) {
	t.Helper()
	old := os.Getenv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		os.Setenv(key, old)
	})
}

// SetEnvForTest sets an environment variable and restores it after the test.
// This is exported for use by test files that need to configure environment.
func SetEnvForTest(t *testing.T, key, value string) {
	setEnvForTest(t, key, value)
}
