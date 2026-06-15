package analytics

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Internal unit tests for the pure dynamic-bucketing + percentile helpers of the
// Cost Distribution card (y1w5). No DB — these lock the log10 decade contract, the
// percentile_cont interpolation semantics, and the 3tr4 rule that sub-cent (< $0.01)
// data points are EXCLUDED entirely (no floor band).

func decs(t *testing.T, ss ...string) []decimal.Decimal {
	t.Helper()
	out := make([]decimal.Decimal, len(ss))
	for i, s := range ss {
		out[i] = decimal.RequireFromString(s)
	}
	return out
}

func decEqual(t *testing.T, got, want string) {
	t.Helper()
	g, err := decimal.NewFromString(got)
	if err != nil {
		t.Fatalf("bad decimal %q: %v", got, err)
	}
	if !g.Equal(decimal.RequireFromString(want)) {
		t.Fatalf("decimal mismatch: got %q want %q", got, want)
	}
}

func labelsOf(card *TrendsCostDistributionCard) []string {
	out := make([]string, len(card.Buckets))
	for i, b := range card.Buckets {
		out[i] = b.Label
	}
	return out
}

func bucketByLabelI(t *testing.T, card *TrendsCostDistributionCard, label string) CostDistributionBucket {
	t.Helper()
	for _, b := range card.Buckets {
		if b.Label == label {
			return b
		}
	}
	t.Fatalf("bucket %q not found among %v", label, labelsOf(card))
	return CostDistributionBucket{}
}

func eqStrings(a, b []string) bool {
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

func hasLabel(card *TrendsCostDistributionCard, label string) bool {
	for _, l := range labelsOf(card) {
		if l == label {
			return true
		}
	}
	return false
}

// TestBuildCostDistribution_DecadesUpToMax: the first band merges the two sub-$1
// decades into a single $0.01–$1 band (bj37); decades from $1 up are one per power
// of 10 up to the band containing the max value. There is NO "< $0.01" floor band,
// and the sub-cent value ($0.005) is excluded entirely.
func TestBuildCostDistribution_DecadesUpToMax(t *testing.T) {
	// max = 50 → top decade [$10, $100). The 0.005 value is dropped.
	values := decs(t, "0.005", "0.05", "0.50", "5.00", "50.00")
	card := buildCostDistribution(values, 4, 4)

	want := []string{"$0.01 – $1", "$1 – $10", "$10 – $100"}
	if !eqStrings(labelsOf(card), want) {
		t.Fatalf("labels: got %v want %v", labelsOf(card), want)
	}
	if hasLabel(card, "< $0.01") {
		t.Fatal("floor band '< $0.01' must not be present")
	}
	// Both sub-$1 values (0.05, 0.50) fold into the merged first band; $5 and $50
	// land in their own decades. The sub-cent value contributes nowhere.
	for _, c := range []struct {
		label string
		count int
		total string
	}{
		{"$0.01 – $1", 2, "0.55"},
		{"$1 – $10", 1, "5.00"},
		{"$10 – $100", 1, "50.00"},
	} {
		b := bucketByLabelI(t, card, c.label)
		if b.SessionCount != c.count {
			t.Errorf("band %q count: got %d want %d", c.label, b.SessionCount, c.count)
		}
		decEqual(t, b.TotalUSD, c.total)
	}
}

// TestBuildCostDistribution_GrowsToLargeMax: a multi-million-dollar value extends the
// bands all the way up, compact-labelled, with empty middle decades included — still
// no floor band.
func TestBuildCostDistribution_GrowsToLargeMax(t *testing.T) {
	values := decs(t, "0.20", "2000000.00") // $0.20 and $2M
	card := buildCostDistribution(values, 2, 2)

	want := []string{
		"$0.01 – $1",
		"$1 – $10",
		"$10 – $100",
		"$100 – $1K",
		"$1K – $10K",
		"$10K – $100K",
		"$100K – $1M",
		"$1M – $10M",
	}
	if !eqStrings(labelsOf(card), want) {
		t.Fatalf("labels: got %v want %v", labelsOf(card), want)
	}
	if b := bucketByLabelI(t, card, "$1M – $10M"); b.SessionCount != 1 {
		t.Errorf("top band count: got %d want 1", b.SessionCount)
	}
	if b := bucketByLabelI(t, card, "$0.01 – $1"); b.SessionCount != 1 {
		t.Errorf("$0.01–$1 band count: got %d want 1", b.SessionCount)
	}
	// An empty middle decade is still present with count 0.
	if b := bucketByLabelI(t, card, "$1K – $10K"); b.SessionCount != 0 {
		t.Errorf("empty middle band count: got %d want 0", b.SessionCount)
	}
}

// TestBuildCostDistribution_BoundariesAreHalfOpen pins the half-open [lo, hi) rule: a
// value exactly on a band edge belongs to the HIGHER band, $0.01 is included (it is
// NOT < $0.01), and a sub-cent value just under the threshold is excluded. The merged
// first band ($0.01–$1) has a ×100 first step, so 1.00 sits exactly on its upper edge
// and falls into $1–$10 (bj37).
func TestBuildCostDistribution_BoundariesAreHalfOpen(t *testing.T) {
	// max = 10 → top decade [$10, $100). Values sit exactly on edges; 0.009 is dropped.
	values := decs(t, "0.009", "0.01", "0.10", "1.00", "10.00")
	card := buildCostDistribution(values, 4, 4)

	if hasLabel(card, "< $0.01") {
		t.Fatal("floor band '< $0.01' must not be present")
	}
	wantCounts := map[string]int{
		"$0.01 – $1": 2, // 0.01 and 0.10 (0.009 excluded); merged sub-$1 band
		"$1 – $10":   1, // 1.00 (on-edge → higher band)
		"$10 – $100": 1, // 10.00 (on-edge → higher band)
	}
	for label, want := range wantCounts {
		if b := bucketByLabelI(t, card, label); b.SessionCount != want {
			t.Errorf("band %q count: got %d want %d", label, b.SessionCount, want)
		}
	}
}

// TestBuildCostDistribution_ExcludesSubCent: $0, negative, and sub-cent values are
// dropped from the buckets, totals, and percentiles entirely — only >= $0.01 counts.
func TestBuildCostDistribution_ExcludesSubCent(t *testing.T) {
	values := decs(t, "-1.00", "0.00", "0.005", "0.009", "0.05", "50.00")
	card := buildCostDistribution(values, 2, 2)

	want := []string{"$0.01 – $1", "$1 – $10", "$10 – $100"}
	if !eqStrings(labelsOf(card), want) {
		t.Fatalf("labels: got %v want %v", labelsOf(card), want)
	}
	// Only the two priced values survive.
	if b := bucketByLabelI(t, card, "$0.01 – $1"); b.SessionCount != 1 {
		t.Errorf("$0.01–$1 count: got %d want 1", b.SessionCount)
	}
	if b := bucketByLabelI(t, card, "$10 – $100"); b.SessionCount != 1 {
		t.Errorf("$10–$100 count: got %d want 1", b.SessionCount)
	}
	// Percentiles run over [0.05, 50.00] only — the excluded values don't drag them down.
	if card.Stats == nil {
		t.Fatal("stats: got nil want values over the priced subset")
	}
	decEqual(t, card.Stats.P50, "25.025") // mean-rank of two values: 0.05 + 0.5*(50-0.05)
	decEqual(t, card.Stats.Avg, "25.025") // (0.05 + 50.00) / 2
}

// TestBuildCostDistribution_AllSubCent: when nothing reaches $0.01 there are NO bands
// at all and percentiles are nil — the whole population was excluded.
func TestBuildCostDistribution_AllSubCent(t *testing.T) {
	values := decs(t, "-1.00", "0.00", "0.005")
	card := buildCostDistribution(values, 0, 3)

	if len(card.Buckets) != 0 {
		t.Fatalf("buckets: got %v want none (all sub-cent)", labelsOf(card))
	}
	if card.Stats != nil {
		t.Errorf("stats: got %+v want nil (all sub-cent)", card.Stats)
	}
}

// TestBuildCostDistribution_EmptyInput: no data points → no bands and nil percentiles.
// (The card renders nothing when covered=0; this only guards the shape.)
func TestBuildCostDistribution_EmptyInput(t *testing.T) {
	card := buildCostDistribution(nil, 0, 0)
	if len(card.Buckets) != 0 {
		t.Fatalf("buckets: got %v want none for empty input", labelsOf(card))
	}
	if card.Stats != nil {
		t.Errorf("stats: got %+v want nil for empty input", card.Stats)
	}
}

// TestBuildCostDistribution_BandEdges checks the numeric lo/hi edges: the FIRST band is
// the merged $0.01–$1 band (lo 0.01, hi 1 — not a 0-floor), and the top band is bounded.
func TestBuildCostDistribution_BandEdges(t *testing.T) {
	card := buildCostDistribution(decs(t, "5.00"), 1, 1) // max 5 → top [$1,$10)
	first := bucketByLabelI(t, card, "$0.01 – $1")
	if first.Lo != 0.01 || first.Hi == nil || *first.Hi != 1 {
		t.Errorf("first band edges: lo=%v hi=%v want lo=0.01 hi=1", first.Lo, first.Hi)
	}
	top := bucketByLabelI(t, card, "$1 – $10")
	if top.Lo != 1 || top.Hi == nil || *top.Hi != 10 {
		t.Errorf("top edges: lo=%v hi=%v want lo=1 hi=10", top.Lo, top.Hi)
	}
}

// TestDecadeEdges_MergedFirstBand pins the edge sequence: the first step is ×100
// (0.01 → 1) so the two sub-$1 decades collapse into one $0.01–$1 band (bj37);
// every step after is ×10. Below $0.01 there are no bands.
func TestDecadeEdges_MergedFirstBand(t *testing.T) {
	edgeStrs := func(max string) []string {
		edges := decadeEdges(decimal.RequireFromString(max))
		out := make([]string, len(edges))
		for i, e := range edges {
			out[i] = e.String()
		}
		return out
	}
	cases := []struct {
		name string
		max  string
		want []string
	}{
		{"below threshold → no bands", "0.005", []string{"0.01"}},
		{"sub-$1 max → single merged step", "0.5", []string{"0.01", "1"}},
		{"on $1 → extends to $10 (half-open top)", "1", []string{"0.01", "1", "10"}},
		{"single-digit max → 0.01,1,10", "5", []string{"0.01", "1", "10"}},
		{"two-digit max → adds $100", "50", []string{"0.01", "1", "10", "100"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := edgeStrs(c.max); !eqStrings(got, c.want) {
				t.Fatalf("decadeEdges(%s): got %v want %v", c.max, got, c.want)
			}
		})
	}
}

// TestCostDistributionBucketIndex_MergedFirstBand checks half-open [lo,hi) assignment
// against the non-uniform edge list [0.01, 1, 10, 100] (bands 0:[0.01,1) 1:[1,10)
// 2:[10,100)): on-edge values go to the HIGHER band, and everything in [0.01, 1)
// folds into the merged first band.
func TestCostDistributionBucketIndex_MergedFirstBand(t *testing.T) {
	edges := decadeEdges(decimal.RequireFromString("50")) // [0.01, 1, 10, 100]
	cases := []struct {
		v   string
		idx int
	}{
		{"0.01", 0},  // lower edge of the merged band
		{"0.99", 0},  // still in $0.01–$1
		{"1.00", 1},  // on-edge → $1–$10
		{"9.99", 1},  // top of $1–$10
		{"10.00", 2}, // on-edge → $10–$100
		{"50.00", 2}, // within [10,100)
	}
	for _, c := range cases {
		got := costDistributionBucketIndex(decimal.RequireFromString(c.v), edges)
		if got != c.idx {
			t.Errorf("bucketIndex(%s): got %d want %d", c.v, got, c.idx)
		}
	}
}

// TestCostDistributionPercentiles locks the percentile_cont (linear interpolation)
// semantics over a SORTED slice. (costDistributionStats is a pure calc over its input;
// sub-cent exclusion happens upstream in buildCostDistribution.)
func TestCostDistributionPercentiles(t *testing.T) {
	t.Run("empty is nil", func(t *testing.T) {
		if got := costDistributionStats(nil); got != nil {
			t.Errorf("got %+v want nil", got)
		}
	})

	t.Run("single value", func(t *testing.T) {
		got := costDistributionStats(decs(t, "5.00"))
		if got == nil {
			t.Fatal("got nil want percentiles")
		}
		decEqual(t, got.P50, "5.00")
		decEqual(t, got.P90, "5.00")
		decEqual(t, got.P99, "5.00")
		decEqual(t, got.Avg, "5.00") // mean of one value is itself
	})

	t.Run("interpolated", func(t *testing.T) {
		// sorted [0.10,0.20,0.30,0.40,0.50], n=5:
		// p50 rank=0.5*4=2.0 → 0.30
		// p90 rank=0.9*4=3.6 → 0.40 + 0.6*0.10 = 0.46
		// p99 rank=0.99*4=3.96 → 0.40 + 0.96*0.10 = 0.496
		// avg = 1.50 / 5 = 0.30
		got := costDistributionStats(decs(t, "0.10", "0.20", "0.30", "0.40", "0.50"))
		if got == nil {
			t.Fatal("got nil want percentiles")
		}
		decEqual(t, got.P50, "0.30")
		decEqual(t, got.P90, "0.46")
		decEqual(t, got.P99, "0.496")
		decEqual(t, got.Avg, "0.30")
	})

	t.Run("avg is the mean, distinct from the median", func(t *testing.T) {
		// sorted [0.10,0.10,0.10,0.10,4.60], n=5: median 0.10 but mean
		// = 5.00 / 5 = 1.00 — a right-skewed set where avg ≠ p50.
		got := costDistributionStats(decs(t, "0.10", "0.10", "0.10", "0.10", "4.60"))
		if got == nil {
			t.Fatal("got nil want percentiles")
		}
		decEqual(t, got.P50, "0.10")
		decEqual(t, got.Avg, "1.00")
	})
}

// TestBuildCostDistribution_PercentilesExcludeSubCent: buildCostDistribution sorts and
// drops sub-cent values before computing percentiles, so a leading $0.005 doesn't shift
// the result versus the priced-only set.
func TestBuildCostDistribution_PercentilesExcludeSubCent(t *testing.T) {
	values := decs(t, "0.005", "0.50", "0.10", "0.40", "0.20", "0.30") // unsorted + one sub-cent
	card := buildCostDistribution(values, 5, 5)
	if card.Stats == nil {
		t.Fatal("percentiles: got nil want p50/p90/p99")
	}
	decEqual(t, card.Stats.P50, "0.30")
	decEqual(t, card.Stats.P90, "0.46")
	decEqual(t, card.Stats.P99, "0.496")
	decEqual(t, card.Stats.Avg, "0.30") // 1.50 / 5 priced values
}
