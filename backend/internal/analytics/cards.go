package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// AllCardTableNames is the canonical list of session_card_* table names the admin
// invalidation feature can operate on. Used for request validation and as the driver
// for per-table DELETE statements during an invalidation run.
var AllCardTableNames = []string{
	"session_card_tokens",
	"session_card_tokens_v2",
	"session_card_session",
	"session_card_tools",
	"session_card_code_activity",
	"session_card_conversation",
	"session_card_agents_and_skills",
	"session_card_redactions",
	"session_card_workflows",
	"session_card_smart_recap",
}

// IsKnownCardTableName reports whether name is one of AllCardTableNames.
func IsKnownCardTableName(name string) bool {
	for _, n := range AllCardTableNames {
		if n == name {
			return true
		}
	}
	return false
}

// Card version constants - increment when compute logic changes
const (
	TokensCardVersion          = 3 // v3: dedup by message.id (fixes multi-line + replay over-counting)
	TokensV2CardVersion        = 1 // v1: hierarchical per-provider/per-model breakdown (OpenCode)
	SessionCardVersion         = 5 // v5: dedup assistant counts by message.id, non-exclusive breakdown
	ToolsCardVersion           = 3 // v3: Codex spawn_agent/wait_agent excluded — surfaced via AgentsAndSkills (CF-443)
	CodeActivityCardVersion    = 2 // v2: Edit counts full old/new lines (matches GitHub diff)
	ConversationCardVersion    = 3 // v3: AssistantTurns = user-prompt-triggered sequences (deduped)
	AgentsAndSkillsCardVersion = 2 // v2: Codex subagent + skill support (CF-443)
	RedactionsCardVersion      = 2 // v2: filter out "TYPE" placeholder
	WorkflowsCardVersion       = 1 // v1: per-run workflow subagent aggregates (CF-534)
	SmartRecapCardVersion      = 1 // v1: initial AI-powered session recap
	SearchIndexVersion         = 1 // v1: initial full-text search index
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

	// Fast mode breakdown
	FastTurns   int             `json:"fast_turns"`
	FastCostUSD decimal.Decimal `json:"fast_cost_usd"`
}

// TokensV2Model is one model's token + cost breakdown within a provider. It is
// both the JSONB storage shape (nested under TokensV2Data) and the API wire
// shape, so json tags are snake_case to match the frontend Zod schema. Costs
// are decimal strings for precision.
type TokensV2Model struct {
	Input      int64  `json:"input"`
	Output     int64  `json:"output"`
	CacheRead  int64  `json:"cache_read"`
	CacheWrite int64  `json:"cache_write"`
	Reasoning  int64  `json:"reasoning"`
	CostUSD    string `json:"cost_usd"`
}

// TokensV2Provider aggregates one provider's models and total cost.
type TokensV2Provider struct {
	CostUSD string                   `json:"cost_usd"`
	Models  map[string]TokensV2Model `json:"models"`
}

// TokensV2Data is the hierarchical token-usage tree: session totals plus a
// per-provider → per-model breakdown. Stored verbatim as JSONB in
// session_card_tokens_v2.data and served verbatim as the tokens_v2 card.
type TokensV2Data struct {
	TotalCostUSD string                      `json:"total_cost_usd"`
	TotalInput   int64                       `json:"total_input"`
	TotalOutput  int64                       `json:"total_output"`
	ByProvider   map[string]TokensV2Provider `json:"by_provider"`
}

// TokensV2CardRecord is the DB record for the hierarchical (per-provider /
// per-model) tokens card. It is a universal card written for every session and
// participates in Cards.AllValid and the staleness gate exactly like the others
// — for providers that don't yet build the per-model tree (Claude/Codex) it is
// written with empty Data, mirroring the Workflows card's "always written, empty
// for N/A sessions" pattern. It is served (ToResponse) only when it has provider
// data, so non-OpenCode API responses are unchanged. The long-term plan is for
// tokens_v2 to replace the flat tokens card for all providers.
type TokensV2CardRecord struct {
	SessionID  string       `json:"session_id"`
	Version    int          `json:"version"`
	ComputedAt time.Time    `json:"computed_at"`
	UpToLine   int64        `json:"up_to_line"`
	Data       TokensV2Data `json:"data"`
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
	SessionID                string    `json:"session_id"`
	Version                  int       `json:"version"`
	ComputedAt               time.Time `json:"computed_at"`
	UpToLine                 int64     `json:"up_to_line"`
	UserTurns                int       `json:"user_turns"`                           // Count of human prompts
	AssistantTurns           int       `json:"assistant_turns"`                      // Count of text responses
	AvgAssistantTurnMs       *int64    `json:"avg_assistant_turn_ms,omitempty"`      // Average assistant turn duration
	AvgUserThinkingMs        *int64    `json:"avg_user_thinking_ms,omitempty"`       // Average user thinking time
	TotalAssistantDurationMs *int64    `json:"total_assistant_duration_ms,omitempty"` // Total assistant turn duration
	TotalUserDurationMs      *int64    `json:"total_user_duration_ms,omitempty"`      // Total user thinking time
	AssistantUtilizationPct  *float64  `json:"assistant_utilization_pct,omitempty"`   // % of time Claude was working (0-100)
}

// AgentStats holds success and error counts for a single agent type.
type AgentStats struct {
	Success int `json:"success"`
	Errors  int `json:"errors"`
}

// SkillStats holds success and error counts for a single skill.
type SkillStats struct {
	Success int `json:"success"`
	Errors  int `json:"errors"`
}

// AgentsAndSkillsCardRecord is the DB record for the combined agents and skills card.
type AgentsAndSkillsCardRecord struct {
	SessionID        string                 `json:"session_id"`
	Version          int                    `json:"version"`
	ComputedAt       time.Time              `json:"computed_at"`
	UpToLine         int64                  `json:"up_to_line"`
	AgentInvocations int                    `json:"agent_invocations"`
	SkillInvocations int                    `json:"skill_invocations"`
	AgentStats       map[string]*AgentStats `json:"agent_stats"` // Per-agent-type success/error counts
	SkillStats       map[string]*SkillStats `json:"skill_stats"` // Per-skill success/error counts
}

// RedactionsCardRecord is the DB record for the redactions card.
type RedactionsCardRecord struct {
	SessionID        string         `json:"session_id"`
	Version          int            `json:"version"`
	ComputedAt       time.Time      `json:"computed_at"`
	UpToLine         int64          `json:"up_to_line"`
	TotalRedactions  int            `json:"total_redactions"`
	RedactionCounts  map[string]int `json:"redaction_counts"` // Type -> count (e.g., "GITHUB_TOKEN" -> 5)
}

// WorkflowRun is a single workflow run's aggregate. It is both the JSONB
// storage shape (in session_card_workflows.runs) and the API wire shape, so
// json tags are snake_case to match the frontend. Runs are stored already
// ordered (by start time), so StartedAt is used only at compute time for
// sorting and is not serialized.
type WorkflowRun struct {
	RunID           string    `json:"run_id"`
	AgentCount      int       `json:"agent_count"`
	InputTokens     int64     `json:"input_tokens"`
	OutputTokens    int64     `json:"output_tokens"`
	CacheCreation   int64     `json:"cache_creation"`
	CacheRead       int64     `json:"cache_read"`
	EstimatedUSD    string    `json:"estimated_usd"` // Decimal as string for precision
	SucceededAgents int       `json:"succeeded_agents"`
	HasJournal      bool      `json:"has_journal"`
	DurationMs      int64     `json:"duration_ms"`
	StartedAt       time.Time `json:"-"` // compute-time ordering only; not persisted/served
}

// WorkflowsCardRecord is the DB record for the workflows card.
type WorkflowsCardRecord struct {
	SessionID  string        `json:"session_id"`
	Version    int           `json:"version"`
	ComputedAt time.Time     `json:"computed_at"`
	UpToLine   int64         `json:"up_to_line"`
	Runs       []WorkflowRun `json:"runs"`
}

// SmartRecapCardRecord is the DB record for the AI-generated smart recap card.
// Unlike other cards, this uses time-based invalidation due to LLM cost.
type SmartRecapCardRecord struct {
	SessionID  string    `json:"session_id"`
	Version    int       `json:"version"`
	ComputedAt time.Time `json:"computed_at"`
	UpToLine   int64     `json:"up_to_line"`

	// LLM-generated content
	Recap                     string          `json:"recap"`
	WentWell                  []AnnotatedItem `json:"went_well"`
	WentBad                   []AnnotatedItem `json:"went_bad"`
	HumanSuggestions          []AnnotatedItem `json:"human_suggestions"`
	EnvironmentSuggestions    []AnnotatedItem `json:"environment_suggestions"`
	DefaultContextSuggestions []AnnotatedItem `json:"default_context_suggestions"`

	// LLM metadata
	ModelUsed        string `json:"model_used"`
	InputTokens      int    `json:"input_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	GenerationTimeMs *int   `json:"generation_time_ms,omitempty"`

	// Race prevention (optimistic lock)
	ComputingStartedAt *time.Time `json:"computing_started_at,omitempty"`
}

// SearchIndexRecord is the DB record for the full-text search index.
// Unlike other cards, this is not a card but a search index with independent freshness tracking.
type SearchIndexRecord struct {
	SessionID       string     `json:"session_id"`
	Version         int        `json:"version"`
	ContentText     string     `json:"content_text"`
	IndexedUpToLine int64      `json:"indexed_up_to_line"`
	RecapIndexedAt  *time.Time `json:"recap_indexed_at,omitempty"`
	MetadataHash    string     `json:"metadata_hash"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Cards aggregates all card data for a session.
type Cards struct {
	Tokens          *TokensCardRecord
	TokensV2        *TokensV2CardRecord // optional; OpenCode only — not part of AllValid
	Session         *SessionCardRecord
	Tools           *ToolsCardRecord
	CodeActivity    *CodeActivityCardRecord
	Conversation    *ConversationCardRecord
	AgentsAndSkills *AgentsAndSkillsCardRecord
	Redactions      *RedactionsCardRecord
	Workflows       *WorkflowsCardRecord

	// Per-card computation errors (graceful degradation)
	CardErrors map[string]string
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

	// Fast mode breakdown (omitted when no fast mode usage)
	FastTurns   *int   `json:"fast_turns,omitempty"`
	FastCostUSD string `json:"fast_cost_usd,omitempty"`
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
	UserTurns                int      `json:"user_turns"`
	AssistantTurns           int      `json:"assistant_turns"`
	AvgAssistantTurnMs       *int64   `json:"avg_assistant_turn_ms,omitempty"`
	AvgUserThinkingMs        *int64   `json:"avg_user_thinking_ms,omitempty"`
	TotalAssistantDurationMs *int64   `json:"total_assistant_duration_ms,omitempty"`
	TotalUserDurationMs      *int64   `json:"total_user_duration_ms,omitempty"`
	AssistantUtilizationPct  *float64 `json:"assistant_utilization_pct,omitempty"`
}

// AgentsAndSkillsCardData is the API response format for the combined agents and skills card.
type AgentsAndSkillsCardData struct {
	AgentInvocations int                    `json:"agent_invocations"`
	SkillInvocations int                    `json:"skill_invocations"`
	AgentStats       map[string]*AgentStats `json:"agent_stats"` // Per-agent-type success/error counts
	SkillStats       map[string]*SkillStats `json:"skill_stats"` // Per-skill success/error counts
}

// RedactionsCardData is the API response format for the redactions card.
type RedactionsCardData struct {
	TotalRedactions int            `json:"total_redactions"`
	RedactionCounts map[string]int `json:"redaction_counts"` // Type -> count
}

// WorkflowsCardData is the API response format for the workflows card.
type WorkflowsCardData struct {
	Runs []WorkflowRun `json:"runs"`
}

// SmartRecapCardData is the API response format for the AI-generated smart recap card.
type SmartRecapCardData struct {
	Recap                     string          `json:"recap"`
	WentWell                  []AnnotatedItem `json:"went_well"`
	WentBad                   []AnnotatedItem `json:"went_bad"`
	HumanSuggestions          []AnnotatedItem `json:"human_suggestions"`
	EnvironmentSuggestions    []AnnotatedItem `json:"environment_suggestions"`
	DefaultContextSuggestions []AnnotatedItem `json:"default_context_suggestions"`
	ComputedAt                string          `json:"computed_at"`
	ModelUsed                 string          `json:"model_used"`
}

// SmartRecapQuotaInfo contains quota information for smart recap generation.
type SmartRecapQuotaInfo struct {
	Used     int  `json:"used"`
	Limit    int  `json:"limit"`
	Exceeded bool `json:"exceeded"`
}

// =============================================================================
// Validation helpers
// =============================================================================

// CardValidator is the interface that all regular card records implement.
// Each card must check both its version constant and line count watermark.
type CardValidator interface {
	IsValid(currentLineCount int64) bool
}

// Compile-time checks that all card types implement CardValidator.
var (
	_ CardValidator = (*TokensCardRecord)(nil)
	_ CardValidator = (*TokensV2CardRecord)(nil)
	_ CardValidator = (*SessionCardRecord)(nil)
	_ CardValidator = (*ToolsCardRecord)(nil)
	_ CardValidator = (*CodeActivityCardRecord)(nil)
	_ CardValidator = (*ConversationCardRecord)(nil)
	_ CardValidator = (*AgentsAndSkillsCardRecord)(nil)
	_ CardValidator = (*RedactionsCardRecord)(nil)
)

// IsValid checks if a tokens card record is valid for the current line count.
func (c *TokensCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == TokensCardVersion && c.UpToLine == currentLineCount
}

// IsValid checks if a tokens_v2 card record is valid for the current line count.
func (c *TokensV2CardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == TokensV2CardVersion && c.UpToLine == currentLineCount
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

// IsValid checks if an agents and skills card record is valid for the current line count.
func (c *AgentsAndSkillsCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == AgentsAndSkillsCardVersion && c.UpToLine == currentLineCount
}

// IsValid checks if a redactions card record is valid for the current line count.
func (c *RedactionsCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == RedactionsCardVersion && c.UpToLine == currentLineCount
}

// IsValid checks if a workflows card record is valid for the current line count.
func (c *WorkflowsCardRecord) IsValid(currentLineCount int64) bool {
	return c != nil && c.Version == WorkflowsCardVersion && c.UpToLine == currentLineCount
}

// HasValidVersion checks if a smart recap card record exists with the correct version.
// Used by API handlers to determine if a cached card can be returned.
func (c *SmartRecapCardRecord) HasValidVersion() bool {
	return c != nil && c.Version == SmartRecapCardVersion
}

// IsUpToDate checks if a smart recap card is up-to-date with the current line count.
// Used by precomputer to determine if regeneration is needed.
func (c *SmartRecapCardRecord) IsUpToDate(currentLineCount int64) bool {
	return c != nil && c.Version == SmartRecapCardVersion && c.UpToLine >= currentLineCount
}

// CanAcquireLock checks if we can acquire the computing lock.
// Returns true if no lock exists or the lock is stale (older than lockTimeoutSeconds).
func (c *SmartRecapCardRecord) CanAcquireLock(lockTimeoutSeconds int) bool {
	if c == nil || c.ComputingStartedAt == nil {
		return true
	}
	// Lock is stale if older than timeout
	return time.Since(*c.ComputingStartedAt).Seconds() >= float64(lockTimeoutSeconds)
}

// AllValid checks if all cards are valid for the current line count.
func (c *Cards) AllValid(currentLineCount int64) bool {
	if c == nil {
		return false
	}
	return c.Tokens.IsValid(currentLineCount) &&
		c.TokensV2.IsValid(currentLineCount) &&
		c.Session.IsValid(currentLineCount) &&
		c.Tools.IsValid(currentLineCount) &&
		c.CodeActivity.IsValid(currentLineCount) &&
		c.Conversation.IsValid(currentLineCount) &&
		c.AgentsAndSkills.IsValid(currentLineCount) &&
		c.Redactions.IsValid(currentLineCount) &&
		c.Workflows.IsValid(currentLineCount)
}
