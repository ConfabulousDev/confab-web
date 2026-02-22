package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// /api/v1/me HTTP Integration Tests
//
// These tests run against a real HTTP server with the production router.
// They verify the meResponse struct includes has_own_sessions and has_api_keys
// fields computed via EXISTS subqueries (CF-252).
// =============================================================================

// setupUserTestServer creates a test server for user/me tests
func setupUserTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
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

	apiServer := NewServer(env.DB, env.Storage, &oauthConfig, nil, "")
	handler := apiServer.SetupRoutes()

	return testutil.StartTestServer(t, env, handler)
}

// =============================================================================
// GET /api/v1/me - Get current user info
// =============================================================================

func TestGetMe_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns user info with has_own_sessions=false and has_api_keys=false for fresh user", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "fresh@example.com", "Fresh User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupUserTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result meResponse
		testutil.ParseJSON(t, resp, &result)

		if result.ID != user.ID {
			t.Errorf("expected user ID %d, got %d", user.ID, result.ID)
		}
		if result.Email != user.Email {
			t.Errorf("expected email %q, got %q", user.Email, result.Email)
		}
		if result.HasOwnSessions {
			t.Error("expected has_own_sessions to be false for fresh user")
		}
		if result.HasAPIKeys {
			t.Error("expected has_api_keys to be false for fresh user")
		}
	})

	t.Run("returns has_own_sessions=true when user has sessions", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "sessions@example.com", "Sessions User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		// Create a session owned by this user
		testutil.CreateTestSession(t, env, user.ID, "ext-session-1")

		ts := setupUserTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result meResponse
		testutil.ParseJSON(t, resp, &result)

		if !result.HasOwnSessions {
			t.Error("expected has_own_sessions to be true when user has sessions")
		}
		if result.HasAPIKeys {
			t.Error("expected has_api_keys to be false when user has no API keys")
		}
	})

	t.Run("returns has_api_keys=true when user has API keys", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "apikeys@example.com", "API Keys User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		// Create an API key for this user
		testutil.CreateTestAPIKey(t, env, user.ID, "test-hash", "My Key")

		ts := setupUserTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result meResponse
		testutil.ParseJSON(t, resp, &result)

		if result.HasOwnSessions {
			t.Error("expected has_own_sessions to be false when user has no sessions")
		}
		if !result.HasAPIKeys {
			t.Error("expected has_api_keys to be true when user has API keys")
		}
	})

	t.Run("returns both true when user has sessions and API keys", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "both@example.com", "Both User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		// Create both a session and an API key
		testutil.CreateTestSession(t, env, user.ID, "ext-session-both")
		testutil.CreateTestAPIKey(t, env, user.ID, "test-hash-both", "Both Key")

		ts := setupUserTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result meResponse
		testutil.ParseJSON(t, resp, &result)

		if !result.HasOwnSessions {
			t.Error("expected has_own_sessions to be true")
		}
		if !result.HasAPIKeys {
			t.Error("expected has_api_keys to be true")
		}
	})

	t.Run("does not count other users sessions or API keys", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User Two")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user1.ID)

		// Create sessions and API keys for user2 only
		testutil.CreateTestSession(t, env, user2.ID, "ext-session-user2")
		testutil.CreateTestAPIKey(t, env, user2.ID, "hash-user2", "User2 Key")

		ts := setupUserTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result meResponse
		testutil.ParseJSON(t, resp, &result)

		if result.ID != user1.ID {
			t.Errorf("expected user ID %d, got %d", user1.ID, result.ID)
		}
		if result.HasOwnSessions {
			t.Error("expected has_own_sessions to be false; other user's sessions should not count")
		}
		if result.HasAPIKeys {
			t.Error("expected has_api_keys to be false; other user's API keys should not count")
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupUserTestServer(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}
