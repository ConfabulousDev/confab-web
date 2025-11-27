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
	"github.com/santaclaude2025/confab/backend/internal/testutil"
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

	t.Run("handles idempotent re-upload (same line range overwrites)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-idempotent")

		lines := []string{
			`{"type":"user","message":"Original content"}`,
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

		// Re-upload same range (idempotent - should overwrite)
		updatedLines := []string{
			`{"type":"user","message":"Updated content"}`,
		}

		reqBody.Lines = updatedLines

		req = testutil.AuthenticatedRequest(t, "POST", "/api/v1/sync/chunk", reqBody, user.ID)
		w = httptest.NewRecorder()

		server.handleSyncChunk(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify S3 has updated content
		expectedS3Key := buildChunkS3Key(user.ID, "test-session-idempotent", "transcript.jsonl", 1, 1)
		content := testutil.VerifyFileInS3(t, env, expectedS3Key)

		if !strings.Contains(string(content), "Updated content") {
			t.Error("expected S3 to contain updated content after idempotent re-upload")
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
		handler := HandleDeleteSessionOrRun(env.DB, env.Storage)
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
