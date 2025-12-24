package api

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// GitHub Links HTTP Integration Tests
//
// These tests run against a real HTTP server with the production router.
// =============================================================================

// setupGitHubLinksTestServer creates a test server for GitHub links tests
func setupGitHubLinksTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
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
// POST /api/v1/sessions/{id}/github-links - Create GitHub link
// =============================================================================

func TestCreateGitHubLink_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates PR link successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-123"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		title := "Add new feature"
		reqBody := CreateGitHubLinkRequest{
			URL:    "https://github.com/owner/repo/pull/123",
			Title:  &title,
			Source: models.GitHubLinkSourceManual,
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/github-links", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusCreated)

		var result models.GitHubLink
		testutil.ParseJSON(t, resp, &result)

		if result.LinkType != models.GitHubLinkTypePullRequest {
			t.Errorf("expected link_type 'pull_request', got %s", result.LinkType)
		}
		if result.Owner != "owner" {
			t.Errorf("expected owner 'owner', got %s", result.Owner)
		}
		if result.Repo != "repo" {
			t.Errorf("expected repo 'repo', got %s", result.Repo)
		}
		if result.Ref != "123" {
			t.Errorf("expected ref '123', got %s", result.Ref)
		}
		if result.Title == nil || *result.Title != "Add new feature" {
			t.Error("expected title 'Add new feature'")
		}
		if result.Source != models.GitHubLinkSourceManual {
			t.Errorf("expected source 'manual', got %s", result.Source)
		}

		// Verify database state
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM session_github_links WHERE session_id = $1",
			sessionID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query github links: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 github link in database, got %d", count)
		}
	})

	t.Run("creates commit link successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-456"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateGitHubLinkRequest{
			URL:    "https://github.com/owner/repo/commit/abc123def456",
			Source: models.GitHubLinkSourceCLIHook,
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/github-links", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusCreated)

		var result models.GitHubLink
		testutil.ParseJSON(t, resp, &result)

		if result.LinkType != models.GitHubLinkTypeCommit {
			t.Errorf("expected link_type 'commit', got %s", result.LinkType)
		}
		if result.Ref != "abc123def456" {
			t.Errorf("expected ref 'abc123def456', got %s", result.Ref)
		}
	})

	t.Run("creates link via API key", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKeyInfo := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "test-key")
		externalID := "test-session-api"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKeyInfo.RawToken)

		reqBody := CreateGitHubLinkRequest{
			URL:    "https://github.com/owner/repo/pull/999",
			Source: models.GitHubLinkSourceCLIHook,
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/github-links", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusCreated)
	})

	t.Run("returns 409 for duplicate link", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-dup"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateGitHubLinkRequest{
			URL:    "https://github.com/owner/repo/pull/123",
			Source: models.GitHubLinkSourceManual,
		}

		// First request should succeed
		resp1, err := client.Post("/api/v1/sessions/"+sessionID+"/github-links", reqBody)
		if err != nil {
			t.Fatalf("first request failed: %v", err)
		}
		resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusCreated)

		// Second request should fail with 409
		resp2, err := client.Post("/api/v1/sessions/"+sessionID+"/github-links", reqBody)
		if err != nil {
			t.Fatalf("second request failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusConflict)
	})

	t.Run("returns 400 for invalid URL", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-bad-url"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateGitHubLinkRequest{
			URL:    "https://example.com/not-github",
			Source: models.GitHubLinkSourceManual,
		}

		resp, err := client.Post("/api/v1/sessions/"+sessionID+"/github-links", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		reqBody := CreateGitHubLinkRequest{
			URL:    "https://github.com/owner/repo/pull/123",
			Source: models.GitHubLinkSourceManual,
		}

		resp, err := client.Post("/api/v1/sessions/non-existent-uuid/github-links", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})
}

// =============================================================================
// GET /api/v1/sessions/{id}/github-links - List GitHub links
// =============================================================================

func TestListGitHubLinks_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("lists links for owner", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-list"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Create some links directly in DB
		title1 := "First PR"
		link1 := &models.GitHubLink{
			SessionID: sessionID,
			LinkType:  models.GitHubLinkTypePullRequest,
			URL:       "https://github.com/owner/repo/pull/1",
			Owner:     "owner",
			Repo:      "repo",
			Ref:       "1",
			Title:     &title1,
			Source:    models.GitHubLinkSourceManual,
		}
		_, err := env.DB.CreateGitHubLink(env.Ctx, link1)
		if err != nil {
			t.Fatalf("failed to create link: %v", err)
		}

		link2 := &models.GitHubLink{
			SessionID: sessionID,
			LinkType:  models.GitHubLinkTypeCommit,
			URL:       "https://github.com/owner/repo/commit/abc123",
			Owner:     "owner",
			Repo:      "repo",
			Ref:       "abc123",
			Source:    models.GitHubLinkSourceCLIHook,
		}
		_, err = env.DB.CreateGitHubLink(env.Ctx, link2)
		if err != nil {
			t.Fatalf("failed to create link: %v", err)
		}

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/github-links")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			Links []models.GitHubLink `json:"links"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.Links) != 2 {
			t.Errorf("expected 2 links, got %d", len(result.Links))
		}
	})

	t.Run("lists links for shared session", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
		viewerToken := testutil.CreateTestWebSessionWithToken(t, env, viewer.ID)
		externalID := "test-session-shared"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		// Create public share
		_, err := env.DB.CreateShare(env.Ctx, sessionID, owner.ID, true, nil, nil)
		if err != nil {
			t.Fatalf("failed to create share: %v", err)
		}

		// Create a link
		link := &models.GitHubLink{
			SessionID: sessionID,
			LinkType:  models.GitHubLinkTypePullRequest,
			URL:       "https://github.com/owner/repo/pull/1",
			Owner:     "owner",
			Repo:      "repo",
			Ref:       "1",
			Source:    models.GitHubLinkSourceManual,
		}
		_, err = env.DB.CreateGitHubLink(env.Ctx, link)
		if err != nil {
			t.Fatalf("failed to create link: %v", err)
		}

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(viewerToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/github-links")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			Links []models.GitHubLink `json:"links"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.Links) != 1 {
			t.Errorf("expected 1 link, got %d", len(result.Links))
		}
	})

	t.Run("returns empty array for session with no links", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-empty"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/github-links")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result struct {
			Links []models.GitHubLink `json:"links"`
		}
		testutil.ParseJSON(t, resp, &result)

		if len(result.Links) != 0 {
			t.Errorf("expected 0 links, got %d", len(result.Links))
		}
	})

	t.Run("returns 404 for non-shared session by non-owner", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
		viewerToken := testutil.CreateTestWebSessionWithToken(t, env, viewer.ID)
		externalID := "test-session-private"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(viewerToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/github-links")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})
}

// =============================================================================
// DELETE /api/v1/sessions/{id}/github-links/{linkID} - Delete GitHub link
// =============================================================================

func TestDeleteGitHubLink_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("deletes link successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-del"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Create a link
		link := &models.GitHubLink{
			SessionID: sessionID,
			LinkType:  models.GitHubLinkTypePullRequest,
			URL:       "https://github.com/owner/repo/pull/1",
			Owner:     "owner",
			Repo:      "repo",
			Ref:       "1",
			Source:    models.GitHubLinkSourceManual,
		}
		createdLink, err := env.DB.CreateGitHubLink(env.Ctx, link)
		if err != nil {
			t.Fatalf("failed to create link: %v", err)
		}

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Delete("/api/v1/sessions/" + sessionID + "/github-links/" + fmt.Sprintf("%d", createdLink.ID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNoContent)

		// Verify database state
		var count int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM session_github_links WHERE session_id = $1",
			sessionID)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("failed to query github links: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 github links in database, got %d", count)
		}
	})

	t.Run("returns 404 for non-existent link", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-session-notfound"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Delete("/api/v1/sessions/" + sessionID + "/github-links/99999")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 404 when deleting link from different session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		sessionID1 := testutil.CreateTestSession(t, env, user.ID, "session-1")
		sessionID2 := testutil.CreateTestSession(t, env, user.ID, "session-2")

		// Create a link on session 1
		link := &models.GitHubLink{
			SessionID: sessionID1,
			LinkType:  models.GitHubLinkTypePullRequest,
			URL:       "https://github.com/owner/repo/pull/1",
			Owner:     "owner",
			Repo:      "repo",
			Ref:       "1",
			Source:    models.GitHubLinkSourceManual,
		}
		createdLink, err := env.DB.CreateGitHubLink(env.Ctx, link)
		if err != nil {
			t.Fatalf("failed to create link: %v", err)
		}

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Try to delete via session 2 - should fail
		resp, err := client.Delete("/api/v1/sessions/" + sessionID2 + "/github-links/" + fmt.Sprintf("%d", createdLink.ID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 404 for non-owner trying to delete", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
		viewerToken := testutil.CreateTestWebSessionWithToken(t, env, viewer.ID)
		externalID := "test-session-noauth"
		sessionID := testutil.CreateTestSession(t, env, owner.ID, externalID)

		// Create a link
		link := &models.GitHubLink{
			SessionID: sessionID,
			LinkType:  models.GitHubLinkTypePullRequest,
			URL:       "https://github.com/owner/repo/pull/1",
			Owner:     "owner",
			Repo:      "repo",
			Ref:       "1",
			Source:    models.GitHubLinkSourceManual,
		}
		createdLink, err := env.DB.CreateGitHubLink(env.Ctx, link)
		if err != nil {
			t.Fatalf("failed to create link: %v", err)
		}

		ts := setupGitHubLinksTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(viewerToken)

		resp, err := client.Delete("/api/v1/sessions/" + sessionID + "/github-links/" + fmt.Sprintf("%d", createdLink.ID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})
}

// =============================================================================
// ParseGitHubURL Unit Tests
// =============================================================================

func TestParseGitHubURL(t *testing.T) {
	t.Run("parses PR URL", func(t *testing.T) {
		result, err := ParseGitHubURL("https://github.com/owner/repo/pull/123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.LinkType != models.GitHubLinkTypePullRequest {
			t.Errorf("expected pull_request, got %s", result.LinkType)
		}
		if result.Owner != "owner" {
			t.Errorf("expected owner 'owner', got %s", result.Owner)
		}
		if result.Repo != "repo" {
			t.Errorf("expected repo 'repo', got %s", result.Repo)
		}
		if result.Ref != "123" {
			t.Errorf("expected ref '123', got %s", result.Ref)
		}
	})

	t.Run("parses commit URL", func(t *testing.T) {
		result, err := ParseGitHubURL("https://github.com/owner/repo/commit/abc123def")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.LinkType != models.GitHubLinkTypeCommit {
			t.Errorf("expected commit, got %s", result.LinkType)
		}
		if result.Ref != "abc123def" {
			t.Errorf("expected ref 'abc123def', got %s", result.Ref)
		}
	})

	t.Run("parses URL with HTTP", func(t *testing.T) {
		result, err := ParseGitHubURL("http://github.com/owner/repo/pull/456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Ref != "456" {
			t.Errorf("expected ref '456', got %s", result.Ref)
		}
	})

	t.Run("fails for non-GitHub URL", func(t *testing.T) {
		_, err := ParseGitHubURL("https://gitlab.com/owner/repo/pull/123")
		if err == nil {
			t.Error("expected error for non-GitHub URL")
		}
	})

	t.Run("fails for GitHub URL without PR or commit", func(t *testing.T) {
		_, err := ParseGitHubURL("https://github.com/owner/repo")
		if err == nil {
			t.Error("expected error for non-PR/commit URL")
		}
	})

	t.Run("fails for issues URL", func(t *testing.T) {
		_, err := ParseGitHubURL("https://github.com/owner/repo/issues/123")
		if err == nil {
			t.Error("expected error for issues URL")
		}
	})
}
