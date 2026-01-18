package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestSmartRecapLock_Acquire(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "lock@test.com", "Lock User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-lock")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// First acquire should succeed
	acquired, err := store.AcquireSmartRecapLock(ctx, sessionID, 60)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock failed: %v", err)
	}
	if !acquired {
		t.Error("expected lock to be acquired on first attempt")
	}

	// Second acquire should fail (lock already held)
	acquired2, err := store.AcquireSmartRecapLock(ctx, sessionID, 60)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock (second) failed: %v", err)
	}
	if acquired2 {
		t.Error("expected lock to NOT be acquired on second attempt")
	}
}

func TestSmartRecapLock_AcquireAfterClear(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "lockclear@test.com", "LockClear User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-lock-clear")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// Acquire lock
	acquired, err := store.AcquireSmartRecapLock(ctx, sessionID, 60)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock failed: %v", err)
	}
	if !acquired {
		t.Error("expected lock to be acquired")
	}

	// Clear the lock
	err = store.ClearSmartRecapLock(ctx, sessionID)
	if err != nil {
		t.Fatalf("ClearSmartRecapLock failed: %v", err)
	}

	// Should be able to acquire again
	acquired2, err := store.AcquireSmartRecapLock(ctx, sessionID, 60)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock (after clear) failed: %v", err)
	}
	if !acquired2 {
		t.Error("expected lock to be acquired after clearing")
	}
}

func TestSmartRecapLock_StaleLockTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "stalelock@test.com", "StaleLock User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-stale-lock")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// Acquire lock with 1 second timeout
	acquired, err := store.AcquireSmartRecapLock(ctx, sessionID, 1)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock failed: %v", err)
	}
	if !acquired {
		t.Error("expected lock to be acquired")
	}

	// Immediately try again - should fail
	acquired2, err := store.AcquireSmartRecapLock(ctx, sessionID, 1)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock (immediate) failed: %v", err)
	}
	if acquired2 {
		t.Error("expected lock to NOT be acquired immediately")
	}

	// Wait for lock to become stale
	time.Sleep(2 * time.Second)

	// Now should be able to acquire (stale lock)
	acquired3, err := store.AcquireSmartRecapLock(ctx, sessionID, 1)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock (after timeout) failed: %v", err)
	}
	if !acquired3 {
		t.Error("expected lock to be acquired after stale timeout")
	}
}

func TestSmartRecapLock_DifferentSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "diffsessions@test.com", "DiffSessions User")
	sessionID1 := testutil.CreateTestSession(t, env, user.ID, "test-session-1")
	sessionID2 := testutil.CreateTestSession(t, env, user.ID, "test-session-2")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// Acquire lock on session 1
	acquired1, err := store.AcquireSmartRecapLock(ctx, sessionID1, 60)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock (session 1) failed: %v", err)
	}
	if !acquired1 {
		t.Error("expected lock to be acquired on session 1")
	}

	// Should be able to acquire lock on session 2 (different session)
	acquired2, err := store.AcquireSmartRecapLock(ctx, sessionID2, 60)
	if err != nil {
		t.Fatalf("AcquireSmartRecapLock (session 2) failed: %v", err)
	}
	if !acquired2 {
		t.Error("expected lock to be acquired on session 2 (different session)")
	}
}

func TestSmartRecapCard_UpsertAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "upsert@test.com", "Upsert User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-upsert")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// Create a card record
	card := &analytics.SmartRecapCardRecord{
		SessionID:                 sessionID,
		Version:                   analytics.SmartRecapCardVersion,
		ComputedAt:                time.Now().UTC(),
		UpToLine:                  100,
		Recap:                     "Test recap of the session",
		WentWell:                  []string{"Good thing 1", "Good thing 2"},
		WentBad:                   []string{"Bad thing 1"},
		HumanSuggestions:          []string{"Human suggestion"},
		EnvironmentSuggestions:    []string{},
		DefaultContextSuggestions: []string{"Add to CLAUDE.md"},
		ModelUsed:                 "claude-haiku-4-5-20251101",
		InputTokens:               1000,
		OutputTokens:              200,
		GenerationTimeMs:          intPtr(1500),
	}

	// Upsert
	err := store.UpsertSmartRecapCard(ctx, card)
	if err != nil {
		t.Fatalf("UpsertSmartRecapCard failed: %v", err)
	}

	// Get
	retrieved, err := store.GetSmartRecapCard(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSmartRecapCard failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected card to be retrieved")
	}

	// Verify fields
	if retrieved.Recap != card.Recap {
		t.Errorf("Recap = %q, want %q", retrieved.Recap, card.Recap)
	}
	if len(retrieved.WentWell) != 2 {
		t.Errorf("WentWell length = %d, want 2", len(retrieved.WentWell))
	}
	if len(retrieved.WentBad) != 1 {
		t.Errorf("WentBad length = %d, want 1", len(retrieved.WentBad))
	}
	if retrieved.ModelUsed != card.ModelUsed {
		t.Errorf("ModelUsed = %q, want %q", retrieved.ModelUsed, card.ModelUsed)
	}
	if retrieved.InputTokens != card.InputTokens {
		t.Errorf("InputTokens = %d, want %d", retrieved.InputTokens, card.InputTokens)
	}

	// Lock should be cleared after upsert
	if retrieved.ComputingStartedAt != nil {
		t.Error("expected ComputingStartedAt to be nil after upsert")
	}
}

func TestSmartRecapCard_GetNonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "nonexistent@test.com", "NonExistent User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-nonexistent")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// Get non-existent card
	card, err := store.GetSmartRecapCard(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSmartRecapCard failed: %v", err)
	}
	if card != nil {
		t.Error("expected nil card for non-existent session")
	}
}

func intPtr(i int) *int {
	return &i
}
