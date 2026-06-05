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

// Spec test for CF-532: a Claude session whose workflow subagent transcripts
// are uploaded under the nested, path-encoded file_name
// "subagents/workflows/<runId>/agent-<id>.jsonl" (file_type=agent) must have
// those tokens attributed in the Tokens card — with NO double counting, and the
// run journal (file_type=workflow_journal) excluded from token compute.
//
// End-to-end through the HTTP analytics endpoint so the fixed ExtractAgentID
// (path.Base before the agent- prefix check) is exercised inside
// downloadClaudeMainAndListAgents.

// Main transcript: assistant spends 100/50, then launches a workflow whose
// tool_result is async_launched — it carries NO agentId and NO usage, so the
// Finalize fallback never fires for it (this is what prevents double counting).
const workflowMainTranscript = `{"type":"user","message":{"role":"user","content":"run a workflow"},"uuid":"u1","timestamp":"2026-06-05T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"tool_use","id":"toolu_wf1","name":"Workflow","input":{"script":"..."}}],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2026-06-05T00:00:01Z","parentUuid":"u1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_wf1","content":"launched"}]},"toolUseResult":{"status":"async_launched"},"uuid":"u2","timestamp":"2026-06-05T00:00:02Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`

// Workflow subagent transcript: one assistant message spending 700/300.
const workflowAgentTranscript = `{"type":"assistant","message":{"id":"msg_a","type":"message","model":"claude-haiku-3","role":"assistant","content":[{"type":"text","text":"subagent work"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":700,"output_tokens":300}},"uuid":"aa1","timestamp":"2026-06-05T00:00:03Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"child","version":"1.0"}
`

// Run journal: an append-only event log, NOT a transcript. Must never reach the
// token analyzer.
const workflowJournal = `{"event":"agent_started","runId":"run-1","agentId":"wf1","ts":"2026-06-05T00:00:02Z"}
{"event":"agent_completed","runId":"run-1","agentId":"wf1","ts":"2026-06-05T00:00:04Z"}
`

func TestGetSessionAnalytics_WorkflowSubagent_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("nested workflow agent tokens attributed with no double count", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "wf@test.com", "Workflow User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-workflow-subagent"
		sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, models.ProviderClaudeCode)

		// Main transcript: 3 lines.
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, externalID, "transcript.jsonl", 1, 3, []byte(workflowMainTranscript))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 3)

		// Workflow subagent transcript under the nested, path-encoded name.
		const agentFile = "subagents/workflows/run-1/agent-wf1.jsonl"
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, externalID, agentFile, 1, 1, []byte(workflowAgentTranscript))
		testutil.CreateTestSyncFile(t, env, sessionID, agentFile, "agent", 1)

		// Run journal (file_type=workflow_journal) — must be ignored by compute.
		const journalFile = "subagents/workflows/run-1/journal.jsonl"
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, externalID, journalFile, 1, 2, []byte(workflowJournal))
		testutil.CreateTestSyncFile(t, env, sessionID, journalFile, "workflow_journal", 2)

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

		// Main 100 + subagent 700 = 800 input; 50 + 300 = 350 output.
		// The journal contributes nothing; the async_launched main result is not
		// double-counted on top of the agent file.
		if result.Tokens.Input != 800 {
			t.Errorf("Tokens.Input = %d, want 800 (main 100 + workflow subagent 700, journal excluded, no double count)", result.Tokens.Input)
		}
		if result.Tokens.Output != 350 {
			t.Errorf("Tokens.Output = %d, want 350 (main 50 + workflow subagent 300)", result.Tokens.Output)
		}
	})
}
