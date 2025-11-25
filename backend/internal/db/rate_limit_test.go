package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santaclaude2025/confab/backend/internal/testutil"
)

func TestCountUserRunsInLastWeek(t *testing.T) {
	// Skip if running short tests (requires database)
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	// Setup test environment with testcontainers
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a test user using raw SQL
	var userID int64
	err := env.DB.QueryRow(ctx,
		`INSERT INTO users (email, name, avatar_url, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW()) RETURNING id`,
		"test@example.com", "Test User", "https://test.com/avatar.png",
	).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create user identity
	_, err = env.DB.Exec(ctx,
		`INSERT INTO user_identities (user_id, provider, provider_id, created_at)
		 VALUES ($1, 'github', $2, NOW())`,
		userID, "test-github")
	if err != nil {
		t.Fatalf("Failed to create user identity: %v", err)
	}

	t.Run("no runs returns zero", func(t *testing.T) {
		count, err := env.DB.CountUserRunsInLastWeek(ctx, userID)
		if err != nil {
			t.Fatalf("CountUserRunsInLastWeek failed: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 runs, got %d", count)
		}
	})

	t.Run("counts recent runs within 7 days", func(t *testing.T) {
		// Create a session using raw SQL with UUID
		externalID := "test-session-recent"
		sessionID := uuid.New().String()
		_, err := env.DB.Exec(ctx,
			`INSERT INTO sessions (id, external_id, user_id, first_seen) VALUES ($1, $2, $3, NOW())`,
			sessionID, externalID, userID)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Create 5 runs in the last week using raw SQL
		for i := 0; i < 5; i++ {
			_, err = env.DB.Exec(ctx,
				`INSERT INTO runs (session_id, transcript_path, cwd, reason, source, end_timestamp)
				 VALUES ($1, $2, $3, $4, $5, NOW())`,
				sessionID, "/path/to/transcript", "/cwd", "test", "hook")
			if err != nil {
				t.Fatalf("Failed to create run %d: %v", i, err)
			}
		}

		count, err := env.DB.CountUserRunsInLastWeek(ctx, userID)
		if err != nil {
			t.Fatalf("CountUserRunsInLastWeek failed: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected 5 runs, got %d", count)
		}
	})

	t.Run("excludes runs older than 7 days", func(t *testing.T) {
		// Create a session for old runs with UUID
		externalID := "test-session-old"
		sessionID := uuid.New().String()
		_, err := env.DB.Exec(ctx,
			`INSERT INTO sessions (id, external_id, user_id, first_seen) VALUES ($1, $2, $3, NOW())`,
			sessionID, externalID, userID)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Create a run and manually backdate it to 8 days ago
		var runID int64
		err = env.DB.QueryRow(ctx,
			`INSERT INTO runs (session_id, transcript_path, cwd, reason, source, end_timestamp)
			 VALUES ($1, $2, $3, $4, $5, NOW()) RETURNING id`,
			sessionID, "/path/to/old", "/cwd", "old test", "hook",
		).Scan(&runID)
		if err != nil {
			t.Fatalf("Failed to create old run: %v", err)
		}

		// Backdate the run by updating created_at directly
		eightDaysAgo := time.Now().UTC().Add(-8 * 24 * time.Hour)
		_, err = env.DB.Exec(ctx,
			`UPDATE runs SET created_at = $1 WHERE id = $2`,
			eightDaysAgo, runID)
		if err != nil {
			t.Fatalf("Failed to backdate run: %v", err)
		}

		// Count should still be 5 (from previous test), not include the old one
		count, err := env.DB.CountUserRunsInLastWeek(ctx, userID)
		if err != nil {
			t.Fatalf("CountUserRunsInLastWeek failed: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected 5 runs (old run should be excluded), got %d", count)
		}
	})

	t.Run("only counts runs for specific user", func(t *testing.T) {
		// Create another user using raw SQL
		var otherUserID int64
		err := env.DB.QueryRow(ctx,
			`INSERT INTO users (email, name, avatar_url, created_at, updated_at)
			 VALUES ($1, $2, $3, NOW(), NOW()) RETURNING id`,
			"other@example.com", "Other User", "https://test.com/other.png",
		).Scan(&otherUserID)
		if err != nil {
			t.Fatalf("Failed to create other user: %v", err)
		}

		// Create user identity for other user
		_, err = env.DB.Exec(ctx,
			`INSERT INTO user_identities (user_id, provider, provider_id, created_at)
			 VALUES ($1, 'github', $2, NOW())`,
			otherUserID, "other-github")
		if err != nil {
			t.Fatalf("Failed to create other user identity: %v", err)
		}

		// Create a session and run for other user with UUID
		otherExternalID := "other-session"
		otherSessionID := uuid.New().String()
		_, err = env.DB.Exec(ctx,
			`INSERT INTO sessions (id, external_id, user_id, first_seen) VALUES ($1, $2, $3, NOW())`,
			otherSessionID, otherExternalID, otherUserID)
		if err != nil {
			t.Fatalf("Failed to create other session: %v", err)
		}

		_, err = env.DB.Exec(ctx,
			`INSERT INTO runs (session_id, transcript_path, cwd, reason, source, end_timestamp)
			 VALUES ($1, $2, $3, $4, $5, NOW())`,
			otherSessionID, "/path/to/other", "/cwd", "other test", "hook")
		if err != nil {
			t.Fatalf("Failed to create other run: %v", err)
		}

		// Original user should still have 5 runs
		count, err := env.DB.CountUserRunsInLastWeek(ctx, userID)
		if err != nil {
			t.Fatalf("CountUserRunsInLastWeek failed: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected 5 runs for original user, got %d", count)
		}

		// Other user should have 1 run
		otherCount, err := env.DB.CountUserRunsInLastWeek(ctx, otherUserID)
		if err != nil {
			t.Fatalf("CountUserRunsInLastWeek failed for other user: %v", err)
		}
		if otherCount != 1 {
			t.Errorf("Expected 1 run for other user, got %d", otherCount)
		}
	})
}

func TestGetUserWeeklyUsage(t *testing.T) {
	// Skip if running short tests (requires database)
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	// Setup test environment with testcontainers
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a test user using raw SQL
	var userID int64
	err := env.DB.QueryRow(ctx,
		`INSERT INTO users (email, name, avatar_url, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW()) RETURNING id`,
		"usage@example.com", "Usage User", "https://test.com/usage.png",
	).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create user identity
	_, err = env.DB.Exec(ctx,
		`INSERT INTO user_identities (user_id, provider, provider_id, created_at)
		 VALUES ($1, 'github', $2, NOW())`,
		userID, "usage-github")
	if err != nil {
		t.Fatalf("Failed to create user identity: %v", err)
	}

	const maxRuns = 200

	t.Run("empty usage", func(t *testing.T) {
		usage, err := env.DB.GetUserWeeklyUsage(ctx, userID, maxRuns)
		if err != nil {
			t.Fatalf("GetUserWeeklyUsage failed: %v", err)
		}

		if usage.CurrentCount != 0 {
			t.Errorf("Expected CurrentCount=0, got %d", usage.CurrentCount)
		}
		if usage.Limit != maxRuns {
			t.Errorf("Expected Limit=%d, got %d", maxRuns, usage.Limit)
		}
		if usage.Remaining != maxRuns {
			t.Errorf("Expected Remaining=%d, got %d", maxRuns, usage.Remaining)
		}
		if usage.PeriodStart.After(time.Now().UTC()) {
			t.Errorf("PeriodStart should be in the past")
		}
		if usage.PeriodEnd.Before(time.Now().UTC().Add(-1 * time.Minute)) {
			t.Errorf("PeriodEnd should be recent")
		}
	})

	t.Run("partial usage", func(t *testing.T) {
		// Create session and 50 runs using raw SQL with UUID
		externalID := "usage-session"
		sessionID := uuid.New().String()
		_, err := env.DB.Exec(ctx,
			`INSERT INTO sessions (id, external_id, user_id, first_seen) VALUES ($1, $2, $3, NOW())`,
			sessionID, externalID, userID)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		for i := 0; i < 50; i++ {
			_, err = env.DB.Exec(ctx,
				`INSERT INTO runs (session_id, transcript_path, cwd, reason, source, end_timestamp)
				 VALUES ($1, $2, $3, $4, $5, NOW())`,
				sessionID, "/path/to/usage", "/cwd", "usage test", "hook")
			if err != nil {
				t.Fatalf("Failed to create run %d: %v", i, err)
			}
		}

		usage, err := env.DB.GetUserWeeklyUsage(ctx, userID, maxRuns)
		if err != nil {
			t.Fatalf("GetUserWeeklyUsage failed: %v", err)
		}

		if usage.CurrentCount != 50 {
			t.Errorf("Expected CurrentCount=50, got %d", usage.CurrentCount)
		}
		if usage.Limit != maxRuns {
			t.Errorf("Expected Limit=%d, got %d", maxRuns, usage.Limit)
		}
		if usage.Remaining != 150 {
			t.Errorf("Expected Remaining=150, got %d", usage.Remaining)
		}
	})

	t.Run("at limit", func(t *testing.T) {
		// Create 150 more runs to reach 200 total using raw SQL
		// Get the session ID from the existing session
		var sessionID string
		err := env.DB.QueryRow(ctx,
			`SELECT id FROM sessions WHERE external_id = 'usage-session' AND user_id = $1`,
			userID).Scan(&sessionID)
		if err != nil {
			t.Fatalf("Failed to get session ID: %v", err)
		}
		for i := 0; i < 150; i++ {
			_, err = env.DB.Exec(ctx,
				`INSERT INTO runs (session_id, transcript_path, cwd, reason, source, end_timestamp)
				 VALUES ($1, $2, $3, $4, $5, NOW())`,
				sessionID, "/path/to/usage", "/cwd", "usage test", "hook")
			if err != nil {
				t.Fatalf("Failed to create run %d: %v", i, err)
			}
		}

		usage, err := env.DB.GetUserWeeklyUsage(ctx, userID, maxRuns)
		if err != nil {
			t.Fatalf("GetUserWeeklyUsage failed: %v", err)
		}

		if usage.CurrentCount != 200 {
			t.Errorf("Expected CurrentCount=200, got %d", usage.CurrentCount)
		}
		if usage.Remaining != 0 {
			t.Errorf("Expected Remaining=0, got %d", usage.Remaining)
		}
	})

	t.Run("over limit", func(t *testing.T) {
		// Create 5 more runs to exceed limit using raw SQL
		// Get the session ID from the existing session
		var sessionID string
		err := env.DB.QueryRow(ctx,
			`SELECT id FROM sessions WHERE external_id = 'usage-session' AND user_id = $1`,
			userID).Scan(&sessionID)
		if err != nil {
			t.Fatalf("Failed to get session ID: %v", err)
		}
		for i := 0; i < 5; i++ {
			_, err = env.DB.Exec(ctx,
				`INSERT INTO runs (session_id, transcript_path, cwd, reason, source, end_timestamp)
				 VALUES ($1, $2, $3, $4, $5, NOW())`,
				sessionID, "/path/to/usage", "/cwd", "usage test", "hook")
			if err != nil {
				t.Fatalf("Failed to create run %d: %v", i, err)
			}
		}

		usage, err := env.DB.GetUserWeeklyUsage(ctx, userID, maxRuns)
		if err != nil {
			t.Fatalf("GetUserWeeklyUsage failed: %v", err)
		}

		if usage.CurrentCount != 205 {
			t.Errorf("Expected CurrentCount=205, got %d", usage.CurrentCount)
		}
		if usage.Remaining != 0 {
			t.Errorf("Expected Remaining=0 (capped), got %d", usage.Remaining)
		}
	})
}
