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

// TestTokensAnalyzer_V2Tree is the 7eje contract: the Claude TokensAnalyzer must
// build a per-provider → per-model tokens_v2 tree under the canonical agent id
// "claude-code", keyed by getModelFamily() families, with fast turns split into a
// "<family> · fast" key, and a TotalCostUSD that reconciles exactly with the flat
// EstimatedCostUSD.
func TestTokensAnalyzer_V2Tree(t *testing.T) {
	jsonl := makeAssistantMessageFull("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, 0, 0,
		[]map[string]interface{}{makeTextBlock("sonnet normal")}) + "\n" +
		makeAssistantMessageFull("a2", "2025-01-01T00:00:02Z", "claude-opus-4-1-20250805", 200, 100, 0, 0,
			[]map[string]interface{}{makeTextBlock("opus normal")}) + "\n" +
		makeAssistantMessageWithMsgIDAndSpeed("a3", "2025-01-01T00:00:03Z", "claude-sonnet-4-20241022", "msg-fast", 300, 150,
			[]map[string]interface{}{makeTextBlock("sonnet fast")}, "fast") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection: %v", err)
	}
	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.TokensV2 == nil {
		t.Fatal("TokensV2 not populated")
	}
	v2 := result.TokensV2

	if len(v2.ByProvider) != 1 {
		t.Fatalf("ByProvider has %d entries, want 1: %+v", len(v2.ByProvider), v2.ByProvider)
	}
	prov, ok := v2.ByProvider["claude-code"]
	if !ok {
		t.Fatalf("ByProvider missing canonical agent key claude-code: %+v", v2.ByProvider)
	}

	want := map[string]struct{ in, out int64 }{
		"sonnet-4":        {100, 50},
		"opus-4-1":        {200, 100},
		"sonnet-4 · fast": {300, 150},
	}
	if len(prov.Models) != len(want) {
		t.Fatalf("models = %d, want %d: %+v", len(prov.Models), len(want), prov.Models)
	}
	for key, w := range want {
		m, ok := prov.Models[key]
		if !ok {
			t.Fatalf("missing model key %q: %+v", key, prov.Models)
		}
		if m.Input != w.in || m.Output != w.out {
			t.Errorf("model %q: input/output = %d/%d, want %d/%d", key, m.Input, m.Output, w.in, w.out)
		}
	}

	if v2.TotalInput != 600 {
		t.Errorf("TotalInput = %d, want 600", v2.TotalInput)
	}
	if v2.TotalOutput != 300 {
		t.Errorf("TotalOutput = %d, want 300", v2.TotalOutput)
	}

	// Reconciliation: the v2 tree total must equal the flat card cost exactly.
	if v2.TotalCostUSD != result.EstimatedCostUSD.String() {
		t.Errorf("TotalCostUSD = %s, want %s (must reconcile with flat EstimatedCostUSD)",
			v2.TotalCostUSD, result.EstimatedCostUSD.String())
	}

	// The fast key carries the 6× cost: it must exceed the same-token non-fast
	// sonnet entry scaled up (300/100 input → already 3× tokens, ×6 fast > 3×).
	fast := prov.Models["sonnet-4 · fast"]
	if fast.CostUSD == "0" || fast.CostUSD == "" {
		t.Errorf("fast model cost = %q, want > 0 (6× fast pricing)", fast.CostUSD)
	}
}

// TestTokensAnalyzer_V2Tree_UnknownModelBucket pins the empty/unrecognized-model
// edge (7eje decision F): such usage lands under the "" family key with tokens
// counted and $0 cost, keeping the v2 totals equal to the flat card's totals.
func TestTokensAnalyzer_V2Tree_UnknownModelBucket(t *testing.T) {
	jsonl := makeAssistantMessageFull("a1", "2025-01-01T00:00:01Z", "", 100, 50, 0, 0,
		[]map[string]interface{}{makeTextBlock("no model")}) + "\n"
	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection: %v", err)
	}
	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.TokensV2 == nil {
		t.Fatal("TokensV2 not populated")
	}
	prov := result.TokensV2.ByProvider["claude-code"]
	m, ok := prov.Models[""]
	if !ok {
		t.Fatalf("missing empty-model bucket: %+v", prov.Models)
	}
	if m.Input != 100 || m.Output != 50 {
		t.Errorf("empty bucket input/output = %d/%d, want 100/50", m.Input, m.Output)
	}
	// Unpriced → $0, and the v2 total still reconciles with the flat total.
	if result.TokensV2.TotalCostUSD != result.EstimatedCostUSD.String() {
		t.Errorf("TotalCostUSD = %s, want %s", result.TokensV2.TotalCostUSD, result.EstimatedCostUSD.String())
	}
}

// TestTokensAnalyzer_V2Tree_NilWhenNoTokens: a session with no assistant token
// usage produces no v2 tree (nil), so the card stays empty and unserved.
func TestTokensAnalyzer_V2Tree_NilWhenNoTokens(t *testing.T) {
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:01Z", "hello") + "\n"
	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection: %v", err)
	}
	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.TokensV2 != nil {
		t.Errorf("TokensV2 = %+v, want nil for a token-less session", result.TokensV2)
	}
}
