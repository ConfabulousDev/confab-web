package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/models"
	"github.com/santaclaude2025/confab/backend/internal/testutil"
)

// TestSessionIsolation_Integration contains adversarial tests to verify
// that sessions are properly isolated between users. These tests should
// FAIL before the (user_id, session_id) composite key refactor and
// PASS after.

func TestSessionIsolation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("two users can have sessions with same session_id", func(t *testing.T) {
		env.CleanDB(t)

		// Create two different users
		alice := testutil.CreateTestUser(t, env, "alice@example.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@example.com", "Bob")

		// Both users create a session with the SAME session ID
		sharedSessionID := "shared-session-id-123"

		// Alice creates her session
		aliceReq := models.SaveSessionRequest{
			SessionID:      sharedSessionID,
			TranscriptPath: "alice-transcript.jsonl",
			CWD:            "/home/alice/project",
			Reason:         "Alice's session",
			Files: []models.FileUpload{
				{
					Path:      "transcript.jsonl",
					Type:      "transcript",
					Content:   []byte(`{"type":"user","message":"Alice's message"}`),
					SizeBytes: 44,
				},
			},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", aliceReq, alice.ID)
		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSaveSession(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Bob creates HIS session with the same ID
		bobReq := models.SaveSessionRequest{
			SessionID:      sharedSessionID,
			TranscriptPath: "bob-transcript.jsonl",
			CWD:            "/home/bob/project",
			Reason:         "Bob's session",
			Files: []models.FileUpload{
				{
					Path:      "transcript.jsonl",
					Type:      "transcript",
					Content:   []byte(`{"type":"user","message":"Bob's message"}`),
					SizeBytes: 42,
				},
			},
		}

		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", bobReq, bob.ID)
		w = httptest.NewRecorder()
		server.handleSaveSession(w, req)

		// This MUST succeed - Bob should be able to create his own session
		// with the same ID without conflicting with Alice's
		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify both sessions exist independently
		var aliceSessionCount, bobSessionCount int

		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sessions WHERE session_id = $1 AND user_id = $2",
			sharedSessionID, alice.ID)
		if err := row.Scan(&aliceSessionCount); err != nil {
			t.Fatalf("failed to query Alice's session: %v", err)
		}

		row = env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sessions WHERE session_id = $1 AND user_id = $2",
			sharedSessionID, bob.ID)
		if err := row.Scan(&bobSessionCount); err != nil {
			t.Fatalf("failed to query Bob's session: %v", err)
		}

		if aliceSessionCount != 1 {
			t.Errorf("expected 1 session for Alice, got %d", aliceSessionCount)
		}
		if bobSessionCount != 1 {
			t.Errorf("expected 1 session for Bob, got %d", bobSessionCount)
		}
	})

	t.Run("user cannot view another user's session with same session_id", func(t *testing.T) {
		env.CleanDB(t)

		alice := testutil.CreateTestUser(t, env, "alice@example.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@example.com", "Bob")

		// Alice creates a session
		sessionID := "private-session-456"
		testutil.CreateTestSession(t, env, alice.ID, sessionID)
		testutil.CreateTestRun(t, env, sessionID, alice.ID, "Alice's run", "/home/alice", "transcript.jsonl")

		// Bob tries to view Alice's session using the same session ID
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, bob.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionId", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSession(env.DB)
		handler(w, req)

		// Bob should get 404 - session doesn't exist FOR HIM
		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("user cannot delete another user's session with same session_id", func(t *testing.T) {
		env.CleanDB(t)

		alice := testutil.CreateTestUser(t, env, "alice@example.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@example.com", "Bob")

		// Alice creates a session
		sessionID := "delete-test-session"
		testutil.CreateTestSession(t, env, alice.ID, sessionID)
		runID := testutil.CreateTestRun(t, env, sessionID, alice.ID, "Alice's run", "/home/alice", "transcript.jsonl")
		testutil.CreateTestFile(t, env, runID, "transcript.jsonl", "transcript", "alice/"+sessionID+"/transcript.jsonl", 100)

		// Bob tries to delete Alice's session
		// Send empty JSON body (required by the handler)
		req := testutil.AuthenticatedRequest(t, "DELETE", "/api/v1/sessions/"+sessionID, map[string]interface{}{}, bob.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionId", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleDeleteSessionOrRun(env.DB, env.Storage)
		handler(w, req)

		// Bob should get 404 or 403 - cannot delete Alice's session
		if w.Code != http.StatusNotFound && w.Code != http.StatusForbidden {
			t.Errorf("expected 404 or 403, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Verify Alice's session still exists
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sessions WHERE session_id = $1 AND user_id = $2",
			sessionID, alice.ID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query session: %v", err)
		}
		if count != 1 {
			t.Error("Alice's session was deleted by Bob!")
		}
	})

	t.Run("user cannot create share for another user's session", func(t *testing.T) {
		env.CleanDB(t)

		alice := testutil.CreateTestUser(t, env, "alice@example.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@example.com", "Bob")

		// Alice creates a session
		sessionID := "share-test-session"
		testutil.CreateTestSession(t, env, alice.ID, sessionID)

		// Bob tries to create a share for Alice's session
		reqBody := CreateShareRequest{
			Visibility: "public",
		}
		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/"+sessionID+"/share", reqBody, bob.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionId", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleCreateShare(env.DB, "https://confab.dev")
		handler(w, req)

		// Bob should get 404 or 403 - session doesn't exist for him
		// After refactor: should be 404 to avoid information disclosure
		// Before refactor: may be 403 due to ownership check
		if w.Code != http.StatusNotFound && w.Code != http.StatusForbidden {
			t.Errorf("expected 404 or 403, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("user cannot list shares for another user's session", func(t *testing.T) {
		env.CleanDB(t)

		alice := testutil.CreateTestUser(t, env, "alice@example.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@example.com", "Bob")

		// Alice creates a session with a share
		sessionID := "list-shares-test"
		testutil.CreateTestSession(t, env, alice.ID, sessionID)
		testutil.CreateTestShare(t, env, sessionID, alice.ID, testutil.GenerateShareToken(), "public", nil, nil)

		// Bob tries to list Alice's shares
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID+"/shares", nil, bob.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionId", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleListShares(env.DB)
		handler(w, req)

		// Bob should get 404 or 403 - session doesn't exist for him
		if w.Code != http.StatusNotFound && w.Code != http.StatusForbidden {
			t.Errorf("expected 404 or 403, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("uploading to existing session_id does not modify another user's session", func(t *testing.T) {
		env.CleanDB(t)

		alice := testutil.CreateTestUser(t, env, "alice@example.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@example.com", "Bob")

		// Alice creates a session with a specific title
		sessionID := "title-test-session"
		testutil.CreateTestSession(t, env, alice.ID, sessionID)

		// Set Alice's session title
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET title = 'Alice Original Title' WHERE session_id = $1 AND user_id = $2",
			sessionID, alice.ID)
		if err != nil {
			t.Fatalf("failed to set title: %v", err)
		}

		// Bob uploads a session with the same ID (should create Bob's own session)
		bobReq := models.SaveSessionRequest{
			SessionID:      sessionID,
			TranscriptPath: "transcript.jsonl",
			CWD:            "/home/bob",
			Reason:         "Bob's upload",
			Files: []models.FileUpload{
				{
					Path:      "transcript.jsonl",
					Type:      "transcript",
					Content:   []byte(`{"type":"summary","summary":"Bob's New Title"}`),
					SizeBytes: 44,
				},
			},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", bobReq, bob.ID)
		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSaveSession(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify Alice's title was NOT modified
		var aliceTitle string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT title FROM sessions WHERE session_id = $1 AND user_id = $2",
			sessionID, alice.ID)
		if err := row.Scan(&aliceTitle); err != nil {
			t.Fatalf("failed to query Alice's title: %v", err)
		}

		if aliceTitle != "Alice Original Title" {
			t.Errorf("Alice's title was modified! Expected 'Alice Original Title', got '%s'", aliceTitle)
		}
	})

	t.Run("session listing only shows user's own sessions", func(t *testing.T) {
		env.CleanDB(t)

		alice := testutil.CreateTestUser(t, env, "alice@example.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@example.com", "Bob")

		// Create sessions for both users
		testutil.CreateTestSession(t, env, alice.ID, "alice-session-1")
		testutil.CreateTestSession(t, env, alice.ID, "alice-session-2")
		testutil.CreateTestSession(t, env, bob.ID, "bob-session-1")
		testutil.CreateTestSession(t, env, bob.ID, "shared-id") // Same ID as below
		testutil.CreateTestSession(t, env, alice.ID, "shared-id") // Same ID as above

		// List Alice's sessions
		sessions, err := env.DB.ListUserSessions(env.Ctx, alice.ID, false)
		if err != nil {
			t.Fatalf("failed to list sessions: %v", err)
		}

		// Alice should see exactly 3 sessions (alice-session-1, alice-session-2, shared-id)
		if len(sessions) != 3 {
			t.Errorf("expected 3 sessions for Alice, got %d", len(sessions))
		}

		// Verify none of Bob's unique sessions appear
		for _, s := range sessions {
			if s.SessionID == "bob-session-1" {
				t.Error("Alice can see Bob's session!")
			}
		}
	})

	t.Run("deleting user cascades only their sessions", func(t *testing.T) {
		env.CleanDB(t)

		alice := testutil.CreateTestUser(t, env, "alice@example.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@example.com", "Bob")

		// Both users have sessions with the same ID
		sessionID := "cascade-test"
		testutil.CreateTestSession(t, env, alice.ID, sessionID)
		testutil.CreateTestSession(t, env, bob.ID, sessionID)
		testutil.CreateTestRun(t, env, sessionID, alice.ID, "Alice run", "/alice", "t.jsonl")
		testutil.CreateTestRun(t, env, sessionID, bob.ID, "Bob run", "/bob", "t.jsonl")

		// Delete Alice's user
		_, err := env.DB.Exec(env.Ctx, "DELETE FROM users WHERE id = $1", alice.ID)
		if err != nil {
			t.Fatalf("failed to delete user: %v", err)
		}

		// Bob's session should still exist
		var bobSessionCount int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sessions WHERE session_id = $1 AND user_id = $2",
			sessionID, bob.ID)
		if err := row.Scan(&bobSessionCount); err != nil {
			t.Fatalf("failed to query Bob's session: %v", err)
		}

		if bobSessionCount != 1 {
			t.Errorf("Bob's session was deleted when Alice was deleted! Count: %d", bobSessionCount)
		}

		// Bob's run should still exist
		var bobRunCount int
		row = env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM runs WHERE session_id = $1 AND user_id = $2",
			sessionID, bob.ID)
		if err := row.Scan(&bobRunCount); err != nil {
			t.Fatalf("failed to query Bob's runs: %v", err)
		}

		if bobRunCount != 1 {
			t.Errorf("Bob's run was deleted when Alice was deleted! Count: %d", bobRunCount)
		}
	})
}
