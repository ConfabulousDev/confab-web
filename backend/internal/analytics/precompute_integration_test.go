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

	// Insert all up-to-date cards (versions match, line counts match)
	insertAllCards(t, env, sessionID, 100)

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

// insertAllCards inserts all 7 card types with matching versions and line counts.
// This is needed for testing that sessions with fully up-to-date cards are not marked stale.
func insertAllCards(t *testing.T, env *testutil.TestEnvironment, sessionID string, upToLine int64) {
	t.Helper()
	now := time.Now().UTC()

	// Tokens card
	_, err := env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_tokens (
			session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			estimated_cost_usd
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, '0.00')
	`, sessionID, analytics.TokensCardVersion, now, upToLine)
	if err != nil {
		t.Fatalf("failed to insert tokens card: %v", err)
	}

	// Session card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_session (
			session_id, version, computed_at, up_to_line,
			total_messages, user_messages, assistant_messages,
			human_prompts, tool_results, text_responses, tool_calls, thinking_blocks,
			duration_ms, models_used,
			compaction_auto, compaction_manual, compaction_avg_time_ms
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, 0, 0, 0, 0, '[]', 0, 0, 0)
	`, sessionID, analytics.SessionCardVersion, now, upToLine)
	if err != nil {
		t.Fatalf("failed to insert session card: %v", err)
	}

	// Tools card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_tools (
			session_id, version, computed_at, up_to_line,
			total_calls, tool_breakdown, error_count
		) VALUES ($1, $2, $3, $4, 0, '{}', 0)
	`, sessionID, analytics.ToolsCardVersion, now, upToLine)
	if err != nil {
		t.Fatalf("failed to insert tools card: %v", err)
	}

	// Code activity card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_code_activity (
			session_id, version, computed_at, up_to_line,
			files_read, files_modified, lines_added, lines_removed, search_count,
			language_breakdown
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, '{}')
	`, sessionID, analytics.CodeActivityCardVersion, now, upToLine)
	if err != nil {
		t.Fatalf("failed to insert code activity card: %v", err)
	}

	// Conversation card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_conversation (
			session_id, version, computed_at, up_to_line,
			user_turns, assistant_turns, avg_assistant_turn_ms, avg_user_thinking_ms,
			total_assistant_duration_ms, total_user_duration_ms, assistant_utilization
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, 0, 0)
	`, sessionID, analytics.ConversationCardVersion, now, upToLine)
	if err != nil {
		t.Fatalf("failed to insert conversation card: %v", err)
	}

	// Agents and skills card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_agents_and_skills (
			session_id, version, computed_at, up_to_line,
			agent_invocations, skill_invocations, agent_stats, skill_stats
		) VALUES ($1, $2, $3, $4, 0, 0, '{}', '{}')
	`, sessionID, analytics.AgentsAndSkillsCardVersion, now, upToLine)
	if err != nil {
		t.Fatalf("failed to insert agents and skills card: %v", err)
	}

	// Redactions card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_redactions (
			session_id, version, computed_at, up_to_line,
			total_redactions, redaction_counts
		) VALUES ($1, $2, $3, $4, 0, '{}')
	`, sessionID, analytics.RedactionsCardVersion, now, upToLine)
	if err != nil {
		t.Fatalf("failed to insert redactions card: %v", err)
	}
}

// insertSmartRecapCard inserts a smart recap card with the given parameters.
func insertSmartRecapCard(t *testing.T, env *testutil.TestEnvironment, sessionID string, version int, upToLine int64, computedAt time.Time) {
	t.Helper()

	_, err := env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_smart_recap (
			session_id, version, computed_at, up_to_line,
			recap, went_well, went_bad, human_suggestions, environment_suggestions, default_context_suggestions,
			model_used, input_tokens, output_tokens, generation_time_ms
		) VALUES ($1, $2, $3, $4, '', '[]', '[]', '[]', '[]', '[]', 'test-model', 0, 0, 0)
	`, sessionID, version, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert smart recap card: %v", err)
	}
}

// =============================================================================
// FindStaleSmartRecapSessions Integration Tests
// =============================================================================

func TestFindStaleSmartRecapSessions_DisabledReturnsNil(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	// Smart recap disabled (no API key, etc.)
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled: false,
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	if sessions != nil {
		t.Errorf("expected nil when smart recap disabled, got %d sessions", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_RegularCardsStale_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "regularstale@test.com", "RegularStale User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "regularstale-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert outdated tokens card (regular cards are stale)
	insertTokensCard(t, env, sessionID, analytics.TokensCardVersion-1, 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10,
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Should NOT find this session - regular cards are stale (Query 1's job)
	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (regular cards stale), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_AllFresh_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "allfresh@test.com", "AllFresh User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "allfresh-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert all up-to-date regular cards
	insertAllCards(t, env, sessionID, 100)

	// Insert up-to-date smart recap card (same line count, recent computed_at)
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, time.Now().UTC())

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10,
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Should NOT find this session - everything is up-to-date
	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (all fresh), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_SmartRecapMissing_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "srmissing@test.com", "SRMissing User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srmissing-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert all up-to-date regular cards (no smart recap)
	insertAllCards(t, env, sessionID, 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10,
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Should find this session - smart recap is missing
	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (smart recap missing), got %d", len(sessions))
	}
	if sessions[0].SessionID != sessionID {
		t.Errorf("session ID = %s, want %s", sessions[0].SessionID, sessionID)
	}
}

func TestFindStaleSmartRecapSessions_SmartRecapOutdatedVersion_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "sroldver@test.com", "SROldVer User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "sroldver-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert all up-to-date regular cards
	insertAllCards(t, env, sessionID, 100)

	// Insert smart recap with outdated version
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion-1, 100, time.Now().UTC())

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10,
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Should find this session - smart recap version is outdated
	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (smart recap outdated version), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_NewLinesWithinThreshold_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "newlinesrecent@test.com", "NewLinesRecent User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "newlinesrecent-external-id")

	// Add transcript sync file with 150 lines (50 new lines)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 150)

	// Insert all up-to-date regular cards
	insertAllCards(t, env, sessionID, 150)

	// Insert smart recap at 100 lines but computed recently (within threshold)
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, time.Now().UTC())

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10, // 10 minute threshold
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Should NOT find - has new lines but computed recently (within threshold)
	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (new lines but within time threshold), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_NewLinesPastThreshold_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "newlinesold@test.com", "NewLinesOld User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "newlinesold-external-id")

	// Add transcript sync file with 150 lines (50 new lines)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 150)

	// Insert all up-to-date regular cards
	insertAllCards(t, env, sessionID, 150)

	// Insert smart recap at 100 lines, computed 20 minutes ago (past 10 min threshold)
	pastThreshold := time.Now().UTC().Add(-20 * time.Minute)
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, pastThreshold)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10, // 10 minute threshold
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Should find - has new lines AND computed time exceeded threshold
	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (new lines past time threshold), got %d", len(sessions))
	}
	if sessions[0].TotalLines != 150 {
		t.Errorf("total lines = %d, want 150", sessions[0].TotalLines)
	}
}

// =============================================================================
// Two-Bucket Discovery Integration Tests
// =============================================================================

func TestTwoBucketDiscovery_BothStale_FoundByQuery1Only(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "bothstale@test.com", "BothStale User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "bothstale-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert outdated tokens card (regular cards stale) - no other cards
	insertTokensCard(t, env, sessionID, analytics.TokensCardVersion-1, 100)
	// No smart recap card either

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10,
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Query 1 should find it (regular cards stale)
	regularSessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}
	if len(regularSessions) != 1 {
		t.Errorf("Query 1: expected 1 session, got %d", len(regularSessions))
	}

	// Query 2 should NOT find it (regular cards not up-to-date)
	smartRecapSessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}
	if len(smartRecapSessions) != 0 {
		t.Errorf("Query 2: expected 0 sessions (regular cards stale), got %d", len(smartRecapSessions))
	}
}

func TestTwoBucketDiscovery_OnlySmartRecapStale_FoundByQuery2Only(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "onlysrstale@test.com", "OnlySRStale User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "onlysrstale-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert all up-to-date regular cards
	insertAllCards(t, env, sessionID, 100)
	// No smart recap card (missing = stale)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10,
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Query 1 should NOT find it (regular cards up-to-date)
	regularSessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}
	if len(regularSessions) != 0 {
		t.Errorf("Query 1: expected 0 sessions (regular cards fresh), got %d", len(regularSessions))
	}

	// Query 2 should find it (smart recap missing)
	smartRecapSessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}
	if len(smartRecapSessions) != 1 {
		t.Errorf("Query 2: expected 1 session (smart recap stale), got %d", len(smartRecapSessions))
	}
}

func TestTwoBucketDiscovery_AllFresh_FoundByNeither(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "alluptodate@test.com", "AllUpToDate User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "alluptodate-external-id")

	// Add transcript sync file
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert all up-to-date regular cards
	insertAllCards(t, env, sessionID, 100)
	// Insert up-to-date smart recap
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, time.Now().UTC())

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:  true,
		StalenessMinutes:   10,
		AnthropicAPIKey:    "test-key",
		SmartRecapModel:    "test-model",
		SmartRecapQuota:    100,
		LockTimeoutSeconds: 60,
	})

	// Query 1 should NOT find it
	regularSessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}
	if len(regularSessions) != 0 {
		t.Errorf("Query 1: expected 0 sessions, got %d", len(regularSessions))
	}

	// Query 2 should NOT find it
	smartRecapSessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}
	if len(smartRecapSessions) != 0 {
		t.Errorf("Query 2: expected 0 sessions, got %d", len(smartRecapSessions))
	}
}
