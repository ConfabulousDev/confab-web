package analytics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// unpriced_models.go: the admin "pricing gap" surface (axk2). It answers "which
// model families appear in stored session data but are absent from the active
// pricing table?" so a newly-released, unpriced model is visible on an admin page
// within minutes instead of only as a backend WARN ("unknown model for pricing",
// pricing.go) that someone has to grep for.
//
// The active pricing table lives in memory (activePricing, an atomic.Pointer —
// NOT a DB table), so the gap is computed in Go: SQL extracts the distinct
// (provider, model-key, session, last-computed) tuples from the tokens_v2 tree,
// and UnpricedModels subtracts the families present in ActivePricingFamilies().
// Keys are normalized to bounded-cardinality families via the same
// normalizeV2ModelKey + getModelFamily logic the cost surfaces use, so the result
// can't blow up keyed by raw dated ids.

// UnpricedModel is one (provider, family) group seen in stored session data whose
// family is missing from the active pricing table. SessionCount is the number of
// distinct sessions that used it; LastSeen is the most recent tokens_v2
// computed_at across those sessions — a recompute time, an acceptable proxy for
// "last seen" (it is not a true ingestion time; surface it as such in the UI).
type UnpricedModel struct {
	Provider     string    `json:"provider"`
	Family       string    `json:"family"`
	SessionCount int       `json:"session_count"`
	LastSeen     time.Time `json:"last_seen"`
}

// ActivePricingFamilies returns the set of family keys in the currently-active
// pricing table (e.g. "opus-4-5", "gpt-5"). It reads the same lock-free
// atomic.Pointer LookupPricing uses, so it reflects any runtime price refresh.
// Exported so the admin pricing-gap surface can diff stored model families
// against priced ones without re-deriving the table.
func ActivePricingFamilies() map[string]struct{} {
	table := *activePricing.Load()
	fams := make(map[string]struct{}, len(table))
	for family := range table {
		fams[family] = struct{}{}
	}
	return fams
}

// unpricedModelScanSQL expands the tokens_v2 tree of EVERY stored session into one
// row per (session, provider-vendor-key, model-key, computed_at). It joins
// sessions for session_type so the model key can be normalized provider-aware in
// Go (OpenCode stores raw dated ids; claude/codex store families). No user
// scoping — this is an admin/system-wide view behind the super-admin gate.
const unpricedModelScanSQL = `
	SELECT s.session_type, v.session_id, mdl.key, v.computed_at
	FROM session_card_tokens_v2 v
	JOIN sessions s ON s.id = v.session_id
	CROSS JOIN LATERAL jsonb_each(v.data->'by_provider') AS prov(key, value)
	CROSS JOIN LATERAL jsonb_each(prov.value->'models') AS mdl(key, value)`

// unpricedBucket accumulates one (provider, family) group as scan rows fold in.
type unpricedBucket struct {
	provider string
	family   string
	sessions map[string]struct{}
	lastSeen time.Time
}

// UnpricedModels returns the model families present in stored session data whose
// family is absent from the active pricing table, grouped by (canonical provider,
// family) with a distinct-session count and a last-seen (max computed_at) proxy
// (axk2). Bounded-cardinality: model keys are collapsed to families
// (normalizeV2ModelKey + " · fast" stripped), and the empty (Unknown) key and the
// synthetic sentinel are excluded since neither is a real model. Rows are ordered
// by session count desc, then (provider, family) for stable output.
func (s *Store) UnpricedModels(ctx context.Context) ([]UnpricedModel, error) {
	priced := ActivePricingFamilies()

	rows, err := s.db.QueryContext(ctx, unpricedModelScanSQL)
	if err != nil {
		return nil, fmt.Errorf("unpriced models query: %w", err)
	}
	defer rows.Close()

	buckets := map[string]*unpricedBucket{}
	for rows.Next() {
		var sessionType, sessionID, rawModel string
		var computedAt time.Time
		if err := rows.Scan(&sessionType, &sessionID, &rawModel, &computedAt); err != nil {
			return nil, fmt.Errorf("unpriced models scan: %w", err)
		}
		provider := models.NormalizeProvider(sessionType)

		// Collapse to the pricing-table key: provider-aware family normalization,
		// then strip Claude's " · fast" suffix so a fast turn of a priced family
		// isn't mistaken for an unpriced family.
		family := strings.TrimSuffix(normalizeV2ModelKey(provider, rawModel), fastModelKeySuffix)
		if family == "" || family == syntheticModelKey {
			continue // Unknown key / synthetic turn — not a real model
		}
		if _, ok := priced[family]; ok {
			continue // present in the active pricing table — not a gap
		}

		groupKey := provider + "\x00" + family
		b := buckets[groupKey]
		if b == nil {
			b = &unpricedBucket{provider: provider, family: family, sessions: map[string]struct{}{}}
			buckets[groupKey] = b
		}
		b.sessions[sessionID] = struct{}{}
		if computedAt.After(b.lastSeen) {
			b.lastSeen = computedAt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unpriced models rows: %w", err)
	}

	out := make([]UnpricedModel, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, UnpricedModel{
			Provider:     b.provider,
			Family:       b.family,
			SessionCount: len(b.sessions),
			LastSeen:     b.lastSeen,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SessionCount != out[j].SessionCount {
			return out[i].SessionCount > out[j].SessionCount
		}
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Family < out[j].Family
	})
	return out, nil
}
