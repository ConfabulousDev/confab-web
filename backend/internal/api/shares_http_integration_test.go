package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// Session Shares HTTP Integration Tests
//
// These tests run against a real HTTP server with the production router.
// Share endpoints require session auth + CSRF for state-changing operations.
// =============================================================================

// setupSharesTestServer creates a test server for shares tests
func setupSharesTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
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

	apiServer := NewServer(env.DB, env.Storage, oauthConfig, nil)
	handler := apiServer.SetupRoutes()

	return testutil.StartTestServer(t, env, handler)
}

// =============================================================================
// POST /api/v1/sessions/{id}/share - Create session share
// =============================================================================

func TestCreateShare_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates public share successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-123"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupSharesTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateShareRequest{
			IsPublic: true,
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/share", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result CreateShareResponse
		testutil.ParseJSON(t, resp, &result)

		if !result.IsPublic {
			t.Errorf("expected is_public true, got false")
		}
		if result.ShareURL == "" {
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

	t.Run("creates recipient share with emails", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-456"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupSharesTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateShareRequest{
			IsPublic:   false,
			Recipients: []string{"friend@example.com", "colleague@example.com"},
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/share", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result CreateShareResponse
		testutil.ParseJSON(t, resp, &result)

		if result.IsPublic {
			t.Errorf("expected is_public false, got true")
		}
		if len(result.Recipients) != 2 {
			t.Errorf("expected 2 recipients, got %d", len(result.Recipients))
		}

		// Verify recipients in database
		var emailCount int
		row := env.DB.QueryRow(env.Ctx,
			`SELECT COUNT(*) FROM session_share_recipients ssr
			 JOIN session_shares ss ON ssr.share_id = ss.id
			 WHERE ss.session_id = $1`,
			sessionID)
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
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupSharesTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateShareRequest{
			IsPublic: true,
		}

		resp, err := client.Post("/api/v1/sessions/non-existent-uuid/share", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("requires recipients for non-public shares", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-abc"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupSharesTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateShareRequest{
			IsPublic:   false,
			Recipients: []string{}, // Empty - should fail
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/share", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("creates share with skip_notifications", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-skip-notify"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupSharesTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateShareRequest{
			IsPublic:          false,
			Recipients:        []string{"friend@example.com", "colleague@example.com"},
			SkipNotifications: true,
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/share", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result CreateShareResponse
		testutil.ParseJSON(t, resp, &result)

		if result.IsPublic {
			t.Errorf("expected is_public false, got true")
		}
		if len(result.Recipients) != 2 {
			t.Errorf("expected 2 recipients, got %d", len(result.Recipients))
		}
		if result.EmailsSent {
			t.Error("expected emails_sent false when skip_notifications is true")
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test")
		externalID := "test-session-unauth"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupSharesTestServer(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		reqBody := CreateShareRequest{
			IsPublic: true,
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/share", reqBody)
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
// System Share Tests (database-level tests)
// =============================================================================

func TestSystemShare_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates system share successfully", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-system"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		// Create system share using database function directly
		share, err := env.DB.CreateSystemShare(env.Ctx, sessionID, nil)
		if err != nil {
			t.Fatalf("failed to create system share: %v", err)
		}

		if share.IsPublic {
			t.Error("system share should not be marked as public")
		}

		// Verify system share entry in database
		var count int
		row := env.DB.QueryRow(env.Ctx,
			`SELECT COUNT(*) FROM session_share_system sss
			 JOIN session_shares ss ON sss.share_id = ss.id
			 WHERE ss.id = $1`,
			share.ID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query system shares: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 system share entry, got %d", count)
		}
	})

	t.Run("system share allows any authenticated user", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-system-access"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		// Create system share
		_, err := env.DB.CreateSystemShare(env.Ctx, sessionID, nil)
		if err != nil {
			t.Fatalf("failed to create system share: %v", err)
		}

		// Create a different user
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

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		externalID := "test-session-system-noauth"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		// Create system share
		_, err := env.DB.CreateSystemShare(env.Ctx, sessionID, nil)
		if err != nil {
			t.Fatalf("failed to create system share: %v", err)
		}

		// Unauthenticated access should fail
		accessInfo, err := env.DB.GetSessionAccessType(env.Ctx, sessionID, nil)
		if err != nil {
			t.Fatalf("GetSessionAccessType failed: %v", err)
		}
		if accessInfo.AccessType != db.SessionAccessNone {
			t.Errorf("expected SessionAccessNone for unauthenticated access, got %v", accessInfo.AccessType)
		}
	})
}
