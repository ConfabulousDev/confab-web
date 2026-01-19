package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// Session Endpoint HTTP Integration Tests
//
// These tests use session cookie authentication (web dashboard endpoints).
// =============================================================================

// =============================================================================
// GET /api/v1/sessions - List user's sessions
// =============================================================================

func TestListSessions_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("lists all sessions for user with session auth", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		// Create test sessions
		testutil.CreateTestSession(t, env, user.ID, "session-1")
		testutil.CreateTestSession(t, env, user.ID, "session-2")
		testutil.CreateTestSession(t, env, user.ID, "session-3")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var sessions []db.SessionListItem
		testutil.ParseJSON(t, resp, &sessions)

		if len(sessions) != 3 {
			t.Errorf("expected 3 sessions, got %d", len(sessions))
		}
	})

	t.Run("returns empty array when user has no sessions", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var sessions []db.SessionListItem
		testutil.ParseJSON(t, resp, &sessions)

		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(sessions))
		}
	})

	t.Run("returns 401 without session cookie", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		resp, err := client.Get("/api/v1/sessions")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("isolates sessions between users", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")
		session1 := testutil.CreateTestWebSessionWithToken(t, env, user1.ID)

		// Create sessions for both users
		testutil.CreateTestSession(t, env, user1.ID, "user1-session")
		testutil.CreateTestSession(t, env, user2.ID, "user2-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(session1)

		resp, err := client.Get("/api/v1/sessions")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var sessions []db.SessionListItem
		testutil.ParseJSON(t, resp, &sessions)

		// User1 should only see their own session
		if len(sessions) != 1 {
			t.Errorf("expected 1 session for user1, got %d", len(sessions))
		}
		if len(sessions) > 0 && sessions[0].ExternalID != "user1-session" {
			t.Errorf("expected 'user1-session', got %s", sessions[0].ExternalID)
		}
	})
}

// =============================================================================
// GET /api/v1/sessions/{id} - Get session details
// =============================================================================

func TestGetSession_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns session details with sync files", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-detail")
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
		testutil.CreateTestSyncFile(t, env, sessionID, "agent.jsonl", "agent", 200)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var session db.SessionDetail
		testutil.ParseJSON(t, resp, &session)

		if session.ExternalID != "test-session-detail" {
			t.Errorf("expected external_id 'test-session-detail', got %s", session.ExternalID)
		}

		if len(session.Files) != 2 {
			t.Errorf("expected 2 files, got %d", len(session.Files))
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions/00000000-0000-0000-0000-000000000000")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("prevents accessing another user's session", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")
		session1 := testutil.CreateTestWebSessionWithToken(t, env, user1.ID)

		// Session owned by user2
		sessionID := testutil.CreateTestSession(t, env, user2.ID, "user2-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(session1)

		// User1 tries to access user2's session
		resp, err := client.Get("/api/v1/sessions/" + sessionID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should return 404 (not 403) to not reveal session existence
		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("allows access via API key auth", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSession(t, env, user.ID, "api-key-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var session db.SessionDetail
		testutil.ParseJSON(t, resp, &session)

		if session.ExternalID != "api-key-session" {
			t.Errorf("expected external_id 'api-key-session', got %s", session.ExternalID)
		}
	})
}

// =============================================================================
// PATCH /api/v1/sessions/{id}/title - Update session custom title
// =============================================================================

func TestUpdateSessionTitle_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("successfully sets custom title", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "session-1")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		body := map[string]interface{}{"custom_title": "My Custom Title"}
		resp, err := client.Patch("/api/v1/sessions/"+sessionID+"/title", body)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result db.SessionDetail
		testutil.ParseJSON(t, resp, &result)

		if result.CustomTitle == nil || *result.CustomTitle != "My Custom Title" {
			t.Errorf("expected custom_title 'My Custom Title', got %v", result.CustomTitle)
		}
	})

	t.Run("clears custom title when null", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "session-1")

		// First set a custom title via database
		customTitle := "Initial Title"
		err := env.DB.UpdateSessionCustomTitle(env.Ctx, sessionID, user.ID, &customTitle)
		if err != nil {
			t.Fatalf("failed to set initial custom title: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Clear it by setting to null
		body := map[string]interface{}{"custom_title": nil}
		resp, err := client.Patch("/api/v1/sessions/"+sessionID+"/title", body)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result db.SessionDetail
		testutil.ParseJSON(t, resp, &result)

		if result.CustomTitle != nil {
			t.Errorf("expected custom_title to be nil, got %v", *result.CustomTitle)
		}
	})

	t.Run("returns 403 for session owned by another user", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@example.com", "Other")
		otherSession := testutil.CreateTestWebSessionWithToken(t, env, other.ID)

		sessionID := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(otherSession)

		body := map[string]interface{}{"custom_title": "Hacked Title"}
		resp, err := client.Patch("/api/v1/sessions/"+sessionID+"/title", body)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("returns 401 without session cookie", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "session-1")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		body := map[string]interface{}{"custom_title": "Test"}
		resp, err := client.Patch("/api/v1/sessions/"+sessionID+"/title", body)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// filippo.io/csrf uses Fetch metadata headers for CSRF validation,
		// so browser-like requests pass CSRF and fail at auth middleware
		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}

// =============================================================================
// GET /api/v1/sessions/by-external-id/{external_id} - Lookup by external ID
// =============================================================================

func TestLookupSessionByExternalID_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("finds session by external ID with API key auth", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSession(t, env, user.ID, "lookup-test-123")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/by-external-id/lookup-test-123")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			SessionID string `json:"session_id"`
		}
		testutil.ParseJSON(t, resp, &result)

		if result.SessionID != sessionID {
			t.Errorf("expected session_id %s, got %s", sessionID, result.SessionID)
		}
	})

	t.Run("finds session by external ID with session auth", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		webSession := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		sessionID := testutil.CreateTestSession(t, env, user.ID, "lookup-test-456")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(webSession)

		resp, err := client.Get("/api/v1/sessions/by-external-id/lookup-test-456")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			SessionID string `json:"session_id"`
		}
		testutil.ParseJSON(t, resp, &result)

		if result.SessionID != sessionID {
			t.Errorf("expected session_id %s, got %s", sessionID, result.SessionID)
		}
	})

	t.Run("returns 404 for non-existent external ID", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/by-external-id/non-existent-id")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No auth

		resp, err := client.Get("/api/v1/sessions/by-external-id/some-id")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}
