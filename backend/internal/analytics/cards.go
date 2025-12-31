package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// Card version constants - increment when compute logic changes
const (
	TokensCardVersion       = 2 // v2: added estimated_cost_usd (merged from cost card)
	SessionCardVersion      = 4 // v4: moved turn counts to conversation card
	ToolsCardVersion        = 2 // v2: per-tool success/error breakdown
	CodeActivityCardVersion = 2 // v2: Edit counts full old/new lines (matches GitHub diff)
	ConversationCardVersion = 1 // v1: initial version with turn timing metrics
	AgentsCardVersion       = 1 // v1: initial version with agent invocation counts by type
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

// SessionCardRecord is the DB record for the session card (includes compaction and message breakdown).
// Note: Turn counts are in the Conversation card.
type SessionCardRecord struct {
	SessionID  string    `json:"session_id"`
	Version    int       `json:"version"`
	ComputedAt time.Time `json:"computed_at"`
	UpToLine   int64     `json:"up_to_line"`

	// Message counts (raw line counts)
	TotalMessages     int `json:"total_messages"`
	UserMessages      int `json:"user_messages"`
	AssistantMessages int `json:"assistant_messages"`

	// Message type breakdown
	HumanPrompts   int `json:"human_prompts"`
	ToolResults    int `json:"tool_results"`
	TextResponses  int `json:"text_responses"`
	ToolCalls      int `json:"tool_calls"`
	ThinkingBlocks int `json:"thinking_blocks"`

	// Session metadata
	DurationMs *int64   `json:"duration_ms,omitempty"`
	ModelsUsed []string `json:"models_used"`

	// Compaction stats
	CompactionAuto      int  `json:"compaction_auto"`
	CompactionManual    int  `json:"compaction_manual"`
	CompactionAvgTimeMs *int `json:"compaction_avg_time_ms,omitempty"`
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

// CodeActivityCardRecord is the DB record for the code activity card.
type CodeActivityCardRecord struct {
	SessionID         string         `json:"session_id"`
	Version           int            `json:"version"`
	ComputedAt        time.Time      `json:"computed_at"`
	UpToLine          int64          `json:"up_to_line"`
	FilesRead         int            `json:"files_read"`
	FilesModified     int            `json:"files_modified"`
	LinesAdded        int            `json:"lines_added"`
	LinesRemoved      int            `json:"lines_removed"`
	SearchCount       int            `json:"search_count"`
	LanguageBreakdown map[string]int `json:"language_breakdown"` // extension -> count
}

// ConversationCardRecord is the DB record for the conversation card.
// It tracks turn counts and timing metrics for conversational turns.
type ConversationCardRecord struct {
	SessionID          string    `json:"session_id"`
	Version            int       `json:"version"`
	ComputedAt         time.Time `json:"computed_at"`
	UpToLine           int64     `json:"up_to_line"`
	UserTurns          int       `json:"user_turns"`                      // Count of human prompts
	AssistantTurns     int       `json:"assistant_turns"`                 // Count of text responses
	AvgAssistantTurnMs *int64    `json:"avg_assistant_turn_ms,omitempty"` // Average assistant turn duration
	AvgUserThinkingMs  *int64    `json:"avg_user_thinking_ms,omitempty"`  // Average user thinking time
}

// AgentStats holds success and error counts for a single agent type.
type AgentStats struct {
	Success int `json:"success"`
	Errors  int `json:"errors"`
}

// AgentsCardRecord is the DB record for the agents card.
type AgentsCardRecord struct {
	SessionID        string                 `json:"session_id"`
	Version          int                    `json:"version"`
	ComputedAt       time.Time              `json:"computed_at"`
	UpToLine         int64                  `json:"up_to_line"`
	TotalInvocations int                    `json:"total_invocations"`
	AgentStats       map[string]*AgentStats `json:"agent_stats"` // Per-agent-type success/error counts
}

// Cards aggregates all card data for a session.
type Cards struct {
	Tokens       *TokensCardRecord
	Session      *SessionCardRecord
	Tools        *ToolsCardRecord
	CodeActivity *CodeActivityCardRecord
	Conversation *ConversationCardRecord
	Agents       *AgentsCardRecord
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

// SessionCardData is the API response format for the session card (includes compaction and message breakdown).
// Note: Turn counts are in the Conversation card.
type SessionCardData struct {
	// Message counts (raw line counts)
	TotalMessages     int `json:"total_messages"`
	UserMessages      int `json:"user_messages"`
	AssistantMessages int `json:"assistant_messages"`

	// Message type breakdown
	HumanPrompts   int `json:"human_prompts"`
	ToolResults    int `json:"tool_results"`
	TextResponses  int `json:"text_responses"`
	ToolCalls      int `json:"tool_calls"`
	ThinkingBlocks int `json:"thinking_blocks"`

	// Session metadata
	DurationMs *int64   `json:"duration_ms,omitempty"`
	ModelsUsed []string `json:"models_used"`

	// Compaction stats
	CompactionAuto      int  `json:"compaction_auto"`
	CompactionManual    int  `json:"compaction_manual"`
	CompactionAvgTimeMs *int `json:"compaction_avg_time_ms,omitempty"`
}

// ToolsCardData is the API response format for the tools card.
type ToolsCardData struct {
	TotalCalls int                   `json:"total_calls"`
	ToolStats  map[string]*ToolStats `json:"tool_stats"` // Per-tool success/error counts
	ErrorCount int                   `json:"error_count"`
}

// CodeActivityCardData is the API response format for the code activity card.
type CodeActivityCardData struct {
	FilesRead         int            `json:"files_read"`
	FilesModified     int            `json:"files_modified"`
	LinesAdded        int            `json:"lines_added"`
	LinesRemoved      int            `json:"lines_removed"`
	SearchCount       int            `json:"search_count"`
	LanguageBreakdown map[string]int `json:"language_breakdown"`
}

// ConversationCardData is the API response format for the conversation card.
type ConversationCardData struct {
	UserTurns          int    `json:"user_turns"`
	AssistantTurns     int    `json:"assistant_turns"`
	AvgAssistantTurnMs *int64 `json:"avg_assistant_turn_ms,omitempty"`
	AvgUserThinkingMs  *int64 `json:"avg_user_thinking_ms,omitempty"`
}

// AgentsCardData is the API response format for the agents card.
type AgentsCardData struct {
	TotalInvocations int                    `json:"total_invocations"`
	AgentStats       map[string]*AgentStats `json:"agent_stats"` // Per-agent-type success/error counts
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

// IsValid checks if a code activity card record is valid for the current line count.
func (c *CodeActivityCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == CodeActivityCardVersion && c.UpToLine == currentLineCount
}

// IsValid checks if a conversation card record is valid for the current line count.
func (c *ConversationCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == ConversationCardVersion && c.UpToLine == currentLineCount
}

// IsValid checks if an agents card record is valid for the current line count.
func (c *AgentsCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == AgentsCardVersion && c.UpToLine == currentLineCount
}

// AllValid checks if all cards are valid for the current line count.
func (c *Cards) AllValid(currentLineCount int64) bool {
	if c == nil {
		return false
	}
	return c.Tokens.IsValid(currentLineCount) &&
		c.Session.IsValid(currentLineCount) &&
		c.Tools.IsValid(currentLineCount) &&
		c.CodeActivity.IsValid(currentLineCount) &&
		c.Conversation.IsValid(currentLineCount) &&
		c.Agents.IsValid(currentLineCount)
}
