package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestSmartRecapQuota_GetOrCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "quota@test.com", "Quota User")
	ctx := context.Background()

	// First call should create the quota
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", quota.UserID, user.ID)
	}
	if quota.ComputeCount != 0 {
		t.Errorf("ComputeCount = %d, want 0", quota.ComputeCount)
	}

	// Second call should return the same quota
	quota2, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota (second) failed: %v", err)
	}
	if quota2.UserID != quota.UserID {
		t.Errorf("UserID mismatch: %d vs %d", quota2.UserID, quota.UserID)
	}
}

func TestSmartRecapQuota_Increment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "increment@test.com", "Increment User")
	ctx := context.Background()

	// Create quota first
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}

	// Increment
	err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
	}

	// Check the count
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != 1 {
		t.Errorf("ComputeCount = %d, want 1", quota.ComputeCount)
	}
	if quota.LastComputeAt == nil {
		t.Error("LastComputeAt should be set after increment")
	}

	// Increment again
	err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("IncrementSmartRecapQuota (second) failed: %v", err)
	}

	quota, err = env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota (second) failed: %v", err)
	}
	if quota.ComputeCount != 2 {
		t.Errorf("ComputeCount = %d, want 2", quota.ComputeCount)
	}
}

func TestSmartRecapQuota_IncrementNoRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "norecord@test.com", "NoRecord User")
	ctx := context.Background()

	// Try to increment without creating first
	err := env.DB.IncrementSmartRecapQuota(ctx, user.ID)
	if err == nil {
		t.Error("expected error when incrementing non-existent quota")
	}
}

func TestSmartRecapQuota_CreateOnFirstAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "firstaccess@test.com", "FirstAccess User")
	ctx := context.Background()

	// First access should create with zero count
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota == nil {
		t.Fatal("expected quota to be created")
	}
	if quota.ComputeCount != 0 {
		t.Errorf("ComputeCount = %d, want 0 for new record", quota.ComputeCount)
	}
}

func TestSmartRecapQuota_ResetNotNeeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "resetnotneeded@test.com", "ResetNotNeeded User")
	ctx := context.Background()

	// Create quota
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}

	// Increment
	err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
	}

	// Reset should not be needed (same month)
	wasReset, err := env.DB.ResetSmartRecapQuotaIfNeeded(ctx, user.ID)
	if err != nil {
		t.Fatalf("ResetSmartRecapQuotaIfNeeded failed: %v", err)
	}
	if wasReset {
		t.Error("expected no reset for same month")
	}

	// Count should still be 1
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != 1 {
		t.Errorf("ComputeCount = %d, want 1", quota.ComputeCount)
	}
}

func TestSmartRecapQuota_ResetOnNewMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "resetmonth@test.com", "ResetMonth User")
	ctx := context.Background()

	// Create quota and increment
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}

	err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
	}

	// Verify count is 1
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != 1 {
		t.Errorf("ComputeCount = %d, want 1", quota.ComputeCount)
	}

	// Simulate next month
	nextMonth := time.Now().UTC().AddDate(0, 1, 0)

	// Reset should be needed (new month)
	wasReset, err := env.DB.ResetSmartRecapQuotaIfNeededAt(ctx, user.ID, nextMonth)
	if err != nil {
		t.Fatalf("ResetSmartRecapQuotaIfNeededAt failed: %v", err)
	}
	if !wasReset {
		t.Error("expected reset for new month")
	}

	// Count should be 0 after reset
	quota, err = env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != 0 {
		t.Errorf("ComputeCount = %d, want 0 after reset", quota.ComputeCount)
	}
	if quota.QuotaResetAt == nil {
		t.Error("QuotaResetAt should be set after reset")
	}
}

func TestSmartRecapQuota_ResetOnNewYear(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "resetyear@test.com", "ResetYear User")
	ctx := context.Background()

	// Create quota and increment
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}

	err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
	}

	// Simulate next year (same month, different year)
	nextYear := time.Now().UTC().AddDate(1, 0, 0)

	// Reset should be needed (new year)
	wasReset, err := env.DB.ResetSmartRecapQuotaIfNeededAt(ctx, user.ID, nextYear)
	if err != nil {
		t.Fatalf("ResetSmartRecapQuotaIfNeededAt failed: %v", err)
	}
	if !wasReset {
		t.Error("expected reset for new year")
	}

	// Count should be 0 after reset
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != 0 {
		t.Errorf("ComputeCount = %d, want 0 after reset", quota.ComputeCount)
	}
}

func TestSmartRecapQuota_NoResetSameMonthAfterReset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "norereset@test.com", "NoReReset User")
	ctx := context.Background()

	// Create quota and increment
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}

	err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
	}

	// Simulate next month - first reset
	nextMonth := time.Now().UTC().AddDate(0, 1, 0)
	wasReset, err := env.DB.ResetSmartRecapQuotaIfNeededAt(ctx, user.ID, nextMonth)
	if err != nil {
		t.Fatalf("ResetSmartRecapQuotaIfNeededAt failed: %v", err)
	}
	if !wasReset {
		t.Error("expected first reset")
	}

	// Increment again after reset
	err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
	}

	// Try to reset again in same month - should NOT reset
	sameDayNextMonth := nextMonth.Add(24 * time.Hour) // Same month, different day
	wasReset2, err := env.DB.ResetSmartRecapQuotaIfNeededAt(ctx, user.ID, sameDayNextMonth)
	if err != nil {
		t.Fatalf("ResetSmartRecapQuotaIfNeededAt (second) failed: %v", err)
	}
	if wasReset2 {
		t.Error("expected NO reset within same month after previous reset")
	}

	// Count should still be 1 (not reset again)
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != 1 {
		t.Errorf("ComputeCount = %d, want 1", quota.ComputeCount)
	}
}

func TestSmartRecapQuota_EnforcementExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "exceeded@test.com", "Exceeded User")
	ctx := context.Background()

	// Create quota
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}

	// Simulate reaching quota limit (increment 100 times)
	quotaLimit := 100
	for i := 0; i < quotaLimit; i++ {
		err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
		if err != nil {
			t.Fatalf("IncrementSmartRecapQuota failed at %d: %v", i, err)
		}
	}

	// Check quota
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != quotaLimit {
		t.Errorf("ComputeCount = %d, want %d", quota.ComputeCount, quotaLimit)
	}

	// Verify quota is exceeded
	exceeded := quota.ComputeCount >= quotaLimit
	if !exceeded {
		t.Error("expected quota to be exceeded")
	}
}

func TestSmartRecapQuota_EnforcementUnderLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "underlimit@test.com", "UnderLimit User")
	ctx := context.Background()

	// Create quota
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}

	// Increment a few times
	for i := 0; i < 5; i++ {
		err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
		if err != nil {
			t.Fatalf("IncrementSmartRecapQuota failed at %d: %v", i, err)
		}
	}

	// Check quota
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != 5 {
		t.Errorf("ComputeCount = %d, want 5", quota.ComputeCount)
	}

	// Verify quota is NOT exceeded
	quotaLimit := 100
	exceeded := quota.ComputeCount >= quotaLimit
	if exceeded {
		t.Error("expected quota to NOT be exceeded")
	}
}

func TestSmartRecapQuota_ResetClearsExceededState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "resetexceeded@test.com", "ResetExceeded User")
	ctx := context.Background()

	// Create quota and reach limit
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}

	quotaLimit := 100
	for i := 0; i < quotaLimit; i++ {
		err = env.DB.IncrementSmartRecapQuota(ctx, user.ID)
		if err != nil {
			t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
		}
	}

	// Verify exceeded
	quota, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount < quotaLimit {
		t.Errorf("ComputeCount = %d, want >= %d", quota.ComputeCount, quotaLimit)
	}

	// Reset with new month
	nextMonth := time.Now().UTC().AddDate(0, 1, 0)
	wasReset, err := env.DB.ResetSmartRecapQuotaIfNeededAt(ctx, user.ID, nextMonth)
	if err != nil {
		t.Fatalf("ResetSmartRecapQuotaIfNeededAt failed: %v", err)
	}
	if !wasReset {
		t.Error("expected reset")
	}

	// Verify no longer exceeded
	quota, err = env.DB.GetOrCreateSmartRecapQuota(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	if quota.ComputeCount != 0 {
		t.Errorf("ComputeCount = %d, want 0 after reset", quota.ComputeCount)
	}

	exceeded := quota.ComputeCount >= quotaLimit
	if exceeded {
		t.Error("expected quota to NOT be exceeded after reset")
	}
}

func TestListUserSmartRecapStats_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()

	// Create a user but no sessions with cache or quota activity
	testutil.CreateTestUser(t, env, "noactivity@test.com", "NoActivity User")

	stats, err := env.DB.ListUserSmartRecapStats(ctx)
	if err != nil {
		t.Fatalf("ListUserSmartRecapStats failed: %v", err)
	}

	if len(stats) != 0 {
		t.Errorf("expected 0 users with recap activity, got %d", len(stats))
	}
}

func TestListUserSmartRecapStats_WithCacheAndQuota(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())

	// Create two users
	user1 := testutil.CreateTestUser(t, env, "user1@test.com", "User One")
	user2 := testutil.CreateTestUser(t, env, "user2@test.com", "User Two")

	// Create sessions for user1
	session1ID := testutil.CreateTestSession(t, env, user1.ID, "session-1")
	session2ID := testutil.CreateTestSession(t, env, user1.ID, "session-2")

	// Create session for user2
	session3ID := testutil.CreateTestSession(t, env, user2.ID, "session-3")

	// Add smart recap cache entries
	now := time.Now()
	for _, sessionID := range []string{session1ID, session2ID, session3ID} {
		err := store.UpsertSmartRecapCard(ctx, &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   1,
			ComputedAt:                now,
			UpToLine:                  100,
			Recap:                     "Test recap",
			WentWell:                  []string{},
			WentBad:                   []string{},
			HumanSuggestions:          []string{},
			EnvironmentSuggestions:    []string{},
			DefaultContextSuggestions: []string{},
			ModelUsed:                 "test-model",
			InputTokens:               100,
			OutputTokens:              50,
		})
		if err != nil {
			t.Fatalf("UpsertSmartRecapCard failed: %v", err)
		}
	}

	// Add quota for user1 with 5 computations
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user1.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := env.DB.IncrementSmartRecapQuota(ctx, user1.ID); err != nil {
			t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
		}
	}

	// Add quota for user2 with 3 computations
	_, err = env.DB.GetOrCreateSmartRecapQuota(ctx, user2.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := env.DB.IncrementSmartRecapQuota(ctx, user2.ID); err != nil {
			t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
		}
	}

	// Query stats
	stats, err := env.DB.ListUserSmartRecapStats(ctx)
	if err != nil {
		t.Fatalf("ListUserSmartRecapStats failed: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("expected 2 users with recap activity, got %d", len(stats))
	}

	// Results should be ordered by computations DESC, so user1 (5) should be first
	if stats[0].UserID != user1.ID {
		t.Errorf("expected user1 to be first (most computations), got user %d", stats[0].UserID)
	}
	if stats[0].SessionsWithCache != 2 {
		t.Errorf("user1 SessionsWithCache = %d, want 2", stats[0].SessionsWithCache)
	}
	if stats[0].ComputationsThisMonth != 5 {
		t.Errorf("user1 ComputationsThisMonth = %d, want 5", stats[0].ComputationsThisMonth)
	}
	if stats[0].Email != "user1@test.com" {
		t.Errorf("user1 Email = %s, want user1@test.com", stats[0].Email)
	}

	if stats[1].UserID != user2.ID {
		t.Errorf("expected user2 to be second, got user %d", stats[1].UserID)
	}
	if stats[1].SessionsWithCache != 1 {
		t.Errorf("user2 SessionsWithCache = %d, want 1", stats[1].SessionsWithCache)
	}
	if stats[1].ComputationsThisMonth != 3 {
		t.Errorf("user2 ComputationsThisMonth = %d, want 3", stats[1].ComputationsThisMonth)
	}
}

func TestListUserSmartRecapStats_CacheOnlyNoQuota(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())

	// Create user with session cache but no quota activity
	user := testutil.CreateTestUser(t, env, "cacheonly@test.com", "CacheOnly User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "session-cache-only")

	// Add smart recap cache entry
	err := store.UpsertSmartRecapCard(ctx, &analytics.SmartRecapCardRecord{
		SessionID:                 sessionID,
		Version:                   1,
		ComputedAt:                time.Now(),
		UpToLine:                  100,
		Recap:                     "Test recap",
		WentWell:                  []string{},
		WentBad:                   []string{},
		HumanSuggestions:          []string{},
		EnvironmentSuggestions:    []string{},
		DefaultContextSuggestions: []string{},
		ModelUsed:                 "test-model",
		InputTokens:               100,
		OutputTokens:              50,
	})
	if err != nil {
		t.Fatalf("UpsertSmartRecapCard failed: %v", err)
	}

	// Query stats - user should appear due to cache entry
	stats, err := env.DB.ListUserSmartRecapStats(ctx)
	if err != nil {
		t.Fatalf("ListUserSmartRecapStats failed: %v", err)
	}

	if len(stats) != 1 {
		t.Fatalf("expected 1 user with recap activity, got %d", len(stats))
	}

	if stats[0].SessionsWithCache != 1 {
		t.Errorf("SessionsWithCache = %d, want 1", stats[0].SessionsWithCache)
	}
	if stats[0].ComputationsThisMonth != 0 {
		t.Errorf("ComputationsThisMonth = %d, want 0", stats[0].ComputationsThisMonth)
	}
	if stats[0].LastComputeAt != nil {
		t.Error("LastComputeAt should be nil for user without quota activity")
	}
}

func TestGetSmartRecapTotals_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()

	totals, err := env.DB.GetSmartRecapTotals(ctx)
	if err != nil {
		t.Fatalf("GetSmartRecapTotals failed: %v", err)
	}

	if totals.TotalSessionsWithCache != 0 {
		t.Errorf("TotalSessionsWithCache = %d, want 0", totals.TotalSessionsWithCache)
	}
	if totals.TotalComputationsThisMonth != 0 {
		t.Errorf("TotalComputationsThisMonth = %d, want 0", totals.TotalComputationsThisMonth)
	}
	if totals.TotalUsersWithActivity != 0 {
		t.Errorf("TotalUsersWithActivity = %d, want 0", totals.TotalUsersWithActivity)
	}
}

func TestGetSmartRecapTotals_WithData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())

	// Create two users
	user1 := testutil.CreateTestUser(t, env, "totals1@test.com", "Totals One")
	user2 := testutil.CreateTestUser(t, env, "totals2@test.com", "Totals Two")

	// Create 3 sessions total
	session1ID := testutil.CreateTestSession(t, env, user1.ID, "session-t1")
	session2ID := testutil.CreateTestSession(t, env, user1.ID, "session-t2")
	session3ID := testutil.CreateTestSession(t, env, user2.ID, "session-t3")

	// Add smart recap cache entries for all
	now := time.Now()
	for _, sessionID := range []string{session1ID, session2ID, session3ID} {
		err := store.UpsertSmartRecapCard(ctx, &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   1,
			ComputedAt:                now,
			UpToLine:                  100,
			Recap:                     "Test recap",
			WentWell:                  []string{},
			WentBad:                   []string{},
			HumanSuggestions:          []string{},
			EnvironmentSuggestions:    []string{},
			DefaultContextSuggestions: []string{},
			ModelUsed:                 "test-model",
			InputTokens:               100,
			OutputTokens:              50,
		})
		if err != nil {
			t.Fatalf("UpsertSmartRecapCard failed: %v", err)
		}
	}

	// Add quota: user1 = 5 computations, user2 = 3 computations
	_, err := env.DB.GetOrCreateSmartRecapQuota(ctx, user1.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := env.DB.IncrementSmartRecapQuota(ctx, user1.ID); err != nil {
			t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
		}
	}

	_, err = env.DB.GetOrCreateSmartRecapQuota(ctx, user2.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSmartRecapQuota failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := env.DB.IncrementSmartRecapQuota(ctx, user2.ID); err != nil {
			t.Fatalf("IncrementSmartRecapQuota failed: %v", err)
		}
	}

	// Query totals
	totals, err := env.DB.GetSmartRecapTotals(ctx)
	if err != nil {
		t.Fatalf("GetSmartRecapTotals failed: %v", err)
	}

	if totals.TotalSessionsWithCache != 3 {
		t.Errorf("TotalSessionsWithCache = %d, want 3", totals.TotalSessionsWithCache)
	}
	if totals.TotalComputationsThisMonth != 8 {
		t.Errorf("TotalComputationsThisMonth = %d, want 8 (5+3)", totals.TotalComputationsThisMonth)
	}
	if totals.TotalUsersWithActivity != 2 {
		t.Errorf("TotalUsersWithActivity = %d, want 2", totals.TotalUsersWithActivity)
	}
}
