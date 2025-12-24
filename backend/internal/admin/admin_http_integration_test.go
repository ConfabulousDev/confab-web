package admin_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// Admin HTTP Integration Tests
//
// These tests run against a real HTTP server with the production router.
// Admin routes require session auth + CSRF + super admin check.
// =============================================================================

// setupAdminTestServer creates a test server with proper environment for admin tests
func setupAdminTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
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

	apiServer := api.NewServer(env.DB, env.Storage, oauthConfig, nil)
	handler := apiServer.SetupRoutes()

	return testutil.StartTestServer(t, env, handler)
}

// =============================================================================
// GET /admin-{uuid}/users - List all users (admin only)
// =============================================================================

func TestAdminListUsers_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("super admin can list users", func(t *testing.T) {
		env.CleanDB(t)

		// Create users
		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin User")
		testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		testutil.CreateTestUser(t, env, "user2@example.com", "User Two")

		// Set super admin
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, adminUser.ID)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(admin.AdminPathPrefix + "/users")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify HTML response contains user list
		body := make([]byte, 8192)
		n, _ := resp.Body.Read(body)
		bodyStr := string(body[:n])

		if !strings.Contains(bodyStr, "admin@example.com") {
			t.Error("expected admin@example.com in response")
		}
		if !strings.Contains(bodyStr, "user1@example.com") {
			t.Error("expected user1@example.com in response")
		}
		if !strings.Contains(bodyStr, "user2@example.com") {
			t.Error("expected user2@example.com in response")
		}
	})

	t.Run("non-admin cannot list users", func(t *testing.T) {
		env.CleanDB(t)

		regularUser := testutil.CreateTestUser(t, env, "user@example.com", "Regular User")

		// Set different user as super admin
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, regularUser.ID)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(admin.AdminPathPrefix + "/users")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("unauthenticated cannot access admin", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		resp, err := client.Get(admin.AdminPathPrefix + "/users")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}

// =============================================================================
// POST /admin-{uuid}/users/{id}/deactivate - Deactivate a user
// =============================================================================

func TestAdminDeactivateUser_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("super admin can deactivate user", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin User")
		targetUser := testutil.CreateTestUser(t, env, "user@example.com", "Target User")

		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, adminUser.ID)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Post(admin.AdminPathPrefix+"/users/2/deactivate", nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should redirect on success
		testutil.RequireStatus(t, resp, http.StatusSeeOther)

		// Verify user is now inactive
		updatedUser, err := env.DB.GetUserByID(context.Background(), targetUser.ID)
		if err != nil {
			t.Fatalf("failed to get user: %v", err)
		}
		if updatedUser.Status != models.UserStatusInactive {
			t.Errorf("expected status %s, got %s", models.UserStatusInactive, updatedUser.Status)
		}
	})

	t.Run("non-admin cannot deactivate user", func(t *testing.T) {
		env.CleanDB(t)

		regularUser := testutil.CreateTestUser(t, env, "user@example.com", "Regular User")
		testutil.CreateTestUser(t, env, "target@example.com", "Target User")

		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, regularUser.ID)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Post(admin.AdminPathPrefix+"/users/2/deactivate", nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusForbidden)
	})
}

// =============================================================================
// POST /admin-{uuid}/users/{id}/activate - Reactivate a user
// =============================================================================

func TestAdminActivateUser_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("super admin can activate user", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin User")
		targetUser := testutil.CreateTestUser(t, env, "user@example.com", "Target User")

		// Deactivate the target user first
		if err := env.DB.UpdateUserStatus(context.Background(), targetUser.ID, models.UserStatusInactive); err != nil {
			t.Fatalf("failed to deactivate user: %v", err)
		}

		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, adminUser.ID)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Post(admin.AdminPathPrefix+"/users/2/activate", nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should redirect on success
		testutil.RequireStatus(t, resp, http.StatusSeeOther)

		// Verify user is now active
		updatedUser, err := env.DB.GetUserByID(context.Background(), targetUser.ID)
		if err != nil {
			t.Fatalf("failed to get user: %v", err)
		}
		if updatedUser.Status != models.UserStatusActive {
			t.Errorf("expected status %s, got %s", models.UserStatusActive, updatedUser.Status)
		}
	})
}

// =============================================================================
// POST /admin-{uuid}/users/{id}/delete - Delete a user
// =============================================================================

func TestAdminDeleteUser_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("super admin can delete user", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin User")
		targetUser := testutil.CreateTestUser(t, env, "user@example.com", "Target User")
		sessionID := testutil.CreateTestSession(t, env, targetUser.ID, "test-session")

		// Upload some test data to S3
		testContent := []byte("test transcript content")
		if _, err := env.Storage.UploadChunk(context.Background(), targetUser.ID, sessionID, "transcript.jsonl", 1, 10, testContent); err != nil {
			t.Fatalf("failed to upload test content: %v", err)
		}

		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, adminUser.ID)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Post(admin.AdminPathPrefix+"/users/2/delete", nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should redirect on success
		testutil.RequireStatus(t, resp, http.StatusSeeOther)

		// Verify user no longer exists
		_, err = env.DB.GetUserByID(context.Background(), targetUser.ID)
		if err == nil {
			t.Error("expected error when getting deleted user")
		}

		// Verify S3 objects are deleted
		chunks, err := env.Storage.ListChunks(context.Background(), targetUser.ID, sessionID, "transcript.jsonl")
		if err != nil {
			t.Fatalf("failed to list chunks: %v", err)
		}
		if len(chunks) != 0 {
			t.Errorf("expected 0 chunks after deletion, got %d", len(chunks))
		}
	})

	t.Run("non-admin cannot delete user", func(t *testing.T) {
		env.CleanDB(t)

		regularUser := testutil.CreateTestUser(t, env, "user@example.com", "Regular User")
		testutil.CreateTestUser(t, env, "target@example.com", "Target User")

		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, regularUser.ID)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Post(admin.AdminPathPrefix+"/users/2/delete", nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("delete non-existent user returns error", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin User")

		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, adminUser.ID)

		ts := setupAdminTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Post(admin.AdminPathPrefix+"/users/99999/delete", nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should redirect with error
		testutil.RequireStatus(t, resp, http.StatusSeeOther)

		location := resp.Header.Get("Location")
		if !strings.Contains(location, "error=") {
			t.Errorf("expected redirect with error param, got: %s", location)
		}
	})
}
