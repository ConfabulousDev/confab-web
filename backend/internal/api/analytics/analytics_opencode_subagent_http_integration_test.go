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

// Spec test for CF-539: OpenCode subagent rollouts uploaded as file_type='agent'
// must aggregate into the parent session's analytics. End-to-end through the
// HTTP analytics endpoint so the registry-driven dispatch + opencodeProvider
// subagent discovery + per-analyzer aggregation are all exercised in one pass.
//
// Phase 3b stub: opencodeProvider.Parse currently only loads file_type='transcript'
// (see opencode_provider.go header comment), so today the response reflects
// main-only data. Once Phase 4 wires subagent discovery + materialize +
// per-card aggregation, the response sums main + subagent files.

// Each line is one {info, parts} JSON object. Main: user prompt + assistant
// reply with 2000 input / 400 output tokens. Provider+model match a Claude
// entry in pricing.json (so per-message authoritative cost wins, but pricing
// is available either way).
const opencodeMainSubagentMain = `{"info":{"id":"msg_main_user","sessionID":"ses_main","role":"user","time":{"created":1717689600000}},"parts":[{"id":"prt_main_u1","type":"text","text":"main user prompt"}]}
{"info":{"id":"msg_main_asst","sessionID":"ses_main","role":"assistant","modelID":"claude-sonnet-4-20250514","providerID":"anthropic","cost":0.001,"tokens":{"input":2000,"output":400,"cache":{"read":0,"write":0}},"time":{"created":1717689601000},"finish":"stop"},"parts":[{"id":"prt_main_a1","type":"text","text":"main reply"}]}
`

// Subagent: independent session id + parentID pointer back to root. Tokens are
// the subagent's own LLM call (verified in the spec's double-counting analysis
// that parent's token totals do NOT include child usage; merging is additive).
const opencodeMainSubagentChild = `{"info":{"id":"msg_sub_user","sessionID":"ses_sub","role":"user","parentID":"msg_main_asst","time":{"created":1717689610000}},"parts":[{"id":"prt_sub_u1","type":"text","text":"sub user prompt"}]}
{"info":{"id":"msg_sub_asst","sessionID":"ses_sub","role":"assistant","parentID":"msg_main_asst","agent":"explore","modelID":"claude-sonnet-4-20250514","providerID":"anthropic","cost":0.0001,"tokens":{"input":500,"output":100,"cache":{"read":0,"write":0}},"time":{"created":1717689611000},"finish":"stop"},"parts":[{"id":"prt_sub_a1","type":"text","text":"sub reply"}]}
`

// TestGetSessionAnalytics_OpencodeSubagent_HTTP_Integration uploads an
// OpenCode session with one main transcript (file_type=transcript) and one
// subagent rollout (file_type=agent), then asserts the analytics endpoint
// returns totals reflecting BOTH files (token sums, conversation counts
// staying main-only per the per-card asymmetry from codex_compute.go).
func TestGetSessionAnalytics_OpencodeSubagent_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("subagent rollout aggregates into parent session analytics", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "opencode-sub@test.com", "OpenCode Sub User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-opencode-subagent"
		sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, models.ProviderOpencode)

		// Main transcript: 2 lines (user + assistant).
		mainBytes := []byte(opencodeMainSubagentMain)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderOpencode, externalID, "transcript-main.jsonl", 1, 2, mainBytes)
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript-main.jsonl", "transcript", 2)

		// Subagent transcript: 2 lines, file_type=agent under SAME session.
		subBytes := []byte(opencodeMainSubagentChild)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderOpencode, externalID, "agent-sub.jsonl", 1, 2, subBytes)
		testutil.CreateTestSyncFile(t, env, sessionID, "agent-sub.jsonl", "agent", 2)

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

		// Tokens: main 2000 input + sub 500 input = 2500 (post-Phase-4).
		// Today: only main reaches the analyzer → 2000.
		if result.Tokens.Input != 2500 {
			t.Errorf("Tokens.Input = %d, want 2500 (main 2000 + subagent 500)", result.Tokens.Input)
		}
		if result.Tokens.Output != 500 {
			t.Errorf("Tokens.Output = %d, want 500 (main 400 + subagent 100)", result.Tokens.Output)
		}
	})

	t.Run("conversation card stays main-only despite subagent file", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "opencode-conv@test.com", "OpenCode Conv User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-opencode-subagent-conv"
		sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, models.ProviderOpencode)

		mainBytes := []byte(opencodeMainSubagentMain)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderOpencode, externalID, "transcript-main.jsonl", 1, 2, mainBytes)
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript-main.jsonl", "transcript", 2)

		subBytes := []byte(opencodeMainSubagentChild)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderOpencode, externalID, "agent-sub.jsonl", 1, 2, subBytes)
		testutil.CreateTestSyncFile(t, env, sessionID, "agent-sub.jsonl", "agent", 2)

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

		// Per-card asymmetry: main rollout drives the Conversation card; the
		// subagent's user/assistant turns do not contribute (subagent reasoning
		// is internal, not user-perceived structure).
		raw, ok := result.Cards["conversation"]
		if !ok {
			t.Fatalf("Cards[\"conversation\"] missing in response: %v", result.Cards)
		}
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
