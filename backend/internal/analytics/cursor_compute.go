package analytics

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// CursorSessionBounds carries the session-level timing anchors used to estimate
// a Cursor session's duration. Cursor JSONL lines have no per-line timestamps
// (4r41), so the only timing signal is the session window:
//
//	start = created_at ?? first_seen      (created_at refines first_seen at ingest)
//	end   = last_message_at ?? last_sync_at
//
// All fields are optional; cursorSessionWindow resolves the precedence and
// computeCursorSession derives DurationMs only when a strictly-positive window
// is available (no invented or non-positive spans — Cursor's honesty rule).
type CursorSessionBounds struct {
	CreatedAt     *time.Time // start anchor, preferred (meta.json createdAtMs)
	FirstSeen     *time.Time // start anchor fallback (session init time)
	LastMessageAt *time.Time // end anchor, preferred (meta.json updatedAtMs)
	LastSyncAt    *time.Time // end anchor fallback (last chunk upload time)

	// Model is the per-session model name recovered from the cursor_session_meta
	// sidecar (zsr6). Cursor JSONL carries no per-line model, so this is the only
	// model signal. Empty when the CLI never sent metadata.model — compute then
	// leaves models_used empty rather than inventing a model.
	Model string
}

// cursorSessionWindow resolves the start/end anchors per the precedence above:
// start = created_at ?? first_seen; end = last_message_at ?? last_sync_at.
// Either result may be nil when no anchor of that kind is present.
func cursorSessionWindow(b CursorSessionBounds) (start, end *time.Time) {
	start = b.CreatedAt
	if start == nil {
		start = b.FirstSeen
	}
	end = b.LastMessageAt
	if end == nil {
		end = b.LastSyncAt
	}
	return start, end
}

// Cursor tool-name constants. These are Cursor's own tool names — they do NOT
// overlap with Claude's (Cursor's edit tool is StrReplace, not Edit/MultiEdit),
// so we use Cursor-specific string constants rather than reusing Claude's.
const (
	cursorToolRead           = "Read"
	cursorToolWrite          = "Write"
	cursorToolStrReplace     = "StrReplace"
	cursorToolDelete         = "Delete"
	cursorToolGlob           = "Glob"
	cursorToolGrep           = "Grep"
	cursorToolSemanticSearch = "SemanticSearch"
	cursorToolTask           = "Task"

	// cursorToolUpdateCurrentStep is a subagent-only progress marker (input keys
	// current_step / final_summary / completed_subtitle), not a real tool call.
	// It is classified-and-skipped everywhere it appears (decision D2, wc9t) so it
	// neither inflates tool stats nor surfaces an unfamiliar Cursor-only name.
	cursorToolUpdateCurrentStep = "UpdateCurrentStep"
)

// ComputeFromCursorRollout maps a parsed Cursor session — the main transcript
// plus any subagent rollouts uploaded as file_type='agent' (wc9t) — onto the
// canonical ComputeResult shape. The slice convention is [main, ...subagents].
//
// Cursor synced JSONL carries no per-line timestamps, tokens, model, or cost.
// The token/cost cards degrade gracefully: tokens_v2 is always written with an
// empty by_provider tree (no invented dollars — real cost is follow-up 59m1).
//
// Per-card aggregation (mirrors OpenCode CF-539's asymmetric merge):
//   - Session counts / DurationMs / models_used: MAIN thread only — subagents
//     nest within the main session window; the bounds carry the main-thread
//     window (start = created_at ?? first_seen; end = last_message_at ??
//     last_sync_at), so DurationMs stays nil when that window is absent or
//     non-positive.
//   - Conversation turn counts: MAIN thread only — user-perceived turn structure
//     does not include subagent reasoning.
//   - Tools / code activity / agents: merged across ALL rollouts (each analyzer
//     accumulates via +=, so per-rollout dispatch composes the cross-rollout
//     total).
//
// Per-row estimated timestamps for transcript display are NOT computed here:
// they are interpolated frontend-side (ce79) from these same session bounds, so
// there is only one estimator and stored JSONL is never mutated.
func ComputeFromCursorRollout(ctx context.Context, rollouts [][]*CursorMessage, bounds CursorSessionBounds) *ComputeResult {
	result := &ComputeResult{
		ToolStats:         make(map[string]*ToolStats),
		LanguageBreakdown: make(map[string]int),
		AgentStats:        make(map[string]*AgentStats),
		SkillStats:        make(map[string]*SkillStats),
		RedactionCounts:   make(map[string]int),
	}

	// tokens_v2 is always written. Cursor JSONL has no usage data, so the tree
	// is empty and total cost is zero.
	result.TokensV2 = &TokensV2Data{
		TotalCostUSD: decimal.Zero.String(),
		ByProvider:   map[string]TokensV2Provider{},
	}
	result.EstimatedCostUSD = decimal.Zero
	result.FastCostUSD = decimal.Zero

	if len(rollouts) == 0 || len(rollouts[0]) == 0 {
		return result
	}

	// Session counts, DurationMs, and conversation turns reflect the main thread
	// only (rollouts[0]). Subagents nest within the main window and never widen
	// the user-perceived conversation.
	main := rollouts[0]
	computeCursorSession(result, main, bounds)
	computeCursorConversation(result, main)

	// Tools / code activity / agents merge across every rollout — each analyzer
	// accumulates via +=, so per-rollout dispatch composes the cross-rollout total.
	for _, messages := range rollouts {
		if len(messages) == 0 {
			continue
		}
		computeCursorTools(result, messages)
		computeCursorCodeActivity(result, messages)
		computeCursorAgents(result, messages)
	}

	return result
}

// computeCursorSession derives message-count stats and estimates DurationMs
// from the session window. Cursor lines carry no per-line timestamps, so
// duration is the span between the session bounds (start = created_at ??
// first_seen; end = last_message_at ?? last_sync_at). It is left nil when either
// anchor is missing or the window is non-positive (end <= start) — Cursor never
// invents a zero or negative span.
func computeCursorSession(out *ComputeResult, messages []*CursorMessage, bounds CursorSessionBounds) {
	// Cursor JSONL carries no per-line model. The model name (when known) comes
	// from the cursor_session_meta sidecar via bounds.Model (zsr6); when present
	// it is the session's sole model. Initialize to a non-nil empty slice so the
	// session card marshals models_used as [] rather than null when no model was
	// recovered — the frontend's required SessionCardDataSchema.models_used
	// rejects null (y0kc). Never invent a model: leave [] when bounds.Model is "".
	out.ModelsUsed = []string{}
	if bounds.Model != "" {
		out.ModelsUsed = []string{bounds.Model}
	}
	if start, end := cursorSessionWindow(bounds); start != nil && end != nil {
		if d := end.Sub(*start).Milliseconds(); d > 0 {
			out.DurationMs = &d
		}
	}
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			out.UserMessages++
			out.HumanPrompts++
		case "assistant":
			out.AssistantMessages++
			hasText := false
			for _, b := range msg.Content {
				switch b.Type {
				case "text":
					hasText = true
				case "tool_use":
					out.ToolCalls++
				}
			}
			if hasText {
				out.TextResponses++
			}
		}
	}
	// Cursor records no tool_result blocks, so ToolResults stays 0. Mirror the
	// other providers' TotalMessages composition (user + assistant + tool I/O);
	// tool results are absent, so each tool call contributes only its call.
	out.TotalMessages = out.UserMessages + out.AssistantMessages + out.ToolCalls
}

// computeCursorConversation counts user/assistant turns. Without timestamps the
// timing-derived conversation metrics (avg turn ms, utilization) are left nil;
// only the turn counts are populated.
func computeCursorConversation(out *ComputeResult, messages []*CursorMessage) {
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			out.UserTurns++
		case "assistant":
			out.AssistantTurns++
		}
	}
}

// computeCursorTools counts each tool_use block once under its name. Cursor
// records no tool outputs/errors, so every call is recorded as a success.
func computeCursorTools(out *ComputeResult, messages []*CursorMessage) {
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, b := range msg.Content {
			if b.Type != "tool_use" || b.Name == "" {
				continue
			}
			// UpdateCurrentStep is a subagent progress marker, not a tool (D2).
			if b.Name == cursorToolUpdateCurrentStep {
				continue
			}
			out.TotalToolCalls++
			if out.ToolStats[b.Name] == nil {
				out.ToolStats[b.Name] = &ToolStats{}
			}
			out.ToolStats[b.Name].Success++
		}
	}
}

// computeCursorCodeActivity classifies file/search tools per the corrected gevp
// mapping. The file-path field is `path` (NOT `file_path`). Searches are
// Grep/Glob/SemanticSearch; WebSearch is a WEB search and is excluded (Codex
// precedent). Cursor records no tool outputs, so line counts come from the tool
// inputs (Write contents, StrReplace old/new strings).
func computeCursorCodeActivity(out *ComputeResult, messages []*CursorMessage) {
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, b := range msg.Content {
			if b.Type != "tool_use" {
				continue
			}
			switch b.Name {
			case cursorToolRead:
				if fp := b.stringInput("path"); fp != "" {
					out.FilesRead++
					recordCursorLanguage(out, fp)
				}
			case cursorToolWrite:
				if fp := b.stringInput("path"); fp != "" {
					out.FilesModified++
					recordCursorLanguage(out, fp)
					out.LinesAdded += countLines(b.stringInput("contents"))
				}
			case cursorToolStrReplace:
				if fp := b.stringInput("path"); fp != "" {
					out.FilesModified++
					recordCursorLanguage(out, fp)
					out.LinesRemoved += countLines(b.stringInput("old_string"))
					out.LinesAdded += countLines(b.stringInput("new_string"))
				}
			case cursorToolDelete:
				if fp := b.stringInput("path"); fp != "" {
					out.FilesModified++
					recordCursorLanguage(out, fp)
				}
			case cursorToolGrep, cursorToolGlob, cursorToolSemanticSearch:
				out.SearchCount++
			}
		}
	}
}

func recordCursorLanguage(out *ComputeResult, path string) {
	if lang := languageFromPath(path); lang != "" {
		out.LanguageBreakdown[lang]++
	}
}

// computeCursorAgents counts Task invocations on the main thread and buckets
// them by the subagent_type field of the Task input (decision #3). v1 counts
// Task invocations, not subagent-file presence.
func computeCursorAgents(out *ComputeResult, messages []*CursorMessage) {
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, b := range msg.Content {
			if b.Type != "tool_use" || b.Name != cursorToolTask {
				continue
			}
			name := b.stringInput("subagent_type")
			if name == "" {
				name = "unknown"
			}
			out.TotalAgentInvocations++
			if out.AgentStats[name] == nil {
				out.AgentStats[name] = &AgentStats{}
			}
			out.AgentStats[name].Success++
		}
	}
}
