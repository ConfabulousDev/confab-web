package analytics

import (
	"context"
	"errors"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// v2_model_key.go: provider-aware normalization of session_card_tokens_v2 model
// keys, plus the timeout predicate for the cost-by-model aggregation (2hh1).
//
// The v2 tree stores model keys differently per provider (verified 7eje):
//   - claude-code / codex: keys are already getModelFamily() families, and Claude
//     bakes the " · fast" suffix into the key. They must NOT be re-normalized
//     (getModelFamily on "opus-4-5 · fast" would mangle it).
//   - opencode: keyed by vendor with RAW model ids (e.g. "claude-opus-4-5-
//     20251101"); these need getModelFamily() at read time to collapse to a
//     family.
//
// normalizeV2ModelKey centralizes that provider-aware rule so the breakdown
// aggregation, the model filter, and the filter-options list all group
// identically (and y1w5 can reuse it).

// normalizeV2ModelKey maps a raw v2 model key to its display/grouping family,
// given the (any-form) session provider. Empty keys pass through as "" (the
// Unknown row). Only OpenCode's raw vendor keys are run through getModelFamily;
// Claude/Codex keys are already families and are returned verbatim so the
// " · fast" suffix survives intact.
func normalizeV2ModelKey(provider, rawKey string) string {
	if rawKey == "" {
		return ""
	}
	if models.NormalizeProvider(provider) == models.ProviderOpencode {
		return getModelFamily(rawKey)
	}
	return rawKey
}

// isTimeoutErr reports whether err is a query cancellation/deadline that should
// degrade the cost-by-model card rather than fail the whole Trends response —
// a context deadline (context.DeadlineExceeded, possibly wrapped) or a Postgres
// query_canceled (SQLSTATE 57014).
func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "57014" {
		return true
	}
	return false
}
