package analytics

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/codex"
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
	return &codex.ParsedRollout{
		Model:      "gpt-5",
		Turns:      []codex.Turn{{Model: "gpt-5"}},
		TokenUsage: usage,
	}
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
