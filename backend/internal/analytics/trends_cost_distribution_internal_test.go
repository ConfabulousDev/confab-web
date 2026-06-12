package analytics

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Internal unit tests for the pure dynamic-bucketing + percentile helpers of the
// Cost Distribution card (y1w5). No DB — these lock the log10 decade contract and
// the percentile_cont interpolation semantics.

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

// TestBuildCostDistribution_FloorAndDecadesUpToMax: the bands are a "< $0.01"
// floor plus one decade per power of 10 up to the band containing the max value.
func TestBuildCostDistribution_FloorAndDecadesUpToMax(t *testing.T) {
	// max = 50 → top decade [$10, $100).
	values := decs(t, "0.005", "0.05", "0.50", "5.00", "50.00")
	card := buildCostDistribution(values, 5, 5)

	want := []string{"< $0.01", "$0.01 – $0.10", "$0.10 – $1", "$1 – $10", "$10 – $100"}
	if !eqStrings(labelsOf(card), want) {
		t.Fatalf("labels: got %v want %v", labelsOf(card), want)
	}
	// Each value lands in its own band.
	for label, total := range map[string]string{
		"< $0.01":       "0.005",
		"$0.01 – $0.10": "0.05",
		"$0.10 – $1":    "0.50",
		"$1 – $10":      "5.00",
		"$10 – $100":    "50.00",
	} {
		b := bucketByLabelI(t, card, label)
		if b.SessionCount != 1 {
			t.Errorf("band %q count: got %d want 1", label, b.SessionCount)
		}
		decEqual(t, b.TotalUSD, total)
	}
}

// TestBuildCostDistribution_GrowsToLargeMax: a multi-million-dollar value extends
// the bands all the way up, compact-labelled, with empty middle decades included.
func TestBuildCostDistribution_GrowsToLargeMax(t *testing.T) {
	values := decs(t, "0.20", "2000000.00") // $0.20 and $2M
	card := buildCostDistribution(values, 2, 2)

	want := []string{
		"< $0.01",
		"$0.01 – $0.10",
		"$0.10 – $1",
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
	if b := bucketByLabelI(t, card, "$0.10 – $1"); b.SessionCount != 1 {
		t.Errorf("$0.10–$1 band count: got %d want 1", b.SessionCount)
	}
	// An empty middle decade is still present with count 0.
	if b := bucketByLabelI(t, card, "$1K – $10K"); b.SessionCount != 0 {
		t.Errorf("empty middle band count: got %d want 0", b.SessionCount)
	}
}

// TestBuildCostDistribution_BoundariesAreHalfOpen pins the half-open [lo, hi)
// rule: a value exactly on a decade edge belongs to the HIGHER band.
func TestBuildCostDistribution_BoundariesAreHalfOpen(t *testing.T) {
	// max = 10 → top decade [$10, $100). Values sit exactly on edges.
	values := decs(t, "0.01", "0.10", "1.00", "10.00")
	card := buildCostDistribution(values, 4, 4)

	wantCounts := map[string]int{
		"< $0.01":       0, // 0.01 is NOT < 0.01
		"$0.01 – $0.10": 1, // 0.01
		"$0.10 – $1":    1, // 0.10
		"$1 – $10":      1, // 1.00
		"$10 – $100":    1, // 10.00
	}
	for label, want := range wantCounts {
		if b := bucketByLabelI(t, card, label); b.SessionCount != want {
			t.Errorf("band %q count: got %d want %d", label, b.SessionCount, want)
		}
	}
}

// TestBuildCostDistribution_AllSubCentOnlyFloor: when nothing reaches $0.01 the
// only band is the floor catch-all (incl. $0 and negatives).
func TestBuildCostDistribution_AllSubCentOnlyFloor(t *testing.T) {
	values := decs(t, "-1.00", "0.00", "0.005")
	card := buildCostDistribution(values, 3, 3)

	if want := []string{"< $0.01"}; !eqStrings(labelsOf(card), want) {
		t.Fatalf("labels: got %v want %v", labelsOf(card), want)
	}
	if card.Buckets[0].SessionCount != 3 {
		t.Errorf("floor count: got %d want 3", card.Buckets[0].SessionCount)
	}
}

// TestBuildCostDistribution_EmptyInput: no data points → just the floor band with
// count 0 and nil percentiles. (The card renders nothing when covered=0; this only
// guards the shape.)
func TestBuildCostDistribution_EmptyInput(t *testing.T) {
	card := buildCostDistribution(nil, 0, 0)
	if want := []string{"< $0.01"}; !eqStrings(labelsOf(card), want) {
		t.Fatalf("labels: got %v want %v", labelsOf(card), want)
	}
	if card.Buckets[0].SessionCount != 0 {
		t.Errorf("floor count: got %d want 0", card.Buckets[0].SessionCount)
	}
	decEqual(t, card.Buckets[0].TotalUSD, "0")
	if card.Percentiles != nil {
		t.Errorf("percentiles: got %+v want nil for empty input", card.Percentiles)
	}
}

// TestBuildCostDistribution_BandEdges checks the numeric lo/hi edges on a couple
// of bands (floor and a decade), incl. the top band being bounded (non-nil hi).
func TestBuildCostDistribution_BandEdges(t *testing.T) {
	card := buildCostDistribution(decs(t, "5.00"), 1, 1) // max 5 → top [$1,$10)
	floor := bucketByLabelI(t, card, "< $0.01")
	if floor.Lo != 0 || floor.Hi == nil || *floor.Hi != 0.01 {
		t.Errorf("floor edges: lo=%v hi=%v want lo=0 hi=0.01", floor.Lo, floor.Hi)
	}
	top := bucketByLabelI(t, card, "$1 – $10")
	if top.Lo != 1 || top.Hi == nil || *top.Hi != 10 {
		t.Errorf("top edges: lo=%v hi=%v want lo=1 hi=10", top.Lo, top.Hi)
	}
}

// TestCostDistributionPercentiles locks the percentile_cont (linear
// interpolation) semantics over a SORTED slice.
func TestCostDistributionPercentiles(t *testing.T) {
	t.Run("empty is nil", func(t *testing.T) {
		if got := costDistributionPercentiles(nil); got != nil {
			t.Errorf("got %+v want nil", got)
		}
	})

	t.Run("single value", func(t *testing.T) {
		got := costDistributionPercentiles(decs(t, "5.00"))
		if got == nil {
			t.Fatal("got nil want percentiles")
		}
		decEqual(t, got.P50, "5.00")
		decEqual(t, got.P90, "5.00")
		decEqual(t, got.P99, "5.00")
	})

	t.Run("interpolated", func(t *testing.T) {
		// sorted [0.10,0.20,0.30,0.40,0.50], n=5:
		// p50 rank=0.5*4=2.0 → 0.30
		// p90 rank=0.9*4=3.6 → 0.40 + 0.6*0.10 = 0.46
		// p99 rank=0.99*4=3.96 → 0.40 + 0.96*0.10 = 0.496
		got := costDistributionPercentiles(decs(t, "0.10", "0.20", "0.30", "0.40", "0.50"))
		if got == nil {
			t.Fatal("got nil want percentiles")
		}
		decEqual(t, got.P50, "0.30")
		decEqual(t, got.P90, "0.46")
		decEqual(t, got.P99, "0.496")
	})
}

// TestBuildCostDistribution_PercentilesOverUnsortedInput: buildCostDistribution
// sorts internally before computing percentiles.
func TestBuildCostDistribution_PercentilesOverUnsortedInput(t *testing.T) {
	values := decs(t, "0.50", "0.10", "0.40", "0.20", "0.30") // unsorted
	card := buildCostDistribution(values, 5, 5)
	if card.Percentiles == nil {
		t.Fatal("percentiles: got nil want p50/p90/p99")
	}
	decEqual(t, card.Percentiles.P50, "0.30")
	decEqual(t, card.Percentiles.P90, "0.46")
	decEqual(t, card.Percentiles.P99, "0.496")
}
