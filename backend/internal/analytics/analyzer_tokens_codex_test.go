package analytics

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/shopspring/decimal"
)

// CF-471 regression: reasoning_output_tokens is a SUBSET of output_tokens on
// the OpenAI wire, not an additive bucket. computeCodexTokens must therefore
// surface output_tokens unchanged and never add reasoning to it.
//
// Evidence from the ticket (verified rollout):
//
//	input_tokens           22,129,704
//	cached_input_tokens    21,288,576
//	output_tokens             35,123
//	reasoning_output_tokens    6,002
//	total_tokens           22,164,827
//
// Math: input + output = total. If reasoning were additive, total would be
// 22,170,829. It is not.

// rolloutWithUsage builds a minimal gpt-5 rollout carrying the given wire
// token counts. All four CF-471 tests below differ only in the TokenUsage
// values, so the rest of the rollout is identical.
func rolloutWithUsage(usage codex.TokenUsage) *codex.ParsedRollout {
	return codexModelRollout("gpt-5", usage)
}

// TestComputeCodexTokens_TicketRollout_ReasoningNotAdded pins the exact
// rollout values from CF-471. computeCodexTokens must report OutputTokens
// equal to wire output_tokens (35,123), not output + reasoning (41,125),
// and the canonical recomposition `uncached_input + cache_read + output`
// must equal wire total_tokens (22,164,827).
func TestComputeCodexTokens_TicketRollout_ReasoningNotAdded(t *testing.T) {
	r := rolloutWithUsage(codex.TokenUsage{
		InputTokens:           22_129_704,
		CachedInputTokens:     21_288_576,
		OutputTokens:          35_123,
		ReasoningOutputTokens: 6_002,
		TotalTokens:           22_164_827,
	})
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.OutputTokens != 35_123 {
		t.Errorf("OutputTokens = %d, want 35,123 (wire output_tokens); reasoning must NOT be added", out.OutputTokens)
	}
	const wantTotal int64 = 22_164_827
	gotTotal := out.InputTokens + out.CacheReadTokens + out.OutputTokens
	if gotTotal != wantTotal {
		t.Errorf("uncached_input + cache_read + output = %d, want %d (wire total_tokens)", gotTotal, wantTotal)
	}
}

// TestComputeCodexTokens_OutputInvariantUnderReasoning asserts that varying
// reasoning_output_tokens does not change result.OutputTokens. Fixed wire
// output, swept reasoning.
func TestComputeCodexTokens_OutputInvariantUnderReasoning(t *testing.T) {
	cases := []struct {
		name      string
		reasoning int64
	}{
		{"reasoning_zero", 0},
		{"reasoning_one", 1},
		{"reasoning_small", 100},
		{"reasoning_huge", 1_000_000},
	}
	const fixedOutput int64 = 2000
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := rolloutWithUsage(codex.TokenUsage{
				InputTokens:           10000,
				CachedInputTokens:     4000,
				OutputTokens:          fixedOutput,
				ReasoningOutputTokens: tc.reasoning,
				TotalTokens:           12000,
			})
			out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{r})
			if out == nil {
				t.Fatal("ComputeFromCodexRollout returned nil")
			}
			if out.OutputTokens != fixedOutput {
				t.Errorf("OutputTokens = %d, want %d (must be invariant to reasoning=%d)", out.OutputTokens, fixedOutput, tc.reasoning)
			}
		})
	}
}

// TestComputeCodexTokens_DefensiveReasoningGreaterThanOutput guards against
// pathologically malformed wire data: even when reasoning_output_tokens is
// somehow larger than output_tokens (should never happen if reasoning is
// truly a subset), OutputTokens must equal the wire output_tokens — never
// the additive sum.
func TestComputeCodexTokens_DefensiveReasoningGreaterThanOutput(t *testing.T) {
	r := rolloutWithUsage(codex.TokenUsage{
		InputTokens:           100,
		CachedInputTokens:     0,
		OutputTokens:          10,
		ReasoningOutputTokens: 999,
		TotalTokens:           110,
	})
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.OutputTokens != 10 {
		t.Errorf("OutputTokens = %d, want 10 (wire output_tokens; reasoning must never inflate)", out.OutputTokens)
	}
}

// TestComputeCodexTokens_MultiRolloutReasoningExcluded pins the invariant
// across multi-rollout (subagent) sessions: total OutputTokens is the sum
// of per-rollout wire output_tokens, with reasoning excluded everywhere.
func TestComputeCodexTokens_MultiRolloutReasoningExcluded(t *testing.T) {
	main := rolloutWithUsage(codex.TokenUsage{
		InputTokens:           5000,
		CachedInputTokens:     1000,
		OutputTokens:          1500,
		ReasoningOutputTokens: 400,
		TotalTokens:           6500,
	})
	sub := rolloutWithUsage(codex.TokenUsage{
		InputTokens:           2000,
		CachedInputTokens:     500,
		OutputTokens:          800,
		ReasoningOutputTokens: 200,
		TotalTokens:           2800,
	})
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{main, sub})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	const wantOutput int64 = 1500 + 800 // 2300; never 1500+400 + 800+200 = 2900
	if out.OutputTokens != wantOutput {
		t.Errorf("OutputTokens = %d, want %d (sum of wire output_tokens across rollouts; reasoning excluded)", out.OutputTokens, wantOutput)
	}
}

// TestComputeCodexTokens_V2Tree is the 7eje contract for Codex: the per-model
// tokens_v2 tree under the canonical agent id "codex", keyed by getModelFamily()
// families, grouped per-rollout model, with reasoning surfaced per model and
// cache_creation 0 when the wire reports no cache writes.
func TestComputeCodexTokens_V2Tree(t *testing.T) {
	main := &codex.ParsedRollout{
		Model: "gpt-5",
		Turns: []codex.Turn{{Model: "gpt-5"}},
		TokenUsage: codex.TokenUsage{
			InputTokens:           10_000,
			CachedInputTokens:     4_000,
			OutputTokens:          2_000,
			ReasoningOutputTokens: 500,
			TotalTokens:           12_000,
		},
	}
	sub := &codex.ParsedRollout{
		Model: "gpt-5-mini",
		Turns: []codex.Turn{{Model: "gpt-5-mini"}},
		TokenUsage: codex.TokenUsage{
			InputTokens:           1_000,
			CachedInputTokens:     0,
			OutputTokens:          300,
			ReasoningOutputTokens: 50,
			TotalTokens:           1_300,
		},
	}

	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{main, sub})
	if out == nil || out.TokensV2 == nil {
		t.Fatal("TokensV2 not populated")
	}
	v2 := out.TokensV2

	prov, ok := v2.ByProvider["codex"]
	if !ok || len(v2.ByProvider) != 1 {
		t.Fatalf("ByProvider = %+v, want single key codex", v2.ByProvider)
	}

	gpt5, ok := prov.Models["gpt-5"]
	if !ok {
		t.Fatalf("missing model gpt-5: %+v", prov.Models)
	}
	// uncached input = 10000 - 4000; cache_read = 4000; output = 2000; reasoning = 500.
	if gpt5.Input != 6_000 || gpt5.CacheRead != 4_000 || gpt5.Output != 2_000 {
		t.Errorf("gpt-5 input/cacheRead/output = %d/%d/%d, want 6000/4000/2000", gpt5.Input, gpt5.CacheRead, gpt5.Output)
	}
	if gpt5.Reasoning != 500 {
		t.Errorf("gpt-5 reasoning = %d, want 500", gpt5.Reasoning)
	}
	if gpt5.CacheWrite != 0 {
		t.Errorf("gpt-5 cacheWrite = %d, want 0 (no cache writes on the wire)", gpt5.CacheWrite)
	}

	mini, ok := prov.Models["gpt-5-mini"]
	if !ok {
		t.Fatalf("missing model gpt-5-mini: %+v", prov.Models)
	}
	if mini.Input != 1_000 || mini.Output != 300 || mini.Reasoning != 50 {
		t.Errorf("gpt-5-mini input/output/reasoning = %d/%d/%d, want 1000/300/50", mini.Input, mini.Output, mini.Reasoning)
	}

	if v2.TotalInput != 7_000 || v2.TotalOutput != 2_300 {
		t.Errorf("totals input/output = %d/%d, want 7000/2300", v2.TotalInput, v2.TotalOutput)
	}
}

// codexModelRollout builds a minimal rollout on the given model carrying the given
// wire token counts.
func codexModelRollout(model string, usage codex.TokenUsage) *codex.ParsedRollout {
	return &codex.ParsedRollout{
		Model:      model,
		Turns:      []codex.Turn{{Model: model}},
		TokenUsage: usage,
	}
}

// wantCodexCost prices a request by hand from the model's table rates:
// ordinary input, cached reads, cache writes and output each at their own
// rate. It deliberately does not call CalculateCost.
func wantCodexCost(t *testing.T, model string, ordinary, cached, cacheWrite, output int64) decimal.Decimal {
	t.Helper()
	p, ok := LookupPricing(model)
	if !ok {
		t.Fatalf("no pricing row for %s", model)
	}
	sum := decimal.NewFromInt(ordinary).Mul(p.Input).
		Add(decimal.NewFromInt(cached).Mul(p.CacheRead)).
		Add(decimal.NewFromInt(cacheWrite).Mul(p.CacheWrite)).
		Add(decimal.NewFromInt(output).Mul(p.Output))
	return sum.Div(decimal.NewFromInt(1_000_000))
}

// md0z: cache_write_input_tokens is a SUBSET of input_tokens (OpenAI's
// prompt-caching guide; Codex's own status line computes
// input - cached - cache_write). The write portion leaves ordinary input,
// surfaces as cache creation, and bills at the table's cacheWrite rate.
func TestComputeCodexTokens_CacheWriteBilledAsSubsetOfInput(t *testing.T) {
	const model = "gpt-5.6-terra"
	if p, _ := LookupPricing(model); !p.CacheWrite.IsPositive() {
		t.Fatalf("%s cacheWrite rate = %s; this test needs a non-zero rate", model, p.CacheWrite)
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{codexModelRollout(model, codex.TokenUsage{
		InputTokens:           1000,
		CachedInputTokens:     600,
		CacheWriteInputTokens: 300,
		OutputTokens:          50,
		TotalTokens:           1050,
	})})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100 (input - cached - cache_write)", out.InputTokens)
	}
	if out.CacheReadTokens != 600 {
		t.Errorf("CacheReadTokens = %d, want 600", out.CacheReadTokens)
	}
	if out.CacheCreationTokens != 300 {
		t.Errorf("CacheCreationTokens = %d, want 300 (wire cache_write_input_tokens)", out.CacheCreationTokens)
	}
	if out.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", out.OutputTokens)
	}
	want := wantCodexCost(t, model, 100, 600, 300, 50)
	if !out.EstimatedCostUSD.Equal(want) {
		t.Errorf("EstimatedCostUSD = %s, want %s (cache writes billed at the cacheWrite rate)", out.EstimatedCostUSD, want)
	}

	if out.TokensV2 == nil {
		t.Fatal("TokensV2 not populated")
	}
	node, ok := out.TokensV2.ByProvider["codex"].Models[model]
	if !ok {
		t.Fatalf("tokens_v2 missing model %s: %+v", model, out.TokensV2.ByProvider)
	}
	if node.CacheWrite != 300 || node.Input != 100 {
		t.Errorf("tokens_v2 %s cache_creation/input = %d/%d, want 300/100", model, node.CacheWrite, node.Input)
	}
	if node.CostUSD != want.String() {
		t.Errorf("tokens_v2 %s cost = %s, want %s", model, node.CostUSD, want)
	}
	if out.TokensV2.TotalCacheCreation != 300 {
		t.Errorf("tokens_v2 total_cache_creation = %d, want 300", out.TokensV2.TotalCacheCreation)
	}
}

// md0z: malformed wire data where cached + cache_write exceeds input clamps
// ordinary input at 0 (never negative), and the write still bills in full.
func TestComputeCodexTokens_CacheWriteClampsOrdinaryInputAtZero(t *testing.T) {
	const model = "gpt-5.6-terra"
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{codexModelRollout(model, codex.TokenUsage{
		InputTokens:           500,
		CachedInputTokens:     400,
		CacheWriteInputTokens: 300,
		OutputTokens:          10,
	})})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0 (ordinary input clamps, never negative)", out.InputTokens)
	}
	if out.CacheCreationTokens != 300 {
		t.Errorf("CacheCreationTokens = %d, want 300 (the write is kept in full)", out.CacheCreationTokens)
	}
	want := wantCodexCost(t, model, 0, 400, 300, 10)
	if !out.EstimatedCostUSD.Equal(want) {
		t.Errorf("EstimatedCostUSD = %s, want %s", out.EstimatedCostUSD, want)
	}
}

// md0z D2: a model whose table cacheWrite rate is 0 (pre-5.6) gets no
// fallback rate. Its write tokens still leave ordinary input and surface as
// cache creation, but bill at $0.
func TestComputeCodexTokens_CacheWriteOnZeroRateModelBillsZero(t *testing.T) {
	const model = "gpt-5.5"
	if p, _ := LookupPricing(model); !p.CacheWrite.IsZero() {
		t.Fatalf("%s cacheWrite rate = %s; this test needs a zero rate", model, p.CacheWrite)
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{codexModelRollout(model, codex.TokenUsage{
		InputTokens:           1000,
		CachedInputTokens:     200,
		CacheWriteInputTokens: 300,
		OutputTokens:          40,
	})})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.InputTokens != 500 {
		t.Errorf("InputTokens = %d, want 500 (write tokens removed from ordinary input)", out.InputTokens)
	}
	if out.CacheCreationTokens != 300 {
		t.Errorf("CacheCreationTokens = %d, want 300", out.CacheCreationTokens)
	}
	want := wantCodexCost(t, model, 500, 200, 0, 40)
	if !out.EstimatedCostUSD.Equal(want) {
		t.Errorf("EstimatedCostUSD = %s, want %s (writes on a 0-rate model bill at $0, no fallback)", out.EstimatedCostUSD, want)
	}
}

// md0z: cache writes sum across the main rollout and its subagents, both in
// the session total and in the tokens_v2 tree.
func TestComputeCodexTokens_CacheWriteSummedAcrossRollouts(t *testing.T) {
	const model = "gpt-5.6-terra"
	main := codexModelRollout(model, codex.TokenUsage{
		InputTokens:           5000,
		CachedInputTokens:     1000,
		CacheWriteInputTokens: 2000,
		OutputTokens:          100,
	})
	sub := codexModelRollout(model, codex.TokenUsage{
		InputTokens:           3000,
		CachedInputTokens:     500,
		CacheWriteInputTokens: 700,
		OutputTokens:          60,
	})
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{main, sub})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.CacheCreationTokens != 2700 {
		t.Errorf("CacheCreationTokens = %d, want 2700 (2000 + 700)", out.CacheCreationTokens)
	}
	if out.InputTokens != 2000+1800 {
		t.Errorf("InputTokens = %d, want 3800 (per-rollout input - cached - cache_write)", out.InputTokens)
	}
	want := wantCodexCost(t, model, 3800, 1500, 2700, 160)
	if !out.EstimatedCostUSD.Equal(want) {
		t.Errorf("EstimatedCostUSD = %s, want %s", out.EstimatedCostUSD, want)
	}
	if out.TokensV2 == nil || out.TokensV2.TotalCacheCreation != 2700 {
		t.Errorf("tokens_v2 total_cache_creation = %+v, want 2700", out.TokensV2)
	}
}
