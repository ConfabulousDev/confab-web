package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/shopspring/decimal"
)

func TestGetOrgAnalytics_EmptyResults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user but no sessions
	testutil.CreateTestUser(t, env, "org-empty@test.com", "Empty User")
	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())

	now := time.Now().UTC()
	req := analytics.OrgAnalyticsRequest{
		StartTS:  now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:    now.Add(24 * time.Hour).Unix(),
		TZOffset: 0,
	}

	response, err := store.GetOrgAnalytics(ctx, req)
	if err != nil {
		t.Fatalf("GetOrgAnalytics failed: %v", err)
	}

	// User should appear with zero sessions
	if len(response.Users) != 1 {
		t.Fatalf("Users length = %d, want 1", len(response.Users))
	}
	if response.Users[0].SessionCount != 0 {
		t.Errorf("SessionCount = %d, want 0", response.Users[0].SessionCount)
	}
	if response.Users[0].TotalCostUSD != "0.00" {
		t.Errorf("TotalCostUSD = %s, want 0.00", response.Users[0].TotalCostUSD)
	}
	if response.Users[0].AvgClaudeTimeMs != nil {
		t.Error("expected AvgClaudeTimeMs to be nil for 0 sessions")
	}
}

func TestGetOrgAnalytics_MultipleUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())

	// Create two users
	user1 := testutil.CreateTestUser(t, env, "alice@test.com", "Alice")
	user2 := testutil.CreateTestUser(t, env, "bob@test.com", "Bob")

	// Create sessions for user1 (2 sessions)
	sid1a := testutil.CreateTestSession(t, env, user1.ID, "alice-session-1")
	sid1b := testutil.CreateTestSession(t, env, user1.ID, "alice-session-2")

	// Create session for user2 (1 session)
	sid2a := testutil.CreateTestSession(t, env, user2.ID, "bob-session-1")

	// Insert both tokens and conversation cards (required for counting)
	now := time.Now().UTC()
	insertCards := func(sessionID string, cost float64, claudeMs, userMs int64, durationMs int64) {
		t.Helper()
		err := store.UpsertCards(ctx, &analytics.Cards{
			Tokens: &analytics.TokensCardRecord{
				SessionID:        sessionID,
				Version:          analytics.TokensCardVersion,
				ComputedAt:       now,
				UpToLine:         100,
				InputTokens:      1000,
				OutputTokens:     500,
				EstimatedCostUSD: decimal.NewFromFloat(cost),
			},
			Conversation: &analytics.ConversationCardRecord{
				SessionID:                sessionID,
				Version:                  analytics.ConversationCardVersion,
				ComputedAt:               now,
				UpToLine:                 100,
				UserTurns:                5,
				AssistantTurns:           5,
				TotalAssistantDurationMs: &claudeMs,
				TotalUserDurationMs:      &userMs,
			},
			Session: &analytics.SessionCardRecord{
				SessionID:  sessionID,
				Version:    analytics.SessionCardVersion,
				ComputedAt: now,
				UpToLine:   100,
				DurationMs: &durationMs,
			},
		})
		if err != nil {
			t.Fatalf("UpsertCards for %s failed: %v", sessionID, err)
		}
	}

	insertCards(sid1a, 1.00, 30000, 60000, 90000)
	insertCards(sid1b, 2.00, 40000, 80000, 120000)
	insertCards(sid2a, 0.50, 10000, 20000, 30000)

	req := analytics.OrgAnalyticsRequest{
		StartTS:  now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:    now.Add(24 * time.Hour).Unix(),
		TZOffset: 0,
	}

	response, err := store.GetOrgAnalytics(ctx, req)
	if err != nil {
		t.Fatalf("GetOrgAnalytics failed: %v", err)
	}

	if len(response.Users) != 2 {
		t.Fatalf("Users length = %d, want 2", len(response.Users))
	}

	// Default sort: name ASC → Alice first, Bob second
	alice := response.Users[0]
	bob := response.Users[1]

	if alice.User.Email != "alice@test.com" {
		t.Errorf("first user email = %s, want alice@test.com", alice.User.Email)
	}
	if bob.User.Email != "bob@test.com" {
		t.Errorf("second user email = %s, want bob@test.com", bob.User.Email)
	}

	// Alice: 2 sessions, $3.00 total, 70000ms claude, 140000ms user, 210000ms duration
	if alice.SessionCount != 2 {
		t.Errorf("Alice.SessionCount = %d, want 2", alice.SessionCount)
	}
	if alice.TotalCostUSD != "3.00" {
		t.Errorf("Alice.TotalCostUSD = %s, want 3.00", alice.TotalCostUSD)
	}
	if alice.TotalClaudeTimeMs != 70000 {
		t.Errorf("Alice.TotalClaudeTimeMs = %d, want 70000", alice.TotalClaudeTimeMs)
	}
	if alice.TotalUserTimeMs != 140000 {
		t.Errorf("Alice.TotalUserTimeMs = %d, want 140000", alice.TotalUserTimeMs)
	}
	if alice.TotalDurationMs != 210000 {
		t.Errorf("Alice.TotalDurationMs = %d, want 210000", alice.TotalDurationMs)
	}

	// Alice averages: $1.50, 35000ms claude, 70000ms user, 105000ms duration
	if alice.AvgCostUSD != "1.50" {
		t.Errorf("Alice.AvgCostUSD = %s, want 1.50", alice.AvgCostUSD)
	}
	if alice.AvgClaudeTimeMs == nil || *alice.AvgClaudeTimeMs != 35000 {
		t.Errorf("Alice.AvgClaudeTimeMs = %v, want 35000", alice.AvgClaudeTimeMs)
	}
	if alice.AvgDurationMs == nil || *alice.AvgDurationMs != 105000 {
		t.Errorf("Alice.AvgDurationMs = %v, want 105000", alice.AvgDurationMs)
	}

	// Bob: 1 session, $0.50 total
	if bob.SessionCount != 1 {
		t.Errorf("Bob.SessionCount = %d, want 1", bob.SessionCount)
	}
	if bob.TotalCostUSD != "0.50" {
		t.Errorf("Bob.TotalCostUSD = %s, want 0.50", bob.TotalCostUSD)
	}
}

func TestGetOrgAnalytics_DateRangeFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())
	user := testutil.CreateTestUser(t, env, "datefilter@test.com", "Date Filter User")

	// Create a session and insert cards
	sid := testutil.CreateTestSession(t, env, user.ID, "date-session")
	now := time.Now().UTC()
	claudeMs := int64(10000)
	userMs := int64(20000)

	err := store.UpsertCards(ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        sid,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       now,
			UpToLine:         100,
			EstimatedCostUSD: decimal.NewFromFloat(1.00),
		},
		Conversation: &analytics.ConversationCardRecord{
			SessionID:                sid,
			Version:                  analytics.ConversationCardVersion,
			ComputedAt:               now,
			UpToLine:                 100,
			TotalAssistantDurationMs: &claudeMs,
			TotalUserDurationMs:      &userMs,
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards failed: %v", err)
	}

	// Query a date range that's entirely in the past (before the session was created)
	pastEnd := now.Add(-30 * 24 * time.Hour)
	pastStart := pastEnd.Add(-7 * 24 * time.Hour)

	req := analytics.OrgAnalyticsRequest{
		StartTS:  pastStart.Unix(),
		EndTS:    pastEnd.Unix(),
		TZOffset: 0,
	}

	response, err := store.GetOrgAnalytics(ctx, req)
	if err != nil {
		t.Fatalf("GetOrgAnalytics failed: %v", err)
	}

	// User appears but with 0 sessions (out of date range)
	if len(response.Users) != 1 {
		t.Fatalf("Users length = %d, want 1", len(response.Users))
	}
	if response.Users[0].SessionCount != 0 {
		t.Errorf("SessionCount = %d, want 0 (session outside date range)", response.Users[0].SessionCount)
	}
	if response.Users[0].TotalCostUSD != "0.00" {
		t.Errorf("TotalCostUSD = %s, want 0.00", response.Users[0].TotalCostUSD)
	}
}

func TestGetOrgAnalytics_DeactivatedUsersExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())

	// Create active and inactive users
	activeUser := testutil.CreateTestUser(t, env, "active@test.com", "Active User")
	inactiveUser := testutil.CreateTestUser(t, env, "inactive@test.com", "Inactive User")

	// Deactivate the second user
	_, err := env.DB.Conn().ExecContext(ctx, "UPDATE users SET status = 'inactive' WHERE id = $1", inactiveUser.ID)
	if err != nil {
		t.Fatalf("Failed to deactivate user: %v", err)
	}

	// Create sessions and cards for both users
	now := time.Now().UTC()
	type userInfo struct {
		id    int64
		label string
	}
	for _, u := range []userInfo{
		{id: activeUser.ID, label: "active"},
		{id: inactiveUser.ID, label: "inactive"},
	} {
		sid := testutil.CreateTestSession(t, env, u.id, u.label+"-session")
		claudeMs := int64(10000)
		userMs := int64(20000)
		err := store.UpsertCards(ctx, &analytics.Cards{
			Tokens: &analytics.TokensCardRecord{
				SessionID:        sid,
				Version:          analytics.TokensCardVersion,
				ComputedAt:       now,
				UpToLine:         100,
				EstimatedCostUSD: decimal.NewFromFloat(1.00),
			},
			Conversation: &analytics.ConversationCardRecord{
				SessionID:                sid,
				Version:                  analytics.ConversationCardVersion,
				ComputedAt:               now,
				UpToLine:                 100,
				TotalAssistantDurationMs: &claudeMs,
				TotalUserDurationMs:      &userMs,
			},
		})
		if err != nil {
			t.Fatalf("UpsertCards failed: %v", err)
		}
	}

	req := analytics.OrgAnalyticsRequest{
		StartTS:  now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:    now.Add(24 * time.Hour).Unix(),
		TZOffset: 0,
	}

	response, err := store.GetOrgAnalytics(ctx, req)
	if err != nil {
		t.Fatalf("GetOrgAnalytics failed: %v", err)
	}

	// Only active user should appear
	if len(response.Users) != 1 {
		t.Fatalf("Users length = %d, want 1 (inactive user excluded)", len(response.Users))
	}
	if response.Users[0].User.Email != "active@test.com" {
		t.Errorf("User email = %s, want active@test.com", response.Users[0].User.Email)
	}
}

func TestGetOrgAnalytics_SessionsMissingOneCardExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())
	user := testutil.CreateTestUser(t, env, "partial@test.com", "Partial Card User")

	// Session 1: has both cards → should be counted
	sid1 := testutil.CreateTestSession(t, env, user.ID, "complete-session")
	// Session 2: only tokens card → should NOT be counted
	sid2 := testutil.CreateTestSession(t, env, user.ID, "tokens-only-session")

	now := time.Now().UTC()
	claudeMs := int64(10000)
	userMs := int64(20000)

	// Session 1: both cards
	err := store.UpsertCards(ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        sid1,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       now,
			UpToLine:         100,
			EstimatedCostUSD: decimal.NewFromFloat(1.00),
		},
		Conversation: &analytics.ConversationCardRecord{
			SessionID:                sid1,
			Version:                  analytics.ConversationCardVersion,
			ComputedAt:               now,
			UpToLine:                 100,
			TotalAssistantDurationMs: &claudeMs,
			TotalUserDurationMs:      &userMs,
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards for session 1 failed: %v", err)
	}

	// Session 2: only tokens card
	err = store.UpsertCards(ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        sid2,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       now,
			UpToLine:         100,
			EstimatedCostUSD: decimal.NewFromFloat(5.00),
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards for session 2 failed: %v", err)
	}

	req := analytics.OrgAnalyticsRequest{
		StartTS:  now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:    now.Add(24 * time.Hour).Unix(),
		TZOffset: 0,
	}

	response, err := store.GetOrgAnalytics(ctx, req)
	if err != nil {
		t.Fatalf("GetOrgAnalytics failed: %v", err)
	}

	if len(response.Users) != 1 {
		t.Fatalf("Users length = %d, want 1", len(response.Users))
	}

	// Only session 1 should be counted (session 2 missing conversation card)
	u := response.Users[0]
	if u.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1 (session with only tokens card excluded)", u.SessionCount)
	}
	if u.TotalCostUSD != "1.00" {
		t.Errorf("TotalCostUSD = %s, want 1.00 (only complete session counted)", u.TotalCostUSD)
	}
}
