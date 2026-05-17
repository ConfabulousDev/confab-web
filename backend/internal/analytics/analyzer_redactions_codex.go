package analytics

import "github.com/ConfabulousDev/confab-web/internal/codex"

// computeCodexRedactions walks every parser-surfaced string for [REDACTED:TYPE]
// markers. Uses the same redactionPattern as analyzer_redactions_claude.go so
// the count semantics match Claude exactly (including the TYPE-placeholder
// exclusion).
//
// NOTE (CF-445): This relies on the Confab CLI applying redaction to Codex
// rollouts at upload time. If the CLI doesn't redact, this card silently
// shows zero. Verify the CLI invariant in the follow-up ticket.
func computeCodexRedactions(out *ComputeResult, r *codex.ParsedRollout) {
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
