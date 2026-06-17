package analytics

import (
	"context"

	"github.com/shopspring/decimal"
)

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
)

// ComputeFromCursorRollout maps a parsed Cursor session (main transcript only
// in v1 — subagents are deferred) onto the canonical ComputeResult shape.
//
// Cursor synced JSONL carries no timestamps, tokens, model, or cost, so the
// timing-derived cards (duration, conversation turn timings) and the
// token/cost cards degrade gracefully: tokens_v2 is always written with an
// empty by_provider tree (no invented dollars — real cost is follow-up 59m1),
// and DurationMs stays nil. The structure-derived cards (session counts,
// tools, code activity, agents, conversation turn counts, search text) are
// computed from message + tool_use structure.
func ComputeFromCursorRollout(ctx context.Context, messages []*CursorMessage) *ComputeResult {
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

	if len(messages) == 0 {
		return result
	}

	computeCursorSession(result, messages)
	computeCursorConversation(result, messages)
	computeCursorTools(result, messages)
	computeCursorCodeActivity(result, messages)
	computeCursorAgents(result, messages)

	return result
}

// computeCursorSession derives message-count stats. Cursor lines carry no
// timestamps, so DurationMs is left nil and timing is unknowable.
func computeCursorSession(out *ComputeResult, messages []*CursorMessage) {
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
