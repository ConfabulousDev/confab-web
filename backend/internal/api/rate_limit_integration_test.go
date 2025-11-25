package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/santaclaude2025/confab/backend/internal/testutil"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/models"
)

func TestRateLimitEnforcement(t *testing.T) {
	// Skip if running unit tests only
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	env := testutil.SetupTestEnvironment(t)
	defer env.CleanDB(t)

	// Create test user
	user := testutil.CreateTestUser(t, env, "ratelimit@example.com", "Rate Limit User")

	// Create server instance
	server := &Server{db: env.DB, storage: env.Storage}

	// Helper to create upload request
	makeUploadRequest := func(externalID string) *http.Request {
		req := models.SaveSessionRequest{
			ExternalID:     externalID,
			TranscriptPath: "test.txt",
			CWD:            "/test",
			Reason:         "test upload",
			Files: []models.FileUpload{
				{
					Path:      "test.txt",
					Type:      "transcript",
					Content:   []byte("test content"),
					SizeBytes: 12,
				},
			},
		}

		return testutil.AuthenticatedRequest(t, http.MethodPost, "/api/v1/sessions/save", req, user.ID)
	}

	t.Run("allows uploads under limit", func(t *testing.T) {
		// Upload 5 sessions (should succeed)
		for i := 0; i < 5; i++ {
			req := makeUploadRequest("test-session-under-limit")
			w := httptest.NewRecorder()

			server.handleSaveSession(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Upload %d failed with status %d, expected 200. Body: %s", i+1, w.Code, w.Body.String())
			}
		}

		// Verify count
		count, err := env.DB.CountUserRunsInLastWeek(env.Ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to count runs: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected 5 runs, got %d", count)
		}
	})

	t.Run("blocks uploads at limit", func(t *testing.T) {
		// Upload 195 more sessions to reach 200 total (5 from previous test + 195 = 200)
		for i := 0; i < 195; i++ {
			req := makeUploadRequest("test-session-at-limit")
			w := httptest.NewRecorder()

			server.handleSaveSession(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Upload %d failed with status %d before reaching limit", i+1, w.Code)
			}
		}

		// Verify we're at 200
		count, err := env.DB.CountUserRunsInLastWeek(env.Ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to count runs: %v", err)
		}
		if count != 200 {
			t.Errorf("Expected 200 runs, got %d", count)
		}

		// Next upload should be blocked (201st)
		req := makeUploadRequest("test-session-over-limit")
		w := httptest.NewRecorder()

		server.handleSaveSession(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status 429 (Too Many Requests), got %d. Body: %s", w.Code, w.Body.String())
		}

		// Verify error message
		body := w.Body.String()
		if body == "" {
			t.Error("Expected error message in response body")
		}
		t.Logf("Rate limit error message: %s", body)

		// Verify count is still 200 (upload was blocked)
		count, err = env.DB.CountUserRunsInLastWeek(env.Ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to count runs: %v", err)
		}
		if count != 200 {
			t.Errorf("Expected count to stay at 200, got %d", count)
		}
	})

	t.Run("boundary condition - exactly 200", func(t *testing.T) {
		// Current count should be exactly 200
		count, err := env.DB.CountUserRunsInLastWeek(env.Ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to count runs: %v", err)
		}
		if count != 200 {
			t.Errorf("Expected exactly 200 runs, got %d", count)
		}

		// Upload should be rejected
		req := makeUploadRequest("boundary-test")
		w := httptest.NewRecorder()

		server.handleSaveSession(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status 429 at boundary, got %d", w.Code)
		}
	})

	t.Run("old runs don't count toward limit", func(t *testing.T) {
		// Create a session and run, then backdate it to 8 days ago
		externalID := "old-session-test"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)
		runID := testutil.CreateTestRun(t, env, sessionID, "old test", "/cwd", "/old/path")

		// Backdate the run by 8 days
		eightDaysAgo := time.Now().UTC().Add(-8 * 24 * time.Hour)
		testutil.BackdateRun(t, env, runID, eightDaysAgo)

		// Count should still be 200 (old run doesn't count)
		count, err := env.DB.CountUserRunsInLastWeek(env.Ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to count runs: %v", err)
		}
		if count != 200 {
			t.Errorf("Expected 200 runs (old run excluded), got %d", count)
		}

		// Upload should still be blocked
		req := makeUploadRequest("old-run-test")
		w := httptest.NewRecorder()

		server.handleSaveSession(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status 429 (old run shouldn't help), got %d", w.Code)
		}
	})
}

func TestWeeklyUsageEndpoint(t *testing.T) {
	// Skip if running unit tests only
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	env := testutil.SetupTestEnvironment(t)
	defer env.CleanDB(t)

	// Create server instance
	server := &Server{db: env.DB, storage: env.Storage}

	// Create test user
	user := testutil.CreateTestUser(t, env, "usage-endpoint@example.com", "Usage Endpoint User")

	t.Run("returns zero usage for new user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/weekly", nil)
		ctx := context.WithValue(req.Context(), auth.GetUserIDContextKey(), user.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		server.handleGetWeeklyUsage(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var usage map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&usage); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if usage["runs_uploaded"].(float64) != 0 {
			t.Errorf("Expected runs_uploaded=0, got %v", usage["runs_uploaded"])
		}
		if usage["limit"].(float64) != 200 {
			t.Errorf("Expected limit=200, got %v", usage["limit"])
		}
		if usage["remaining"].(float64) != 200 {
			t.Errorf("Expected remaining=200, got %v", usage["remaining"])
		}
	})

	t.Run("returns correct usage after uploads", func(t *testing.T) {
		// Create 10 runs
		externalID := "usage-test-session"
		sessionID := testutil.CreateTestSession(t, env, user.ID, externalID)

		for i := 0; i < 10; i++ {
			testutil.CreateTestRun(t, env, sessionID, "test", "/cwd", "/test")
		}

		// Get usage
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/weekly", nil)
		ctx := context.WithValue(req.Context(), auth.GetUserIDContextKey(), user.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		server.handleGetWeeklyUsage(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var usage map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&usage); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if usage["runs_uploaded"].(float64) != 10 {
			t.Errorf("Expected runs_uploaded=10, got %v", usage["runs_uploaded"])
		}
		if usage["remaining"].(float64) != 190 {
			t.Errorf("Expected remaining=190, got %v", usage["remaining"])
		}
	})

	t.Run("requires authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/weekly", nil)
		// No auth context

		w := httptest.NewRecorder()
		server.handleGetWeeklyUsage(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}
