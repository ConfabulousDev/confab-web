package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/shopspring/decimal"
)

func TestGetTrends_EmptyResults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-empty@test.com", "Trends Empty User")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// Get trends with no sessions
	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}

	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends failed: %v", err)
	}

	if response.SessionCount != 0 {
		t.Errorf("SessionCount = %d, want 0", response.SessionCount)
	}

	// Cards should be non-nil but with zero values
	if response.Cards.Overview == nil {
		t.Error("expected Overview card to be non-nil")
	} else if response.Cards.Overview.SessionCount != 0 {
		t.Errorf("Overview.SessionCount = %d, want 0", response.Cards.Overview.SessionCount)
	}

	if response.Cards.Tokens == nil {
		t.Error("expected Tokens card to be non-nil")
	}

	if response.Cards.Activity == nil {
		t.Error("expected Activity card to be non-nil")
	}

	if response.Cards.Tools == nil {
		t.Error("expected Tools card to be non-nil")
	}

	if response.Cards.AgentsAndSkills == nil {
		t.Error("expected AgentsAndSkills card to be non-nil")
	} else if response.Cards.AgentsAndSkills.TotalAgentInvocations != 0 {
		t.Errorf("AgentsAndSkills.TotalAgentInvocations = %d, want 0", response.Cards.AgentsAndSkills.TotalAgentInvocations)
	}

	if response.Cards.TopSessions == nil {
		t.Error("expected TopSessions card to be non-nil")
	} else if len(response.Cards.TopSessions.Sessions) != 0 {
		t.Errorf("TopSessions.Sessions length = %d, want 0", len(response.Cards.TopSessions.Sessions))
	}
}

func TestGetTrends_WithSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-sessions@test.com", "Trends Sessions User")
	ctx := context.Background()

	// Create two sessions
	sessionID1 := testutil.CreateTestSession(t, env, user.ID, "test-session-trends-1")
	sessionID2 := testutil.CreateTestSession(t, env, user.ID, "test-session-trends-2")

	store := analytics.NewStore(env.DB.Conn())

	// Insert tokens card for session 1
	tokensCard1 := &analytics.TokensCardRecord{
		SessionID:           sessionID1,
		Version:             analytics.TokensCardVersion,
		ComputedAt:          time.Now().UTC(),
		UpToLine:            100,
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 100,
		CacheReadTokens:     200,
		EstimatedCostUSD:    decimal.NewFromFloat(0.50),
	}
	err := store.UpsertCards(ctx, &analytics.Cards{Tokens: tokensCard1})
	if err != nil {
		t.Fatalf("UpsertCards (session 1) failed: %v", err)
	}

	// Insert tokens card for session 2
	tokensCard2 := &analytics.TokensCardRecord{
		SessionID:           sessionID2,
		Version:             analytics.TokensCardVersion,
		ComputedAt:          time.Now().UTC(),
		UpToLine:            50,
		InputTokens:         2000,
		OutputTokens:        1000,
		CacheCreationTokens: 200,
		CacheReadTokens:     400,
		EstimatedCostUSD:    decimal.NewFromFloat(1.00),
	}
	err = store.UpsertCards(ctx, &analytics.Cards{Tokens: tokensCard2})
	if err != nil {
		t.Fatalf("UpsertCards (session 2) failed: %v", err)
	}

	// Insert code activity for session 1
	codeActivityCard := &analytics.CodeActivityCardRecord{
		SessionID:         sessionID1,
		Version:           analytics.CodeActivityCardVersion,
		ComputedAt:        time.Now().UTC(),
		UpToLine:          100,
		FilesRead:         10,
		FilesModified:     5,
		LinesAdded:        100,
		LinesRemoved:      50,
		SearchCount:       3,
		LanguageBreakdown: map[string]int{"go": 8, "ts": 2},
	}
	err = store.UpsertCards(ctx, &analytics.Cards{CodeActivity: codeActivityCard})
	if err != nil {
		t.Fatalf("UpsertCards (code activity) failed: %v", err)
	}

	// Insert tools card for session 1
	toolsCard := &analytics.ToolsCardRecord{
		SessionID:  sessionID1,
		Version:    analytics.ToolsCardVersion,
		ComputedAt: time.Now().UTC(),
		UpToLine:   100,
		TotalCalls: 20,
		ToolStats: map[string]*analytics.ToolStats{
			"Read":  {Success: 10, Errors: 0},
			"Write": {Success: 8, Errors: 2},
		},
		ErrorCount: 2,
	}
	err = store.UpsertCards(ctx, &analytics.Cards{Tools: toolsCard})
	if err != nil {
		t.Fatalf("UpsertCards (tools) failed: %v", err)
	}

	// Insert agents and skills card for session 1
	agentsCard1 := &analytics.AgentsAndSkillsCardRecord{
		SessionID:        sessionID1,
		Version:          analytics.AgentsAndSkillsCardVersion,
		ComputedAt:       time.Now().UTC(),
		UpToLine:         100,
		AgentInvocations: 5,
		SkillInvocations: 3,
		AgentStats: map[string]*analytics.AgentStats{
			"Explore": {Success: 3, Errors: 0},
			"Plan":    {Success: 2, Errors: 0},
		},
		SkillStats: map[string]*analytics.SkillStats{
			"commit": {Success: 2, Errors: 1},
		},
	}
	err = store.UpsertCards(ctx, &analytics.Cards{AgentsAndSkills: agentsCard1})
	if err != nil {
		t.Fatalf("UpsertCards (agents session 1) failed: %v", err)
	}

	// Get trends
	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}

	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends failed: %v", err)
	}

	if response.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2", response.SessionCount)
	}

	// Check tokens aggregation
	if response.Cards.Tokens.TotalInputTokens != 3000 {
		t.Errorf("TotalInputTokens = %d, want 3000", response.Cards.Tokens.TotalInputTokens)
	}
	if response.Cards.Tokens.TotalOutputTokens != 1500 {
		t.Errorf("TotalOutputTokens = %d, want 1500", response.Cards.Tokens.TotalOutputTokens)
	}
	if response.Cards.Tokens.TotalCostUSD != "1.5" {
		t.Errorf("TotalCostUSD = %s, want 1.5", response.Cards.Tokens.TotalCostUSD)
	}

	// Check activity aggregation
	if response.Cards.Activity.TotalFilesRead != 10 {
		t.Errorf("TotalFilesRead = %d, want 10", response.Cards.Activity.TotalFilesRead)
	}
	if response.Cards.Activity.TotalLinesAdded != 100 {
		t.Errorf("TotalLinesAdded = %d, want 100", response.Cards.Activity.TotalLinesAdded)
	}

	// Check tools aggregation
	if response.Cards.Tools.TotalCalls != 20 {
		t.Errorf("TotalCalls = %d, want 20", response.Cards.Tools.TotalCalls)
	}
	if response.Cards.Tools.TotalErrors != 2 {
		t.Errorf("TotalErrors = %d, want 2", response.Cards.Tools.TotalErrors)
	}

	// Check top sessions — both sessions have cost > 0, ordered descending
	if response.Cards.TopSessions == nil {
		t.Fatal("expected TopSessions card to be non-nil")
	}
	if len(response.Cards.TopSessions.Sessions) != 2 {
		t.Fatalf("TopSessions.Sessions length = %d, want 2", len(response.Cards.TopSessions.Sessions))
	}
	// Session 2 ($1.00) should be first, session 1 ($0.50) second
	if response.Cards.TopSessions.Sessions[0].EstimatedCostUSD != "1" {
		t.Errorf("TopSessions[0].EstimatedCostUSD = %s, want 1", response.Cards.TopSessions.Sessions[0].EstimatedCostUSD)
	}
	if response.Cards.TopSessions.Sessions[1].EstimatedCostUSD != "0.5" {
		t.Errorf("TopSessions[1].EstimatedCostUSD = %s, want 0.5", response.Cards.TopSessions.Sessions[1].EstimatedCostUSD)
	}

	// Check agents and skills aggregation
	if response.Cards.AgentsAndSkills == nil {
		t.Fatal("expected AgentsAndSkills card to be non-nil")
	}
	if response.Cards.AgentsAndSkills.TotalAgentInvocations != 5 {
		t.Errorf("TotalAgentInvocations = %d, want 5", response.Cards.AgentsAndSkills.TotalAgentInvocations)
	}
	if response.Cards.AgentsAndSkills.TotalSkillInvocations != 3 {
		t.Errorf("TotalSkillInvocations = %d, want 3", response.Cards.AgentsAndSkills.TotalSkillInvocations)
	}
	if explore, ok := response.Cards.AgentsAndSkills.AgentStats["Explore"]; !ok {
		t.Error("expected AgentStats to contain 'Explore'")
	} else if explore.Success != 3 {
		t.Errorf("Explore.Success = %d, want 3", explore.Success)
	}
	if commit, ok := response.Cards.AgentsAndSkills.SkillStats["commit"]; !ok {
		t.Error("expected SkillStats to contain 'commit'")
	} else if commit.Errors != 1 {
		t.Errorf("commit.Errors = %d, want 1", commit.Errors)
	}
}

func TestGetTrends_RepoFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-repo@test.com", "Trends Repo User")
	ctx := context.Background()

	// Create session with git info
	sessionID := testutil.CreateTestSessionWithGitInfo(t, env, user.ID, "test-session-repo", "org/repo1")

	store := analytics.NewStore(env.DB.Conn())

	// Insert tokens card
	tokensCard := &analytics.TokensCardRecord{
		SessionID:        sessionID,
		Version:          analytics.TokensCardVersion,
		ComputedAt:       time.Now().UTC(),
		UpToLine:         100,
		InputTokens:      1000,
		OutputTokens:     500,
		EstimatedCostUSD: decimal.NewFromFloat(0.50),
	}
	err := store.UpsertCards(ctx, &analytics.Cards{Tokens: tokensCard})
	if err != nil {
		t.Fatalf("UpsertCards failed: %v", err)
	}

	now := time.Now().UTC()

	// Filter by matching repo
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{"org/repo1"},
		IncludeNoRepo: false,
	}

	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends (matching repo) failed: %v", err)
	}

	if response.SessionCount != 1 {
		t.Errorf("SessionCount (matching repo) = %d, want 1", response.SessionCount)
	}

	// Filter by non-matching repo
	req2 := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{"org/other-repo"},
		IncludeNoRepo: false,
	}

	response2, err := store.GetTrends(ctx, user.ID, req2)
	if err != nil {
		t.Fatalf("GetTrends (non-matching repo) failed: %v", err)
	}

	if response2.SessionCount != 0 {
		t.Errorf("SessionCount (non-matching repo) = %d, want 0", response2.SessionCount)
	}
}

func TestGetTrends_DateRange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-dates@test.com", "Trends Dates User")
	ctx := context.Background()

	// Create sessions with different dates (we'll use the default first_seen = NOW())
	_ = testutil.CreateTestSession(t, env, user.ID, "test-session-today")

	store := analytics.NewStore(env.DB.Conn())

	now := time.Now().UTC()

	// Query for today only (UTC midnight to midnight)
	todayMidnight := now.Truncate(24 * time.Hour)
	req := analytics.TrendsRequest{
		StartTS:       todayMidnight.Unix(),
		EndTS:         todayMidnight.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}

	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends (today) failed: %v", err)
	}

	if response.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1", response.SessionCount)
	}

	// Query for yesterday (should be empty)
	yesterdayMidnight := todayMidnight.Add(-24 * time.Hour)
	req2 := analytics.TrendsRequest{
		StartTS:       yesterdayMidnight.Unix(),
		EndTS:         todayMidnight.Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}

	response2, err := store.GetTrends(ctx, user.ID, req2)
	if err != nil {
		t.Fatalf("GetTrends (yesterday) failed: %v", err)
	}

	if response2.SessionCount != 0 {
		t.Errorf("SessionCount (yesterday) = %d, want 0", response2.SessionCount)
	}
}

func TestGetTrends_RepoFilterScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-repo-scenarios@test.com", "Trends Repo Scenarios User")
	ctx := context.Background()

	// Create session WITH repo (repo1)
	sessionWithRepo1 := testutil.CreateTestSessionWithGitInfo(t, env, user.ID, "session-with-repo1", "org/repo1")
	// Create session WITH different repo (repo2)
	sessionWithRepo2 := testutil.CreateTestSessionWithGitInfo(t, env, user.ID, "session-with-repo2", "org/repo2")
	// Create session WITHOUT repo
	sessionNoRepo := testutil.CreateTestSession(t, env, user.ID, "session-no-repo")

	store := analytics.NewStore(env.DB.Conn())

	// Insert tokens cards for all sessions so we can verify aggregation
	for i, sid := range []string{sessionWithRepo1, sessionWithRepo2, sessionNoRepo} {
		tokensCard := &analytics.TokensCardRecord{
			SessionID:        sid,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         100,
			InputTokens:      int64(1000 * (i + 1)),
			OutputTokens:     int64(500 * (i + 1)),
			EstimatedCostUSD: decimal.NewFromFloat(0.50 * float64(i+1)),
		}
		err := store.UpsertCards(ctx, &analytics.Cards{Tokens: tokensCard})
		if err != nil {
			t.Fatalf("UpsertCards failed: %v", err)
		}
	}

	now := time.Now().UTC()
	startTS := now.Add(-7 * 24 * time.Hour).Unix()
	endTS := now.Add(24 * time.Hour).Unix()

	t.Run("explicit all repos + includeNoRepo=true should return ALL sessions", func(t *testing.T) {
		req := analytics.TrendsRequest{
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      0,
			Repos:         []string{"org/repo1", "org/repo2"},
			IncludeNoRepo: true,
		}
		response, err := store.GetTrends(ctx, user.ID, req)
		if err != nil {
			t.Fatalf("GetTrends failed: %v", err)
		}
		if response.SessionCount != 3 {
			t.Errorf("SessionCount = %d, want 3", response.SessionCount)
		}
		// Total tokens: 1000+2000+3000 = 6000
		if response.Cards.Tokens.TotalInputTokens != 6000 {
			t.Errorf("TotalInputTokens = %d, want 6000", response.Cards.Tokens.TotalInputTokens)
		}
	})

	t.Run("explicit all repos + includeNoRepo=false should return only sessions WITH repos", func(t *testing.T) {
		req := analytics.TrendsRequest{
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      0,
			Repos:         []string{"org/repo1", "org/repo2"},
			IncludeNoRepo: false,
		}
		response, err := store.GetTrends(ctx, user.ID, req)
		if err != nil {
			t.Fatalf("GetTrends failed: %v", err)
		}
		if response.SessionCount != 2 {
			t.Errorf("SessionCount = %d, want 2 (sessions with repos only)", response.SessionCount)
		}
		// Total tokens: 1000+2000 = 3000 (repo1 + repo2, excluding no-repo)
		if response.Cards.Tokens.TotalInputTokens != 3000 {
			t.Errorf("TotalInputTokens = %d, want 3000", response.Cards.Tokens.TotalInputTokens)
		}
	})

	t.Run("empty repos + includeNoRepo=true should return only no-repo sessions", func(t *testing.T) {
		req := analytics.TrendsRequest{
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      0,
			Repos:         []string{},
			IncludeNoRepo: true,
		}
		response, err := store.GetTrends(ctx, user.ID, req)
		if err != nil {
			t.Fatalf("GetTrends failed: %v", err)
		}
		if response.SessionCount != 1 {
			t.Errorf("SessionCount = %d, want 1 (only no-repo session)", response.SessionCount)
		}
		// Total tokens: 3000 (no-repo session only)
		if response.Cards.Tokens.TotalInputTokens != 3000 {
			t.Errorf("TotalInputTokens = %d, want 3000", response.Cards.Tokens.TotalInputTokens)
		}
	})

	t.Run("empty repos + includeNoRepo=false should return no sessions", func(t *testing.T) {
		req := analytics.TrendsRequest{
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      0,
			Repos:         []string{},
			IncludeNoRepo: false,
		}
		response, err := store.GetTrends(ctx, user.ID, req)
		if err != nil {
			t.Fatalf("GetTrends failed: %v", err)
		}
		if response.SessionCount != 0 {
			t.Errorf("SessionCount = %d, want 0 (no repos specified, includeNoRepo=false)", response.SessionCount)
		}
	})

	t.Run("specific repo + includeNoRepo=true should return matching repo AND no-repo sessions", func(t *testing.T) {
		req := analytics.TrendsRequest{
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      0,
			Repos:         []string{"org/repo1"},
			IncludeNoRepo: true,
		}
		response, err := store.GetTrends(ctx, user.ID, req)
		if err != nil {
			t.Fatalf("GetTrends failed: %v", err)
		}
		if response.SessionCount != 2 {
			t.Errorf("SessionCount = %d, want 2 (repo1 + no-repo)", response.SessionCount)
		}
		// Total tokens: 1000+3000 = 4000 (repo1 + no-repo)
		if response.Cards.Tokens.TotalInputTokens != 4000 {
			t.Errorf("TotalInputTokens = %d, want 4000", response.Cards.Tokens.TotalInputTokens)
		}
	})

	t.Run("specific repo + includeNoRepo=false should return only matching repo", func(t *testing.T) {
		req := analytics.TrendsRequest{
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      0,
			Repos:         []string{"org/repo1"},
			IncludeNoRepo: false,
		}
		response, err := store.GetTrends(ctx, user.ID, req)
		if err != nil {
			t.Fatalf("GetTrends failed: %v", err)
		}
		if response.SessionCount != 1 {
			t.Errorf("SessionCount = %d, want 1 (repo1 only)", response.SessionCount)
		}
		// Total tokens: 1000 (repo1 only)
		if response.Cards.Tokens.TotalInputTokens != 1000 {
			t.Errorf("TotalInputTokens = %d, want 1000", response.Cards.Tokens.TotalInputTokens)
		}
	})

	t.Run("multiple repos should return sessions matching any of them", func(t *testing.T) {
		req := analytics.TrendsRequest{
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      0,
			Repos:         []string{"org/repo1", "org/repo2"},
			IncludeNoRepo: false,
		}
		response, err := store.GetTrends(ctx, user.ID, req)
		if err != nil {
			t.Fatalf("GetTrends failed: %v", err)
		}
		if response.SessionCount != 2 {
			t.Errorf("SessionCount = %d, want 2 (repo1 + repo2)", response.SessionCount)
		}
		// Total tokens: 1000+2000 = 3000
		if response.Cards.Tokens.TotalInputTokens != 3000 {
			t.Errorf("TotalInputTokens = %d, want 3000", response.Cards.Tokens.TotalInputTokens)
		}
	})

	t.Run("non-matching repo should return no sessions", func(t *testing.T) {
		req := analytics.TrendsRequest{
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      0,
			Repos:         []string{"org/nonexistent"},
			IncludeNoRepo: false,
		}
		response, err := store.GetTrends(ctx, user.ID, req)
		if err != nil {
			t.Fatalf("GetTrends failed: %v", err)
		}
		if response.SessionCount != 0 {
			t.Errorf("SessionCount = %d, want 0", response.SessionCount)
		}
	})
}

func TestGetTrends_DifferentUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "trends-user1@test.com", "User 1")
	user2 := testutil.CreateTestUser(t, env, "trends-user2@test.com", "User 2")
	ctx := context.Background()

	// Create session for user 1 only
	_ = testutil.CreateTestSession(t, env, user1.ID, "test-session-user1")

	store := analytics.NewStore(env.DB.Conn())

	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}

	// User 1 should see the session
	response1, err := store.GetTrends(ctx, user1.ID, req)
	if err != nil {
		t.Fatalf("GetTrends (user 1) failed: %v", err)
	}
	if response1.SessionCount != 1 {
		t.Errorf("User 1 SessionCount = %d, want 1", response1.SessionCount)
	}

	// User 2 should not see the session
	response2, err := store.GetTrends(ctx, user2.ID, req)
	if err != nil {
		t.Fatalf("GetTrends (user 2) failed: %v", err)
	}
	if response2.SessionCount != 0 {
		t.Errorf("User 2 SessionCount = %d, want 0", response2.SessionCount)
	}
}

func TestGetTrends_AgentsAndSkillsAggregation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-agents@test.com", "Trends Agents User")
	ctx := context.Background()

	sessionID1 := testutil.CreateTestSession(t, env, user.ID, "test-session-agents-1")
	sessionID2 := testutil.CreateTestSession(t, env, user.ID, "test-session-agents-2")
	sessionID3 := testutil.CreateTestSession(t, env, user.ID, "test-session-agents-3")

	store := analytics.NewStore(env.DB.Conn())

	// Session 1: agents and skills
	err := store.UpsertCards(ctx, &analytics.Cards{
		AgentsAndSkills: &analytics.AgentsAndSkillsCardRecord{
			SessionID:        sessionID1,
			Version:          analytics.AgentsAndSkillsCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         100,
			AgentInvocations: 5,
			SkillInvocations: 3,
			AgentStats: map[string]*analytics.AgentStats{
				"Explore": {Success: 3, Errors: 1},
				"Plan":    {Success: 1, Errors: 0},
			},
			SkillStats: map[string]*analytics.SkillStats{
				"commit":    {Success: 2, Errors: 0},
				"review-pr": {Success: 1, Errors: 0},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards (session 1) failed: %v", err)
	}

	// Session 2: same agent names should merge
	err = store.UpsertCards(ctx, &analytics.Cards{
		AgentsAndSkills: &analytics.AgentsAndSkillsCardRecord{
			SessionID:        sessionID2,
			Version:          analytics.AgentsAndSkillsCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         50,
			AgentInvocations: 8,
			SkillInvocations: 2,
			AgentStats: map[string]*analytics.AgentStats{
				"Explore": {Success: 5, Errors: 0},
				"Bash":    {Success: 3, Errors: 0},
			},
			SkillStats: map[string]*analytics.SkillStats{
				"commit": {Success: 1, Errors: 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards (session 2) failed: %v", err)
	}

	// Session 3: no agents and skills card (should not affect totals)
	_ = sessionID3

	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}

	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends failed: %v", err)
	}

	if response.Cards.AgentsAndSkills == nil {
		t.Fatal("expected AgentsAndSkills card to be non-nil")
	}

	card := response.Cards.AgentsAndSkills

	// Check totals: 5+8 = 13 agents, 3+2 = 5 skills
	if card.TotalAgentInvocations != 13 {
		t.Errorf("TotalAgentInvocations = %d, want 13", card.TotalAgentInvocations)
	}
	if card.TotalSkillInvocations != 5 {
		t.Errorf("TotalSkillInvocations = %d, want 5", card.TotalSkillInvocations)
	}

	// Check agent stats merging: Explore = (3+1) + (5+0) = 8 success, 1 error
	if explore, ok := card.AgentStats["Explore"]; !ok {
		t.Error("expected AgentStats to contain 'Explore'")
	} else {
		if explore.Success != 8 {
			t.Errorf("Explore.Success = %d, want 8", explore.Success)
		}
		if explore.Errors != 1 {
			t.Errorf("Explore.Errors = %d, want 1", explore.Errors)
		}
	}

	// Check Plan only from session 1
	if plan, ok := card.AgentStats["Plan"]; !ok {
		t.Error("expected AgentStats to contain 'Plan'")
	} else if plan.Success != 1 {
		t.Errorf("Plan.Success = %d, want 1", plan.Success)
	}

	// Check Bash only from session 2
	if bash, ok := card.AgentStats["Bash"]; !ok {
		t.Error("expected AgentStats to contain 'Bash'")
	} else if bash.Success != 3 {
		t.Errorf("Bash.Success = %d, want 3", bash.Success)
	}

	// Check skill stats merging: commit = (2+0) + (1+1) = 3 success, 1 error
	if commit, ok := card.SkillStats["commit"]; !ok {
		t.Error("expected SkillStats to contain 'commit'")
	} else {
		if commit.Success != 3 {
			t.Errorf("commit.Success = %d, want 3", commit.Success)
		}
		if commit.Errors != 1 {
			t.Errorf("commit.Errors = %d, want 1", commit.Errors)
		}
	}

	// Check review-pr only from session 1
	if reviewPR, ok := card.SkillStats["review-pr"]; !ok {
		t.Error("expected SkillStats to contain 'review-pr'")
	} else if reviewPR.Success != 1 {
		t.Errorf("review-pr.Success = %d, want 1", reviewPR.Success)
	}
}

func TestGetTrends_AgentsAndSkillsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-agents-empty@test.com", "Trends Agents Empty User")
	ctx := context.Background()

	// Create a session with no agents_and_skills card
	_ = testutil.CreateTestSession(t, env, user.ID, "test-session-no-agents")

	store := analytics.NewStore(env.DB.Conn())

	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}

	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends failed: %v", err)
	}

	// AgentsAndSkills should be non-nil but with zero totals and empty maps
	if response.Cards.AgentsAndSkills == nil {
		t.Fatal("expected AgentsAndSkills card to be non-nil")
	}
	if response.Cards.AgentsAndSkills.TotalAgentInvocations != 0 {
		t.Errorf("TotalAgentInvocations = %d, want 0", response.Cards.AgentsAndSkills.TotalAgentInvocations)
	}
	if response.Cards.AgentsAndSkills.TotalSkillInvocations != 0 {
		t.Errorf("TotalSkillInvocations = %d, want 0", response.Cards.AgentsAndSkills.TotalSkillInvocations)
	}
	if len(response.Cards.AgentsAndSkills.AgentStats) != 0 {
		t.Errorf("AgentStats length = %d, want 0", len(response.Cards.AgentsAndSkills.AgentStats))
	}
	if len(response.Cards.AgentsAndSkills.SkillStats) != 0 {
		t.Errorf("SkillStats length = %d, want 0", len(response.Cards.AgentsAndSkills.SkillStats))
	}
}

func TestGetTrends_TimezoneOffset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-tz@test.com", "Trends TZ User")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// Create a session at a known UTC time
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-session-tz")

	// Insert a tokens card so we have data to aggregate
	err := store.UpsertCards(ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        sessionID,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         10,
			InputTokens:      100,
			OutputTokens:     50,
			EstimatedCostUSD: decimal.NewFromFloat(0.01),
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards failed: %v", err)
	}

	now := time.Now().UTC()

	// Query with UTC offset (tz_offset=0) — should find the session
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}
	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends (UTC) failed: %v", err)
	}
	if response.SessionCount != 1 {
		t.Errorf("SessionCount (UTC) = %d, want 1", response.SessionCount)
	}

	// Query with PST offset (tz_offset=480, UTC-8) — wide range should still find it
	req2 := analytics.TrendsRequest{
		StartTS:       now.Add(-24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      480,
		Repos:         []string{},
		IncludeNoRepo: true,
	}
	response2, err := store.GetTrends(ctx, user.ID, req2)
	if err != nil {
		t.Fatalf("GetTrends (PST) failed: %v", err)
	}
	if response2.SessionCount != 1 {
		t.Errorf("SessionCount (PST) = %d, want 1", response2.SessionCount)
	}

	// Query with JST offset (tz_offset=-540, UTC+9) — wide range should still find it
	req3 := analytics.TrendsRequest{
		StartTS:       now.Add(-24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      -540,
		Repos:         []string{},
		IncludeNoRepo: true,
	}
	response3, err := store.GetTrends(ctx, user.ID, req3)
	if err != nil {
		t.Fatalf("GetTrends (JST) failed: %v", err)
	}
	if response3.SessionCount != 1 {
		t.Errorf("SessionCount (JST) = %d, want 1", response3.SessionCount)
	}
}

func TestGetTrends_TopSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-top-sessions@test.com", "Trends TopSessions User")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// Session 1: has summary as title, with repo, cost $5.00
	session1 := testutil.CreateTestSessionFull(t, env, user.ID, "top-session-1", testutil.TestSessionFullOpts{
		RepoURL:          "https://github.com/org/expensive-repo.git",
		Summary:          "Implement dark mode",
		FirstUserMessage: "Add dark mode to settings",
	})
	err := store.UpsertCards(ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        session1,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         100,
			InputTokens:      50000,
			OutputTokens:     25000,
			EstimatedCostUSD: decimal.NewFromFloat(5.00),
		},
		Session: &analytics.SessionCardRecord{
			SessionID:  session1,
			Version:    analytics.SessionCardVersion,
			ComputedAt: time.Now().UTC(),
			UpToLine:   100,
			DurationMs: int64Ptr(3600000), // 1 hour
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards (session 1) failed: %v", err)
	}

	// Session 2: has first_user_message only (no summary), no repo, cost $10.00 (most expensive)
	session2 := testutil.CreateTestSession(t, env, user.ID, "top-session-2")
	// Set first_user_message directly
	_, err = env.DB.Exec(env.Ctx, `UPDATE sessions SET first_user_message = $1 WHERE id = $2`, "Debug auth flow", session2)
	if err != nil {
		t.Fatalf("failed to set first_user_message: %v", err)
	}
	err = store.UpsertCards(ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        session2,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         200,
			InputTokens:      100000,
			OutputTokens:     50000,
			EstimatedCostUSD: decimal.NewFromFloat(10.00),
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards (session 2) failed: %v", err)
	}

	// Session 3: no title fields at all, cost $2.50
	session3 := testutil.CreateTestSession(t, env, user.ID, "top-session-3")
	err = store.UpsertCards(ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        session3,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         50,
			InputTokens:      25000,
			OutputTokens:     12500,
			EstimatedCostUSD: decimal.NewFromFloat(2.50),
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards (session 3) failed: %v", err)
	}

	// Session 4: cost $0.00 (should be excluded)
	session4 := testutil.CreateTestSession(t, env, user.ID, "top-session-4")
	err = store.UpsertCards(ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        session4,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         10,
			InputTokens:      100,
			OutputTokens:     50,
			EstimatedCostUSD: decimal.NewFromFloat(0.00),
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards (session 4) failed: %v", err)
	}

	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{"org/expensive-repo"},
		IncludeNoRepo: true,
	}

	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends failed: %v", err)
	}

	if response.Cards.TopSessions == nil {
		t.Fatal("expected TopSessions card to be non-nil")
	}

	sessions := response.Cards.TopSessions.Sessions

	// Should have 3 sessions (session 4 with $0 cost excluded)
	if len(sessions) != 3 {
		t.Fatalf("TopSessions length = %d, want 3", len(sessions))
	}

	// Verify descending cost order: $10, $5, $2.50
	if sessions[0].EstimatedCostUSD != "10" {
		t.Errorf("sessions[0].EstimatedCostUSD = %s, want 10", sessions[0].EstimatedCostUSD)
	}
	if sessions[1].EstimatedCostUSD != "5" {
		t.Errorf("sessions[1].EstimatedCostUSD = %s, want 5", sessions[1].EstimatedCostUSD)
	}
	if sessions[2].EstimatedCostUSD != "2.5" {
		t.Errorf("sessions[2].EstimatedCostUSD = %s, want 2.5", sessions[2].EstimatedCostUSD)
	}

	// Verify title resolution
	// Session 2 ($10): first_user_message = "Debug auth flow"
	if sessions[0].Title != "Debug auth flow" {
		t.Errorf("sessions[0].Title = %q, want %q", sessions[0].Title, "Debug auth flow")
	}
	// Session 1 ($5): summary = "Implement dark mode" (takes precedence over first_user_message)
	if sessions[1].Title != "Implement dark mode" {
		t.Errorf("sessions[1].Title = %q, want %q", sessions[1].Title, "Implement dark mode")
	}
	// Session 3 ($2.50): no title fields, should fallback
	if sessions[2].Title != "Untitled session - top-sess" {
		t.Errorf("sessions[2].Title = %q, want %q", sessions[2].Title, "Untitled session - top-sess")
	}

	// Verify git_repo populated for session 1, nil for session 2
	if sessions[1].GitRepo == nil || *sessions[1].GitRepo != "org/expensive-repo" {
		t.Errorf("sessions[1].GitRepo = %v, want 'org/expensive-repo'", sessions[1].GitRepo)
	}
	if sessions[0].GitRepo != nil {
		t.Errorf("sessions[0].GitRepo = %v, want nil", sessions[0].GitRepo)
	}

	// Verify duration populated for session 1, nil for session 2
	if sessions[1].DurationMs == nil || *sessions[1].DurationMs != 3600000 {
		t.Errorf("sessions[1].DurationMs = %v, want 3600000", sessions[1].DurationMs)
	}
	if sessions[0].DurationMs != nil {
		t.Errorf("sessions[0].DurationMs = %v, want nil", sessions[0].DurationMs)
	}

	// Verify IDs are UUIDs
	if sessions[0].ID != session2 {
		t.Errorf("sessions[0].ID = %s, want %s", sessions[0].ID, session2)
	}
	if sessions[1].ID != session1 {
		t.Errorf("sessions[1].ID = %s, want %s", sessions[1].ID, session1)
	}
}

func TestGetTrends_TopSessionsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "trends-top-empty@test.com", "Trends TopEmpty User")
	ctx := context.Background()

	// Create a session with no tokens card
	_ = testutil.CreateTestSession(t, env, user.ID, "session-no-tokens")

	store := analytics.NewStore(env.DB.Conn())

	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}

	response, err := store.GetTrends(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends failed: %v", err)
	}

	if response.Cards.TopSessions == nil {
		t.Fatal("expected TopSessions card to be non-nil")
	}
	if len(response.Cards.TopSessions.Sessions) != 0 {
		t.Errorf("TopSessions.Sessions length = %d, want 0", len(response.Cards.TopSessions.Sessions))
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
