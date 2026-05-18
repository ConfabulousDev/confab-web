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
		TZOffset:      0,
		IncludeNoRepo: true,
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
	if response.Users[0].AvgAssistantTimeMs != nil {
		t.Error("expected AvgAssistantTimeMs to be nil for 0 sessions")
	}
	// providers_present is always non-nil; empty here since no qualifying sessions.
	if response.ProvidersPresent == nil {
		t.Error("expected ProvidersPresent to be non-nil, got nil")
	}
	if len(response.ProvidersPresent) != 0 {
		t.Errorf("ProvidersPresent = %v, want empty (no qualifying sessions)", response.ProvidersPresent)
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
		TZOffset:      0,
		IncludeNoRepo: true,
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

	// Alice: 2 sessions, $3.00 total, 70000ms assistant, 140000ms user, 210000ms duration
	if alice.SessionCount != 2 {
		t.Errorf("Alice.SessionCount = %d, want 2", alice.SessionCount)
	}
	if alice.TotalCostUSD != "3.00" {
		t.Errorf("Alice.TotalCostUSD = %s, want 3.00", alice.TotalCostUSD)
	}
	if alice.TotalAssistantTimeMs != 70000 {
		t.Errorf("Alice.TotalAssistantTimeMs = %d, want 70000", alice.TotalAssistantTimeMs)
	}
	if alice.TotalUserTimeMs != 140000 {
		t.Errorf("Alice.TotalUserTimeMs = %d, want 140000", alice.TotalUserTimeMs)
	}
	if alice.TotalDurationMs != 210000 {
		t.Errorf("Alice.TotalDurationMs = %d, want 210000", alice.TotalDurationMs)
	}

	// Alice averages: $1.50, 35000ms assistant, 70000ms user, 105000ms duration
	if alice.AvgCostUSD != "1.50" {
		t.Errorf("Alice.AvgCostUSD = %s, want 1.50", alice.AvgCostUSD)
	}
	if alice.AvgAssistantTimeMs == nil || *alice.AvgAssistantTimeMs != 35000 {
		t.Errorf("Alice.AvgAssistantTimeMs = %v, want 35000", alice.AvgAssistantTimeMs)
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

	// All sessions default to session_type='claude-code' (migration 000043).
	if len(response.ProvidersPresent) != 1 || response.ProvidersPresent[0] != "claude-code" {
		t.Errorf("ProvidersPresent = %v, want [claude-code]", response.ProvidersPresent)
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
		TZOffset:      0,
		IncludeNoRepo: true,
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
		TZOffset:      0,
		IncludeNoRepo: true,
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
		TZOffset:      0,
		IncludeNoRepo: true,
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

// TestGetOrgAnalytics_ProviderFilter asserts that the `Providers` filter on
// OrgAnalyticsRequest narrows session counting to the requested canonical
// providers, and that legacy `Claude Code` rows fold into a `claude-code`
// filter via ExpandWithAliases.
func TestGetOrgAnalytics_ProviderFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())
	user := testutil.CreateTestUser(t, env, "provider-filter@test.com", "Provider Filter User")

	// Three sessions: canonical Claude, legacy Claude, Codex. Distinct costs to
	// keep aggregation arithmetic obvious in failure messages.
	claudeSession := testutil.CreateTestSessionWithProvider(t, env, user.ID, "pf-claude", "claude-code")
	legacySession := testutil.CreateTestSessionLegacyClaudeCode(t, env, user.ID, "pf-legacy")
	codexSession := testutil.CreateTestSessionWithProvider(t, env, user.ID, "pf-codex", "codex")

	now := time.Now().UTC()
	seed := func(sessionID string, cost float64, claudeMs, userMs int64) {
		t.Helper()
		err := store.UpsertCards(ctx, &analytics.Cards{
			Tokens: &analytics.TokensCardRecord{
				SessionID:        sessionID,
				Version:          analytics.TokensCardVersion,
				ComputedAt:       now,
				UpToLine:         100,
				EstimatedCostUSD: decimal.NewFromFloat(cost),
			},
			Conversation: &analytics.ConversationCardRecord{
				SessionID:                sessionID,
				Version:                  analytics.ConversationCardVersion,
				ComputedAt:               now,
				UpToLine:                 100,
				TotalAssistantDurationMs: &claudeMs,
				TotalUserDurationMs:      &userMs,
			},
		})
		if err != nil {
			t.Fatalf("UpsertCards (%s): %v", sessionID, err)
		}
	}
	seed(claudeSession, 6.00, 10000, 20000)
	seed(legacySession, 3.00, 5000, 10000)
	seed(codexSession, 9.00, 15000, 30000)

	cases := []struct {
		name                 string
		providers            []string
		wantSessionCount     int
		wantTotalCostUSD     string
		wantProvidersPresent []string
	}{
		{
			name:                 "nil filter — all sessions",
			providers:            nil,
			wantSessionCount:     3,
			wantTotalCostUSD:     "18.00",
			wantProvidersPresent: []string{"claude-code", "codex"},
		},
		{
			// providers_present reports the unfiltered set so the dropdown can
			// offer codex even when the user has currently filtered to claude.
			name:                 "claude-code only — excludes codex, includes legacy",
			providers:            []string{"claude-code"},
			wantSessionCount:     2,
			wantTotalCostUSD:     "9.00",
			wantProvidersPresent: []string{"claude-code", "codex"},
		},
		{
			name:                 "codex only — excludes claude-code and legacy",
			providers:            []string{"codex"},
			wantSessionCount:     1,
			wantTotalCostUSD:     "9.00",
			wantProvidersPresent: []string{"claude-code", "codex"},
		},
		{
			name:                 "both providers — same as nil filter",
			providers:            []string{"claude-code", "codex"},
			wantSessionCount:     3,
			wantTotalCostUSD:     "18.00",
			wantProvidersPresent: []string{"claude-code", "codex"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := analytics.OrgAnalyticsRequest{
				StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
				EndTS:         now.Add(24 * time.Hour).Unix(),
				TZOffset:      0,
				Providers:     tc.providers,
				IncludeNoRepo: true,
			}

			response, err := store.GetOrgAnalytics(ctx, req)
			if err != nil {
				t.Fatalf("GetOrgAnalytics failed: %v", err)
			}
			if len(response.Users) != 1 {
				t.Fatalf("Users length = %d, want 1", len(response.Users))
			}
			got := response.Users[0]
			if got.SessionCount != tc.wantSessionCount {
				t.Errorf("SessionCount = %d, want %d", got.SessionCount, tc.wantSessionCount)
			}
			if got.TotalCostUSD != tc.wantTotalCostUSD {
				t.Errorf("TotalCostUSD = %s, want %s", got.TotalCostUSD, tc.wantTotalCostUSD)
			}
			if !equalStringSlice(response.ProvidersPresent, tc.wantProvidersPresent) {
				t.Errorf("ProvidersPresent = %v, want %v", response.ProvidersPresent, tc.wantProvidersPresent)
			}
		})
	}
}

// TestGetOrgAnalytics_RepoFilter asserts that the `Repos` + `IncludeNoRepo`
// filter narrows session counting by extracted repo name (owner/name) and
// optionally includes sessions without a repo_url.
func TestGetOrgAnalytics_RepoFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())
	user := testutil.CreateTestUser(t, env, "repo-filter@test.com", "Repo Filter User")

	// Three sessions: one in repo "ConfabulousDev/confab-web", one in
	// "ConfabulousDev/cli", one without a repo_url.
	sid1 := testutil.CreateTestSessionWithGitInfo(t, env, user.ID, "rf-confab", "https://github.com/ConfabulousDev/confab-web.git")
	sid2 := testutil.CreateTestSessionWithGitInfo(t, env, user.ID, "rf-cli", "git@github.com:ConfabulousDev/cli.git")
	sid3 := testutil.CreateTestSession(t, env, user.ID, "rf-norepo")

	now := time.Now().UTC()
	seed := func(sessionID string, cost float64) {
		t.Helper()
		assistantMs := int64(10000)
		userMs := int64(20000)
		err := store.UpsertCards(ctx, &analytics.Cards{
			Tokens: &analytics.TokensCardRecord{
				SessionID:        sessionID,
				Version:          analytics.TokensCardVersion,
				ComputedAt:       now,
				UpToLine:         100,
				EstimatedCostUSD: decimal.NewFromFloat(cost),
			},
			Conversation: &analytics.ConversationCardRecord{
				SessionID:                sessionID,
				Version:                  analytics.ConversationCardVersion,
				ComputedAt:               now,
				UpToLine:                 100,
				TotalAssistantDurationMs: &assistantMs,
				TotalUserDurationMs:      &userMs,
			},
		})
		if err != nil {
			t.Fatalf("UpsertCards (%s): %v", sessionID, err)
		}
	}
	seed(sid1, 2.00)
	seed(sid2, 3.00)
	seed(sid3, 5.00)

	// Filter semantics mirror trends:
	//   include_no_repo = true → also include sessions with NULL/empty repo_url
	//   repos = [...] → include sessions whose extracted owner/name is in the set
	// To match "all sessions" you must pass every repo AND include_no_repo=true,
	// which is the auto-select-all-on-load behavior the page implements.
	cases := []struct {
		name             string
		repos            []string
		includeNoRepo    bool
		wantSessionCount int
		wantTotalCostUSD string
	}{
		{
			name:             "all repos selected + include_no_repo=true — every session",
			repos:            []string{"ConfabulousDev/confab-web", "ConfabulousDev/cli"},
			includeNoRepo:    true,
			wantSessionCount: 3,
			wantTotalCostUSD: "10.00",
		},
		{
			name:             "single repo, no_repo excluded",
			repos:            []string{"ConfabulousDev/confab-web"},
			includeNoRepo:    false,
			wantSessionCount: 1,
			wantTotalCostUSD: "2.00",
		},
		{
			name:             "multiple repos, no_repo excluded",
			repos:            []string{"ConfabulousDev/confab-web", "ConfabulousDev/cli"},
			includeNoRepo:    false,
			wantSessionCount: 2,
			wantTotalCostUSD: "5.00",
		},
		{
			name:             "no repos selected, include_no_repo=true — only sessions without repo",
			repos:            nil,
			includeNoRepo:    true,
			wantSessionCount: 1,
			wantTotalCostUSD: "5.00",
		},
		{
			name:             "no repos selected, include_no_repo=false — empty",
			repos:            nil,
			includeNoRepo:    false,
			wantSessionCount: 0,
			wantTotalCostUSD: "0.00",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := analytics.OrgAnalyticsRequest{
				StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
				EndTS:         now.Add(24 * time.Hour).Unix(),
				TZOffset:      0,
				Repos:         tc.repos,
				IncludeNoRepo: tc.includeNoRepo,
			}

			response, err := store.GetOrgAnalytics(ctx, req)
			if err != nil {
				t.Fatalf("GetOrgAnalytics failed: %v", err)
			}
			if len(response.Users) != 1 {
				t.Fatalf("Users length = %d, want 1", len(response.Users))
			}
			got := response.Users[0]
			if got.SessionCount != tc.wantSessionCount {
				t.Errorf("SessionCount = %d, want %d", got.SessionCount, tc.wantSessionCount)
			}
			if got.TotalCostUSD != tc.wantTotalCostUSD {
				t.Errorf("TotalCostUSD = %s, want %s", got.TotalCostUSD, tc.wantTotalCostUSD)
			}
		})
	}
}

// TestGetOrgAnalytics_RepoAndProviderCoFilter asserts that the repo and
// provider filters combine multiplicatively: a session must satisfy BOTH to
// be counted. Guards against either filter degrading to "all" when the other
// is also set (a class of bug that wouldn't surface in single-axis tests).
func TestGetOrgAnalytics_RepoAndProviderCoFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()
	store := analytics.NewStore(env.DB.Conn())
	user := testutil.CreateTestUser(t, env, "co-filter@test.com", "Co Filter User")

	// 4 sessions:
	//   claude + confab-web
	//   claude + cli
	//   codex  + confab-web
	//   codex  + cli
	type seed struct {
		externalID string
		provider   string
		repoURL    string
		cost       float64
	}
	seeds := []seed{
		{"cf-cw-claude", "claude-code", "https://github.com/ConfabulousDev/confab-web.git", 1.00},
		{"cf-cli-claude", "claude-code", "git@github.com:ConfabulousDev/cli.git", 2.00},
		{"cf-cw-codex", "codex", "https://github.com/ConfabulousDev/confab-web.git", 4.00},
		{"cf-cli-codex", "codex", "git@github.com:ConfabulousDev/cli.git", 8.00},
	}
	now := time.Now().UTC()
	for _, s := range seeds {
		sid := createTestSessionWithProviderAndGit(t, env, user.ID, s.externalID, s.provider, s.repoURL)
		assistantMs := int64(10000)
		userMs := int64(20000)
		err := store.UpsertCards(ctx, &analytics.Cards{
			Tokens: &analytics.TokensCardRecord{
				SessionID:        sid,
				Version:          analytics.TokensCardVersion,
				ComputedAt:       now,
				UpToLine:         100,
				EstimatedCostUSD: decimal.NewFromFloat(s.cost),
			},
			Conversation: &analytics.ConversationCardRecord{
				SessionID:                sid,
				Version:                  analytics.ConversationCardVersion,
				ComputedAt:               now,
				UpToLine:                 100,
				TotalAssistantDurationMs: &assistantMs,
				TotalUserDurationMs:      &userMs,
			},
		})
		if err != nil {
			t.Fatalf("UpsertCards (%s): %v", sid, err)
		}
	}

	// codex + confab-web only → exactly one session, cost $4.00.
	req := analytics.OrgAnalyticsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Providers:     []string{"codex"},
		Repos:         []string{"ConfabulousDev/confab-web"},
		IncludeNoRepo: false,
	}
	response, err := store.GetOrgAnalytics(ctx, req)
	if err != nil {
		t.Fatalf("GetOrgAnalytics failed: %v", err)
	}
	if len(response.Users) != 1 {
		t.Fatalf("Users length = %d, want 1", len(response.Users))
	}
	got := response.Users[0]
	if got.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1 (codex × confab-web is the only intersection)", got.SessionCount)
	}
	if got.TotalCostUSD != "4.00" {
		t.Errorf("TotalCostUSD = %s, want 4.00", got.TotalCostUSD)
	}
	// providers_present ignores the provider filter but still respects the
	// repo filter, so we see both providers present inside confab-web even
	// though the request narrowed to codex.
	if !equalStringSlice(response.ProvidersPresent, []string{"claude-code", "codex"}) {
		t.Errorf("ProvidersPresent = %v, want [claude-code codex]", response.ProvidersPresent)
	}
}

// createTestSessionWithProviderAndGit creates a session with both a custom
// session_type and a git_info->>'repo_url'. The shared testutil helpers cover
// each axis in isolation but not both — and the co-filter test needs the
// combination.
func createTestSessionWithProviderAndGit(t *testing.T, env *testutil.TestEnvironment, userID int64, externalID, provider, repoURL string) string {
	t.Helper()
	sid := testutil.CreateTestSessionWithProvider(t, env, userID, externalID, provider)
	_, err := env.DB.Conn().ExecContext(env.Ctx,
		`UPDATE sessions SET git_info = jsonb_build_object('repo_url', $2::text) WHERE id = $1`,
		sid, repoURL)
	if err != nil {
		t.Fatalf("set git_info on %s: %v", sid, err)
	}
	return sid
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
