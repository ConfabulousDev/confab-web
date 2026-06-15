package db

import "fmt"

// V2TotalCostExpr returns the SQL text expression that extracts a session's
// total estimated cost (USD) from its session_card_tokens_v2.data JSONB blob,
// given that table's alias in the surrounding query (e.g. "v"). The result is
// text and nullable — NULL when the joined row or the key is absent — so:
//
//   - presentational LEFT-JOIN callers that want "no card → no cost" use it raw
//     (the session list scans it into a *string);
//   - aggregating INNER-JOIN callers that need a number wrap it as
//     COALESCE(<expr>, '0')::numeric for SUM / ORDER BY / comparison.
//
// 37cg centralized this so the cost readers migrated off the flat
// session_card_tokens (v1) table — session list, org analytics, the Trends
// costliest-sessions card — share one source of truth for the JSONB key and
// never drift on its spelling.
func V2TotalCostExpr(alias string) string {
	return v2DataKeyExpr(alias, "total_cost_usd")
}

// v2DataKeyExpr returns the SQL text expression extracting one top-level key from
// the aliased session_card_tokens_v2.data JSONB blob. Shared by the cost and
// count accessors below so the `alias.data->>'key'` shape lives in one place.
func v2DataKeyExpr(alias, key string) string {
	return fmt.Sprintf("%s.data->>'%s'", alias, key)
}

// V2TotalInputExpr, V2TotalOutputExpr, V2TotalCacheCreationExpr, and
// V2TotalCacheReadExpr return the SQL text expressions extracting a session's
// top-level token COUNTS (not dollars) from its session_card_tokens_v2.data
// JSONB blob, given that table's alias. Like V2TotalCostExpr the result is text
// and nullable — NULL when the joined row or key is absent — so aggregating
// callers wrap each as COALESCE(<expr>, '0')::bigint for SUM.
//
// pjnz added total_cache_creation / total_cache_read to TokensV2Data (mirroring
// the existing total_input / total_output scalars) so the Trends daily
// time-series reader could move off the flat v1 session_card_tokens table — it
// sums these four counts plus cost per day. The counts are provider-agnostic
// session scalars that reproduce the v1 flat columns exactly.
func V2TotalInputExpr(alias string) string {
	return v2DataKeyExpr(alias, "total_input")
}

func V2TotalOutputExpr(alias string) string {
	return v2DataKeyExpr(alias, "total_output")
}

func V2TotalCacheCreationExpr(alias string) string {
	return v2DataKeyExpr(alias, "total_cache_creation")
}

func V2TotalCacheReadExpr(alias string) string {
	return v2DataKeyExpr(alias, "total_cache_read")
}
