package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/api/testutil"
)

// TestHandleGetFileContent_Integration tests file downloads with real database and S3
func TestHandleGetFileContent_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("downloads file successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := "test-session"

		testutil.CreateTestSession(t, env, user.ID, sessionID)
		runID := testutil.CreateTestRun(t, env, sessionID, user.ID, "Test", "/home/test", "transcript.jsonl")

		// Upload file to S3
		fileContent := []byte(`{"type":"message","content":"test"}`)
		s3Key := testutil.UploadTestFile(t, env, user.ID, sessionID, "transcript.jsonl", fileContent)

		// Create file record in database
		fileID := testutil.CreateTestFile(t, env, runID, "transcript.jsonl", "transcript", s3Key, int64(len(fileContent)))

		fileIDStr := strconv.FormatInt(fileID, 10)
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/files/"+fileIDStr, nil, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("fileId", fileIDStr)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify content-type
		contentType := w.Header().Get("Content-Type")
		if contentType != "application/x-ndjson" {
			t.Errorf("expected content-type 'application/x-ndjson', got %s", contentType)
		}

		// Verify content
		if w.Body.String() != string(fileContent) {
			t.Errorf("expected content %s, got %s", string(fileContent), w.Body.String())
		}
	})

	t.Run("returns 404 for non-existent file", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/files/99999", nil, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("fileId", "99999")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("prevents accessing another user's file", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User One")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User Two")

		sessionID := "user2-session"
		testutil.CreateTestSession(t, env, user2.ID, sessionID)
		runID := testutil.CreateTestRun(t, env, sessionID, user2.ID, "Test", "/home/test", "transcript.jsonl")

		fileContent := []byte("user2 file")
		s3Key := testutil.UploadTestFile(t, env, user2.ID, sessionID, "file.txt", fileContent)

		fileID := testutil.CreateTestFile(t, env, runID, "file.txt", "transcript", s3Key, int64(len(fileContent)))

		// Try to access as user1
		fileIDStr := strconv.FormatInt(fileID, 10)
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/files/"+fileIDStr, nil, user1.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("fileId", fileIDStr)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("returns 400 for invalid file ID", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/files/invalid", nil, user.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("fileId", "invalid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("returns 401 for unauthenticated request", func(t *testing.T) {
		env.CleanDB(t)

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/files/123", nil, 0)
		req = req.WithContext(context.Background())

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("fileId", "123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusUnauthorized)
	})
}

// TestHandleGetSharedFileContent_Integration tests shared file downloads
func TestHandleGetSharedFileContent_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("downloads public shared file successfully", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := "shared-session"
		shareToken := "12345678901234567890123456789012" // Exactly 32 chars

		testutil.CreateTestSession(t, env, user.ID, sessionID)
		runID := testutil.CreateTestRun(t, env, sessionID, user.ID, "Test", "/home/test", "transcript.jsonl")

		// Upload file to S3
		fileContent := []byte(`{"type":"message","content":"shared"}`)
		s3Key := testutil.UploadTestFile(t, env, user.ID, sessionID, "transcript.jsonl", fileContent)

		// Create file record
		fileID := testutil.CreateTestFile(t, env, runID, "transcript.jsonl", "transcript", s3Key, int64(len(fileContent)))

		// Create public share
		testutil.CreateTestShare(t, env, sessionID, user.ID, shareToken, "public", nil, nil)

		fileIDStr := strconv.FormatInt(fileID, 10)
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/share/"+sessionID+"/"+shareToken+"/files/"+fileIDStr, nil, 0)
		req = req.WithContext(context.Background()) // No auth required for public shares

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionId", sessionID)
		rctx.URLParams.Add("shareToken", shareToken)
		rctx.URLParams.Add("fileId", fileIDStr)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSharedFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusOK)

		// Verify content
		if w.Body.String() != string(fileContent) {
			t.Errorf("expected content %s, got %s", string(fileContent), w.Body.String())
		}
	})

	t.Run("returns 404 for invalid share token", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := "shared-session"

		testutil.CreateTestSession(t, env, user.ID, sessionID)
		runID := testutil.CreateTestRun(t, env, sessionID, user.ID, "Test", "/home/test", "transcript.jsonl")

		fileContent := []byte("test")
		s3Key := testutil.UploadTestFile(t, env, user.ID, sessionID, "file.txt", fileContent)

		fileID := testutil.CreateTestFile(t, env, runID, "file.txt", "transcript", s3Key, int64(len(fileContent)))

		// Create share with different token (must be 32 hex chars)
		correctToken := "abc123def456abc123def456abc123de" // 32 hex chars
		wrongToken := "111222333444555666777888999aaabb"   // 32 hex chars (different)
		testutil.CreateTestShare(t, env, sessionID, user.ID, correctToken, "public", nil, nil)

		fileIDStr := strconv.FormatInt(fileID, 10)
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/share/"+sessionID+"/"+wrongToken+"/files/"+fileIDStr, nil, 0)
		req = req.WithContext(context.Background())

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionId", sessionID)
		rctx.URLParams.Add("shareToken", wrongToken)
		rctx.URLParams.Add("fileId", fileIDStr)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSharedFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusNotFound)
	})

	t.Run("returns 410 for expired share", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := "expired-session"
		shareToken := "deadbeefdeadbeefdeadbeefdeadbeef" // 32 hex chars

		testutil.CreateTestSession(t, env, user.ID, sessionID)
		runID := testutil.CreateTestRun(t, env, sessionID, user.ID, "Test", "/home/test", "transcript.jsonl")

		fileContent := []byte("test")
		s3Key := testutil.UploadTestFile(t, env, user.ID, sessionID, "file.txt", fileContent)

		fileID := testutil.CreateTestFile(t, env, runID, "file.txt", "transcript", s3Key, int64(len(fileContent)))

		// Create expired share (yesterday)
		yesterday := time.Now().Add(-24 * time.Hour)
		testutil.CreateTestShare(t, env, sessionID, user.ID, shareToken, "public", &yesterday, nil)

		fileIDStr := strconv.FormatInt(fileID, 10)
		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/share/"+sessionID+"/"+shareToken+"/files/"+fileIDStr, nil, 0)
		req = req.WithContext(context.Background())

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionId", sessionID)
		rctx.URLParams.Add("shareToken", shareToken)
		rctx.URLParams.Add("fileId", fileIDStr)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSharedFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusGone)
	})

	t.Run("returns 400 for invalid session ID", func(t *testing.T) {
		env.CleanDB(t)

		req := testutil.AuthenticatedRequest(t, "GET", "/api/v1/share/invalid/token/files/1", nil, 0)
		req = req.WithContext(context.Background())

		// Add URL parameters with too-long session ID
		longSessionID := string(make([]byte, 257))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionId", longSessionID)
		rctx.URLParams.Add("shareToken", "token")
		rctx.URLParams.Add("fileId", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler := HandleGetSharedFileContent(env.DB, env.Storage)
		handler(w, req)

		testutil.AssertStatus(t, w, http.StatusBadRequest)
	})
}
