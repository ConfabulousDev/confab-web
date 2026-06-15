package analytics_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func rtPtr[T any](v T) *T { return &v }

// rtComputedAt is a microsecond-precision UTC instant so it round-trips through
// Postgres timestamptz (microsecond precision) without truncation loss.
var rtComputedAt = time.Date(2026, 2, 3, 4, 5, 6, 123456000, time.UTC)

// buildAllCards returns a Cards with every card populated with distinctive,
// non-zero values, exercising the non-uniform cards in particular:
//   - tokens: decimal cost columns stored as strings
//   - agents_and_skills: two independent JSONB columns
//   - conversation: all-scalar (nullable pointer) columns, no JSONB
//   - workflows: JSONB slice (also see the nil-runs case in its own test)
func buildAllCards(sessionID string) *analytics.Cards {
	return &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID: sessionID, Version: analytics.TokensCardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			InputTokens: 111, OutputTokens: 222, CacheCreationTokens: 333, CacheReadTokens: 444,
			// DECIMAL(10,4) columns — use 4-decimal values so the round-trip is lossless.
			EstimatedCostUSD: decimal.RequireFromString("1.2345"),
			FastTurns:        7, FastCostUSD: decimal.RequireFromString("0.0099"),
		},
		TokensV2: &analytics.TokensV2CardRecord{
			SessionID: sessionID, Version: analytics.TokensV2CardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			Data: analytics.TokensV2Data{
				TotalCostUSD: "1.25", TotalInput: 111, TotalOutput: 222,
				ByProvider: map[string]analytics.TokensV2Provider{
					"claude-code": {CostUSD: "1.25", Models: map[string]analytics.TokensV2Model{
						"sonnet": {Input: 111, Output: 222, CacheRead: 444, CacheWrite: 333, Reasoning: 5, CostUSD: "1.25"},
					}},
				},
			},
		},
		Session: &analytics.SessionCardRecord{
			SessionID: sessionID, Version: analytics.SessionCardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			TotalMessages: 10, UserMessages: 4, AssistantMessages: 6,
			HumanPrompts: 4, ToolResults: 3, TextResponses: 6, ToolCalls: 8, ThinkingBlocks: 2,
			DurationMs: rtPtr(int64(54321)), ModelsUsed: []string{"sonnet", "haiku"},
			CompactionAuto: 1, CompactionManual: 2, CompactionAvgTimeMs: rtPtr(99),
		},
		Tools: &analytics.ToolsCardRecord{
			SessionID: sessionID, Version: analytics.ToolsCardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			TotalCalls: 9, ErrorCount: 1,
			ToolStats: map[string]*analytics.ToolStats{"Read": {Success: 5, Errors: 0}, "Write": {Success: 3, Errors: 1}},
		},
		CodeActivity: &analytics.CodeActivityCardRecord{
			SessionID: sessionID, Version: analytics.CodeActivityCardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			FilesRead: 12, FilesModified: 5, LinesAdded: 120, LinesRemoved: 34, SearchCount: 7,
			LanguageBreakdown: map[string]int{"go": 8, "ts": 4},
		},
		Conversation: &analytics.ConversationCardRecord{
			SessionID: sessionID, Version: analytics.ConversationCardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			UserTurns: 4, AssistantTurns: 6,
			AvgAssistantTurnMs: rtPtr(int64(1500)), AvgUserThinkingMs: rtPtr(int64(800)),
			TotalAssistantDurationMs: rtPtr(int64(9000)), TotalUserDurationMs: rtPtr(int64(3200)),
			AssistantUtilizationPct: rtPtr(73.5),
		},
		AgentsAndSkills: &analytics.AgentsAndSkillsCardRecord{
			SessionID: sessionID, Version: analytics.AgentsAndSkillsCardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			AgentInvocations: 3, SkillInvocations: 2,
			AgentStats: map[string]*analytics.AgentStats{"explore": {Success: 2, Errors: 1}},
			SkillStats: map[string]*analytics.SkillStats{"commit": {Success: 2, Errors: 0}},
		},
		Redactions: &analytics.RedactionsCardRecord{
			SessionID: sessionID, Version: analytics.RedactionsCardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			TotalRedactions: 5, RedactionCounts: map[string]int{"GITHUB_TOKEN": 3, "API_KEY": 2},
		},
		Workflows: &analytics.WorkflowsCardRecord{
			SessionID: sessionID, Version: analytics.WorkflowsCardVersion, ComputedAt: rtComputedAt, UpToLine: 100,
			Runs: []analytics.WorkflowRun{{
				RunID: "wf_1", AgentCount: 3, InputTokens: 10, OutputTokens: 20, CacheCreation: 5, CacheRead: 7,
				EstimatedUSD: "0.50", SucceededAgents: 2, HasJournal: true, DurationMs: 1234,
			}},
		},
	}
}

// assertCardJSONEqual compares two card records by JSON after normalizing time
// representation to UTC (Postgres timestamptz may come back in a different zone
// for the same instant).
func assertCardJSONEqual(t *testing.T, name string, want, got any) {
	t.Helper()
	wb, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("%s: marshal want: %v", name, err)
	}
	gb, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("%s: marshal got: %v", name, err)
	}
	if string(wb) != string(gb) {
		t.Errorf("%s round-trip mismatch:\n want %s\n  got %s", name, wb, gb)
	}
}

// normalizeTimes forces every card's ComputedAt to UTC so JSON time strings
// compare by instant rather than by zone representation.
func normalizeTimes(c *analytics.Cards) {
	if c.Tokens != nil {
		c.Tokens.ComputedAt = c.Tokens.ComputedAt.UTC()
	}
	if c.TokensV2 != nil {
		c.TokensV2.ComputedAt = c.TokensV2.ComputedAt.UTC()
	}
	if c.Session != nil {
		c.Session.ComputedAt = c.Session.ComputedAt.UTC()
	}
	if c.Tools != nil {
		c.Tools.ComputedAt = c.Tools.ComputedAt.UTC()
	}
	if c.CodeActivity != nil {
		c.CodeActivity.ComputedAt = c.CodeActivity.ComputedAt.UTC()
	}
	if c.Conversation != nil {
		c.Conversation.ComputedAt = c.Conversation.ComputedAt.UTC()
	}
	if c.AgentsAndSkills != nil {
		c.AgentsAndSkills.ComputedAt = c.AgentsAndSkills.ComputedAt.UTC()
	}
	if c.Redactions != nil {
		c.Redactions.ComputedAt = c.Redactions.ComputedAt.UTC()
	}
	if c.Workflows != nil {
		c.Workflows.ComputedAt = c.Workflows.ComputedAt.UTC()
	}
}

func TestStore_UpsertGetCards_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "rt@test.com", "RT User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "rt-external-id")

	store := analytics.NewStore(env.DB.Conn())
	ctx := context.Background()

	in := buildAllCards(sessionID)
	if err := store.UpsertCards(ctx, in); err != nil {
		t.Fatalf("UpsertCards: %v", err)
	}

	got, err := store.GetCards(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetCards: %v", err)
	}

	// Decimal columns compare by value (representation-independent).
	if got.Tokens == nil || !got.Tokens.EstimatedCostUSD.Equal(in.Tokens.EstimatedCostUSD) {
		t.Errorf("tokens EstimatedCostUSD = %v, want %v", got.Tokens.EstimatedCostUSD, in.Tokens.EstimatedCostUSD)
	}
	if got.Tokens == nil || !got.Tokens.FastCostUSD.Equal(in.Tokens.FastCostUSD) {
		t.Errorf("tokens FastCostUSD = %v, want %v", got.Tokens.FastCostUSD, in.Tokens.FastCostUSD)
	}

	// Decimals verified by value above; normalize representation (DECIMAL scale)
	// so the JSON comparison below checks the remaining fields.
	if got.Tokens != nil {
		got.Tokens.EstimatedCostUSD = in.Tokens.EstimatedCostUSD
		got.Tokens.FastCostUSD = in.Tokens.FastCostUSD
	}

	normalizeTimes(in)
	normalizeTimes(got)
	assertCardJSONEqual(t, "tokens", in.Tokens, got.Tokens)
	assertCardJSONEqual(t, "tokens_v2", in.TokensV2, got.TokensV2)
	assertCardJSONEqual(t, "session", in.Session, got.Session)
	assertCardJSONEqual(t, "tools", in.Tools, got.Tools)
	assertCardJSONEqual(t, "code_activity", in.CodeActivity, got.CodeActivity)
	assertCardJSONEqual(t, "conversation", in.Conversation, got.Conversation)
	assertCardJSONEqual(t, "agents_and_skills", in.AgentsAndSkills, got.AgentsAndSkills)
	assertCardJSONEqual(t, "redactions", in.Redactions, got.Redactions)
	assertCardJSONEqual(t, "workflows", in.Workflows, got.Workflows)
}

func TestStore_UpsertWorkflowsCard_NilRunsStoredAsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "rt2@test.com", "RT2 User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "rt2-external-id")

	store := analytics.NewStore(env.DB.Conn())
	ctx := context.Background()

	// nil Runs must persist as an empty JSON array and read back as a non-nil
	// empty slice, never as JSON null.
	if err := store.UpsertCards(ctx, &analytics.Cards{
		Workflows: &analytics.WorkflowsCardRecord{
			SessionID: sessionID, Version: analytics.WorkflowsCardVersion, ComputedAt: rtComputedAt, UpToLine: 50,
			Runs: nil,
		},
	}); err != nil {
		t.Fatalf("UpsertCards: %v", err)
	}

	got, err := store.GetCards(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetCards: %v", err)
	}
	if got.Workflows == nil {
		t.Fatal("workflows card not found")
	}
	if got.Workflows.Runs == nil {
		t.Errorf("workflows Runs = nil, want empty non-nil slice (nil must store as [])")
	}
	if len(got.Workflows.Runs) != 0 {
		t.Errorf("workflows Runs len = %d, want 0", len(got.Workflows.Runs))
	}
}
