package api

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
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

		// Upload JSONL content with assistant messages (using full schema-compliant format)
		jsonlContent := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4-20241022","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":30}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"u1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_2","type":"message","model":"claude-sonnet-4-20241022","role":"assistant","content":[{"type":"text","text":"Goodbye!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":200,"output_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":50}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 3, []byte(jsonlContent))
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

		// Upload JSONL content (using full schema-compliant format)
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":500,"output_tokens":200}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 1, []byte(jsonlContent))
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

		// Upload JSONL content (using full schema-compliant format)
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 1, []byte(jsonlContent))
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

		// Upload initial JSONL content (1 line, 100 input tokens, using full schema-compliant format)
		jsonlContent1 := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 1, []byte(jsonlContent1))
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

		// Simulate CLI syncing new data: upload new chunk with additional line (using full schema-compliant format)
		jsonlContent2 := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_2","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Goodbye!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":200,"output_tokens":100}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 2, []byte(jsonlContent2))
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

		// Upload JSONL with compaction boundaries (using full schema-compliant format)
		jsonlContent := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:10Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0","isMeta":true}
{"type":"assistant","message":{"id":"msg_2","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Continuing!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":80,"output_tokens":40}},"uuid":"a2","timestamp":"2025-01-01T00:01:00Z","parentUuid":"c1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"manual","preTokens":60000},"logicalParentUuid":"a2","uuid":"c2","timestamp":"2025-01-01T00:02:00Z","parentUuid":"a2","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0","isMeta":true}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 4, []byte(jsonlContent))
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

	t.Run("returns 304 when as_of_line equals current line count", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Upload JSONL content (3 lines, using full schema-compliant format)
		jsonlContent := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"u1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"user","message":{"role":"user","content":"thanks"},"uuid":"u2","timestamp":"2025-01-01T00:00:02Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 3, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 3)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Request with as_of_line=3 (equal to current line count)
		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics?as_of_line=3", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotModified)
	})

	t.Run("returns 304 when as_of_line exceeds current line count", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Upload JSONL content (2 lines, using full schema-compliant format)
		jsonlContent := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"u1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 2, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 2)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Request with as_of_line=100 (greater than current line count of 2)
		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics?as_of_line=100", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotModified)
	})

	t.Run("returns fresh analytics when as_of_line is less than current line count", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Upload JSONL content (3 lines, using full schema-compliant format)
		jsonlContent := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"u1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_2","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Goodbye!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":200,"output_tokens":100}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 3, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 3)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Request with as_of_line=1 (less than current line count of 3)
		resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics?as_of_line=1", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.AnalyticsResponse
		testutil.ParseJSON(t, resp, &result)

		// Should return analytics for all 3 lines (300 input tokens total)
		if result.Tokens.Input != 300 {
			t.Errorf("expected input tokens 300, got %d", result.Tokens.Input)
		}
		if result.ComputedLines != 3 {
			t.Errorf("expected computed_lines 3, got %d", result.ComputedLines)
		}
	})

	t.Run("returns 400 for invalid as_of_line parameter", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		// Test non-integer
		resp1, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics?as_of_line=abc", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp1.Body.Close()
		testutil.RequireStatus(t, resp1, http.StatusBadRequest)

		// Test negative
		resp2, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics?as_of_line=-5", sessionID))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp2.Body.Close()
		testutil.RequireStatus(t, resp2, http.StatusBadRequest)
	})

	t.Run("response includes computed_at timestamp", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session")

		// Upload JSONL content
		jsonlContent := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
`
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "test-session", "transcript.jsonl", 1, 1, []byte(jsonlContent))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)

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

		// ComputedAt should be set and recent (within the last minute)
		if result.ComputedAt.IsZero() {
			t.Error("expected computed_at to be set")
		}
	})
}

// =============================================================================
// Codex Session Analytics HTTP Integration Tests (CF-364)
//
// Exercises the Codex provider branch in HandleGetSessionAnalytics. The branch
// must mirror codexProvider synchronously: download the rollout via storage,
// parse via codex.ParseRollout, compute via ComputeFromCodexRollout, upsert via
// Cards, respond.
// =============================================================================

// codexExactFixture is a hand-crafted minimal Codex rollout that exercises one
// turn with a known token_count and a single apply_patch creating one file.
//
// Expected (per analytics.ComputeFromCodexRollout's documented mapping):
//   - tokens.input          = 800   (1000 - 200 cached, OpenAI semantics)
//   - tokens.output         = 200   (150 output + 50 reasoning)
//   - tokens.cache_creation = 0     (OpenAI doesn't charge for cache writes)
//   - tokens.cache_read     = 200   (cached portion)
//   - code_activity.files_modified = 1 (one Add File)
//   - code_activity.files_read     = 0 (Codex has no Read tool)
//   - code_activity.lines_added    = 3 (the three +-prefixed lines)
const codexExactFixture = `{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"id":"019e-test-aaaa-bbbb-cccc-dddddddddddd","timestamp":"2026-05-13T01:00:00.000Z","cwd":"/test","originator":"codex-tui","cli_version":"0.130.0","source":"cli","thread_source":"user","model_provider":"openai","model":"gpt-5"}}
{"timestamp":"2026-05-13T01:00:00.100Z","type":"turn_context","payload":{"turn_id":"t1","cwd":"/test","approval_policy":"on-request"}}
{"timestamp":"2026-05-13T01:00:00.200Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1778634000,"model_context_window":258400,"model":"gpt-5"}}
{"timestamp":"2026-05-13T01:00:00.500Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"do something"}]}}
{"timestamp":"2026-05-13T01:00:02.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK."}],"phase":"final"}}
{"timestamp":"2026-05-13T01:00:03.000Z","type":"response_item","payload":{"type":"custom_tool_call","status":"completed","call_id":"c1","name":"apply_patch","input":"*** Begin Patch\n*** Add File: foo.go\n+package foo\n+\n+func Bar() {}\n*** End Patch"}}
{"timestamp":"2026-05-13T01:00:03.100Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"c1","output":"{\"output\":\"Success.\\n\",\"metadata\":{\"exit_code\":0}}"}}
{"timestamp":"2026-05-13T01:00:03.200Z","type":"event_msg","payload":{"type":"patch_apply_end","call_id":"c1","turn_id":"t1","stdout":"Success.\n","stderr":"","success":true,"changes":{"/test/foo.go":{"type":"add","content":"package foo\n\nfunc Bar() {}"}}}}
{"timestamp":"2026-05-13T01:00:04.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":150,"reasoning_output_tokens":50,"total_tokens":1400},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":150,"reasoning_output_tokens":50,"total_tokens":1400},"model_context_window":258400},"rate_limits":{}}}
{"timestamp":"2026-05-13T01:00:05.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","completed_at":1778634005,"duration_ms":5000}}
`

// TestGetSessionAnalytics_Codex_HTTP_Integration_Smoke uploads the canonical
// codex parser fixture and confirms the analytics endpoint dispatches through
// the Codex branch, returning real cards instead of an empty/Claude-parsed
// response.
func TestGetSessionAnalytics_Codex_HTTP_Integration_Smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	rollout, err := os.ReadFile("../codex/testdata/sample_rollout.jsonl")
	if err != nil {
		t.Fatalf("failed to read sample_rollout.jsonl: %v", err)
	}
	lineCount := bytes.Count(rollout, []byte{'\n'})
	if len(rollout) > 0 && rollout[len(rollout)-1] != '\n' {
		lineCount++
	}

	user := testutil.CreateTestUser(t, env, "codex@example.com", "Codex User")
	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
	externalID := "codex-smoke-session"
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, models.ProviderCodex)

	testutil.UploadTestChunk(t, env, user.ID, models.ProviderCodex, externalID, "rollout.jsonl", 1, lineCount, rollout)
	testutil.CreateTestSyncFile(t, env, sessionID, "rollout.jsonl", "transcript", lineCount)

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

	// Tokens populated (OpenAI semantics: uncached portion only).
	if result.Tokens.Input == 0 {
		t.Error("expected non-zero input tokens for codex session")
	}
	if result.Tokens.CacheRead == 0 {
		t.Error("expected non-zero cache_read tokens for codex session (cached portion)")
	}
	if result.Tokens.CacheCreation != 0 {
		t.Errorf("expected cache_creation=0 for codex (OpenAI doesn't charge for writes), got %d", result.Tokens.CacheCreation)
	}

	// Cost is computed via gpt-5 pricing entry.
	if result.Cost.EstimatedUSD.IsZero() {
		t.Error("expected non-zero estimated cost for codex session (gpt-5 pricing should apply)")
	}

	// Cards map should carry the per-card payloads the frontend reads.
	for _, key := range []string{"tokens", "session", "conversation", "code_activity", "tools"} {
		if _, ok := result.Cards[key]; !ok {
			t.Errorf("expected Cards[%q] to be present in codex analytics response", key)
		}
	}

	// agents_and_skills always zero for Codex; the cards layer may emit it
	// with zero counts. The frontend hides it via shouldRender, which we
	// don't assert here — only that the backend produces a stable shape.
}

// TestGetSessionAnalytics_Codex_HTTP_Integration_Exact pins the exact token
// counts the Codex branch must produce for a hand-crafted single-turn rollout
// so future regressions in ComputeFromCodexRollout or the OpenAI-cache subtraction
// surface immediately.
func TestGetSessionAnalytics_Codex_HTTP_Integration_Exact(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	rollout := []byte(codexExactFixture)
	lineCount := bytes.Count(rollout, []byte{'\n'})

	user := testutil.CreateTestUser(t, env, "codex-exact@example.com", "Codex User")
	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
	externalID := "codex-exact-session"
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, models.ProviderCodex)

	testutil.UploadTestChunk(t, env, user.ID, models.ProviderCodex, externalID, "rollout.jsonl", 1, lineCount, rollout)
	testutil.CreateTestSyncFile(t, env, sessionID, "rollout.jsonl", "transcript", lineCount)

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

	// Exact token counts per ComputeFromCodexRollout mapping.
	if result.Tokens.Input != 800 {
		t.Errorf("tokens.input = %d, want 800 (1000 - 200 cached)", result.Tokens.Input)
	}
	if result.Tokens.Output != 200 {
		t.Errorf("tokens.output = %d, want 200 (150 output + 50 reasoning)", result.Tokens.Output)
	}
	if result.Tokens.CacheRead != 200 {
		t.Errorf("tokens.cache_read = %d, want 200", result.Tokens.CacheRead)
	}
	if result.Tokens.CacheCreation != 0 {
		t.Errorf("tokens.cache_creation = %d, want 0 (OpenAI semantics)", result.Tokens.CacheCreation)
	}

	// Code activity: one apply_patch creating one file with 3 added lines.
	codeActivityRaw, ok := result.Cards["code_activity"]
	if !ok {
		t.Fatal("Cards[\"code_activity\"] missing")
	}
	codeActivity, ok := codeActivityRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("Cards[\"code_activity\"] = %T, want map[string]interface{}", codeActivityRaw)
	}
	if got := jsonNumber(codeActivity["files_read"]); got != 0 {
		t.Errorf("code_activity.files_read = %d, want 0 (no Read tool in codex)", got)
	}
	if got := jsonNumber(codeActivity["files_modified"]); got != 1 {
		t.Errorf("code_activity.files_modified = %d, want 1", got)
	}
	if got := jsonNumber(codeActivity["lines_added"]); got != 3 {
		t.Errorf("code_activity.lines_added = %d, want 3", got)
	}
	if got := jsonNumber(codeActivity["lines_removed"]); got != 0 {
		t.Errorf("code_activity.lines_removed = %d, want 0", got)
	}
}

// jsonNumber unwraps a JSON-decoded number (float64 from encoding/json) to int64.
// Returns -1 for missing or non-numeric fields so test errors are obvious.
func jsonNumber(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	default:
		return -1
	}
}
