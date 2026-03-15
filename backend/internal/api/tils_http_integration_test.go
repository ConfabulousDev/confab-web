package api

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func setupTILsTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
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

	apiServer := NewServer(env.DB, env.Storage, &oauthConfig, nil, "")
	handler := apiServer.SetupRoutes()

	return testutil.StartTestServer(t, env, handler)
}

// readBody reads and returns the response body as a string without closing it
// (for tests that need to inspect the body after RequireStatus).
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	return string(body)
}

// =============================================================================
// POST /api/v1/tils - Create TIL (API key auth)
// =============================================================================

func TestCreateTIL_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("creates TIL successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "creator@test.com", "Creator")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "test-key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-create-1")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		msgUUID := "msg-uuid-123"
		reqBody := createTILRequest{
			Title:       "Learned about channels",
			Summary:     "Go channels are great",
			SessionID:   sessionID,
			MessageUUID: &msgUUID,
		}

		resp, err := client.Post("/api/v1/tils", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusCreated)

		var result models.TIL
		testutil.ParseJSON(t, resp, &result)

		if result.ID == 0 {
			t.Error("expected non-zero ID")
		}
		if result.Title != "Learned about channels" {
			t.Errorf("expected title 'Learned about channels', got %q", result.Title)
		}
		if result.SessionID != sessionID {
			t.Errorf("expected session_id %q, got %q", sessionID, result.SessionID)
		}
	})

	// --- Input validation (prevents stored payloads) ---

	t.Run("rejects missing title", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "creator@test.com", "Creator")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "test-key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-no-title")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := createTILRequest{
			Summary:   "Summary without title",
			SessionID: sessionID,
		}

		resp, err := client.Post("/api/v1/tils", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects missing summary", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "creator@test.com", "Creator")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "test-key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-no-summary")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := createTILRequest{
			Title:     "Title without summary",
			SessionID: sessionID,
		}

		resp, err := client.Post("/api/v1/tils", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects title exceeding 500 chars", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "creator@test.com", "Creator")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "test-key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-long-title")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := createTILRequest{
			Title:     strings.Repeat("x", 501),
			Summary:   "Valid summary",
			SessionID: sessionID,
		}

		resp, err := client.Post("/api/v1/tils", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects summary exceeding 10000 chars", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "creator@test.com", "Creator")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "test-key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-long-summary")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := createTILRequest{
			Title:     "Valid title",
			Summary:   strings.Repeat("x", 10001),
			SessionID: sessionID,
		}

		resp, err := client.Post("/api/v1/tils", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	// --- Authorization: session ownership ---

	t.Run("rejects session not owned by caller", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, other.ID, "other-key")
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "ext-not-owned")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := createTILRequest{
			Title:     "Should fail",
			Summary:   "Not my session",
			SessionID: sessionID,
		}

		resp, err := client.Post("/api/v1/tils", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("rejects creation on shared session (not owner)", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, viewer.ID, "viewer-key")
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "ext-shared-create")

		// Share session with viewer — viewer has access but is NOT owner
		testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"viewer@test.com"})

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := createTILRequest{
			Title:     "Should fail even with share access",
			Summary:   "Shared but not owned",
			SessionID: sessionID,
		}

		resp, err := client.Post("/api/v1/tils", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// GetSessionDetail checks user_id = $2, so shared access doesn't help
		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})
}

// =============================================================================
// GET /api/v1/tils - List TILs (web session auth)
// =============================================================================

func TestListTILs_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("lists own TILs", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "lister@test.com", "Lister")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-list-1", testutil.TestSessionFullOpts{Summary: "s"})

		testutil.CreateTestTIL(t, env, user.ID, sessionID, "My TIL", "My Summary", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/tils")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			TILs    []map[string]interface{} `json:"tils"`
			HasMore bool                     `json:"has_more"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL, got %d", len(result.TILs))
		}
		if result.TILs[0]["title"] != "My TIL" {
			t.Errorf("expected title 'My TIL', got %v", result.TILs[0]["title"])
		}
	})

	// --- Visibility: inaccessible sessions excluded ---

	t.Run("excludes TILs on inaccessible sessions", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		// User's own session with TIL
		ownSession := testutil.CreateTestSessionFull(t, env, user.ID, "ext-own", testutil.TestSessionFullOpts{Summary: "own"})
		testutil.CreateTestTIL(t, env, user.ID, ownSession, "My TIL", "visible", nil)

		// Other user's UNSHARED session with TIL — must NOT appear
		otherSession := testutil.CreateTestSessionFull(t, env, other.ID, "ext-other", testutil.TestSessionFullOpts{Summary: "other"})
		testutil.CreateTestTIL(t, env, other.ID, otherSession, "Secret TIL", "should not see this", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/tils")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			TILs []map[string]interface{} `json:"tils"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.TILs) != 1 {
			t.Fatalf("expected exactly 1 TIL (own only), got %d", len(result.TILs))
		}
		if result.TILs[0]["title"] != "My TIL" {
			t.Errorf("expected 'My TIL', got %v", result.TILs[0]["title"])
		}
	})

	// --- Visibility: system-shared sessions included ---

	t.Run("includes TILs on system-shared sessions", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		// Other user's session with a system share
		otherSession := testutil.CreateTestSessionFull(t, env, other.ID, "ext-sys", testutil.TestSessionFullOpts{Summary: "system shared"})
		testutil.CreateTestTIL(t, env, other.ID, otherSession, "System Shared TIL", "visible via system share", nil)
		testutil.CreateTestSystemShare(t, env, otherSession, nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/tils")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			TILs []map[string]interface{} `json:"tils"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL from system share, got %d", len(result.TILs))
		}
		if result.TILs[0]["title"] != "System Shared TIL" {
			t.Errorf("expected 'System Shared TIL', got %v", result.TILs[0]["title"])
		}
	})
}

// =============================================================================
// GET /api/v1/tils/{id} - Get TIL
// =============================================================================

func TestGetTIL_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns TIL for owner", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "getter@test.com", "Getter")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-get-1")
		tilID := testutil.CreateTestTIL(t, env, user.ID, sessionID, "Get Me", "Get Summary", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/tils/" + strconv.FormatInt(tilID, 10))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result models.TIL
		testutil.ParseJSON(t, resp, &result)

		if result.Title != "Get Me" {
			t.Errorf("expected title 'Get Me', got %q", result.Title)
		}
	})

	t.Run("response does not contain owner_id", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "getter@test.com", "Getter")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-no-owner-id")
		tilID := testutil.CreateTestTIL(t, env, user.ID, sessionID, "Check Wire Format", "verify internal fields stripped", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/tils/" + strconv.FormatInt(tilID, 10))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		body := readBody(t, resp)
		resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)
		if strings.Contains(body, `"owner_id"`) {
			t.Errorf("response contains \"owner_id\" key — internal field leaked to wire format: %s", body)
		}
	})

	t.Run("returns 404 for nonexistent TIL", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "getter@test.com", "Getter")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/tils/99999")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	// --- OptionalAuth: unauthenticated access on public session ---

	t.Run("unauthenticated user can read TIL on public session", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "ext-public-1")
		tilID := testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Public TIL", "Anyone can see", nil)

		// Create a public share on the session
		testutil.CreateTestShare(t, env, sessionID, true, nil, nil)

		ts := setupTILsTestServer(t, env)
		// No auth — raw client without session or API key
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Get("/api/v1/tils/" + strconv.FormatInt(tilID, 10))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Must succeed — endpoint is in OptionalAuth group, public share grants access
		testutil.RequireStatus(t, resp, http.StatusOK)

		var result models.TIL
		testutil.ParseJSON(t, resp, &result)
		if result.Title != "Public TIL" {
			t.Errorf("expected 'Public TIL', got %q", result.Title)
		}
	})

	// --- Authorization: session access check ---

	t.Run("returns 404 for TIL on inaccessible session", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		otherToken := testutil.CreateTestWebSessionWithToken(t, env, other.ID)
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "ext-private-1")
		tilID := testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Private TIL", "Secret stuff", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(otherToken)

		resp, err := client.Get("/api/v1/tils/" + strconv.FormatInt(tilID, 10))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	// --- No existence leak: uniform 404 body ---

	t.Run("uniform 404 body for nonexistent vs inaccessible TIL", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		otherToken := testutil.CreateTestWebSessionWithToken(t, env, other.ID)
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "ext-uniform-1")
		tilID := testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Private TIL", "Secret", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(otherToken)

		// Request 1: TIL exists but session is inaccessible
		resp1, err := client.Get("/api/v1/tils/" + strconv.FormatInt(tilID, 10))
		if err != nil {
			t.Fatalf("request 1 failed: %v", err)
		}
		body1 := readBody(t, resp1)
		resp1.Body.Close()

		// Request 2: TIL does not exist at all
		resp2, err := client.Get("/api/v1/tils/99999")
		if err != nil {
			t.Fatalf("request 2 failed: %v", err)
		}
		body2 := readBody(t, resp2)
		resp2.Body.Close()

		if resp1.StatusCode != resp2.StatusCode {
			t.Errorf("status codes differ: inaccessible=%d, nonexistent=%d", resp1.StatusCode, resp2.StatusCode)
		}
		if body1 != body2 {
			t.Errorf("response bodies differ — existence leak!\ninaccessible: %s\nnonexistent:  %s", body1, body2)
		}
	})
}

// =============================================================================
// DELETE /api/v1/tils/{id} - Delete TIL
// =============================================================================

func TestDeleteTIL_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("deletes own TIL", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "deleter@test.com", "Deleter")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-del-1")
		tilID := testutil.CreateTestTIL(t, env, user.ID, sessionID, "Delete Me", "Summary", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Delete("/api/v1/tils/" + strconv.FormatInt(tilID, 10))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNoContent)
	})

	t.Run("returns 404 for nonexistent TIL", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "deleter@test.com", "Deleter")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Delete("/api/v1/tils/99999")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	// --- Authorization: only owner can delete ---

	t.Run("returns 404 for other user's TIL (no existence leak)", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		otherToken := testutil.CreateTestWebSessionWithToken(t, env, other.ID)
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "ext-forbid-1")
		tilID := testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Not Yours", "Summary", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(otherToken)

		resp, err := client.Delete("/api/v1/tils/" + strconv.FormatInt(tilID, 10))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Returns 404 (not 403) to avoid confirming TIL existence
		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	// --- No existence leak: uniform 404 body ---

	t.Run("uniform 404 body for nonexistent vs non-owner TIL", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		otherToken := testutil.CreateTestWebSessionWithToken(t, env, other.ID)
		sessionID := testutil.CreateTestSession(t, env, owner.ID, "ext-uniform-del")
		tilID := testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Not Yours", "Summary", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(otherToken)

		// Request 1: TIL exists but not owned
		resp1, err := client.Delete("/api/v1/tils/" + strconv.FormatInt(tilID, 10))
		if err != nil {
			t.Fatalf("request 1 failed: %v", err)
		}
		body1 := readBody(t, resp1)
		resp1.Body.Close()

		// Request 2: TIL does not exist
		resp2, err := client.Delete("/api/v1/tils/99999")
		if err != nil {
			t.Fatalf("request 2 failed: %v", err)
		}
		body2 := readBody(t, resp2)
		resp2.Body.Close()

		if resp1.StatusCode != resp2.StatusCode {
			t.Errorf("status codes differ: not-owner=%d, nonexistent=%d", resp1.StatusCode, resp2.StatusCode)
		}
		if body1 != body2 {
			t.Errorf("response bodies differ — existence leak!\nnot-owner:   %s\nnonexistent: %s", body1, body2)
		}
	})
}

// =============================================================================
// GET /api/v1/sessions/{id}/tils - Session TILs (canonical access)
// =============================================================================

func TestListSessionTILs_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("owner sees session TILs", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-stils-1", testutil.TestSessionFullOpts{Summary: "s"})

		testutil.CreateTestTIL(t, env, user.ID, sessionID, "Session TIL", "summary", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/tils")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			TILs []models.TIL `json:"tils"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL, got %d", len(result.TILs))
		}
	})

	// --- Authorization: canonical access check ---

	t.Run("returns 404 for inaccessible session", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		viewerToken := testutil.CreateTestWebSessionWithToken(t, env, viewer.ID)
		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-noaccess-1", testutil.TestSessionFullOpts{Summary: "s"})

		// Create a TIL on owner's session — viewer must NOT see it
		testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Hidden TIL", "should not leak", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(viewerToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/tils")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("shared user sees session TILs", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		viewerToken := testutil.CreateTestWebSessionWithToken(t, env, viewer.ID)
		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-shared-1", testutil.TestSessionFullOpts{Summary: "s"})

		testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Shared Session TIL", "visible", nil)
		testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"viewer@test.com"})

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(viewerToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/tils")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			TILs []models.TIL `json:"tils"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL, got %d", len(result.TILs))
		}
	})

	t.Run("system share grants access to session TILs", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		viewerToken := testutil.CreateTestWebSessionWithToken(t, env, viewer.ID)
		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-sys-1", testutil.TestSessionFullOpts{Summary: "s"})

		testutil.CreateTestTIL(t, env, owner.ID, sessionID, "System Shared TIL", "visible", nil)
		testutil.CreateTestSystemShare(t, env, sessionID, nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(viewerToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/tils")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			TILs []models.TIL `json:"tils"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL via system share, got %d", len(result.TILs))
		}
	})

	t.Run("empty array when no TILs", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "empty@test.com", "Empty")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-empty-1", testutil.TestSessionFullOpts{Summary: "s"})

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/tils")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			TILs []models.TIL `json:"tils"`
		}
		testutil.ParseJSON(t, resp, &result)

		if result.TILs == nil {
			t.Error("expected empty array, got nil")
		}
		if len(result.TILs) != 0 {
			t.Errorf("expected 0 TILs, got %d", len(result.TILs))
		}
	})
}
