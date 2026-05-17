package api

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestSanitizeContentDispositionFilename pins the CF-425 behavior that the
// download endpoint emits a safe filename in the Content-Disposition header —
// stripping characters that could break header syntax (CR/LF/quotes) or be
// misinterpreted as a path by client tools (/, \, ..).
func TestSanitizeContentDispositionFilename(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"transcript.jsonl", "transcript.jsonl"},
		{"file with spaces.jsonl", "file_with_spaces.jsonl"},
		{"../../etc/passwd", ".._.._etc_passwd"},
		{"name\r\nInjected: yes", "name__Injected__yes"},
		{`evil";X-Injected: 1`, "evil__X-Injected__1"},
		{"", "download.txt"},
		{"中文.txt", "__.txt"},
	}
	for _, tc := range cases {
		got := sanitizeContentDispositionFilename(tc.in)
		if got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// =============================================================================
// Condensed Transcript API — HTTP Integration Tests
//
// Tests the external condensed transcript endpoint through the full HTTP stack
// including authentication (API key), rate limiting, canonical access model,
// and transcript download from S3.
// =============================================================================

func TestCondensedTranscript_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("owner can fetch own session transcript by UUID", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-123", testutil.TestSessionFullOpts{
			Summary:          "Test session summary",
			FirstUserMessage: "Hello",
			RepoURL:          "https://github.com/org/repo.git",
			Branch:           "main",
		})

		// Upload a valid transcript to S3
		transcript := validTestTranscript()
		testutil.UploadTestTranscript(t, env, user.ID, models.ProviderClaudeCode, "ext-123", "transcript.jsonl", transcript)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result CondensedTranscriptResponse
		testutil.ParseJSON(t, resp, &result)

		// Verify metadata
		if result.Metadata.SessionID != sessionID {
			t.Errorf("expected session_id %q, got %q", sessionID, result.Metadata.SessionID)
		}
		if result.Metadata.ExternalID != "ext-123" {
			t.Errorf("expected external_id %q, got %q", "ext-123", result.Metadata.ExternalID)
		}
		if result.Metadata.Title != "Test session summary" {
			t.Errorf("expected title %q, got %q", "Test session summary", result.Metadata.Title)
		}
		if result.Metadata.Repo == nil || *result.Metadata.Repo != "org/repo" {
			t.Errorf("expected repo %q, got %v", "org/repo", result.Metadata.Repo)
		}
		if result.Metadata.Branch == nil || *result.Metadata.Branch != "main" {
			t.Errorf("expected branch %q, got %v", "main", result.Metadata.Branch)
		}

		// Verify transcript is non-empty XML
		if result.Transcript == "" {
			t.Error("expected non-empty transcript")
		}
		if !strings.Contains(result.Transcript, "<transcript>") {
			t.Error("expected transcript to contain <transcript> tag")
		}
	})

	t.Run("no API key returns 401", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-123", testutil.TestSessionFullOpts{
			Summary: "Test session",
		})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // no API key

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("non-owner without share gets 404", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@example.com", "Other")
		otherKey := testutil.CreateTestAPIKeyWithToken(t, env, other.ID, "Other Key")

		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-123", testutil.TestSessionFullOpts{
			Summary: "Owner's session",
		})

		transcript := validTestTranscript()
		testutil.UploadTestTranscript(t, env, owner.ID, models.ProviderClaudeCode, "ext-123", "transcript.jsonl", transcript)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(otherKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("recipient share grants access", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
		recipientKey := testutil.CreateTestAPIKeyWithToken(t, env, recipient.ID, "Recipient Key")

		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-123", testutil.TestSessionFullOpts{
			Summary: "Shared session",
		})

		transcript := validTestTranscript()
		testutil.UploadTestTranscript(t, env, owner.ID, models.ProviderClaudeCode, "ext-123", "transcript.jsonl", transcript)

		// Share with recipient
		testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"recipient@example.com"})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(recipientKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result CondensedTranscriptResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Transcript == "" {
			t.Error("expected non-empty transcript for shared session")
		}
	})

	t.Run("max_chars truncates transcript from beginning", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-123", testutil.TestSessionFullOpts{
			Summary: "Truncation test",
		})

		testutil.UploadTestTranscript(t, env, user.ID, models.ProviderClaudeCode, "ext-123", "transcript.jsonl", longTestTranscript())

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// First get full transcript to know its length
		fullResp, err := client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		var fullResult CondensedTranscriptResponse
		testutil.ParseJSON(t, fullResp, &fullResult)

		fullLen := len(fullResult.Transcript)
		if fullLen < 100 {
			t.Fatalf("transcript too short to test truncation: %d chars", fullLen)
		}

		// Now request with max_chars smaller than full
		maxChars := fullLen / 2
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript?max_chars=" + strconv.Itoa(maxChars))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		testutil.RequireStatus(t, resp, http.StatusOK)

		var truncResult CondensedTranscriptResponse
		testutil.ParseJSON(t, resp, &truncResult)

		if len(truncResult.Transcript) >= fullLen {
			t.Errorf("expected truncated transcript to be shorter than %d, got %d", fullLen, len(truncResult.Transcript))
		}
		if !strings.Contains(truncResult.Transcript, "[Transcript truncated") {
			t.Error("expected truncation header in transcript")
		}
	})

	t.Run("max_chars invalid value returns 400", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-123", testutil.TestSessionFullOpts{
			Summary: "Test session",
		})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Non-numeric
		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript?max_chars=abc")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()

		// Zero
		resp, err = client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript?max_chars=0")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()

		// Negative
		resp, err = client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript?max_chars=-1")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()
	})

	t.Run("session with no transcript returns 404", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		// Create session with SyncLines: -1 so no sync file is created
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-no-transcript")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/condensed-transcript")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Session exists but has no sync files / transcript → 404
		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("nonexistent session UUID returns 404", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/00000000-0000-0000-0000-000000000000/condensed-transcript")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})
}

// =============================================================================
// Session Files API — HTTP Integration Tests
//
// Tests the file list and download endpoints through the full HTTP stack
// including API key authentication, canonical access model, and S3 download.
// =============================================================================

func TestSessionFiles_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("list files by UUID returns transcript and agent files", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-files-1", testutil.TestSessionFullOpts{
			Summary: "Files test session",
		})

		// Add agent sync files (transcript already created by CreateTestSessionFull)
		testutil.CreateTestSyncFile(t, env, sessionID, "agent-abc.jsonl", "agent", 50)
		testutil.CreateTestSyncFile(t, env, sessionID, "agent-def.jsonl", "agent", 30)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SessionFilesResponse
		testutil.ParseJSON(t, resp, &result)

		if len(result.Files) != 3 {
			t.Fatalf("expected 3 files, got %d", len(result.Files))
		}

		// Verify file types present
		fileTypes := map[string]int{}
		for _, f := range result.Files {
			fileTypes[f.FileType]++
		}
		if fileTypes["transcript"] != 1 {
			t.Errorf("expected 1 transcript file, got %d", fileTypes["transcript"])
		}
		if fileTypes["agent"] != 2 {
			t.Errorf("expected 2 agent files, got %d", fileTypes["agent"])
		}
	})

	t.Run("list files returns empty array for session with no sync files", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		// SyncLines: -1 means no sync file created
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-no-files", testutil.TestSessionFullOpts{
			Summary:   "No files session",
			SyncLines: -1,
		})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SessionFilesResponse
		testutil.ParseJSON(t, resp, &result)

		if len(result.Files) != 0 {
			t.Errorf("expected 0 files, got %d", len(result.Files))
		}
	})

	t.Run("non-owner without share gets 404", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@example.com", "Other")
		otherKey := testutil.CreateTestAPIKeyWithToken(t, env, other.ID, "Other Key")

		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-private", testutil.TestSessionFullOpts{
			Summary: "Private session",
		})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(otherKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("recipient share grants file list access", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
		recipientKey := testutil.CreateTestAPIKeyWithToken(t, env, recipient.ID, "Recipient Key")

		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-shared", testutil.TestSessionFullOpts{
			Summary: "Shared session",
		})

		testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"recipient@example.com"})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(recipientKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result SessionFilesResponse
		testutil.ParseJSON(t, resp, &result)

		if len(result.Files) != 1 {
			t.Errorf("expected 1 file, got %d", len(result.Files))
		}
	})

	t.Run("no API key returns 401", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-noauth", testutil.TestSessionFullOpts{
			Summary: "Test session",
		})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // no API key

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("nonexistent session UUID returns 404", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/00000000-0000-0000-0000-000000000000/files")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})
}

func TestSessionFileDownload_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("download file by UUID returns raw JSONL content", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-dl-1", testutil.TestSessionFullOpts{
			Summary: "Download test",
		})

		transcript := validTestTranscript()
		testutil.UploadTestTranscript(t, env, user.ID, models.ProviderClaudeCode, "ext-dl-1", "transcript.jsonl", transcript)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files/download?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Errorf("expected Content-Type text/plain; charset=utf-8, got %q", ct)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}

		if len(body) == 0 {
			t.Error("expected non-empty response body")
		}
		// Verify it contains JSONL content
		if !strings.Contains(string(body), `"type"`) {
			t.Error("expected JSONL content with type field")
		}
	})

	t.Run("download unknown file returns 404", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-dl-3", testutil.TestSessionFullOpts{
			Summary: "Unknown file test",
		})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files/download?file_name=nonexistent.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("download missing file_name param returns 400", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-dl-4", testutil.TestSessionFullOpts{
			Summary: "Missing param test",
		})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files/download")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("non-owner without share cannot download", func(t *testing.T) {
		env.CleanDB(t)

		owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		other := testutil.CreateTestUser(t, env, "other@example.com", "Other")
		otherKey := testutil.CreateTestAPIKeyWithToken(t, env, other.ID, "Other Key")

		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-dl-5", testutil.TestSessionFullOpts{
			Summary: "Private download",
		})

		transcript := validTestTranscript()
		testutil.UploadTestTranscript(t, env, owner.ID, models.ProviderClaudeCode, "ext-dl-5", "transcript.jsonl", transcript)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(otherKey.RawToken)

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files/download?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("no API key returns 401", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-dl-6", testutil.TestSessionFullOpts{
			Summary: "No auth download",
		})

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // no API key

		resp, err := client.Get("/api/v1/sessions/" + sessionID + "/files/download?file_name=transcript.jsonl")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}

// =============================================================================
// Test transcript helpers
// =============================================================================

// validTestTranscript returns a minimal valid JSONL transcript that passes full validation.
// Includes all required fields: parentUuid, isSidechain, userType, cwd, sessionId, version,
// and for assistant messages: message.model, message.id, message.type, message.stop_sequence.
func validTestTranscript() []byte {
	return []byte(`{"type":"user","message":{"role":"user","content":"Hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_01","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Hi there! How can I help you today?"}],"usage":{"input_tokens":10,"output_tokens":8},"stop_reason":"end_turn","stop_sequence":null},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"u1","isSidechain":false}
`)
}

// longTestTranscript returns a multi-exchange transcript for truncation testing.
func longTestTranscript() []byte {
	return []byte(`{"type":"user","message":{"role":"user","content":"First question: what is the meaning of life, the universe, and everything?"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_01","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"The answer to the ultimate question of life, the universe, and everything is 42. This comes from Douglas Adams' Hitchhiker's Guide to the Galaxy."}],"usage":{"input_tokens":20,"output_tokens":30},"stop_reason":"end_turn","stop_sequence":null},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"u1","isSidechain":false}
{"type":"user","message":{"role":"user","content":"Second question: can you explain quantum mechanics in simple terms for a beginner who has never studied physics?"},"uuid":"u2","timestamp":"2025-01-01T00:00:02Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_02","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Quantum mechanics describes the behavior of particles at the smallest scales. Unlike classical physics, particles can exist in multiple states simultaneously until observed. This is called superposition."}],"usage":{"input_tokens":30,"output_tokens":40},"stop_reason":"end_turn","stop_sequence":null},"uuid":"a2","timestamp":"2025-01-01T00:00:03Z","parentUuid":"u2","isSidechain":false}
{"type":"user","message":{"role":"user","content":"Third question: what are the best practices for writing clean, maintainable code in Go?"},"uuid":"u3","timestamp":"2025-01-01T00:00:04Z","parentUuid":"a2","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_03","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Key Go best practices include: use short variable names in small scopes, handle errors explicitly, prefer composition over inheritance, write table-driven tests, and keep interfaces small."}],"usage":{"input_tokens":25,"output_tokens":35},"stop_reason":"end_turn","stop_sequence":null},"uuid":"a3","timestamp":"2025-01-01T00:00:05Z","parentUuid":"u3","isSidechain":false}
`)
}

// =============================================================================
// Unit tests for helper functions
// =============================================================================

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/org/repo.git", "org/repo"},
		{"https://github.com/org/repo", "org/repo"},
		// SSH URLs use colon separator — extractRepoName splits on "/" only
		{"git@github.com:org/repo.git", "git@github.com:org/repo"},
		{"repo", "repo"},
	}

	for _, tc := range tests {
		result := extractRepoName(tc.input)
		if result != tc.expected {
			t.Errorf("extractRepoName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTruncateTranscriptFromStart(t *testing.T) {
	// Build a sample transcript
	transcript := `<transcript>
<user id="1">Hello world</user>
<assistant id="2">Hi there! How can I help?</assistant>
<user id="3">What is 2+2?</user>
<assistant id="4">The answer is 4.</assistant>
</transcript>`

	t.Run("no truncation when under limit", func(t *testing.T) {
		result := truncateTranscriptFromStart(transcript, len(transcript)+100)
		if result != transcript {
			t.Error("expected no truncation")
		}
	})

	t.Run("truncates from beginning preserving element boundaries", func(t *testing.T) {
		// Request only ~100 chars — should find a clean element boundary
		result := truncateTranscriptFromStart(transcript, 100)
		if !strings.Contains(result, "[Transcript truncated") {
			t.Error("expected truncation header")
		}
		// Should start at an element boundary
		if !strings.Contains(result, "<user ") && !strings.Contains(result, "<assistant ") {
			t.Error("expected result to contain a complete element start")
		}
	})

	t.Run("exact length returns unchanged", func(t *testing.T) {
		result := truncateTranscriptFromStart(transcript, len(transcript))
		if result != transcript {
			t.Error("expected no truncation at exact length")
		}
	})
}
