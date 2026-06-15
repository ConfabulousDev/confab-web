package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/shopspring/decimal"
)

// costByModelFixture seeds a visible session set spanning all three providers
// plus an Unknown-key session and a session with NO v2 data, then returns the
// store + a request covering the range. Shared by the cost-by-model tests.
//
// Seeded per-model costs (decimal strings):
//
//	claude-code:  opus-4-5 = 1.00, "opus-4-5 · fast" = 0.50
//	codex:        gpt-5    = 2.00
//	opencode:     anthropic/"claude-opus-4-5-20251101" = 0.30  (raw → opus-4-5)
//	              openai/"gpt-5-2026-05-01"            = 0.20  (raw → gpt-5)
//	claude-code:  "" (Unknown) = 0.00
//	(one more session with no v2 row at all → not covered)
func costByModelFixture(t *testing.T) (*analytics.Store, int64, analytics.TrendsRequest) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "trends-cbm@test.com", "Trends CBM User")

	claudeID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cbm-claude", models.ProviderClaudeCode)
	codexID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cbm-codex", models.ProviderCodex)
	opencodeID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cbm-opencode", models.ProviderOpencode)
	unknownID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cbm-unknown", models.ProviderClaudeCode)
	_ = testutil.CreateTestSessionWithProvider(t, env, user.ID, "cbm-no-v2", models.ProviderCodex)

	// 0407: the model dropdown only surfaces listable sessions; make every
	// v2-carrying fixture session listable so it remains a dropdown source.
	for _, id := range []string{claudeID, codexID, opencodeID, unknownID} {
		testutil.MakeSessionListable(t, env, id)
	}

	testutil.SeedTokensV2Card(t, env, claudeID, analytics.TokensV2Data{
		TotalCostUSD: "1.50", TotalInput: 1200, TotalOutput: 600,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "1.50", Models: map[string]analytics.TokensV2Model{
				"opus-4-5":        {Input: 1000, Output: 500, CacheRead: 100, CacheWrite: 50, CostUSD: "1.00"},
				"opus-4-5 · fast": {Input: 200, Output: 100, CostUSD: "0.50"},
			}},
		},
	})
	testutil.SeedTokensV2Card(t, env, codexID, analytics.TokensV2Data{
		TotalCostUSD: "2.00", TotalInput: 2000, TotalOutput: 800,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderCodex: {CostUSD: "2.00", Models: map[string]analytics.TokensV2Model{
				"gpt-5": {Input: 2000, Output: 800, CostUSD: "2.00"},
			}},
		},
	})
	testutil.SeedTokensV2Card(t, env, opencodeID, analytics.TokensV2Data{
		TotalCostUSD: "0.50", TotalInput: 700, TotalOutput: 250,
		ByProvider: map[string]analytics.TokensV2Provider{
			"anthropic": {CostUSD: "0.30", Models: map[string]analytics.TokensV2Model{
				"claude-opus-4-5-20251101": {Input: 300, Output: 100, CostUSD: "0.30"},
			}},
			"openai": {CostUSD: "0.20", Models: map[string]analytics.TokensV2Model{
				"gpt-5-2026-05-01": {Input: 400, Output: 150, CostUSD: "0.20"},
			}},
		},
	})
	testutil.SeedTokensV2Card(t, env, unknownID, analytics.TokensV2Data{
		TotalCostUSD: "0.00", TotalInput: 50, TotalOutput: 10,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0.00", Models: map[string]analytics.TokensV2Model{
				"": {Input: 50, Output: 10, CostUSD: "0.00"},
			}},
		},
	})

	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		TZOffset:      0,
		Repos:         []string{},
		IncludeNoRepo: true,
	}
	return analytics.NewStore(env.DB.Conn()), user.ID, req
}

func findRow(rows []analytics.CostByModelRow, provider, model string) (analytics.CostByModelRow, bool) {
	for _, r := range rows {
		if r.Provider == provider && r.Model == model {
			return r, true
		}
	}
	return analytics.CostByModelRow{}, false
}

func decEq(t *testing.T, got, want string) bool {
	t.Helper()
	g, err := decimal.NewFromString(got)
	if err != nil {
		t.Fatalf("bad decimal %q: %v", got, err)
	}
	w := decimal.RequireFromString(want)
	return g.Equal(w)
}

// TestGetTrends_CostByModel_Grouping is the core correctness test: per-(provider,
// model) bucketing, OpenCode raw-key collapse to family under the canonical
// opencode provider, same family across providers staying as separate rows, the
// fast variant and Unknown rows, cost-desc ordering, and pct summing to ~100.
func TestGetTrends_CostByModel_Grouping(t *testing.T) {
	store, userID, req := costByModelFixture(t)

	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	card := resp.Cards.CostByModel
	if card == nil {
		t.Fatal("expected CostByModel card to be non-nil")
	}
	if card.TimedOut {
		t.Error("expected TimedOut=false")
	}

	// OpenCode's raw vendor key must collapse to the family AND attribute to the
	// canonical 'opencode' provider (not 'anthropic' / 'claude-code').
	oc, ok := findRow(card.Rows, models.ProviderOpencode, "opus-4-5")
	if !ok {
		t.Fatalf("missing (opencode, opus-4-5) row; got rows %+v", card.Rows)
	}
	if !decEq(t, oc.CostUSD, "0.30") {
		t.Errorf("(opencode, opus-4-5) cost = %q, want 0.30", oc.CostUSD)
	}
	// Same family under a different provider is a distinct row.
	if _, ok := findRow(card.Rows, models.ProviderClaudeCode, "opus-4-5"); !ok {
		t.Error("expected a separate (claude-code, opus-4-5) row")
	}
	// Fast variant is its own row; Unknown ("") row present.
	if _, ok := findRow(card.Rows, models.ProviderClaudeCode, "opus-4-5 · fast"); !ok {
		t.Error("expected (claude-code, opus-4-5 · fast) row")
	}
	if _, ok := findRow(card.Rows, models.ProviderClaudeCode, ""); !ok {
		t.Error("expected Unknown (empty-model) row")
	}
	// opencode openai raw → gpt-5 family.
	if _, ok := findRow(card.Rows, models.ProviderOpencode, "gpt-5"); !ok {
		t.Error("expected (opencode, gpt-5) row from openai raw key")
	}

	if len(card.Rows) != 6 {
		t.Errorf("len(Rows) = %d, want 6; rows %+v", len(card.Rows), card.Rows)
	}
	// Cost-desc: the costliest row is codex gpt-5 (2.00).
	if len(card.Rows) > 0 && (card.Rows[0].Provider != models.ProviderCodex || card.Rows[0].Model != "gpt-5") {
		t.Errorf("first row = (%s, %s), want (codex, gpt-5)", card.Rows[0].Provider, card.Rows[0].Model)
	}
	for i := 1; i < len(card.Rows); i++ {
		if decimal.RequireFromString(card.Rows[i-1].CostUSD).LessThan(decimal.RequireFromString(card.Rows[i].CostUSD)) {
			t.Errorf("rows not sorted cost-desc at %d: %s < %s", i, card.Rows[i-1].CostUSD, card.Rows[i].CostUSD)
		}
	}

	// pct_of_total over ALL rows (incl. ~$0 Unknown) sums to ~100.
	var pct float64
	for _, r := range card.Rows {
		pct += r.PctOfTotal
	}
	if pct < 99.9 || pct > 100.1 {
		t.Errorf("pct_of_total sum = %.4f, want ~100", pct)
	}
	codex, _ := findRow(card.Rows, models.ProviderCodex, "gpt-5")
	if codex.PctOfTotal < 49.0 || codex.PctOfTotal > 51.0 {
		t.Errorf("codex gpt-5 pct = %.2f, want ~50 (2.00 of 4.00)", codex.PctOfTotal)
	}

	// Coverage: 4 sessions carry v2 data; 5 total sessions in range.
	if card.CoveredSessionCount != 4 {
		t.Errorf("CoveredSessionCount = %d, want 4", card.CoveredSessionCount)
	}
	if card.TotalSessionCount != 5 {
		t.Errorf("TotalSessionCount = %d, want 5", card.TotalSessionCount)
	}
}

// TestGetTrends_CostByModel_TokenColumns pins that input/output and the SPLIT
// cache (read vs write) survive the aggregation per row.
func TestGetTrends_CostByModel_TokenColumns(t *testing.T) {
	store, userID, req := costByModelFixture(t)
	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	row, ok := findRow(resp.Cards.CostByModel.Rows, models.ProviderClaudeCode, "opus-4-5")
	if !ok {
		t.Fatal("missing (claude-code, opus-4-5) row")
	}
	if row.Input != 1000 || row.Output != 500 || row.CacheRead != 100 || row.CacheWrite != 50 {
		t.Errorf("token columns = in %d out %d cr %d cw %d, want 1000/500/100/50",
			row.Input, row.Output, row.CacheRead, row.CacheWrite)
	}
	if row.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1", row.SessionCount)
	}
}

// TestGetTrends_CostByModel_ModelFilter pins session-level ?model= narrowing:
// selecting a family restricts the whole response to sessions that used it
// (across providers), and the card shows ALL models in those sessions.
func TestGetTrends_CostByModel_ModelFilter(t *testing.T) {
	store, userID, req := costByModelFixture(t)
	req.Models = []string{"opus-4-5"} // matches claude session + opencode session

	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	// Whole response narrows to the 2 matching sessions (filter threads through
	// the shared CTE).
	if resp.SessionCount != 2 {
		t.Errorf("SessionCount under ?model=opus-4-5 = %d, want 2", resp.SessionCount)
	}
	rows := resp.Cards.CostByModel.Rows
	// codex gpt-5 (codex session) and the Unknown row (separate session) excluded.
	if _, ok := findRow(rows, models.ProviderCodex, "gpt-5"); ok {
		t.Error("codex gpt-5 should be excluded under ?model=opus-4-5")
	}
	if _, ok := findRow(rows, models.ProviderClaudeCode, ""); ok {
		t.Error("Unknown row should be excluded under ?model=opus-4-5")
	}
	// All models of the matching sessions remain (card shows full breakdown).
	for _, want := range []struct{ p, m string }{
		{models.ProviderClaudeCode, "opus-4-5"},
		{models.ProviderClaudeCode, "opus-4-5 · fast"},
		{models.ProviderOpencode, "opus-4-5"},
		{models.ProviderOpencode, "gpt-5"},
	} {
		if _, ok := findRow(rows, want.p, want.m); !ok {
			t.Errorf("expected (%s, %s) under ?model=opus-4-5", want.p, want.m)
		}
	}
}

// TestGetTrends_CostByModel_ProviderAndModel pins provider ∩ model = AND.
func TestGetTrends_CostByModel_ProviderAndModel(t *testing.T) {
	store, userID, req := costByModelFixture(t)
	req.Providers = []string{models.ProviderClaudeCode}
	req.Models = []string{"opus-4-5"}

	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	// Only the claude session satisfies both → opencode opus-4-5 excluded.
	if _, ok := findRow(resp.Cards.CostByModel.Rows, models.ProviderOpencode, "opus-4-5"); ok {
		t.Error("opencode opus-4-5 should be excluded when provider=claude-code AND model=opus-4-5")
	}
	if _, ok := findRow(resp.Cards.CostByModel.Rows, models.ProviderClaudeCode, "opus-4-5"); !ok {
		t.Error("claude-code opus-4-5 should remain")
	}
}

// TestGetTrends_CostByModel_FilterOptions pins the model dropdown source:
// distinct normalized families + fast variants across visible sessions,
// alphabetical, excluding the empty Unknown key.
func TestGetTrends_CostByModel_FilterOptions(t *testing.T) {
	store, userID, req := costByModelFixture(t)
	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	got := resp.FilterOptions.Models
	want := []string{"gpt-5", "opus-4-5", "opus-4-5 · fast"}
	if len(got) != len(want) {
		t.Fatalf("filter_options.models = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filter_options.models[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestGetTrends_CostByModel_ExcludesSynthetic pins that the "<synthetic>" model
// sentinel (Claude's no-real-model turns; 0 tokens, $0) is dropped from the
// breakdown rows AND the model dropdown, while real models on the same session
// still surface (vtrz).
func TestGetTrends_CostByModel_ExcludesSynthetic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "trends-synthetic@test.com", "Trends Synthetic User")
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cbm-synthetic", models.ProviderClaudeCode)
	testutil.MakeSessionListable(t, env, sessionID)
	testutil.SeedTokensV2Card(t, env, sessionID, analytics.TokensV2Data{
		TotalCostUSD: "1.00", TotalInput: 1000, TotalOutput: 500,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "1.00", Models: map[string]analytics.TokensV2Model{
				"opus-4-5":    {Input: 1000, Output: 500, CostUSD: "1.00"},
				"<synthetic>": {Input: 0, Output: 0, CostUSD: "0.00"},
			}},
		},
	})

	now := time.Now().UTC()
	req := analytics.TrendsRequest{
		StartTS:       now.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:         now.Add(24 * time.Hour).Unix(),
		Repos:         []string{},
		IncludeNoRepo: true,
	}
	resp, err := analytics.NewStore(env.DB.Conn()).GetTrends(context.Background(), user.ID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}

	if _, ok := findRow(resp.Cards.CostByModel.Rows, models.ProviderClaudeCode, "<synthetic>"); ok {
		t.Error("the <synthetic> sentinel must not appear as a cost-by-model row")
	}
	if _, ok := findRow(resp.Cards.CostByModel.Rows, models.ProviderClaudeCode, "opus-4-5"); !ok {
		t.Error("the real opus-4-5 model on the same session should still appear")
	}
	for _, m := range resp.FilterOptions.Models {
		if m == "<synthetic>" {
			t.Errorf("filter_options.models must not offer <synthetic>; got %v", resp.FilterOptions.Models)
		}
	}
}
