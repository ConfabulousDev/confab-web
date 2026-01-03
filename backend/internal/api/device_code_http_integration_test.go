package api

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// Device Code HTTP Integration Tests
//
// These tests run against a real HTTP server with the production router.
// Device code endpoints are unauthenticated (public) for CLI auth flow.
// =============================================================================

// setupDeviceCodeTestServer creates a test server for device code tests
func setupDeviceCodeTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()

	testutil.SetEnvForTest(t, "CSRF_SECRET_KEY", "test-csrf-secret-key-32-bytes!!")
	testutil.SetEnvForTest(t, "ALLOWED_ORIGINS", "http://localhost:3000")
	testutil.SetEnvForTest(t, "FRONTEND_URL", "http://localhost:3000")
	testutil.SetEnvForTest(t, "INSECURE_DEV_MODE", "true")

	oauthConfig := auth.OAuthConfig{
		GitHubClientID:     "test-github-client-id",
		GitHubClientSecret: "test-github-client-secret",
		GitHubRedirectURL:  "http://localhost:3000/auth/github/callback",
		GoogleClientID:     "test-google-client-id",
		GoogleClientSecret: "test-google-client-secret",
		GoogleRedirectURL:  "http://localhost:3000/auth/google/callback",
	}

	apiServer := NewServer(env.DB, env.Storage, oauthConfig, nil)
	handler := apiServer.SetupRoutes()

	return testutil.StartTestServer(t, env, handler)
}

// =============================================================================
// POST /auth/device/code - Request device code
// =============================================================================

func TestDeviceCode_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates device code successfully", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		reqBody := auth.DeviceCodeRequest{
			KeyName: "My CLI Key",
		}

		resp, err := client.Post("/auth/device/code", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result auth.DeviceCodeResponse
		testutil.ParseJSON(t, resp, &result)

		if result.DeviceCode == "" {
			t.Error("expected non-empty device_code")
		}
		if len(result.DeviceCode) != 64 {
			t.Errorf("expected device_code length 64, got %d", len(result.DeviceCode))
		}

		if result.UserCode == "" {
			t.Error("expected non-empty user_code")
		}
		if len(result.UserCode) != 9 {
			t.Errorf("expected user_code length 9, got %d", len(result.UserCode))
		}

		if result.ExpiresIn != 300 {
			t.Errorf("expected expires_in 300, got %d", result.ExpiresIn)
		}

		// Verify in database
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM device_codes WHERE device_code = $1",
			result.DeviceCode)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 device code, got %d", count)
		}
	})

	t.Run("creates device code with default key name", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Post("/auth/device/code", auth.DeviceCodeRequest{KeyName: ""})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var keyName string
		row := env.DB.QueryRow(env.Ctx, "SELECT key_name FROM device_codes LIMIT 1")
		if err := row.Scan(&keyName); err != nil {
			t.Fatalf("failed to query: %v", err)
		}
		if keyName != "CLI Key" {
			t.Errorf("expected 'CLI Key', got %s", keyName)
		}
	})

	t.Run("creates device code with empty body", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		// Send empty JSON object instead of a structured request
		resp, err := client.Post("/auth/device/code", map[string]string{})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result auth.DeviceCodeResponse
		testutil.ParseJSON(t, resp, &result)

		if result.DeviceCode == "" {
			t.Error("expected non-empty device_code even with empty body")
		}
	})

	t.Run("generates unique device codes", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		codes := make(map[string]bool)
		userCodes := make(map[string]bool)

		// Create multiple device codes
		for i := 0; i < 10; i++ {
			resp, err := client.Post("/auth/device/code", auth.DeviceCodeRequest{KeyName: "Test"})
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}

			testutil.RequireStatus(t, resp, http.StatusOK)

			var result auth.DeviceCodeResponse
			testutil.ParseJSON(t, resp, &result)
			resp.Body.Close()

			if codes[result.DeviceCode] {
				t.Errorf("duplicate device_code generated: %s", result.DeviceCode)
			}
			codes[result.DeviceCode] = true

			if userCodes[result.UserCode] {
				t.Errorf("duplicate user_code generated: %s", result.UserCode)
			}
			userCodes[result.UserCode] = true
		}
	})
}

// =============================================================================
// POST /auth/device/token - Poll for access token
// =============================================================================

func TestDeviceToken_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns authorization_pending when not yet authorized", func(t *testing.T) {
		env.CleanDB(t)

		deviceCode := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		expiresAt := time.Now().UTC().Add(15 * time.Minute)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "ABCD-1234", "Test Key", expiresAt)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Post("/auth/device/token", auth.DeviceTokenRequest{DeviceCode: deviceCode})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result auth.DeviceTokenResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Error != "authorization_pending" {
			t.Errorf("expected 'authorization_pending', got %s", result.Error)
		}
	})

	t.Run("returns access token when authorized", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		deviceCode := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		expiresAt := time.Now().UTC().Add(15 * time.Minute)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "EFGH-5678", "Test Key", expiresAt)
		testutil.AuthorizeTestDeviceCode(t, env, "EFGH-5678", user.ID)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Post("/auth/device/token", auth.DeviceTokenRequest{DeviceCode: deviceCode})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result auth.DeviceTokenResponse
		testutil.ParseJSON(t, resp, &result)

		if result.AccessToken == "" {
			t.Error("expected access_token")
		}
		if !strings.HasPrefix(result.AccessToken, "cfb_") {
			t.Error("expected cfb_ prefix")
		}
	})

	t.Run("returns expired_token for expired device code", func(t *testing.T) {
		env.CleanDB(t)

		deviceCode := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		expiresAt := time.Now().UTC().Add(-1 * time.Hour)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "WXYZ-9999", "Test Key", expiresAt)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Post("/auth/device/token", auth.DeviceTokenRequest{DeviceCode: deviceCode})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result auth.DeviceTokenResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Error != "expired_token" {
			t.Errorf("expected 'expired_token', got %s", result.Error)
		}
	})

	t.Run("returns invalid_grant for non-existent device code", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Post("/auth/device/token", auth.DeviceTokenRequest{DeviceCode: "nonexistent"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result auth.DeviceTokenResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Error != "invalid_grant" {
			t.Errorf("expected 'invalid_grant', got %s", result.Error)
		}
	})

	t.Run("returns invalid_request for empty device code", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Post("/auth/device/token", auth.DeviceTokenRequest{DeviceCode: ""})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result auth.DeviceTokenResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Error != "invalid_request" {
			t.Errorf("expected 'invalid_request', got %s", result.Error)
		}
	})
}

// =============================================================================
// POST /auth/device/verify - Device code verification (web form)
// =============================================================================

func TestDeviceVerify_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("authorizes device code successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		// Create device code
		deviceCode := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		expiresAt := time.Now().UTC().Add(15 * time.Minute)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "VERI-FY12", "Test Key", expiresAt)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.PostForm("/auth/device/verify", "code=VERI-FY12")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify device code was authorized
		var userID *int64
		var authorizedAt *time.Time
		row := env.DB.QueryRow(env.Ctx,
			"SELECT user_id, authorized_at FROM device_codes WHERE user_code = $1",
			"VERI-FY12")
		if err := row.Scan(&userID, &authorizedAt); err != nil {
			t.Fatalf("failed to query device_codes: %v", err)
		}
		if userID == nil || *userID != user.ID {
			t.Errorf("expected user_id %d, got %v", user.ID, userID)
		}
		if authorizedAt == nil {
			t.Error("expected authorized_at to be set")
		}
	})

	t.Run("normalizes user code format", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		deviceCode := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		expiresAt := time.Now().UTC().Add(15 * time.Minute)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "NORM-ALIZ", "Test Key", expiresAt)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Send code without dash, lowercase - should still work
		resp, err := client.PostForm("/auth/device/verify", "code=normaliz")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify it was authorized
		var userID *int64
		row := env.DB.QueryRow(env.Ctx,
			"SELECT user_id FROM device_codes WHERE user_code = $1",
			"NORM-ALIZ")
		if err := row.Scan(&userID); err != nil {
			t.Fatalf("failed to query device_codes: %v", err)
		}
		if userID == nil {
			t.Error("expected device code to be authorized")
		}
	})

	t.Run("returns error for invalid code", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.PostForm("/auth/device/verify", "code=INVALID1")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("redirects to login when not authenticated", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		resp, err := client.PostForm("/auth/device/verify", "code=TEST-1234")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusTemporaryRedirect)

		location := resp.Header.Get("Location")
		if !strings.Contains(location, "/auth/login") {
			t.Errorf("expected redirect to /auth/login, got %s", location)
		}
	})

	t.Run("redirects to login for expired session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		// Create expired session
		expiredSessionID := "expired-session-id"
		testutil.CreateTestWebSession(t, env, expiredSessionID, user.ID, time.Now().UTC().Add(-1*time.Hour))

		ts := setupDeviceCodeTestServer(t, env)
		// Can't use WithSession because it creates a valid session - manually create client
		client := testutil.NewTestClient(t, ts)

		// Make request with expired session cookie
		resp, err := client.RequestWithHeaders(http.MethodPost, "/auth/device/verify", "code=TEST-1234", map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
			"Cookie":       auth.SessionCookieName + "=" + expiredSessionID,
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusTemporaryRedirect)

		location := resp.Header.Get("Location")
		if !strings.Contains(location, "/auth/login") {
			t.Errorf("expected redirect to /auth/login, got %s", location)
		}
	})
}

// =============================================================================
// Complete device code flow test
// =============================================================================

func TestDeviceCodeFlow_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("complete flow from code request to token", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "flow@example.com", "Flow User")

		ts := setupDeviceCodeTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		// Step 1: Request device code
		codeResp, err := client.Post("/auth/device/code", auth.DeviceCodeRequest{KeyName: "Flow Key"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, codeResp, http.StatusOK)

		var codeResult auth.DeviceCodeResponse
		testutil.ParseJSON(t, codeResp, &codeResult)

		// Step 2: Poll - should get pending
		pendingResp, err := client.Post("/auth/device/token", auth.DeviceTokenRequest{DeviceCode: codeResult.DeviceCode})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, pendingResp, http.StatusBadRequest)

		var pendingResult auth.DeviceTokenResponse
		testutil.ParseJSON(t, pendingResp, &pendingResult)
		if pendingResult.Error != "authorization_pending" {
			t.Errorf("expected authorization_pending, got %s", pendingResult.Error)
		}

		// Step 3: Authorize via database
		testutil.AuthorizeTestDeviceCode(t, env, codeResult.UserCode, user.ID)

		// Step 4: Poll again - should get token
		tokenResp, err := client.Post("/auth/device/token", auth.DeviceTokenRequest{DeviceCode: codeResult.DeviceCode})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, tokenResp, http.StatusOK)

		var tokenResult auth.DeviceTokenResponse
		testutil.ParseJSON(t, tokenResp, &tokenResult)

		if tokenResult.AccessToken == "" {
			t.Error("expected access_token")
		}

		// Verify API key works
		keyHash := auth.HashAPIKey(tokenResult.AccessToken)
		userID, _, _, _, err := env.DB.ValidateAPIKey(env.Ctx, keyHash)
		if err != nil {
			t.Fatalf("failed to validate API key: %v", err)
		}
		if userID != user.ID {
			t.Errorf("expected user_id %d, got %d", user.ID, userID)
		}
	})
}
