package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
)

// TestRequestDeviceCode tests the device code request function
func TestRequestDeviceCode(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/auth/device/code" {
			t.Errorf("Expected /auth/device/code, got %s", r.URL.Path)
		}

		// Parse request
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req["key_name"] != "test-key" {
			t.Errorf("Expected key_name 'test-key', got %s", req["key_name"])
		}

		// Return mock response
		resp := DeviceCodeResponse{
			DeviceCode:      "device-code-123",
			UserCode:        "ABCD-1234",
			VerificationURI: "http://localhost/device",
			ExpiresIn:       900,
			Interval:        5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test
	deviceCode, err := requestDeviceCode(server.URL, "test-key")
	if err != nil {
		t.Fatalf("requestDeviceCode failed: %v", err)
	}

	if deviceCode.DeviceCode != "device-code-123" {
		t.Errorf("Expected device_code 'device-code-123', got %s", deviceCode.DeviceCode)
	}
	if deviceCode.UserCode != "ABCD-1234" {
		t.Errorf("Expected user_code 'ABCD-1234', got %s", deviceCode.UserCode)
	}
	if deviceCode.ExpiresIn != 900 {
		t.Errorf("Expected expires_in 900, got %d", deviceCode.ExpiresIn)
	}
}

// TestPollDeviceToken_Pending tests polling when authorization is pending
func TestPollDeviceToken_Pending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/auth/device/token" {
			t.Errorf("Expected /auth/device/token, got %s", r.URL.Path)
		}

		// Return pending status
		resp := DeviceTokenResponse{
			Error: "authorization_pending",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	token, err := pollDeviceToken(server.URL, "device-code-123")
	if err != nil {
		t.Fatalf("pollDeviceToken failed: %v", err)
	}

	if token.Error != "authorization_pending" {
		t.Errorf("Expected error 'authorization_pending', got %s", token.Error)
	}
	if token.AccessToken != "" {
		t.Errorf("Expected no access_token, got %s", token.AccessToken)
	}
}

// TestPollDeviceToken_Success tests polling when authorization succeeds
func TestPollDeviceToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := DeviceTokenResponse{
			AccessToken: "cfb_test-api-key-123456",
			TokenType:   "Bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	token, err := pollDeviceToken(server.URL, "device-code-123")
	if err != nil {
		t.Fatalf("pollDeviceToken failed: %v", err)
	}

	if token.Error != "" {
		t.Errorf("Expected no error, got %s", token.Error)
	}
	if token.AccessToken != "cfb_test-api-key-123456" {
		t.Errorf("Expected access_token 'cfb_test-api-key-123456', got %s", token.AccessToken)
	}
}

// TestDeviceCodeFlow_Integration tests the full device code flow
func TestDeviceCodeFlow_Integration(t *testing.T) {
	// Setup: Use temp config file
	tmpDir := t.TempDir()
	testConfigPath := fmt.Sprintf("%s/config.json", tmpDir)
	t.Setenv("CONFAB_CONFIG_PATH", testConfigPath)

	// Track request count to simulate progression
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/device/code" {
			resp := DeviceCodeResponse{
				DeviceCode:      "device-code-integration-test",
				UserCode:        "TEST-1234",
				VerificationURI: "http://test/device",
				ExpiresIn:       300,
				Interval:        1, // Fast polling for test
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/auth/device/token" {
			requestCount++
			w.Header().Set("Content-Type", "application/json")

			// First 2 requests: pending, then success
			if requestCount < 3 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "authorization_pending"})
			} else {
				json.NewEncoder(w).Encode(DeviceTokenResponse{
					AccessToken: "cfb_integration-test-key",
					TokenType:   "Bearer",
				})
			}
			return
		}

		t.Errorf("Unexpected request to %s", r.URL.Path)
	}))
	defer server.Close()

	// Run the device code flow
	done := make(chan error, 1)
	go func() {
		// Request device code
		dc, err := requestDeviceCode(server.URL, "test-key")
		if err != nil {
			done <- err
			return
		}

		// Poll for token
		pollInterval := time.Duration(dc.Interval) * time.Second
		expiresAt := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

		for {
			if time.Now().After(expiresAt) {
				done <- fmt.Errorf("timeout")
				return
			}

			time.Sleep(pollInterval)

			token, err := pollDeviceToken(server.URL, dc.DeviceCode)
			if err != nil {
				done <- err
				return
			}

			if token.Error == "authorization_pending" {
				continue
			}

			if token.Error != "" {
				done <- fmt.Errorf("error: %s", token.Error)
				return
			}

			if token.AccessToken != "" {
				// Save config
				cfg := &config.UploadConfig{
					BackendURL: server.URL,
					APIKey:     token.AccessToken,
				}
				if err := config.SaveUploadConfig(cfg); err != nil {
					done <- err
					return
				}
				done <- nil
				return
			}
		}
	}()

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Device code flow failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for device code flow")
	}

	// Verify config was saved
	cfg, err := config.GetUploadConfig()
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if cfg.APIKey != "cfb_integration-test-key" {
		t.Errorf("Expected API key 'cfb_integration-test-key', got %s", cfg.APIKey)
	}
	if cfg.BackendURL != server.URL {
		t.Errorf("Expected backend URL %s, got %s", server.URL, cfg.BackendURL)
	}
}

// TestPollDeviceToken_ServerError tests handling of server errors
func TestPollDeviceToken_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return valid JSON with error field
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "server_error"})
	}))
	defer server.Close()

	token, err := pollDeviceToken(server.URL, "device-code-123")
	if err != nil {
		t.Fatalf("pollDeviceToken should not return network error: %v", err)
	}

	// Server error results in error field being set
	if token.Error != "server_error" {
		t.Errorf("Expected error 'server_error', got %s", token.Error)
	}
	if token.AccessToken != "" {
		t.Errorf("Expected no access_token on error, got %s", token.AccessToken)
	}
}

// TestRequestDeviceCode_ServerError tests handling of server errors during code request
func TestRequestDeviceCode_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	}))
	defer server.Close()

	_, err := requestDeviceCode(server.URL, "test-key")
	if err == nil {
		t.Error("Expected error for server error, got nil")
	}
}

// Note: We don't test openBrowser() because it has side effects (opens browser)
// and is platform-specific. It's a simple switch statement that's not worth
// the complexity of mocking exec.Command or the annoyance of actually opening
// browsers during tests.
