package analytics_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// Spec test for CF-403: Codex subagent rollouts uploaded as file_type='agent'
// must aggregate into the parent session's analytics. End-to-end through the
// HTTP analytics endpoint so the registry-driven dispatch + codexProvider
// subagent discovery + per-analyzer aggregation are all exercised in one pass.
//
// Phase 3b stub: codexProvider.Parse only loads the main transcript file, so
// the response reflects main-only data. Once Phase 4 wires the subagent
// discovery + aggregation, the response sums both files.

const codexMainSubagentMain = `{"timestamp":"2026-05-16T10:00:00Z","type":"session_meta","payload":{"id":"sess","model":"gpt-5","model_provider":"openai"}}
{"timestamp":"2026-05-16T10:00:01Z","type":"event_msg","payload":{"type":"task_started","task_id":"t-main","started_at":"2026-05-16T10:00:01Z","model":"gpt-5"}}
{"timestamp":"2026-05-16T10:00:02Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"main user prompt"}]}}
{"timestamp":"2026-05-16T10:00:03Z","type":"response_item","payload":{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"main reply"}]}}
{"timestamp":"2026-05-16T10:00:04Z","type":"event_msg","payload":{"type":"task_complete","task_id":"t-main","completed_at":"2026-05-16T10:00:04Z","duration_ms":3000}}
{"timestamp":"2026-05-16T10:00:05Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":2000,"cached_input_tokens":0,"output_tokens":400,"reasoning_output_tokens":0,"total_tokens":2400}}}}
`

const codexMainSubagentChild = `{"timestamp":"2026-05-16T10:10:00Z","type":"session_meta","payload":{"id":"child","model":"gpt-5","model_provider":"openai"}}
{"timestamp":"2026-05-16T10:10:01Z","type":"event_msg","payload":{"type":"task_started","task_id":"t-sub","started_at":"2026-05-16T10:10:01Z","model":"gpt-5"}}
{"timestamp":"2026-05-16T10:10:02Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"sub user prompt"}]}}
{"timestamp":"2026-05-16T10:10:03Z","type":"response_item","payload":{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"sub reply"}]}}
{"timestamp":"2026-05-16T10:10:04Z","type":"event_msg","payload":{"type":"task_complete","task_id":"t-sub","completed_at":"2026-05-16T10:10:04Z","duration_ms":3000}}
{"timestamp":"2026-05-16T10:10:05Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":500,"cached_input_tokens":0,"output_tokens":100,"reasoning_output_tokens":0,"total_tokens":600}}}}
`

// TestGetSessionAnalytics_CodexSubagent_HTTP_Integration uploads a Codex
// session with one main rollout (file_type=transcript) and one subagent
// rollout (file_type=agent), then asserts the analytics endpoint returns
// totals reflecting BOTH files (token sums, conversation counts staying main-
// only per the per-card asymmetry).
func TestGetSessionAnalytics_CodexSubagent_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("subagent rollout aggregates into parent session analytics", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "codex-sub@test.com", "Codex Sub User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-codex-subagent"
		sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, models.ProviderCodex)

		// Main rollout: 6 lines
		mainBytes := []byte(codexMainSubagentMain)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderCodex, externalID, "rollout-main.jsonl", 1, 6, mainBytes)
		testutil.CreateTestSyncFile(t, env, sessionID, "rollout-main.jsonl", "transcript", 6)

		// Subagent rollout: 6 lines, file_type=agent under SAME session
		subBytes := []byte(codexMainSubagentChild)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderCodex, externalID, "rollout-sub.jsonl", 1, 6, subBytes)
		testutil.CreateTestSyncFile(t, env, sessionID, "rollout-sub.jsonl", "agent", 6)

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

		// Tokens: main 2000 input + sub 500 input = 2500 (after Phase 4).
		// Today: only main reaches the analyzer → 2000.
		if result.Tokens.Input != 2500 {
			t.Errorf("Tokens.Input = %d, want 2500 (main 2000 + subagent 500)", result.Tokens.Input)
		}
		if result.Tokens.Output != 500 {
			t.Errorf("Tokens.Output = %d, want 500 (main 400 + subagent 100)", result.Tokens.Output)
		}
	})

	t.Run("conversation card excludes subagent turns", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "codex-conv@test.com", "Codex Conv User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-codex-subagent-conv"
		sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, models.ProviderCodex)

		mainBytes := []byte(codexMainSubagentMain)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderCodex, externalID, "rollout-main.jsonl", 1, 6, mainBytes)
		testutil.CreateTestSyncFile(t, env, sessionID, "rollout-main.jsonl", "transcript", 6)

		subBytes := []byte(codexMainSubagentChild)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderCodex, externalID, "rollout-sub.jsonl", 1, 6, subBytes)
		testutil.CreateTestSyncFile(t, env, sessionID, "rollout-sub.jsonl", "agent", 6)

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

		// Conversation card lives under Cards["conversation"]. The Codex
		// per-card asymmetry: main rollout drives this card, subagent turns
		// do not contribute.
		raw, ok := result.Cards["conversation"]
		if !ok {
			t.Fatalf("Cards[\"conversation\"] missing in response: %v", result.Cards)
		}
		// JSON decode round-trips ConversationCardData as map[string]interface{}.
		m, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("conversation card unexpected type: %T", raw)
		}
		gotUserTurns, _ := m["user_turns"].(float64)
		gotAssistantTurns, _ := m["assistant_turns"].(float64)
		if int(gotUserTurns) != 1 {
			t.Errorf("conversation.user_turns = %v, want 1 (subagent excluded from Conversation card)", gotUserTurns)
		}
		if int(gotAssistantTurns) != 1 {
			t.Errorf("conversation.assistant_turns = %v, want 1 (subagent excluded from Conversation card)", gotAssistantTurns)
		}
	})
}
