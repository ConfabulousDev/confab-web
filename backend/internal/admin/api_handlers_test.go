package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// setupTestServer creates a test HTTP server with the full middleware stack.
func setupTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()

	testutil.SetEnvForTest(t, "CSRF_SECRET_KEY", "test-csrf-secret-key-32-bytes!!")
	testutil.SetEnvForTest(t, "ALLOWED_ORIGINS", "http://localhost:3000")
	testutil.SetEnvForTest(t, "FRONTEND_URL", "http://localhost:3000")
	testutil.SetEnvForTest(t, "INSECURE_DEV_MODE", "true")
	testutil.SetEnvForTest(t, "ENABLE_SHARE_CREATION", "true")

	oauthConfig := auth.OAuthConfig{
		PasswordEnabled: true,
	}

	apiServer := api.NewServer(env.DB, env.Storage, &oauthConfig, nil, "")
	handler := apiServer.SetupRoutes()

	return testutil.StartTestServer(t, env, handler)
}

// adminClient creates an authenticated test client for the given admin user.
func adminClient(t *testing.T, env *testutil.TestEnvironment, ts *testutil.TestServer, userID int64) *testutil.TestClient {
	t.Helper()
	token := testutil.CreateTestWebSessionWithToken(t, env, userID)
	return testutil.NewTestClient(t, ts).WithSession(token)
}

// ===================================================================
// GET /api/v1/me — is_admin field
// ===================================================================

func TestGetMe_IsAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("admin user sees is_admin true", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, user.ID)

		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		testutil.RequireStatus(t, resp, http.StatusOK)

		var body map[string]interface{}
		testutil.ParseJSON(t, resp, &body)
		if body["is_admin"] != true {
			t.Errorf("expected is_admin=true, got %v", body["is_admin"])
		}
	})

	t.Run("non-admin user sees is_admin false", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "user@example.com", "User")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, user.ID)

		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		testutil.RequireStatus(t, resp, http.StatusOK)

		var body map[string]interface{}
		testutil.ParseJSON(t, resp, &body)
		if body["is_admin"] != false {
			t.Errorf("expected is_admin=false, got %v", body["is_admin"])
		}
	})
}

// ===================================================================
// Admin API auth enforcement
// ===================================================================

func TestAdminAPI_AuthEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	// Use empty body for POST requests to satisfy Content-Type validation middleware
	emptyBody := map[string]string{}

	endpoints := []struct {
		method string
		path   string
		body   interface{}
	}{
		{"GET", "/api/v1/admin/users", nil},
		{"POST", "/api/v1/admin/users", emptyBody},
		{"POST", "/api/v1/admin/users/1/deactivate", emptyBody},
		{"POST", "/api/v1/admin/users/1/activate", emptyBody},
		{"DELETE", "/api/v1/admin/users/1", nil},
		{"GET", "/api/v1/admin/system-shares", nil},
		{"POST", "/api/v1/admin/system-shares", emptyBody},
	}

	t.Run("unauthenticated gets 401", func(t *testing.T) {
		env.CleanDB(t)
		ts := setupTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		for _, ep := range endpoints {
			resp, err := client.Request(ep.method, ep.path, ep.body)
			if err != nil {
				t.Fatalf("%s %s: request failed: %v", ep.method, ep.path, err)
			}
			if resp.StatusCode != http.StatusUnauthorized {
				resp.Body.Close()
				t.Errorf("%s %s: expected 401, got %d", ep.method, ep.path, resp.StatusCode)
			} else {
				resp.Body.Close()
			}
		}
	})

	t.Run("non-admin gets 403", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "user@example.com", "User")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, user.ID)

		for _, ep := range endpoints {
			resp, err := client.Request(ep.method, ep.path, ep.body)
			if err != nil {
				t.Fatalf("%s %s: request failed: %v", ep.method, ep.path, err)
			}
			if resp.StatusCode != http.StatusForbidden {
				resp.Body.Close()
				t.Errorf("%s %s: expected 403, got %d", ep.method, ep.path, resp.StatusCode)
			} else {
				resp.Body.Close()
			}
		}
	})
}

// ===================================================================
// GET /api/v1/admin/users
// ===================================================================

func TestAdminListUsersAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns user list with stats", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		testutil.CreateTestUser(t, env, "user2@example.com", "User Two")

		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Get("/api/v1/admin/users")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result admin.AdminUserListResponse
		testutil.ParseJSON(t, resp, &result)

		if len(result.Users) != 3 {
			t.Fatalf("expected 3 users, got %d", len(result.Users))
		}

		// Verify fields present
		found := false
		for _, u := range result.Users {
			if u.Email == "user1@example.com" {
				found = true
				if u.Status != "active" {
					t.Errorf("expected active status, got %s", u.Status)
				}
				if u.Name == nil || *u.Name != "User One" {
					t.Errorf("expected name 'User One', got %v", u.Name)
				}
			}
		}
		if !found {
			t.Error("user1@example.com not found in response")
		}
	})
}

// ===================================================================
// POST /api/v1/admin/users/{id}/deactivate + activate
// ===================================================================

func TestAdminDeactivateActivateUserAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("deactivate then activate", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		target := testutil.CreateTestUser(t, env, "target@example.com", "Target")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// Deactivate
		resp, err := client.Post(fmt.Sprintf("/api/v1/admin/users/%d/deactivate", target.ID), map[string]string{})
		if err != nil {
			t.Fatalf("deactivate request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var deactivateResult admin.StatusChangeResponse
		testutil.ParseJSON(t, resp, &deactivateResult)
		if deactivateResult.Status != "inactive" {
			t.Errorf("expected inactive, got %s", deactivateResult.Status)
		}

		// Verify in DB
		userStore := &dbuser.Store{DB: env.DB}
		dbTarget, err := userStore.GetUserByID(env.Ctx, target.ID)
		if err != nil {
			t.Fatalf("failed to get user: %v", err)
		}
		if dbTarget.Status != models.UserStatusInactive {
			t.Errorf("expected inactive in DB, got %s", dbTarget.Status)
		}

		// Activate
		resp, err = client.Post(fmt.Sprintf("/api/v1/admin/users/%d/activate", target.ID), map[string]string{})
		if err != nil {
			t.Fatalf("activate request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var activateResult admin.StatusChangeResponse
		testutil.ParseJSON(t, resp, &activateResult)
		if activateResult.Status != "active" {
			t.Errorf("expected active, got %s", activateResult.Status)
		}
	})
}

// ===================================================================
// POST /api/v1/admin/users — create user
// ===================================================================

func TestAdminCreateUserAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates user with valid data", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Post("/api/v1/admin/users", map[string]string{
			"email":    "newuser@example.com",
			"password": "securepassword123",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var result admin.CreateUserResponse
		testutil.ParseJSON(t, resp, &result)
		if result.Email != "newuser@example.com" {
			t.Errorf("expected email newuser@example.com, got %s", result.Email)
		}
		if result.ID == 0 {
			t.Error("expected non-zero user ID")
		}
	})

	t.Run("rejects short password", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Post("/api/v1/admin/users", map[string]string{
			"email":    "newuser@example.com",
			"password": "short",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects invalid email", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Post("/api/v1/admin/users", map[string]string{
			"email":    "not-an-email",
			"password": "securepassword123",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.CreateTestUser(t, env, "existing@example.com", "Existing")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Post("/api/v1/admin/users", map[string]string{
			"email":    "existing@example.com",
			"password": "securepassword123",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusConflict)
	})
}

// ===================================================================
// DELETE /api/v1/admin/users/{id}
// ===================================================================

func TestAdminDeleteUserAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("deletes user DB and S3 data, preserves other users", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		target := testutil.CreateTestUser(t, env, "target@example.com", "Target")
		bystander := testutil.CreateTestUser(t, env, "bystander@example.com", "Bystander")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		// Create sessions with S3 data for target user
		targetSession1 := testutil.CreateTestSessionFull(t, env, target.ID, "target-ext-1", testutil.TestSessionFullOpts{})
		_ = testutil.CreateTestSessionFull(t, env, target.ID, "target-ext-2", testutil.TestSessionFullOpts{})

		// Upload S3 chunks for target
		transcript := testutil.MinimalTranscript()
		testutil.UploadTestChunk(t, env, target.ID, "target-ext-1", "transcript.jsonl", 1, 3, transcript)
		testutil.UploadTestChunk(t, env, target.ID, "target-ext-2", "transcript.jsonl", 1, 3, transcript)

		// Create session with S3 data for bystander (must survive)
		_ = testutil.CreateTestSessionFull(t, env, bystander.ID, "bystander-ext-1", testutil.TestSessionFullOpts{})
		testutil.UploadTestChunk(t, env, bystander.ID, "bystander-ext-1", "transcript.jsonl", 1, 3, transcript)

		// Verify S3 data exists before deletion
		targetS3Key := fmt.Sprintf("%d/claude-code/target-ext-1/chunks/transcript.jsonl/chunk_00000001_00000003.jsonl", target.ID)
		bystanderS3Key := fmt.Sprintf("%d/claude-code/bystander-ext-1/chunks/transcript.jsonl/chunk_00000001_00000003.jsonl", bystander.ID)
		testutil.VerifyFileInS3(t, env, targetS3Key)
		testutil.VerifyFileInS3(t, env, bystanderS3Key)

		// Delete target user
		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Delete(fmt.Sprintf("/api/v1/admin/users/%d", target.ID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		// Verify target user gone from DB
		userStore := &dbuser.Store{DB: env.DB}
		_, err = userStore.GetUserByID(env.Ctx, target.ID)
		if err == nil {
			t.Error("expected target user to be deleted from DB")
		}

		// Verify target sessions gone from DB (CASCADE)
		ids, err := userStore.GetUserSessionIDs(env.Ctx, target.ID)
		if err != nil {
			t.Fatalf("GetUserSessionIDs failed: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("expected 0 sessions for deleted user, got %d", len(ids))
		}

		// Verify target S3 data is deleted
		_, err = env.Storage.Download(env.Ctx, targetS3Key)
		if err == nil {
			t.Error("expected target S3 data to be deleted")
		}

		// Verify bystander is untouched — DB and S3
		bystanderUser, err := userStore.GetUserByID(env.Ctx, bystander.ID)
		if err != nil {
			t.Fatalf("bystander user should still exist: %v", err)
		}
		if bystanderUser.Email != "bystander@example.com" {
			t.Errorf("expected bystander email, got %s", bystanderUser.Email)
		}

		bystanderData := testutil.VerifyFileInS3(t, env, bystanderS3Key)
		if len(bystanderData) == 0 {
			t.Error("expected bystander S3 data to still exist")
		}

		// Verify bystander session still in DB
		_ = targetSession1 // used above for S3 key construction
		bystanderIDs, err := userStore.GetUserSessionIDs(env.Ctx, bystander.ID)
		if err != nil {
			t.Fatalf("bystander GetUserSessionIDs failed: %v", err)
		}
		if len(bystanderIDs) != 1 {
			t.Errorf("expected 1 session for bystander, got %d", len(bystanderIDs))
		}
	})

	t.Run("returns 404 for non-existent user", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Delete("/api/v1/admin/users/99999")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// ===================================================================
// GET + POST /api/v1/admin/system-shares
// ===================================================================

func TestAdminSystemSharesAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("create and list system shares", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		// Create a session to share
		sessionID := testutil.CreateTestSessionFull(t, env, adminUser.ID, "ext-1", testutil.TestSessionFullOpts{})

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// Create system share
		resp, err := client.Post("/api/v1/admin/system-shares", map[string]string{
			"session_id": sessionID,
		})
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var createResult admin.CreateSystemShareResponse
		testutil.ParseJSON(t, resp, &createResult)
		if createResult.ShareURL == "" {
			t.Error("expected non-empty share URL")
		}
		if createResult.ShareID == 0 {
			t.Error("expected non-zero share ID")
		}

		// List system shares
		resp, err = client.Get("/api/v1/admin/system-shares")
		if err != nil {
			t.Fatalf("list request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var listResult admin.SystemSharesResponse
		testutil.ParseJSON(t, resp, &listResult)
		if len(listResult.Shares) != 1 {
			t.Fatalf("expected 1 share, got %d", len(listResult.Shares))
		}
		if listResult.Shares[0].SessionID != sessionID {
			t.Errorf("expected session ID %s, got %s", sessionID, listResult.Shares[0].SessionID)
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Post("/api/v1/admin/system-shares", map[string]string{
			"session_id": "00000000-0000-0000-0000-000000000000",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})
}

// ===================================================================
// GET /api/v1/auth/config — password_auth_enabled
// ===================================================================

func TestAuthConfig_PasswordAuthEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ts := setupTestServer(t, env)
	client := testutil.NewTestClient(t, ts)

	resp, err := client.Get("/api/v1/auth/config")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)

	var body struct {
		Features struct {
			PasswordAuthEnabled bool `json:"password_auth_enabled"`
		} `json:"features"`
	}
	testutil.ParseJSON(t, resp, &body)
	if !body.Features.PasswordAuthEnabled {
		t.Error("expected password_auth_enabled=true")
	}
}

// Suppress unused import warning
var _ = json.Marshal
