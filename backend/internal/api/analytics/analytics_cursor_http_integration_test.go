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

// Spec test for gevp: a Cursor session uploaded through the sync protocol must
// be precomputable end-to-end and its analytics served from the HTTP endpoint.
// This exercises the registry-driven dispatch (ProviderFor -> cursorProvider),
// the Cursor JSONL parser, and the consolidated card compute in one pass.
//
// The uploaded body mirrors the fy5q fixture's wire shape: {role, message.
// content[]} conversation rows plus turn_ended markers, with NO per-line
// timestamps, tokens, or model fields.
const cursorAnalyticsTranscript = `{"role":"user","message":{"content":[{"type":"text","text":"Add input validation to the session handler and write a test for it."}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"Reading the handler and searching for validation helpers."},{"type":"tool_use","name":"Read","input":{"path":"internal/api/session_handler.go"}},{"type":"tool_use","name":"Grep","input":{"pattern":"func validate","path":"internal/api"}}]}}
{"type":"turn_ended","status":"success"}
{"role":"user","message":{"content":[{"type":"text","text":"Make the edit and add the test."}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"Editing the handler and creating the test."},{"type":"tool_use","name":"StrReplace","input":{"path":"internal/api/session_handler.go","old_string":"a","new_string":"b"}},{"type":"tool_use","name":"Write","input":{"path":"internal/api/session_handler_test.go","contents":"package api\n"}}]}}
{"type":"turn_ended","status":"error","error":"You've hit your usage limit."}
`

// TestGetSessionAnalytics_Cursor_HTTP_Integration uploads a Cursor session
// (file_type=transcript) and asserts the analytics endpoint returns a valid
// card payload with empty token/cost data (no usage in Cursor JSONL) and
// non-trivial tool/code-activity stats derived from the message structure.
func TestGetSessionAnalytics_Cursor_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("cursor session precomputes and serves analytics with empty tokens", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "cursor-analytics@test.com", "Cursor User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		externalID := "test-cursor-analytics"
		sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, models.ProviderCursor)

		body := []byte(cursorAnalyticsTranscript)
		testutil.UploadTestChunk(t, env, user.ID, models.ProviderCursor, externalID, "transcript.jsonl", 1, 6, body)
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 6)

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

		// Tokens always zero — Cursor JSONL carries no usage data. (The
		// tokens_v2 card record is always written to storage, but the API omits
		// it from the Cards map when it has no provider data, so we assert the
		// zero token summary rather than the card's presence.)
		if result.Tokens.Input != 0 || result.Tokens.Output != 0 {
			t.Errorf("expected zero tokens, got input=%d output=%d", result.Tokens.Input, result.Tokens.Output)
		}

		// Code activity card must reflect the structure-derived counts.
		raw, ok := result.Cards["code_activity"]
		if !ok {
			t.Fatalf("code_activity card missing: %v", result.Cards)
		}
		m, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("code_activity card unexpected type: %T", raw)
		}
		filesRead, _ := m["files_read"].(float64)
		if int(filesRead) != 1 {
			t.Errorf("code_activity.files_read = %v, want 1 (Read only)", filesRead)
		}
	})
}
