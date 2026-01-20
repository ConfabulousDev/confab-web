package analytics

import (
	"time"
)

// =============================================================================
// Request types
// =============================================================================

// TrendsRequest contains parameters for querying trends.
type TrendsRequest struct {
	StartDate     time.Time // Start of date range
	EndDate       time.Time // End of date range (exclusive)
	Repos         []string  // Filter by these repo names (explicit list required)
	IncludeNoRepo bool      // Include sessions without a git repo
}

// =============================================================================
// Response types
// =============================================================================

// TrendsResponse is the API response for trends data.
type TrendsResponse struct {
	ComputedAt    time.Time             `json:"computed_at"`
	DateRange     DateRange             `json:"date_range"`
	SessionCount  int                   `json:"session_count"`
	ReposIncluded []string              `json:"repos_included"`
	IncludeNoRepo bool                  `json:"include_no_repo"`
	Cards         TrendsCards           `json:"cards"`
}

// DateRange specifies the start and end dates (inclusive).
type DateRange struct {
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD
}

// TrendsCards holds all the trend card data.
type TrendsCards struct {
	Overview *TrendsOverviewCard `json:"overview"`
	Tokens   *TrendsTokensCard   `json:"tokens"`
	Activity *TrendsActivityCard `json:"activity"`
	Tools    *TrendsToolsCard    `json:"tools"`
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
type TrendsTokensCard struct {
	TotalInputTokens         int64             `json:"total_input_tokens"`
	TotalOutputTokens        int64             `json:"total_output_tokens"`
	TotalCacheCreationTokens int64             `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64             `json:"total_cache_read_tokens"`
	TotalCostUSD             string            `json:"total_cost_usd"` // Decimal as string
	DailyCosts               []DailyCostPoint  `json:"daily_costs"`
}

// DailyCostPoint represents a single day's cost for charting.
type DailyCostPoint struct {
	Date    string `json:"date"` // YYYY-MM-DD
	CostUSD string `json:"cost_usd"`
}

// TrendsActivityCard provides code activity summary.
type TrendsActivityCard struct {
	TotalFilesRead     int                   `json:"total_files_read"`
	TotalFilesModified int                   `json:"total_files_modified"`
	TotalLinesAdded    int                   `json:"total_lines_added"`
	TotalLinesRemoved  int                   `json:"total_lines_removed"`
	DailySessionCounts []DailySessionCount   `json:"daily_session_counts"`
}

// DailySessionCount represents a single day's session count for charting.
type DailySessionCount struct {
	Date         string `json:"date"` // YYYY-MM-DD
	SessionCount int    `json:"session_count"`
}

// TrendsToolsCard provides tool usage summary.
type TrendsToolsCard struct {
	TotalCalls int                        `json:"total_calls"`
	TotalErrors int                       `json:"total_errors"`
	ToolStats  map[string]*ToolStats      `json:"tool_stats"` // Per-tool breakdown
}

// =============================================================================
// Internal aggregation types (used during SQL query)
// =============================================================================

// DailyActivityAggregation holds per-day activity stats from the SQL query.
type DailyActivityAggregation struct {
	Date                string
	SessionCount        int
	FilesRead           int
	FilesModified       int
	LinesAdded          int
	LinesRemoved        int
	DurationMs          int64
	AssistantDurationMs int64
}
