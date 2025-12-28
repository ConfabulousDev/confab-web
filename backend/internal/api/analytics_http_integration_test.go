package api

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// Session Analytics HTTP Integration Tests
//
// GET /api/v1/sessions/{id}/analytics
// =============================================================================

func TestGetSessionAnalytics_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns analytics for session owner", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Upload JSONL content with assistant messages
		jsonlContent := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":30}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":200,"output_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":50}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session", "transcript.jsonl", 1, 3, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 3)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.AnalyticsResponse
		testutil.ParseJSON(t, resp, &result)

		// Verify token stats (sum of both assistant messages)
		if result.Tokens.Input != 300 {
			t.Errorf("expected input tokens 300, got %d", result.Tokens.Input)
		}
		if result.Tokens.Output != 150 {
			t.Errorf("expected output tokens 150, got %d", result.Tokens.Output)
		}
		if result.Tokens.CacheCreation != 20 {
			t.Errorf("expected cache creation tokens 20, got %d", result.Tokens.CacheCreation)
		}
		if result.Tokens.CacheRead != 80 {
			t.Errorf("expected cache read tokens 80, got %d", result.Tokens.CacheRead)
		}

		// Verify cost is computed (non-zero)
		if result.Cost.EstimatedUSD.IsZero() {
			t.Error("expected non-zero cost")
		}
	})

	t.Run("returns empty analytics for session with no transcript", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.AnalyticsResponse
		testutil.ParseJSON(t, resp, &result)

		// Should be empty/zero
		if result.Tokens.Input != 0 {
			t.Errorf("expected 0 input tokens, got %d", result.Tokens.Input)
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/sessions/00000000-0000-0000-0000-000000000000/analytics")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 404 for other user's private session", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "user1@example.com", "User 1")
		user2 := testutil.CreateTestUser(t, env, "user2@example.com", "User 2")
		sessionID := testutil.CreateTestSession(t, env, user1.ID, "user1-session")
		user2Token := testutil.CreateTestWebSessionWithToken(t, env, user2.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(user2Token)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns analytics for publicly shared session without auth", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Create public share
		testutil.CreateTestShare(t, env, sessionID, true, nil, nil)

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":500,"output_tokens":200}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No auth

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.AnalyticsResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Tokens.Input != 500 {
			t.Errorf("expected input tokens 500, got %d", result.Tokens.Input)
		}
	})

	t.Run("caches analytics and returns cached result", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// First request - computes and caches
		resp1, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request 1 failed: %v", err)
		}
		resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusOK)

		// Second request - should return cached result
		resp2, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request 2 failed: %v", err)
		}
		defer resp2.Body.Close()
		testutil.RequireStatus(t, resp2, http.StatusOK)

		var result analytics.AnalyticsResponse
		testutil.ParseJSON(t, resp2, &result)

		// Same result
		if result.Tokens.Input != 100 {
			t.Errorf("expected input tokens 100, got %d", result.Tokens.Input)
		}
	})

	t.Run("invalidates cache when new data is synced", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Upload initial JSONL content (1 line, 100 input tokens)
		jsonlContent1 := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session", "transcript.jsonl", 1, 1, []byte(jsonlContent1))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// First request - computes and caches with up_to_line=1
		resp1, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request 1 failed: %v", err)
		}
		defer resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusOK)

		var result1 analytics.AnalyticsResponse
		testutil.ParseJSON(t, resp1, &result1)

		if result1.Tokens.Input != 100 {
			t.Errorf("expected initial input tokens 100, got %d", result1.Tokens.Input)
		}
		if result1.ComputedLines != 1 {
			t.Errorf("expected computed_lines 1, got %d", result1.ComputedLines)
		}

		// Simulate CLI syncing new data: upload new chunk with additional line
		jsonlContent2 := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":200,"output_tokens":100}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session", "transcript.jsonl", 1, 2, []byte(jsonlContent2))
		// Update sync_files to reflect new line count (CreateTestSyncFile uses ON CONFLICT DO UPDATE)
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 2)

		// Second request - cache should be invalid (line count mismatch), recompute
		resp2, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request 2 failed: %v", err)
		}
		defer resp2.Body.Close()
		testutil.RequireStatus(t, resp2, http.StatusOK)

		var result2 analytics.AnalyticsResponse
		testutil.ParseJSON(t, resp2, &result2)

		// Should reflect NEW data (100 + 200 = 300 input tokens)
		if result2.Tokens.Input != 300 {
			t.Errorf("expected updated input tokens 300, got %d", result2.Tokens.Input)
		}
		if result2.ComputedLines != 2 {
			t.Errorf("expected computed_lines 2, got %d", result2.ComputedLines)
		}
	})

	t.Run("compaction stats are computed correctly", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Upload JSONL with compaction boundaries
		jsonlContent := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:10Z"}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":80,"output_tokens":40}},"uuid":"a2","timestamp":"2025-01-01T00:01:00Z"}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"manual","preTokens":60000},"logicalParentUuid":"a2","uuid":"c2","timestamp":"2025-01-01T00:02:00Z"}
`
		testutil.UploadTestChunk(t, env, user.ID, "test-session", "transcript.jsonl", 1, 4, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 4)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.AnalyticsResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Compaction.Auto != 1 {
			t.Errorf("expected 1 auto compaction, got %d", result.Compaction.Auto)
		}
		if result.Compaction.Manual != 1 {
			t.Errorf("expected 1 manual compaction, got %d", result.Compaction.Manual)
		}
		// Avg time should be ~5000ms (5 seconds from a1 to c1)
		if result.Compaction.AvgTimeMs == nil {
			t.Error("expected avg compaction time to be set")
		} else if *result.Compaction.AvgTimeMs != 5000 {
			t.Errorf("expected avg compaction time 5000ms, got %d", *result.Compaction.AvgTimeMs)
		}
	})
}
