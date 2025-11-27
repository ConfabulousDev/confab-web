package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/testutil"
)

// TestHandleDeviceCode_Integration tests device code creation with real database
func TestHandleDeviceCode_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates device code successfully", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := auth.DeviceCodeRequest{
			KeyName: "My CLI Key",
		}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/device/code", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceCode(env.DB, "http://localhost:8080")
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp auth.DeviceCodeResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify response fields
		if resp.DeviceCode == "" {
			t.Error("expected non-empty device_code")
		}
		if len(resp.DeviceCode) != 64 {
			t.Errorf("expected device_code length 64, got %d", len(resp.DeviceCode))
		}

		if resp.UserCode == "" {
			t.Error("expected non-empty user_code")
		}
		// User code format: XXXX-XXXX (9 chars including dash)
		if len(resp.UserCode) != 9 {
			t.Errorf("expected user_code length 9, got %d", len(resp.UserCode))
		}
		if resp.UserCode[4] != '-' {
			t.Errorf("expected user_code format XXXX-XXXX, got %s", resp.UserCode)
		}

		if resp.VerificationURI != "http://localhost:8080/auth/device" {
			t.Errorf("expected verification_uri 'http://localhost:8080/auth/device', got %s", resp.VerificationURI)
		}

		if resp.ExpiresIn != 300 { // 5 minutes (DeviceCodeExpiry)
			t.Errorf("expected expires_in 300, got %d", resp.ExpiresIn)
		}

		if resp.Interval != 5 {
			t.Errorf("expected interval 5, got %d", resp.Interval)
		}

		// Verify device code exists in database
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM device_codes WHERE device_code = $1 AND user_code = $2 AND key_name = $3",
			resp.DeviceCode, resp.UserCode, "My CLI Key")
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query device_codes: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 device code in database, got %d", count)
		}
	})

	t.Run("creates device code with default key name when not provided", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := auth.DeviceCodeRequest{
			KeyName: "", // Empty - should default to "CLI Key"
		}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/device/code", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceCode(env.DB, "http://localhost:8080")
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify default key name in database
		var keyName string
		row := env.DB.QueryRow(env.Ctx, "SELECT key_name FROM device_codes LIMIT 1")
		if err := row.Scan(&keyName); err != nil {
			t.Fatalf("failed to query device_codes: %v", err)
		}
		if keyName != "CLI Key" {
			t.Errorf("expected default key_name 'CLI Key', got %s", keyName)
		}
	})

	t.Run("creates device code with empty body", func(t *testing.T) {
		env.CleanDB(t)

		req := httptest.NewRequest("POST", "/auth/device/code", bytes.NewReader([]byte{}))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceCode(env.DB, "http://localhost:8080")
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp auth.DeviceCodeResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.DeviceCode == "" {
			t.Error("expected non-empty device_code even with empty body")
		}
	})

	t.Run("generates unique device codes", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := auth.DeviceCodeRequest{KeyName: "Test"}
		bodyJSON, _ := json.Marshal(reqBody)

		codes := make(map[string]bool)
		userCodes := make(map[string]bool)

		// Create multiple device codes
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("POST", "/auth/device/code", bytes.NewReader(bodyJSON))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler := auth.HandleDeviceCode(env.DB, "http://localhost:8080")
			handler(w, req)

			testutil.AssertStatus(t, w, http.StatusOK)

			var resp auth.DeviceCodeResponse
			testutil.ParseJSONResponse(t, w, &resp)

			if codes[resp.DeviceCode] {
				t.Errorf("duplicate device_code generated: %s", resp.DeviceCode)
			}
			codes[resp.DeviceCode] = true

			if userCodes[resp.UserCode] {
				t.Errorf("duplicate user_code generated: %s", resp.UserCode)
			}
			userCodes[resp.UserCode] = true
		}
	})
}

// TestHandleDeviceToken_Integration tests device token polling with real database
func TestHandleDeviceToken_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns authorization_pending when not yet authorized", func(t *testing.T) {
		env.CleanDB(t)

		// Create device code with plenty of time buffer
		// Device code must be exactly 64 characters (hex-encoded 32 bytes)
		deviceCode := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		expiresAt := time.Now().UTC().Add(15 * time.Minute)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "ABCD-1234", "Test Key", expiresAt)

		reqBody := auth.DeviceTokenRequest{
			DeviceCode: deviceCode,
		}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/device/token", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceToken(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp auth.DeviceTokenResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Error != "authorization_pending" {
			t.Errorf("expected error 'authorization_pending', got %s", resp.Error)
		}
		if resp.AccessToken != "" {
			t.Errorf("expected no access_token, got %s", resp.AccessToken)
		}
	})

	t.Run("returns access token when authorized", func(t *testing.T) {
		env.CleanDB(t)

		// Create user
		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Create and authorize device code (64 chars)
		deviceCode := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		expiresAt := time.Now().UTC().Add(15 * time.Minute)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "EFGH-5678", "Test Key", expiresAt)
		testutil.AuthorizeTestDeviceCode(t, env, "EFGH-5678", user.ID)

		reqBody := auth.DeviceTokenRequest{
			DeviceCode: deviceCode,
		}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/device/token", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceToken(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp auth.DeviceTokenResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Error != "" {
			t.Errorf("expected no error, got %s", resp.Error)
		}
		if resp.AccessToken == "" {
			t.Error("expected non-empty access_token")
		}
		if !strings.HasPrefix(resp.AccessToken, "cfb_") {
			t.Errorf("expected access_token to start with 'cfb_', got %s", resp.AccessToken[:4])
		}
		if resp.TokenType != "Bearer" {
			t.Errorf("expected token_type 'Bearer', got %s", resp.TokenType)
		}

		// Verify API key was created in database
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND name = $2",
			user.ID, "Test Key")
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query api_keys: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 API key in database, got %d", count)
		}

		// Verify device code was deleted (one-time use)
		row = env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM device_codes WHERE device_code = $1",
			deviceCode)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query device_codes: %v", err)
		}
		if count != 0 {
			t.Error("expected device code to be deleted after token exchange")
		}
	})

	t.Run("returns expired_token for expired device code", func(t *testing.T) {
		env.CleanDB(t)

		// Create expired device code (64 chars)
		deviceCode := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		expiresAt := time.Now().UTC().Add(-1 * time.Hour) // Expired 1 hour ago
		testutil.CreateTestDeviceCode(t, env, deviceCode, "WXYZ-9999", "Test Key", expiresAt)

		reqBody := auth.DeviceTokenRequest{
			DeviceCode: deviceCode,
		}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/device/token", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceToken(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp auth.DeviceTokenResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Error != "expired_token" {
			t.Errorf("expected error 'expired_token', got %s", resp.Error)
		}
	})

	t.Run("returns invalid_grant for non-existent device code", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := auth.DeviceTokenRequest{
			DeviceCode: "non-existent-device-code",
		}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/device/token", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceToken(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp auth.DeviceTokenResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Error != "invalid_grant" {
			t.Errorf("expected error 'invalid_grant', got %s", resp.Error)
		}
	})

	t.Run("returns invalid_request for empty device code", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := auth.DeviceTokenRequest{
			DeviceCode: "",
		}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/device/token", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceToken(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp auth.DeviceTokenResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Error != "invalid_request" {
			t.Errorf("expected error 'invalid_request', got %s", resp.Error)
		}
	})

	t.Run("returns invalid_request for malformed JSON", func(t *testing.T) {
		env.CleanDB(t)

		req := httptest.NewRequest("POST", "/auth/device/token", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceToken(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp auth.DeviceTokenResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Error != "invalid_request" {
			t.Errorf("expected error 'invalid_request', got %s", resp.Error)
		}
	})
}

// TestHandleDeviceVerify_Integration tests device verification with real database
func TestHandleDeviceVerify_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("authorizes device code successfully", func(t *testing.T) {
		env.CleanDB(t)

		// Create user and web session
		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := "test-session-id-123"
		testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(24*time.Hour))

		// Create device code (64 chars)
		deviceCode := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		expiresAt := time.Now().UTC().Add(15 * time.Minute)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "VERI-FY12", "Test Key", expiresAt)

		// Create request with session cookie
		req := httptest.NewRequest("POST", "/auth/device/verify", strings.NewReader("code=VERI-FY12"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{
			Name:  auth.SessionCookieName,
			Value: sessionID,
		})

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceVerify(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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
		sessionID := "test-session-id-456"
		testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(24*time.Hour))

		deviceCode := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		expiresAt := time.Now().UTC().Add(15 * time.Minute)
		testutil.CreateTestDeviceCode(t, env, deviceCode, "NORM-ALIZ", "Test Key", expiresAt)

		// Send code without dash, lowercase
		req := httptest.NewRequest("POST", "/auth/device/verify", strings.NewReader("code=normaliz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{
			Name:  auth.SessionCookieName,
			Value: sessionID,
		})

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceVerify(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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
		sessionID := "test-session-id-789"
		testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(24*time.Hour))

		req := httptest.NewRequest("POST", "/auth/device/verify", strings.NewReader("code=INVALID1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{
			Name:  auth.SessionCookieName,
			Value: sessionID,
		})

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceVerify(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		// Verify error page is returned
		if !strings.Contains(w.Body.String(), "Invalid or expired code") {
			t.Error("expected error message in response")
		}
	})

	t.Run("redirects to login when not authenticated", func(t *testing.T) {
		env.CleanDB(t)

		req := httptest.NewRequest("POST", "/auth/device/verify", strings.NewReader("code=TEST-1234"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// No session cookie

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceVerify(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusTemporaryRedirect)

		location := w.Header().Get("Location")
		if !strings.Contains(location, "/auth/login") {
			t.Errorf("expected redirect to /auth/login, got %s", location)
		}
	})

	t.Run("redirects to login for expired session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := "expired-session-id"
		// Create expired session
		testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(-1*time.Hour))

		req := httptest.NewRequest("POST", "/auth/device/verify", strings.NewReader("code=TEST-1234"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{
			Name:  auth.SessionCookieName,
			Value: sessionID,
		})

		w := httptest.NewRecorder()
		handler := auth.HandleDeviceVerify(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusTemporaryRedirect)

		location := w.Header().Get("Location")
		if !strings.Contains(location, "/auth/login") {
			t.Errorf("expected redirect to /auth/login, got %s", location)
		}
	})
}

// TestDeviceCodeFlow_FullIntegration tests the complete device code flow end-to-end
func TestDeviceCodeFlow_FullIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("complete flow from code request to token", func(t *testing.T) {
		env.CleanDB(t)

		// Create user for authorization
		user := testutil.CreateTestUser(t, env, "flow@example.com", "Flow User")
		sessionID := "flow-session-id"
		testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(24*time.Hour))

		// Step 1: CLI requests device code
		codeReqBody := auth.DeviceCodeRequest{KeyName: "Flow Test Key"}
		codeBodyJSON, _ := json.Marshal(codeReqBody)

		codeReq := httptest.NewRequest("POST", "/auth/device/code", bytes.NewReader(codeBodyJSON))
		codeReq.Header.Set("Content-Type", "application/json")

		codeW := httptest.NewRecorder()
		auth.HandleDeviceCode(env.DB, "http://localhost:8080")(codeW, codeReq)

		testutil.AssertStatus(t, codeW, http.StatusOK)

		var codeResp auth.DeviceCodeResponse
		testutil.ParseJSONResponse(t, codeW, &codeResp)

		// Step 2: CLI polls - should get authorization_pending
		tokenReqBody := auth.DeviceTokenRequest{DeviceCode: codeResp.DeviceCode}
		tokenBodyJSON, _ := json.Marshal(tokenReqBody)

		pendingReq := httptest.NewRequest("POST", "/auth/device/token", bytes.NewReader(tokenBodyJSON))
		pendingReq.Header.Set("Content-Type", "application/json")

		pendingW := httptest.NewRecorder()
		auth.HandleDeviceToken(env.DB)(pendingW, pendingReq)

		testutil.AssertStatus(t, pendingW, http.StatusBadRequest)

		var pendingResp auth.DeviceTokenResponse
		testutil.ParseJSONResponse(t, pendingW, &pendingResp)
		if pendingResp.Error != "authorization_pending" {
			t.Errorf("expected authorization_pending, got %s", pendingResp.Error)
		}

		// Step 3: User verifies device code in browser
		verifyReq := httptest.NewRequest("POST", "/auth/device/verify",
			strings.NewReader("code="+codeResp.UserCode))
		verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		verifyReq.AddCookie(&http.Cookie{
			Name:  auth.SessionCookieName,
			Value: sessionID,
		})

		verifyW := httptest.NewRecorder()
		auth.HandleDeviceVerify(env.DB)(verifyW, verifyReq)

		testutil.AssertStatus(t, verifyW, http.StatusOK)

		// Step 4: CLI polls again - should get access token
		tokenReq := httptest.NewRequest("POST", "/auth/device/token", bytes.NewReader(tokenBodyJSON))
		tokenReq.Header.Set("Content-Type", "application/json")

		tokenW := httptest.NewRecorder()
		auth.HandleDeviceToken(env.DB)(tokenW, tokenReq)

		testutil.AssertStatus(t, tokenW, http.StatusOK)

		var tokenResp auth.DeviceTokenResponse
		testutil.ParseJSONResponse(t, tokenW, &tokenResp)

		if tokenResp.Error != "" {
			t.Errorf("expected no error, got %s", tokenResp.Error)
		}
		if tokenResp.AccessToken == "" {
			t.Error("expected access_token")
		}
		if !strings.HasPrefix(tokenResp.AccessToken, "cfb_") {
			t.Error("expected access_token to start with cfb_")
		}

		// Step 5: Verify API key works
		keyHash := auth.HashAPIKey(tokenResp.AccessToken)
		userID, _, err := env.DB.ValidateAPIKey(env.Ctx, keyHash)
		if err != nil {
			t.Fatalf("failed to validate API key: %v", err)
		}
		if userID != user.ID {
			t.Errorf("expected user_id %d, got %d", user.ID, userID)
		}
	})
}
