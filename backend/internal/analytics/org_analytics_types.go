package analytics

import "time"

// =============================================================================
// Organization Analytics - Request/Response types
// =============================================================================

// OrgAnalyticsRequest contains parameters for querying organization analytics.
type OrgAnalyticsRequest struct {
	StartTS  int64 // Start of date range (epoch seconds, inclusive — local midnight)
	EndTS    int64 // End of date range (epoch seconds, exclusive — local midnight of day after last day)
	TZOffset int   // Client timezone offset in minutes (from JS getTimezoneOffset: positive=behind UTC, negative=ahead)
}

// OrgAnalyticsResponse is the API response for organization analytics.
type OrgAnalyticsResponse struct {
	ComputedAt time.Time          `json:"computed_at"`
	DateRange  DateRange          `json:"date_range"` // Reuses existing DateRange type
	Users      []OrgUserAnalytics `json:"users"`
}

// OrgUserAnalytics represents aggregated analytics for a single user.
// The nested User object future-proofs for team/group aggregation.
type OrgUserAnalytics struct {
	User              OrgUserInfo `json:"user"`
	SessionCount      int         `json:"session_count"`
	TotalCostUSD      string      `json:"total_cost_usd"`
	TotalDurationMs   int64       `json:"total_duration_ms"`
	TotalClaudeTimeMs int64       `json:"total_claude_time_ms"`
	TotalUserTimeMs   int64       `json:"total_user_time_ms"`
	AvgCostUSD        string      `json:"avg_cost_usd"`
	AvgDurationMs     *int64      `json:"avg_duration_ms,omitempty"`
	AvgClaudeTimeMs   *int64      `json:"avg_claude_time_ms,omitempty"`
	AvgUserTimeMs     *int64      `json:"avg_user_time_ms,omitempty"`
}

// OrgUserInfo contains user identity information.
type OrgUserInfo struct {
	ID    int64   `json:"id"`
	Email string  `json:"email"`
	Name  *string `json:"name,omitempty"`
}
