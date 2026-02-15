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

// defaultTestConfig returns a PrecomputeConfig with default thresholds for testing.
func defaultTestConfig() analytics.PrecomputeConfig {
	return analytics.PrecomputeConfig{
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
	}
}

func TestFindStaleSessions_NoSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

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
// PrecomputeRegularCards Integration Tests
// =============================================================================

func TestPrecomputeRegularCards_ComputesAndUpsertsCards(t *testing.T) {
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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

	staleSession := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     user.ID,
		ExternalID: "precompute-external-id",
		TotalLines: 3,
	}

	err := precomputer.PrecomputeRegularCards(context.Background(), staleSession)
	if err != nil {
		t.Fatalf("PrecomputeRegularCards failed: %v", err)
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

func TestPrecomputeRegularCards_EmptyTranscript(t *testing.T) {
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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

	staleSession := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     user.ID,
		ExternalID: "empty-external-id",
		TotalLines: 0,
	}

	// Should not error on empty transcript
	err := precomputer.PrecomputeRegularCards(context.Background(), staleSession)
	if err != nil {
		t.Fatalf("PrecomputeRegularCards failed on empty transcript: %v", err)
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

func TestPrecomputeRegularCards_UpdatesExistingCards(t *testing.T) {
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
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

	staleSession := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     user.ID,
		ExternalID: "update-external-id",
		TotalLines: 3,
	}

	err := precomputer.PrecomputeRegularCards(context.Background(), staleSession)
	if err != nil {
		t.Fatalf("PrecomputeRegularCards failed: %v", err)
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
			total_assistant_duration_ms, total_user_duration_ms, assistant_utilization_pct
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
		SmartRecapEnabled:      false,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
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
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
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
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
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
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
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
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
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

func TestFindStaleSmartRecapSessions_NewLines_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and session
	user := testutil.CreateTestUser(t, env, "newlines@test.com", "NewLines User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "newlines-external-id")

	// Add transcript sync file with 1000 lines (200 new lines since recap at 800)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1000)

	// Insert all up-to-date regular cards
	insertAllCards(t, env, sessionID, 1000)

	// Insert smart recap at 800 lines (gap = 200, threshold = max(150, 800*0.20=160) = 160)
	// Gap 200 > 160, so should be stale
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 800, time.Now().UTC())

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
	})

	// Should find - has new lines (up_to_line 800 < total_lines 1000, gap 200 > threshold 160)
	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (has new lines), got %d", len(sessions))
	}
	if sessions[0].TotalLines != 1000 {
		t.Errorf("total lines = %d, want 1000", sessions[0].TotalLines)
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
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
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
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
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
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
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

// =============================================================================
// Staleness Threshold Integration Tests
// =============================================================================

// testThresholds returns low thresholds suitable for testing.
func testThresholds() analytics.StalenessThresholds {
	return analytics.StalenessThresholds{
		ThresholdPct:    0.20, // 20%
		BaseMinLines:    5,
		BaseMinTime:     3 * time.Minute,
		MinInitialLines: 10,
		MinSessionAge:   10 * time.Minute,
	}
}

// setSessionFirstSeen updates a session's first_seen timestamp for testing.
// Note: Converts to UTC for consistent behavior with PostgreSQL NOW().
func setSessionFirstSeen(t *testing.T, env *testutil.TestEnvironment, sessionID string, firstSeen time.Time) {
	t.Helper()
	// Convert to UTC to match PostgreSQL's timestamp handling
	firstSeenUTC := firstSeen.UTC()
	_, err := env.DB.Exec(env.Ctx, "UPDATE sessions SET first_seen = $1 WHERE id = $2", firstSeenUTC, sessionID)
	if err != nil {
		t.Fatalf("failed to set first_seen: %v", err)
	}
}

// insertAllCardsWithComputedAt inserts all 7 card types with specified computed_at time.
// Note: Converts to UTC for consistent behavior with PostgreSQL NOW().
func insertAllCardsWithComputedAt(t *testing.T, env *testutil.TestEnvironment, sessionID string, upToLine int64, computedAt time.Time) {
	t.Helper()
	// Convert to UTC to match PostgreSQL's timestamp handling
	computedAt = computedAt.UTC()

	// Tokens card
	_, err := env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_tokens (
			session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			estimated_cost_usd
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, '0.00')
	`, sessionID, analytics.TokensCardVersion, computedAt, upToLine)
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
	`, sessionID, analytics.SessionCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert session card: %v", err)
	}

	// Tools card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_tools (
			session_id, version, computed_at, up_to_line,
			total_calls, tool_breakdown, error_count
		) VALUES ($1, $2, $3, $4, 0, '{}', 0)
	`, sessionID, analytics.ToolsCardVersion, computedAt, upToLine)
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
	`, sessionID, analytics.CodeActivityCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert code activity card: %v", err)
	}

	// Conversation card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_conversation (
			session_id, version, computed_at, up_to_line,
			user_turns, assistant_turns, avg_assistant_turn_ms, avg_user_thinking_ms,
			total_assistant_duration_ms, total_user_duration_ms, assistant_utilization_pct
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, 0, 0)
	`, sessionID, analytics.ConversationCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert conversation card: %v", err)
	}

	// Agents and skills card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_agents_and_skills (
			session_id, version, computed_at, up_to_line,
			agent_invocations, skill_invocations, agent_stats, skill_stats
		) VALUES ($1, $2, $3, $4, 0, 0, '{}', '{}')
	`, sessionID, analytics.AgentsAndSkillsCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert agents and skills card: %v", err)
	}

	// Redactions card
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_redactions (
			session_id, version, computed_at, up_to_line,
			total_redactions, redaction_counts
		) VALUES ($1, $2, $3, $4, 0, '{}')
	`, sessionID, analytics.RedactionsCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert redactions card: %v", err)
	}
}

// insertAllCardsWithWrongTokensVersion inserts all 7 cards but with the specified version
// for the tokens card (to test version mismatch scenarios).
func insertAllCardsWithWrongTokensVersion(t *testing.T, env *testutil.TestEnvironment, sessionID string, upToLine int64, tokensVersion int) {
	t.Helper()
	computedAt := time.Now().UTC()

	// Tokens card with wrong version
	_, err := env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_tokens (
			session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			estimated_cost_usd
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, '0.00')
	`, sessionID, tokensVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert tokens card: %v", err)
	}

	// Session card (correct version)
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_session (
			session_id, version, computed_at, up_to_line,
			total_messages, user_messages, assistant_messages,
			human_prompts, tool_results, text_responses, tool_calls, thinking_blocks,
			duration_ms, models_used,
			compaction_auto, compaction_manual, compaction_avg_time_ms
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, 0, 0, 0, 0, '[]', 0, 0, 0)
	`, sessionID, analytics.SessionCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert session card: %v", err)
	}

	// Tools card (correct version)
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_tools (
			session_id, version, computed_at, up_to_line,
			total_calls, tool_breakdown, error_count
		) VALUES ($1, $2, $3, $4, 0, '{}', 0)
	`, sessionID, analytics.ToolsCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert tools card: %v", err)
	}

	// Code activity card (correct version)
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_code_activity (
			session_id, version, computed_at, up_to_line,
			files_read, files_modified, lines_added, lines_removed, search_count,
			language_breakdown
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, '{}')
	`, sessionID, analytics.CodeActivityCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert code activity card: %v", err)
	}

	// Conversation card (correct version)
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_conversation (
			session_id, version, computed_at, up_to_line,
			user_turns, assistant_turns, avg_assistant_turn_ms, avg_user_thinking_ms,
			total_assistant_duration_ms, total_user_duration_ms, assistant_utilization_pct
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, 0, 0)
	`, sessionID, analytics.ConversationCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert conversation card: %v", err)
	}

	// Agents and skills card (correct version)
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_agents_and_skills (
			session_id, version, computed_at, up_to_line,
			agent_invocations, skill_invocations, agent_stats, skill_stats
		) VALUES ($1, $2, $3, $4, 0, 0, '{}', '{}')
	`, sessionID, analytics.AgentsAndSkillsCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert agents and skills card: %v", err)
	}

	// Redactions card (correct version)
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_redactions (
			session_id, version, computed_at, up_to_line,
			total_redactions, redaction_counts
		) VALUES ($1, $2, $3, $4, 0, '{}')
	`, sessionID, analytics.RedactionsCardVersion, computedAt, upToLine)
	if err != nil {
		t.Fatalf("failed to insert redactions card: %v", err)
	}
}

func TestFindStaleSessions_NewSession_BelowMinLines_Young_Skipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session with 5 lines (below min_initial_lines=10) and young (2 min old)
	user := testutil.CreateTestUser(t, env, "newsmall@test.com", "NewSmall User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "newsmall-external-id")

	// Set first_seen to 2 minutes ago (below min_session_age=10m)
	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-2*time.Minute))

	// Add 5 lines (below min_initial_lines=10)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 5)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should NOT find - too few lines AND too young
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (below min lines and young), got %d", len(sessions))
	}
}

func TestFindStaleSessions_NewSession_MeetsMinLines_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session with 15 lines (above min_initial_lines=10)
	user := testutil.CreateTestUser(t, env, "newlarge@test.com", "NewLarge User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "newlarge-external-id")

	// Young session (2 min old) - should still be found due to line count
	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-2*time.Minute))

	// Add 15 lines (above min_initial_lines=10)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 15)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should find - meets min_initial_lines
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (meets min lines), got %d", len(sessions))
	}
}

func TestFindStaleSessions_NewSession_BelowMinLines_Old_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session with 5 lines but old (15 min, above min_session_age=10m)
	user := testutil.CreateTestUser(t, env, "newold@test.com", "NewOld User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "newold-external-id")

	// Set first_seen to 15 minutes ago (above min_session_age=10m)
	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-15*time.Minute))

	// Add 5 lines (below min_initial_lines=10)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 5)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should find - session is old enough (catch-all)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (old enough), got %d", len(sessions))
	}
}

func TestFindStaleSessions_Cached_NoLineGap_Skipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session with all cards up-to-date (line_gap = 0)
	user := testutil.CreateTestUser(t, env, "nogap@test.com", "NoGap User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "nogap-external-id")

	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert all cards at line 100 (fully up-to-date)
	insertAllCards(t, env, sessionID, 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should NOT find - line_gap = 0
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (no line gap), got %d", len(sessions))
	}
}

func TestFindStaleSessions_Cached_SmallLineGap_Recent_Skipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session: 105 lines, cards at 100 (5 line gap, below 20% threshold)
	user := testutil.CreateTestUser(t, env, "smallgap@test.com", "SmallGap User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "smallgap-external-id")

	// Session started 1 hour ago
	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-1*time.Hour))

	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 105)

	// Insert cards at 100 lines, computed 1 minute ago (recent)
	insertAllCardsWithComputedAt(t, env, sessionID, 100, time.Now().Add(-1*time.Minute))

	th := testThresholds()

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: th,
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should NOT find - line_gap=5 < line_threshold=max(5, 100*0.20)=20, time_gap=1m < time_threshold
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (below thresholds), got %d", len(sessions))
	}
}

func TestFindStaleSessions_Cached_LineThresholdMet_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session: 125 lines, cards at 100 (25 line gap, meets 20% threshold)
	user := testutil.CreateTestUser(t, env, "linethreshold@test.com", "LineThreshold User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "linethreshold-external-id")

	// Session started 1 hour ago
	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-1*time.Hour))

	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 125)

	// Insert cards at 100 lines, computed 1 minute ago
	insertAllCardsWithComputedAt(t, env, sessionID, 100, time.Now().Add(-1*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should find - line_gap=25 >= line_threshold=max(5, 100*0.20)=20
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (line threshold met), got %d", len(sessions))
	}
}

func TestFindStaleSessions_Cached_TimeThresholdMet_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session: 105 lines, cards at 100 (5 line gap), but computed long ago
	user := testutil.CreateTestUser(t, env, "timethreshold@test.com", "TimeThreshold User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "timethreshold-external-id")

	// Session started 1 hour ago
	firstSeen := time.Now().Add(-1 * time.Hour)
	setSessionFirstSeen(t, env, sessionID, firstSeen)

	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 105)

	// Insert cards at 100 lines, computed 30 minutes ago (long time_gap)
	// prior_duration = computed_at - first_seen = 30 min
	// time_threshold = max(3m, 30m * 0.20) = max(3m, 6m) = 6m
	// time_gap = now - computed_at = 30m
	// 30m >= 6m ✓
	insertAllCardsWithComputedAt(t, env, sessionID, 100, time.Now().Add(-30*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should find - time_gap meets threshold
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (time threshold met), got %d", len(sessions))
	}
}

func TestFindStaleSessions_LargeSession_SmallGap_Skipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a large session: 1050 lines, cards at 1000 (50 line gap)
	// line_threshold = max(5, 1000*0.20) = 200
	// 50 < 200 - should be skipped
	user := testutil.CreateTestUser(t, env, "largesmall@test.com", "LargeSmall User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "largesmall-external-id")

	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-2*time.Hour))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1050)

	// Insert cards at 1000 lines, computed 5 minutes ago
	insertAllCardsWithComputedAt(t, env, sessionID, 1000, time.Now().Add(-5*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should NOT find - gap=50 < threshold=200
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (large session, small gap), got %d", len(sessions))
	}
}

func TestFindStaleSessions_LargeSession_20PercentGap_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a large session: 1250 lines, cards at 1000 (250 line gap = 25%)
	// line_threshold = max(5, 1000*0.20) = 200
	// 250 >= 200 - should be found
	user := testutil.CreateTestUser(t, env, "largelarge@test.com", "LargeLarge User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "largelarge-external-id")

	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-2*time.Hour))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1250)

	// Insert cards at 1000 lines, computed 1 minute ago (recent, but line gap is large)
	insertAllCardsWithComputedAt(t, env, sessionID, 1000, time.Now().Add(-1*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should find - gap=250 >= threshold=200
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (large gap percentage), got %d", len(sessions))
	}
}

func TestFindStaleSessions_VersionMismatch_AlwaysFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a session with wrong version card (even with no line gap)
	user := testutil.CreateTestUser(t, env, "wrongver@test.com", "WrongVer User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "wrongver-external-id")

	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-1*time.Hour))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	// Insert tokens card with old version (all others current)
	insertTokensCard(t, env, sessionID, analytics.TokensCardVersion-1, 100)
	// Insert other cards with current version
	now := time.Now().UTC()
	_, _ = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_session (session_id, version, computed_at, up_to_line, total_messages, user_messages, assistant_messages, human_prompts, tool_results, text_responses, tool_calls, thinking_blocks, duration_ms, models_used, compaction_auto, compaction_manual, compaction_avg_time_ms)
		VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, 0, 0, 0, 0, '[]', 0, 0, 0)
	`, sessionID, analytics.SessionCardVersion, now, 100)
	_, _ = env.DB.Exec(env.Ctx, `INSERT INTO session_card_tools (session_id, version, computed_at, up_to_line, total_calls, tool_breakdown, error_count) VALUES ($1, $2, $3, $4, 0, '{}', 0)`, sessionID, analytics.ToolsCardVersion, now, 100)
	_, _ = env.DB.Exec(env.Ctx, `INSERT INTO session_card_code_activity (session_id, version, computed_at, up_to_line, files_read, files_modified, lines_added, lines_removed, search_count, language_breakdown) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, '{}')`, sessionID, analytics.CodeActivityCardVersion, now, 100)
	_, _ = env.DB.Exec(env.Ctx, `INSERT INTO session_card_conversation (session_id, version, computed_at, up_to_line, user_turns, assistant_turns, avg_assistant_turn_ms, avg_user_thinking_ms, total_assistant_duration_ms, total_user_duration_ms, assistant_utilization_pct) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, 0, 0)`, sessionID, analytics.ConversationCardVersion, now, 100)
	_, _ = env.DB.Exec(env.Ctx, `INSERT INTO session_card_agents_and_skills (session_id, version, computed_at, up_to_line, agent_invocations, skill_invocations, agent_stats, skill_stats) VALUES ($1, $2, $3, $4, 0, 0, '{}', '{}')`, sessionID, analytics.AgentsAndSkillsCardVersion, now, 100)
	_, _ = env.DB.Exec(env.Ctx, `INSERT INTO session_card_redactions (session_id, version, computed_at, up_to_line, total_redactions, redaction_counts) VALUES ($1, $2, $3, $4, 0, '{}')`, sessionID, analytics.RedactionsCardVersion, now, 100)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	// Should find - version mismatch always triggers recompute
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (version mismatch), got %d", len(sessions))
	}
}

func TestFindStaleSessions_OrdersByPriority(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "ordering@test.com", "Ordering User")

	// Create session 1: new session (highest priority)
	session1 := testutil.CreateTestSession(t, env, user.ID, "ordering-1")
	setSessionFirstSeen(t, env, session1, time.Now().Add(-15*time.Minute))
	testutil.CreateTestSyncFile(t, env, session1, "transcript.jsonl", "transcript", 50)

	// Create session 2: version mismatch (second priority)
	// All cards must exist for version mismatch to be detected (not Case 1)
	session2 := testutil.CreateTestSession(t, env, user.ID, "ordering-2")
	setSessionFirstSeen(t, env, session2, time.Now().Add(-1*time.Hour))
	testutil.CreateTestSyncFile(t, env, session2, "transcript.jsonl", "transcript", 100)
	insertAllCardsWithWrongTokensVersion(t, env, session2, 100, analytics.TokensCardVersion-1) // wrong version

	// Create session 3: large line gap (third priority but larger gap)
	session3 := testutil.CreateTestSession(t, env, user.ID, "ordering-3")
	setSessionFirstSeen(t, env, session3, time.Now().Add(-2*time.Hour))
	testutil.CreateTestSyncFile(t, env, session3, "transcript.jsonl", "transcript", 150)
	insertAllCardsWithComputedAt(t, env, session3, 100, time.Now().Add(-1*time.Minute)) // 50 line gap

	// Create session 4: smaller line gap
	session4 := testutil.CreateTestSession(t, env, user.ID, "ordering-4")
	setSessionFirstSeen(t, env, session4, time.Now().Add(-2*time.Hour))
	testutil.CreateTestSyncFile(t, env, session4, "transcript.jsonl", "transcript", 130)
	insertAllCardsWithComputedAt(t, env, session4, 100, time.Now().Add(-1*time.Minute)) // 30 line gap

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		RegularCardsThresholds: testThresholds(),
	})

	sessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	if len(sessions) != 4 {
		t.Fatalf("expected 4 sessions, got %d", len(sessions))
	}

	// Verify ordering: new session first, then version mismatch, then by line gap
	if sessions[0].SessionID != session1 {
		t.Errorf("expected session1 (new) first, got %s", sessions[0].SessionID)
	}
	if sessions[1].SessionID != session2 {
		t.Errorf("expected session2 (version mismatch) second, got %s", sessions[1].SessionID)
	}
	if sessions[2].SessionID != session3 {
		t.Errorf("expected session3 (larger gap=50) third, got %s", sessions[2].SessionID)
	}
	if sessions[3].SessionID != session4 {
		t.Errorf("expected session4 (smaller gap=30) fourth, got %s", sessions[3].SessionID)
	}
}

// =============================================================================
// Smart Recap Staleness Threshold Tests
// =============================================================================

func TestFindStaleSmartRecapSessions_NewRecap_BelowMinLines_Young_Skipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create session with all regular cards up-to-date, but no smart recap
	user := testutil.CreateTestUser(t, env, "srnewsmall@test.com", "SRNewSmall User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srnewsmall-external-id")

	// Young session (2 min old)
	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-2*time.Minute))

	// 5 lines (below MinInitialLines=10)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 5)
	insertAllCards(t, env, sessionID, 5)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   testThresholds(), // Using test thresholds
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	// Should NOT find - too few lines AND too young
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (below min lines and young), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_NewRecap_MeetsMinLines_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create session with all regular cards up-to-date, no smart recap, meets min lines
	user := testutil.CreateTestUser(t, env, "srnewlarge@test.com", "SRNewLarge User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srnewlarge-external-id")

	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-2*time.Minute))

	// 15 lines (above MinInitialLines=10)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 15)
	insertAllCards(t, env, sessionID, 15)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   testThresholds(),
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	// Should find - meets min_initial_lines
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (meets min lines), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_NewRecap_BelowMinLines_Old_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create session with all regular cards up-to-date, no smart recap
	// Below min lines but old enough (catch-all)
	user := testutil.CreateTestUser(t, env, "srnewold@test.com", "SRNewOld User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srnewold-external-id")

	// Old session (15 min, above MinSessionAge=10m)
	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-15*time.Minute))

	// 5 lines (below MinInitialLines=10)
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 5)
	insertAllCards(t, env, sessionID, 5)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   testThresholds(),
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	// Should find - session is old enough (catch-all)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (old enough), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_Cached_SmallLineGap_Recent_Skipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create session: 105 lines, smart recap at 100 (5 line gap, below 20% threshold)
	user := testutil.CreateTestUser(t, env, "srsmallgap@test.com", "SRSmallGap User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srsmallgap-external-id")

	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-1*time.Hour))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 105)
	insertAllCards(t, env, sessionID, 105)
	// Smart recap at 100 lines, computed 1 minute ago (recent)
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, time.Now().Add(-1*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   testThresholds(),
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	// Should NOT find - line_gap=5 < line_threshold=max(5, 100*0.20)=20, time_gap=1m < time_threshold
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (below thresholds), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_Cached_TimeThresholdMet_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create session: 105 lines, smart recap at 100 (5 line gap), but computed long ago
	user := testutil.CreateTestUser(t, env, "srtimethresh@test.com", "SRTimeThresh User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srtimethresh-external-id")

	// Session started 1 hour ago
	firstSeen := time.Now().Add(-1 * time.Hour)
	setSessionFirstSeen(t, env, sessionID, firstSeen)

	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 105)
	insertAllCards(t, env, sessionID, 105)
	// Smart recap at 100 lines, computed 30 minutes ago (long time_gap)
	// prior_duration = computed_at - first_seen = 30 min
	// time_threshold = max(3m, 30m * 0.20) = max(3m, 6m) = 6m
	// time_gap = now - computed_at = 30m
	// 30m >= 6m ✓
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, time.Now().Add(-30*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   testThresholds(),
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	// Should find - time_gap meets threshold
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (time threshold met), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_Cached_NoLineGap_Skipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create session with smart recap fully up-to-date (line_gap = 0)
	user := testutil.CreateTestUser(t, env, "srnogap@test.com", "SRNoGap User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srnogap-external-id")

	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
	insertAllCards(t, env, sessionID, 100)
	// Smart recap at exactly 100 lines (no gap)
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, time.Now().UTC())

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   testThresholds(),
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	// Should NOT find - line_gap = 0
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (no line gap), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_Cached_LineThresholdMet_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create session: 125 lines, smart recap at 100 (25 line gap, meets 20% threshold)
	user := testutil.CreateTestUser(t, env, "srlinethresh@test.com", "SRLineThresh User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srlinethresh-external-id")

	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-1*time.Hour))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 125)
	insertAllCards(t, env, sessionID, 125)
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, time.Now().Add(-1*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   testThresholds(),
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	// Should find - line_gap=25 >= threshold=20
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (line threshold met), got %d", len(sessions))
	}
}

func TestFindStaleSmartRecapSessions_LargeSession_SmallGap_Skipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a large session: 1050 lines, smart recap at 1000 (50 line gap)
	// line_threshold = max(BaseMinLines=5, 1000*0.20) = 200
	// 50 < 200 - should be skipped (percentage scaling)
	user := testutil.CreateTestUser(t, env, "srlargesmall@test.com", "SRLargeSmall User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "srlargesmall-external-id")

	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-2*time.Hour))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1050)
	insertAllCards(t, env, sessionID, 1050)
	// Smart recap at 1000 lines, computed 5 minutes ago
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 1000, time.Now().Add(-5*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   testThresholds(),
	})

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}

	// Should NOT find - gap=50 < threshold=200
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (large session, small gap), got %d", len(sessions))
	}
}

// =============================================================================
// Independence Tests - Different Thresholds for Regular vs Smart Recap
// =============================================================================

func TestIndependentThresholds_RegularCardsHigherBaseMin_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Test that different BaseMinLines thresholds work independently
	// Regular cards: BaseMinLines=5, Smart recap: BaseMinLines=50
	user := testutil.CreateTestUser(t, env, "indepthresh@test.com", "IndepThresh User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "indepthresh-external-id")

	setSessionFirstSeen(t, env, sessionID, time.Now().Add(-1*time.Hour))
	// Session has 120 lines, all cards/recap at 100
	// Line gap = 20
	// Regular cards: threshold = max(5, 100*0.20) = 20, gap=20 >= 20 ✓
	// Smart recap: threshold = max(50, 100*0.20) = 50, gap=20 < 50 ✗
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 120)
	insertAllCardsWithComputedAt(t, env, sessionID, 100, time.Now().Add(-1*time.Minute))
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 100, time.Now().Add(-1*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	// Use different thresholds for regular vs recap
	regularThresholds := analytics.StalenessThresholds{
		ThresholdPct:    0.20,
		BaseMinLines:    5,  // Low floor
		BaseMinTime:     3 * time.Minute,
		MinInitialLines: 10,
		MinSessionAge:   10 * time.Minute,
	}
	recapThresholds := analytics.StalenessThresholds{
		ThresholdPct:    0.20,
		BaseMinLines:    50, // Higher floor
		BaseMinTime:     15 * time.Minute,
		MinInitialLines: 10,
		MinSessionAge:   10 * time.Minute,
	}

	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: regularThresholds,
		SmartRecapThresholds:   recapThresholds,
	})

	// Regular cards query should find (meets its lower threshold)
	regularSessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}
	if len(regularSessions) != 1 {
		t.Errorf("Regular cards: expected 1 session (threshold met), got %d", len(regularSessions))
	}

	// Smart recap query should NOT find (below its higher threshold)
	recapSessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}
	if len(recapSessions) != 0 {
		t.Errorf("Smart recap: expected 0 sessions (below threshold), got %d", len(recapSessions))
	}
}

func TestIndependentThresholds_SmartRecapTimeThreshold_RegularFresh(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Test: Regular cards are fresh (up-to-date), but smart recap is stale due to time threshold
	// This demonstrates the independence: recap query only runs when regular cards are fresh
	user := testutil.CreateTestUser(t, env, "indeptime@test.com", "IndepTime User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "indeptime-external-id")

	firstSeen := time.Now().Add(-1 * time.Hour)
	setSessionFirstSeen(t, env, sessionID, firstSeen)

	// Session has 100 lines, regular cards at 100 (fully fresh)
	// Smart recap at 95 lines (small line gap=5, won't meet 20% threshold via lines)
	// But smart recap computed 30 min ago, so time gap triggers
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
	insertAllCardsWithComputedAt(t, env, sessionID, 100, time.Now().Add(-1*time.Minute)) // Fresh
	// Smart recap has small gap (100-95=5) but old computed_at
	// prior_duration = 30m, time_gap = 30m
	// time_threshold = max(3m, 30m*0.20) = 6m, 30m >= 6m ✓
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 95, time.Now().Add(-30*time.Minute))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	thresholds := testThresholds()

	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: thresholds,
		SmartRecapThresholds:   thresholds,
	})

	// Regular cards query should NOT find (cards are up-to-date)
	regularSessions, err := precomputer.FindStaleSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}
	if len(regularSessions) != 0 {
		t.Errorf("Regular cards: expected 0 sessions (fresh), got %d", len(regularSessions))
	}

	// Smart recap query should find (time threshold met)
	recapSessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions failed: %v", err)
	}
	if len(recapSessions) != 1 {
		t.Errorf("Smart recap: expected 1 session (time threshold met), got %d", len(recapSessions))
	}
}

// =============================================================================
// Precompute Quota Reset Integration Tests
// =============================================================================

// TestPrecomputeQuotaReset_ResetsOnNewMonth verifies that ResetQuotaIfNewMonth
// correctly resets quota when the month has changed. This covers the case where
// a user's quota was exhausted in a prior month and the worker is the first
// path to touch their quota in the new month.
func TestPrecomputeQuotaReset_ResetsOnNewMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "quotareset@test.com", "QuotaReset User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Create a quota record with count at limit, reset_at set to last month
	_, err := conn.ExecContext(ctx, `
		INSERT INTO smart_recap_quota (user_id, compute_count, last_compute_at, quota_reset_at)
		VALUES ($1, 100, NOW() - INTERVAL '35 days', NOW() - INTERVAL '35 days')
	`, user.ID)
	if err != nil {
		t.Fatalf("failed to insert quota: %v", err)
	}

	// Verify quota is at 100 before reset
	var countBefore int
	err = conn.QueryRowContext(ctx, `SELECT compute_count FROM smart_recap_quota WHERE user_id = $1`, user.ID).Scan(&countBefore)
	if err != nil {
		t.Fatalf("failed to read quota: %v", err)
	}
	if countBefore != 100 {
		t.Fatalf("expected compute_count=100 before reset, got %d", countBefore)
	}

	// Run the same reset function used in precomputeSmartRecap
	err = analytics.ResetQuotaIfNewMonth(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("ResetQuotaIfNewMonth failed: %v", err)
	}

	// Verify quota was reset to 0
	var countAfter int
	err = conn.QueryRowContext(ctx, `SELECT compute_count FROM smart_recap_quota WHERE user_id = $1`, user.ID).Scan(&countAfter)
	if err != nil {
		t.Fatalf("failed to read quota after reset: %v", err)
	}
	if countAfter != 0 {
		t.Errorf("expected compute_count=0 after reset, got %d", countAfter)
	}
}

// TestPrecomputeQuotaReset_NoResetSameMonth verifies that ResetQuotaIfNewMonth
// does NOT reset quota when the quota was already reset in the current month.
func TestPrecomputeQuotaReset_NoResetSameMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "quotanoreset@test.com", "QuotaNoReset User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Create a quota record with count at 50, reset_at set to today (same month)
	_, err := conn.ExecContext(ctx, `
		INSERT INTO smart_recap_quota (user_id, compute_count, last_compute_at, quota_reset_at)
		VALUES ($1, 50, NOW(), NOW())
	`, user.ID)
	if err != nil {
		t.Fatalf("failed to insert quota: %v", err)
	}

	err = analytics.ResetQuotaIfNewMonth(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("ResetQuotaIfNewMonth failed: %v", err)
	}

	// Verify quota was NOT reset (still 50)
	var countAfter int
	err = conn.QueryRowContext(ctx, `SELECT compute_count FROM smart_recap_quota WHERE user_id = $1`, user.ID).Scan(&countAfter)
	if err != nil {
		t.Fatalf("failed to read quota after reset attempt: %v", err)
	}
	if countAfter != 50 {
		t.Errorf("expected compute_count=50 (no reset same month), got %d", countAfter)
	}
}

// TestPrecomputeQuotaReset_NullResetAt_UsesLastComputeAt verifies the fallback
// path where quota_reset_at is NULL and the reset uses last_compute_at instead.
func TestPrecomputeQuotaReset_NullResetAt_UsesLastComputeAt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "quotanullreset@test.com", "QuotaNullReset User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Create a quota record with no reset_at but last_compute_at in previous month
	_, err := conn.ExecContext(ctx, `
		INSERT INTO smart_recap_quota (user_id, compute_count, last_compute_at, quota_reset_at)
		VALUES ($1, 75, NOW() - INTERVAL '35 days', NULL)
	`, user.ID)
	if err != nil {
		t.Fatalf("failed to insert quota: %v", err)
	}

	err = analytics.ResetQuotaIfNewMonth(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("ResetQuotaIfNewMonth failed: %v", err)
	}

	// Verify quota was reset to 0
	var countAfter int
	err = conn.QueryRowContext(ctx, `SELECT compute_count FROM smart_recap_quota WHERE user_id = $1`, user.ID).Scan(&countAfter)
	if err != nil {
		t.Fatalf("failed to read quota after reset: %v", err)
	}
	if countAfter != 0 {
		t.Errorf("expected compute_count=0 after reset (null reset_at fallback), got %d", countAfter)
	}
}
