package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/santaclaude2025/confab/backend/internal/testutil"
	"github.com/santaclaude2025/confab/backend/internal/models"
)

// TestHandleSaveSession_Integration tests the handleSaveSession handler with real database and S3
func TestHandleSaveSession_Integration(t *testing.T) {
	// Skip if running unit tests only (go test -short)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment (PostgreSQL + MinIO containers)
	env := testutil.SetupTestEnvironment(t)

	t.Run("uploads session with multiple files successfully", func(t *testing.T) {
		// Clean DB state for test isolation
		env.CleanDB(t)

		// Create test user
		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Create session upload request
		externalID := "test-session-upload-123"
		reqBody := models.SaveSessionRequest{
			ExternalID:     externalID,
			TranscriptPath: "transcript.jsonl",
			CWD:            "/home/user/project",
			Reason:         "Test upload",
			Files: []models.FileUpload{
				{
					Path:      "transcript.jsonl",
					Type:      "transcript",
					Content:   []byte(`{"type":"message","content":"test"}`),
					SizeBytes: 36,
				},
				{
					Path:      "agent_001.jsonl",
					Type:      "agent",
					Content:   []byte(`{"type":"agent","name":"test-agent"}`),
					SizeBytes: 39,
				},
			},
		}

		// Create authenticated request
		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", reqBody, user.ID)

		// Execute handler
		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSaveSession(w, req)

		// Assert response
		testutil.AssertStatus(t, w, http.StatusOK)

		var resp models.SaveSessionResponse
		testutil.ParseJSONResponse(t, w, &resp)

		if !resp.Success {
			t.Error("expected success=true")
		}
		if resp.ExternalID != externalID {
			t.Errorf("expected external_id %s, got %s", externalID, resp.ExternalID)
		}
		if resp.ID == "" {
			t.Error("expected non-empty id (UUID)")
		}
		if resp.RunID == 0 {
			t.Error("expected non-zero run_id")
		}

		// Verify database state - session exists
		var sessionCount int
		row := env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM sessions WHERE external_id = $1 AND user_id = $2",
			externalID, user.ID)
		if err := row.Scan(&sessionCount); err != nil {
			t.Fatalf("failed to query sessions: %v", err)
		}
		if sessionCount != 1 {
			t.Errorf("expected 1 session in database, got %d", sessionCount)
		}

		// Verify run exists
		var runCount int
		row = env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM runs WHERE id = $1",
			resp.RunID)
		if err := row.Scan(&runCount); err != nil {
			t.Fatalf("failed to query runs: %v", err)
		}
		if runCount != 1 {
			t.Errorf("expected 1 run in database, got %d", runCount)
		}

		// Verify files exist
		var fileCount int
		row = env.DB.QueryRow(env.Ctx,
			"SELECT COUNT(*) FROM files WHERE run_id = $1",
			resp.RunID)
		if err := row.Scan(&fileCount); err != nil {
			t.Fatalf("failed to query files: %v", err)
		}
		if fileCount != 2 {
			t.Errorf("expected 2 files in database, got %d", fileCount)
		}

		// Verify files uploaded to S3
		for _, file := range reqBody.Files {
			var s3Key string
			row := env.DB.QueryRow(env.Ctx,
				"SELECT s3_key FROM files WHERE run_id = $1 AND file_path = $2",
				resp.RunID, file.Path)
			if err := row.Scan(&s3Key); err != nil {
				t.Fatalf("failed to get s3_key for file %s: %v", file.Path, err)
			}

			// Download from S3 and verify content
			testutil.AssertFileContent(t, env, s3Key, file.Content)
		}
	})

	t.Run("returns 400 for missing external_id", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := models.SaveSessionRequest{
			ExternalID:     "", // Empty - should fail
			TranscriptPath: "transcript.jsonl",
			Files: []models.FileUpload{
				{
					Path:    "transcript.jsonl",
					Type:    "transcript",
					Content: []byte("test"),
				},
			},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", reqBody, user.ID)
		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSaveSession(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if resp["error"] == "" {
			t.Error("expected error message for missing external_id")
		}
		if !strings.Contains(resp["error"], "external_id") {
			t.Errorf("expected error about external_id, got: %s", resp["error"])
		}
	})

	t.Run("returns 400 for empty files array", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := models.SaveSessionRequest{
			ExternalID:     "test-session-empty-files",
			TranscriptPath: "transcript.jsonl",
			Files:          []models.FileUpload{}, // Empty - should fail
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", reqBody, user.ID)
		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSaveSession(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "files array cannot be empty") {
			t.Errorf("expected error about empty files, got: %s", resp["error"])
		}
	})

	t.Run("returns 400 for path traversal attempt", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := models.SaveSessionRequest{
			ExternalID:     "test-session-path-traversal",
			TranscriptPath: "transcript.jsonl",
			Files: []models.FileUpload{
				{
					Path:    "../../../etc/passwd", // Path traversal attempt
					Type:    "transcript",
					Content: []byte("malicious"),
				},
			},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", reqBody, user.ID)
		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSaveSession(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "..") {
			t.Errorf("expected error about path traversal, got: %s", resp["error"])
		}
	})

	t.Run("returns 400 for external_id too long", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Create external ID longer than MaxExternalIDLength (256)
		longExternalID := strings.Repeat("a", 257)

		reqBody := models.SaveSessionRequest{
			ExternalID:     longExternalID,
			TranscriptPath: "transcript.jsonl",
			Files: []models.FileUpload{
				{
					Path:    "transcript.jsonl",
					Type:    "transcript",
					Content: []byte("test"),
				},
			},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", reqBody, user.ID)
		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSaveSession(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "external_id must be between") {
			t.Errorf("expected error about external_id length, got: %s", resp["error"])
		}
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		reqBody := models.SaveSessionRequest{
			ExternalID:     "test-session-unauth",
			TranscriptPath: "transcript.jsonl",
			Files: []models.FileUpload{
				{
					Path:    "transcript.jsonl",
					Type:    "transcript",
					Content: []byte("test"),
				},
			},
		}

		// Create request WITHOUT user authentication
		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/save", reqBody, 0)
		// Remove user ID from context to simulate unauthenticated request
		req = req.WithContext(context.Background())

		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		server.handleSaveSession(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})
}

// TestHandleCheckSessions_Integration tests the HandleCheckSessions handler
func TestHandleCheckSessions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns existing and missing external IDs", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Create two sessions
		testutil.CreateTestSession(t, env, user.ID, "session-exists-1")
		testutil.CreateTestSession(t, env, user.ID, "session-exists-2")

		// Check which sessions exist
		reqBody := struct {
			ExternalIDs []string `json:"external_ids"`
		}{
			ExternalIDs: []string{
				"session-exists-1",
				"session-exists-2",
				"session-missing-1",
				"session-missing-2",
			},
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/check", reqBody, user.ID)

		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		handler := HandleCheckSessions(server)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		var resp map[string][]string
		testutil.ParseJSONResponse(t, w, &resp)

		// Check existing sessions
		if len(resp["existing"]) != 2 {
			t.Errorf("expected 2 existing sessions, got %d", len(resp["existing"]))
		}

		// Check missing sessions
		if len(resp["missing"]) != 2 {
			t.Errorf("expected 2 missing sessions, got %d", len(resp["missing"]))
		}

		// Verify specific IDs
		existingMap := make(map[string]bool)
		for _, id := range resp["existing"] {
			existingMap[id] = true
		}

		if !existingMap["session-exists-1"] {
			t.Error("expected session-exists-1 in existing list")
		}
		if !existingMap["session-exists-2"] {
			t.Error("expected session-exists-2 in existing list")
		}

		missingMap := make(map[string]bool)
		for _, id := range resp["missing"] {
			missingMap[id] = true
		}

		if !missingMap["session-missing-1"] {
			t.Error("expected session-missing-1 in missing list")
		}
		if !missingMap["session-missing-2"] {
			t.Error("expected session-missing-2 in missing list")
		}
	})

	t.Run("returns 400 for empty external_ids array", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		reqBody := struct {
			ExternalIDs []string `json:"external_ids"`
		}{
			ExternalIDs: []string{}, // Empty - should fail
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/check", reqBody, user.ID)

		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		handler := HandleCheckSessions(server)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "external_ids is required") {
			t.Errorf("expected error about external_ids, got: %s", resp["error"])
		}
	})

	t.Run("returns 400 for too many external IDs", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		// Create 1001 external IDs (limit is 1000)
		externalIDs := make([]string, 1001)
		for i := 0; i < 1001; i++ {
			externalIDs[i] = "session-" + strings.Repeat("a", 10)
		}

		reqBody := struct {
			ExternalIDs []string `json:"external_ids"`
		}{
			ExternalIDs: externalIDs,
		}

		req := testutil.AuthenticatedRequest(t, "POST", "/api/v1/sessions/check", reqBody, user.ID)

		w := httptest.NewRecorder()
		server := &Server{db: env.DB, storage: env.Storage}
		handler := HandleCheckSessions(server)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)

		var resp map[string]string
		testutil.ParseJSONResponse(t, w, &resp)

		if !strings.Contains(resp["error"], "Too many external IDs") {
			t.Errorf("expected error about too many IDs, got: %s", resp["error"])
		}
	})
}
