package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// Card version constants - increment when compute logic changes
const (
	TokensCardVersion     = 1
	CostCardVersion       = 1
	CompactionCardVersion = 1
)

// =============================================================================
// Database record types (stored in session_card_* tables)
// =============================================================================

// TokensCardRecord is the DB record for the tokens card.
type TokensCardRecord struct {
	SessionID           string    `json:"session_id"`
	Version             int       `json:"version"`
	ComputedAt          time.Time `json:"computed_at"`
	UpToLine            int64     `json:"up_to_line"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
}

// CostCardRecord is the DB record for the cost card.
type CostCardRecord struct {
	SessionID        string          `json:"session_id"`
	Version          int             `json:"version"`
	ComputedAt       time.Time       `json:"computed_at"`
	UpToLine         int64           `json:"up_to_line"`
	EstimatedCostUSD decimal.Decimal `json:"estimated_cost_usd"`
}

// CompactionCardRecord is the DB record for the compaction card.
type CompactionCardRecord struct {
	SessionID   string    `json:"session_id"`
	Version     int       `json:"version"`
	ComputedAt  time.Time `json:"computed_at"`
	UpToLine    int64     `json:"up_to_line"`
	AutoCount   int       `json:"auto_count"`
	ManualCount int       `json:"manual_count"`
	AvgTimeMs   *int      `json:"avg_time_ms,omitempty"`
}

// Cards aggregates all card data for a session.
type Cards struct {
	Tokens     *TokensCardRecord
	Cost       *CostCardRecord
	Compaction *CompactionCardRecord
}

// =============================================================================
// API response types (returned in JSON)
// =============================================================================

// TokensCardData is the API response format for the tokens card.
type TokensCardData struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
}

// CostCardData is the API response format for the cost card.
type CostCardData struct {
	EstimatedUSD string `json:"estimated_usd"` // Decimal as string for precision
}

// CompactionCardData is the API response format for the compaction card.
type CompactionCardData struct {
	Auto      int  `json:"auto"`
	Manual    int  `json:"manual"`
	AvgTimeMs *int `json:"avg_time_ms,omitempty"`
}

// =============================================================================
// Validation helpers
// =============================================================================

// IsValid checks if a tokens card record is valid for the current line count.
func (c *TokensCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == TokensCardVersion && c.UpToLine == currentLineCount
}

// IsValid checks if a cost card record is valid for the current line count.
func (c *CostCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == CostCardVersion && c.UpToLine == currentLineCount
}

// IsValid checks if a compaction card record is valid for the current line count.
func (c *CompactionCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == CompactionCardVersion && c.UpToLine == currentLineCount
}

// AllValid checks if all cards are valid for the current line count.
func (c *Cards) AllValid(currentLineCount int64) bool {
	if c == nil {
		return false
	}
	return c.Tokens.IsValid(currentLineCount) &&
		c.Cost.IsValid(currentLineCount) &&
		c.Compaction.IsValid(currentLineCount)
}
