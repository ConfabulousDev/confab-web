package analytics

import "testing"

// TestTokensAnalyzer_IncludesWorkflowAgentTokens is the CF-534 acceptance check:
// a workflow session's headline Tokens total must include its subagent cost.
// Workflow subagent files (subagents/workflows/<runId>/agent-<id>.jsonl) classify
// as agent files (ExtractAgentID on the nested path, locked by CF-532's
// precompute_test), so they flow into fc.Agents and TokensAnalyzer sums them with
// the main transcript. This test guards that end-to-end accumulation.
func TestTokensAnalyzer_IncludesWorkflowAgentTokens(t *testing.T) {
	mainJSONL := makeAssistantMessageFull("m1", "2025-01-01T00:00:00Z", "claude-sonnet-4-20241022", 100, 50, 0, 0, []map[string]interface{}{makeTextBlock("main")}) + "\n"
	// A workflow subagent transcript (keyed by its extracted agent id).
	workflowAgentJSONL := makeAssistantMessageFull("w1", "2025-01-01T00:00:05Z", "claude-sonnet-4-20241022", 200, 100, 0, 0, []map[string]interface{}{makeTextBlock("agent")}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJSONL), map[string][]byte{
		"abc123": []byte(workflowAgentJSONL),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Main (100/50) + workflow agent (200/100).
	if result.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300 (main 100 + workflow agent 200)", result.InputTokens)
	}
	if result.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150 (main 50 + workflow agent 100)", result.OutputTokens)
	}

	// The agent's cost must be folded into the headline total: dropping the
	// agent file would lower the cost, so it must exceed a main-only computation.
	mainOnly, err := NewFileCollection([]byte(mainJSONL))
	if err != nil {
		t.Fatalf("NewFileCollection: %v", err)
	}
	mainOnlyResult, err := (&TokensAnalyzer{}).Analyze(mainOnly)
	if err != nil {
		t.Fatalf("Analyze main-only: %v", err)
	}
	if !result.EstimatedCostUSD.GreaterThan(mainOnlyResult.EstimatedCostUSD) {
		t.Errorf("EstimatedCostUSD %s not greater than main-only %s; workflow agent cost was not included",
			result.EstimatedCostUSD, mainOnlyResult.EstimatedCostUSD)
	}
}
