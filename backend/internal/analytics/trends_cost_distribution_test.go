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

// costDistFixture seeds a visible session set with a spread of per-session costs
// that lands one session in each of the five log-scale bands, plus a multi-model
// session (whose opus-only portion re-buckets under ?model=) and a session with NO
// v2 data (counted in total, not covered). Shared by the cost-distribution tests.
//
// Seeded per-session totals (and per-model breakdown):
//
//	cd-tiny  (claude): opus-4-5 = 0.005                 → total 0.005   (band < $0.01)
//	cd-small (claude): opus-4-5 = 0.05                  → total 0.05    (band $0.01–$0.10)
//	cd-mid   (codex):  gpt-5    = 0.50                  → total 0.50    (band $0.10–$1)
//	cd-big   (claude): opus-4-5 = 0.50, gpt-5 = 4.50    → total 5.00    (band $1–$10)
//	cd-huge  (claude): opus-4-5 = 50.00                 → total 50.00   (band > $10)
//	cd-no-v2 (codex):  (no v2 row)                      → not covered
func costDistFixture(t *testing.T) (*analytics.Store, int64, analytics.TrendsRequest) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "trends-cd@test.com", "Trends CD User")

	tinyID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cd-tiny", models.ProviderClaudeCode)
	smallID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cd-small", models.ProviderClaudeCode)
	midID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cd-mid", models.ProviderCodex)
	bigID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cd-big", models.ProviderClaudeCode)
	hugeID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cd-huge", models.ProviderClaudeCode)
	_ = testutil.CreateTestSessionWithProvider(t, env, user.ID, "cd-no-v2", models.ProviderCodex)

	testutil.SeedTokensV2Card(t, env, tinyID, analytics.TokensV2Data{
		TotalCostUSD: "0.005", TotalInput: 10, TotalOutput: 5,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0.005", Models: map[string]analytics.TokensV2Model{
				"opus-4-5": {Input: 10, Output: 5, CostUSD: "0.005"},
			}},
		},
	})
	testutil.SeedTokensV2Card(t, env, smallID, analytics.TokensV2Data{
		TotalCostUSD: "0.05", TotalInput: 100, TotalOutput: 50,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0.05", Models: map[string]analytics.TokensV2Model{
				"opus-4-5": {Input: 100, Output: 50, CostUSD: "0.05"},
			}},
		},
	})
	testutil.SeedTokensV2Card(t, env, midID, analytics.TokensV2Data{
		TotalCostUSD: "0.50", TotalInput: 500, TotalOutput: 200,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderCodex: {CostUSD: "0.50", Models: map[string]analytics.TokensV2Model{
				"gpt-5": {Input: 500, Output: 200, CostUSD: "0.50"},
			}},
		},
	})
	testutil.SeedTokensV2Card(t, env, bigID, analytics.TokensV2Data{
		TotalCostUSD: "5.00", TotalInput: 5000, TotalOutput: 2000,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "5.00", Models: map[string]analytics.TokensV2Model{
				"opus-4-5": {Input: 500, Output: 200, CostUSD: "0.50"},
				"gpt-5":    {Input: 4500, Output: 1800, CostUSD: "4.50"},
			}},
		},
	})
	testutil.SeedTokensV2Card(t, env, hugeID, analytics.TokensV2Data{
		TotalCostUSD: "50.00", TotalInput: 50000, TotalOutput: 20000,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "50.00", Models: map[string]analytics.TokensV2Model{
				"opus-4-5":        {Input: 50000, Output: 20000, CostUSD: "50.00"},
				syntheticModelLit: {Input: 0, Output: 0, CostUSD: "0.00"},
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

// syntheticModelLit mirrors analytics.syntheticModelKey ("<synthetic>"); the
// constant is unexported, so the external test re-declares the literal.
const syntheticModelLit = "<synthetic>"

func bucketByLabel(t *testing.T, card *analytics.TrendsCostDistributionCard, label string) analytics.CostDistributionBucket {
	t.Helper()
	for _, b := range card.Buckets {
		if b.Label == label {
			return b
		}
	}
	t.Fatalf("bucket %q not found among %d buckets", label, len(card.Buckets))
	return analytics.CostDistributionBucket{}
}

func cdDecEq(t *testing.T, got, want string) {
	t.Helper()
	g, err := decimal.NewFromString(got)
	if err != nil {
		t.Fatalf("bad decimal %q: %v", got, err)
	}
	if !g.Equal(decimal.RequireFromString(want)) {
		t.Fatalf("decimal mismatch: got %q want %q", got, want)
	}
}

// TestGetTrends_CostDistribution_PerSession: with no model filter, each seeded
// session's TOTAL cost lands in exactly one band; all five bands are present.
func TestGetTrends_CostDistribution_PerSession(t *testing.T) {
	store, userID, req := costDistFixture(t)

	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	card := resp.Cards.CostDistribution
	if card == nil {
		t.Fatal("cost_distribution card is nil")
	}
	if card.TimedOut {
		t.Fatal("unexpected timed_out")
	}
	// max seeded total is $50 → bands run from the floor up to [$10, $100): five.
	if len(card.Buckets) != 5 {
		t.Fatalf("want 5 buckets (floor + 4 decades up to $10–$100), got %d", len(card.Buckets))
	}

	cases := []struct {
		label string
		count int
		total string
	}{
		{"< $0.01", 1, "0.005"},
		{"$0.01 – $0.10", 1, "0.05"},
		{"$0.10 – $1", 1, "0.50"},
		{"$1 – $10", 1, "5.00"},
		{"$10 – $100", 1, "50.00"},
	}
	for _, c := range cases {
		b := bucketByLabel(t, card, c.label)
		if b.SessionCount != c.count {
			t.Errorf("band %q count: got %d want %d", c.label, b.SessionCount, c.count)
		}
		cdDecEq(t, b.TotalUSD, c.total)
	}
}

// TestGetTrends_CostDistribution_Coverage: the session with no v2 row is counted
// in total but not in covered.
func TestGetTrends_CostDistribution_Coverage(t *testing.T) {
	store, userID, req := costDistFixture(t)

	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	card := resp.Cards.CostDistribution
	if card.CoveredSessionCount != 5 {
		t.Errorf("covered: got %d want 5", card.CoveredSessionCount)
	}
	if card.TotalSessionCount != 6 {
		t.Errorf("total: got %d want 6 (includes the no-v2 session)", card.TotalSessionCount)
	}
}

// TestGetTrends_CostDistribution_Percentiles: percentiles are computed over the
// per-session cost values with percentile_cont semantics.
func TestGetTrends_CostDistribution_Percentiles(t *testing.T) {
	store, userID, req := costDistFixture(t)

	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	card := resp.Cards.CostDistribution
	if card.Percentiles == nil {
		t.Fatal("percentiles: got nil want values")
	}
	// sorted [0.005, 0.05, 0.50, 5.00, 50.00], n=5:
	// p50 rank=2.0 → 0.50; p90 rank=3.6 → 5 + 0.6*45 = 32; p99 rank=3.96 → 5 + 0.96*45 = 48.2
	cdDecEq(t, card.Percentiles.P50, "0.50")
	cdDecEq(t, card.Percentiles.P90, "32.00")
	cdDecEq(t, card.Percentiles.P99, "48.20")
}

// TestGetTrends_CostDistribution_ModelFilter: under ?model=opus-4-5 the unit
// becomes per-(session, model). Only opus-4-5's portion of each matched session
// counts — so cd-big (full-session 5.00, opus-only 0.50) re-buckets from $1–$10
// down to $0.10–$1; cd-mid (gpt-5 only) drops out entirely; the synthetic model on
// cd-huge contributes no data point.
func TestGetTrends_CostDistribution_ModelFilter(t *testing.T) {
	store, userID, req := costDistFixture(t)
	req.Models = []string{"opus-4-5"}

	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	card := resp.Cards.CostDistribution
	if card == nil {
		t.Fatal("cost_distribution card is nil")
	}

	// Matched sessions: cd-tiny, cd-small, cd-big, cd-huge (all use opus-4-5).
	// Per-(session, opus-4-5) data points: 0.005, 0.05, 0.50, 50.00.
	wantCounts := map[string]int{
		"< $0.01":       1, // cd-tiny  0.005
		"$0.01 – $0.10": 1, // cd-small 0.05
		"$0.10 – $1":    1, // cd-big   0.50 (opus only — NOT 5.00)
		"$1 – $10":      0,
		"$10 – $100":    1, // cd-huge  50.00
	}
	for label, want := range wantCounts {
		b := bucketByLabel(t, card, label)
		if b.SessionCount != want {
			t.Errorf("band %q count under ?model=opus-4-5: got %d want %d", label, b.SessionCount, want)
		}
	}
	// cd-big's opus-only portion proves per-(session,model) re-scoping.
	cdDecEq(t, bucketByLabel(t, card, "$0.10 – $1").TotalUSD, "0.50")

	if card.CoveredSessionCount != 4 {
		t.Errorf("covered under filter: got %d want 4", card.CoveredSessionCount)
	}
	if card.TotalSessionCount != 4 {
		t.Errorf("total under filter: got %d want 4 (filtered to matched sessions)", card.TotalSessionCount)
	}
}
