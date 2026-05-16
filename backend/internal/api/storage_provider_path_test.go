package api

// CF-351 storage-path spec tests.
//
// These exercise the provider-aware chunk-storage signatures against a real
// MinIO bucket via the shared testutil environment. They live in the api
// package (not storage) because that's where the MinIO-backed harness is
// wired up.

import (
	"fmt"
	"slices"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestUploadChunk_CodexPath asserts that an UploadChunk call carrying the
// codex provider lands at a path prefixed with `/codex/`, not `/claude-code/`.
func TestUploadChunk_CodexPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex User")
	const externalID = "codex-ext-1"

	key, err := env.Storage.UploadChunk(env.Ctx, user.ID, models.ProviderCodex, externalID, "transcript.jsonl", 1, 3, []byte("{}\n{}\n{}\n"))
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}

	wantPrefix := fmt.Sprintf("%d/codex/%s/chunks/transcript.jsonl/", user.ID, externalID)
	if key[:len(wantPrefix)] != wantPrefix {
		t.Errorf("upload landed at %q, want prefix %q", key, wantPrefix)
	}
	// Verify the object actually exists at that key.
	testutil.VerifyFileInS3(t, env, key)
}

// TestUploadChunk_ClaudeCodePath is the byte-identical regression for the
// pre-CF-351 hardcoded `/claude-code/` literal. The interpolated value MUST
// match the historical path so existing chunks keep resolving.
func TestUploadChunk_ClaudeCodePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "claude@test.com", "Claude User")
	const externalID = "claude-ext-1"

	key, err := env.Storage.UploadChunk(env.Ctx, user.ID, models.ProviderClaudeCode, externalID, "transcript.jsonl", 1, 1, []byte("{}\n"))
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}

	wantKey := fmt.Sprintf("%d/claude-code/%s/chunks/transcript.jsonl/chunk_00000001_00000001.jsonl", user.ID, externalID)
	if key != wantKey {
		t.Errorf("upload landed at %q, want exact %q", key, wantKey)
	}
}

// TestListChunks_ProviderScoped asserts that chunks uploaded for the same
// (userID, externalID) under two different providers do not bleed into each
// other's ListChunks result.
func TestListChunks_ProviderScoped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "scope@test.com", "Scope User")
	const externalID = "scope-ext-1"

	ccKey, err := env.Storage.UploadChunk(env.Ctx, user.ID, models.ProviderClaudeCode, externalID, "transcript.jsonl", 1, 1, []byte("{}\n"))
	if err != nil {
		t.Fatalf("upload claude-code chunk failed: %v", err)
	}
	cxKey, err := env.Storage.UploadChunk(env.Ctx, user.ID, models.ProviderCodex, externalID, "transcript.jsonl", 1, 1, []byte("{}\n"))
	if err != nil {
		t.Fatalf("upload codex chunk failed: %v", err)
	}

	codexKeys, err := env.Storage.ListChunks(env.Ctx, user.ID, models.ProviderCodex, externalID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("ListChunks codex failed: %v", err)
	}
	if len(codexKeys) != 1 || codexKeys[0] != cxKey {
		t.Errorf("codex listing = %v, want exactly [%q]", codexKeys, cxKey)
	}
	if slices.Contains(codexKeys, ccKey) {
		t.Errorf("codex listing leaked the claude-code key %q", ccKey)
	}

	claudeKeys, err := env.Storage.ListChunks(env.Ctx, user.ID, models.ProviderClaudeCode, externalID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("ListChunks claude-code failed: %v", err)
	}
	if len(claudeKeys) != 1 || claudeKeys[0] != ccKey {
		t.Errorf("claude-code listing = %v, want exactly [%q]", claudeKeys, ccKey)
	}
}

// TestDeleteAllSessionChunks_ProviderScoped asserts that DeleteAllSessionChunks
// only removes chunks for the named provider; chunks under a different
// provider for the same (userID, externalID) are untouched.
func TestDeleteAllSessionChunks_ProviderScoped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "del@test.com", "Delete User")
	const externalID = "del-ext-1"

	ccKey, err := env.Storage.UploadChunk(env.Ctx, user.ID, models.ProviderClaudeCode, externalID, "transcript.jsonl", 1, 1, []byte("{}\n"))
	if err != nil {
		t.Fatalf("upload claude-code chunk failed: %v", err)
	}
	cxKey, err := env.Storage.UploadChunk(env.Ctx, user.ID, models.ProviderCodex, externalID, "transcript.jsonl", 1, 1, []byte("{}\n"))
	if err != nil {
		t.Fatalf("upload codex chunk failed: %v", err)
	}

	if err := env.Storage.DeleteAllSessionChunks(env.Ctx, user.ID, models.ProviderCodex, externalID); err != nil {
		t.Fatalf("DeleteAllSessionChunks codex failed: %v", err)
	}

	// Codex key gone.
	if _, err := env.Storage.Download(env.Ctx, cxKey); err == nil {
		t.Errorf("codex chunk %q still exists after provider-scoped delete", cxKey)
	}
	// Claude-code key survived.
	if _, err := env.Storage.Download(env.Ctx, ccKey); err != nil {
		t.Errorf("claude-code chunk %q was deleted by codex-scoped DeleteAllSessionChunks: %v", ccKey, err)
	}
}

// TestSyncChunk_HTTP_CodexUploadsToProviderPath drives a real HTTP request
// for a codex sync session and asserts the chunk lands under `/codex/` in S3.
func TestSyncChunk_HTTP_CodexUploadsToProviderPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "codex-http@test.com", "Codex HTTP User")
	apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Codex Key")
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

	const externalID = "codex-http-ext-1"
	providerLit := models.ProviderCodex
	initBody := SyncInitRequest{
		ExternalID:     externalID,
		TranscriptPath: "/tmp/codex.jsonl",
		Provider:       &providerLit,
	}
	resp, err := client.Post("/api/v1/sync/init", initBody)
	if err != nil {
		t.Fatalf("init request failed: %v", err)
	}
	testutil.RequireStatus(t, resp, 200)
	var initResp SyncInitResponse
	testutil.ParseJSON(t, resp, &initResp)
	resp.Body.Close()
	if initResp.Provider != models.ProviderCodex {
		t.Fatalf("init response provider = %q, want %q", initResp.Provider, models.ProviderCodex)
	}

	chunkBody := SyncChunkRequest{
		SessionID: initResp.SessionID,
		FileName:  "transcript.jsonl",
		FileType:  "transcript",
		FirstLine: 1,
		Lines:     []string{`{"type":"codex-event","ts":"2026-05-13T00:00:00Z"}`},
	}
	resp, err = client.Post("/api/v1/sync/chunk", chunkBody)
	if err != nil {
		t.Fatalf("chunk request failed: %v", err)
	}
	testutil.RequireStatus(t, resp, 200)
	resp.Body.Close()

	// The chunk must have landed under the codex-scoped prefix, not claude-code.
	codexKey := fmt.Sprintf("%d/codex/%s/chunks/transcript.jsonl/chunk_00000001_00000001.jsonl", user.ID, externalID)
	testutil.VerifyFileInS3(t, env, codexKey)

	// Defensive: assert no chunks landed at the claude-code path for this session.
	claudeKeys, err := env.Storage.ListChunks(env.Ctx, user.ID, models.ProviderClaudeCode, externalID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("ListChunks claude-code failed: %v", err)
	}
	if len(claudeKeys) != 0 {
		t.Errorf("codex session leaked chunks into claude-code path: %v", claudeKeys)
	}
}
