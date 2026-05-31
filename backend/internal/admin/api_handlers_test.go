package admin_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/logger"
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

	apiServer := api.NewServer(env.DB, env.Storage, &oauthConfig, nil, api.BuildInfo{})
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
		testutil.UploadTestChunk(t, env, target.ID, models.ProviderClaudeCode, "target-ext-1", "transcript.jsonl", 1, 3, transcript)
		testutil.UploadTestChunk(t, env, target.ID, models.ProviderClaudeCode, "target-ext-2", "transcript.jsonl", 1, 3, transcript)

		// Create session with S3 data for bystander (must survive)
		_ = testutil.CreateTestSessionFull(t, env, bystander.ID, "bystander-ext-1", testutil.TestSessionFullOpts{})
		testutil.UploadTestChunk(t, env, bystander.ID, models.ProviderClaudeCode, "bystander-ext-1", "transcript.jsonl", 1, 3, transcript)

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

	// CF-370: admin must be able to triage Codex vs Claude shares from the list.
	t.Run("list includes canonical provider per row", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		claudeSessionID := testutil.CreateTestSessionWithProvider(t, env, adminUser.ID, "ext-claude", "claude-code")
		codexSessionID := testutil.CreateTestSessionWithProvider(t, env, adminUser.ID, "ext-codex", "codex")
		legacySessionID := testutil.CreateTestSessionLegacyClaudeCode(t, env, adminUser.ID, "ext-legacy")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		for _, sid := range []string{claudeSessionID, codexSessionID, legacySessionID} {
			resp, err := client.Post("/api/v1/admin/system-shares", map[string]string{
				"session_id": sid,
			})
			if err != nil {
				t.Fatalf("create request failed for %s: %v", sid, err)
			}
			testutil.RequireStatus(t, resp, http.StatusOK)
		}

		resp, err := client.Get("/api/v1/admin/system-shares")
		if err != nil {
			t.Fatalf("list request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var listResult admin.SystemSharesResponse
		testutil.ParseJSON(t, resp, &listResult)
		if len(listResult.Shares) != 3 {
			t.Fatalf("expected 3 shares, got %d", len(listResult.Shares))
		}

		wantProvider := map[string]string{
			claudeSessionID: "claude-code",
			codexSessionID:  "codex",
			legacySessionID: "claude-code", // legacy "Claude Code" normalizes
		}
		for _, share := range listResult.Shares {
			want, ok := wantProvider[share.SessionID]
			if !ok {
				t.Errorf("unexpected share for session %s", share.SessionID)
				continue
			}
			if share.Provider != want {
				t.Errorf("session %s: response.provider = %q, want %q", share.SessionID, share.Provider, want)
			}
		}
	})

	// CF-370: audit trail must carry the provider so historic actions remain
	// interpretable even if the underlying session row is later deleted.
	t.Run("create records provider in audit log", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		codexSessionID := testutil.CreateTestSessionWithProvider(t, env, adminUser.ID, "ext-codex-audit", "codex")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		auditLines := captureAuditLog(t, func() {
			resp, err := client.Post("/api/v1/admin/system-shares", map[string]string{
				"session_id": codexSessionID,
			})
			if err != nil {
				t.Fatalf("create request failed: %v", err)
			}
			testutil.RequireStatus(t, resp, http.StatusOK)
		})

		entry := findAuditEntry(t, auditLines, string(admin.ActionSystemShareCreate))
		if entry["provider"] != "codex" {
			t.Errorf("audit log: provider = %v, want %q (entry=%v)", entry["provider"], "codex", entry)
		}
	})
}

// captureAuditLog redirects the logger output through a pipe so tests can read
// emitted JSON log lines while f runs. The original handler is restored on
// cleanup. LOG_FORMAT=json is already set by the enclosing TestAdminSystemSharesAPI.
func captureAuditLog(t *testing.T, f func()) []map[string]any {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(logger.SetOutputForTest(w))

	f()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	out := strings.TrimSpace(buf.String())
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	parsed := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			continue // skip non-JSON lines (e.g. test harness output)
		}
		parsed = append(parsed, decoded)
	}
	return parsed
}

// findAuditEntry locates the first ADMIN_AUDIT log entry whose "action" field
// matches the wanted action. Fails the test if no such entry is found.
func findAuditEntry(t *testing.T, lines []map[string]any, action string) map[string]any {
	t.Helper()
	for _, line := range lines {
		if line["msg"] == "ADMIN_AUDIT" && line["action"] == action {
			return line
		}
	}
	t.Fatalf("no ADMIN_AUDIT log entry found for action %q in %d captured lines", action, len(lines))
	return nil
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

// ===================================================================
// Smart Recap Prompt Settings API
// ===================================================================

func TestAdminSmartRecapPrompt_GetDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("GET returns default when no custom prompt set", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Get("/api/v1/admin/settings/smart-recap-prompt")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var body admin.SmartRecapPromptResponse
		testutil.ParseJSON(t, resp, &body)

		if body.IsCustom {
			t.Error("expected is_custom=false when no custom prompt set")
		}
		if body.Instructions == "" {
			t.Error("expected non-empty default instructions")
		}
		if body.UpdatedAt != nil {
			t.Errorf("expected updated_at to be nil for default, got %v", body.UpdatedAt)
		}
		if body.InputFormat == "" {
			t.Error("expected non-empty input_format")
		}
		if body.OutputSchema == "" {
			t.Error("expected non-empty output_schema")
		}
		if body.Example == "" {
			t.Error("expected non-empty example")
		}
	})
}

func TestAdminSmartRecapPrompt_PutThenGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("PUT saves custom prompt, GET returns it with is_custom=true", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// PUT custom instructions
		putResp, err := client.Request("PUT", "/api/v1/admin/settings/smart-recap-prompt", map[string]string{
			"instructions": "Custom analysis instructions for testing.",
		})
		if err != nil {
			t.Fatalf("PUT request failed: %v", err)
		}
		testutil.RequireStatus(t, putResp, http.StatusOK)

		var putBody admin.SetSmartRecapPromptResponse
		testutil.ParseJSON(t, putResp, &putBody)

		if !putBody.IsCustom {
			t.Error("expected is_custom=true in PUT response")
		}
		if putBody.Instructions != "Custom analysis instructions for testing." {
			t.Errorf("unexpected instructions in PUT response: %q", putBody.Instructions)
		}
		if putBody.UpdatedAt == "" {
			t.Error("expected non-empty updated_at in PUT response")
		}

		// GET should return the custom prompt
		getResp, err := client.Get("/api/v1/admin/settings/smart-recap-prompt")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		testutil.RequireStatus(t, getResp, http.StatusOK)

		var getBody admin.SmartRecapPromptResponse
		testutil.ParseJSON(t, getResp, &getBody)

		if !getBody.IsCustom {
			t.Error("expected is_custom=true in GET response after PUT")
		}
		if getBody.Instructions != "Custom analysis instructions for testing." {
			t.Errorf("unexpected instructions in GET response: %q", getBody.Instructions)
		}
		if getBody.UpdatedAt == nil {
			t.Error("expected non-nil updated_at in GET response after PUT")
		}
	})
}

func TestAdminSmartRecapPrompt_PutEmptyString(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("PUT with empty string saves as custom with is_custom=true", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// PUT empty instructions
		putResp, err := client.Request("PUT", "/api/v1/admin/settings/smart-recap-prompt", map[string]string{
			"instructions": "",
		})
		if err != nil {
			t.Fatalf("PUT request failed: %v", err)
		}
		testutil.RequireStatus(t, putResp, http.StatusOK)

		var putBody admin.SetSmartRecapPromptResponse
		testutil.ParseJSON(t, putResp, &putBody)
		if !putBody.IsCustom {
			t.Error("expected is_custom=true for empty string")
		}
		if putBody.Instructions != "" {
			t.Errorf("expected empty instructions, got %q", putBody.Instructions)
		}

		// GET confirms it
		getResp, err := client.Get("/api/v1/admin/settings/smart-recap-prompt")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		testutil.RequireStatus(t, getResp, http.StatusOK)

		var getBody admin.SmartRecapPromptResponse
		testutil.ParseJSON(t, getResp, &getBody)
		if !getBody.IsCustom {
			t.Error("expected is_custom=true in GET after PUT empty string")
		}
		if getBody.Instructions != "" {
			t.Errorf("expected empty instructions in GET, got %q", getBody.Instructions)
		}
	})
}

func TestAdminSmartRecapPrompt_DeleteResets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("DELETE resets to default, GET returns is_custom=false", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// First PUT a custom prompt
		putResp, err := client.Request("PUT", "/api/v1/admin/settings/smart-recap-prompt", map[string]string{
			"instructions": "Custom prompt to be deleted.",
		})
		if err != nil {
			t.Fatalf("PUT request failed: %v", err)
		}
		testutil.RequireStatus(t, putResp, http.StatusOK)
		putResp.Body.Close()

		// DELETE to reset
		delResp, err := client.Delete("/api/v1/admin/settings/smart-recap-prompt")
		if err != nil {
			t.Fatalf("DELETE request failed: %v", err)
		}
		testutil.RequireStatus(t, delResp, http.StatusOK)

		var delBody admin.DeleteSmartRecapPromptResponse
		testutil.ParseJSON(t, delResp, &delBody)
		if delBody.IsCustom {
			t.Error("expected is_custom=false in DELETE response")
		}
		if delBody.Instructions == "" {
			t.Error("expected non-empty default instructions in DELETE response")
		}

		// GET should return default
		getResp, err := client.Get("/api/v1/admin/settings/smart-recap-prompt")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		testutil.RequireStatus(t, getResp, http.StatusOK)

		var getBody admin.SmartRecapPromptResponse
		testutil.ParseJSON(t, getResp, &getBody)
		if getBody.IsCustom {
			t.Error("expected is_custom=false after DELETE")
		}
		if getBody.UpdatedAt != nil {
			t.Error("expected nil updated_at after DELETE")
		}
	})
}

func TestAdminSmartRecapPrompt_GetDefaultEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("GET /default always returns hardcoded default regardless of state", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// Set a custom prompt first
		putResp, err := client.Request("PUT", "/api/v1/admin/settings/smart-recap-prompt", map[string]string{
			"instructions": "Custom prompt that should not affect /default.",
		})
		if err != nil {
			t.Fatalf("PUT request failed: %v", err)
		}
		testutil.RequireStatus(t, putResp, http.StatusOK)
		putResp.Body.Close()

		// GET /default should still return the hardcoded default
		resp, err := client.Get("/api/v1/admin/settings/smart-recap-prompt/default")
		if err != nil {
			t.Fatalf("GET /default request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var body map[string]string
		testutil.ParseJSON(t, resp, &body)

		instructions, ok := body["instructions"]
		if !ok {
			t.Fatal("expected 'instructions' field in response")
		}
		if instructions == "" {
			t.Error("expected non-empty default instructions")
		}
		// Should NOT be the custom prompt
		if instructions == "Custom prompt that should not affect /default." {
			t.Error("GET /default should return hardcoded default, not custom prompt")
		}
	})
}

func TestAdminSmartRecapPrompt_PutValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("rejects null bytes", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		resp, err := client.Request("PUT", "/api/v1/admin/settings/smart-recap-prompt", map[string]string{
			"instructions": "contains\x00null",
		})
		if err != nil {
			t.Fatalf("PUT request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()
	})

	t.Run("rejects content exceeding 50000 chars", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// Create a string that exceeds 50000 chars
		longString := strings.Repeat("a", 50001)

		resp, err := client.Request("PUT", "/api/v1/admin/settings/smart-recap-prompt", map[string]string{
			"instructions": longString,
		})
		if err != nil {
			t.Fatalf("PUT request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()
	})
}

func TestAdminSmartRecapPrompt_RegenerateCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns count of smart recap cards", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// With empty DB, count should be 0
		resp, err := client.Get("/api/v1/admin/settings/smart-recap-prompt/regenerate-count")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var body map[string]int
		testutil.ParseJSON(t, resp, &body)

		count, ok := body["count"]
		if !ok {
			t.Fatal("expected 'count' field in response")
		}
		if count != 0 {
			t.Errorf("expected count=0 with empty DB, got %d", count)
		}
	})
}

func TestAdminSmartRecapPrompt_RegenerateAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("writes regen timestamp and returns count", func(t *testing.T) {
		env.CleanDB(t)

		adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, adminUser.ID)

		// Trigger regeneration
		resp, err := client.Post("/api/v1/admin/settings/smart-recap-prompt/regenerate-all", map[string]string{})
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)

		var body map[string]int
		testutil.ParseJSON(t, resp, &body)

		_, ok := body["sessions_queued"]
		if !ok {
			t.Fatal("expected 'sessions_queued' field in response")
		}

		// Verify the regen timestamp was written to admin_settings
		var value string
		err = env.DB.Conn().QueryRow(
			"SELECT value FROM admin_settings WHERE key = 'smart_recap_regen_requested_at'",
		).Scan(&value)
		if err != nil {
			t.Fatalf("expected regen timestamp in admin_settings, got error: %v", err)
		}
		if value == "" {
			t.Error("expected non-empty regen timestamp value")
		}
	})
}

// Suppress unused import warning
var _ = json.Marshal
