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
// that lands one session in each priced log-scale band, plus a multi-model session
// (whose opus-only portion re-buckets under ?model=), a sub-cent session and a $0
// session (both EXCLUDED from the card per 3tr4), and a session with NO v2 data
// (counted in total, not covered). Shared by the cost-distribution tests.
//
// Seeded per-session totals (and per-model breakdown):
//
//	cd-tiny  (claude): opus-4-5 = 0.005                 → total 0.005   (< $0.01: EXCLUDED)
//	cd-small (claude): opus-4-5 = 0.05                  → total 0.05    (band $0.01–$0.10)
//	cd-mid   (codex):  gpt-5    = 0.50                  → total 0.50    (band $0.10–$1)
//	cd-big   (claude): opus-4-5 = 0.50, gpt-5 = 4.50    → total 5.00    (band $1–$10)
//	cd-huge  (claude): opus-4-5 = 50.00                 → total 50.00   (band $10–$100)
//	cd-zero  (codex):  gpt-5    = 0.00                  → total 0.00    ($0: EXCLUDED)
//	cd-no-v2 (codex):  (no v2 row)                      → not covered
//
// Priced sessions (>= $0.01): cd-small, cd-mid, cd-big, cd-huge → covered = 4 of 7.
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
	zeroID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "cd-zero", models.ProviderCodex)
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
	// $0 session: has a v2 row but no priced cost — excluded from the card (counted
	// in total, not covered), proving the $0/unpriced case is dropped, not floored.
	testutil.SeedTokensV2Card(t, env, zeroID, analytics.TokensV2Data{
		TotalCostUSD: "0.00", TotalInput: 10, TotalOutput: 0,
		ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderCodex: {CostUSD: "0.00", Models: map[string]analytics.TokensV2Model{
				"gpt-5": {Input: 10, Output: 0, CostUSD: "0.00"},
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

// TestGetTrends_CostDistribution_PerSession: with no model filter, each priced
// session's TOTAL cost lands in exactly one band. The sub-cent (cd-tiny) and $0
// (cd-zero) sessions are excluded entirely — there is NO "< $0.01" floor band.
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
	// max priced total is $50 → bands run from the merged $0.01–$1 up to [$10, $100): three.
	if len(card.Buckets) != 3 {
		t.Fatalf("want 3 buckets ($0.01–$1 up to $10–$100, no floor), got %d", len(card.Buckets))
	}
	for _, b := range card.Buckets {
		if b.Label == "< $0.01" {
			t.Fatal("floor band '< $0.01' must not be present")
		}
	}

	cases := []struct {
		label string
		count int
		total string
	}{
		{"$0.01 – $1", 2, "0.55"}, // 0.05 + 0.50 fold into the merged sub-$1 band
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

// TestGetTrends_CostDistribution_Coverage: covered counts only sessions priced
// >= $0.01. The sub-cent (cd-tiny), $0 (cd-zero), and no-v2 sessions are all counted
// in total but excluded from covered.
func TestGetTrends_CostDistribution_Coverage(t *testing.T) {
	store, userID, req := costDistFixture(t)

	resp, err := store.GetTrends(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}
	card := resp.Cards.CostDistribution
	if card.CoveredSessionCount != 4 {
		t.Errorf("covered: got %d want 4 (only sessions priced >= $0.01)", card.CoveredSessionCount)
	}
	if card.TotalSessionCount != 7 {
		t.Errorf("total: got %d want 7 (all filtered sessions, incl. sub-cent/$0/no-v2)", card.TotalSessionCount)
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
	if card.Stats == nil {
		t.Fatal("percentiles: got nil want values")
	}
	// Priced subset only (sub-cent 0.005 and $0 excluded):
	// sorted [0.05, 0.50, 5.00, 50.00], n=4:
	// p50 rank=1.5 → 0.50 + 0.5*4.50 = 2.75
	// p90 rank=2.7 → 5 + 0.7*45 = 36.50
	// p99 rank=2.97 → 5 + 0.97*45 = 48.65
	// avg = 55.55 / 4 = 13.8875
	cdDecEq(t, card.Stats.P50, "2.75")
	cdDecEq(t, card.Stats.P90, "36.50")
	cdDecEq(t, card.Stats.P99, "48.65")
	cdDecEq(t, card.Stats.Avg, "13.8875")
}

// TestGetTrends_CostDistribution_ModelFilter: under ?model=opus-4-5 the unit
// becomes per-(session, model). Only opus-4-5's portion of each matched session
// counts — so cd-big (full-session 5.00, opus-only 0.50) re-buckets from $1–$10
// down to the merged $0.01–$1 band; cd-mid (gpt-5 only) drops out entirely; the synthetic model on
// cd-huge contributes no data point. cd-tiny's opus-only 0.005 is sub-cent and is
// EXCLUDED, so it neither buckets nor counts as covered.
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
	for _, b := range card.Buckets {
		if b.Label == "< $0.01" {
			t.Fatal("floor band '< $0.01' must not be present under ?model=")
		}
	}

	// Matched sessions: cd-tiny, cd-small, cd-big, cd-huge (all use opus-4-5).
	// Per-(session, opus-4-5) data points >= $0.01: 0.05, 0.50, 50.00 (cd-tiny's
	// 0.005 is excluded).
	wantCounts := map[string]int{
		"$0.01 – $1": 2, // cd-small 0.05 + cd-big 0.50 (opus only — NOT 5.00)
		"$1 – $10":   0, // empty: cd-big re-scoped DOWN out of this decade
		"$10 – $100": 1, // cd-huge  50.00
	}
	for label, want := range wantCounts {
		b := bucketByLabel(t, card, label)
		if b.SessionCount != want {
			t.Errorf("band %q count under ?model=opus-4-5: got %d want %d", label, b.SessionCount, want)
		}
	}
	// cd-big's opus-only portion proves per-(session,model) re-scoping: its 0.50 lands
	// in the merged sub-$1 band (total 0.05 + 0.50) and $1–$10 is empty — if the full
	// 5.00 session total had leaked through, $1–$10 would be non-empty instead.
	cdDecEq(t, bucketByLabel(t, card, "$0.01 – $1").TotalUSD, "0.55")

	// cd-tiny's only opus pair is sub-cent → not covered. Matched sessions still
	// number 4 (the CTE-level model filter is independent of the price threshold).
	if card.CoveredSessionCount != 3 {
		t.Errorf("covered under filter: got %d want 3 (cd-tiny's 0.005 excluded)", card.CoveredSessionCount)
	}
	if card.TotalSessionCount != 4 {
		t.Errorf("total under filter: got %d want 4 (filtered to matched sessions)", card.TotalSessionCount)
	}
}
