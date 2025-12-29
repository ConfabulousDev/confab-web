// Package analytics provides session analytics computation and caching.
package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// AnalyticsResponse is the API response format for analytics.
// This response includes both the legacy flat format (Tokens, Cost, Compaction)
// and the new cards-based format (Cards map). During migration, frontend can
// transition from flat fields to Cards. Once complete, flat fields will be removed.
type AnalyticsResponse struct {
	ComputedAt    time.Time `json:"computed_at"`    // When analytics were computed
	ComputedLines int64     `json:"computed_lines"` // Line count when analytics were computed

	// Legacy flat format (deprecated - use Cards instead)
	Tokens     TokenStats     `json:"tokens"`
	Cost       CostStats      `json:"cost"`
	Compaction CompactionInfo `json:"compaction"`

	// New cards-based format
	Cards map[string]interface{} `json:"cards"`
}

// TokenStats contains token usage information (legacy flat format).
type TokenStats struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
}

// CostStats contains cost information (legacy flat format).
type CostStats struct {
	EstimatedUSD decimal.Decimal `json:"estimated_usd"`
}

// CompactionInfo contains compaction statistics (legacy flat format).
type CompactionInfo struct {
	Auto      int  `json:"auto"`
	Manual    int  `json:"manual"`
	AvgTimeMs *int `json:"avg_time_ms,omitempty"`
}
