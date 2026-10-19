package analytics_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// failAttempts drives n claim+failure cycles for (sessionID, bucket), the way
// the worker does, and returns the last RecordFailure result.
func failAttempts(t *testing.T, store *analytics.Store, sessionID, bucket string, n int) (int, bool) {
	t.Helper()
	ctx := context.Background()
	var attempts int
	var quarantined bool
	for i := range n {
		if err := store.ClaimAttempt(ctx, sessionID, bucket); err != nil {
			t.Fatalf("ClaimAttempt #%d: %v", i+1, err)
		}
		var err error
		attempts, quarantined, err = store.RecordFailure(ctx, sessionID, bucket, fmt.Sprintf("boom %d", i+1))
		if err != nil {
			t.Fatalf("RecordFailure #%d: %v", i+1, err)
		}
	}
	return attempts, quarantined
}

type failureRow struct {
	exists      bool
	attempts    int
	lastError   *string
	inProgress  bool
	quarantined bool
}

func readFailureRow(t *testing.T, env *testutil.TestEnvironment, sessionID, bucket string) failureRow {
	t.Helper()
	var r failureRow
	err := env.DB.QueryRow(env.Ctx, `
		SELECT attempts, last_error, in_progress_since IS NOT NULL, quarantined_at IS NOT NULL
		FROM session_precompute_failures WHERE session_id = $1 AND bucket = $2
	`, sessionID, bucket).Scan(&r.attempts, &r.lastError, &r.inProgress, &r.quarantined)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return failureRow{}
		}
		t.Fatalf("read failure row: %v", err)
	}
	r.exists = true
	return r
}

// newStaleSession creates a session with a transcript and no cards, so it is a
// category-1 (missing cards) candidate for FindStaleSessions.
func newStaleSession(t *testing.T, env *testutil.TestEnvironment, userID int64, ext string) string {
	t.Helper()
	id := testutil.CreateTestSession(t, env, userID, ext)
	testutil.CreateTestSyncFile(t, env, id, "transcript.jsonl", "transcript", 100)
	return id
}

func sessionIDs(sessions []analytics.StaleSession) map[string]bool {
	out := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		out[s.SessionID] = true
	}
	return out
}

func TestPrecomputeFailures_QuarantinesExactlyAtFifthConsecutiveFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "q5@test.com", "Q5")
	sessionID := newStaleSession(t, env, user.ID, "q5-ext")

	store := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, defaultTestConfig())

	attempts, quarantined := failAttempts(t, store, sessionID, analytics.BucketRegularCards, analytics.QuarantineAfterAttempts-1)
	if attempts != 4 || quarantined {
		t.Fatalf("after 4 failures: attempts=%d quarantined=%v, want 4/false", attempts, quarantined)
	}
	found, err := precomputer.FindStaleSessions(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindStaleSessions: %v", err)
	}
	if !sessionIDs(found)[sessionID] {
		t.Fatal("a session with 4 failures must still be returned by FindStaleSessions")
	}

	attempts, quarantined = failAttempts(t, store, sessionID, analytics.BucketRegularCards, 1)
	if attempts != 5 || !quarantined {
		t.Fatalf("after 5th failure: attempts=%d quarantined=%v, want 5/true", attempts, quarantined)
	}
	found, err = precomputer.FindStaleSessions(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindStaleSessions: %v", err)
	}
	if sessionIDs(found)[sessionID] {
		t.Fatal("a session quarantined after 5 failures must be excluded from FindStaleSessions")
	}

	row := readFailureRow(t, env, sessionID, analytics.BucketRegularCards)
	if row.inProgress {
		t.Error("RecordFailure must clear in_progress_since")
	}
	if row.lastError == nil || *row.lastError != "boom 1" {
		t.Errorf("last_error = %v, want the latest failure message", row.lastError)
	}
}

func TestFindStaleSessions_QuarantinedSessionsDoNotFillLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "starve@test.com", "Starve")
	store := analytics.NewStore(env.DB.Conn())

	const n = 3
	for i := range n {
		id := newStaleSession(t, env, user.ID, fmt.Sprintf("failing-%d", i))
		failAttempts(t, store, id, analytics.BucketRegularCards, analytics.QuarantineAfterAttempts)
	}
	// The healthy session has a valid but lagging card set (category 3), so
	// without SQL exclusion the three missing-card (category 1) sessions would
	// sort ahead of it and fill the limit.
	healthy := testutil.CreateTestSession(t, env, user.ID, "healthy")
	testutil.CreateTestSyncFile(t, env, healthy, "transcript.jsonl", "transcript", 200)
	insertAllCards(t, env, healthy, 100)

	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, defaultTestConfig())
	found, err := precomputer.FindStaleSessions(context.Background(), n)
	if err != nil {
		t.Fatalf("FindStaleSessions: %v", err)
	}
	if len(found) != 1 || found[0].SessionID != healthy {
		t.Fatalf("FindStaleSessions(limit=%d) = %+v, want only the healthy session", n, found)
	}
}

func TestPrecomputeFailures_QuarantineIsPerBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "perbucket@test.com", "PerBucket")
	sessionID := newStaleSession(t, env, user.ID, "perbucket")
	store := analytics.NewStore(env.DB.Conn())

	failAttempts(t, store, sessionID, analytics.BucketSmartRecap, analytics.QuarantineAfterAttempts)

	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, defaultTestConfig())
	found, err := precomputer.FindStaleSessions(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindStaleSessions: %v", err)
	}
	if !sessionIDs(found)[sessionID] {
		t.Fatal("a smart_recap quarantine must not exclude the session from the regular_cards bucket")
	}
}

func TestFindStaleSmartRecapSessions_ExcludesQuarantined(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "srq@test.com", "SRQ")
	store := analytics.NewStore(env.DB.Conn())

	quarantined := testutil.CreateTestSession(t, env, user.ID, "srq-bad")
	eligible := testutil.CreateTestSession(t, env, user.ID, "srq-good")
	for _, id := range []string{quarantined, eligible} {
		testutil.CreateTestSyncFile(t, env, id, "transcript.jsonl", "transcript", 100)
		insertAllCards(t, env, id, 100)
	}
	failAttempts(t, store, quarantined, analytics.BucketSmartRecap, analytics.QuarantineAfterAttempts)

	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, analytics.PrecomputeConfig{
		SmartRecapEnabled:      true,
		SmartRecapAPIKey:       "test-key",
		SmartRecapModel:        "test-model",
		SmartRecapQuota:        100,
		LockTimeoutSeconds:     60,
		RegularCardsThresholds: analytics.DefaultRegularCardsThresholds(),
		SmartRecapThresholds:   analytics.DefaultSmartRecapThresholds(),
	})
	found, err := precomputer.FindStaleSmartRecapSessions(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindStaleSmartRecapSessions: %v", err)
	}
	ids := sessionIDs(found)
	if ids[quarantined] {
		t.Error("quarantined smart_recap session must be excluded")
	}
	if !ids[eligible] {
		t.Error("eligible smart_recap session must still be returned")
	}
}

func TestFindStaleSearchIndexSessions_ExcludesQuarantined(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "siq@test.com", "SIQ")
	store := analytics.NewStore(env.DB.Conn())

	quarantined := testutil.CreateTestSession(t, env, user.ID, "siq-bad")
	eligible := testutil.CreateTestSession(t, env, user.ID, "siq-good")
	for _, id := range []string{quarantined, eligible} {
		testutil.CreateTestSyncFile(t, env, id, "transcript.jsonl", "transcript", 100)
		insertAllCards(t, env, id, 100)
	}
	failAttempts(t, store, quarantined, analytics.BucketSearchIndex, analytics.QuarantineAfterAttempts)

	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, store, defaultTestConfig())
	found, err := precomputer.FindStaleSearchIndexSessions(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindStaleSearchIndexSessions: %v", err)
	}
	ids := sessionIDs(found)
	if ids[quarantined] {
		t.Error("quarantined search_index session must be excluded")
	}
	if !ids[eligible] {
		t.Error("eligible search_index session must still be returned")
	}
}

func TestFindTitleRecomputeSessions_ExcludesQuarantined(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "tq@test.com", "TQ")
	store := analytics.NewStore(env.DB.Conn())

	quarantined := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
	eligible := seedTitleSession(t, env, user.ID, models.ProviderCodex, nil, true)
	failAttempts(t, store, quarantined.SessionID, analytics.BucketSessionTitle, analytics.QuarantineAfterAttempts)

	found, err := titlePrecomputer(env, env.Storage).FindTitleRecomputeSessions(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindTitleRecomputeSessions: %v", err)
	}
	ids := sessionIDs(found)
	if ids[quarantined.SessionID] {
		t.Error("quarantined session_title session must be excluded")
	}
	if !ids[eligible.SessionID] {
		t.Error("eligible session_title session must still be returned")
	}
}

func TestPrecomputeFailures_SuccessDeletesRowAndResetsAttempts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "reset@test.com", "Reset")
	sessionID := newStaleSession(t, env, user.ID, "reset")
	store := analytics.NewStore(env.DB.Conn())
	ctx := context.Background()

	failAttempts(t, store, sessionID, analytics.BucketRegularCards, 3)
	if err := store.ClaimAttempt(ctx, sessionID, analytics.BucketRegularCards); err != nil {
		t.Fatalf("ClaimAttempt: %v", err)
	}
	if err := store.RecordSuccess(ctx, sessionID, analytics.BucketRegularCards); err != nil {
		t.Fatalf("RecordSuccess: %v", err)
	}
	if row := readFailureRow(t, env, sessionID, analytics.BucketRegularCards); row.exists {
		t.Fatalf("success must delete the failure row, got %+v", row)
	}

	// The count starts over: 4 more failures still don't quarantine.
	if _, quarantined := failAttempts(t, store, sessionID, analytics.BucketRegularCards, 4); quarantined {
		t.Error("attempts must reset on success; 4 failures after a success must not quarantine")
	}
}

func TestPrecomputeFailures_ReleaseAttemptLeavesAttemptsUnchanged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "release@test.com", "Release")
	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())

	t.Run("fresh claim released leaves no row", func(t *testing.T) {
		sessionID := newStaleSession(t, env, user.ID, "release-fresh")
		if err := store.ClaimAttempt(ctx, sessionID, analytics.BucketSmartRecap); err != nil {
			t.Fatalf("ClaimAttempt: %v", err)
		}
		row := readFailureRow(t, env, sessionID, analytics.BucketSmartRecap)
		if !row.exists || !row.inProgress || row.attempts != 0 {
			t.Fatalf("claim must create an in-progress row with 0 attempts, got %+v", row)
		}
		if err := store.ReleaseAttempt(ctx, sessionID, analytics.BucketSmartRecap); err != nil {
			t.Fatalf("ReleaseAttempt: %v", err)
		}
		if row := readFailureRow(t, env, sessionID, analytics.BucketSmartRecap); row.exists {
			t.Fatalf("releasing a 0-attempt claim must delete the row, got %+v", row)
		}
	})

	t.Run("claim released after failures keeps the count", func(t *testing.T) {
		sessionID := newStaleSession(t, env, user.ID, "release-failed")
		failAttempts(t, store, sessionID, analytics.BucketSmartRecap, 2)
		if err := store.ClaimAttempt(ctx, sessionID, analytics.BucketSmartRecap); err != nil {
			t.Fatalf("ClaimAttempt: %v", err)
		}
		if err := store.ReleaseAttempt(ctx, sessionID, analytics.BucketSmartRecap); err != nil {
			t.Fatalf("ReleaseAttempt: %v", err)
		}
		row := readFailureRow(t, env, sessionID, analytics.BucketSmartRecap)
		if !row.exists || row.attempts != 2 || row.inProgress || row.quarantined {
			t.Fatalf("release must keep attempts=2 and clear in_progress, got %+v", row)
		}
	})
}

func TestPrecomputeFailures_RecordFailureTruncatesLastErrorTo1KB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "trunc@test.com", "Trunc")
	sessionID := newStaleSession(t, env, user.ID, "trunc")
	store := analytics.NewStore(env.DB.Conn())

	// RecordFailure without a prior claim must still record (an on-view success
	// may have deleted the claim row mid-attempt).
	if _, _, err := store.RecordFailure(context.Background(), sessionID, analytics.BucketRegularCards, strings.Repeat("é", 2000)); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	row := readFailureRow(t, env, sessionID, analytics.BucketRegularCards)
	if !row.exists || row.attempts != 1 || row.lastError == nil {
		t.Fatalf("RecordFailure without a claim must insert attempts=1, got %+v", row)
	}
	if n := len(*row.lastError); n > 1024 || n < 1000 {
		t.Errorf("last_error length = %d bytes, want truncated to at most 1024", n)
	}
}

func TestSweepCrashedAttempts_CountsUnsettledClaimsAsFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "sweep@test.com", "Sweep")
	store := analytics.NewStore(env.DB.Conn())
	ctx := context.Background()

	nearlyQuarantined := newStaleSession(t, env, user.ID, "sweep-4")
	failAttempts(t, store, nearlyQuarantined, analytics.BucketRegularCards, 4)
	if err := store.ClaimAttempt(ctx, nearlyQuarantined, analytics.BucketRegularCards); err != nil {
		t.Fatalf("ClaimAttempt: %v", err)
	}
	firstAttempt := newStaleSession(t, env, user.ID, "sweep-0")
	if err := store.ClaimAttempt(ctx, firstAttempt, analytics.BucketSearchIndex); err != nil {
		t.Fatalf("ClaimAttempt: %v", err)
	}
	settled := newStaleSession(t, env, user.ID, "sweep-settled")
	failAttempts(t, store, settled, analytics.BucketRegularCards, 2)

	swept, err := store.SweepCrashedAttempts(ctx)
	if err != nil {
		t.Fatalf("SweepCrashedAttempts: %v", err)
	}
	if swept != 2 {
		t.Errorf("swept = %d, want 2 (only in-progress rows)", swept)
	}

	row := readFailureRow(t, env, nearlyQuarantined, analytics.BucketRegularCards)
	if row.attempts != 5 || !row.quarantined || row.inProgress {
		t.Errorf("crashed 5th attempt: got %+v, want attempts=5 quarantined", row)
	}
	if row.lastError == nil || *row.lastError != "worker exited during attempt" {
		t.Errorf("crashed attempt last_error = %v, want %q", row.lastError, "worker exited during attempt")
	}
	row = readFailureRow(t, env, firstAttempt, analytics.BucketSearchIndex)
	if row.attempts != 1 || row.quarantined || row.inProgress {
		t.Errorf("crashed 1st attempt: got %+v, want attempts=1 not quarantined", row)
	}
	row = readFailureRow(t, env, settled, analytics.BucketRegularCards)
	if row.attempts != 2 {
		t.Errorf("settled row must be untouched by the sweep, got %+v", row)
	}
}

func TestPrecomputeFailures_ListAndReleaseQuarantined(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "list@test.com", "List")
	store := analytics.NewStore(env.DB.Conn())
	ctx := context.Background()

	a := newStaleSession(t, env, user.ID, "list-a")
	b := testutil.CreateTestSessionWithProvider(t, env, user.ID, "list-b", models.ProviderCodex)
	notYet := newStaleSession(t, env, user.ID, "list-notyet")
	failAttempts(t, store, a, analytics.BucketRegularCards, analytics.QuarantineAfterAttempts)
	failAttempts(t, store, a, analytics.BucketSmartRecap, analytics.QuarantineAfterAttempts)
	failAttempts(t, store, b, analytics.BucketSearchIndex, analytics.QuarantineAfterAttempts)
	failAttempts(t, store, notYet, analytics.BucketRegularCards, 2)

	list, err := store.ListQuarantined(ctx)
	if err != nil {
		t.Fatalf("ListQuarantined: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListQuarantined returned %d rows, want 3 (quarantined only): %+v", len(list), list)
	}
	var codexRow *analytics.QuarantinedSession
	for i := range list {
		if list[i].SessionID == notYet {
			t.Error("a not-yet-quarantined row must not be listed")
		}
		if list[i].SessionID == b {
			codexRow = &list[i]
		}
	}
	if codexRow == nil {
		t.Fatal("codex quarantined row missing")
	}
	if codexRow.Provider != models.ProviderCodex || codexRow.ExternalID != "list-b" || codexRow.UserID != user.ID ||
		codexRow.Bucket != analytics.BucketSearchIndex || codexRow.Attempts != 5 || codexRow.LastError != "boom 5" || codexRow.QuarantinedAt.IsZero() {
		t.Errorf("codex row = %+v", *codexRow)
	}

	// Releasing a non-quarantined row is a no-op.
	if n, err := store.ReleaseQuarantined(ctx, notYet, analytics.BucketRegularCards); err != nil || n != 0 {
		t.Errorf("ReleaseQuarantined(not quarantined) = %d, %v; want 0, nil", n, err)
	}
	if n, err := store.ReleaseQuarantined(ctx, a, analytics.BucketRegularCards); err != nil || n != 1 {
		t.Fatalf("ReleaseQuarantined = %d, %v; want 1, nil", n, err)
	}
	if row := readFailureRow(t, env, a, analytics.BucketRegularCards); row.exists {
		t.Error("released row must be deleted")
	}
	if row := readFailureRow(t, env, a, analytics.BucketSmartRecap); !row.quarantined {
		t.Error("release must only touch the named bucket")
	}

	if n, err := store.ReleaseAllQuarantined(ctx); err != nil || n != 2 {
		t.Fatalf("ReleaseAllQuarantined = %d, %v; want 2, nil", n, err)
	}
	if row := readFailureRow(t, env, notYet, analytics.BucketRegularCards); !row.exists || row.attempts != 2 {
		t.Error("release-all must not delete non-quarantined rows")
	}
	if list, _ := store.ListQuarantined(ctx); len(list) != 0 {
		t.Errorf("after release-all, %d quarantined rows remain", len(list))
	}
}

func TestPrecomputeFailures_DeletedSessionCascades(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "cascade@test.com", "Cascade")
	sessionID := newStaleSession(t, env, user.ID, "cascade")
	store := analytics.NewStore(env.DB.Conn())
	failAttempts(t, store, sessionID, analytics.BucketRegularCards, analytics.QuarantineAfterAttempts)

	if _, err := env.DB.Exec(env.Ctx, `DELETE FROM sessions WHERE id = $1`, sessionID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if row := readFailureRow(t, env, sessionID, analytics.BucketRegularCards); row.exists {
		t.Error("deleting a session must cascade to its failure rows")
	}
}
