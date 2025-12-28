// Package analytics provides session analytics computation and caching.
package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// SessionAnalytics represents computed analytics for a session.
type SessionAnalytics struct {
	SessionID         string    `json:"session_id"`
	AnalyticsVersion  int       `json:"analytics_version"`
	UpToLine          int64     `json:"up_to_line"`
	ComputedAt        time.Time `json:"computed_at"`

	// Token stats
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`

	// Cost
	EstimatedCostUSD decimal.Decimal `json:"estimated_cost_usd"`

	// Compaction stats
	CompactionAuto      int  `json:"compaction_auto"`
	CompactionManual    int  `json:"compaction_manual"`
	CompactionAvgTimeMs *int `json:"compaction_avg_time_ms,omitempty"`

	// Flexible details for future expansion
	Details map[string]interface{} `json:"details,omitempty"`
}

// AnalyticsResponse is the API response format for analytics.
// Cache details are internal - the frontend just gets the computed data.
type AnalyticsResponse struct {
	ComputedLines int64          `json:"computed_lines"` // Line count when analytics were computed
	Tokens        TokenStats     `json:"tokens"`
	Cost          CostStats      `json:"cost"`
	Compaction    CompactionInfo `json:"compaction"`
}

// TokenStats contains token usage information.
type TokenStats struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
}

// CostStats contains cost information.
type CostStats struct {
	EstimatedUSD decimal.Decimal `json:"estimated_usd"`
}

// CompactionInfo contains compaction statistics.
type CompactionInfo struct {
	Auto      int  `json:"auto"`
	Manual    int  `json:"manual"`
	AvgTimeMs *int `json:"avg_time_ms,omitempty"`
}
