package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
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

	t.Run("returns session details with sync files", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		externalID := "test-session-detail"

		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Create sync files for the session
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
		testutil.CreateTestSyncFile(t, env, sessionID, "agent.jsonl", "agent", 200)

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

		if len(session.Files) != 2 {
			t.Errorf("expected 2 files, got %d", len(session.Files))
		}

		// Verify files
		filesByName := make(map[string]db.SyncFileDetail)
		for _, f := range session.Files {
			filesByName[f.FileName] = f
		}

		if f, ok := filesByName["transcript.jsonl"]; !ok {
			t.Error("expected transcript.jsonl file")
		} else if f.LastSyncedLine != 100 {
			t.Errorf("expected 100 synced lines, got %d", f.LastSyncedLine)
		}

		if f, ok := filesByName["agent.jsonl"]; !ok {
			t.Error("expected agent.jsonl file")
		} else if f.LastSyncedLine != 200 {
			t.Errorf("expected 200 synced lines, got %d", f.LastSyncedLine)
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

	t.Run("returns 404 for unauthenticated request without public share", func(t *testing.T) {
		env.CleanDB(t)

		// Create a session but no public share
		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Unauthenticated request (no session cookie)
		req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSession(env.DB)
		handler(w, req)

		// Should return 404 (not 401) to not reveal session existence
		testutil.AssertStatus(t, w, http.StatusNotFound)
	})
}

// TestHandleUpdateSessionTitle_Integration tests updating session custom title
func TestHandleUpdateSessionTitle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("successfully sets custom title", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "session-1")

		customTitle := "My Custom Title"
		body := map[string]interface{}{"custom_title": customTitle}
		req := testutil.AuthenticatedRequest(t, "PATCH", "/api/v1/sessions/"+sessionID+"/title", body, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleUpdateSessionTitle(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var result db.SessionDetail
		testutil.ParseJSONResponse(t, w, &result)

		if result.CustomTitle == nil || *result.CustomTitle != customTitle {
			t.Errorf("expected custom_title %q, got %v", customTitle, result.CustomTitle)
		}
	})

	t.Run("clears custom title when null", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "session-1")

		// First set a custom title
		customTitle := "My Custom Title"
		ctx := context.Background()
		err := env.DB.UpdateSessionCustomTitle(ctx, sessionID, user.ID, &customTitle)
		if err != nil {
			t.Fatalf("failed to set initial custom title: %v", err)
		}

		// Now clear it
		body := map[string]interface{}{"custom_title": nil}
		req := testutil.AuthenticatedRequest(t, "PATCH", "/api/v1/sessions/"+sessionID+"/title", body, user.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleUpdateSessionTitle(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var result db.SessionDetail
		testutil.ParseJSONResponse(t, w, &result)

		if result.CustomTitle != nil {
			t.Errorf("expected custom_title to be nil, got %v", *result.CustomTitle)
		}
	})

	t.Run("rejects title over 255 characters", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "session-1")

		// Create a title that's too long (256 characters)
		longTitle := ""
		for i := 0; i < 256; i++ {
			longTitle += "a"
		}

		body := map[string]interface{}{"custom_title": longTitle}
		req := testutil.AuthenticatedRequest(t, "PATCH", "/api/v1/sessions/"+sessionID+"/title", body, user.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleUpdateSessionTitle(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Use a valid UUID format that doesn't exist in the database
		nonExistentUUID := "00000000-0000-0000-0000-000000000000"

		body := map[string]interface{}{"custom_title": "Test"}
		req := testutil.AuthenticatedRequest(t, "PATCH", "/api/v1/sessions/"+nonExistentUUID+"/title", body, user.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", nonExistentUUID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleUpdateSessionTitle(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("returns 403 for session owned by another user", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@example.com", "Other")
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "session-1")

		body := map[string]interface{}{"custom_title": "Hacked Title"}
		req := testutil.AuthenticatedRequest(t, "PATCH", "/api/v1/sessions/"+sessionID+"/title", body, other.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleUpdateSessionTitle(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusForbidden)
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "session-1")

		body := map[string]interface{}{"custom_title": "Test"}
		req := testutil.AuthenticatedRequest(t, "PATCH", "/api/v1/sessions/"+sessionID+"/title", body, 0)

		// Remove auth context to simulate unauthenticated request
		req = req.WithContext(context.Background())

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleUpdateSessionTitle(env.DB)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})
}
