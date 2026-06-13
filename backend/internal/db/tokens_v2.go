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
	return fmt.Sprintf("%s.data->>'total_cost_usd'", alias)
}
