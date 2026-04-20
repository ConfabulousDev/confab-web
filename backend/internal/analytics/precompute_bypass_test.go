package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// seedFullQuotaForUser sets the user's smart recap quota to `count` for the current month.
func seedFullQuotaForUser(t *testing.T, env *testutil.TestEnvironment, userID int64, count int) {
	t.Helper()
	_, err := env.DB.Exec(env.Ctx, `
		INSERT INTO smart_recap_quota (user_id, compute_count, quota_month, last_compute_at)
		VALUES ($1, $2, TO_CHAR(NOW() AT TIME ZONE 'UTC', 'YYYY-MM'), NOW())
	`, userID, count)
	if err != nil {
		t.Fatalf("seed quota: %v", err)
	}
}

// seedAdminCardInvalidation inserts an audit row for the given session + card types.
func seedAdminCardInvalidation(t *testing.T, env *testutil.TestEnvironment, sessionID string, adminUserID int64, cardTypes []string, invalidatedAt time.Time) uuid.UUID {
	t.Helper()
	corr := uuid.New()
	_, err := env.DB.Exec(env.Ctx, `
		INSERT INTO admin_card_invalidations (
			session_id, admin_user_id, card_types, correlation_id, invalidated_at, reason
		) VALUES ($1, $2, $3, $4, $5, 'test')
	`, sessionID, adminUserID, pq.Array(cardTypes), corr, invalidatedAt)
	if err != nil {
		t.Fatalf("seed admin_card_invalidations: %v", err)
	}
	return corr
}

// bypassTestConfig returns a PrecomputeConfig with smart recap enabled and a
// low quota so tests can easily saturate it.
func bypassTestConfig(quota int) analytics.PrecomputeConfig {
	return analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		AnthropicAPIKey:        "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        quota,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
	}
}

// seedSessionReadyForSmartRecap creates a session with all 7 regular cards up-to-date
// and NO smart_recap card (simulating the post-DELETE state after admin invalidation).
// All regular cards will have up_to_line = totalLines so the "regular cards fresh" guard
// in FindStaleSmartRecapSessions is satisfied.
func seedSessionReadyForSmartRecap(t *testing.T, env *testutil.TestEnvironment, userID int64, totalLines int64) string {
	t.Helper()
	sessionID := testutil.CreateTestSession(t, env, userID, "bypass-"+uuid.NewString()[:8])
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", int(totalLines))
	insertAllCards(t, env, sessionID, totalLines)
	return sessionID
}

func TestBypass_OverQuota_NoInvalidation_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "overquota@test.com", "Over Quota")
	_ = seedSessionReadyForSmartRecap(t, env, user.ID, 1000)

	// User is fully saturated on quota, no invalidation.
	seedFullQuotaForUser(t, env, user.ID, 5)

	store := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, bypassTestConfig(5))

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (over quota, no invalidation), got %d", len(sessions))
	}
}

func TestBypass_OverQuota_WithInvalidation_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "overquota2@test.com", "Over Quota")
	sessionID := seedSessionReadyForSmartRecap(t, env, user.ID, 1000)

	seedFullQuotaForUser(t, env, user.ID, 5)
	// Admin invalidation covering smart_recap — smart_recap card is already absent (never inserted).
	seedAdminCardInvalidation(t, env, sessionID, admin.ID, []string{"session_card_smart_recap"}, time.Now().UTC())

	store := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, bypassTestConfig(5))

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (bypass via admin_card_invalidations), got %d", len(sessions))
	}
	if sessions[0].SessionID != sessionID {
		t.Errorf("expected session %s, got %s", sessionID, sessions[0].SessionID)
	}
}

func TestBypass_OverQuota_InvalidationDoesNotIncludeSmartRecap_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "overquota3@test.com", "Over Quota")
	sessionID := seedSessionReadyForSmartRecap(t, env, user.ID, 1000)

	seedFullQuotaForUser(t, env, user.ID, 5)
	// Invalidation for tokens only — smart recap bypass should NOT trigger.
	seedAdminCardInvalidation(t, env, sessionID, admin.ID, []string{"session_card_tokens"}, time.Now().UTC())

	store := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, bypassTestConfig(5))

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (invalidation does not cover smart_recap), got %d", len(sessions))
	}
}

func TestBypass_InvalidationConsumedByNewerRecap_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "consumed@test.com", "Consumed")
	sessionID := seedSessionReadyForSmartRecap(t, env, user.ID, 1000)

	// Admin invalidation happened an hour ago…
	past := time.Now().UTC().Add(-1 * time.Hour)
	seedAdminCardInvalidation(t, env, sessionID, admin.ID, []string{"session_card_smart_recap"}, past)

	// …and a fresh smart recap card was computed AFTER the invalidation (consumed).
	insertSmartRecapCard(t, env, sessionID, analytics.SmartRecapCardVersion, 1000, time.Now().UTC())

	seedFullQuotaForUser(t, env, user.ID, 5)

	store := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, bypassTestConfig(5))

	sessions, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (invalidation consumed by newer recap), got %d", len(sessions))
	}
}
