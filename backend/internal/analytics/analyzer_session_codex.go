package analytics

import (
	"sort"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// computeCodexSession fills the Session card fields: message counts,
// breakdowns, ModelsUsed, DurationMs, and compactions.
func computeCodexSession(out *ComputeResult, r *codex.ParsedRollout) {
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

// sortedKeys returns the keys of m in sorted order. Shared helper for
// stable ModelsUsed output.
func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
