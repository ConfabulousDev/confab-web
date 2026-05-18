// Package codex parses OpenAI Codex rollout JSONL files into a normalized
// representation suitable for analytics, smart recap, and search indexing.
//
// The on-disk format is the Codex CLI's rollout — one JSON object per line —
// with five top-level types: session_meta, turn_context, response_item,
// event_msg, and compacted. ParseRollout consumes a Reader and returns a
// ParsedRollout aggregating turns, token usage, model info, and compactions.
//
// The parser is permissive: unknown line types are skipped silently (forward
// compatibility), JSON-decode failures are recorded as ValidationErrors but do
// not abort the parse, and files that end mid-turn leave the last turn open.
package codex

import "time"

// ParsedRollout is the full normalized output of ParseRollout.
type ParsedRollout struct {
	Turns            []Turn
	TokenUsage       TokenUsage
	Model            string                 // from session_meta.model (older CLIs) or the first turn_context.model
	ModelProvider    string                 // from session_meta (e.g. "openai")
	CWD              string                 // from session_meta
	CLIVersion       string                 // from session_meta.cli_version (e.g. "0.130.0")
	GitInfo          map[string]interface{} // from session_meta (passthrough)
	Compactions      []CompactionEvent
	ValidationErrors []ValidationError
	TotalLines       int

	// Subagent is non-nil for rollouts that were spawned as a subagent
	// (session_meta.source.sub_agent.thread_spawn). Nil for top-level / CLI
	// sessions and for non-thread-spawn subagent variants (review, compact,
	// memory_consolidation).
	Subagent *SubagentSource

	// SkillInvocations records every <skill>...</skill> user-message block.
	// Order is rollout order.
	SkillInvocations []SkillInvocation

	// SubagentSpawns records every spawn_agent function_call observed on the
	// parent side, progressively filled in as the matching function_call_output
	// and wait_agent results arrive. Order is rollout order.
	SubagentSpawns []SubagentSpawn

	// AvailableSkills is the catalog enumerated in the first developer-role
	// <skills_instructions> block. Ephemeral — not persisted in any card.
	// Subsequent <skills_instructions> blocks (rare) are ignored.
	AvailableSkills []SkillAvailable
}

// SubagentSource describes how a child rollout was spawned by a parent.
// Mapped from session_meta.source.sub_agent.thread_spawn.
type SubagentSource struct {
	ParentThreadID string // parent rollout's thread ID
	Depth          int    // i32 in Rust; always ≥1 when set
	AgentPath      string // optional, e.g. "root/researcher"
	AgentNickname  string // optional, e.g. "Nash" — auto-generated scientist surname
	AgentRole      string // optional, e.g. "default", "explorer"; accepts legacy alias "agent_type"
}

// SkillInvocation is one <skill>...</skill> user-message block.
//
// Codex injects a synthetic user-role message whose input_text wraps the entire
// SKILL.md (frontmatter + body) between <skill> tags. The parser extracts the
// metadata fields here and strips the wrapper from the surfaced Message.Text.
type SkillInvocation struct {
	Name      string    // from <name> tag
	Path      string    // from <path> tag (absolute file path to SKILL.md)
	Timestamp time.Time // timestamp of the response_item carrying the block
}

// SubagentSpawn is one parent-side spawn_agent function_call paired with its
// matching wait_agent status (when available). spawn_agent / wait_agent are
// excluded from the generic Turn.ToolCalls list — they only appear here.
//
// Built progressively:
//   - spawn_agent function_call fills CallID, AgentType, Message, ReasoningEffort,
//     ForkContext, Timestamp.
//   - spawn_agent function_call_output fills ResultAgentID, ResultNickname.
//   - wait_agent function_call_output (or, fallback, a <subagent_notification>
//     user message) fills Completed, CompletionStatus, CompletionText.
//
// Orphan spawns (no matching wait_agent before the rollout ends) keep
// Completed=false and bucket as errors in the AgentsAndSkills card.
type SubagentSpawn struct {
	CallID           string    // function_call.call_id of the spawn_agent
	AgentType        string    // arguments.agent_type (== child's agent_role)
	Message          string    // arguments.message (the prompt sent to child)
	ReasoningEffort  string    // optional
	ForkContext      bool      // optional; carried for future drill-down, not bucketed on
	ResultAgentID    string    // spawn_agent output.agent_id (== child thread id)
	ResultNickname   string    // spawn_agent output.nickname
	Completed        bool      // true iff wait_agent status == "completed"
	CompletionStatus string    // raw status key from wait_agent: "completed" | "failed" | "" (orphan)
	CompletionText   string    // body of the status, truncated at 1000 chars
	Timestamp        time.Time // when spawn_agent fired
}

// SkillAvailable is one entry in the <skills_instructions> catalog. Ephemeral.
type SkillAvailable struct {
	Name        string
	Description string
	Path        string
}

// Turn is one task_started → task_complete cycle (or an implicit/open turn
// when those markers are missing).
type Turn struct {
	TurnID             string
	StartedAt          *time.Time // from task_started.started_at (unix seconds)
	CompletedAt        *time.Time // from task_complete.completed_at
	DurationMs         *int64     // task_complete.duration_ms
	TimeToFirstTokenMs *int64     // task_complete.time_to_first_token_ms
	Model              string     // task_started.model or turn_context.model; falls back to ParsedRollout.Model
	UserMessages       []Message
	AssistantMessages  []Message
	ToolCalls          []ToolCall
	ReasoningCount     int // count of reasoning items (encrypted or otherwise)
}

// Message is one user or assistant message from response_item.message.
type Message struct {
	Role      string    // "user" | "assistant"
	Text      string    // concatenated input_text/output_text blocks, joined with "\n"
	Phase     string    // assistant: "commentary" | "final"; empty for user
	Timestamp time.Time
}

// ToolCall is a function_call / custom_tool_call paired with its output.
type ToolCall struct {
	CallID     string
	Name       string // function name; "<unknown>" if output arrives without a preceding call
	Arguments  string // raw arguments JSON (function_call) or input string (custom_tool_call)
	Output     string // function_call_output.output (exec_command preamble stripped)
	Status     string // "pending" | "completed" | "failed"
	ExitCode   *int   // exec_command only
	WallTimeMs *int   // exec_command only
	Timestamp  time.Time
}

// TokenUsage is the final running token totals from the last non-null
// event_msg.token_count.info.total_token_usage.
//
// CachedInputTokens is a subset of InputTokens (OpenAI's API semantics — not
// a separate count). Callers that bill cached tokens at a different rate must
// subtract cached from input before applying the uncached rate.
type TokenUsage struct {
	InputTokens           int64
	CachedInputTokens     int64
	OutputTokens          int64
	ReasoningOutputTokens int64
	TotalTokens           int64
}

// CompactionEvent records one `compacted` line in the rollout.
type CompactionEvent struct {
	Timestamp        time.Time
	ReplacementCount int // len(replacement_history)
}

// ValidationError is a per-line failure during parsing. The line continues to
// count toward TotalLines but is not otherwise processed.
type ValidationError struct {
	Line   int    // 1-based
	Type   string // top-level type, if extractable
	Reason string
}
