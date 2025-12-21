package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// POST /api/v1/sync/init - Initialize or resume sync session
// =============================================================================

func TestHandleSyncInit_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("creates new session and returns sync state", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := SyncInitRequest{
			ExternalID:     "new-session-123",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify response structure
		if resp.SessionID == "" {
			t.Error("expected non-empty session_id")
		}
		if resp.Files == nil {
			t.Error("expected files map to be initialized")
		}

		// New session should have no synced files yet
		if len(resp.Files) != 0 {
			t.Errorf("expected 0 files for new session, got %d", len(resp.Files))
		}

		// Verify session was created in database
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

	t.Run("resumes existing session and returns current sync state", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Create existing session with some synced files
		sessionID := testutil.CreateTestSession(t, env, user.ID, "existing-session-456")

		// Insert sync state for transcript (simulating previous sync)
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 150)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		// Insert sync state for agent file
		_, err = env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line)
			 VALUES ($1, 'agent-abc123.jsonl', 'agent', 50)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		reqBody := SyncInitRequest{
			ExternalID:     "existing-session-456",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify session ID is returned
		if resp.SessionID != sessionID {
			t.Errorf("expected session_id %s, got %s", sessionID, resp.SessionID)
		}

		// Verify sync state for both files
		if len(resp.Files) != 2 {
			t.Errorf("expected 2 files in sync state, got %d", len(resp.Files))
		}

		transcriptState, ok := resp.Files["transcript.jsonl"]
		if !ok {
			t.Error("expected transcript.jsonl in files map")
		} else if transcriptState.LastSyncedLine != 150 {
			t.Errorf("expected last_synced_line 150 for transcript, got %d", transcriptState.LastSyncedLine)
		}

		agentState, ok := resp.Files["agent-abc123.jsonl"]
		if !ok {
			t.Error("expected agent-abc123.jsonl in files map")
		} else if agentState.LastSyncedLine != 50 {
			t.Errorf("expected last_synced_line 50 for agent, got %d", agentState.LastSyncedLine)
		}
	})

	t.Run("returns 400 for missing external_id", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := SyncInitRequest{
			ExternalID:     "", // Missing
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "external_id") {
			t.Errorf("expected error about external_id, got: %s", resp["error"])
		}
	})

	t.Run("returns 400 for missing transcript_path", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := SyncInitRequest{
			ExternalID:     "test-session",
			TranscriptPath: "", // Missing
			CWD:            "/home/user/project",
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "transcript_path") {
			t.Errorf("expected error about transcript_path, got: %s", resp["error"])
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := SyncInitRequest{
			ExternalID:     "test-session",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, 0)
		req = req.WithContext(context.Background()) // Remove auth context

		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("isolates sessions between users", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")

		// User1 creates a session
		reqBody := SyncInitRequest{
			ExternalID:     "shared-external-id",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/project",
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user1.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp1 SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp1)

		// User2 creates a session with same external_id (should be different session)
		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user2.ID)
		w = httptest.NewRecorder()

		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp2 SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp2)

		// Session IDs should be different
		if resp1.SessionID == resp2.SessionID {
			t.Error("expected different session IDs for different users with same external_id")
		}

		// Verify both sessions exist in database
		var sessionCount int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sessions WHERE external_id = $1",
			"shared-external-id")
		if err := row.Scan(&sessionCount); err != nil {
			t.Fatalf("failed to query sessions: %v", err)
		}
		if sessionCount != 2 {
			t.Errorf("expected 2 sessions in database, got %d", sessionCount)
		}
	})
}

// =============================================================================
// POST /api/v1/sync/init - Metadata Nesting (New vs Deprecated Format)
// =============================================================================

func TestHandleSyncInit_MetadataNesting_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("new format: reads cwd and git_info from metadata", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Use map to send new nested format
		reqBody := map[string]interface{}{
			"external_id":     "new-format-session",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"metadata": map[string]interface{}{
				"cwd":      "/home/user/new-format-project",
				"git_info": map[string]string{"branch": "main", "repo_url": "https://github.com/test/repo.git"},
			},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify session was created
		if resp.SessionID == "" {
			t.Error("expected non-empty session_id")
		}

		// Verify cwd and git_info were stored correctly
		var storedCWD string
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd, git_info FROM sessions WHERE id = $1",
			resp.SessionID)
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

		// Use the old struct format (top-level cwd and git_info)
		reqBody := SyncInitRequest{
			ExternalID:     "old-format-session",
			TranscriptPath: "/home/user/project/transcript.jsonl",
			CWD:            "/home/user/old-format-project",
			GitInfo:        json.RawMessage(`{"branch":"feature","repo_url":"https://github.com/test/old.git"}`),
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify cwd and git_info were stored correctly from top-level fields
		var storedCWD string
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd, git_info FROM sessions WHERE id = $1",
			resp.SessionID)
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

		// Send both top-level AND metadata - metadata should win
		reqBody := map[string]interface{}{
			"external_id":     "mixed-format-session",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"cwd":             "/home/user/top-level-cwd",       // Should be ignored
			"git_info":        map[string]string{"branch": "old"}, // Should be ignored
			"metadata": map[string]interface{}{
				"cwd":      "/home/user/metadata-cwd",
				"git_info": map[string]string{"branch": "new"},
			},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify metadata values took precedence
		var storedCWD string
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd, git_info FROM sessions WHERE id = $1",
			resp.SessionID)
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

		// Send top-level with empty metadata - should use top-level
		reqBody := map[string]interface{}{
			"external_id":     "empty-metadata-session",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"cwd":             "/home/user/fallback-cwd",
			"git_info":        map[string]string{"branch": "fallback"},
			"metadata":        map[string]interface{}{}, // Empty metadata
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify top-level values were used as fallback
		var storedCWD string
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd, git_info FROM sessions WHERE id = $1",
			resp.SessionID)
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

		// Send top-level with null metadata
		reqBody := map[string]interface{}{
			"external_id":     "null-metadata-session",
			"transcript_path": "/home/user/project/transcript.jsonl",
			"cwd":             "/home/user/null-fallback-cwd",
			"git_info":        map[string]string{"branch": "null-fallback"},
			"metadata":        nil,
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncInitResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify top-level values were used as fallback
		var storedCWD string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT cwd FROM sessions WHERE id = $1",
			resp.SessionID)
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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncInit(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "cwd") {
			t.Errorf("expected error about cwd, got: %s", resp["error"])
		}
	})
}

// =============================================================================
// POST /api/v1/sync/chunk - Upload a chunk of lines
// =============================================================================

func TestHandleSyncChunk_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("uploads first chunk successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-chunk")

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncChunkResponse
		testutil.ParseJSONResponse(t, w, &resp)

		// Verify high-water mark updated
		if resp.LastSyncedLine != 3 {
			t.Errorf("expected last_synced_line 3, got %d", resp.LastSyncedLine)
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

		// Verify chunk exists in S3
		expectedS3Key := buildChunkS3Key(user.ID, "test-session-chunk", "transcript.jsonl", 1, 3)
		content := testutil.VerifyFileInS3(t, env, expectedS3Key)

		expectedContent := strings.Join(lines, "\n") + "\n"
		if string(content) != expectedContent {
			t.Errorf("S3 content mismatch.\nExpected: %q\nGot: %q", expectedContent, string(content))
		}
	})

	t.Run("uploads subsequent chunk successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-subsequent")

		// Simulate existing sync state (first 100 lines already synced)
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 100)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp SyncChunkResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if resp.LastSyncedLine != 102 {
			t.Errorf("expected last_synced_line 102, got %d", resp.LastSyncedLine)
		}

		// Verify S3 chunk with correct naming
		expectedS3Key := buildChunkS3Key(user.ID, "test-session-subsequent", "transcript.jsonl", 101, 102)
		testutil.VerifyFileInS3(t, env, expectedS3Key)
	})

	t.Run("rejects overlapping chunk (re-upload of same lines)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-overlap")

		lines := []string{
			`{"type":"user","message":"Hello"}`,
		}

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     lines,
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Try to re-upload same range - should be rejected as overlap
		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w = httptest.NewRecorder()

		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "first_line must be 2") {
			t.Errorf("expected error about first_line must be 2, got: %s", resp["error"])
		}
	})

	t.Run("rejects chunk with gap (skipped lines)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-gap")

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Try to upload chunk starting at line 5 (gap - should start at 3)
		reqBody.FirstLine = 5
		reqBody.Lines = []string{`{"type":"user","message":"Line 5"}`}

		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w = httptest.NewRecorder()

		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "first_line must be 3") {
			t.Errorf("expected error about first_line must be 3, got: %s", resp["error"])
		}
	})

	t.Run("returns 400 for invalid first_line (must be >= 1)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 0, // Invalid - must be >= 1
			Lines:     []string{`{"type":"user"}`},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "first_line") {
			t.Errorf("expected error about first_line, got: %s", resp["error"])
		}
	})

	t.Run("returns 400 for empty lines array", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{}, // Empty
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "lines") {
			t.Errorf("expected error about lines, got: %s", resp["error"])
		}
	})

	t.Run("returns 400 for missing session_id", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := SyncChunkRequest{
			SessionID: "", // Missing
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user"}`},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "session_id") {
			t.Errorf("expected error about session_id, got: %s", resp["error"])
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := SyncChunkRequest{
			SessionID: "00000000-0000-0000-0000-000000000000", // Non-existent
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user"}`},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("returns 403 for session owned by another user", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")

		// Session owned by user1
		sessionID := testutil.CreateTestSession(t, env, user1.ID, "user1-session")

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user"}`},
		}

		// User2 tries to upload to user1's session
		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user2.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusForbidden)
	})

	t.Run("handles multiple files for same session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "multi-file-session")

		server := &Server{db: env.DB, storage: env.Storage}

		// Upload transcript chunk
		transcriptReq := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Main transcript"}`},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", transcriptReq, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

		// Upload agent chunk
		agentReq := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "agent-abc123.jsonl",
			FileType:  "agent",
			FirstLine: 1,
			Lines:     []string{`{"type":"tool_use","tool":"read"}`},
		}

		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", agentReq, user.ID)
		w = httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify both files tracked separately
		var fileCount int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sync_files WHERE session_id = $1",
			sessionID)
		if err := row.Scan(&fileCount); err != nil {
			t.Fatalf("failed to query sync_files: %v", err)
		}
		if fileCount != 2 {
			t.Errorf("expected 2 sync_files entries, got %d", fileCount)
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := SyncChunkRequest{
			SessionID: "some-session-id",
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user"}`},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, 0)
		req = req.WithContext(context.Background())

		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("updates git_info from chunk metadata for transcript files", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-gitinfo")

		// Upload chunk with metadata containing git_info
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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify git_info was stored in the session
		var storedGitInfo json.RawMessage
		row := env.DB.QueryRow(env.Ctx,
			"SELECT git_info FROM sessions WHERE id = $1",
			sessionID)
		if err := row.Scan(&storedGitInfo); err != nil {
			t.Fatalf("failed to query session git_info: %v", err)
		}

		// Parse and verify git_info content
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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-gitinfo-update")

		server := &Server{db: env.DB, storage: env.Storage}

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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

		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w = httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-agent-no-git")

		// First set git_info via transcript
		server := &Server{db: env.DB, storage: env.Storage}
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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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

		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", agentReq, user.ID)
		w = httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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
// GET /api/v1/sync/file - Read merged file content
// =============================================================================

func TestHandleSyncFileRead_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("reads and concatenates multiple chunks in order", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "read-test-session")

		server := &Server{db: env.DB, storage: env.Storage}

		// Upload three chunks
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

			req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
			w := httptest.NewRecorder()
			server.handleSyncChunk(w, req)
			testutil.AssertStatus(t, w, http.StatusOK)
		}

		// Read merged file
		req := testutil.AuthenticatedRequest(t, "GET",
			"/api/v1/sync/file?session_id="+sessionID+"&file_name=transcript.jsonl", nil, user.ID)

		// Add chi URL params
		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify content is concatenated in correct order
		body := w.Body.String()

		expectedLines := []string{
			`{"line":1}`,
			`{"line":2}`,
			`{"line":3}`,
			`{"line":4}`,
			`{"line":5}`,
			`{"line":6}`,
		}

		for i, line := range expectedLines {
			if !strings.Contains(body, line) {
				t.Errorf("expected line %d (%s) in response", i+1, line)
			}
		}

		// Verify order by checking line numbers appear in sequence
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if len(lines) != 6 {
			t.Errorf("expected 6 lines, got %d", len(lines))
		}
	})

	t.Run("returns 404 for non-existent file", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "no-file-session")

		req := testutil.AuthenticatedRequest(t, "GET",
			"/api/v1/sync/file?session_id="+sessionID+"&file_name=nonexistent.jsonl", nil, user.ID)

		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("returns 403 for session owned by another user", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")

		sessionID := testutil.CreateTestSession(t, env, user1.ID, "user1-session")

		// User2 tries to read user1's file
		req := testutil.AuthenticatedRequest(t, "GET",
			"/api/v1/sync/file?session_id="+sessionID+"&file_name=transcript.jsonl", nil, user2.ID)

		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusForbidden)
	})

	t.Run("returns 400 for missing session_id", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		req := testutil.AuthenticatedRequest(t, "GET",
			"/api/v1/sync/file?file_name=transcript.jsonl", nil, user.ID)

		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("returns 400 for missing file_name", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		req := testutil.AuthenticatedRequest(t, "GET",
			"/api/v1/sync/file?session_id="+sessionID, nil, user.ID)

		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)
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
		externalID := "overlap-test-session"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Directly upload overlapping chunks to S3 (bypassing API validation)
		// This simulates the partial failure scenario
		ctx := context.Background()

		// First chunk: lines 1-5 (the "failed" upload that wrote to S3 but didn't update DB)
		chunk1Data := []byte(`{"line":1,"chunk":"old"}
{"line":2,"chunk":"old"}
{"line":3,"chunk":"old"}
{"line":4,"chunk":"old"}
{"line":5,"chunk":"old"}
`)
		_, err := env.Storage.UploadChunk(ctx, user.ID, externalID, "transcript.jsonl", 1, 5, chunk1Data)
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
		_, err = env.Storage.UploadChunk(ctx, user.ID, externalID, "transcript.jsonl", 1, 10, chunk2Data)
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

		// Read merged file
		req := testutil.AuthenticatedRequest(t, "GET",
			"/api/v1/sync/file?session_id="+sessionID+"&file_name=transcript.jsonl", nil, user.ID)

		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify content: should have 10 lines, all from "new" chunk
		body := w.Body.String()
		lines := strings.Split(strings.TrimSpace(body), "\n")

		if len(lines) != 10 {
			t.Errorf("expected 10 lines, got %d: %v", len(lines), lines)
		}

		// All lines should come from the "new" chunk (extends further)
		for i, line := range lines {
			if !strings.Contains(line, `"chunk":"new"`) {
				t.Errorf("line %d should be from 'new' chunk, got: %s", i+1, line)
			}
			expectedLineNum := fmt.Sprintf(`"line":%d`, i+1)
			if !strings.Contains(line, expectedLineNum) {
				t.Errorf("line %d should contain %s, got: %s", i+1, expectedLineNum, line)
			}
		}
	})

	t.Run("merges partially overlapping chunks correctly", func(t *testing.T) {
		// Scenario: chunk 1-5, then chunk 3-10 (partial overlap on 3-5)
		// Should take lines 1-2 from first, lines 3-10 from second
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		externalID := "partial-overlap-session"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		ctx := context.Background()

		// First chunk: lines 1-5
		chunk1Data := []byte(`{"line":1,"source":"A"}
{"line":2,"source":"A"}
{"line":3,"source":"A"}
{"line":4,"source":"A"}
{"line":5,"source":"A"}
`)
		_, err := env.Storage.UploadChunk(ctx, user.ID, externalID, "transcript.jsonl", 1, 5, chunk1Data)
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
		_, err = env.Storage.UploadChunk(ctx, user.ID, externalID, "transcript.jsonl", 3, 10, chunk2Data)
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

		// Read merged file
		req := testutil.AuthenticatedRequest(t, "GET",
			"/api/v1/sync/file?session_id="+sessionID+"&file_name=transcript.jsonl", nil, user.ID)

		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		body := w.Body.String()
		lines := strings.Split(strings.TrimSpace(body), "\n")

		if len(lines) != 10 {
			t.Errorf("expected 10 lines, got %d", len(lines))
		}

		// Lines 1-2 should come from "A" (only chunk covering them)
		// Lines 3-10 should come from "B" (extends further than A)
		expectedSources := []string{"A", "A", "B", "B", "B", "B", "B", "B", "B", "B"}
		for i, line := range lines {
			expectedSource := fmt.Sprintf(`"source":"%s"`, expectedSources[i])
			if !strings.Contains(line, expectedSource) {
				t.Errorf("line %d: expected source %s, got: %s", i+1, expectedSources[i], line)
			}
		}
	})
}

// =============================================================================
// DELETE session with chunks - Cleanup chunked files
// =============================================================================

func TestDeleteSessionWithChunks_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("deletes all chunks when session is deleted", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "delete-test-session")

		server := &Server{db: env.DB, storage: env.Storage}

		// Upload multiple chunks
		chunks := []struct {
			fileName  string
			firstLine int
			lines     []string
		}{
			{"transcript.jsonl", 1, []string{`{"line":1}`}},
			{"transcript.jsonl", 2, []string{`{"line":2}`}},
			{"agent-123.jsonl", 1, []string{`{"agent":1}`}},
		}

		for _, chunk := range chunks {
			reqBody := SyncChunkRequest{
				SessionID: sessionID,
				FileName:  chunk.fileName,
				FileType:  "transcript",
				FirstLine: chunk.firstLine,
				Lines:     chunk.lines,
			}

			req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
			w := httptest.NewRecorder()
			server.handleSyncChunk(w, req)
			testutil.AssertStatus(t, w, http.StatusOK)
		}

		// Verify chunks exist in S3 before deletion
		s3Key1 := buildChunkS3Key(user.ID, "delete-test-session", "transcript.jsonl", 1, 1)
		testutil.VerifyFileInS3(t, env, s3Key1)

		// Delete session (need to send empty JSON body for the handler)
		req := testutil.AuthenticatedRequest(t, "DELETE", "/api/v1/sessions/"+sessionID, struct{}{}, user.ID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleDeleteSession(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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
		_, err := env.Storage.Download(env.Ctx, s3Key1)
		if err == nil {
			t.Error("expected S3 chunk to be deleted")
		}
	})
}

// =============================================================================
// Race condition test for FindOrCreateSyncSession
// =============================================================================

func TestSyncInit_RaceCondition_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("concurrent init requests for same session all succeed", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

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

				reqBody := SyncInitRequest{
					ExternalID:     externalID,
					TranscriptPath: "/home/user/project/transcript.jsonl",
					CWD:            "/home/user/project",
				}

				req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/init", reqBody, user.ID)
				w := httptest.NewRecorder()

				server := &Server{db: env.DB, storage: env.Storage}
				server.handleSyncInit(w, req)

				if w.Code != http.StatusOK {
					results <- result{err: fmt.Errorf("status %d: %s", w.Code, w.Body.String())}
					return
				}

				var resp SyncInitResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					results <- result{err: err}
					return
				}

				results <- result{sessionID: resp.SessionID}
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
			} else {
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

		firstID := sessionIDs[0]
		for i, id := range sessionIDs {
			if id != firstID {
				t.Errorf("session ID mismatch: goroutine 0 got %s, goroutine %d got %s", firstID, i, id)
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

// Note: Request/response types and buildChunkS3Key are defined in sync.go

// =============================================================================
// Summary/FirstUserMessage API Tests
// =============================================================================

func TestSyncChunk_Summary_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("summary last write wins - chunk overwrites previous summary", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-chunk-test")

		// Set initial summary directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET summary = $1 WHERE id = $2",
			"Initial Summary", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial summary: %v", err)
		}

		server := &Server{db: env.DB, storage: env.Storage}

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-overwrite-test")

		// Set initial summary A directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET summary = $1 WHERE id = $2",
			"Summary A", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial summary: %v", err)
		}

		server := &Server{db: env.DB, storage: env.Storage}

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", chunkBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-clear-test")

		// Set initial summary directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET summary = $1 WHERE id = $2",
			"Existing Summary", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial summary: %v", err)
		}

		server := &Server{db: env.DB, storage: env.Storage}

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-preserve-test")

		// Set initial summary directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET summary = $1 WHERE id = $2",
			"Preserved Summary", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial summary: %v", err)
		}

		server := &Server{db: env.DB, storage: env.Storage}

		// Upload chunk WITHOUT summary field (using SyncChunkRequest struct, not map)
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
			// No summary field - should preserve existing
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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

func TestSyncChunk_FirstUserMessage_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("first_user_message first write wins - subsequent chunks do not overwrite", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "first-msg-test")

		server := &Server{db: env.DB, storage: env.Storage}

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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

		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody2, user.ID)
		w = httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "first-msg-immutable-test")

		// Set initial first_user_message directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET first_user_message = $1 WHERE id = $2",
			"Original message", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial first_user_message: %v", err)
		}

		server := &Server{db: env.DB, storage: env.Storage}

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

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", chunkBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "first-msg-preserve-test")

		// Set initial first_user_message directly in DB
		_, err := env.DB.Exec(env.Ctx,
			"UPDATE sessions SET first_user_message = $1 WHERE id = $2",
			"Preserved Message", sessionID)
		if err != nil {
			t.Fatalf("failed to set initial first_user_message: %v", err)
		}

		server := &Server{db: env.DB, storage: env.Storage}

		// Upload chunk WITHOUT first_user_message field
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Hello"}`},
			// No first_user_message field - should preserve existing
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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
// Chunk Count Tracking and Limits
// =============================================================================

func TestHandleSyncChunk_ChunkCountTracking_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("increments chunk_count on each upload", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-chunk-count")

		server := &Server{db: env.DB, storage: env.Storage}

		// Upload first chunk
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"type":"user","message":"Line 1"}`},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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

		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w = httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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

	t.Run("rejects upload when chunk limit exceeded", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-chunk-limit")

		// Insert sync_files record with chunk_count at limit
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, chunk_count)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 100, $2)`,
			sessionID, storage.MaxChunksPerFile)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		server := &Server{db: env.DB, storage: env.Storage}

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 101,
			Lines:     []string{`{"type":"user","message":"Should be rejected"}`},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "too many chunks") {
			t.Errorf("expected error about too many chunks, got: %s", resp["error"])
		}
	})

	t.Run("allows upload when chunk_count is NULL (legacy file)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-legacy-null")

		// Insert sync_files record with NULL chunk_count (legacy)
		_, err := env.DB.Exec(env.Ctx,
			`INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, chunk_count)
			 VALUES ($1, 'transcript.jsonl', 'transcript', 100, NULL)`,
			sessionID)
		if err != nil {
			t.Fatalf("failed to insert sync file: %v", err)
		}

		server := &Server{db: env.DB, storage: env.Storage}

		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 101,
			Lines:     []string{`{"type":"user","message":"Should be allowed"}`},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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
}

func TestHandleSyncFileRead_SelfHealing_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("self-heals chunk_count from NULL to actual count", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		externalID := "test-selfheal-null"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Upload 3 chunks directly to S3
		ctx := context.Background()
		for i := 1; i <= 3; i++ {
			firstLine := (i-1)*10 + 1
			lastLine := i * 10
			data := []byte(fmt.Sprintf(`{"chunk":%d}`, i) + "\n")
			_, err := env.Storage.UploadChunk(ctx, user.ID, externalID, "transcript.jsonl", firstLine, lastLine, data)
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

		// Read the file
		server := &Server{db: env.DB, storage: env.Storage}

		req := testutil.AuthenticatedRequest(t, "GET",
			fmt.Sprintf("/api/v1/sync/file?session_id=%s&file_name=transcript.jsonl", sessionID),
			nil, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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
		externalID := "test-selfheal-wrong"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Upload 2 chunks directly to S3
		ctx := context.Background()
		for i := 1; i <= 2; i++ {
			firstLine := (i-1)*10 + 1
			lastLine := i * 10
			data := []byte(fmt.Sprintf(`{"chunk":%d}`, i) + "\n")
			_, err := env.Storage.UploadChunk(ctx, user.ID, externalID, "transcript.jsonl", firstLine, lastLine, data)
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

		// Read the file
		server := &Server{db: env.DB, storage: env.Storage}

		req := testutil.AuthenticatedRequest(t, "GET",
			fmt.Sprintf("/api/v1/sync/file?session_id=%s&file_name=transcript.jsonl", sessionID),
			nil, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

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
		externalID := "test-selfheal-correct"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		// Upload 2 chunks directly to S3
		ctx := context.Background()
		for i := 1; i <= 2; i++ {
			firstLine := (i-1)*10 + 1
			lastLine := i * 10
			data := []byte(fmt.Sprintf(`{"chunk":%d}`, i) + "\n")
			_, err := env.Storage.UploadChunk(ctx, user.ID, externalID, "transcript.jsonl", firstLine, lastLine, data)
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

		// Read the file
		server := &Server{db: env.DB, storage: env.Storage}

		req := testutil.AuthenticatedRequest(t, "GET",
			fmt.Sprintf("/api/v1/sync/file?session_id=%s&file_name=transcript.jsonl", sessionID),
			nil, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify chunk_count is still 2 and updated_at was NOT changed
		// (since no healing was needed)
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
// GET /api/v1/sync/file - line_offset for incremental fetching
// =============================================================================

func TestHandleSyncFileRead_LineOffset_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	// Helper to upload chunks via API
	uploadChunks := func(t *testing.T, server *Server, userID int64, sessionID string, chunks []struct {
		firstLine int
		lines     []string
	}) {
		for _, chunk := range chunks {
			reqBody := SyncChunkRequest{
				SessionID: sessionID,
				FileName:  "transcript.jsonl",
				FileType:  "transcript",
				FirstLine: chunk.firstLine,
				Lines:     chunk.lines,
			}
			req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, userID)
			w := httptest.NewRecorder()
			server.handleSyncChunk(w, req)
			testutil.AssertStatus(t, w, http.StatusOK)
		}
	}

	// Helper to read file with optional line_offset
	readFile := func(t *testing.T, server *Server, userID int64, sessionID, fileName string, lineOffset *int) *httptest.ResponseRecorder {
		url := "/api/v1/sync/file?session_id=" + sessionID + "&file_name=" + fileName
		if lineOffset != nil {
			url += fmt.Sprintf("&line_offset=%d", *lineOffset)
		}
		req := testutil.AuthenticatedRequest(t, "GET", url, nil, userID)
		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		server.handleSyncFileRead(w, req)
		return w
	}

	t.Run("returns all lines when line_offset is not specified (backward compatible)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-1")
		server := &Server{db: env.DB, storage: env.Storage}

		// Upload chunks with 6 lines total
		uploadChunks(t, server, user.ID, sessionID, []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
			{4, []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}},
		})

		// Read without line_offset
		w := readFile(t, server, user.ID, sessionID, "transcript.jsonl", nil)
		testutil.AssertStatus(t, w, http.StatusOK)

		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		if len(lines) != 6 {
			t.Errorf("expected 6 lines, got %d: %v", len(lines), lines)
		}
	})

	t.Run("returns all lines when line_offset=0", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-2")
		server := &Server{db: env.DB, storage: env.Storage}

		uploadChunks(t, server, user.ID, sessionID, []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
			{4, []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}},
		})

		offset := 0
		w := readFile(t, server, user.ID, sessionID, "transcript.jsonl", &offset)
		testutil.AssertStatus(t, w, http.StatusOK)

		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		if len(lines) != 6 {
			t.Errorf("expected 6 lines, got %d: %v", len(lines), lines)
		}
	})

	t.Run("returns only lines after line_offset", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-3")
		server := &Server{db: env.DB, storage: env.Storage}

		uploadChunks(t, server, user.ID, sessionID, []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
			{4, []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}},
		})

		// Request lines after line 3 (should return lines 4, 5, 6)
		offset := 3
		w := readFile(t, server, user.ID, sessionID, "transcript.jsonl", &offset)
		testutil.AssertStatus(t, w, http.StatusOK)

		body := w.Body.String()
		lines := strings.Split(strings.TrimSpace(body), "\n")

		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d: %v", len(lines), lines)
		}

		// Verify we got lines 4, 5, 6
		for i, line := range lines {
			expected := fmt.Sprintf(`{"line":%d}`, i+4)
			if line != expected {
				t.Errorf("line %d: expected %s, got %s", i, expected, line)
			}
		}

		// Verify line 3 is NOT in output
		if strings.Contains(body, `"line":3`) {
			t.Error("response should not contain line 3")
		}
	})

	t.Run("returns empty response when line_offset equals total lines", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-4")
		server := &Server{db: env.DB, storage: env.Storage}

		uploadChunks(t, server, user.ID, sessionID, []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
		})

		// Request lines after line 3 (none exist)
		offset := 3
		w := readFile(t, server, user.ID, sessionID, "transcript.jsonl", &offset)
		testutil.AssertStatus(t, w, http.StatusOK)

		body := strings.TrimSpace(w.Body.String())
		if body != "" {
			t.Errorf("expected empty response, got: %s", body)
		}
	})

	t.Run("returns empty response when line_offset exceeds total lines", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-5")
		server := &Server{db: env.DB, storage: env.Storage}

		uploadChunks(t, server, user.ID, sessionID, []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
		})

		// Request lines after line 100 (none exist)
		offset := 100
		w := readFile(t, server, user.ID, sessionID, "transcript.jsonl", &offset)
		testutil.AssertStatus(t, w, http.StatusOK)

		body := strings.TrimSpace(w.Body.String())
		if body != "" {
			t.Errorf("expected empty response, got: %s", body)
		}
	})

	t.Run("returns 400 for negative line_offset", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-6")
		server := &Server{db: env.DB, storage: env.Storage}

		uploadChunks(t, server, user.ID, sessionID, []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`}},
		})

		offset := -1
		w := readFile(t, server, user.ID, sessionID, "transcript.jsonl", &offset)
		testutil.AssertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("returns 400 for invalid line_offset", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-7")

		// Use string instead of int for line_offset
		url := "/api/v1/sync/file?session_id=" + sessionID + "&file_name=transcript.jsonl&line_offset=invalid"
		req := testutil.AuthenticatedRequest(t, "GET", url, nil, user.ID)
		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSyncFileRead(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("filters output to lines within a single chunk", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-8")
		server := &Server{db: env.DB, storage: env.Storage}

		// Upload one chunk with 5 lines
		uploadChunks(t, server, user.ID, sessionID, []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`, `{"line":2}`, `{"line":3}`, `{"line":4}`, `{"line":5}`}},
		})

		// Request lines after line 2 (should return lines 3, 4, 5)
		offset := 2
		w := readFile(t, server, user.ID, sessionID, "transcript.jsonl", &offset)
		testutil.AssertStatus(t, w, http.StatusOK)

		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "offset-test-9")
		server := &Server{db: env.DB, storage: env.Storage}

		// Upload 3 chunks with 3 lines each (total 9 lines)
		uploadChunks(t, server, user.ID, sessionID, []struct {
			firstLine int
			lines     []string
		}{
			{1, []string{`{"line":1}`, `{"line":2}`, `{"line":3}`}},
			{4, []string{`{"line":4}`, `{"line":5}`, `{"line":6}`}},
			{7, []string{`{"line":7}`, `{"line":8}`, `{"line":9}`}},
		})

		// Request lines after line 5 (should return lines 6, 7, 8, 9)
		offset := 5
		w := readFile(t, server, user.ID, sessionID, "transcript.jsonl", &offset)
		testutil.AssertStatus(t, w, http.StatusOK)

		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
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

// TestHandleSyncFileRead_LineOffset_DBShortCircuit tests the behavior where
// line_offset >= last_synced_line returns empty response efficiently.
// The DB short-circuit optimization (skipping S3) is verified through functional tests.
func TestHandleSyncFileRead_LineOffset_DBShortCircuit_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	// Helper to read file with line_offset
	readFileWithOffset := func(t *testing.T, server *Server, userID int64, sessionID, fileName string, lineOffset int) *httptest.ResponseRecorder {
		url := fmt.Sprintf("/api/v1/sync/file?session_id=%s&file_name=%s&line_offset=%d", sessionID, fileName, lineOffset)
		req := testutil.AuthenticatedRequest(t, "GET", url, nil, userID)
		rctx := chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		server.handleSyncFileRead(w, req)
		return w
	}

	t.Run("verifies last_synced_line is correctly tracked", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "shortcircuit-test-1")
		server := &Server{db: env.DB, storage: env.Storage}

		// Upload a chunk with 3 lines
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"line":1}`, `{"line":2}`, `{"line":3}`},
		}
		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

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
		w = readFileWithOffset(t, server, user.ID, sessionID, "transcript.jsonl", 3)
		testutil.AssertStatus(t, w, http.StatusOK)
		body := strings.TrimSpace(w.Body.String())
		if body != "" {
			t.Errorf("expected empty response for line_offset=last_synced_line, got: %s", body)
		}
	})

	t.Run("returns new lines after incremental sync", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "shortcircuit-test-2")
		server := &Server{db: env.DB, storage: env.Storage}

		// Upload first chunk (lines 1-3)
		reqBody := SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 1,
			Lines:     []string{`{"line":1}`, `{"line":2}`, `{"line":3}`},
		}
		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w := httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

		// Client knows it has 3 lines, polls with line_offset=3
		w = readFileWithOffset(t, server, user.ID, sessionID, "transcript.jsonl", 3)
		testutil.AssertStatus(t, w, http.StatusOK)
		if strings.TrimSpace(w.Body.String()) != "" {
			t.Errorf("expected empty before new sync, got: %s", w.Body.String())
		}

		// New data arrives - upload second chunk (lines 4-6)
		reqBody = SyncChunkRequest{
			SessionID: sessionID,
			FileName:  "transcript.jsonl",
			FileType:  "transcript",
			FirstLine: 4,
			Lines:     []string{`{"line":4}`, `{"line":5}`, `{"line":6}`},
		}
		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w = httptest.NewRecorder()
		server.handleSyncChunk(w, req)
		testutil.AssertStatus(t, w, http.StatusOK)

		// Client polls again with same line_offset=3, now gets new lines
		w = readFileWithOffset(t, server, user.ID, sessionID, "transcript.jsonl", 3)
		testutil.AssertStatus(t, w, http.StatusOK)

		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 new lines, got %d: %v", len(lines), lines)
		}

		// Verify content is lines 4-6
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
		sessionID := testutil.CreateTestSession(t, env, user.ID, "shortcircuit-test-3")
		server := &Server{db: env.DB, storage: env.Storage}

		// No chunks uploaded, so no sync_files record
		w := readFileWithOffset(t, server, user.ID, sessionID, "transcript.jsonl", 0)
		testutil.AssertStatus(t, w, http.StatusNotFound)
	})
}
