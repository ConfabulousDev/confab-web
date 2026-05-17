package analytics

import "github.com/ConfabulousDev/confab-web/internal/codex"

// computeCodexTools fills the Tools card: TotalToolCalls, per-tool
// success/error breakdown, ToolErrorCount. Orphan outputs (synthetic
// "<unknown>" tools emitted by the parser) are counted as-is.
func computeCodexTools(out *ComputeResult, r *codex.ParsedRollout) {
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
