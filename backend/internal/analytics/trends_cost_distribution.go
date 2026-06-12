package analytics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/shopspring/decimal"
)

// trends_cost_distribution.go: the Cost Distribution card (y1w5) — a fixed
// log-scale histogram of per-session cost plus p50/p90/p99 percentiles. Mirrors
// trends_cost_by_model.go's structure (the shared costByModelTimeout budget,
// graceful degradation, coverage caption) and reuses 2hh1's tokens_v2 plumbing.
//
// Two fetch paths share one bucketing/percentile pass:
//   - No model filter: one per-session scalar (data->>'total_cost_usd') — no
//     LATERAL tree expansion, so it's cheap (the perf win the ticket flagged).
//   - ?model= active: expand the v2 tree (v2ModelScanSQL) and fold per
//     (session, normalized-model), keeping only the selected models — so each
//     (session, model) pair is one data point and only the selected model's cost
//     in a session counts.
// Percentiles run in Go (percentile_cont semantics), which sidesteps the fact
// that OpenCode model keys can't be family-grouped in SQL — no migration needed.

// perSessionCostScanSQL fetches one per-session total-cost scalar for every
// filtered session that carries tokens_v2 data. No tree expansion.
const perSessionCostScanSQL = `
	SELECT fs.id, COALESCE(v.data->>'total_cost_usd', '0')
	FROM filtered_sessions fs
	JOIN session_card_tokens_v2 v ON v.session_id = fs.id`

// Buckets are dynamic log10 decades: a fixed "< $0.01" catch-all (for $0 / tiny /
// negative values that can't sit on a log axis) followed by one bucket per power
// of 10, from $0.01 up to the decade that contains the most expensive data point
// (y1w5). The low end is fixed so the axis is comparable across ranges; only the
// top grows with the data, including any empty middle decades.

// costDistributionFloorEdge is the upper bound of the "< $0.01" catch-all band.
var costDistributionFloorEdge = decimal.RequireFromString("0.01")

const (
	costDistributionFloorLabel = "< $0.01"
	// maxCostDecades caps the number of decades (~$10^13) — a defensive guard
	// against an absurd/NaN maximum producing an unbounded bucket list.
	maxCostDecades = 16
)

// percentile points, as decimals so rank/frac arithmetic stays exact.
var (
	pctP50 = decimal.RequireFromString("0.50")
	pctP90 = decimal.RequireFromString("0.90")
	pctP99 = decimal.RequireFromString("0.99")
)

// costDistributionBand is one resolved bucket: label + dollar edges ([lo, hi)).
type costDistributionBand struct {
	label string
	lo    float64
	hi    float64
}

// decadeEdges returns the power-of-10 boundaries [0.01, 0.10, 1, …, B] where B is
// the smallest power of 10 strictly greater than max — so the last decade [.., B)
// contains max. Returns just [0.01] when max < 0.01 (only the floor band is
// needed). Capped at maxCostDecades decades.
func decadeEdges(max decimal.Decimal) []decimal.Decimal {
	edges := []decimal.Decimal{costDistributionFloorEdge}
	ten := decimal.NewFromInt(10)
	for range maxCostDecades {
		last := edges[len(edges)-1]
		if last.GreaterThan(max) {
			break
		}
		edges = append(edges, last.Mul(ten))
	}
	return edges
}

// costDistributionBands resolves the bucket list (floor + one per decade) for the
// given decade edges. Always includes the "< $0.01" floor band first.
func costDistributionBands(edges []decimal.Decimal) []costDistributionBand {
	floorHi, _ := costDistributionFloorEdge.Float64()
	bands := []costDistributionBand{{label: costDistributionFloorLabel, lo: 0, hi: floorHi}}
	for i := 0; i+1 < len(edges); i++ {
		lo, _ := edges[i].Float64()
		hi, _ := edges[i+1].Float64()
		bands = append(bands, costDistributionBand{
			label: formatDecadeEdge(lo) + " – " + formatDecadeEdge(hi),
			lo:    lo,
			hi:    hi,
		})
	}
	return bands
}

// formatDecadeEdge renders a power-of-10 dollar edge compactly: $0.01, $0.10, $1,
// $100, $1K, $10K, $1M, $1B. Edges are exact powers of 10, so integer division is
// clean.
func formatDecadeEdge(f float64) string {
	switch {
	case f < 1:
		return fmt.Sprintf("$%.2f", f)
	case f < 1_000:
		return fmt.Sprintf("$%d", int64(f))
	case f < 1_000_000:
		return fmt.Sprintf("$%dK", int64(f)/1_000)
	case f < 1_000_000_000:
		return fmt.Sprintf("$%dM", int64(f)/1_000_000)
	default:
		return fmt.Sprintf("$%dB", int64(f)/1_000_000_000)
	}
}

// costDistributionBucketIndex returns the band index for a cost value, given the
// decade edges. Half-open [lo, hi): a value exactly on an edge belongs to the
// HIGHER band; $0, tiny, and negative/unpriced values all fall in the floor band
// (index 0).
func costDistributionBucketIndex(v decimal.Decimal, edges []decimal.Decimal) int {
	if v.LessThan(costDistributionFloorEdge) {
		return 0
	}
	for i := 0; i+1 < len(edges); i++ {
		if v.LessThan(edges[i+1]) {
			return i + 1 // +1 to skip the floor band
		}
	}
	return len(edges) - 1 // top decade (edges top > max ≥ v, so normally unreached)
}

// aggregateCostDistribution builds the per-session cost histogram over the
// filtered, visible sessions that carry tokens_v2 data. On a query timeout it
// degrades to an empty card with TimedOut=true (the whole Trends response still
// succeeds) and logs a PII-safe WARN with the request shape.
func (s *Store) aggregateCostDistribution(ctx context.Context, tq trendsQuery, userID int64, req TrendsRequest) (*TrendsCostDistributionCard, error) {
	cdCtx, cancel := context.WithTimeout(ctx, costByModelTimeout)
	defer cancel()
	started := time.Now()

	// Denominator for the coverage caption: all filtered sessions in range.
	var total int
	if err := s.db.QueryRowContext(cdCtx, tq.cteSQL+"\nSELECT COUNT(*) FROM filtered_sessions", tq.args...).Scan(&total); err != nil {
		if isTimeoutErr(err) {
			return degradedCostDistributionCard(ctx, userID, req, started, err), nil
		}
		return nil, fmt.Errorf("cost-distribution total count: %w", err)
	}

	var (
		values  []decimal.Decimal
		covered map[string]struct{}
		err     error
	)
	if len(req.Models) > 0 {
		values, covered, err = s.costDistributionPerModelValues(cdCtx, tq, req)
	} else {
		values, covered, err = s.costDistributionPerSessionValues(cdCtx, tq)
	}
	if err != nil {
		if isTimeoutErr(err) {
			return degradedCostDistributionCard(ctx, userID, req, started, err), nil
		}
		return nil, err
	}

	return buildCostDistribution(values, len(covered), total), nil
}

// costDistributionPerSessionValues fetches one total-cost scalar per filtered
// session with v2 data (the no-filter path). covered = the sessions that carry a
// data point.
func (s *Store) costDistributionPerSessionValues(ctx context.Context, tq trendsQuery) ([]decimal.Decimal, map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, tq.cteSQL+perSessionCostScanSQL, tq.args...)
	if err != nil {
		return nil, nil, fmt.Errorf("cost-distribution per-session query: %w", err)
	}
	defer rows.Close()

	var values []decimal.Decimal
	covered := map[string]struct{}{}
	for rows.Next() {
		var sessionID, costStr string
		if err := rows.Scan(&sessionID, &costStr); err != nil {
			return nil, nil, fmt.Errorf("cost-distribution per-session scan: %w", err)
		}
		cost, err := decimal.NewFromString(costStr)
		if err != nil {
			cost = decimal.Zero
		}
		values = append(values, cost)
		covered[sessionID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("cost-distribution per-session rows: %w", err)
	}
	return values, covered, nil
}

// costDistributionPerModelValues expands the v2 tree and folds cost per
// (session, normalized-model), keeping only the rows whose normalized family is
// in req.Models (the ?model= path). Each surviving (session, model) pair is one
// data point. Synthetic turns are excluded. covered = the distinct sessions that
// contributed a data point.
func (s *Store) costDistributionPerModelValues(ctx context.Context, tq trendsQuery, req TrendsRequest) ([]decimal.Decimal, map[string]struct{}, error) {
	selected := make(map[string]struct{}, len(req.Models))
	for _, m := range req.Models {
		selected[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}

	rows, err := s.db.QueryContext(ctx, tq.cteSQL+v2ModelScanSQL, tq.args...)
	if err != nil {
		return nil, nil, fmt.Errorf("cost-distribution per-model query: %w", err)
	}
	defer rows.Close()

	type pairKey struct{ session, model string }
	pairCost := map[pairKey]decimal.Decimal{}
	covered := map[string]struct{}{}
	for rows.Next() {
		var sessionID, sessionType, rawModel, costStr string
		var input, output, cacheRead, cacheWrite int64
		if err := rows.Scan(&sessionID, &sessionType, &rawModel, &costStr, &input, &output, &cacheRead, &cacheWrite); err != nil {
			return nil, nil, fmt.Errorf("cost-distribution per-model scan: %w", err)
		}
		norm := normalizeV2ModelKey(models.NormalizeProvider(sessionType), rawModel)
		if norm == syntheticModelKey {
			continue
		}
		if _, ok := selected[strings.ToLower(norm)]; !ok {
			continue
		}
		cost, err := decimal.NewFromString(costStr)
		if err != nil {
			cost = decimal.Zero
		}
		k := pairKey{session: sessionID, model: norm}
		pairCost[k] = pairCost[k].Add(cost)
		covered[sessionID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("cost-distribution per-model rows: %w", err)
	}

	values := make([]decimal.Decimal, 0, len(pairCost))
	for _, c := range pairCost {
		values = append(values, c)
	}
	return values, covered, nil
}

// buildCostDistribution folds per-data-point cost values into dynamic log10 bands
// (a "< $0.01" floor plus one decade per power of 10 up to the band containing the
// most expensive value) and computes percentiles. covered/total pass straight
// through to the card's coverage caption.
func buildCostDistribution(values []decimal.Decimal, covered, total int) *TrendsCostDistributionCard {
	max := decimal.Zero
	for _, v := range values {
		if v.GreaterThan(max) {
			max = v
		}
	}
	edges := decadeEdges(max)
	bands := costDistributionBands(edges)

	// counts and totals are parallel to bands; make() zero-values both (the
	// decimal zero value is a valid zero for Add/String).
	counts := make([]int, len(bands))
	totals := make([]decimal.Decimal, len(bands))
	for _, v := range values {
		idx := costDistributionBucketIndex(v, edges)
		counts[idx]++
		totals[idx] = totals[idx].Add(v)
	}

	buckets := make([]CostDistributionBucket, len(bands))
	for i, band := range bands {
		hi := band.hi
		buckets[i] = CostDistributionBucket{
			Label:        band.label,
			Lo:           band.lo,
			Hi:           &hi,
			SessionCount: counts[i],
			TotalUSD:     totals[i].String(),
		}
	}

	sorted := append([]decimal.Decimal(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LessThan(sorted[j]) })

	return &TrendsCostDistributionCard{
		Buckets:             buckets,
		Percentiles:         costDistributionPercentiles(sorted),
		CoveredSessionCount: covered,
		TotalSessionCount:   total,
		TimedOut:            false,
	}
}

// costDistributionPercentiles returns the p50/p90/p99 of the SORTED (ascending)
// cost values, or nil when there are none.
func costDistributionPercentiles(sorted []decimal.Decimal) *CostPercentiles {
	if len(sorted) == 0 {
		return nil
	}
	return &CostPercentiles{
		P50: percentileCont(sorted, pctP50).String(),
		P90: percentileCont(sorted, pctP90).String(),
		P99: percentileCont(sorted, pctP99).String(),
	}
}

// percentileCont computes the p-th percentile (p in [0,1]) of a SORTED ascending
// slice using linear interpolation between closest ranks — matching Postgres'
// percentile_cont. Arithmetic stays in decimal so the result is exact.
func percentileCont(sorted []decimal.Decimal, p decimal.Decimal) decimal.Decimal {
	n := len(sorted)
	if n == 1 {
		return sorted[0]
	}
	rank := p.Mul(decimal.NewFromInt(int64(n - 1)))
	loDec := rank.Floor()
	lo := int(loDec.IntPart())
	if lo+1 >= n {
		return sorted[lo]
	}
	frac := rank.Sub(loDec)
	return sorted[lo].Add(frac.Mul(sorted[lo+1].Sub(sorted[lo])))
}

// degradedCostDistributionCard logs the shared PII-safe timeout WARN and returns
// the empty, TimedOut card.
func degradedCostDistributionCard(ctx context.Context, userID int64, req TrendsRequest, started time.Time, err error) *TrendsCostDistributionCard {
	logTrendsCardTimeout(ctx, "cost_distribution", userID, req, started, err)
	return &TrendsCostDistributionCard{Buckets: []CostDistributionBucket{}, TimedOut: true}
}
