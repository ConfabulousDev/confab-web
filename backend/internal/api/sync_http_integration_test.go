package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// HTTP Integration Tests
//
// These tests run against a real HTTP server with the production router.
// They exercise the full middleware chain including:
//   - Authentication (API key / session cookie)
//   - Rate limiting
//   - CORS
//   - Request body size limits
//   - Content-Type validation
//
// =============================================================================

// setupTestServerWithEnv creates a test server with proper environment variables
func setupTestServerWithEnv(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()

	// Set required environment variables
	testutil.SetEnvForTest(t, "CSRF_SECRET_KEY", "test-csrf-secret-key-32-bytes!!")
	testutil.SetEnvForTest(t, "ALLOWED_ORIGINS", "http://localhost:3000")
	testutil.SetEnvForTest(t, "FRONTEND_URL", "http://localhost:3000")
	testutil.SetEnvForTest(t, "INSECURE_DEV_MODE", "true")

	// Create the API server
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
// POST /api/v1/sync/init - Initialize or resume sync session
// =============================================================================

func TestSyncInit_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	// Disable logging during tests
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates new session with valid API key", func(t *testing.T) {
		env.CleanDB(t)

		// Setup
		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Execute
		reqBody := SyncInitRequest{
			ExternalID:     "new-session-123",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Assert
		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncInitResponse
		testutil.ParseJSON(t, resp, &result)

		if result.SessionID == "" {
			t.Error("expected non-empty session_id")
		}
		if result.Files == nil {
			t.Error("expected files map to be initialized")
		}
		if len(result.Files) != 0 {
			t.Errorf("expected 0 files for new session, got %d", len(result.Files))
		}

		// Verify database state
		var sessionCount int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sessions WHERE external_id = $1 AND user_id = $2",
			"new-session-123", user.ID)
		if err := row.Scan(&sessionCount); err != nil {
			t.Fatalf("failed to query sessions: %v", err)
		}
		if sessionCount != 1 {
			t.Errorf("expected 1 session in database, got %d", sessionCount)
		}
	})

	t.Run("returns 401 without API key", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No API key

		reqBody := SyncInitRequest{
			ExternalID:     "test-session",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("returns 401 with invalid API key", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey("cfb_invalid_api_key_that_does_not_exist_1234")

		reqBody := SyncInitRequest{
			ExternalID:     "test-session",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("returns 400 for missing external_id", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncInitRequest{
			ExternalID:     "", // Missing
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp, &result)

		if !strings.Contains(result["error"], "external_id") {
			t.Errorf("expected error about external_id, got: %s", result["error"])
		}
	})

	t.Run("returns 400 for missing transcript_path", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncInitRequest{
			ExternalID:     "test-session",
			TranscriptPath: "", // Missing
			CWD:            "/home/user/project",
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp, &result)

		if !strings.Contains(result["error"], "transcript_path") {
			t.Errorf("expected error about transcript_path, got: %s", result["error"])
		}
	})

	t.Run("resumes existing session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		// Create existing session with synced files
		sessionID := testutil.CreateTestSession(t, env, user.ID, "existing-session-456")
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 150)
		testutil.CreateTestSyncFile(t, env, sessionID, "agent-abc123.jsonl", "agent", 50)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncInitRequest{
			ExternalID:     "existing-session-456",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncInitResponse
		testutil.ParseJSON(t, resp, &result)

		// Verify session ID is returned
		if result.SessionID != sessionID {
			t.Errorf("expected session_id %s, got %s", sessionID, result.SessionID)
		}

		// Verify sync state for both files
		if len(result.Files) != 2 {
			t.Errorf("expected 2 files in sync state, got %d", len(result.Files))
		}

		transcriptState, ok := result.Files["transcript.jsonl"]
		if !ok {
			t.Error("expected transcript.jsonl in files map")
		} else if transcriptState.LastSyncedLine != 150 {
			t.Errorf("expected last_synced_line 150 for transcript, got %d", transcriptState.LastSyncedLine)
		}
	})

	t.Run("isolates sessions between users", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")
		apiKey1 := testutil.CreateTestAPIKeyWithToken(t, env, user1.ID, "User1 Key")
		apiKey2 := testutil.CreateTestAPIKeyWithToken(t, env, user2.ID, "User2 Key")

		ts := setupTestServerWithEnv(t, env)

		reqBody := SyncInitRequest{
			ExternalID:     "shared-external-id",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		// User1 creates a session
		client1 := testutil.NewTestClient(t, ts).WithAPIKey(apiKey1.RawToken)
		resp1, err := client1.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp1, http.StatusOK)

		var result1 SyncInitResponse
		testutil.ParseJSON(t, resp1, &result1)

		// User2 creates a session with same external_id (should be different session)
		client2 := testutil.NewTestClient(t, ts).WithAPIKey(apiKey2.RawToken)
		resp2, err := client2.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp2, http.StatusOK)

		var result2 SyncInitResponse
		testutil.ParseJSON(t, resp2, &result2)

		// Session IDs should be different
		if result1.SessionID == result2.SessionID {
			t.Error("expected different session IDs for different users with same external_id")
		}
	})
}

// =============================================================================
// POST /api/v1/sync/chunk - Upload a chunk of lines
// =============================================================================

func TestSyncChunk_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("uploads first chunk successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-chunk")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		lines := []string{
			`{"type":"user","message":"Hello"}`,
			`{"type":"assistant","message":"Hi there!"}`,
			`{"type":"user","message":"How are you?"}`,
		}

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     lines,
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncChunkResponse
		testutil.ParseJSON(t, resp, &result)

		// Verify high-water mark updated
		if result.LastSyncedLine != 3 {
			t.Errorf("expected last_synced_line 3, got %d", result.LastSyncedLine)
		}

		// Verify sync_files table updated
		var lastSyncedLine int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT last_synced_line FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, "transcript.jsonl")
		if err := row.Scan(&lastSyncedLine); err != nil {
			t.Fatalf("failed to query sync_files: %v", err)
		}
		if lastSyncedLine != 3 {
			t.Errorf("expected last_synced_line 3 in DB, got %d", lastSyncedLine)
		}
	})

	t.Run("returns 401 without API key", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No API key

		reqBody := SyncChunkRequest{
			SessionID: "some-session-id",
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user"}`},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("returns 403 for session owned by another user", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")
		apiKey2 := testutil.CreateTestAPIKeyWithToken(t, env, user2.ID, "User2 Key")

		// Session owned by user1
		sessionID := testutil.CreateTestSession(t, env, user1.ID, "user1-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey2.RawToken)

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user"}`},
		}

		// User2 tries to upload to user1's session
		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("rejects overlapping chunk", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-overlap")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
		}

		// First upload
		resp1, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusOK)

		// Try to re-upload same range - should be rejected as overlap
		resp2, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp2, &result)

		if !strings.Contains(result["error"], "first_line must be 2") {
			t.Errorf("expected error about first_line must be 2, got: %s", result["error"])
		}
	})

	t.Run("updates git_info from chunk metadata", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-gitinfo")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		gitInfo := json.RawMessage(`{"repo_url":"https://github.com/test/repo.git","branch":"feature-branch"}`)
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
			Metadata: &SyncChunkMetadata{
				GitInfo: gitInfo,
			},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify git_info was stored in the session
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT git_info FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&storedGitInfo); err != nil {
			t.Fatalf("failed to query session git_info: %v", err)
		}

		var gitData map[string]string
		if err := json.Unmarshal(storedGitInfo, &gitData); err != nil {
			t.Fatalf("failed to parse stored git_info: %v", err)
		}

		if gitData["repo_url"] != "https://github.com/test/repo.git" {
			t.Errorf("expected repo_url 'https://github.com/test/repo.git', got %q", gitData["repo_url"])
		}
		if gitData["branch"] != "feature-branch" {
			t.Errorf("expected branch 'feature-branch', got %q", gitData["branch"])
		}
	})

	t.Run("updates git_info on subsequent chunks", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-gitinfo-update")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// First chunk with initial git_info
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
			Metadata: &SyncChunkMetadata{
				GitInfo: json.RawMessage(`{"repo_url":"https://github.com/test/repo.git","branch":"main"}`),
			},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		// Second chunk with updated branch (simulating branch switch)
		reqBody = SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 2,
			Lines:     []string{`{"type":"assistant","message":"Hi!"}`},
			Metadata: &SyncChunkMetadata{
				GitInfo: json.RawMessage(`{"repo_url":"https://github.com/test/repo.git","branch":"feature-new"}`),
			},
		}

		resp, err = client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify git_info reflects the latest update
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT git_info FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&storedGitInfo); err != nil {
			t.Fatalf("failed to query session git_info: %v", err)
		}

		var gitData map[string]string
		if err := json.Unmarshal(storedGitInfo, &gitData); err != nil {
			t.Fatalf("failed to parse stored git_info: %v", err)
		}

		if gitData["branch"] != "feature-new" {
			t.Errorf("expected branch 'feature-new' after update, got %q", gitData["branch"])
		}
	})

	t.Run("does not update git_info for agent files", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-agent-no-git")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// First set git_info via transcript
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
			Metadata: &SyncChunkMetadata{
				GitInfo: json.RawMessage(`{"repo_url":"https://github.com/test/repo.git","branch":"main"}`),
			},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		// Upload agent file chunk with different git_info (should be ignored)
		agentReq := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "agent-abc123.jsonl",
			FileType:  "agent",
			FirstLine: 1,
			Lines:     []string{`{"type":"tool_use"}`},
			Metadata: &SyncChunkMetadata{
				GitInfo: json.RawMessage(`{"repo_url":"https://github.com/test/repo.git","branch":"should-be-ignored"}`),
			},
		}

		resp, err = client.Post("/api/v1/sync/chunk", agentReq)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify git_info still has original branch (agent metadata was ignored)
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT git_info FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&storedGitInfo); err != nil {
			t.Fatalf("failed to query session git_info: %v", err)
		}

		var gitData map[string]string
		if err := json.Unmarshal(storedGitInfo, &gitData); err != nil {
			t.Fatalf("failed to parse stored git_info: %v", err)
		}

		if gitData["branch"] != "main" {
			t.Errorf("expected branch 'main' (agent metadata should be ignored), got %q", gitData["branch"])
		}
	})
}

// =============================================================================
// Health check endpoint
// =============================================================================

func TestHealth_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts)

	resp, err := client.Get("/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	testutil.RequireStatus(t, resp, http.StatusOK)

	var result map[string]string
	testutil.ParseJSON(t, resp, &result)

	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", result["status"])
	}
}

// =============================================================================
// POST /api/v1/sync/init - Metadata Nesting Tests (backward compatibility)
// =============================================================================

func TestSyncInit_MetadataNesting_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("new format: reads cwd and git_info from metadata", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Use map to send new nested format
		reqBody := map[string]interface{}{
			"external_id":     "new-format-session",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"metadata": map[string]interface{}{
				"cwd":      "/home/user/new-format-project",
				"git_info": map[string]string{"branch": "main", "repo_url": "https://github.com/test/repo.git"},
			},
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncInitResponse
		testutil.ParseJSON(t, resp, &result)

		if result.SessionID == "" {
			t.Error("expected non-empty session_id")
		}

		// Verify cwd and git_info were stored correctly
		var storedCWD string
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd, git_info FROM sessions WHERE id = $1",
			result.SessionID)
		if err := row.Scan(&storedCWD, &storedGitInfo); err != nil {
			t.Fatalf("failed to query session: %v", err)
		}

		if storedCWD != "/home/user/new-format-project" {
			t.Errorf("expected cwd '/home/user/new-format-project', got %q", storedCWD)
		}

		var gitData map[string]string
		if err := json.Unmarshal(storedGitInfo, &gitData); err != nil {
			t.Fatalf("failed to parse stored git_info: %v", err)
		}
		if gitData["branch"] != "main" {
			t.Errorf("expected branch 'main', got %q", gitData["branch"])
		}
	})

	t.Run("old format: reads cwd and git_info from top-level (backward compat)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Use the old struct format (top-level cwd and git_info)
		reqBody := SyncInitRequest{
			ExternalID:     "old-format-session",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/old-format-project",
			GitInfo:        json.RawMessage(`{"branch":"feature","repo_url":"https://github.com/test/old.git"}`),
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncInitResponse
		testutil.ParseJSON(t, resp, &result)

		// Verify cwd and git_info were stored correctly from top-level fields
		var storedCWD string
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd, git_info FROM sessions WHERE id = $1",
			result.SessionID)
		if err := row.Scan(&storedCWD, &storedGitInfo); err != nil {
			t.Fatalf("failed to query session: %v", err)
		}

		if storedCWD != "/home/user/old-format-project" {
			t.Errorf("expected cwd '/home/user/old-format-project', got %q", storedCWD)
		}

		var gitData map[string]string
		if err := json.Unmarshal(storedGitInfo, &gitData); err != nil {
			t.Fatalf("failed to parse stored git_info: %v", err)
		}
		if gitData["branch"] != "feature" {
			t.Errorf("expected branch 'feature', got %q", gitData["branch"])
		}
	})

	t.Run("mixed format: metadata takes precedence over top-level", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Send both top-level AND metadata - metadata should win
		reqBody := map[string]interface{}{
			"external_id":     "mixed-format-session",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"cwd":             "/home/user/top-level-cwd",        // Should be ignored
			"git_info":        map[string]string{"branch": "old"}, // Should be ignored
			"metadata": map[string]interface{}{
				"cwd":      "/home/user/metadata-cwd",
				"git_info": map[string]string{"branch": "new"},
			},
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncInitResponse
		testutil.ParseJSON(t, resp, &result)

		// Verify metadata values took precedence
		var storedCWD string
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd, git_info FROM sessions WHERE id = $1",
			result.SessionID)
		if err := row.Scan(&storedCWD, &storedGitInfo); err != nil {
			t.Fatalf("failed to query session: %v", err)
		}

		if storedCWD != "/home/user/metadata-cwd" {
			t.Errorf("expected metadata cwd '/home/user/metadata-cwd', got %q", storedCWD)
		}

		var gitData map[string]string
		if err := json.Unmarshal(storedGitInfo, &gitData); err != nil {
			t.Fatalf("failed to parse stored git_info: %v", err)
		}
		if gitData["branch"] != "new" {
			t.Errorf("expected metadata branch 'new', got %q", gitData["branch"])
		}
	})

	t.Run("empty metadata object falls back to top-level", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Send top-level with empty metadata - should use top-level
		reqBody := map[string]interface{}{
			"external_id":     "empty-metadata-session",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"cwd":             "/home/user/fallback-cwd",
			"git_info":        map[string]string{"branch": "fallback"},
			"metadata":        map[string]interface{}{}, // Empty metadata
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncInitResponse
		testutil.ParseJSON(t, resp, &result)

		// Verify top-level values were used as fallback
		var storedCWD string
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd, git_info FROM sessions WHERE id = $1",
			result.SessionID)
		if err := row.Scan(&storedCWD, &storedGitInfo); err != nil {
			t.Fatalf("failed to query session: %v", err)
		}

		if storedCWD != "/home/user/fallback-cwd" {
			t.Errorf("expected fallback cwd '/home/user/fallback-cwd', got %q", storedCWD)
		}

		var gitData map[string]string
		if err := json.Unmarshal(storedGitInfo, &gitData); err != nil {
			t.Fatalf("failed to parse stored git_info: %v", err)
		}
		if gitData["branch"] != "fallback" {
			t.Errorf("expected fallback branch 'fallback', got %q", gitData["branch"])
		}
	})

	t.Run("null metadata falls back to top-level", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Send top-level with null metadata
		reqBody := map[string]interface{}{
			"external_id":     "null-metadata-session",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"cwd":             "/home/user/null-fallback-cwd",
			"git_info":        map[string]string{"branch": "null-fallback"},
			"metadata":        nil,
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncInitResponse
		testutil.ParseJSON(t, resp, &result)

		// Verify top-level values were used as fallback
		var storedCWD string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd FROM sessions WHERE id = $1",
			result.SessionID)
		if err := row.Scan(&storedCWD); err != nil {
			t.Fatalf("failed to query session: %v", err)
		}

		if storedCWD != "/home/user/null-fallback-cwd" {
			t.Errorf("expected fallback cwd '/home/user/null-fallback-cwd', got %q", storedCWD)
		}
	})

	t.Run("cwd validation applies regardless of field location", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Create an extremely long cwd to trigger validation error
		longCWD := "/" + strings.Repeat("a", 9000) // Exceeds 8192 limit

		// Test with metadata.cwd
		reqBody := map[string]interface{}{
			"external_id":     "cwd-validation-test",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"metadata": map[string]interface{}{
				"cwd": longCWD,
			},
		}

		resp, err := client.Post("/api/v1/sync/init", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp, &result)

		if !strings.Contains(result["error"], "cwd") {
			t.Errorf("expected error about cwd, got: %s", result["error"])
		}
	})
}

// =============================================================================
// POST /api/v1/sync/chunk - Additional Edge Cases
// =============================================================================

func TestSyncChunk_EdgeCases_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("uploads subsequent chunk successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-subsequent")

		// Simulate existing sync state (first 100 lines already synced)
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 100)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		lines := []string{
			`{"type":"user","message":"Line 101"}`,
			`{"type":"assistant","message":"Line 102"}`,
		}

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 101,
			Lines:     lines,
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SyncChunkResponse
		testutil.ParseJSON(t, resp, &result)

		if result.LastSyncedLine != 102 {
			t.Errorf("expected last_synced_line 102, got %d", result.LastSyncedLine)
		}
	})

	t.Run("rejects chunk with gap (skipped lines)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-gap")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload first chunk (lines 1-2)
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines: []string{
				`{"type":"user","message":"Line 1"}`,
				`{"type":"assistant","message":"Line 2"}`,
			},
		}

		resp1, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusOK)

		// Try to upload chunk starting at line 5 (gap - should start at 3)
		reqBody.FirstLine = 5
		reqBody.Lines = []string{`{"type":"user","message":"Line 5"}`}

		resp2, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp2, &result)

		if !strings.Contains(result["error"], "first_line must be 3") {
			t.Errorf("expected error about first_line must be 3, got: %s", result["error"])
		}
	})

	t.Run("returns 400 for invalid first_line (must be >= 1)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 0, // Invalid - must be >= 1
			Lines:     []string{`{"type":"user"}`},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp, &result)

		if !strings.Contains(result["error"], "first_line") {
			t.Errorf("expected error about first_line, got: %s", result["error"])
		}
	})

	t.Run("returns 400 for empty lines array", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{}, // Empty
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp, &result)

		if !strings.Contains(result["error"], "lines") {
			t.Errorf("expected error about lines, got: %s", result["error"])
		}
	})

	t.Run("returns 400 for missing session_id", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncChunkRequest{
			SessionID: "", // Missing
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user"}`},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp, &result)

		if !strings.Contains(result["error"], "session_id") {
			t.Errorf("expected error about session_id, got: %s", result["error"])
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncChunkRequest{
			SessionID: "00000000-0000-0000-0000-000000000000", // Non-existent
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user"}`},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("handles multiple files for same session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "multi-file-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload transcript chunk
		transcriptReq := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
		}

		resp1, err := client.Post("/api/v1/sync/chunk", transcriptReq)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusOK)

		// Upload agent chunk (different file)
		agentReq := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "agent-123.jsonl",
			FileType:  "agent",
			FirstLine: 1,
			Lines:     []string{`{"agent":"task1"}`},
		}

		resp2, err := client.Post("/api/v1/sync/chunk", agentReq)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusOK)

		// Verify both files in sync_files table
		var fileCount int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sync_files WHERE session_id = $1",
			sessionID)
		if err := row.Scan(&fileCount); err != nil {
			t.Fatalf("failed to count sync files: %v", err)
		}
		if fileCount != 2 {
			t.Errorf("expected 2 sync files, got %d", fileCount)
		}
	})
}

// =============================================================================
// GET /api/v1/sessions/{id}/sync/file - Read synced file
// =============================================================================

func TestSyncFileRead_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("reads and concatenates multiple chunks in order", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "read-test-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload three chunks via HTTP
		chunks := []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`, `{"line":2}`}},
			{3, []string{`{"line":3}`, `{"line":4}`, `{"line":5}`}},
			{6, []string{`{"line":6}`}},
		}

		for _, chunk := range chunks {
			reqBody := SyncChunkRequest{
				SessionID: sessionID,
				FileName:  "transcript.jsonl",
				FileType:  "transcript",
				FirstLine: chunk.firstLine,
				Lines:     chunk.lines,
			}

			resp, err := client.Post("/api/v1/sync/chunk", reqBody)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()
		}

		// Read merged file via canonical endpoint
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Read body
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		bodyStr := string(body[:n])

		// Verify all 6 lines are present
		expectedLines := []string{
			`{"line":1}`,
			`{"line":2}`,
			`{"line":3}`,
			`{"line":4}`,
			`{"line":5}`,
			`{"line":6}`,
		}

		for _, line := range expectedLines {
			if !strings.Contains(bodyStr, line) {
				t.Errorf("expected line %s in response", line)
			}
		}

		// Verify correct number of lines
		lines := strings.Split(strings.TrimSpace(bodyStr), "\n")
		if len(lines) != 6 {
			t.Errorf("expected 6 lines, got %d", len(lines))
		}
	})

	t.Run("returns 404 for non-existent file", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "no-file-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=nonexistent.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 404 for session owned by another user", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")
		apiKey2 := testutil.CreateTestAPIKeyWithToken(t, env, user2.ID, "User2 Key")

		sessionID := testutil.CreateTestSession(t, env, user1.ID, "user1-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey2.RawToken)

		// User2 tries to read user1's file
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 400 for missing file_name", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("merges overlapping chunks correctly", func(t *testing.T) {
		// This test simulates the scenario where:
		// 1. Client uploads chunk 1-5
		// 2. S3 write succeeds but DB update fails
		// 3. Client retries and uploads chunk 1-10
		// 4. Now S3 has two overlapping chunks
		// 5. Read should merge them, preferring the more complete chunk
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		externalID := "overlap-test-session"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Directly upload overlapping chunks to S3 (bypassing API validation)
		// This simulates the partial failure scenario
		// First chunk: lines 1-5 (the "failed" upload that wrote to S3 but didn't update DB)
		chunk1Data := []byte(`{"line":1,"chunk":"old"}
{"line":2,"chunk":"old"}
{"line":3,"chunk":"old"}
{"line":4,"chunk":"old"}
{"line":5,"chunk":"old"}
`)
		_, err := env.Storage.UploadChunk(env.Ctx, user.ID, externalID, "transcript.jsonl", 1, 5, chunk1Data)
		if err != nil {
			t.Fatalf("failed to upload chunk 1: %v", err)
		}

		// Second chunk: lines 1-10 (the retry that succeeded)
		chunk2Data := []byte(`{"line":1,"chunk":"new"}
{"line":2,"chunk":"new"}
{"line":3,"chunk":"new"}
{"line":4,"chunk":"new"}
{"line":5,"chunk":"new"}
{"line":6,"chunk":"new"}
{"line":7,"chunk":"new"}
{"line":8,"chunk":"new"}
{"line":9,"chunk":"new"}
{"line":10,"chunk":"new"}
`)
		_, err = env.Storage.UploadChunk(env.Ctx, user.ID, externalID, "transcript.jsonl", 1, 10, chunk2Data)
		if err != nil {
			t.Fatalf("failed to upload chunk 2: %v", err)
		}

		// Update sync state in DB (as if the second upload succeeded)
		_, err = env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 10)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		// Read merged file via HTTP
		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Read body
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		bodyStr := strings.TrimSpace(string(body[:n]))
		lines := strings.Split(bodyStr, "\n")

		if len(lines) != 10 {
			t.Errorf("expected 10 lines, got %d: %v", len(lines), lines)
		}

		// All lines should come from the "new" chunk (extends further)
		for i, line := range lines {
			if !strings.Contains(line, `"chunk":"new"`) {
				t.Errorf("line %d should be from 'new' chunk, got: %s", i+1, line)
			}
		}
	})

	t.Run("merges partially overlapping chunks correctly", func(t *testing.T) {
		// Scenario: chunk 1-5, then chunk 3-10 (partial overlap on 3-5)
		// Should take lines 1-2 from first, lines 3-10 from second
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		externalID := "partial-overlap-session"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// First chunk: lines 1-5
		chunk1Data := []byte(`{"line":1,"source":"A"}
{"line":2,"source":"A"}
{"line":3,"source":"A"}
{"line":4,"source":"A"}
{"line":5,"source":"A"}
`)
		_, err := env.Storage.UploadChunk(env.Ctx, user.ID, externalID, "transcript.jsonl", 1, 5, chunk1Data)
		if err != nil {
			t.Fatalf("failed to upload chunk 1: %v", err)
		}

		// Second chunk: lines 3-10 (overlaps on 3-5, extends to 10)
		chunk2Data := []byte(`{"line":3,"source":"B"}
{"line":4,"source":"B"}
{"line":5,"source":"B"}
{"line":6,"source":"B"}
{"line":7,"source":"B"}
{"line":8,"source":"B"}
{"line":9,"source":"B"}
{"line":10,"source":"B"}
`)
		_, err = env.Storage.UploadChunk(env.Ctx, user.ID, externalID, "transcript.jsonl", 3, 10, chunk2Data)
		if err != nil {
			t.Fatalf("failed to upload chunk 2: %v", err)
		}

		// Update sync state
		_, err = env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 10)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		// Read merged file via HTTP
		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		bodyStr := strings.TrimSpace(string(body[:n]))
		lines := strings.Split(bodyStr, "\n")

		if len(lines) != 10 {
			t.Errorf("expected 10 lines, got %d", len(lines))
		}

		// Lines 1-2 should come from "A" (only chunk covering them)
		// Lines 3-10 should come from "B" (extends further than A)
		expectedSources := []string{"A", "A", "B", "B", "B", "B", "B", "B", "B", "B"}
		for i, line := range lines {
			expectedSource := `"source":"` + expectedSources[i] + `"`
			if !strings.Contains(line, expectedSource) {
				t.Errorf("line %d: expected source %s, got: %s", i+1, expectedSources[i], line)
			}
		}
	})
}

// =============================================================================
// POST /api/v1/sync/chunk - Summary Field Handling
// =============================================================================

func TestSyncChunk_Summary_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("summary last write wins - chunk overwrites previous summary", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-chunk-test")

		// Set initial summary directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET summary = $1 WHERE id = $2",
			"Initial Summary", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial summary: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk with new summary (via metadata)
		reqBody := map[string]interface{}{
			"session_id": sessionID,
			"file_name":  "transcript.jsonl",
			"file_type":  "transcript",
			"first_line": 1,
			"lines":      []string{`{"type":"user","message":"Hello"}`},
			"metadata": map[string]interface{}{
				"summary": "Updated Summary",
			},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify summary was updated in database
		var summary *string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT summary FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&summary); err != nil {
			t.Fatalf("failed to query session summary: %v", err)
		}

		if summary == nil || *summary != "Updated Summary" {
			t.Errorf("expected summary 'Updated Summary', got %v", summary)
		}
	})

	t.Run("summary chunk overwrites existing summary", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-overwrite-test")

		// Set initial summary A directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET summary = $1 WHERE id = $2",
			"Summary A", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial summary: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk with summary B (via metadata)
		chunkBody := map[string]interface{}{
			"session_id": sessionID,
			"file_name":  "transcript.jsonl",
			"file_type":  "transcript",
			"first_line": 1,
			"lines":      []string{`{"type":"user","message":"Hello"}`},
			"metadata": map[string]interface{}{
				"summary": "Summary B",
			},
		}

		resp, err := client.Post("/api/v1/sync/chunk", chunkBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify summary is B (not A)
		var summary *string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT summary FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&summary); err != nil {
			t.Fatalf("failed to query session summary: %v", err)
		}

		if summary == nil || *summary != "Summary B" {
			t.Errorf("expected summary 'Summary B', got %v", summary)
		}
	})

	t.Run("empty summary clears existing summary", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-clear-test")

		// Set initial summary directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET summary = $1 WHERE id = $2",
			"Existing Summary", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial summary: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk with empty summary via metadata (should clear summary)
		reqBody := map[string]interface{}{
			"session_id": sessionID,
			"file_name":  "transcript.jsonl",
			"file_type":  "transcript",
			"first_line": 1,
			"lines":      []string{`{"type":"user","message":"Hello"}`},
			"metadata": map[string]interface{}{
				"summary": "", // Empty string - should clear summary
			},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify summary was cleared (empty string or null)
		var summary *string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT summary FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&summary); err != nil {
			t.Fatalf("failed to query session summary: %v", err)
		}

		// Empty string means cleared - should be "" not nil, but nil is also acceptable
		if summary != nil && *summary != "" {
			t.Errorf("expected summary to be cleared (empty or null), got %q", *summary)
		}
	})

	t.Run("absent summary field preserves existing summary", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-preserve-test")

		// Set initial summary directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET summary = $1 WHERE id = $2",
			"Preserved Summary", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial summary: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk WITHOUT summary field (using SyncChunkRequest struct, not map)
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
			// No summary field - should preserve existing
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify summary was NOT changed
		var summary *string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT summary FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&summary); err != nil {
			t.Fatalf("failed to query session summary: %v", err)
		}

		if summary == nil || *summary != "Preserved Summary" {
			t.Errorf("expected summary 'Preserved Summary' to be preserved, got %v", summary)
		}
	})
}

// =============================================================================
// POST /api/v1/sync/chunk - First User Message Field Handling
// =============================================================================

func TestSyncChunk_FirstUserMessage_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("first_user_message first write wins - subsequent chunks do not overwrite", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "first-msg-test")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload first chunk with first_user_message (via metadata)
		reqBody := map[string]interface{}{
			"session_id": sessionID,
			"file_name":  "transcript.jsonl",
			"file_type":  "transcript",
			"first_line": 1,
			"lines":      []string{`{"type":"user","message":"Hello"}`},
			"metadata": map[string]interface{}{
				"first_user_message": "First message A",
			},
		}

		resp1, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusOK)

		// Upload second chunk trying to overwrite first_user_message (via metadata)
		reqBody2 := map[string]interface{}{
			"session_id": sessionID,
			"file_name":  "transcript.jsonl",
			"file_type":  "transcript",
			"first_line": 2,
			"lines":      []string{`{"type":"assistant","message":"Hi"}`},
			"metadata": map[string]interface{}{
				"first_user_message": "First message B - should be ignored",
			},
		}

		resp2, err := client.Post("/api/v1/sync/chunk", reqBody2)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusOK)

		// Verify first_user_message is still A (not B)
		var firstUserMessage *string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT first_user_message FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&firstUserMessage); err != nil {
			t.Fatalf("failed to query session first_user_message: %v", err)
		}

		if firstUserMessage == nil || *firstUserMessage != "First message A" {
			t.Errorf("expected first_user_message 'First message A', got %v", firstUserMessage)
		}
	})

	t.Run("first_user_message once set cannot be overwritten by chunk", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "first-msg-immutable-test")

		// Set initial first_user_message directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET first_user_message = $1 WHERE id = $2",
			"Original message", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial first_user_message: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk trying to overwrite first_user_message (via metadata)
		chunkBody := map[string]interface{}{
			"session_id": sessionID,
			"file_name":  "transcript.jsonl",
			"file_type":  "transcript",
			"first_line": 1,
			"lines":      []string{`{"type":"user","message":"Hello"}`},
			"metadata": map[string]interface{}{
				"first_user_message": "Message from chunk - should be ignored",
			},
		}

		resp, err := client.Post("/api/v1/sync/chunk", chunkBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify first_user_message is still original (first write wins)
		var firstUserMessage *string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT first_user_message FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&firstUserMessage); err != nil {
			t.Fatalf("failed to query session first_user_message: %v", err)
		}

		if firstUserMessage == nil || *firstUserMessage != "Original message" {
			t.Errorf("expected first_user_message 'Original message', got %v", firstUserMessage)
		}
	})

	t.Run("absent first_user_message field preserves existing value", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "first-msg-preserve-test")

		// Set initial first_user_message directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET first_user_message = $1 WHERE id = $2",
			"Preserved Message", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial first_user_message: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk WITHOUT first_user_message field
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
			// No first_user_message field - should preserve existing
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify first_user_message was NOT changed
		var firstUserMessage *string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT first_user_message FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&firstUserMessage); err != nil {
			t.Fatalf("failed to query session first_user_message: %v", err)
		}

		if firstUserMessage == nil || *firstUserMessage != "Preserved Message" {
			t.Errorf("expected first_user_message 'Preserved Message' to be preserved, got %v", firstUserMessage)
		}
	})
}

// =============================================================================
// POST /api/v1/sync/chunk - Chunk Count Tracking
// =============================================================================

func TestSyncChunk_ChunkCountTracking_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("increments chunk_count on each upload", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-chunk-count")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload first chunk
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Line 1"}`},
		}

		resp1, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusOK)

		// Verify chunk_count is 1
		var chunkCount *int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT chunk_count FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, "transcript.jsonl")
		if err := row.Scan(&chunkCount); err != nil {
			t.Fatalf("failed to query chunk_count: %v", err)
		}
		if chunkCount == nil || *chunkCount != 1 {
			t.Errorf("expected chunk_count 1 after first upload, got %v", chunkCount)
		}

		// Upload second chunk
		reqBody.FirstLine = 2
		reqBody.Lines = []string{`{"type":"assistant","message":"Line 2"}`}

		resp2, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp2.Body.Close()
		testutil.RequireStatus(t, resp2, http.StatusOK)

		// Verify chunk_count is 2
		row = env.DB.QueryRow(env.Ctx,
			"SELECT chunk_count FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, "transcript.jsonl")
		if err := row.Scan(&chunkCount); err != nil {
			t.Fatalf("failed to query chunk_count: %v", err)
		}
		if chunkCount == nil || *chunkCount != 2 {
			t.Errorf("expected chunk_count 2 after second upload, got %v", chunkCount)
		}
	})

	t.Run("allows upload when chunk_count is NULL (legacy file)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-legacy-null")

		// Insert sync_files record with NULL chunk_count (legacy)
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, chunk_count)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 100, NULL)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 101,
			Lines:     []string{`{"type":"user","message":"Should be allowed"}`},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify chunk_count is now 1 (COALESCE(NULL, 0) + 1)
		var chunkCount *int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT chunk_count FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, "transcript.jsonl")
		if err := row.Scan(&chunkCount); err != nil {
			t.Fatalf("failed to query chunk_count: %v", err)
		}
		if chunkCount == nil || *chunkCount != 1 {
			t.Errorf("expected chunk_count 1 after upload to legacy file, got %v", chunkCount)
		}
	})

	t.Run("rejects upload when chunk limit exceeded", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-chunk-limit")

		// Insert sync_files record with chunk_count at limit
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, chunk_count)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 100, $2)`,
			sessionID, storage.MaxChunksPerFile)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 101,
			Lines:     []string{`{"type":"user","message":"Should be rejected"}`},
		}

		resp, err := client.Post("/api/v1/sync/chunk", reqBody)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)

		var result map[string]string
		testutil.ParseJSON(t, resp, &result)

		if !strings.Contains(result["error"], "too many chunks") {
			t.Errorf("expected error about too many chunks, got: %s", result["error"])
		}
	})
}

// =============================================================================
// GET /api/v1/sessions/{id}/sync/file - line_offset for incremental fetching
// =============================================================================

func TestSyncFileRead_LineOffset_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns all lines when line_offset is not specified (backward compatible)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-1")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunks with 6 lines total
		chunks := []SyncChunkRequest{
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 4, Lines: []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}},
		}
		for _, chunk := range chunks {
			resp, _ := client.Post("/api/v1/sync/chunk", chunk)
			resp.Body.Close()
		}

		// Read without line_offset
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		lines := strings.Split(strings.TrimSpace(string(body[:n])), "\n")
		if len(lines) != 6 {
			t.Errorf("expected 6 lines, got %d: %v", len(lines), lines)
		}
	})

	t.Run("returns only lines after line_offset", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-3")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunks
		chunks := []SyncChunkRequest{
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 4, Lines: []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}},
		}
		for _, chunk := range chunks {
			resp, _ := client.Post("/api/v1/sync/chunk", chunk)
			resp.Body.Close()
		}

		// Request lines after line 3 (should return lines 4, 5, 6)
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=3")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		bodyStr := string(body[:n])
		lines := strings.Split(strings.TrimSpace(bodyStr), "\n")

		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d: %v", len(lines), lines)
		}

		// Verify line 3 is NOT in output
		if strings.Contains(bodyStr, `"line":3`) {
			t.Error("response should not contain line 3")
		}
	})

	t.Run("returns empty response when line_offset equals total lines", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-4")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk
		chunk := SyncChunkRequest{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}}
		resp1, _ := client.Post("/api/v1/sync/chunk", chunk)
		resp1.Body.Close()

		// Request lines after line 3 (none exist)
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=3")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		bodyStr := strings.TrimSpace(string(body[:n]))
		if bodyStr != "" {
			t.Errorf("expected empty response, got: %s", bodyStr)
		}
	})

	t.Run("returns all lines when line_offset=0", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-zero")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunks with 6 lines total
		chunks := []SyncChunkRequest{
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 4, Lines: []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}},
		}
		for _, chunk := range chunks {
			resp, _ := client.Post("/api/v1/sync/chunk", chunk)
			resp.Body.Close()
		}

		// Read with line_offset=0 (should return all lines)
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=0")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		lines := strings.Split(strings.TrimSpace(string(body[:n])), "\n")
		if len(lines) != 6 {
			t.Errorf("expected 6 lines, got %d: %v", len(lines), lines)
		}
	})

	t.Run("returns empty response when line_offset exceeds total lines", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-exceed")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk with 3 lines
		chunk := SyncChunkRequest{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}}
		resp1, _ := client.Post("/api/v1/sync/chunk", chunk)
		resp1.Body.Close()

		// Request lines after line 100 (none exist)
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=100")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		bodyStr := strings.TrimSpace(string(body[:n]))
		if bodyStr != "" {
			t.Errorf("expected empty response, got: %s", bodyStr)
		}
	})

	t.Run("returns 400 for negative line_offset", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-6")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload chunk to create the file
		chunk := SyncChunkRequest{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`}}
		resp1, _ := client.Post("/api/v1/sync/chunk", chunk)
		resp1.Body.Close()

		// Request with negative offset
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=-1")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 400 for invalid line_offset", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-invalid")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Request with invalid (non-numeric) offset
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=invalid")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("filters output to lines within a single chunk", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-single-chunk")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload one chunk with 5 lines
		chunk := SyncChunkRequest{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`, `{"line":2}`, `{"line":3}`, `{"line":4}`, `{"line":5}`}}
		resp1, _ := client.Post("/api/v1/sync/chunk", chunk)
		resp1.Body.Close()

		// Request lines after line 2 (should return lines 3, 4, 5)
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=2")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		lines := strings.Split(strings.TrimSpace(string(body[:n])), "\n")
		expected := []string{`{"line":3}`, `{"line":4}`, `{"line":5}`}
		if len(lines) != len(expected) {
			t.Fatalf("expected %d lines, got %d: %v", len(expected), len(lines), lines)
		}

		// Verify content
		for i, line := range lines {
			if line != expected[i] {
				t.Errorf("line %d: expected %s, got %s", i, expected[i], line)
			}
		}
	})

	t.Run("works correctly across chunk boundaries", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-9")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload 3 chunks with 3 lines each (total 9 lines)
		chunks := []SyncChunkRequest{
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 4, Lines: []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}},
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 7, Lines: []string{`{"line":7}`, `{"line":8}`, `{"line":9}`}},
		}
		for _, chunk := range chunks {
			resp, _ := client.Post("/api/v1/sync/chunk", chunk)
			resp.Body.Close()
		}

		// Request lines after line 5 (should return lines 6, 7, 8, 9)
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=5")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		lines := strings.Split(strings.TrimSpace(string(body[:n])), "\n")
		if len(lines) != 4 {
			t.Errorf("expected 4 lines, got %d: %v", len(lines), lines)
		}

		// Verify content
		expected := []string{`{"line":6}`, `{"line":7}`, `{"line":8}`, `{"line":9}`}
		for i, line := range lines {
			if line != expected[i] {
				t.Errorf("line %d: expected %s, got %s", i, expected[i], line)
			}
		}
	})
}

// =============================================================================
// DELETE /api/v1/sessions/{id} - Delete session with chunks (cascade delete)
// =============================================================================

func TestDeleteSession_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("deletes all chunks when session is deleted", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "delete-test-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload multiple chunks
		chunks := []SyncChunkRequest{
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 1, Lines: []string{`{"line":1}`}},
			{SessionID: sessionID, FileName: "transcript.jsonl", FileType: "transcript", FirstLine: 2, Lines: []string{`{"line":2}`}},
			{SessionID: sessionID, FileName: "agent-123.jsonl", FileType: "agent", FirstLine: 1, Lines: []string{`{"agent":1}`}},
		}

		for _, chunk := range chunks {
			resp, err := client.Post("/api/v1/sync/chunk", chunk)
			if err != nil {
				t.Fatalf("chunk upload failed: %v", err)
			}
			testutil.RequireStatus(t, resp, http.StatusOK)
			resp.Body.Close()
		}

		// Verify chunks exist in S3 before deletion
		s3Key := buildChunkS3Key(user.ID, "delete-test-session", "transcript.jsonl", 1, 1)
		testutil.VerifyFileInS3(t, env, s3Key)

		// Need session auth for delete endpoint (web dashboard endpoint)
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionClient := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Delete session
		resp, err := sessionClient.Delete("/api/v1/sessions/" + sessionID)
		if err != nil {
			t.Fatalf("delete request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify session deleted from DB
		var sessionCount int
		row := env.DB.QueryRow(env.Ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", sessionID)
		if err := row.Scan(&sessionCount); err != nil {
			t.Fatalf("failed to query sessions: %v", err)
		}
		if sessionCount != 0 {
			t.Error("expected session to be deleted from DB")
		}

		// Verify sync_files deleted from DB
		var syncFileCount int
		row = env.DB.QueryRow(env.Ctx, "SELECT COUNT(*) FROM sync_files WHERE session_id = $1", sessionID)
		if err := row.Scan(&syncFileCount); err != nil {
			t.Fatalf("failed to query sync_files: %v", err)
		}
		if syncFileCount != 0 {
			t.Error("expected sync_files to be deleted from DB")
		}

		// Verify chunks deleted from S3 (download should fail)
		_, err = env.Storage.Download(env.Ctx, s3Key)
		if err == nil {
			t.Error("expected S3 chunk to be deleted")
		}
	})
}

// =============================================================================
// Race condition test for sync/init
// =============================================================================

func TestSyncInit_RaceCondition_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("concurrent init requests for same session all succeed", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)

		const numGoroutines = 10
		externalID := "race-test-session"

		// Channel to collect results
		type result struct {
			sessionID string
			err       error
		}
		results := make(chan result, numGoroutines)

		// Start barrier to ensure all goroutines start simultaneously
		start := make(chan struct{})

		// Spawn concurrent requests
		for i := 0; i < numGoroutines; i++ {
			go func() {
				// Wait for start signal
				<-start

				client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

				reqBody := SyncInitRequest{
					ExternalID:     externalID,
					TranscriptPath: "/home/user/project/transcript.jsonl",
					CWD:            "/home/user/project",
				}

				resp, err := client.Post("/api/v1/sync/init", reqBody)
				if err != nil {
					results <- result{err: err}
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					results <- result{err: nil} // Will be caught by session ID check
					return
				}

				var initResp SyncInitResponse
				if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
					results <- result{err: err}
					return
				}

				results <- result{sessionID: initResp.SessionID}
			}()
		}

		// Release all goroutines at once
		close(start)

		// Collect results
		var sessionIDs []string
		var errors []error
		for i := 0; i < numGoroutines; i++ {
			r := <-results
			if r.err != nil {
				errors = append(errors, r.err)
			} else if r.sessionID != "" {
				sessionIDs = append(sessionIDs, r.sessionID)
			}
		}

		// All requests should succeed
		if len(errors) > 0 {
			t.Errorf("expected all requests to succeed, got %d errors: %v", len(errors), errors)
		}

		// All requests should return the same session ID
		if len(sessionIDs) != numGoroutines {
			t.Errorf("expected %d session IDs, got %d", numGoroutines, len(sessionIDs))
		}

		if len(sessionIDs) > 0 {
			firstID := sessionIDs[0]
			for i, id := range sessionIDs {
				if id != firstID {
					t.Errorf("session ID mismatch: goroutine 0 got %s, goroutine %d got %s", firstID, i, id)
				}
			}
		}

		// Verify only one session exists in database
		var sessionCount int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sessions WHERE external_id = $1 AND user_id = $2",
			externalID, user.ID)
		if err := row.Scan(&sessionCount); err != nil {
			t.Fatalf("failed to query sessions: %v", err)
		}
		if sessionCount != 1 {
			t.Errorf("expected exactly 1 session in database, got %d", sessionCount)
		}
	})
}

// =============================================================================
// GET /api/v1/sessions/{id}/sync/file - Self-healing for chunk_count
// =============================================================================

func TestSyncFileRead_SelfHealing_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("self-heals chunk_count from NULL to actual count", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		externalID := "test-selfheal-null"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload 3 chunks directly to S3 (bypassing the API to simulate legacy data)
		for i := 1; i <= 3; i++ {
			firstLine := (i-1)*10 + 1
			lastLine := i * 10
			data := []byte(`{"chunk":` + string(rune('0'+i)) + `}` + "\n")
			_, err := env.Storage.UploadChunk(env.Ctx, user.ID, externalID, "transcript.jsonl", firstLine, lastLine, data)
			if err != nil {
				t.Fatalf("failed to upload chunk %d: %v", i, err)
			}
		}

		// Insert sync_files record with NULL chunk_count (simulating legacy or drift)
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, chunk_count)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 30, NULL)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		// Read the file via canonical endpoint
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify chunk_count was self-healed to 3
		var chunkCount *int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT chunk_count FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, "transcript.jsonl")
		if err := row.Scan(&chunkCount); err != nil {
			t.Fatalf("failed to query chunk_count: %v", err)
		}
		if chunkCount == nil || *chunkCount != 3 {
			t.Errorf("expected chunk_count to be self-healed to 3, got %v", chunkCount)
		}
	})

	t.Run("self-heals chunk_count from incorrect value to actual count", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		externalID := "test-selfheal-wrong"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload 2 chunks directly to S3
		for i := 1; i <= 2; i++ {
			firstLine := (i-1)*10 + 1
			lastLine := i * 10
			data := []byte(`{"chunk":` + string(rune('0'+i)) + `}` + "\n")
			_, err := env.Storage.UploadChunk(env.Ctx, user.ID, externalID, "transcript.jsonl", firstLine, lastLine, data)
			if err != nil {
				t.Fatalf("failed to upload chunk %d: %v", i, err)
			}
		}

		// Insert sync_files record with incorrect chunk_count (simulating drift)
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, chunk_count)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 20, 5)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		// Read the file via canonical endpoint
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify chunk_count was self-healed to 2 (actual count)
		var chunkCount *int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT chunk_count FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, "transcript.jsonl")
		if err := row.Scan(&chunkCount); err != nil {
			t.Fatalf("failed to query chunk_count: %v", err)
		}
		if chunkCount == nil || *chunkCount != 2 {
			t.Errorf("expected chunk_count to be self-healed to 2, got %v", chunkCount)
		}
	})

	t.Run("does not update chunk_count if already correct", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		externalID := "test-selfheal-correct"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload 2 chunks directly to S3
		for i := 1; i <= 2; i++ {
			firstLine := (i-1)*10 + 1
			lastLine := i * 10
			data := []byte(`{"chunk":` + string(rune('0'+i)) + `}` + "\n")
			_, err := env.Storage.UploadChunk(env.Ctx, user.ID, externalID, "transcript.jsonl", firstLine, lastLine, data)
			if err != nil {
				t.Fatalf("failed to upload chunk %d: %v", i, err)
			}
		}

		// Insert sync_files record with correct chunk_count
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, chunk_count, updated_at)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 20, 2, '2020-01-01 00:00:00')`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		// Read the file via canonical endpoint
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		// Verify chunk_count is still 2 and updated_at was NOT changed
		var chunkCount *int
		var updatedAt string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT chunk_count, updated_at::text FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, "transcript.jsonl")
		if err := row.Scan(&chunkCount, &updatedAt); err != nil {
			t.Fatalf("failed to query chunk_count: %v", err)
		}
		if chunkCount == nil || *chunkCount != 2 {
			t.Errorf("expected chunk_count to remain 2, got %v", chunkCount)
		}
		// Check updated_at wasn't touched (still the old value)
		if !strings.HasPrefix(updatedAt, "2020-01-01") {
			t.Errorf("expected updated_at to remain unchanged, got %s", updatedAt)
		}
	})
}

// =============================================================================
// GET /api/v1/sessions/{id}/sync/file - DB short-circuit for line_offset
// =============================================================================

func TestSyncFileRead_LineOffset_DBShortCircuit_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("verifies last_synced_line is correctly tracked", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "shortcircuit-test-1")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload a chunk with 3 lines
		chunk := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"line":1}`, `{"line":2}`, `{"line":3}`},
		}
		resp, err := client.Post("/api/v1/sync/chunk", chunk)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)
		resp.Body.Close()

		// Verify last_synced_line is 3
		var lastSyncedLine int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT last_synced_line FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, "transcript.jsonl")
		if err := row.Scan(&lastSyncedLine); err != nil {
			t.Fatalf("failed to get last_synced_line: %v", err)
		}
		if lastSyncedLine != 3 {
			t.Errorf("expected last_synced_line=3, got %d", lastSyncedLine)
		}

		// Request with line_offset=3 should return empty (no lines after 3)
		resp, err = client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=3")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		if strings.TrimSpace(string(body[:n])) != "" {
			t.Errorf("expected empty response for line_offset=last_synced_line, got: %s", string(body[:n]))
		}
	})

	t.Run("returns new lines after incremental sync", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "shortcircuit-test-2")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Upload first chunk (lines 1-3)
		chunk1 := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"line":1}`, `{"line":2}`, `{"line":3}`},
		}
		resp, _ := client.Post("/api/v1/sync/chunk", chunk1)
		resp.Body.Close()

		// Client knows it has 3 lines, polls with line_offset=3
		resp, _ = client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=3")
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		resp.Body.Close()
		if strings.TrimSpace(string(body[:n])) != "" {
			t.Errorf("expected empty before new sync, got: %s", string(body[:n]))
		}

		// New data arrives - upload second chunk (lines 4-6)
		chunk2 := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 4,
			Lines:     []string{`{"line":4}`, `{"line":5}`, `{"line":6}`},
		}
		resp, _ = client.Post("/api/v1/sync/chunk", chunk2)
		resp.Body.Close()

		// Client polls again with same line_offset=3, now gets new lines
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=3")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		body = make([]byte, 4096)
		n, _ = resp.Body.Read(body)
		lines := strings.Split(strings.TrimSpace(string(body[:n])), "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 new lines, got %d: %v", len(lines), lines)
		}

		expected := []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}
		for i, line := range lines {
			if line != expected[i] {
				t.Errorf("line %d: expected %s, got %s", i, expected[i], line)
			}
		}
	})

	t.Run("returns 404 when file has no DB record", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "shortcircuit-test-3")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// No chunks uploaded, so no sync_files record
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=transcript.jsonl&line_offset=0")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})
}
