package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// Card version constants - increment when compute logic changes
const (
	TokensCardVersion  = 2 // v2: added estimated_cost_usd (merged from cost card)
	SessionCardVersion = 2 // v2: added compaction stats (merged from compaction card)
	ToolsCardVersion   = 2 // v2: per-tool success/error breakdown
)

// =============================================================================
// Database record types (stored in session_card_* tables)
// =============================================================================

// TokensCardRecord is the DB record for the tokens card (includes cost).
type TokensCardRecord struct {
	SessionID           string          `json:"session_id"`
	Version             int             `json:"version"`
	ComputedAt          time.Time       `json:"computed_at"`
	UpToLine            int64           `json:"up_to_line"`
	InputTokens         int64           `json:"input_tokens"`
	OutputTokens        int64           `json:"output_tokens"`
	CacheCreationTokens int64           `json:"cache_creation_tokens"`
	CacheReadTokens     int64           `json:"cache_read_tokens"`
	EstimatedCostUSD    decimal.Decimal `json:"estimated_cost_usd"`
}

// SessionCardRecord is the DB record for the session card (includes compaction).
type SessionCardRecord struct {
	SessionID          string    `json:"session_id"`
	Version            int       `json:"version"`
	ComputedAt         time.Time `json:"computed_at"`
	UpToLine           int64     `json:"up_to_line"`
	UserTurns          int       `json:"user_turns"`
	AssistantTurns     int       `json:"assistant_turns"`
	DurationMs         *int64    `json:"duration_ms,omitempty"`
	ModelsUsed         []string  `json:"models_used"`
	CompactionAuto     int       `json:"compaction_auto"`
	CompactionManual   int       `json:"compaction_manual"`
	CompactionAvgTimeMs *int     `json:"compaction_avg_time_ms,omitempty"`
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
	Tokens  *TokensCardRecord
	Session *SessionCardRecord
	Tools   *ToolsCardRecord
}

// =============================================================================
// API response types (returned in JSON)
// =============================================================================

// TokensCardData is the API response format for the tokens card (includes cost).
type TokensCardData struct {
	Input         int64  `json:"input"`
	Output        int64  `json:"output"`
	CacheCreation int64  `json:"cache_creation"`
	CacheRead     int64  `json:"cache_read"`
	EstimatedUSD  string `json:"estimated_usd"` // Decimal as string for precision
}

// SessionCardData is the API response format for the session card (includes compaction).
type SessionCardData struct {
	UserTurns          int      `json:"user_turns"`
	AssistantTurns     int      `json:"assistant_turns"`
	DurationMs         *int64   `json:"duration_ms,omitempty"`
	ModelsUsed         []string `json:"models_used"`
	CompactionAuto     int      `json:"compaction_auto"`
	CompactionManual   int      `json:"compaction_manual"`
	CompactionAvgTimeMs *int    `json:"compaction_avg_time_ms,omitempty"`
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
		c.Session.IsValid(currentLineCount) &&
		c.Tools.IsValid(currentLineCount)
}
