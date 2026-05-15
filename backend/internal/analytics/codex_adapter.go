package analytics

import (
	"bufio"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/shopspring/decimal"
)

// ComputeFromCodexRollout maps a parsed Codex rollout onto the same
// ComputeResult shape produced by ComputeStreaming for Claude transcripts.
//
// Per-card mapping decisions (locked at interview time, see /tmp/plan-CF-350.md):
//   - Tokens: cached is a subset of input (OpenAI semantics) — subtract before
//     applying the uncached rate. Reasoning tokens add to output (same rate).
//   - Session: full population (TotalMessages, breakdowns, ModelsUsed, Duration).
//     Compactions all classified as "auto" (Codex doesn't distinguish auto vs manual).
//   - Tools: standard success/error breakdown; orphan "<unknown>" tools counted.
//   - Code activity: apply_patch envelopes drive FilesModified/LinesAdded/Removed
//     and LanguageBreakdown. FilesRead stays 0 (Codex has no Read tool).
//   - Conversation: AssistantTurns = count of user-prompt-triggered sequences,
//     not raw Codex task_started→task_complete cycles.
//   - Agents/skills: zero (no Codex equivalent).
//   - Redactions: walk all parser-surfaced strings.
func ComputeFromCodexRollout(rollout *codex.ParsedRollout) *ComputeResult {
	if rollout == nil {
		return &ComputeResult{}
	}

	result := &ComputeResult{
		ToolStats:         make(map[string]*ToolStats),
		LanguageBreakdown: make(map[string]int),
		AgentStats:        make(map[string]*AgentStats),
		SkillStats:        make(map[string]*SkillStats),
		RedactionCounts:   make(map[string]int),
	}

	applyCodexTokens(result, rollout)
	applyCodexSession(result, rollout)
	applyCodexTools(result, rollout)
	applyCodexCodeActivityWithLangs(result, rollout)
	applyCodexConversation(result, rollout)
	applyCodexRedactions(result, rollout)

	return result
}

// ----------------------------------------------------------------------------
// Tokens
// ----------------------------------------------------------------------------

// applyCodexTokens fills InputTokens/OutputTokens/cache fields and cost.
// OpenAI semantics: CachedInputTokens is a subset of InputTokens, so we
// subtract it before billing the uncached portion at the full input rate.
// Reasoning tokens are billed as output (same rate), so they fold in there.
// CacheCreationTokens stays 0 — OpenAI doesn't charge for cache writes.
func applyCodexTokens(out *ComputeResult, r *codex.ParsedRollout) {
	tu := r.TokenUsage
	uncached := tu.InputTokens - tu.CachedInputTokens
	if uncached < 0 {
		uncached = 0
	}
	out.InputTokens = uncached
	out.CacheReadTokens = tu.CachedInputTokens
	out.CacheCreationTokens = 0
	out.OutputTokens = tu.OutputTokens + tu.ReasoningOutputTokens

	pricing := GetPricing(r.Model)
	out.EstimatedCostUSD = CalculateCost(
		pricing,
		out.InputTokens,
		out.OutputTokens,
		out.CacheCreationTokens,
		out.CacheReadTokens,
	)
	// Codex doesn't expose a "fast mode" toggle.
	out.FastTurns = 0
	out.FastCostUSD = decimal.Zero
}

// ----------------------------------------------------------------------------
// Session
// ----------------------------------------------------------------------------

func applyCodexSession(out *ComputeResult, r *codex.ParsedRollout) {
	models := map[string]struct{}{}
	if r.Model != "" {
		models[r.Model] = struct{}{}
	}

	var firstStart, lastComplete *time.Time
	for _, turn := range r.Turns {
		if turn.Model != "" {
			models[turn.Model] = struct{}{}
		}

		out.UserMessages += len(turn.UserMessages)
		out.AssistantMessages += len(turn.AssistantMessages)
		out.HumanPrompts += len(turn.UserMessages) // parser already stripped env-context-only
		for _, m := range turn.AssistantMessages {
			if m.Text != "" {
				out.TextResponses++
			}
		}
		for _, tc := range turn.ToolCalls {
			out.ToolCalls++
			if tc.Output != "" {
				out.ToolResults++
			}
		}
		out.ThinkingBlocks += turn.ReasoningCount

		if turn.StartedAt != nil && (firstStart == nil || turn.StartedAt.Before(*firstStart)) {
			firstStart = turn.StartedAt
		}
		if turn.CompletedAt != nil && (lastComplete == nil || turn.CompletedAt.After(*lastComplete)) {
			lastComplete = turn.CompletedAt
		}
	}

	// TotalMessages mirrors Claude's count semantics: user + assistant + tool
	// call lines (request + output each count as one).
	out.TotalMessages = out.UserMessages + out.AssistantMessages + (out.ToolCalls * 2)

	out.ModelsUsed = sortedKeys(models)

	if firstStart != nil && lastComplete != nil {
		if d := lastComplete.Sub(*firstStart).Milliseconds(); d >= 0 {
			out.DurationMs = &d
		}
	}

	// Codex doesn't distinguish auto vs manual compaction — all are "auto".
	out.CompactionAuto = len(r.Compactions)
	out.CompactionManual = 0
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ----------------------------------------------------------------------------
// Tools
// ----------------------------------------------------------------------------

func applyCodexTools(out *ComputeResult, r *codex.ParsedRollout) {
	for _, turn := range r.Turns {
		for _, tc := range turn.ToolCalls {
			out.TotalToolCalls++
			name := tc.Name
			if name == "" {
				name = "<unknown>"
			}
			if out.ToolStats[name] == nil {
				out.ToolStats[name] = &ToolStats{}
			}
			if tc.Status == "failed" {
				out.ToolStats[name].Errors++
				out.ToolErrorCount++
			} else {
				out.ToolStats[name].Success++
			}
		}
	}
}

// ----------------------------------------------------------------------------
// Code activity
// ----------------------------------------------------------------------------

// applyCodexCodeActivityWithLangs inspects apply_patch tool calls (the Codex
// equivalent of Edit/Write). Codex doesn't have a Read tool, so FilesRead
// stays at zero — intentional, not an omission.
func applyCodexCodeActivityWithLangs(out *ComputeResult, r *codex.ParsedRollout) {
	for _, turn := range r.Turns {
		for _, tc := range turn.ToolCalls {
			if tc.Name != "apply_patch" {
				continue
			}
			files, added, removed := parseApplyPatch(tc.Arguments, out.LanguageBreakdown)
			out.FilesModified += files
			out.LinesAdded += added
			out.LinesRemoved += removed
		}
	}
}

// parseApplyPatch parses a Codex apply_patch envelope, returning the number
// of files touched (any of Add/Update/Delete) and the cumulative +/- line
// counts. If langs is non-nil it's updated with file-extension language counts.
func parseApplyPatch(envelope string, langs map[string]int) (files, added, removed int) {
	scanner := bufio.NewScanner(strings.NewReader(envelope))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	inFile := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "*** Add File: "),
			strings.HasPrefix(line, "*** Update File: "),
			strings.HasPrefix(line, "*** Delete File: "):
			files++
			inFile = true
			if langs != nil {
				path := line[strings.Index(line, ": ")+2:]
				if lang := languageFromPath(path); lang != "" {
					langs[lang]++
				}
			}
		case strings.HasPrefix(line, "*** End Patch"),
			strings.HasPrefix(line, "*** Begin Patch"):
			inFile = false
		case inFile && strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			added++
		case inFile && strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			removed++
		}
	}
	return files, added, removed
}

// languageFromPath returns a language label from a file extension, mirroring
// the conventions used elsewhere in analytics (e.g. analyzer_code_activity).
// Returns "" for unrecognized extensions.
func languageFromPath(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "go":
		return "go"
	case "py":
		return "python"
	case "ts", "tsx":
		return "typescript"
	case "js", "jsx":
		return "javascript"
	case "rs":
		return "rust"
	case "java":
		return "java"
	case "rb":
		return "ruby"
	case "cs":
		return "csharp"
	case "cpp", "cc", "cxx", "hpp", "h":
		return "cpp"
	case "c":
		return "c"
	case "sh", "bash", "zsh":
		return "shell"
	case "md", "markdown":
		return "markdown"
	case "yml", "yaml":
		return "yaml"
	case "json":
		return "json"
	case "sql":
		return "sql"
	case "html":
		return "html"
	case "css", "scss":
		return "css"
	}
	return ""
}

// ----------------------------------------------------------------------------
// Conversation
// ----------------------------------------------------------------------------

// applyCodexConversation counts user-prompt-triggered sequences as
// AssistantTurns, not raw Codex task_started→task_complete cycles. Within a
// single Codex turn the user may type multiple prompts mid-stream; each one
// that triggers ≥1 assistant message before the next prompt counts as one
// AssistantTurn (closer to Claude's semantics).
func applyCodexConversation(out *ComputeResult, r *codex.ParsedRollout) {
	for _, turn := range r.Turns {
		out.UserTurns += len(turn.UserMessages)

		for i, prompt := range turn.UserMessages {
			// Window = [prompt.Timestamp, nextPrompt.Timestamp). For the last
			// prompt the window extends to the end of the turn (no upper bound).
			hasNext := i+1 < len(turn.UserMessages)
			for _, asst := range turn.AssistantMessages {
				if asst.Timestamp.Before(prompt.Timestamp) {
					continue
				}
				if hasNext && !asst.Timestamp.Before(turn.UserMessages[i+1].Timestamp) {
					continue
				}
				out.AssistantTurns++
				break
			}
		}
	}
}

// ----------------------------------------------------------------------------
// Redactions
// ----------------------------------------------------------------------------

// applyCodexRedactions walks every parser-surfaced string for [REDACTED:TYPE]
// markers. We use the same redactionPattern as RedactionsAnalyzer so the count
// semantics match Claude exactly (including the TYPE-placeholder exclusion).
func applyCodexRedactions(out *ComputeResult, r *codex.ParsedRollout) {
	count := func(s string) {
		matches := redactionPattern.FindAllStringSubmatch(s, -1)
		for _, m := range matches {
			if len(m) < 2 || m[1] == "TYPE" {
				continue
			}
			out.RedactionCounts[m[1]]++
			out.TotalRedactions++
		}
	}

	// Session-level strings.
	count(r.CWD)
	for _, v := range r.GitInfo {
		if s, ok := v.(string); ok {
			count(s)
		}
	}
	for _, turn := range r.Turns {
		for _, m := range turn.UserMessages {
			count(m.Text)
		}
		for _, m := range turn.AssistantMessages {
			count(m.Text)
		}
		for _, tc := range turn.ToolCalls {
			count(tc.Arguments)
			count(tc.Output)
		}
	}
}
