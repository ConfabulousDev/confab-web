package analytics_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// Rollout lines below are sanitized from real local Codex rollouts
// (~/.codex/sessions). Chunk 1 of a real rollout opens with session_meta,
// a task_started event, developer + user-role response_items carrying injected
// context, and turn_context — the human prompt only arrives as an event_msg
// (line 7 on <=0.130.0, ~line 10 on >=0.149.1).
const (
	rolloutSessionMeta    = `{"timestamp":"2026-05-18T15:26:15.001Z","type":"session_meta","payload":{"id":"019e3bb1-e75b-7423-b373-f8b8d573b237","cwd":"/home/u/dev/example","originator":"codex_cli_rs","cli_version":"0.130.0"}}`
	rolloutTaskStarted    = `{"timestamp":"2026-05-18T15:26:15.100Z","type":"event_msg","payload":{"type":"task_started","turn_id":"1"}}`
	rolloutDeveloperItem  = `{"timestamp":"2026-05-18T15:26:15.127Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"<permissions instructions>\nFilesystem sandboxing..."}]}}`
	rolloutEnvContextItem = `{"timestamp":"2026-05-18T15:26:15.127Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\n  <cwd>/home/u/dev/example</cwd>\n</environment_context>"}]}}`
	rolloutTurnContext    = `{"timestamp":"2026-05-18T15:26:15.130Z","type":"turn_context","payload":{"cwd":"/home/u/dev/example","model":"gpt-5.5-codex"}}`
	rolloutAgentsMDItem   = `{"timestamp":"2026-05-18T15:26:15.131Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions for /home/u/dev/example\n\n<INSTRUCTIONS>\n## Development\n</INSTRUCTIONS>"}]}}`
	rolloutTokenCount     = `{"timestamp":"2026-05-18T15:26:29.000Z","type":"event_msg","payload":{"type":"token_count","info":null}}`
	oldEraUserMessage     = `{"timestamp":"2026-05-18T15:26:28.570Z","type":"event_msg","payload":{"type":"user_message","message":"add a retry to the uploader","images":[],"local_images":[],"text_elements":[]}}`
	modernEraUserMessage  = `{"timestamp":"2026-09-10T21:44:11.768Z","ordinal":9,"type":"event_msg","payload":{"type":"item_completed","thread_id":"01a08d46-8735-7c00-b558-f23eb1ce6cbc","turn_id":"01a08d47-3d8b-7442-9479-15fa190372fc","item":{"type":"UserMessage","id":"01a08d47-3ef8-75a0-9e73-cb16a9ec747d","content":[{"type":"text","text":"explore this project and review my draft post","text_elements":[]}],"started_at_ms":1789076651768,"completed_at_ms":1789076651768}}}`
	laterChunkUserMessage = `{"timestamp":"2026-05-18T16:00:00.000Z","type":"event_msg","payload":{"type":"user_message","message":"a later prompt that must not become the title"}}`
	codexRolloutFileName  = "rollout-2026-05-18T08-26-15-019e3bb1.jsonl"
)

// codexChunk1 builds a realistic chunk-1 body with the given user-message line
// (or none when userLine is "") in the position real rollouts put it.
func codexChunk1(userLine string) []byte {
	lines := []string{rolloutSessionMeta, rolloutTaskStarted, rolloutDeveloperItem, rolloutEnvContextItem, rolloutTurnContext, rolloutAgentsMDItem}
	if userLine != "" {
		lines = append(lines, userLine)
	}
	lines = append(lines, rolloutTokenCount)
	return []byte(strings.Join(lines, "\n") + "\n")
}

func lineCount(b []byte) int { return strings.Count(string(b), "\n") }

// seedTitleSession inserts a session with the given provider and
// first_user_message (nil = NULL), marked for title recompute when marked is true.
func seedTitleSession(t *testing.T, env *testutil.TestEnvironment, userID int64, provider string, firstUserMessage *string, marked bool) analytics.StaleSession {
	t.Helper()
	sid := uuid.NewString()
	ext := "ext-" + sid[:8]
	var marker any
	if marked {
		marker = time.Now().UTC()
	}
	if _, err := env.DB.Exec(env.Ctx, `
		INSERT INTO sessions (id, user_id, external_id, first_seen, last_message_at, session_type, first_user_message, title_recompute_requested_at)
		VALUES ($1, $2, $3, NOW(), NOW(), $4, $5, $6)
	`, sid, userID, ext, provider, firstUserMessage, marker); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return analytics.StaleSession{SessionID: sid, UserID: userID, ExternalID: ext, Provider: models.NormalizeProvider(provider)}
}

// seedCodexTranscript records the transcript sync_files row and uploads chunk 1.
func seedCodexTranscript(t *testing.T, env *testutil.TestEnvironment, s analytics.StaleSession, chunk1 []byte) {
	t.Helper()
	n := lineCount(chunk1)
	testutil.CreateTestSyncFile(t, env, s.SessionID, codexRolloutFileName, "transcript", n)
	testutil.UploadTestChunk(t, env, s.UserID, models.ProviderCodex, s.ExternalID, codexRolloutFileName, 1, n, chunk1)
}

type titleState struct {
	title  *string
	marked bool
}

func readTitleState(t *testing.T, env *testutil.TestEnvironment, sessionID string) titleState {
	t.Helper()
	var st titleState
	if err := env.DB.QueryRow(env.Ctx,
		`SELECT first_user_message, title_recompute_requested_at IS NOT NULL FROM sessions WHERE id = $1`, sessionID,
	).Scan(&st.title, &st.marked); err != nil {
		t.Fatalf("read title state: %v", err)
	}
	return st
}

func strp(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func titlePrecomputer(env *testutil.TestEnvironment, store *storage.S3Storage) *analytics.Precomputer {
	return analytics.NewPrecomputer(env.DB.Conn(), store, analytics.NewStore(env.DB.Conn()), analytics.PrecomputeConfig{})
}

func TestFindTitleRecomputeSessions_ReturnsOnlyMarkedSessionsWithNormalizedProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "title@test.com", "Title")

	markedCodex := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
	seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, false)
	markedLegacy := seedTitleSession(t, env, user.ID, models.ProviderClaudeCodeLegacy, strp("x"), true)

	got, err := titlePrecomputer(env, env.Storage).FindTitleRecomputeSessions(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindTitleRecomputeSessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2 (marked only): %+v", len(got), got)
	}
	byID := map[string]analytics.StaleSession{}
	for _, s := range got {
		byID[s.SessionID] = s
	}
	if s, ok := byID[markedCodex.SessionID]; !ok || s.Provider != models.ProviderCodex || s.ExternalID != markedCodex.ExternalID || s.UserID != user.ID {
		t.Errorf("marked codex session missing or wrong: %+v", s)
	}
	if s, ok := byID[markedLegacy.SessionID]; !ok || s.Provider != models.ProviderClaudeCode {
		t.Errorf("legacy session provider = %q, want normalized %q", s.Provider, models.ProviderClaudeCode)
	}

	limited, err := titlePrecomputer(env, env.Storage).FindTitleRecomputeSessions(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindTitleRecomputeSessions(limit 1): %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limit 1 returned %d sessions", len(limited))
	}
}

func TestRecomputeSessionTitle_CodexFillsNullTitleFromChunk1(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)

	cases := []struct {
		name     string
		userLine string
		want     string
	}{
		{"<=0.130.0 event_msg.user_message", oldEraUserMessage, "add a retry to the uploader"},
		{">=0.149.1 event_msg.item_completed UserMessage", modernEraUserMessage, "explore this project and review my draft post"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env.CleanDB(t)
			user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
			s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
			chunk1 := codexChunk1(tc.userLine)
			seedCodexTranscript(t, env, s, chunk1)
			// A later chunk carries a different prompt; only chunk 1 may be consulted.
			n := lineCount(chunk1)
			testutil.UploadTestChunk(t, env, s.UserID, models.ProviderCodex, s.ExternalID, codexRolloutFileName, n+1, n+1, []byte(laterChunkUserMessage+"\n"))

			if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
				t.Fatalf("RecomputeSessionTitle: %v", err)
			}
			st := readTitleState(t, env, s.SessionID)
			if st.title == nil || *st.title != tc.want {
				t.Errorf("first_user_message = %v, want %q", st.title, tc.want)
			}
			if st.marked {
				t.Error("marker should be cleared after a successful fill")
			}
		})
	}
}

func TestRecomputeSessionTitle_CodexNotDerivableStaysNullAndClearsMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)

	t.Run("only injected-context response_items", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
		s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
		seedCodexTranscript(t, env, s, codexChunk1(""))

		if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
			t.Fatalf("RecomputeSessionTitle: %v", err)
		}
		st := readTitleState(t, env, s.SessionID)
		if st.title != nil {
			t.Errorf("first_user_message = %q, want NULL (never empty string, never injected context)", *st.title)
		}
		if st.marked {
			t.Error("marker should be cleared when nothing is derivable")
		}
	})

	t.Run("whitespace-only user message", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
		s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
		seedCodexTranscript(t, env, s, codexChunk1(`{"type":"event_msg","payload":{"type":"user_message","message":"   \n  "}}`))

		if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
			t.Fatalf("RecomputeSessionTitle: %v", err)
		}
		if st := readTitleState(t, env, s.SessionID); st.title != nil || st.marked {
			t.Errorf("state = %+v, want NULL title and cleared marker", st)
		}
	})

	t.Run("no transcript sync_files row", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
		s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)

		if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
			t.Fatalf("RecomputeSessionTitle: %v", err)
		}
		if st := readTitleState(t, env, s.SessionID); st.title != nil || st.marked {
			t.Errorf("state = %+v, want NULL title and cleared marker", st)
		}
	})

	t.Run("no chunk starting at line 1", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
		s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
		testutil.CreateTestSyncFile(t, env, s.SessionID, codexRolloutFileName, "transcript", 200)
		testutil.UploadTestChunk(t, env, s.UserID, models.ProviderCodex, s.ExternalID, codexRolloutFileName, 101, 200, []byte(oldEraUserMessage+"\n"))

		if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
			t.Fatalf("RecomputeSessionTitle: %v", err)
		}
		if st := readTitleState(t, env, s.SessionID); st.title != nil || st.marked {
			t.Errorf("state = %+v, want NULL title and cleared marker", st)
		}
	})
}

func TestRecomputeSessionTitle_CodexStorageErrorKeepsMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
	s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
	testutil.CreateTestSyncFile(t, env, s.SessionID, codexRolloutFileName, "transcript", 8)

	// Build a storage client on a scratch bucket, then remove the bucket so every
	// list/download fails the way a transient object-store outage would.
	endpoint, accessKey, secretKey := testutil.MinioCredentials(t, env)
	mc, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	bucket := "title-err-" + uuid.NewString()[:8]
	if err := mc.MakeBucket(env.Ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("make bucket: %v", err)
	}
	broken, err := storage.NewS3Storage(storage.S3Config{Endpoint: endpoint, AccessKeyID: accessKey, SecretAccessKey: secretKey, BucketName: bucket})
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}
	if err := mc.RemoveBucket(env.Ctx, bucket); err != nil {
		t.Fatalf("remove bucket: %v", err)
	}

	if err := titlePrecomputer(env, broken).RecomputeSessionTitle(context.Background(), s); err == nil {
		t.Fatal("expected an error when the object store is unavailable")
	}
	st := readTitleState(t, env, s.SessionID)
	if !st.marked {
		t.Error("marker must be kept on a transient storage failure so the next tick retries")
	}
	if st.title != nil {
		t.Errorf("first_user_message = %q, want NULL", *st.title)
	}
}

func TestRecomputeSessionTitle_CodexAlreadySetIsNotOverwritten(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
	s := seedTitleSession(t, env, user.ID, models.ProviderCodex, strp("client supplied"), true)
	seedCodexTranscript(t, env, s, codexChunk1(oldEraUserMessage))

	if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
		t.Fatalf("RecomputeSessionTitle: %v", err)
	}
	st := readTitleState(t, env, s.SessionID)
	if st.title == nil || *st.title != "client supplied" {
		t.Errorf("first_user_message = %v, want unchanged %q", st.title, "client supplied")
	}
	if st.marked {
		t.Error("marker should be cleared (already_set)")
	}
}

func TestRecomputeSessionTitle_ConcurrentIngestWriteWins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
	s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
	seedCodexTranscript(t, env, s, codexChunk1(oldEraUserMessage))

	restore := analytics.SetTitleRecomputeBeforeWriteHook(func(ctx context.Context, sessionID string) {
		if _, err := env.DB.Exec(ctx, `UPDATE sessions SET first_user_message = 'from ingest' WHERE id = $1`, sessionID); err != nil {
			t.Errorf("simulate ingest: %v", err)
		}
	})
	defer restore()

	if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
		t.Fatalf("RecomputeSessionTitle: %v", err)
	}
	st := readTitleState(t, env, s.SessionID)
	if st.title == nil || *st.title != "from ingest" {
		t.Errorf("first_user_message = %v, want the concurrently ingested value", st.title)
	}
}

func TestRecomputeSessionTitle_ReinvalidationDuringProcessingKeepsMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
	s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
	seedCodexTranscript(t, env, s, codexChunk1(""))

	restore := analytics.SetTitleRecomputeBeforeWriteHook(func(ctx context.Context, sessionID string) {
		if _, err := env.DB.Exec(ctx,
			`UPDATE sessions SET title_recompute_requested_at = title_recompute_requested_at + INTERVAL '1 second' WHERE id = $1`, sessionID); err != nil {
			t.Errorf("simulate re-invalidation: %v", err)
		}
	})
	defer restore()

	if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
		t.Fatalf("RecomputeSessionTitle: %v", err)
	}
	if st := readTitleState(t, env, s.SessionID); !st.marked {
		t.Error("a re-invalidation that landed mid-processing must not be cleared")
	}
}

func TestRecomputeSessionTitle_CodexClampsToByteLimitLikeIngest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
	s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)

	// "é" is two bytes, and the leading "a" puts the byte limit mid-rune.
	long := "a" + strings.Repeat("é", validation.MaxFirstUserMessageLength)
	payload, err := json.Marshal(map[string]any{
		"type":    "event_msg",
		"payload": map[string]any{"type": "user_message", "message": long},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	seedCodexTranscript(t, env, s, codexChunk1(string(payload)))

	if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
		t.Fatalf("RecomputeSessionTitle: %v", err)
	}
	st := readTitleState(t, env, s.SessionID)
	want := validation.TruncateToByteLimit(long, validation.MaxFirstUserMessageLength)
	if st.title == nil || *st.title != want {
		t.Fatalf("stored title (len %d) differs from ingest clamp (len %d)", len(deref(st.title)), len(want))
	}
	if len(*st.title) > validation.MaxFirstUserMessageLength || !utf8.ValidString(*st.title) {
		t.Errorf("stored title is %d bytes / valid UTF-8 = %v", len(*st.title), utf8.ValidString(*st.title))
	}
}

func TestRecomputeSessionTitle_Cursor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)

	cases := []struct {
		name      string
		stored    string
		wantTitle string
	}{
		{"envelope is stripped", "<user_query>\nfix the flaky upload test\n</user_query>", "fix the flaky upload test"},
		{"empty envelope is left as-is", "<user_query>\n</user_query>", "<user_query>\n</user_query>"},
		{"clean value is untouched", "fix the flaky upload test", "fix the flaky upload test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env.CleanDB(t)
			user := testutil.CreateTestUser(t, env, "cursor@test.com", "Cursor")
			s := seedTitleSession(t, env, user.ID, models.ProviderCursor, strp(tc.stored), true)

			if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
				t.Fatalf("RecomputeSessionTitle: %v", err)
			}
			st := readTitleState(t, env, s.SessionID)
			if st.title == nil || *st.title != tc.wantTitle {
				t.Errorf("first_user_message = %v, want %q", st.title, tc.wantTitle)
			}
			if st.marked {
				t.Error("marker should be cleared")
			}
		})
	}
}

func TestRecomputeSessionTitle_UnmarkedOrOtherProviderSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)

	t.Run("unmarked session is a no-op", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
		s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, false)
		seedCodexTranscript(t, env, s, codexChunk1(oldEraUserMessage))

		if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
			t.Fatalf("RecomputeSessionTitle: %v", err)
		}
		if st := readTitleState(t, env, s.SessionID); st.title != nil {
			t.Errorf("unmarked session was modified: %q", *st.title)
		}
	})

	t.Run("marked claude-code session only has its marker cleared", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "claude@test.com", "Claude")
		s := seedTitleSession(t, env, user.ID, models.ProviderClaudeCode, nil, true)

		if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
			t.Fatalf("RecomputeSessionTitle: %v", err)
		}
		if st := readTitleState(t, env, s.SessionID); st.title != nil || st.marked {
			t.Errorf("state = %+v, want NULL title and cleared marker", st)
		}
	})
}

// TestRecomputeSessionTitle_DoesNotModifyStoredObjects pins that the repair is
// read-only against the object store: same keys, same ETags, same mtimes.
func TestRecomputeSessionTitle_DoesNotModifyStoredObjects(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "codex@test.com", "Codex")
	s := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
	chunk1 := codexChunk1(modernEraUserMessage)
	seedCodexTranscript(t, env, s, chunk1)
	n := lineCount(chunk1)
	testutil.UploadTestChunk(t, env, s.UserID, models.ProviderCodex, s.ExternalID, codexRolloutFileName, n+1, n+1, []byte(laterChunkUserMessage+"\n"))

	endpoint, accessKey, secretKey := testutil.MinioCredentials(t, env)
	mc, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	snapshot := func() map[string]string {
		out := map[string]string{}
		for obj := range mc.ListObjects(env.Ctx, "confab-test", minio.ListObjectsOptions{Recursive: true}) {
			if obj.Err != nil {
				t.Fatalf("list objects: %v", obj.Err)
			}
			out[obj.Key] = obj.ETag + "@" + obj.LastModified.String()
		}
		return out
	}
	before := snapshot()
	if len(before) == 0 {
		t.Fatal("expected seeded objects in the bucket")
	}

	if err := titlePrecomputer(env, env.Storage).RecomputeSessionTitle(context.Background(), s); err != nil {
		t.Fatalf("RecomputeSessionTitle: %v", err)
	}
	after := snapshot()
	if len(after) != len(before) {
		t.Fatalf("object count changed: %d → %d", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Errorf("object %s changed: %s → %s", k, v, after[k])
		}
	}
}
