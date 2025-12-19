package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// UpdateSyncFileState Tests
// =============================================================================

// TestUpdateSyncFileState_BasicUpsert tests basic file sync state creation and update
func TestUpdateSyncFileState_BasicUpsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "sync@test.com", "Sync User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-id")

	ctx := context.Background()

	// Create new sync file state
	err := env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (create) failed: %v", err)
	}

	// Verify state was created
	state, err := env.DB.GetSyncFileState(ctx, sessionID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("GetSyncFileState failed: %v", err)
	}
	if state.LastSyncedLine != 100 {
		t.Errorf("LastSyncedLine = %d, want 100", state.LastSyncedLine)
	}
	if state.FileType != "transcript" {
		t.Errorf("FileType = %s, want transcript", state.FileType)
	}

	// Update existing sync file state
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 200, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (update) failed: %v", err)
	}

	// Verify state was updated
	state, err = env.DB.GetSyncFileState(ctx, sessionID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("GetSyncFileState failed: %v", err)
	}
	if state.LastSyncedLine != 200 {
		t.Errorf("LastSyncedLine = %d, want 200", state.LastSyncedLine)
	}
}

// TestUpdateSyncFileState_ChunkCountIncrement tests that chunk_count increments on each call
func TestUpdateSyncFileState_ChunkCountIncrement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "chunk@test.com", "Chunk User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "chunk-session")

	ctx := context.Background()

	// First chunk upload - chunk_count should be 1
	err := env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (1) failed: %v", err)
	}

	state, err := env.DB.GetSyncFileState(ctx, sessionID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("GetSyncFileState failed: %v", err)
	}
	if state.ChunkCount == nil || *state.ChunkCount != 1 {
		t.Errorf("ChunkCount after first upload = %v, want 1", state.ChunkCount)
	}

	// Second chunk upload - chunk_count should be 2
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 200, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (2) failed: %v", err)
	}

	state, err = env.DB.GetSyncFileState(ctx, sessionID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("GetSyncFileState failed: %v", err)
	}
	if state.ChunkCount == nil || *state.ChunkCount != 2 {
		t.Errorf("ChunkCount after second upload = %v, want 2", state.ChunkCount)
	}

	// Third chunk upload - chunk_count should be 3
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 300, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (3) failed: %v", err)
	}

	state, err = env.DB.GetSyncFileState(ctx, sessionID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("GetSyncFileState failed: %v", err)
	}
	if state.ChunkCount == nil || *state.ChunkCount != 3 {
		t.Errorf("ChunkCount after third upload = %v, want 3", state.ChunkCount)
	}
}

// TestUpdateSyncFileState_LastMessageAt tests conditional last_message_at update
func TestUpdateSyncFileState_LastMessageAt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "msgtime@test.com", "MsgTime User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "msgtime-session")

	ctx := context.Background()

	// Verify last_message_at starts as NULL
	var initialLastMsgAt *time.Time
	err := env.DB.QueryRow(ctx, "SELECT last_message_at FROM sessions WHERE id = $1", sessionID).Scan(&initialLastMsgAt)
	if err != nil {
		t.Fatalf("query initial last_message_at failed: %v", err)
	}
	if initialLastMsgAt != nil {
		t.Error("last_message_at should initially be NULL")
	}

	// Set initial last_message_at (from NULL)
	initialTime := time.Now().Add(-time.Hour).UTC()
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, &initialTime, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (initial) failed: %v", err)
	}

	// Verify last_message_at was set from NULL
	var lastMsgAtAfterFirst *time.Time
	err = env.DB.QueryRow(ctx, "SELECT last_message_at FROM sessions WHERE id = $1", sessionID).Scan(&lastMsgAtAfterFirst)
	if err != nil {
		t.Fatalf("query last_message_at after first update failed: %v", err)
	}
	if lastMsgAtAfterFirst == nil {
		t.Fatal("last_message_at should be set after first update from NULL")
	}

	// Try to set older time - should NOT update
	olderTime := time.Now().Add(-2 * time.Hour).UTC()
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 150, &olderTime, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (older) failed: %v", err)
	}

	// Verify last_message_at was NOT updated (still initial time)
	var lastMsgAt *time.Time
	err = env.DB.QueryRow(ctx, "SELECT last_message_at FROM sessions WHERE id = $1", sessionID).Scan(&lastMsgAt)
	if err != nil {
		t.Fatalf("query last_message_at failed: %v", err)
	}
	if lastMsgAt == nil {
		t.Fatal("last_message_at should be set")
	}
	// Allow 1 second tolerance for database timestamp precision
	if lastMsgAt.Before(initialTime.Add(-time.Second)) || lastMsgAt.After(initialTime.Add(time.Second)) {
		t.Errorf("last_message_at = %v, should be around %v (older time %v should not have been used)", lastMsgAt, initialTime, olderTime)
	}

	// Set newer time - SHOULD update
	newerTime := time.Now().UTC()
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 200, &newerTime, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (newer) failed: %v", err)
	}

	// Verify last_message_at was updated
	err = env.DB.QueryRow(ctx, "SELECT last_message_at FROM sessions WHERE id = $1", sessionID).Scan(&lastMsgAt)
	if err != nil {
		t.Fatalf("query last_message_at failed: %v", err)
	}
	if lastMsgAt.Before(newerTime.Add(-time.Second)) || lastMsgAt.After(newerTime.Add(time.Second)) {
		t.Errorf("last_message_at = %v, should be around %v", lastMsgAt, newerTime)
	}
}

// TestUpdateSyncFileState_Summary tests summary update (last write wins)
func TestUpdateSyncFileState_Summary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "summary@test.com", "Summary User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-session")

	ctx := context.Background()

	// Set initial summary
	summary1 := "First summary"
	err := env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, nil, &summary1, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (summary1) failed: %v", err)
	}

	// Verify summary was set
	var storedSummary *string
	err = env.DB.QueryRow(ctx, "SELECT summary FROM sessions WHERE id = $1", sessionID).Scan(&storedSummary)
	if err != nil {
		t.Fatalf("query summary failed: %v", err)
	}
	if storedSummary == nil || *storedSummary != summary1 {
		t.Errorf("summary = %v, want %s", storedSummary, summary1)
	}

	// Update summary (last write wins)
	summary2 := "Updated summary"
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 150, nil, &summary2, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (summary2) failed: %v", err)
	}

	// Verify summary was updated
	err = env.DB.QueryRow(ctx, "SELECT summary FROM sessions WHERE id = $1", sessionID).Scan(&storedSummary)
	if err != nil {
		t.Fatalf("query summary failed: %v", err)
	}
	if storedSummary == nil || *storedSummary != summary2 {
		t.Errorf("summary = %v, want %s", storedSummary, summary2)
	}

	// Clear summary with empty string
	emptyStr := ""
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 200, nil, &emptyStr, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (empty summary) failed: %v", err)
	}

	// Verify summary was cleared
	err = env.DB.QueryRow(ctx, "SELECT summary FROM sessions WHERE id = $1", sessionID).Scan(&storedSummary)
	if err != nil {
		t.Fatalf("query summary failed: %v", err)
	}
	if storedSummary == nil || *storedSummary != "" {
		t.Errorf("summary = %v, want empty string", storedSummary)
	}
}

// TestUpdateSyncFileState_FirstUserMessage tests first_user_message (first write wins)
func TestUpdateSyncFileState_FirstUserMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "firstmsg@test.com", "FirstMsg User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "firstmsg-session")

	ctx := context.Background()

	// Set initial first_user_message
	msg1 := "First user message"
	err := env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, nil, nil, &msg1, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (msg1) failed: %v", err)
	}

	// Verify first_user_message was set
	var storedMsg *string
	err = env.DB.QueryRow(ctx, "SELECT first_user_message FROM sessions WHERE id = $1", sessionID).Scan(&storedMsg)
	if err != nil {
		t.Fatalf("query first_user_message failed: %v", err)
	}
	if storedMsg == nil || *storedMsg != msg1 {
		t.Errorf("first_user_message = %v, want %s", storedMsg, msg1)
	}

	// Try to update first_user_message (first write wins - should NOT update)
	msg2 := "Second user message"
	err = env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 150, nil, nil, &msg2, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (msg2) failed: %v", err)
	}

	// Verify first_user_message was NOT updated
	err = env.DB.QueryRow(ctx, "SELECT first_user_message FROM sessions WHERE id = $1", sessionID).Scan(&storedMsg)
	if err != nil {
		t.Fatalf("query first_user_message failed: %v", err)
	}
	if storedMsg == nil || *storedMsg != msg1 {
		t.Errorf("first_user_message = %v, want %s (should not have changed)", storedMsg, msg1)
	}
}

// TestUpdateSyncFileState_GitInfo tests git_info update
func TestUpdateSyncFileState_GitInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "gitinfo@test.com", "GitInfo User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "gitinfo-session")

	ctx := context.Background()

	// Set git_info
	gitInfo := json.RawMessage(`{"repo_url": "https://github.com/example/repo", "branch": "main"}`)
	err := env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, nil, nil, nil, gitInfo)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (gitinfo) failed: %v", err)
	}

	// Verify git_info was set
	var storedGitInfo []byte
	err = env.DB.QueryRow(ctx, "SELECT git_info FROM sessions WHERE id = $1", sessionID).Scan(&storedGitInfo)
	if err != nil {
		t.Fatalf("query git_info failed: %v", err)
	}
	if storedGitInfo == nil {
		t.Fatal("git_info should be set")
	}

	var parsed map[string]string
	if err := json.Unmarshal(storedGitInfo, &parsed); err != nil {
		t.Fatalf("failed to unmarshal git_info: %v", err)
	}
	if parsed["repo_url"] != "https://github.com/example/repo" {
		t.Errorf("git_info.repo_url = %s, want https://github.com/example/repo", parsed["repo_url"])
	}
	if parsed["branch"] != "main" {
		t.Errorf("git_info.branch = %s, want main", parsed["branch"])
	}
}

// TestUpdateSyncFileState_CombinedParameters tests multiple parameters at once
func TestUpdateSyncFileState_CombinedParameters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "combined@test.com", "Combined User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "combined-session")

	ctx := context.Background()

	// Set all optional parameters at once
	msgTime := time.Now().UTC()
	summary := "Combined test summary"
	firstMsg := "Combined first message"
	gitInfo := json.RawMessage(`{"repo_url": "https://github.com/combined/repo", "branch": "develop"}`)

	err := env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, &msgTime, &summary, &firstMsg, gitInfo)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (combined) failed: %v", err)
	}

	// Verify all fields were set
	var storedMsgTime *time.Time
	var storedSummary *string
	var storedFirstMsg *string
	var storedGitInfo []byte

	err = env.DB.QueryRow(ctx,
		"SELECT last_message_at, summary, first_user_message, git_info FROM sessions WHERE id = $1",
		sessionID).Scan(&storedMsgTime, &storedSummary, &storedFirstMsg, &storedGitInfo)
	if err != nil {
		t.Fatalf("query session failed: %v", err)
	}

	if storedMsgTime == nil {
		t.Error("last_message_at should be set")
	}
	if storedSummary == nil || *storedSummary != summary {
		t.Errorf("summary = %v, want %s", storedSummary, summary)
	}
	if storedFirstMsg == nil || *storedFirstMsg != firstMsg {
		t.Errorf("first_user_message = %v, want %s", storedFirstMsg, firstMsg)
	}
	if storedGitInfo == nil {
		t.Error("git_info should be set")
	}
}

// TestUpdateSyncFileState_NoOptionalParameters tests with no optional parameters
func TestUpdateSyncFileState_NoOptionalParameters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "noopt@test.com", "NoOpt User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "noopt-session")

	ctx := context.Background()

	// Update with no optional parameters
	err := env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState (no opts) failed: %v", err)
	}

	// Verify last_sync_at was still updated
	var lastSyncAt *time.Time
	err = env.DB.QueryRow(ctx, "SELECT last_sync_at FROM sessions WHERE id = $1", sessionID).Scan(&lastSyncAt)
	if err != nil {
		t.Fatalf("query last_sync_at failed: %v", err)
	}
	if lastSyncAt == nil {
		t.Error("last_sync_at should be set even with no optional parameters")
	}
}

// TestGetSyncFileState_NotFound tests error for non-existent file
func TestGetSyncFileState_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "notfound@test.com", "NotFound User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "notfound-session")

	ctx := context.Background()

	_, err := env.DB.GetSyncFileState(ctx, sessionID, "nonexistent.jsonl")
	if !errors.Is(err, db.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

// TestUpdateSyncFileChunkCount tests direct chunk count update (self-healing)
func TestUpdateSyncFileChunkCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "chunkfix@test.com", "ChunkFix User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "chunkfix-session")

	ctx := context.Background()

	// Create initial sync file state
	err := env.DB.UpdateSyncFileState(ctx, sessionID, "transcript.jsonl", "transcript", 100, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState failed: %v", err)
	}

	// Self-heal chunk count to specific value
	err = env.DB.UpdateSyncFileChunkCount(ctx, sessionID, "transcript.jsonl", 10)
	if err != nil {
		t.Fatalf("UpdateSyncFileChunkCount failed: %v", err)
	}

	// Verify chunk count was updated
	state, err := env.DB.GetSyncFileState(ctx, sessionID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("GetSyncFileState failed: %v", err)
	}
	if state.ChunkCount == nil || *state.ChunkCount != 10 {
		t.Errorf("ChunkCount = %v, want 10", state.ChunkCount)
	}
}

// =============================================================================
// FindOrCreateSyncSession Tests
// =============================================================================

// TestFindOrCreateSyncSession_CreateNew tests creating a new session
func TestFindOrCreateSyncSession_CreateNew(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "newsync@test.com", "NewSync User")

	ctx := context.Background()

	params := db.SyncSessionParams{
		ExternalID:     "new-external-id",
		TranscriptPath: "/path/to/transcript.jsonl",
		CWD:            "/home/user/project",
		GitInfo:        json.RawMessage(`{"repo_url": "https://github.com/example/repo"}`),
		Hostname:       "myhostname",
		Username:       "myuser",
	}

	sessionID, files, err := env.DB.FindOrCreateSyncSession(ctx, user.ID, params)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession failed: %v", err)
	}

	if sessionID == "" {
		t.Error("sessionID should not be empty")
	}
	if len(files) != 0 {
		t.Errorf("new session should have 0 files, got %d", len(files))
	}

	// Verify session was created with correct metadata
	var cwd, transcriptPath, hostname, username *string
	err = env.DB.QueryRow(ctx,
		"SELECT cwd, transcript_path, hostname, username FROM sessions WHERE id = $1",
		sessionID).Scan(&cwd, &transcriptPath, &hostname, &username)
	if err != nil {
		t.Fatalf("query session failed: %v", err)
	}
	if cwd == nil || *cwd != params.CWD {
		t.Errorf("cwd = %v, want %s", cwd, params.CWD)
	}
	if transcriptPath == nil || *transcriptPath != params.TranscriptPath {
		t.Errorf("transcript_path = %v, want %s", transcriptPath, params.TranscriptPath)
	}
	if hostname == nil || *hostname != params.Hostname {
		t.Errorf("hostname = %v, want %s", hostname, params.Hostname)
	}
	if username == nil || *username != params.Username {
		t.Errorf("username = %v, want %s", username, params.Username)
	}
}

// TestFindOrCreateSyncSession_FindExisting tests finding an existing session
func TestFindOrCreateSyncSession_FindExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "existsync@test.com", "ExistSync User")

	ctx := context.Background()

	// Create session first time
	params := db.SyncSessionParams{
		ExternalID:     "existing-external-id",
		TranscriptPath: "/path/to/transcript.jsonl",
		CWD:            "/home/user/project",
	}

	sessionID1, _, err := env.DB.FindOrCreateSyncSession(ctx, user.ID, params)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession (1) failed: %v", err)
	}

	// Add some sync files
	err = env.DB.UpdateSyncFileState(ctx, sessionID1, "transcript.jsonl", "transcript", 100, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState failed: %v", err)
	}
	err = env.DB.UpdateSyncFileState(ctx, sessionID1, "todo.jsonl", "todo", 50, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSyncFileState failed: %v", err)
	}

	// Find existing session
	sessionID2, files, err := env.DB.FindOrCreateSyncSession(ctx, user.ID, params)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession (2) failed: %v", err)
	}

	if sessionID2 != sessionID1 {
		t.Errorf("sessionID2 = %s, want %s (same session)", sessionID2, sessionID1)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
	if files["transcript.jsonl"].LastSyncedLine != 100 {
		t.Errorf("transcript.jsonl.LastSyncedLine = %d, want 100", files["transcript.jsonl"].LastSyncedLine)
	}
	if files["todo.jsonl"].LastSyncedLine != 50 {
		t.Errorf("todo.jsonl.LastSyncedLine = %d, want 50", files["todo.jsonl"].LastSyncedLine)
	}
}

// TestFindOrCreateSyncSession_UpdatesMetadata tests that metadata is updated on find
func TestFindOrCreateSyncSession_UpdatesMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "updatemeta@test.com", "UpdateMeta User")

	ctx := context.Background()

	// Create session with initial metadata
	params1 := db.SyncSessionParams{
		ExternalID:     "update-meta-external-id",
		TranscriptPath: "/path/to/transcript.jsonl",
		CWD:            "/home/user/project1",
		Hostname:       "host1",
		Username:       "user1",
	}

	sessionID, _, err := env.DB.FindOrCreateSyncSession(ctx, user.ID, params1)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession (1) failed: %v", err)
	}

	// Find again with updated metadata
	params2 := db.SyncSessionParams{
		ExternalID:     "update-meta-external-id",
		TranscriptPath: "/path/to/new-transcript.jsonl",
		CWD:            "/home/user/project2",
		GitInfo:        json.RawMessage(`{"repo_url": "https://github.com/new/repo"}`),
		Hostname:       "host2",
		Username:       "user2",
	}

	_, _, err = env.DB.FindOrCreateSyncSession(ctx, user.ID, params2)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession (2) failed: %v", err)
	}

	// Verify metadata was updated
	var cwd, transcriptPath, hostname, username *string
	var gitInfo []byte
	err = env.DB.QueryRow(ctx,
		"SELECT cwd, transcript_path, git_info, hostname, username FROM sessions WHERE id = $1",
		sessionID).Scan(&cwd, &transcriptPath, &gitInfo, &hostname, &username)
	if err != nil {
		t.Fatalf("query session failed: %v", err)
	}
	if cwd == nil || *cwd != params2.CWD {
		t.Errorf("cwd = %v, want %s", cwd, params2.CWD)
	}
	if transcriptPath == nil || *transcriptPath != params2.TranscriptPath {
		t.Errorf("transcript_path = %v, want %s", transcriptPath, params2.TranscriptPath)
	}
	if hostname == nil || *hostname != params2.Hostname {
		t.Errorf("hostname = %v, want %s", hostname, params2.Hostname)
	}
	if username == nil || *username != params2.Username {
		t.Errorf("username = %v, want %s", username, params2.Username)
	}
	if gitInfo == nil {
		t.Error("git_info should be set")
	}
}

// TestFindOrCreateSyncSession_DifferentUsers tests isolation between users
func TestFindOrCreateSyncSession_DifferentUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "user1@test.com", "User 1")
	user2 := testutil.CreateTestUser(t, env, "user2@test.com", "User 2")

	ctx := context.Background()

	params := db.SyncSessionParams{
		ExternalID: "shared-external-id", // Same external ID
		CWD:        "/home/user/project",
	}

	// Create session for user1
	sessionID1, _, err := env.DB.FindOrCreateSyncSession(ctx, user1.ID, params)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession (user1) failed: %v", err)
	}

	// Create session for user2 (should be different session)
	sessionID2, _, err := env.DB.FindOrCreateSyncSession(ctx, user2.ID, params)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession (user2) failed: %v", err)
	}

	if sessionID1 == sessionID2 {
		t.Error("different users should get different sessions even with same external_id")
	}
}

// TestFindOrCreateSyncSession_EmptyHostnameUsername tests empty hostname/username handling
func TestFindOrCreateSyncSession_EmptyHostnameUsername(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "emptyhost@test.com", "EmptyHost User")

	ctx := context.Background()

	// Create with hostname/username
	params1 := db.SyncSessionParams{
		ExternalID: "empty-host-external-id",
		CWD:        "/home/user/project",
		Hostname:   "myhostname",
		Username:   "myuser",
	}

	sessionID, _, err := env.DB.FindOrCreateSyncSession(ctx, user.ID, params1)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession (1) failed: %v", err)
	}

	// Update with empty hostname/username (should NOT clear existing values)
	params2 := db.SyncSessionParams{
		ExternalID: "empty-host-external-id",
		CWD:        "/home/user/project",
		Hostname:   "", // Empty
		Username:   "", // Empty
	}

	_, _, err = env.DB.FindOrCreateSyncSession(ctx, user.ID, params2)
	if err != nil {
		t.Fatalf("FindOrCreateSyncSession (2) failed: %v", err)
	}

	// Verify hostname/username were NOT cleared
	var hostname, username *string
	err = env.DB.QueryRow(ctx,
		"SELECT hostname, username FROM sessions WHERE id = $1",
		sessionID).Scan(&hostname, &username)
	if err != nil {
		t.Fatalf("query session failed: %v", err)
	}
	if hostname == nil || *hostname != params1.Hostname {
		t.Errorf("hostname = %v, want %s (should not be cleared)", hostname, params1.Hostname)
	}
	if username == nil || *username != params1.Username {
		t.Errorf("username = %v, want %s (should not be cleared)", username, params1.Username)
	}
}

// =============================================================================
// GetSessionOwnerAndExternalID Tests
// =============================================================================

// TestGetSessionOwnerAndExternalID_Success tests successful retrieval
func TestGetSessionOwnerAndExternalID_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "owner@test.com", "Owner User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-external-id")

	ctx := context.Background()

	userID, externalID, err := env.DB.GetSessionOwnerAndExternalID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionOwnerAndExternalID failed: %v", err)
	}
	if userID != user.ID {
		t.Errorf("userID = %d, want %d", userID, user.ID)
	}
	if externalID != "test-external-id" {
		t.Errorf("externalID = %s, want test-external-id", externalID)
	}
}

// TestGetSessionOwnerAndExternalID_NotFound tests non-existent session
func TestGetSessionOwnerAndExternalID_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()

	_, _, err := env.DB.GetSessionOwnerAndExternalID(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}
