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
	Model            string                 // from session_meta
	ModelProvider    string                 // from session_meta (e.g. "openai")
	CWD              string                 // from session_meta
	GitInfo          map[string]interface{} // from session_meta (passthrough)
	Compactions      []CompactionEvent
	ValidationErrors []ValidationError
	TotalLines       int
}

// Turn is one task_started → task_complete cycle (or an implicit/open turn
// when those markers are missing).
type Turn struct {
	TurnID             string
	StartedAt          *time.Time // from task_started.started_at (unix seconds)
	CompletedAt        *time.Time // from task_complete.completed_at
	DurationMs         *int64     // task_complete.duration_ms
	TimeToFirstTokenMs *int64     // task_complete.time_to_first_token_ms
	Model              string     // task_started.model; falls back to session_meta.Model
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
