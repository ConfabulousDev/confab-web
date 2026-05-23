package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-491 — when a sync chunk contains a pr-link transcript line and the
// session's git_info.repo_url extracts to a different owner/repo than the
// PR's owner/repo, the resolver in HandleSyncChunk stamps the fork→root
// mapping onto session_repos.

// readRootName returns the (root_name, root_source) stored on session_repos
// for the given repo, or empty strings if the row or columns are NULL.
func readRootName(t *testing.T, env *testutil.TestEnvironment, repoName string) (string, string) {
	t.Helper()
	var root, source *string
	err := env.DB.Conn().QueryRowContext(env.Ctx,
		`SELECT root_name, root_source FROM session_repos WHERE repo_name = $1`,
		repoName).Scan(&root, &source)
	if err != nil {
		t.Fatalf("read session_repos(%s): %v", repoName, err)
	}
	out := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return out(root), out(source)
}

// TestSyncChunk_PRLinkFromFork_RecordsRoot is the happy path: chunk has
// git_info pointing to a fork, plus a pr-link line pointing to the upstream.
// After the chunk lands, session_repos.root_name for the fork must equal the
// upstream owner/repo.
func TestSyncChunk_PRLinkFromFork_RecordsRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "rr-fork@test.com", "RR Fork")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-rr-fork")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	prLine := `{"type":"pr-link","prNumber":1,"prUrl":"https://github.com/ConfabulousDev/confab-web/pull/1","prRepository":"ConfabulousDev/confab-web","sessionId":"abc","timestamp":"2025-01-01T00:00:00Z"}`

	reqBody := SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines: []string{
			`{"type":"user","message":"open a PR"}`,
			prLine,
		},
		Metadata: &SyncChunkMetadata{
			GitInfo: json.RawMessage(`{"repo_url":"https://github.com/jackie/confab-web.git","branch":"main"}`),
		},
	}

	resp, err := client.Post("/api/v1/sync/chunk", reqBody)
	if err != nil {
		t.Fatalf("sync request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	root, source := readRootName(t, env, "jackie/confab-web")
	if root != "ConfabulousDev/confab-web" {
		t.Errorf("expected jackie/confab-web -> ConfabulousDev/confab-web, got %q", root)
	}
	if source != "pr_inference" {
		t.Errorf("expected root_source=pr_inference, got %q", source)
	}
}

// TestSyncChunk_PRLinkFromUpstream_NoOp verifies that when the chunk's
// git_info points to the same repo as the PR, no mapping is written
// (self-loop is not a fork→root observation).
func TestSyncChunk_PRLinkFromUpstream_NoOp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "rr-up@test.com", "RR Up")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-rr-up")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	prLine := `{"type":"pr-link","prNumber":2,"prUrl":"https://github.com/ConfabulousDev/confab-web/pull/2","prRepository":"ConfabulousDev/confab-web","sessionId":"abc","timestamp":"2025-01-01T00:00:00Z"}`

	reqBody := SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines: []string{
			`{"type":"user","message":"open a PR"}`,
			prLine,
		},
		Metadata: &SyncChunkMetadata{
			GitInfo: json.RawMessage(`{"repo_url":"https://github.com/ConfabulousDev/confab-web.git","branch":"main"}`),
		},
	}

	resp, err := client.Post("/api/v1/sync/chunk", reqBody)
	if err != nil {
		t.Fatalf("sync request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	root, _ := readRootName(t, env, "ConfabulousDev/confab-web")
	if root != "" {
		t.Errorf("expected NULL root_name for self-loop, got %q", root)
	}
}

// TestSyncChunk_CommitLink_NoOp verifies that commit links never trigger the
// resolver. Commits can be cherry-picked across forks and don't reliably
// identify the upstream.
//
// Note: the production sync path only extracts pr-link rows from transcript
// JSONL (see extractPRLinkFromLine). Commit links arrive via the manual API
// path (HandleCreateGitHubLink), which does not invoke the resolver at all.
// This test therefore demonstrates the negative case by sending a transcript
// chunk that contains *no* pr-link rows but a session that already has a
// commit link recorded — the post-sync state should still have NULL
// root_name even though a commit link exists.
func TestSyncChunk_CommitLink_NoOp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "rr-commit@test.com", "RR Commit")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-rr-commit")

	// Pre-seed a commit link pointing to a different owner/repo (e.g. someone
	// cherry-picked from upstream). Resolver should not infer fork→root from
	// this signal.
	testutil.CreateTestGitHubLink(t, env, sessionID, "commit", "deadbeef")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	reqBody := SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines: []string{
			`{"type":"user","message":"no PR here"}`,
			`{"type":"assistant","message":"OK"}`,
		},
		Metadata: &SyncChunkMetadata{
			GitInfo: json.RawMessage(`{"repo_url":"https://github.com/jackie/confab-web.git","branch":"main"}`),
		},
	}

	resp, err := client.Post("/api/v1/sync/chunk", reqBody)
	if err != nil {
		t.Fatalf("sync request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	root, _ := readRootName(t, env, "jackie/confab-web")
	if root != "" {
		t.Errorf("expected NULL root_name when only a commit link exists, got %q", root)
	}
}
