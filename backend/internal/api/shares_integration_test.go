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
			IsPublic: true,
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

		if !resp.IsPublic {
			t.Errorf("expected is_public true, got false")
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

		// Verify database state - check session_shares and session_share_public
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

		// Verify public flag
		row = env.DB.QueryRow(env.Ctx,
			`SELECT COUNT(*) FROM session_share_public ssp
			 JOIN session_shares ss ON ssp.share_id = ss.id
			 WHERE ss.share_token = $1`,
			resp.ShareToken)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query public shares: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 public share entry, got %d", count)
		}
	})

	t.Run("creates recipient share with emails", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-456"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		reqBody := CreateShareRequest{
			IsPublic:   false,
			Recipients: []string{"friend@example.com", "colleague@example.com"},
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

		if resp.IsPublic {
			t.Errorf("expected is_public false, got true")
		}
		if len(resp.Recipients) != 2 {
			t.Errorf("expected 2 recipients, got %d", len(resp.Recipients))
		}

		// Verify recipients in database
		var emailCount int
		row := env.DB.QueryRow(env.Ctx,
			`SELECT COUNT(*) FROM session_share_recipients ssr
			 JOIN session_shares ss ON ssr.share_id = ss.id
			 WHERE ss.share_token = $1`,
			resp.ShareToken)
		if err := row.Scan(&emailCount); err != nil {
			t.Fatalf("failed to query recipients: %v", err)
		}
		if emailCount != 2 {
			t.Errorf("expected 2 recipients in database, got %d", emailCount)
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test")
		nonExistentID := "non-existent-uuid"

		reqBody := CreateShareRequest{
			IsPublic: true,
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

	t.Run("requires recipients for non-public shares", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test")
		externalID := "test-session-abc"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		reqBody := CreateShareRequest{
			IsPublic:   false,
			Recipients: []string{}, // Empty - should fail
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

	t.Run("creates recipient share with skip_notifications", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-skip-notify"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		reqBody := CreateShareRequest{
			IsPublic:          false,
			Recipients:        []string{"friend@example.com", "colleague@example.com"},
			SkipNotifications: true, // Skip sending emails
		}
		req := testutil.AuthenticatedRequest(t, "POST",
			"/api/v1/sessions/"+sessionID+"/share", reqBody, user.ID)

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		// Pass nil email service to simulate no email service configured
		handler := HandleCreateShare(env.DB, "https://confab.dev", nil)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp CreateShareResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Share should be created successfully
		if resp.IsPublic {
			t.Errorf("expected is_public false, got true")
		}
		if len(resp.Recipients) != 2 {
			t.Errorf("expected 2 recipients, got %d", len(resp.Recipients))
		}
		if resp.ShareToken == "" {
			t.Error("expected share token, got empty")
		}

		// EmailsSent should be false when skip_notifications is true
		if resp.EmailsSent {
			t.Error("expected emails_sent false when skip_notifications is true")
		}

		// Verify recipients are still stored in database
		var emailCount int
		row := env.DB.QueryRow(env.Ctx,
			`SELECT COUNT(*) FROM session_share_recipients ssr
			 JOIN session_shares ss ON ssr.share_id = ss.id
			 WHERE ss.share_token = $1`,
			resp.ShareToken)
		if err := row.Scan(&emailCount); err != nil {
			t.Fatalf("failed to query recipients: %v", err)
		}
		if emailCount != 2 {
			t.Errorf("expected 2 recipients in database, got %d", emailCount)
		}
	})
}

// TestSystemShare_Integration tests system share creation and access
func TestSystemShare_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates system share successfully", func(t *testing.T) {
		env.CleanDB(t)

		// Create a user and session
		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-system"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		// Create system share using database function directly
		shareToken, err := GenerateShareToken()
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		share, err := env.DB.CreateSystemShare(env.Ctx, sessionID, shareToken, nil)
		if err != nil {
			t.Fatalf("failed to create system share: %v", err)
		}

		if share.ShareToken != shareToken {
			t.Errorf("expected share token %s, got %s", shareToken, share.ShareToken)
		}
		if share.IsPublic {
			t.Error("system share should not be marked as public")
		}

		// Verify system share entry in database
		var count int
		row := env.DB.QueryRow(env.Ctx,
			`SELECT COUNT(*) FROM session_share_system sss
			 JOIN session_shares ss ON sss.share_id = ss.id
			 WHERE ss.share_token = $1`,
			shareToken)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query system shares: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 system share entry, got %d", count)
		}
	})

	t.Run("system share allows any authenticated user", func(t *testing.T) {
		env.CleanDB(t)

		// Create owner and session
		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-system-access"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		// Create system share
		shareToken, _ := GenerateShareToken()
		_, err := env.DB.CreateSystemShare(env.Ctx, sessionID, shareToken, nil)
		if err != nil {
			t.Fatalf("failed to create system share: %v", err)
		}

		// Create a different user (not the owner, not a recipient)
		otherUser := testutil.CreateTestUser(t, env, "other@example.com", "Other User")

		// Other user should be able to access via canonical access
		accessInfo, err := env.DB.GetSessionAccessType(env.Ctx, sessionID, &otherUser.ID)
		if err != nil {
			t.Fatalf("authenticated user should be able to access system share: %v", err)
		}
		if accessInfo.AccessType != db.SessionAccessSystem {
			t.Errorf("expected SessionAccessSystem, got %v", accessInfo.AccessType)
		}
	})

	t.Run("system share requires authentication", func(t *testing.T) {
		env.CleanDB(t)

		// Create owner and session
		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-system-noauth"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		// Create system share
		shareToken, _ := GenerateShareToken()
		_, err := env.DB.CreateSystemShare(env.Ctx, sessionID, shareToken, nil)
		if err != nil {
			t.Fatalf("failed to create system share: %v", err)
		}

		// Unauthenticated access (nil viewerUserID) should fail - system shares require auth
		accessInfo, err := env.DB.GetSessionAccessType(env.Ctx, sessionID, nil)
		if err != nil {
			t.Fatalf("GetSessionAccessType failed: %v", err)
		}
		if accessInfo.AccessType != db.SessionAccessNone {
			t.Errorf("expected SessionAccessNone for unauthenticated access to system share, got %v", accessInfo.AccessType)
		}
	})
}
