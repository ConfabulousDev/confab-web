package recapquota_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestGetOrCreate_NewUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "newuser@test.com", "New User")
	ctx := context.Background()
	conn := env.DB.Conn()

	quota, err := recapquota.GetOrCreate(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	if quota.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", quota.UserID, user.ID)
	}
	if quota.ComputeCount != 0 {
		t.Errorf("ComputeCount = %d, want 0", quota.ComputeCount)
	}
	if quota.QuotaMonth != recapquota.CurrentMonth() {
		t.Errorf("QuotaMonth = %s, want %s", quota.QuotaMonth, recapquota.CurrentMonth())
	}
}

func TestGetOrCreate_SameMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "samemonth@test.com", "SameMonth User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Create and increment
	_, err := recapquota.GetOrCreate(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	if err := recapquota.Increment(ctx, conn, user.ID); err != nil {
		t.Fatalf("Increment failed: %v", err)
	}

	// Re-read — count should be preserved
	quota, err := recapquota.GetOrCreate(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate (second) failed: %v", err)
	}
	if quota.ComputeCount != 1 {
		t.Errorf("ComputeCount = %d, want 1", quota.ComputeCount)
	}
}

func TestGetOrCreate_StaleMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "stalemonth@test.com", "StaleMonth User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Insert a row with an old month and high count via raw SQL
	_, err := conn.ExecContext(ctx, `
		INSERT INTO smart_recap_quota (user_id, compute_count, quota_month)
		VALUES ($1, 50, '2020-01')
	`, user.ID)
	if err != nil {
		t.Fatalf("failed to insert old quota: %v", err)
	}

	// GetOrCreate should reset count to 0 and update month
	quota, err := recapquota.GetOrCreate(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	if quota.ComputeCount != 0 {
		t.Errorf("ComputeCount = %d, want 0 (stale month reset)", quota.ComputeCount)
	}
	if quota.QuotaMonth != recapquota.CurrentMonth() {
		t.Errorf("QuotaMonth = %s, want %s", quota.QuotaMonth, recapquota.CurrentMonth())
	}
}

func TestGetOrCreateForMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "formonth@test.com", "ForMonth User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Create for a specific month
	quota, err := recapquota.GetOrCreateForMonth(ctx, conn, user.ID, "2030-06")
	if err != nil {
		t.Fatalf("GetOrCreateForMonth failed: %v", err)
	}
	if quota.QuotaMonth != "2030-06" {
		t.Errorf("QuotaMonth = %s, want 2030-06", quota.QuotaMonth)
	}

	// Increment in same month
	if err := recapquota.IncrementForMonth(ctx, conn, user.ID, "2030-06"); err != nil {
		t.Fatalf("IncrementForMonth failed: %v", err)
	}

	// Re-read — should be 1
	quota, err = recapquota.GetOrCreateForMonth(ctx, conn, user.ID, "2030-06")
	if err != nil {
		t.Fatalf("GetOrCreateForMonth (re-read) failed: %v", err)
	}
	if quota.ComputeCount != 1 {
		t.Errorf("ComputeCount = %d, want 1", quota.ComputeCount)
	}

	// Read for a different month — should reset to 0
	quota, err = recapquota.GetOrCreateForMonth(ctx, conn, user.ID, "2030-07")
	if err != nil {
		t.Fatalf("GetOrCreateForMonth (new month) failed: %v", err)
	}
	if quota.ComputeCount != 0 {
		t.Errorf("ComputeCount = %d, want 0 (new month)", quota.ComputeCount)
	}
	if quota.QuotaMonth != "2030-07" {
		t.Errorf("QuotaMonth = %s, want 2030-07", quota.QuotaMonth)
	}
}

func TestIncrement_Basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "increment@test.com", "Increment User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Create quota first
	_, err := recapquota.GetOrCreate(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	// Increment twice
	if err := recapquota.Increment(ctx, conn, user.ID); err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if err := recapquota.Increment(ctx, conn, user.ID); err != nil {
		t.Fatalf("Increment (second) failed: %v", err)
	}

	// Verify count = 2
	quota, err := recapquota.GetOrCreate(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	if quota.ComputeCount != 2 {
		t.Errorf("ComputeCount = %d, want 2", quota.ComputeCount)
	}
	if quota.LastComputeAt == nil {
		t.Error("LastComputeAt should be set after increment")
	}
}

func TestIncrement_StaleMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "incstale@test.com", "IncStale User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Insert old-month row with count=50
	_, err := conn.ExecContext(ctx, `
		INSERT INTO smart_recap_quota (user_id, compute_count, quota_month)
		VALUES ($1, 50, '2020-01')
	`, user.ID)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Increment — should reset to 1 and update month
	if err := recapquota.Increment(ctx, conn, user.ID); err != nil {
		t.Fatalf("Increment failed: %v", err)
	}

	count, err := recapquota.GetCount(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("ComputeCount = %d, want 1 (stale month reset + 1)", count)
	}
}

func TestIncrement_NoRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "norecord@test.com", "NoRecord User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Increment without creating first — should error
	err := recapquota.Increment(ctx, conn, user.ID)
	if err == nil {
		t.Error("expected error when incrementing non-existent quota")
	}
}

func TestGetCount_CurrentMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "getcount@test.com", "GetCount User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Create and increment 3 times
	_, err := recapquota.GetOrCreate(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := recapquota.Increment(ctx, conn, user.ID); err != nil {
			t.Fatalf("Increment failed at %d: %v", i, err)
		}
	}

	count, err := recapquota.GetCount(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestGetCount_StaleMonth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "getstale@test.com", "GetStale User")
	ctx := context.Background()
	conn := env.DB.Conn()

	// Insert old-month row
	_, err := conn.ExecContext(ctx, `
		INSERT INTO smart_recap_quota (user_id, compute_count, quota_month)
		VALUES ($1, 42, '2020-01')
	`, user.ID)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	count, err := recapquota.GetCount(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (stale month)", count)
	}
}

func TestGetCount_NoRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "getnorecord@test.com", "GetNoRecord User")
	ctx := context.Background()
	conn := env.DB.Conn()

	count, err := recapquota.GetCount(ctx, conn, user.ID)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (no record)", count)
	}
}

func TestListUserStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	conn := env.DB.Conn()
	store := analytics.NewStore(conn)

	// Create two users
	user1 := testutil.CreateTestUser(t, env, "stats1@test.com", "Stats One")
	user2 := testutil.CreateTestUser(t, env, "stats2@test.com", "Stats Two")

	// Create sessions and add recap cache
	session1ID := testutil.CreateTestSession(t, env, user1.ID, "session-s1")
	session2ID := testutil.CreateTestSession(t, env, user1.ID, "session-s2")
	session3ID := testutil.CreateTestSession(t, env, user2.ID, "session-s3")

	now := time.Now()
	for _, sessionID := range []string{session1ID, session2ID, session3ID} {
		err := store.UpsertSmartRecapCard(ctx, &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   1,
			ComputedAt:                now,
			UpToLine:                  100,
			Recap:                     "Test recap",
			WentWell:                  []analytics.AnnotatedItem{},
			WentBad:                   []analytics.AnnotatedItem{},
			HumanSuggestions:          []analytics.AnnotatedItem{},
			EnvironmentSuggestions:    []analytics.AnnotatedItem{},
			DefaultContextSuggestions: []analytics.AnnotatedItem{},
			ModelUsed:                 "test-model",
			InputTokens:               100,
			OutputTokens:              50,
		})
		if err != nil {
			t.Fatalf("UpsertSmartRecapCard failed: %v", err)
		}
	}

	// Add quota: user1=5, user2=3
	if _, err := recapquota.GetOrCreate(ctx, conn, user1.ID); err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := recapquota.Increment(ctx, conn, user1.ID); err != nil {
			t.Fatalf("Increment failed: %v", err)
		}
	}
	if _, err := recapquota.GetOrCreate(ctx, conn, user2.ID); err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := recapquota.Increment(ctx, conn, user2.ID); err != nil {
			t.Fatalf("Increment failed: %v", err)
		}
	}

	stats, err := recapquota.ListUserStats(ctx, conn)
	if err != nil {
		t.Fatalf("ListUserStats failed: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 users, got %d", len(stats))
	}

	// Ordered by computations DESC
	if stats[0].UserID != user1.ID {
		t.Errorf("expected user1 first, got user %d", stats[0].UserID)
	}
	if stats[0].SessionsWithCache != 2 {
		t.Errorf("user1 SessionsWithCache = %d, want 2", stats[0].SessionsWithCache)
	}
	if stats[0].ComputationsThisMonth != 5 {
		t.Errorf("user1 ComputationsThisMonth = %d, want 5", stats[0].ComputationsThisMonth)
	}
	if stats[1].SessionsWithCache != 1 {
		t.Errorf("user2 SessionsWithCache = %d, want 1", stats[1].SessionsWithCache)
	}
	if stats[1].ComputationsThisMonth != 3 {
		t.Errorf("user2 ComputationsThisMonth = %d, want 3", stats[1].ComputationsThisMonth)
	}
}

func TestListUserStats_StaleMonthExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	conn := env.DB.Conn()

	user := testutil.CreateTestUser(t, env, "statsold@test.com", "StatsOld User")

	// Insert quota from an old month
	_, err := conn.ExecContext(ctx, `
		INSERT INTO smart_recap_quota (user_id, compute_count, quota_month)
		VALUES ($1, 10, '2020-01')
	`, user.ID)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	stats, err := recapquota.ListUserStats(ctx, conn)
	if err != nil {
		t.Fatalf("ListUserStats failed: %v", err)
	}
	// Old-month quota should not show up (no cache, no current-month quota)
	if len(stats) != 0 {
		t.Errorf("expected 0 users (stale month excluded), got %d", len(stats))
	}
}

func TestGetTotals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	conn := env.DB.Conn()
	store := analytics.NewStore(conn)

	// Create users
	user1 := testutil.CreateTestUser(t, env, "totals1@test.com", "Totals One")
	user2 := testutil.CreateTestUser(t, env, "totals2@test.com", "Totals Two")

	// Create sessions and recap cache
	session1ID := testutil.CreateTestSession(t, env, user1.ID, "session-t1")
	session2ID := testutil.CreateTestSession(t, env, user2.ID, "session-t2")

	now := time.Now()
	for _, sessionID := range []string{session1ID, session2ID} {
		err := store.UpsertSmartRecapCard(ctx, &analytics.SmartRecapCardRecord{
			SessionID:                 sessionID,
			Version:                   1,
			ComputedAt:                now,
			UpToLine:                  100,
			Recap:                     "Test recap",
			WentWell:                  []analytics.AnnotatedItem{},
			WentBad:                   []analytics.AnnotatedItem{},
			HumanSuggestions:          []analytics.AnnotatedItem{},
			EnvironmentSuggestions:    []analytics.AnnotatedItem{},
			DefaultContextSuggestions: []analytics.AnnotatedItem{},
			ModelUsed:                 "test-model",
			InputTokens:               100,
			OutputTokens:              50,
		})
		if err != nil {
			t.Fatalf("UpsertSmartRecapCard failed: %v", err)
		}
	}

	// Add quota: user1=5, user2=3
	if _, err := recapquota.GetOrCreate(ctx, conn, user1.ID); err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := recapquota.Increment(ctx, conn, user1.ID); err != nil {
			t.Fatalf("Increment failed: %v", err)
		}
	}
	if _, err := recapquota.GetOrCreate(ctx, conn, user2.ID); err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := recapquota.Increment(ctx, conn, user2.ID); err != nil {
			t.Fatalf("Increment failed: %v", err)
		}
	}

	totals, err := recapquota.GetTotals(ctx, conn)
	if err != nil {
		t.Fatalf("GetTotals failed: %v", err)
	}
	if totals.TotalSessionsWithCache != 2 {
		t.Errorf("TotalSessionsWithCache = %d, want 2", totals.TotalSessionsWithCache)
	}
	if totals.TotalComputationsThisMonth != 8 {
		t.Errorf("TotalComputationsThisMonth = %d, want 8 (5+3)", totals.TotalComputationsThisMonth)
	}
	if totals.TotalUsersWithActivity != 2 {
		t.Errorf("TotalUsersWithActivity = %d, want 2", totals.TotalUsersWithActivity)
	}
}

func TestGetTotals_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	conn := env.DB.Conn()

	totals, err := recapquota.GetTotals(ctx, conn)
	if err != nil {
		t.Fatalf("GetTotals failed: %v", err)
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
