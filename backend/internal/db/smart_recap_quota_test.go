package db_test

import (
	"context"
	"testing"
	"time"

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
