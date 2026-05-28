package sync_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// CF-491 originally — when a sync chunk contained a pr-link transcript line
// and the session's git_info.repo_url extracted to a different owner/repo
// than the PR's owner/repo, the resolver in HandleSyncChunk stamped a
// fork→root mapping onto session_repos.
//
// CF-233 retires that path entirely. The pr_inference heuristic was too
// aggressive: any PR link to a *different* repo (cross-org PR, dependency,
// sibling repo) was treated as upstream evidence, producing misclassified
// root mappings that surfaced as wrong repos under filters in /sessions,
// /org, and /trends. CF-494's git_remote signal is the only authoritative
// fork→upstream source after CF-233; pr_inference no longer runs.

// readRootName returns the (root_name, root_source) stored on session_repos
// for the given repo, or empty strings if the row or columns are NULL.
func readRootName(t *testing.T, env *testutil.TestEnvironment, repoName string) (string, string) {
	t.Helper()
	var root, source sql.NullString
	err := env.DB.Conn().QueryRowContext(env.Ctx,
		`SELECT root_name, root_source FROM session_repos WHERE repo_name = $1`,
		repoName).Scan(&root, &source)
	if err != nil {
		t.Fatalf("read session_repos(%s): %v", repoName, err)
	}
	return root.String, source.String
}

// TestSyncChunk_PRLinkFromFork_DoesNotStamp_CF233 is the inverted CF-491
// test: a chunk whose git_info points to one repo and whose pr-link points
// at a different repo must NOT stamp session_repos.root_name. The
// pr_inference path is retired; only git_remote (CF-494) can collapse forks.
func TestSyncChunk_PRLinkFromFork_DoesNotStamp_CF233(t *testing.T) {
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

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines: []string{
			`{"type":"user","message":"open a PR"}`,
			prLine,
		},
		Metadata: &api.SyncChunkMetadata{
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
	if root != "" {
		t.Errorf("expected NULL root_name after CF-233 (pr_inference retired), got %q", root)
	}
	if source != "" {
		t.Errorf("expected NULL root_source after CF-233, got %q", source)
	}
}

// TestSyncChunk_PRLinkToUnrelatedRepo_DoesNotStamp_CF233 explicitly captures
// the bug pr_inference produced: a PR opened from working-repo MyOrg/my-app
// against a sibling repo MyOrg/some-library (not an upstream of my-app) used
// to stamp my-app → some-library, silently moving my-app's sessions under
// the wrong filter chip. After CF-233 the chunk handler must record nothing.
func TestSyncChunk_PRLinkToUnrelatedRepo_DoesNotStamp_CF233(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "rr-cross@test.com", "RR Cross")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-rr-cross")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	// Working repo: MyOrg/my-app. PR target: MyOrg/some-library (sibling,
	// not an upstream). Pre-CF-233 this would have stamped the misclassification.
	prLine := `{"type":"pr-link","prNumber":42,"prUrl":"https://github.com/MyOrg/some-library/pull/42","prRepository":"MyOrg/some-library","sessionId":"abc","timestamp":"2025-01-01T00:00:00Z"}`

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines: []string{
			`{"type":"user","message":"open a PR against the shared lib"}`,
			prLine,
		},
		Metadata: &api.SyncChunkMetadata{
			GitInfo: json.RawMessage(`{"repo_url":"https://github.com/MyOrg/my-app.git","branch":"main"}`),
		},
	}

	resp, err := client.Post("/api/v1/sync/chunk", reqBody)
	if err != nil {
		t.Fatalf("sync request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	root, _ := readRootName(t, env, "MyOrg/my-app")
	if root != "" {
		t.Errorf("MyOrg/my-app must not be stamped as a fork of MyOrg/some-library — got root_name=%q", root)
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

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines: []string{
			`{"type":"user","message":"open a PR"}`,
			prLine,
		},
		Metadata: &api.SyncChunkMetadata{
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

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines: []string{
			`{"type":"user","message":"no PR here"}`,
			`{"type":"assistant","message":"OK"}`,
		},
		Metadata: &api.SyncChunkMetadata{
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

// ----------------------------------------------------------------------------
// CF-494 — git_remote-signal resolver (primary), with pr_inference as fallback.
// ----------------------------------------------------------------------------

// gitInfoWithRemotes builds a git_info JSON payload with the CF-494 new fields.
func gitInfoWithRemotes(repoURL, trackingRemote string, remotes [][3]string) json.RawMessage {
	type remote struct {
		Name     string `json:"name"`
		FetchURL string `json:"fetch_url"`
		PushURL  string `json:"push_url"`
	}
	type info struct {
		RepoURL        string   `json:"repo_url"`
		Branch         string   `json:"branch"`
		Remotes        []remote `json:"remotes"`
		TrackingRemote string   `json:"tracking_remote"`
	}
	rs := make([]remote, 0, len(remotes))
	for _, r := range remotes {
		rs = append(rs, remote{Name: r[0], FetchURL: r[1], PushURL: r[2]})
	}
	out, _ := json.Marshal(info{
		RepoURL:        repoURL,
		Branch:         "main",
		Remotes:        rs,
		TrackingRemote: trackingRemote,
	})
	return out
}

// TestSyncChunk_GitRemoteFromFork_RecordsRoot — CF-494 acceptance criterion #1.
// Chunk carries new-shape git_info (remotes + tracking_remote) and no pr-link
// rows. After the chunk lands, session_repos.root_name for the fork must equal
// the upstream owner/repo with root_source='git_remote'.
func TestSyncChunk_GitRemoteFromFork_RecordsRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gr-fork@test.com", "GR Fork")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-gr-fork")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines: []string{
			`{"type":"user","message":"hello"}`,
		},
		Metadata: &api.SyncChunkMetadata{
			GitInfo: gitInfoWithRemotes(
				"git@github.com:jackie/confab-web.git",
				"upstream",
				[][3]string{
					{"origin", "git@github.com:jackie/confab-web.git", "git@github.com:jackie/confab-web.git"},
					{"upstream", "https://github.com/ConfabulousDev/confab-web.git", "https://github.com/ConfabulousDev/confab-web.git"},
				},
			),
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
	if source != "git_remote" {
		t.Errorf("expected root_source=git_remote, got %q", source)
	}
}

// TestSyncChunk_OldShape_NoRemotes_DoesNotStamp_CF233 — old-shape payloads
// (no remotes / tracking_remote) used to fall back to pr_inference. After
// CF-233 they record nothing; sessions from pre-CF-494 CLIs stay
// un-collapsed, which is the accurate posture given no upstream signal.
func TestSyncChunk_OldShape_NoRemotes_DoesNotStamp_CF233(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gr-old@test.com", "GR Old")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-gr-old")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	prLine := `{"type":"pr-link","prNumber":1,"prUrl":"https://github.com/ConfabulousDev/confab-web/pull/1","prRepository":"ConfabulousDev/confab-web","sessionId":"abc","timestamp":"2025-01-01T00:00:00Z"}`

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines:     []string{`{"type":"user","message":"open"}`, prLine},
		Metadata: &api.SyncChunkMetadata{
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
	if root != "" {
		t.Errorf("expected NULL root_name for old-shape payload after CF-233, got %q", root)
	}
	if source != "" {
		t.Errorf("expected NULL root_source for old-shape payload after CF-233, got %q", source)
	}
}

// TestSyncChunk_BothSignals_OnlyGitRemoteStamps_CF233 — git_remote and
// pr-link signals both present. After CF-233, only git_remote runs;
// "first-write-wins" is moot because pr_inference no longer fires.
func TestSyncChunk_BothSignals_OnlyGitRemoteStamps_CF233(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gr-both@test.com", "GR Both")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-gr-both")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	prLine := `{"type":"pr-link","prNumber":7,"prUrl":"https://github.com/ConfabulousDev/confab-web/pull/7","prRepository":"ConfabulousDev/confab-web","sessionId":"abc","timestamp":"2025-01-01T00:00:00Z"}`

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines:     []string{`{"type":"user","message":"x"}`, prLine},
		Metadata: &api.SyncChunkMetadata{
			GitInfo: gitInfoWithRemotes(
				"https://github.com/jackie/confab-web.git",
				"upstream",
				[][3]string{
					{"origin", "https://github.com/jackie/confab-web.git", "https://github.com/jackie/confab-web.git"},
					{"upstream", "https://github.com/ConfabulousDev/confab-web.git", "https://github.com/ConfabulousDev/confab-web.git"},
				},
			),
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
		t.Errorf("root = %q", root)
	}
	if source != "git_remote" {
		t.Errorf("after CF-233 pr_inference is gone; git_remote is the only writer, got %q", source)
	}
}

// TestSyncChunk_MalformedTrackingRemote_NoOp — tracking_remote names a remote
// not in the list. Resolver returns false; chunk still succeeds (non-fatal).
func TestSyncChunk_MalformedTrackingRemote_NoOp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gr-mal@test.com", "GR Mal")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-gr-mal")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines:     []string{`{"type":"user","message":"hi"}`},
		Metadata: &api.SyncChunkMetadata{
			GitInfo: gitInfoWithRemotes(
				"https://github.com/jackie/confab-web.git",
				"nonexistent",
				[][3]string{
					{"origin", "https://github.com/jackie/confab-web.git", "https://github.com/jackie/confab-web.git"},
				},
			),
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
		t.Errorf("expected NULL root_name on unknown tracking_remote, got %q", root)
	}
}

// TestSyncChunk_InvalidRemotesEntry_400 — strict per-entry validation:
// empty remote.name → 400.
func TestSyncChunk_InvalidRemotesEntry_400(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gr-bad@test.com", "GR Bad")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-gr-bad")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines:     []string{`{"type":"user","message":"x"}`},
		Metadata: &api.SyncChunkMetadata{
			GitInfo: json.RawMessage(`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"","fetch_url":"https://x.git"}]}`),
		},
	}
	resp, err := client.Post("/api/v1/sync/chunk", reqBody)
	if err != nil {
		t.Fatalf("sync request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusBadRequest)
}

// TestSyncChunk_TooManyRemotes_400 — size cap enforced at validation layer.
func TestSyncChunk_TooManyRemotes_400(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gr-many@test.com", "GR Many")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-gr-many")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	// One more than the cap; names must be unique so the cap check (not the
	// duplicate-name first-match behavior) is what trips the validator.
	remotes := make([][3]string, validation.MaxGitRemotesCount+1)
	for i := range remotes {
		remotes[i] = [3]string{
			fmt.Sprintf("r%d", i),
			"https://x/y.git",
			"https://x/y.git",
		}
	}
	reqBody := api.SyncChunkRequest{
		SessionID: sessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines:     []string{`{"type":"user","message":"x"}`},
		Metadata: &api.SyncChunkMetadata{
			GitInfo: gitInfoWithRemotes("https://github.com/me/repo.git", "", remotes),
		},
	}
	resp, err := client.Post("/api/v1/sync/chunk", reqBody)
	if err != nil {
		t.Fatalf("sync request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusBadRequest)
}

// TestSyncInit_GitRemoteFromFork_RecordsRoot — locks Q3 decision (resolver
// also runs in handleSyncInit). Stamping fires at init time, before any
// chunk lands.
func TestSyncInit_GitRemoteFromFork_RecordsRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gr-init@test.com", "GR Init")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	initBody := api.SyncInitRequest{
		ExternalID:     "ext-gr-init",
		TranscriptPath: "/tmp/t.jsonl",
		Metadata: &api.SyncInitMetadata{
			GitInfo: gitInfoWithRemotes(
				"git@github.com:jackie/confab-web.git",
				"upstream",
				[][3]string{
					{"origin", "git@github.com:jackie/confab-web.git", "git@github.com:jackie/confab-web.git"},
					{"upstream", "https://github.com/ConfabulousDev/confab-web.git", "https://github.com/ConfabulousDev/confab-web.git"},
				},
			),
		},
	}
	resp, err := client.Post("/api/v1/sync/init", initBody)
	if err != nil {
		t.Fatalf("init request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	root, source := readRootName(t, env, "jackie/confab-web")
	if root != "ConfabulousDev/confab-web" {
		t.Errorf("expected init-time stamping, got root=%q", root)
	}
	if source != "git_remote" {
		t.Errorf("expected root_source=git_remote at init, got %q", source)
	}
}
