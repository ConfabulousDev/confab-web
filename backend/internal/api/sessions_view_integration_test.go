package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/testutil"
)

// TestHandleListSessions_Integration tests listing sessions with real database
func TestHandleListSessions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("lists all sessions for user", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Create test sessions
		testutil.CreateTestSession(t, env, user.ID, "session-1")
		testutil.CreateTestSession(t, env, user.ID, "session-2")
		testutil.CreateTestSession(t, env, user.ID, "session-3")

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions", nil, user.ID)

		w := httptest.NewRecorder()
		handler := HandleListSessions(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var sessions []db.SessionListItem
		testutil.ParseJSONResponse(t, w, &sessions)

		if len(sessions) != 3 {
			t.Errorf("expected 3 sessions, got %d", len(sessions))
		}

		// Verify external IDs are present
		externalIDs := make(map[string]bool)
		for _, session := range sessions {
			externalIDs[session.ExternalID] = true
		}

		if !externalIDs["session-1"] {
			t.Error("expected session-1 in response")
		}
		if !externalIDs["session-2"] {
			t.Error("expected session-2 in response")
		}
		if !externalIDs["session-3"] {
			t.Error("expected session-3 in response")
		}
	})

	t.Run("returns empty array when user has no sessions", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions", nil, user.ID)

		w := httptest.NewRecorder()
		handler := HandleListSessions(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var sessions []db.SessionListItem
		testutil.ParseJSONResponse(t, w, &sessions)

		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(sessions))
		}
	})

	t.Run("only returns sessions belonging to authenticated user", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User Two")

		// Create sessions for both users
		testutil.CreateTestSession(t, env, user1.ID, "user1-session")
		testutil.CreateTestSession(t, env, user2.ID, "user2-session")

		// Request as user1
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions", nil, user1.ID)

		w := httptest.NewRecorder()
		handler := HandleListSessions(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var sessions []db.SessionListItem
		testutil.ParseJSONResponse(t, w, &sessions)

		if len(sessions) != 1 {
			t.Errorf("expected 1 session for user1, got %d", len(sessions))
		}

		if sessions[0].ExternalID != "user1-session" {
			t.Errorf("expected 'user1-session', got %s", sessions[0].ExternalID)
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions", nil, 0)
		req = req.WithContext(context.Background())

		w := httptest.NewRecorder()
		handler := HandleListSessions(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})
}

// TestHandleGetSession_Integration tests getting session details with real database
func TestHandleGetSession_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns session details successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		externalID := "test-session-detail"

		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Create a run for the session
		runID := testutil.CreateTestRun(t, env, sessionID, "Test reason", "/home/test", "transcript.jsonl")

		// Create files for the run
		testutil.CreateTestFile(t, env, runID, "transcript.jsonl", "transcript", "s3-key-1", 100)
		testutil.CreateTestFile(t, env, runID, "agent.jsonl", "agent", "s3-key-2", 200)

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSession(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var session db.SessionDetail
		testutil.ParseJSONResponse(t, w, &session)

		if session.ExternalID != externalID {
			t.Errorf("expected external_id %s, got %s", externalID, session.ExternalID)
		}

		if len(session.Runs) == 0 {
			t.Error("expected at least one run")
		} else {
			if session.Runs[0].Reason != "Test reason" {
				t.Errorf("expected reason 'Test reason', got %s", session.Runs[0].Reason)
			}

			if len(session.Runs[0].Files) != 2 {
				t.Errorf("expected 2 files, got %d", len(session.Runs[0].Files))
			}
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		nonExistentID := "non-existent-session"

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+nonExistentID, nil, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", nonExistentID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSession(env.DB)
		handler(w, req)

		testutil.AssertErrorResponse(t, w, http.StatusNotFound, "Session not found")
	})

	t.Run("prevents accessing another user's session", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User Two")

		externalID := "user2-session"
		sessionID := testutil.CreateTestSession(t, env, user2.ID, externalID)

		// Try to access as user1
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, user1.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSession(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("returns 404 for invalid session UUID", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Invalid UUID format - not a valid UUID
		invalidSessionID := "not-a-valid-uuid"

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+invalidSessionID, nil, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", invalidSessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSession(env.DB)
		handler(w, req)

		// Invalid UUID returns 404 (session not found) not 400
		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("returns 400 for empty session ID", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/", nil, user.ID)

		// Add empty URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSession(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		sessionID := "test-session"

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/sessions/"+sessionID, nil, 0)
		req = req.WithContext(context.Background())

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSession(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})
}
