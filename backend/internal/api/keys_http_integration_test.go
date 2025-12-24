package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// API Keys HTTP Integration Tests
//
// These tests run against a real HTTP server with the production router.
// API key endpoints require session auth + CSRF for state-changing operations.
// =============================================================================

// setupKeysTestServer creates a test server for keys tests
func setupKeysTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
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
// POST /api/v1/keys - Create API key
// =============================================================================

func TestCreateAPIKey_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates API key successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateAPIKeyRequest{
			Name: "My Test Key",
		}

		resp, err := client.Post("/api/v1/keys", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result CreateAPIKeyResponse
		testutil.ParseJSON(t, resp, &result)

		if result.ID == 0 {
			t.Error("expected non-zero key ID")
		}
		if result.Name != "My Test Key" {
			t.Errorf("expected name 'My Test Key', got %s", result.Name)
		}
		if result.Key == "" {
			t.Error("expected non-empty API key")
		}
		// API keys are "cfb_" (4 chars) + 40 chars base64 = 44 chars total
		if len(result.Key) != 44 {
			t.Errorf("expected API key length 44, got %d", len(result.Key))
		}
		if !strings.HasPrefix(result.Key, "cfb_") {
			t.Errorf("expected API key to start with 'cfb_', got %s", result.Key[:4])
		}

		// Verify key exists in database
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM api_keys WHERE id = $1 AND user_id = $2 AND name = $3",
			result.ID, user.ID, "My Test Key")
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query api_keys: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 API key in database, got %d", count)
		}
	})

	t.Run("creates API key with default name when not provided", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateAPIKeyRequest{
			Name: "", // Empty - should default to "API Key"
		}

		resp, err := client.Post("/api/v1/keys", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result CreateAPIKeyResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Name != "API Key" {
			t.Errorf("expected default name 'API Key', got %s", result.Name)
		}
	})

	t.Run("returns 403 CSRF error for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		reqBody := CreateAPIKeyRequest{
			Name: "Test Key",
		}

		resp, err := client.Post("/api/v1/keys", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// CSRF validation happens before auth, so we get 403 instead of 401
		testutil.RequireStatus(t, resp, http.StatusForbidden)
	})
}

// =============================================================================
// GET /api/v1/keys - List API keys
// =============================================================================

func TestListAPIKeys_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("lists all API keys for user", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		// Create two API keys
		testutil.CreateTestAPIKey(t, env, user.ID, "hash1", "Key One")
		testutil.CreateTestAPIKey(t, env, user.ID, "hash2", "Key Two")

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/keys")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var keys []models.APIKey
		testutil.ParseJSON(t, resp, &keys)

		if len(keys) != 2 {
			t.Errorf("expected 2 keys, got %d", len(keys))
		}

		// Verify names
		names := make(map[string]bool)
		for _, key := range keys {
			names[key.Name] = true
			// Verify key_hash is NOT returned (security)
			if key.KeyHash != "" {
				t.Error("key_hash should not be returned in list response")
			}
		}

		if !names["Key One"] {
			t.Error("expected 'Key One' in response")
		}
		if !names["Key Two"] {
			t.Error("expected 'Key Two' in response")
		}
	})

	t.Run("returns empty array when user has no keys", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/keys")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var keys []models.APIKey
		testutil.ParseJSON(t, resp, &keys)

		if len(keys) != 0 {
			t.Errorf("expected 0 keys, got %d", len(keys))
		}
	})

	t.Run("only returns keys belonging to authenticated user", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User Two")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user1.ID)

		// Create keys for both users
		testutil.CreateTestAPIKey(t, env, user1.ID, "hash1", "User1 Key")
		testutil.CreateTestAPIKey(t, env, user2.ID, "hash2", "User2 Key")

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/keys")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var keys []models.APIKey
		testutil.ParseJSON(t, resp, &keys)

		if len(keys) != 1 {
			t.Errorf("expected 1 key for user1, got %d", len(keys))
		}

		if len(keys) > 0 && keys[0].Name != "User1 Key" {
			t.Errorf("expected 'User1 Key', got %s", keys[0].Name)
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		resp, err := client.Get("/api/v1/keys")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}

// =============================================================================
// DELETE /api/v1/keys/{id} - Delete API key
// =============================================================================

func TestDeleteAPIKey_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("deletes API key successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		keyID := testutil.CreateTestAPIKey(t, env, user.ID, "hash1", "Key to Delete")

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Delete("/api/v1/keys/" + strconv.FormatInt(keyID, 10))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNoContent)

		// Verify key was deleted
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM api_keys WHERE id = $1",
			keyID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query api_keys: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 keys after deletion, got %d", count)
		}
	})

	t.Run("returns 404 when deleting non-existent key", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Delete("/api/v1/keys/99999")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("prevents deleting another user's key", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User Two")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user1.ID)

		// Create key for user2
		keyID := testutil.CreateTestAPIKey(t, env, user2.ID, "hash1", "User2 Key")

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Try to delete as user1
		resp, err := client.Delete("/api/v1/keys/" + strconv.FormatInt(keyID, 10))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)

		// Verify key still exists
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM api_keys WHERE id = $1",
			keyID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query api_keys: %v", err)
		}
		if count != 1 {
			t.Error("key should still exist after unauthorized delete attempt")
		}
	})

	t.Run("returns 400 for invalid key ID", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Delete("/api/v1/keys/invalid")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 403 CSRF error for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupKeysTestServer(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		resp, err := client.Delete("/api/v1/keys/123")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// CSRF validation happens before auth, so we get 403 instead of 401
		testutil.RequireStatus(t, resp, http.StatusForbidden)
	})
}
