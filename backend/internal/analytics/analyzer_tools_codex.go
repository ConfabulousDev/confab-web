package analytics

import "github.com/ConfabulousDev/confab-web/internal/codex"

// computeCodexTools fills the Tools card: TotalToolCalls, per-tool
// success/error breakdown, ToolErrorCount. Synthetic "<unknown>" tools
// (parser-emitted orphan outputs with no matching function_call) are
// dropped — the anomaly is recorded as a ParsedRollout.ValidationError at
// parse time. CF-438.
func computeCodexTools(out *ComputeResult, r *codex.ParsedRollout) {
	for _, turn := range r.Turns {
		for _, tc := range turn.ToolCalls {
			name := tc.Name
			if name == "" || name == "<unknown>" {
				continue
			}
			out.TotalToolCalls++
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
