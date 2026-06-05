package sync_test

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-532: the backend must store and serve workflow subagent transcripts (nested
// path-encoded file_name) and the run journal (file_type=workflow_journal),
// tolerating slashed file_names end-to-end through the sync engine.
func TestWorkflowFiles_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	const agentFile = "subagents/workflows/run-1/agent-wf1.jsonl"
	const journalFile = "subagents/workflows/run-1/journal.jsonl"

	t.Run("accepts workflow_journal file_type and stores it", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "wf@example.com", "WF User")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "wf-journal-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		req := api.SyncChunkRequest{
			SessionID: sessionID,
			FileName:  journalFile,
			FileType:  "workflow_journal",
			FirstLine: 1,
			Lines:     []string{`{"event":"agent_started","runId":"run-1"}`},
		}

		resp, err := client.Post("/api/v1/sync/chunk", req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		// Stored with the workflow_journal file_type.
		var fileType string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT file_type FROM sync_files WHERE session_id = $1 AND file_name = $2",
			sessionID, journalFile)
		if err := row.Scan(&fileType); err != nil {
			t.Fatalf("query sync_files: %v", err)
		}
		if fileType != "workflow_journal" {
			t.Errorf("file_type = %q, want %q", fileType, "workflow_journal")
		}
	})

	t.Run("workflow_journal does not advance session timeline (not Claude-parsed)", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "wf2@example.com", "WF User2")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "wf-journal-noparse")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// A journal line carrying a timestamp must NOT update last_message_at,
		// because only file_type=transcript is parsed for timestamps.
		req := api.SyncChunkRequest{
			SessionID: sessionID,
			FileName:  journalFile,
			FileType:  "workflow_journal",
			FirstLine: 1,
			Lines:     []string{`{"event":"agent_started","timestamp":"2030-01-01T00:00:00Z"}`},
		}

		resp, err := client.Post("/api/v1/sync/chunk", req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		var lastMessageAt *string
		row := env.DB.QueryRow(env.Ctx,
			"SELECT to_char(last_message_at, 'YYYY') FROM sessions WHERE id = $1", sessionID)
		if err := row.Scan(&lastMessageAt); err != nil {
			t.Fatalf("query session: %v", err)
		}
		if lastMessageAt != nil && *lastMessageAt == "2030" {
			t.Error("workflow_journal timestamp must not advance last_message_at (journal is not parsed)")
		}
	})

	t.Run("workflow agent + journal both surface in session files[]", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "wf3@example.com", "WF User3")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Key")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		sessionID := testutil.CreateTestSession(t, env, user.ID, "wf-files-session")

		ts := setupTestServerWithEnv(t, env)
		apiClient := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		for _, f := range []struct {
			name, ftype, line string
		}{
			{agentFile, "agent", `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":1,"output_tokens":1}},"uuid":"aa1"}`},
			{journalFile, "workflow_journal", `{"event":"agent_started"}`},
		} {
			resp, err := apiClient.Post("/api/v1/sync/chunk", api.SyncChunkRequest{
				SessionID: sessionID,
				FileName:  f.name,
				FileType:  f.ftype,
				FirstLine: 1,
				Lines:     []string{f.line},
			})
			if err != nil {
				t.Fatalf("upload %s: %v", f.name, err)
			}
			resp.Body.Close()
			testutil.RequireStatus(t, resp, http.StatusOK)
		}

		webClient := testutil.NewTestClient(t, ts).WithSession(sessionToken)
		resp, err := webClient.Get("/api/v1/sessions/" + sessionID)
		if err != nil {
			t.Fatalf("get session: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		var session db.SessionDetail
		testutil.ParseJSON(t, resp, &session)

		byName := map[string]string{}
		for _, f := range session.Files {
			byName[f.FileName] = f.FileType
		}
		if byName[agentFile] != "agent" {
			t.Errorf("files[] missing nested agent file %q (got type %q)", agentFile, byName[agentFile])
		}
		if byName[journalFile] != "workflow_journal" {
			t.Errorf("files[] missing journal %q with type workflow_journal (got %q)", journalFile, byName[journalFile])
		}
	})

	t.Run("slashed workflow agent file_name is served by the read endpoint", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "wf4@example.com", "WF User4")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Key")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "wf-read-session")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		line := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":1,"output_tokens":1}},"uuid":"aa1"}`
		resp, err := client.Post("/api/v1/sync/chunk", api.SyncChunkRequest{
			SessionID: sessionID,
			FileName:  agentFile,
			FileType:  "agent",
			FirstLine: 1,
			Lines:     []string{line},
		})
		if err != nil {
			t.Fatalf("upload: %v", err)
		}
		resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		readResp, err := client.Get("/api/v1/sessions/" + sessionID + "/sync/file?file_name=" + url.QueryEscape(agentFile))
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		defer readResp.Body.Close()
		testutil.RequireStatus(t, readResp, http.StatusOK)

		buf := make([]byte, 4096)
		n, _ := readResp.Body.Read(buf)
		if !strings.Contains(string(buf[:n]), `"input_tokens":1`) {
			t.Errorf("served slashed-file content missing expected line; got: %s", string(buf[:n]))
		}
	})
}
