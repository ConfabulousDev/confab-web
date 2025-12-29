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
	SessionCardVersion    = 1
	ToolsCardVersion      = 2 // v2: per-tool success/error breakdown
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

// SessionCardRecord is the DB record for the session card.
type SessionCardRecord struct {
	SessionID      string    `json:"session_id"`
	Version        int       `json:"version"`
	ComputedAt     time.Time `json:"computed_at"`
	UpToLine       int64     `json:"up_to_line"`
	UserTurns      int       `json:"user_turns"`
	AssistantTurns int       `json:"assistant_turns"`
	DurationMs     *int64    `json:"duration_ms,omitempty"`
	ModelsUsed     []string  `json:"models_used"` // Stored as JSON array
}

// ToolsCardRecord is the DB record for the tools card.
type ToolsCardRecord struct {
	SessionID  string                `json:"session_id"`
	Version    int                   `json:"version"`
	ComputedAt time.Time             `json:"computed_at"`
	UpToLine   int64                 `json:"up_to_line"`
	TotalCalls int                   `json:"total_calls"`
	ToolStats  map[string]*ToolStats `json:"tool_stats"` // Per-tool success/error counts
	ErrorCount int                   `json:"error_count"`
}

// Cards aggregates all card data for a session.
type Cards struct {
	Tokens     *TokensCardRecord
	Cost       *CostCardRecord
	Compaction *CompactionCardRecord
	Session    *SessionCardRecord
	Tools      *ToolsCardRecord
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

// SessionCardData is the API response format for the session card.
type SessionCardData struct {
	UserTurns      int      `json:"user_turns"`
	AssistantTurns int      `json:"assistant_turns"`
	DurationMs     *int64   `json:"duration_ms,omitempty"`
	ModelsUsed     []string `json:"models_used"`
}

// ToolsCardData is the API response format for the tools card.
type ToolsCardData struct {
	TotalCalls int                   `json:"total_calls"`
	ToolStats  map[string]*ToolStats `json:"tool_stats"` // Per-tool success/error counts
	ErrorCount int                   `json:"error_count"`
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

// IsValid checks if a session card record is valid for the current line count.
func (c *SessionCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == SessionCardVersion && c.UpToLine == currentLineCount
}

// IsValid checks if a tools card record is valid for the current line count.
func (c *ToolsCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == ToolsCardVersion && c.UpToLine == currentLineCount
}

// AllValid checks if all cards are valid for the current line count.
func (c *Cards) AllValid(currentLineCount int64) bool {
	if c == nil {
		return false
	}
	return c.Tokens.IsValid(currentLineCount) &&
		c.Cost.IsValid(currentLineCount) &&
		c.Compaction.IsValid(currentLineCount) &&
		c.Session.IsValid(currentLineCount) &&
		c.Tools.IsValid(currentLineCount)
}
