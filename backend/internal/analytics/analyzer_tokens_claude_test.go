package analytics

import (
	"log/slog"
	"strings"
	"testing"
)

// filelessSubagentFixture builds a main transcript (sonnet) that spawns one Task
// sub-agent whose own transcript was NOT synced — its token usage arrives only
// via the toolUseResult.usage on the main line. No agent file is provided, so the
// Finalize fallback handles it.
func filelessSubagentFixture(agentInput, agentOutput int64) (*FileCollection, error) {
	mainJSONL := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Task", map[string]interface{}{"subagent_type": "Explore"}),
	}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "Done", false),
		}, map[string]interface{}{
			"agentId": "agent1",
			"usage":   map[string]interface{}{"input_tokens": float64(agentInput), "output_tokens": float64(agentOutput)},
		}) + "\n"
	return NewFileCollection([]byte(mainJSONL)) // no agent files → file-less path
}

// TestTokensAnalyzer_FilelessSubagentPricedAtMainModel is the CF-546 cost-bug
// contract: a file-less sub-agent's tokens must be priced at the MAIN session
// model (sonnet here), not at $0. Total cost must equal main-group cost plus the
// sub-agent's usage costed at sonnet.
func TestTokensAnalyzer_FilelessSubagentPricedAtMainModel(t *testing.T) {
	const agentInput, agentOutput = int64(1_000_000), int64(0)
	fc, err := filelessSubagentFixture(agentInput, agentOutput)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	sonnet, _ := LookupPricing("claude-sonnet-4-20241022")
	wantMain := CalculateTotalCost(sonnet, &TokenUsage{InputTokens: 100, OutputTokens: 50})
	wantAgent := CalculateTotalCost(sonnet, &TokenUsage{InputTokens: agentInput, OutputTokens: agentOutput})
	want := wantMain.Add(wantAgent)

	if !result.EstimatedCostUSD.Equal(want) {
		t.Errorf("EstimatedCostUSD = %s, want %s (main + file-less sub-agent priced at sonnet, not $0)",
			result.EstimatedCostUSD, want)
	}
	if !wantAgent.IsPositive() {
		t.Fatal("test misconfigured: expected sub-agent cost should be > 0")
	}
}

// TestTokensAnalyzer_FilelessSubagentNoWarn is the CF-546 log-spam contract: the
// file-less sub-agent path must NOT emit the empty-model "unknown model for
// pricing" WARN, because it now resolves the main session model.
func TestTokensAnalyzer_FilelessSubagentNoWarn(t *testing.T) {
	fc, err := filelessSubagentFixture(500, 250)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}

	log, buf := newCaptureLogger(slog.LevelDebug)
	if _, err := (&TokensAnalyzer{log: log}).Analyze(fc); err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if out := buf.String(); strings.Contains(out, "unknown model for pricing") {
		t.Errorf("file-less sub-agent path must not emit unknown-model WARN\ngot: %s", out)
	}
}

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
