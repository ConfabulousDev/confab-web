package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// FindStaleSessions Integration Tests
// =============================================================================

func TestFindStaleSessions_NoSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestFindStaleSessions_SessionWithNoCards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "stale@test.com", "Stale User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "stale-external-id")

	// Add transcript sync file (required for session to be considered)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	if sessions[0].SessionID != sessionID {
		t.Errorf("session ID = %s, want %s", sessions[0].SessionID, sessionID)
	}
	if sessions[0].UserID != user.ID {
		t.Errorf("user ID = %d, want %d", sessions[0].UserID, user.ID)
	}
	if sessions[0].TotalLines != 100 {
		t.Errorf("total lines = %d, want 100", sessions[0].TotalLines)
	}
}

func TestFindStaleSessions_SessionWithUpToDateCards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "uptodate@test.com", "UpToDate User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "uptodate-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert up-to-date tokens card (version matches, line count matches)
	insertTokensCard(t, env, sessionID, analytics.TokensCardVersion, 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should not find this session since cards are up to date
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (cards up to date), got %d", len(sessions))
	}
}

func TestFindStaleSessions_SessionWithOutdatedVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "oldversion@test.com", "OldVersion User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "oldversion-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert outdated tokens card (old version)
	insertTokensCard(t, env, sessionID, analytics.TokensCardVersion-1, 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should find this session since version is outdated
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (outdated version), got %d", len(sessions))
	}
}

func TestFindStaleSessions_SessionWithNewLines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "newlines@test.com", "NewLines User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "newlines-external-id")

	// Add transcript sync file with 150 lines
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 150)

	// Insert tokens card computed at 100 lines (stale because current is 150)
	insertTokensCard(t, env, sessionID, analytics.TokensCardVersion, 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should find this session since line count increased
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (new lines), got %d", len(sessions))
	}
	if sessions[0].TotalLines != 150 {
		t.Errorf("total lines = %d, want 150", sessions[0].TotalLines)
	}
}

func TestFindStaleSessions_IncludesAgentFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "agents@test.com", "Agents User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "agents-external-id")

	// Add transcript and agent files
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
	testutil.CreateTestSyncFile(t, env, sessionID, "agent-abc123.jsonl", "agent", 50)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	// Total lines should include both transcript and agent files
	if sessions[0].TotalLines != 150 {
		t.Errorf("total lines = %d, want 150 (100 + 50)", sessions[0].TotalLines)
	}
}

func TestFindStaleSessions_IgnoresNonClaudeCodeSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session with a different session_type (not Claude Code)
	user := testutil.CreateTestUser(t, env, "regular@test.com", "Regular User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "regular-external-id")

	// Change session_type to something else
	_, err := env.DB.Exec(env.Ctx, "UPDATE sessions SET session_type = 'Other Tool' WHERE id = $1", sessionID)
	if err != nil {
		t.Fatalf("failed to update session type: %v", err)
	}

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should not find this session since it's not a Claude Code session
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (not Claude Code), got %d", len(sessions))
	}
}

func TestFindStaleSessions_RespectsLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create multiple stale sessions
	user := testutil.CreateTestUser(t, env, "limit@test.com", "Limit User")
	for i := 0; i < 5; i++ {
		sessionID := testutil.CreateTestSession(t, env, user.ID, "limit-external-id-"+string(rune('a'+i)))
		testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
	}

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	// Request only 2
	sessions, err := precomputer.FindStaleSessions(context.Background(), 2)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions (limit), got %d", len(sessions))
	}
}

func TestFindStaleSessions_IgnoresEmptySessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session with no sync files
	user := testutil.CreateTestUser(t, env, "empty@test.com", "Empty User")
	_ = testutil.CreateTestSession(t, env, user.ID, "empty-external-id")

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should not find empty sessions
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (empty), got %d", len(sessions))
	}
}

// =============================================================================
// PrecomputeSession Integration Tests
// =============================================================================

func TestPrecomputeSession_ComputesAndUpsertsCards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create user and session
	user := testutil.CreateTestUser(t, env, "precompute@test.com", "Precompute User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "precompute-external-id")

	// Create sync file record
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 3)

	// Upload test transcript to S3
	transcript := testutil.MinimalTranscript()
	testutil.UploadTestTranscript(t, env, user.ID, "precompute-external-id", "transcript.jsonl", transcript)

	// Create precomputer and run
	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	staleSession := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     user.ID,
		ExternalID: "precompute-external-id",
		TotalLines: 3,
	}

	err := precomputer.PrecomputeSession(context.Background(), staleSession)
	if err != nil {
		t.Fatalf("PrecomputeSession failed: %v", err)
	}

	// Verify cards were created
	cards, err := analyticsStore.GetCards(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetCards failed: %v", err)
	}

	if cards == nil {
		t.Fatal("expected cards to be created, got nil")
	}

	// Verify tokens card
	if cards.Tokens == nil {
		t.Error("expected tokens card to be created")
	} else {
		if cards.Tokens.UpToLine != 3 {
			t.Errorf("tokens card up_to_line = %d, want 3", cards.Tokens.UpToLine)
		}
		if cards.Tokens.Version != analytics.TokensCardVersion {
			t.Errorf("tokens card version = %d, want %d", cards.Tokens.Version, analytics.TokensCardVersion)
		}
	}

	// Verify session card
	if cards.Session == nil {
		t.Error("expected session card to be created")
	}

	// Verify tools card
	if cards.Tools == nil {
		t.Error("expected tools card to be created")
	}
}

func TestPrecomputeSession_EmptyTranscript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create user and session
	user := testutil.CreateTestUser(t, env, "empty@test.com", "Empty User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "empty-external-id")

	// Create sync file record but don't upload any S3 data
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 0)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	staleSession := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     user.ID,
		ExternalID: "empty-external-id",
		TotalLines: 0,
	}

	// Should not error on empty transcript
	err := precomputer.PrecomputeSession(context.Background(), staleSession)
	if err != nil {
		t.Fatalf("PrecomputeSession failed on empty transcript: %v", err)
	}

	// Cards should not be created for empty transcript
	cards, err := analyticsStore.GetCards(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetCards failed: %v", err)
	}

	if cards != nil && cards.Tokens != nil {
		t.Error("expected no cards for empty transcript")
	}
}

func TestPrecomputeSession_UpdatesExistingCards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create user and session
	user := testutil.CreateTestUser(t, env, "update@test.com", "Update User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "update-external-id")

	// Insert old tokens card
	insertTokensCard(t, env, sessionID, analytics.TokensCardVersion, 1)

	// Create sync file with more lines
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 3)

	// Upload test transcript
	transcript := testutil.MinimalTranscript()
	testutil.UploadTestTranscript(t, env, user.ID, "update-external-id", "transcript.jsonl", transcript)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{})

	staleSession := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     user.ID,
		ExternalID: "update-external-id",
		TotalLines: 3,
	}

	err := precomputer.PrecomputeSession(context.Background(), staleSession)
	if err != nil {
		t.Fatalf("PrecomputeSession failed: %v", err)
	}

	// Verify card was updated
	cards, err := analyticsStore.GetCards(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetCards failed: %v", err)
	}

	if cards.Tokens.UpToLine != 3 {
		t.Errorf("tokens card up_to_line = %d, want 3 (updated)", cards.Tokens.UpToLine)
	}
}

// =============================================================================
// Helper functions
// =============================================================================

// insertTokensCard inserts a tokens card with the given version and line count
func insertTokensCard(t *testing.T, env *testutil.TestEnvironment, sessionID string, version int, upToLine int64) {
	t.Helper()

	query := `
		INSERT INTO session_card_tokens (
			session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			estimated_cost_usd
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, '0.00')
	`

	_, err := env.DB.Exec(env.Ctx, query, sessionID, version, time.Now().UTC(), upToLine)
	if err != nil {
		t.Fatalf("failed to insert tokens card: %v", err)
	}
}
