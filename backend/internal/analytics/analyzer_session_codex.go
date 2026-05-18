package analytics

import (
	"sort"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// computeCodexSession fills the Session card across all rollouts: per-turn
// counts sum, ModelsUsed unions, DurationMs spans the min-start/max-complete
// across all rollouts, compactions sum.
func computeCodexSession(out *ComputeResult, rollouts []*codex.ParsedRollout) {
	models := map[string]struct{}{}
	var firstStart, lastComplete *time.Time
	totalCompactions := 0

	for _, r := range rollouts {
		if r == nil {
			continue
		}
		if r.Model != "" {
			models[r.Model] = struct{}{}
		}
		for _, turn := range r.Turns {
			if turn.Model != "" {
				models[turn.Model] = struct{}{}
			}

			out.UserMessages += len(turn.UserMessages)
			out.AssistantMessages += len(turn.AssistantMessages)
			// HumanPrompts == UserMessages for Codex: the parser already
			// separates tool outputs (function_call_output / custom_tool_call_output
			// land on turn.ToolCalls, not turn.UserMessages) and strips
			// env-context-only messages. Claude needs an IsHumanMessage filter at
			// compute time because its protocol overloads the "user" role to also
			// carry tool_result blocks; Codex's wire format separates them. See
			// analyzer_session_codex_test.go for the regression guard (CF-437).
			out.HumanPrompts += len(turn.UserMessages)
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
		totalCompactions += len(r.Compactions)
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
	out.CompactionAuto = totalCompactions
	out.CompactionManual = 0
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
