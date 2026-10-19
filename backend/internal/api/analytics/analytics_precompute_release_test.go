package analytics_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/anthropic"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// 6kbe: a successful on-view compute releases the session's worker quarantine
// for that bucket, so the worker resumes refreshing it.

const releaseTestTranscript = `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`

func quarantineBucket(t *testing.T, env *testutil.TestEnvironment, sessionID, bucket string) {
	t.Helper()
	store := analytics.NewStore(env.DB.Conn())
	for range analytics.QuarantineAfterAttempts {
		if _, _, err := store.RecordFailure(context.Background(), sessionID, bucket, "boom"); err != nil {
			t.Fatalf("RecordFailure: %v", err)
		}
	}
}

func isQuarantined(t *testing.T, env *testutil.TestEnvironment, sessionID, bucket string) bool {
	t.Helper()
	list, err := analytics.NewStore(env.DB.Conn()).ListQuarantined(context.Background())
	if err != nil {
		t.Fatalf("ListQuarantined: %v", err)
	}
	for _, q := range list {
		if q.SessionID == sessionID && q.Bucket == bucket {
			return true
		}
	}
	return false
}

func seedReleaseSession(t *testing.T, env *testutil.TestEnvironment, ext string) (userID int64, sessionID, token string) {
	t.Helper()
	user := testutil.CreateTestUser(t, env, ext+"@test.com", "Release User")
	token = testutil.CreateTestWebSessionWithToken(t, env, user.ID)
	sessionID = testutil.CreateTestSession(t, env, user.ID, ext)
	testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, ext, "transcript.jsonl", 1, 1, []byte(releaseTestTranscript))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)
	return user.ID, sessionID, token
}

func TestGetSessionAnalytics_SuccessfulComputeReleasesRegularCardsQuarantine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	_, sessionID, token := seedReleaseSession(t, env, "release-regular")
	quarantineBucket(t, env, sessionID, analytics.BucketRegularCards)
	quarantineBucket(t, env, sessionID, analytics.BucketSearchIndex)

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(token)
	resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	if isQuarantined(t, env, sessionID, analytics.BucketRegularCards) {
		t.Error("a successful on-view compute must release the regular_cards quarantine")
	}
	if !isQuarantined(t, env, sessionID, analytics.BucketSearchIndex) {
		t.Error("on-view compute must not release buckets it did not compute (search_index)")
	}
}

func TestGetSessionAnalytics_SuccessfulSmartRecapReleasesSmartRecapQuarantine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(anthropic.MessagesResponse{
			ID: "msg_test", Type: "message", Role: "assistant", StopReason: "end_turn",
			Content: []anthropic.ContentBlock{{
				Type: "text",
				Text: `{"suggested_session_title": "T", "recap": "R.", "went_well": [], "went_bad": [], "human_suggestions": [], "environment_suggestions": [], "default_context_suggestions": []}`,
			}},
			Usage: anthropic.Usage{InputTokens: 10, OutputTokens: 5},
		})
	}))
	defer mockServer.Close()

	t.Setenv("SMART_RECAP_ENABLED", "true")
	t.Setenv("SMART_RECAP_MODEL", "test-model")
	t.Setenv("SMART_RECAP_QUOTA_LIMIT", "20")
	t.Setenv("TEST_SMART_RECAP_BASE_URL", mockServer.URL)
	t.Setenv("SMART_RECAP_LLM_PROVIDER", "")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	_, sessionID, token := seedReleaseSession(t, env, "release-recap")
	quarantineBucket(t, env, sessionID, analytics.BucketSmartRecap)

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(token)
	resp, err := client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", sessionID))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	card, err := analytics.NewStore(env.DB.Conn()).GetSmartRecapCard(context.Background(), sessionID)
	if err != nil || card == nil {
		t.Fatalf("smart recap must have been generated on view (card=%v, err=%v)", card, err)
	}
	if isQuarantined(t, env, sessionID, analytics.BucketSmartRecap) {
		t.Error("a successful on-view smart recap generation must release the smart_recap quarantine")
	}
}
