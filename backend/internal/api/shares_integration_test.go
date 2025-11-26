package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/testutil"
)

// TestHandleCreateShare_Integration tests the HandleCreateShare handler with real database
func TestHandleCreateShare_Integration(t *testing.T) {
	// Skip if running unit tests only (go test -short)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment (PostgreSQL + MinIO containers)
	env := testutil.SetupTestEnvironment(t)

	t.Run("creates public share successfully", func(t *testing.T) {
		// Clean DB state for test isolation
		env.CleanDB(t)

		// Create test data
		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		externalID := "test-session-123"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Create request
		reqBody := CreateShareRequest{
			Visibility: "public",
		}
		req := testutil.AuthenticatedRequest(t, "POST",
			"/api/v1/sessions/"+sessionID+"/share", reqBody, user.ID)

		// Setup chi router with URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Execute handler
		w := httptest.NewRecorder()
		handler := HandleCreateShare(env.DB, "https://confab.dev", nil)
		handler(w, req)

		// Assert response
		testutil.AssertStatus(t, w, http.StatusOK)

		var resp CreateShareResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Visibility != "public" {
			t.Errorf("expected visibility 'public', got %s", resp.Visibility)
		}
		if resp.ShareToken == "" {
			t.Error("expected share token, got empty")
		}
		if len(resp.ShareToken) != 32 {
			t.Errorf("expected share token length 32, got %d", len(resp.ShareToken))
		}
		if resp.ShareURL == "" {
			t.Error("expected share URL, got empty")
		}

		// Verify database state
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM session_shares WHERE session_id = $1",
			sessionID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query shares: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 share in database, got %d", count)
		}
	})

	t.Run("creates private share with invited emails", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-456"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		reqBody := CreateShareRequest{
			Visibility:    "private",
			InvitedEmails: []string{"friend@example.com", "colleague@example.com"},
		}
		req := testutil.AuthenticatedRequest(t, "POST",
			"/api/v1/sessions/"+sessionID+"/share", reqBody, user.ID)

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleCreateShare(env.DB, "https://confab.dev", nil)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp CreateShareResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.Visibility != "private" {
			t.Errorf("expected visibility 'private', got %s", resp.Visibility)
		}
		if len(resp.InvitedEmails) != 2 {
			t.Errorf("expected 2 invited emails, got %d", len(resp.InvitedEmails))
		}

		// Verify invited emails in database
		var emailCount int
		row := env.DB.QueryRow(env.Ctx,
			`SELECT COUNT(*) FROM session_share_invites ssi
			 JOIN session_shares ss ON ssi.share_id = ss.id
			 WHERE ss.share_token = $1`,
			resp.ShareToken)
		if err := row.Scan(&emailCount); err != nil {
			t.Fatalf("failed to query invited emails: %v", err)
		}
		if emailCount != 2 {
			t.Errorf("expected 2 invited emails in database, got %d", emailCount)
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test")
		nonExistentID := "non-existent-uuid"

		reqBody := CreateShareRequest{
			Visibility: "public",
		}
		req := testutil.AuthenticatedRequest(t, "POST",
			"/api/v1/sessions/"+nonExistentID+"/share", reqBody, user.ID)

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", nonExistentID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleCreateShare(env.DB, "https://confab.dev", nil)
		handler(w, req)

		testutil.AssertErrorResponse(t, w, http.StatusNotFound, "Session not found")
	})

	t.Run("validates visibility field", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test")
		externalID := "test-session-789"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		reqBody := CreateShareRequest{
			Visibility: "invalid",
		}
		req := testutil.AuthenticatedRequest(t, "POST",
			"/api/v1/sessions/"+sessionID+"/share", reqBody, user.ID)

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleCreateShare(env.DB, "https://confab.dev", nil)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if resp["error"] == "" {
			t.Error("expected error message for invalid visibility")
		}
	})

	t.Run("requires invited emails for private shares", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test")
		externalID := "test-session-abc"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		reqBody := CreateShareRequest{
			Visibility:    "private",
			InvitedEmails: []string{}, // Empty - should fail
		}
		req := testutil.AuthenticatedRequest(t, "POST",
			"/api/v1/sessions/"+sessionID+"/share", reqBody, user.ID)

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleCreateShare(env.DB, "https://confab.dev", nil)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)
	})
}
