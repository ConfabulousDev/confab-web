package analytics

import "time"

// =============================================================================
// Request types
// =============================================================================

// TrendsRequest contains parameters for querying trends.
type TrendsRequest struct {
	StartTS       int64    // Start of date range (epoch seconds, inclusive — local midnight)
	EndTS         int64    // End of date range (epoch seconds, exclusive — local midnight of day after last day)
	TZOffset      int      // Client timezone offset in minutes (from JS getTimezoneOffset: positive=behind UTC, negative=ahead)
	Repos         []string // Filter by extracted repo names (e.g., "org/repo", not full URL)
	IncludeNoRepo bool     // Include sessions without a git repo
	// Providers filters sessions by canonical session_type (claude-code, codex).
	// nil/empty aggregates across models.AllowedProviders; resolveProviderFilter
	// expands canonical values with legacy aliases before the SQL ANY clause.
	Providers []string
	// Owners narrows results to sessions whose owner email matches one of
	// the provided values (case-insensitive). nil/empty = aggregate across
	// all visible owners. Privacy invariant: the filter narrows within the
	// visible set defined by db.VisibleSessionsCTE; it cannot broaden access
	// to sessions the caller couldn't already see. CF-495.
	Owners []string
	// ShareAllSessions mirrors db.DB.ShareAllSessions. Threaded through the
	// request so analytics.Store doesn't need a direct dependency on *db.DB.
	// The HTTP handler / worker reads database.ShareAllSessions and assigns
	// it here. CF-495.
	ShareAllSessions bool
	// TopSessionsLimit bounds the Costliest Sessions card (?top_n=). Values
	// outside the {10,25,50} allowlist (including the int zero-value when the
	// caller omits it) are normalized to 10 inside aggregateTopSessions, so the
	// data layer never emits LIMIT 0.
	TopSessionsLimit int
	// Models narrows results to sessions that used at least one of the given
	// model-family keys (2hh1). Family-grain (e.g. "opus-4-5", "opus-4-5 · fast",
	// "gpt-5"); matched after provider-aware normalization via normalizeV2ModelKey,
	// so OpenCode's raw vendor keys collapse to families before comparison.
	// nil/empty = no model narrowing. Scope is session-level (like Owners): a
	// session matches if any of its v2 models matches; per-card costs are NOT
	// re-scoped to the selected model's portion (see c30r/y1w5). AND-combined
	// with Providers.
	Models []string
}

// =============================================================================
// Response types
// =============================================================================

// TrendsResponse is the API response for trends data.
type TrendsResponse struct {
	ComputedAt    time.Time `json:"computed_at"`
	DateRange     DateRange `json:"date_range"`
	SessionCount  int       `json:"session_count"`
	ReposIncluded []string  `json:"repos_included"`
	IncludeNoRepo bool      `json:"include_no_repo"`
	// ProvidersPresent enumerates the distinct canonical providers in the
	// filtered result set, sorted alphabetically. Always non-nil ([] when
	// empty). Drives the Tokens card's multi-provider caveat when len >= 2
	// (CF-424).
	ProvidersPresent []string    `json:"providers_present"`
	Cards            TrendsCards `json:"cards"`
	// FilterOptions is the pre-materialized owner + repo dropdown source
	// for TrendsPage. Mirrors SessionFilterOptions: the lists reflect the
	// caller's visible-session set and are STATIC across active filter
	// changes (date/repo/provider/owner). Always non-nil; empty slices
	// when nothing is visible. CF-495.
	FilterOptions TrendsFilterOptions `json:"filter_options"`
}

// TrendsFilterOptions surfaces the dropdown source for owners + repos + models
// on TrendsPage. Owners are lowercased; the frontend pins the viewer's own
// email to the top in the component.
type TrendsFilterOptions struct {
	Owners []string `json:"owners"`
	Repos  []string `json:"repos"`
	// Models lists the distinct normalized model-family keys (family + "· fast"
	// variants) across the caller's visible sessions, alphabetical. Sources the
	// model dropdown (2hh1). Excludes the empty "" key (rendered as the Unknown
	// breakdown row, not a filterable option). Always non-nil; [] when empty.
	Models []string `json:"models"`
}

// DateRange specifies the start and end dates (inclusive).
type DateRange struct {
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD
}

// TrendsCards holds all the trend card data.
type TrendsCards struct {
	Overview         *TrendsOverviewCard         `json:"overview"`
	Tokens           *TrendsTokensCard           `json:"tokens"`
	Activity         *TrendsActivityCard         `json:"activity"`
	Tools            *TrendsToolsCard            `json:"tools"`
	Utilization      *TrendsUtilizationCard      `json:"utilization"`
	AgentsAndSkills  *TrendsAgentsAndSkillsCard  `json:"agents_and_skills"`
	TopSessions      *TrendsTopSessionsCard      `json:"top_sessions"`
	CostByModel      *TrendsCostByModelCard      `json:"cost_by_model"`
	CostDistribution *TrendsCostDistributionCard `json:"cost_distribution"`
}

// =============================================================================
// Card types
// =============================================================================

// TrendsOverviewCard provides session count and duration summary.
type TrendsOverviewCard struct {
	SessionCount             int      `json:"session_count"`
	TotalDurationMs          int64    `json:"total_duration_ms"`
	AvgDurationMs            *int64   `json:"avg_duration_ms,omitempty"`
	DaysCovered              int      `json:"days_covered"`
	TotalAssistantDurationMs int64    `json:"total_assistant_duration_ms"`
	AssistantUtilizationPct  *float64 `json:"assistant_utilization_pct,omitempty"`
}

// TrendsTokensCard provides token usage and cost summary.
//
// CF-435: PerProvider holds per-canonical-provider breakdowns of the same
// metrics, populated server-side regardless of how many providers are in the
// filtered range. Always non-nil ({} when empty range). Cross-provider total
// fields stay populated for non-UI consumers; the frontend switches to a
// per-provider table when len(PerProvider) >= 2.
type TrendsTokensCard struct {
	TotalInputTokens         int64                               `json:"total_input_tokens"`
	TotalOutputTokens        int64                               `json:"total_output_tokens"`
	TotalCacheCreationTokens int64                               `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64                               `json:"total_cache_read_tokens"`
	TotalCostUSD             string                              `json:"total_cost_usd"` // Decimal as string
	DailyCosts               []DailyCostPoint                    `json:"daily_costs"`
	PerProvider              map[string]*TrendsTokensPerProvider `json:"per_provider"`
}

// TrendsTokensPerProvider holds aggregated token usage and cost for one
// canonical provider (e.g. "claude-code", "codex"). Legacy session_type
// aliases are folded into the canonical key at the Scan site via
// models.NormalizeProvider before reaching this struct.
type TrendsTokensPerProvider struct {
	TotalInputTokens         int64  `json:"total_input_tokens"`
	TotalOutputTokens        int64  `json:"total_output_tokens"`
	TotalCacheCreationTokens int64  `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64  `json:"total_cache_read_tokens"`
	TotalCostUSD             string `json:"total_cost_usd"` // Decimal as string
}

// DailyCostPoint represents a single day's cost for charting. CostUSD is the
// cross-provider total for the day; PerProvider holds the per-provider
// breakdown for that day (canonical provider id → decimal cost as string),
// driving the stacked-bar chart on the frontend. Always non-nil — an empty
// map is emitted for days with no sessions so JSON yields `{}` not `null`.
type DailyCostPoint struct {
	Date        string            `json:"date"` // YYYY-MM-DD
	CostUSD     string            `json:"cost_usd"`
	PerProvider map[string]string `json:"per_provider"`
}

// TrendsActivityCard provides code activity summary.
type TrendsActivityCard struct {
	TotalFilesRead     int                 `json:"total_files_read"`
	TotalFilesModified int                 `json:"total_files_modified"`
	TotalLinesAdded    int                 `json:"total_lines_added"`
	TotalLinesRemoved  int                 `json:"total_lines_removed"`
	DailySessionCounts []DailySessionCount `json:"daily_session_counts"`
}

// DailySessionCount represents a single day's session count for charting.
// PerProvider keys are canonical provider ids (legacy session_type values are
// folded via models.NormalizeProvider at the Scan site). Always non-nil —
// emitted as `{}` for days with no sessions so JSON yields `{}` not `null`.
// Drives the stacked-bar chart on the frontend, mirroring DailyCostPoint
// from the Tokens card (CF-444).
type DailySessionCount struct {
	Date         string         `json:"date"` // YYYY-MM-DD
	SessionCount int            `json:"session_count"`
	PerProvider  map[string]int `json:"per_provider"`
}

// TrendsToolsCard provides tool usage summary.
type TrendsToolsCard struct {
	TotalCalls  int                   `json:"total_calls"`
	TotalErrors int                   `json:"total_errors"`
	ToolStats   map[string]*ToolStats `json:"tool_stats"` // Per-tool breakdown
}

// TrendsUtilizationCard provides daily assistant utilization breakdown.
type TrendsUtilizationCard struct {
	DailyUtilization []DailyUtilizationPoint `json:"daily_utilization"`
}

// DailyUtilizationPoint represents a single day's utilization for charting.
type DailyUtilizationPoint struct {
	Date           string   `json:"date"`            // YYYY-MM-DD
	UtilizationPct *float64 `json:"utilization_pct"` // nil if no sessions that day
}

// TrendsAgentsAndSkillsCard provides agent and skill usage summary across sessions.
type TrendsAgentsAndSkillsCard struct {
	TotalAgentInvocations int                    `json:"total_agent_invocations"`
	TotalSkillInvocations int                    `json:"total_skill_invocations"`
	AgentStats            map[string]*AgentStats `json:"agent_stats"`
	SkillStats            map[string]*SkillStats `json:"skill_stats"`
}

// TrendsTopSessionsCard provides the most expensive sessions ranked by cost.
type TrendsTopSessionsCard struct {
	Sessions []TopSessionItem `json:"sessions"`
}

// TrendsCostByModelCard breaks down cost + tokens per (provider, model family)
// across the filtered, visible sessions that carry tokens_v2 data (2hh1).
//
// Scope vs the Tokens headline: rows sum the v2 per-model cost (covers only
// sessions WITH v2 data — partial during backfill), while the h7xe grand-total
// headline sums the flat card (full coverage). They are deliberately DIFFERENT
// scopes and do NOT reconcile; PctOfTotal is each row's share of the v2
// model-attributed total (rows sum to ~100% among themselves), and the frontend
// shows a coverage caption — never a reconciliation line.
//
// TimedOut signals graceful degradation: when the aggregation exceeds its
// dedicated budget the card returns empty with TimedOut=true (the whole Trends
// response still succeeds) and the server emits a PII-safe WARN with the request
// shape for upstream debugging.
type TrendsCostByModelCard struct {
	Rows []CostByModelRow `json:"rows"`
	// CoveredSessionCount = sessions with per-model v2 data contributing to the
	// rows; TotalSessionCount = all filtered sessions in range. The caption reads
	// "Covers N of M sessions with per-model data".
	CoveredSessionCount int  `json:"covered_session_count"`
	TotalSessionCount   int  `json:"total_session_count"`
	TimedOut            bool `json:"timed_out"`
}

// CostByModelRow is one (provider, model-family) bucket. Provider is the
// canonical session provider (claude-code/codex/opencode); Model is the
// normalized family key ("" → rendered "Unknown"; "<family> · fast" kept as its
// own row). Cost is a decimal string; PctOfTotal is a percentage (0–100).
type CostByModelRow struct {
	Model        string  `json:"model"`
	Provider     string  `json:"provider"`
	CostUSD      string  `json:"cost_usd"`
	PctOfTotal   float64 `json:"pct_of_total"`
	Input        int64   `json:"input"`
	Output       int64   `json:"output"`
	CacheRead    int64   `json:"cache_read"`
	CacheWrite   int64   `json:"cache_write"`
	SessionCount int     `json:"session_count"`
}

// TrendsCostDistributionCard is the log-bucket histogram of per-session cost plus
// p50/p90/p99 percentiles, exposing the long tail of spend (y1w5). It reuses the
// same filtered+visible tokens_v2 session set as the Cost by Model card.
//
// Units: by default each data point is one session's total cost
// (session_card_tokens_v2.data->>'total_cost_usd'). When a ?model= filter is active
// the unit becomes per-(session, selected-model): each (session, model) pair is one
// data point and only the selected model's cost in that session counts. The frontend
// flags that unit shift with a ⓘ caveat (driven by modelFilterActive); the wire shape
// is identical either way.
//
// Coverage mirrors Cost by Model: CoveredSessionCount = sessions with tokens_v2 data
// contributing data points; TotalSessionCount = all filtered sessions in range. The
// caption reads "Covers N of M sessions with cost data; percentiles reflect this subset"
// — percentiles are biased by the partial v2 subset during backfill.
//
// Buckets always carries all five fixed log-scale bands (count 0 where empty) for a
// stable, comparable histogram. Percentiles is nil when there are no data points.
// TimedOut signals the same graceful degradation as Cost by Model.
type TrendsCostDistributionCard struct {
	Buckets             []CostDistributionBucket `json:"buckets"`
	Stats               *CostDistributionStats   `json:"stats"`
	CoveredSessionCount int                      `json:"covered_session_count"`
	TotalSessionCount   int                      `json:"total_session_count"`
	TimedOut            bool                     `json:"timed_out"`
}

// CostDistributionBucket is one fixed log-scale cost band. Lo/Hi are the band's
// dollar edges (half-open [Lo, Hi)); Hi is nil for the unbounded top band (> $10).
// Label is the display string the frontend renders verbatim ("$0.01 – $1"). Despite
// the name, SessionCount is the data-point count — sessions by default, or
// (session, model) pairs when a model filter is active. TotalUSD is the decimal-string
// sum of cost across the band's data points.
type CostDistributionBucket struct {
	Label        string   `json:"label"`
	Lo           float64  `json:"lo"`
	Hi           *float64 `json:"hi"`
	SessionCount int      `json:"session_count"`
	TotalUSD     string   `json:"total_usd"`
}

// CostDistributionStats holds the summary statistics of the per-data-point cost
// values: the p50/p90/p99 percentiles (percentile_cont / linear-interpolation
// semantics) plus the arithmetic mean. Each is a decimal string, computed in one Go
// pass. Only present (non-nil on the card) when there is at least one data point.
// Named "stats" rather than "percentiles" because it also carries the (non-percentile) mean.
type CostDistributionStats struct {
	P50 string `json:"p50"`
	P90 string `json:"p90"`
	P99 string `json:"p99"`
	// Avg is the arithmetic mean of the per-data-point cost values.
	Avg string `json:"avg"`
}

// TopSessionItem represents a single session in the top sessions ranking.
type TopSessionItem struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Provider         string  `json:"provider"`
	EstimatedCostUSD string  `json:"estimated_cost_usd"`
	DurationMs       *int64  `json:"duration_ms,omitempty"`
	GitRepo          *string `json:"git_repo,omitempty"`
}
