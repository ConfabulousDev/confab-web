package analytics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/shopspring/decimal"
)

// costByModelTimeout bounds the per-model cost aggregation. It sits UNDER the
// API layer's global per-request DatabaseTimeout (5s) so a slow JSONB scan
// cancels just this card — degrading it gracefully — instead of failing the
// whole Trends response. A per-model breakdown must expand the tokens_v2 tree
// for every visible session in range, so no index can avoid the scan; this
// budget is the guardrail (2hh1).
const costByModelTimeout = 4 * time.Second

// v2ModelScanSQL expands the tokens_v2 tree of the filtered sessions into one
// row per (session, provider-vendor, model). cost_usd stays a decimal string;
// the family normalization (getModelFamily for OpenCode's raw keys) happens in
// Go via normalizeV2ModelKey, so it is NOT pushed into SQL.
const v2ModelScanSQL = `
	SELECT fs.id, fs.session_type, mdl.key,
		COALESCE(mdl.value->>'cost_usd', '0'),
		COALESCE((mdl.value->>'input')::bigint, 0),
		COALESCE((mdl.value->>'output')::bigint, 0),
		COALESCE((mdl.value->>'cache_read')::bigint, 0),
		COALESCE((mdl.value->>'cache_write')::bigint, 0)
	FROM filtered_sessions fs
	JOIN session_card_tokens_v2 v ON v.session_id = fs.id
	CROSS JOIN LATERAL jsonb_each(v.data->'by_provider') AS prov(key, value)
	CROSS JOIN LATERAL jsonb_each(prov.value->'models') AS mdl(key, value)`

// visibleV2ModelKeysFrom is the FROM/JOIN tail that expands the tokens_v2 tree
// of the caller's visible sessions into one row per (session, model) — the
// shared body of sessionsMatchingModels and modelFilterOptions. Pair it with a
// `WITH <VisibleSessionsCTE>, visible_unique AS (SELECT DISTINCT id FROM
// visible_sessions)` prelude and a SELECT list that draws from vu/s/mdl.
const visibleV2ModelKeysFrom = `
	FROM visible_unique vu
	JOIN sessions s ON s.id = vu.id
	JOIN session_card_tokens_v2 v ON v.session_id = vu.id
	CROSS JOIN LATERAL jsonb_each(v.data->'by_provider') AS prov(key, value)
	CROSS JOIN LATERAL jsonb_each(prov.value->'models') AS mdl(key, value)`

// costByModelBucket accumulates one (provider, model-family) group as the scan
// rows are folded in Go.
type costByModelBucket struct {
	provider   string
	model      string
	cost       decimal.Decimal
	input      int64
	output     int64
	cacheRead  int64
	cacheWrite int64
	sessions   map[string]struct{}
}

// aggregateCostByModel builds the per-(provider, model) cost breakdown over the
// filtered, visible sessions that carry tokens_v2 data (2hh1). Money is summed
// as decimal.Decimal in Go (exact) after a SQL jsonb scan, so OpenCode's raw
// vendor keys can be normalized to families via normalizeV2ModelKey (reusing
// getModelFamily) and Claude's " · fast" keys pass through untouched.
//
// On a query timeout it degrades to an empty card with TimedOut=true (the whole
// Trends response still succeeds) and logs a PII-safe WARN with the request
// shape for upstream debugging.
func (s *Store) aggregateCostByModel(ctx context.Context, tq trendsQuery, userID int64, req TrendsRequest) (*TrendsCostByModelCard, error) {
	cbmCtx, cancel := context.WithTimeout(ctx, costByModelTimeout)
	defer cancel()
	started := time.Now()

	// Denominator for the coverage caption: all filtered sessions in range.
	var total int
	if err := s.db.QueryRowContext(cbmCtx, tq.cteSQL+"\nSELECT COUNT(*) FROM filtered_sessions", tq.args...).Scan(&total); err != nil {
		if isTimeoutErr(err) {
			return degradedCostByModelCard(ctx, userID, req, started, err), nil
		}
		return nil, fmt.Errorf("cost-by-model total count: %w", err)
	}

	rows, err := s.db.QueryContext(cbmCtx, tq.cteSQL+v2ModelScanSQL, tq.args...)
	if err != nil {
		if isTimeoutErr(err) {
			return degradedCostByModelCard(ctx, userID, req, started, err), nil
		}
		return nil, fmt.Errorf("cost-by-model query: %w", err)
	}
	defer rows.Close()

	buckets := map[string]*costByModelBucket{}
	covered := map[string]struct{}{}
	for rows.Next() {
		var sessionID, sessionType, rawModel, costStr string
		var input, output, cacheRead, cacheWrite int64
		if err := rows.Scan(&sessionID, &sessionType, &rawModel, &costStr, &input, &output, &cacheRead, &cacheWrite); err != nil {
			return nil, fmt.Errorf("cost-by-model scan: %w", err)
		}
		provider := models.NormalizeProvider(sessionType)
		model := normalizeV2ModelKey(provider, rawModel)
		cost, err := decimal.NewFromString(costStr)
		if err != nil {
			cost = decimal.Zero
		}

		groupKey := provider + "\x00" + model
		b := buckets[groupKey]
		if b == nil {
			b = &costByModelBucket{provider: provider, model: model, cost: decimal.Zero, sessions: map[string]struct{}{}}
			buckets[groupKey] = b
		}
		b.cost = b.cost.Add(cost)
		b.input += input
		b.output += output
		b.cacheRead += cacheRead
		b.cacheWrite += cacheWrite
		b.sessions[sessionID] = struct{}{}
		covered[sessionID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		if isTimeoutErr(err) {
			return degradedCostByModelCard(ctx, userID, req, started, err), nil
		}
		return nil, fmt.Errorf("cost-by-model rows: %w", err)
	}

	ordered := make([]*costByModelBucket, 0, len(buckets))
	grand := decimal.Zero
	for _, b := range buckets {
		ordered = append(ordered, b)
		grand = grand.Add(b.cost)
	}
	// Cost desc; stable secondary sort by (provider, model) so equal-cost rows
	// (incl. the $0 Unknown / unpriced rows) have a deterministic order.
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].cost.Equal(ordered[j].cost) {
			return ordered[i].cost.GreaterThan(ordered[j].cost)
		}
		if ordered[i].provider != ordered[j].provider {
			return ordered[i].provider < ordered[j].provider
		}
		return ordered[i].model < ordered[j].model
	})

	out := make([]CostByModelRow, 0, len(ordered))
	for _, b := range ordered {
		var pct float64
		if grand.IsPositive() {
			pct, _ = b.cost.Div(grand).Mul(decimal.NewFromInt(100)).Float64()
		}
		out = append(out, CostByModelRow{
			Model:        b.model,
			Provider:     b.provider,
			CostUSD:      b.cost.String(),
			PctOfTotal:   pct,
			Input:        b.input,
			Output:       b.output,
			CacheRead:    b.cacheRead,
			CacheWrite:   b.cacheWrite,
			SessionCount: len(b.sessions),
		})
	}

	return &TrendsCostByModelCard{
		Rows:                out,
		CoveredSessionCount: len(covered),
		TotalSessionCount:   total,
		TimedOut:            false,
	}, nil
}

// degradedCostByModelCard logs a PII-safe WARN describing the request shape (so
// a self-hoster can file a useful upstream issue) and returns the empty,
// TimedOut card. It logs filter SHAPES/COUNTS only — never owner emails or repo
// names.
func degradedCostByModelCard(ctx context.Context, userID int64, req TrendsRequest, started time.Time, err error) *TrendsCostByModelCard {
	rangeDays := 0
	if req.EndTS > req.StartTS {
		rangeDays = int((req.EndTS - req.StartTS) / 86400)
	}
	logger.Ctx(ctx).Warn("trends cost-by-model aggregation timed out — report upstream with this metadata",
		"card", "cost_by_model",
		"user_id", userID,
		"start_ts", req.StartTS,
		"end_ts", req.EndTS,
		"range_days", rangeDays,
		"tz_offset", req.TZOffset,
		"providers", strings.Join(req.Providers, ","),
		"owner_count", len(req.Owners),
		"repo_count", len(req.Repos),
		"include_no_repo", req.IncludeNoRepo,
		"model_filter_count", len(req.Models),
		"timeout_ms", costByModelTimeout.Milliseconds(),
		"elapsed_ms", time.Since(started).Milliseconds(),
		"error", err,
	)
	return &TrendsCostByModelCard{Rows: []CostByModelRow{}, TimedOut: true}
}

// sessionsMatchingModels returns the ids of visible sessions that used at least
// one of req.Models (family-grain, provider-agnostic). Because OpenCode stores
// raw vendor keys, the family match needs normalizeV2ModelKey at read time, so
// it can't be a pure-SQL predicate — the id set is computed here once in Go and
// then threaded into buildTrendsQuery as a bind array so every Trends card
// honors ?model= uniformly (the AND with ?provider= falls out of
// filtered_sessions). Scoped to visible_sessions; the date/repo/provider/owner
// filters are applied downstream in filtered_sessions.
func (s *Store) sessionsMatchingModels(ctx context.Context, userID int64, req TrendsRequest) ([]string, error) {
	selected := make(map[string]struct{}, len(req.Models))
	for _, m := range req.Models {
		selected[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}

	query := `WITH ` + db.VisibleSessionsCTE(req.ShareAllSessions) + `,
		visible_unique AS (SELECT DISTINCT id FROM visible_sessions)
		SELECT vu.id, s.session_type, mdl.key` + visibleV2ModelKeysFrom

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("sessions matching models: %w", err)
	}
	defer rows.Close()

	matched := map[string]struct{}{}
	for rows.Next() {
		var sessionID, sessionType, rawModel string
		if err := rows.Scan(&sessionID, &sessionType, &rawModel); err != nil {
			return nil, fmt.Errorf("sessions matching models scan: %w", err)
		}
		norm := normalizeV2ModelKey(models.NormalizeProvider(sessionType), rawModel)
		if _, ok := selected[strings.ToLower(norm)]; ok {
			matched[sessionID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sessions matching models rows: %w", err)
	}

	out := make([]string, 0, len(matched))
	for id := range matched {
		out = append(out, id)
	}
	return out, nil
}

// modelFilterOptions returns the distinct normalized model-family keys (family +
// "· fast" variants) across the caller's VISIBLE sessions, alphabetical,
// excluding the empty Unknown key. Sources the model dropdown; static across
// active filters, mirroring owners/repos in aggregateFilterOptions (2hh1).
//
// Unlike the cost card this scans ALL visible sessions (not the date-filtered
// set) and expands the v2 tree, so it shares the same JSONB-scan latency risk.
// It is bounded by the same timeout and degrades to an empty dropdown rather
// than failing the whole filter-options aggregation (and with it the Trends
// page) — an absent model dropdown is a far smaller loss than a 500.
func (s *Store) modelFilterOptions(ctx context.Context, userID int64, shareAllSessions bool) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, costByModelTimeout)
	defer cancel()

	query := `WITH ` + db.VisibleSessionsCTE(shareAllSessions) + `,
		visible_unique AS (SELECT DISTINCT id FROM visible_sessions)
		SELECT s.session_type, mdl.key` + visibleV2ModelKeysFrom

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		if isTimeoutErr(err) {
			logger.Ctx(ctx).Warn("trends model filter-options timed out — dropdown empty", "user_id", userID)
			return []string{}, nil
		}
		return nil, fmt.Errorf("model filter options: %w", err)
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	for rows.Next() {
		var sessionType, rawModel string
		if err := rows.Scan(&sessionType, &rawModel); err != nil {
			return nil, fmt.Errorf("model filter options scan: %w", err)
		}
		norm := normalizeV2ModelKey(models.NormalizeProvider(sessionType), rawModel)
		if norm != "" {
			seen[norm] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		if isTimeoutErr(err) {
			logger.Ctx(ctx).Warn("trends model filter-options timed out — dropdown empty", "user_id", userID)
			return []string{}, nil
		}
		return nil, fmt.Errorf("model filter options rows: %w", err)
	}

	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out, nil
}
