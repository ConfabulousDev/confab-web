package analytics

import (
	"testing"

	"github.com/shopspring/decimal"
)

func opencodeAssistantMsg(providerID, modelID string, tokens OpenCodeTokens) *OpenCodeMessage {
	finish := "stop"
	return &OpenCodeMessage{
		Info: OpenCodeMessageInfo{
			ID:         "msg_01",
			SessionID:  "ses_01",
			Role:       "assistant",
			ModelID:    modelID,
			ProviderID: providerID,
			Finish:     &finish,
			Cost:       0,
			Tokens:     tokens,
			Time:       OpenCodeTime{Created: 1717689600000},
		},
		Parts: []OpenCodePart{},
	}
}

func TestComputeOpenCodeTokens_AnthropicProvider(t *testing.T) {
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			opencodeAssistantMsg("anthropic", "claude-sonnet-4-20250514", OpenCodeTokens{
				Input: 10000, Output: 5000, Reasoning: 2000,
				Cache: OpenCodeCache{Read: 3000, Write: 2000},
			}),
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.InputTokens != 10000 {
		t.Errorf("InputTokens = %d, want 10000 (Anthropic: input is total, not adjusted)", out.InputTokens)
	}
	if out.OutputTokens != 5000 {
		t.Errorf("OutputTokens = %d, want 5000", out.OutputTokens)
	}
	if out.CacheCreationTokens != 2000 {
		t.Errorf("CacheCreationTokens = %d, want 2000 (Anthropic: cache_write is billed independently)", out.CacheCreationTokens)
	}
	if out.CacheReadTokens != 3000 {
		t.Errorf("CacheReadTokens = %d, want 3000", out.CacheReadTokens)
	}
	if out.EstimatedCostUSD.IsZero() {
		t.Errorf("EstimatedCostUSD = 0, want non-zero for known Anthropic model")
	}
}

func TestComputeOpenCodeTokens_OpenAIProvider(t *testing.T) {
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			opencodeAssistantMsg("openai", "gpt-4o", OpenCodeTokens{
				Input: 10000, Output: 2000, Reasoning: 500,
				Cache: OpenCodeCache{Read: 4000, Write: 0},
			}),
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	// OpenAI: cached is subset of input, so uncached = input - cached = 6000
	if out.InputTokens != 6000 {
		t.Errorf("InputTokens = %d, want 6000 (OpenAI: 10000 raw - 4000 cached)", out.InputTokens)
	}
	if out.CacheReadTokens != 4000 {
		t.Errorf("CacheReadTokens = %d, want 4000", out.CacheReadTokens)
	}
	// OpenAI: cache writes are free
	if out.CacheCreationTokens != 0 {
		t.Errorf("CacheCreationTokens = %d, want 0 (OpenAI doesn't charge cache writes)", out.CacheCreationTokens)
	}
	// OpenAI: reasoning is subset of output, output passes through unchanged
	if out.OutputTokens != 2000 {
		t.Errorf("OutputTokens = %d, want 2000 (reasoning is subset, not additive)", out.OutputTokens)
	}
}

func TestComputeOpenCodeTokens_V2Tree(t *testing.T) {
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			opencodeAssistantMsg("anthropic", "claude-sonnet-4-20250514", OpenCodeTokens{
				Input: 10000, Output: 5000, Reasoning: 2000,
				Cache: OpenCodeCache{Read: 3000, Write: 2000},
			}),
			opencodeAssistantMsg("openai", "gpt-4o", OpenCodeTokens{
				Input: 8000, Output: 3000,
				Cache: OpenCodeCache{Read: 2000, Write: 0},
			}),
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil || out.TokensV2 == nil {
		t.Fatal("TokensV2 not populated")
	}
	v2 := out.TokensV2

	// Totals mirror the flat card (consistency invariant).
	if v2.TotalInput != out.InputTokens || v2.TotalInput != 16000 {
		t.Errorf("TotalInput = %d, want 16000 (== flat InputTokens %d)", v2.TotalInput, out.InputTokens)
	}
	if v2.TotalOutput != out.OutputTokens || v2.TotalOutput != 8000 {
		t.Errorf("TotalOutput = %d, want 8000 (== flat OutputTokens %d)", v2.TotalOutput, out.OutputTokens)
	}
	if v2.TotalCostUSD != out.EstimatedCostUSD.String() {
		t.Errorf("TotalCostUSD = %q, want %q (flat cost)", v2.TotalCostUSD, out.EstimatedCostUSD.String())
	}

	if len(v2.ByProvider) != 2 {
		t.Fatalf("ByProvider has %d providers, want 2: %+v", len(v2.ByProvider), v2.ByProvider)
	}

	anthropic, ok := v2.ByProvider["anthropic"]
	if !ok {
		t.Fatal("missing anthropic provider")
	}
	am, ok := anthropic.Models["claude-sonnet-4-20250514"]
	if !ok {
		t.Fatal("missing anthropic model")
	}
	if am.Input != 10000 || am.Output != 5000 || am.CacheRead != 3000 || am.CacheWrite != 2000 || am.Reasoning != 2000 {
		t.Errorf("anthropic model tokens = %+v", am)
	}

	openai, ok := v2.ByProvider["openai"]
	if !ok {
		t.Fatal("missing openai provider")
	}
	om, ok := openai.Models["gpt-4o"]
	if !ok {
		t.Fatal("missing openai model")
	}
	// OpenAI: input adjusted to uncached (8000-2000), cache_write zeroed.
	if om.Input != 6000 || om.Output != 3000 || om.CacheRead != 2000 || om.CacheWrite != 0 {
		t.Errorf("openai model tokens = %+v, want input=6000 cache_write=0", om)
	}
}

func TestComputeOpenCodeTokens_PrefersReportedCost(t *testing.T) {
	// An unpriced model (no pricing-table entry) with OpenCode-reported cost:
	// hybrid must use the reported cost, not the $0 pricing fallback. This is the
	// long-tail case — OpenCode supports 75+ providers, most unpriced by us.
	msg := opencodeAssistantMsg("deepseek", "future-model-2099", OpenCodeTokens{Input: 1000, Output: 500})
	msg.Info.Cost = 0.42
	out := ComputeFromOpenCodeRollout(&opencodeRollout{Messages: []*OpenCodeMessage{msg}})
	if out == nil || out.TokensV2 == nil {
		t.Fatal("TokensV2 not populated")
	}
	want := "0.42"
	if got := out.EstimatedCostUSD.String(); got != want {
		t.Errorf("EstimatedCostUSD = %s, want %s (reported cost, not $0 pricing fallback)", got, want)
	}
	if out.TokensV2.TotalCostUSD != want {
		t.Errorf("TokensV2.TotalCostUSD = %s, want %s", out.TokensV2.TotalCostUSD, want)
	}
	if m := out.TokensV2.ByProvider["deepseek"].Models["future-model-2099"]; m.CostUSD != want {
		t.Errorf("model cost = %s, want %s", m.CostUSD, want)
	}
}

func TestComputeOpenCodeTokens_MultiProviderSession(t *testing.T) {
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			opencodeAssistantMsg("anthropic", "claude-sonnet-4-20250514", OpenCodeTokens{
				Input: 10000, Output: 5000,
				Cache: OpenCodeCache{Read: 3000, Write: 2000},
			}),
			opencodeAssistantMsg("openai", "gpt-4o", OpenCodeTokens{
				Input: 8000, Output: 3000,
				Cache: OpenCodeCache{Read: 2000, Write: 0},
			}),
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	// Anthropic: input=10000 (no adjustment); OpenAI: input=8000-2000=6000
	if out.InputTokens != 16000 {
		t.Errorf("InputTokens = %d, want 16000 (10000 Anthropic + 6000 OpenAI uncached)", out.InputTokens)
	}
	if out.OutputTokens != 8000 {
		t.Errorf("OutputTokens = %d, want 8000 (5000 + 3000)", out.OutputTokens)
	}
	if out.CacheCreationTokens != 2000 {
		t.Errorf("CacheCreationTokens = %d, want 2000 (only Anthropic charges writes)", out.CacheCreationTokens)
	}
	if out.CacheReadTokens != 5000 {
		t.Errorf("CacheReadTokens = %d, want 5000 (3000 + 2000)", out.CacheReadTokens)
	}
	if out.EstimatedCostUSD.IsZero() {
		t.Errorf("EstimatedCostUSD = 0, want non-zero for known models")
	}
}

func TestComputeOpenCodeTokens_UnknownModel(t *testing.T) {
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			opencodeAssistantMsg("unknown-provider", "future-model-2099", OpenCodeTokens{
				Input: 1000, Output: 500,
			}),
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if !out.EstimatedCostUSD.IsZero() {
		t.Errorf("EstimatedCostUSD = %s, want 0 for unknown model", out.EstimatedCostUSD)
	}
	// Token counts still pass through even without pricing
	if out.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", out.InputTokens)
	}
	if out.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", out.OutputTokens)
	}
}

func TestComputeOpenCodeTokens_FastTurnsAlwaysZero(t *testing.T) {
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			opencodeAssistantMsg("anthropic", "claude-sonnet-4-20250514", OpenCodeTokens{
				Input: 10000, Output: 5000,
			}),
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	// OpenCode has no fast mode concept
	if out.FastTurns != 0 {
		t.Errorf("FastTurns = %d, want 0 (OpenCode has no fast mode)", out.FastTurns)
	}
	if !out.FastCostUSD.IsZero() {
		t.Errorf("FastCostUSD = %s, want 0", out.FastCostUSD)
	}
}

func TestComputeOpenCodeTokens_CostPrecision(t *testing.T) {
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			opencodeAssistantMsg("anthropic", "claude-sonnet-4-20250514", OpenCodeTokens{
				Input: 1000000, Output: 500000,
				Cache: OpenCodeCache{Read: 500000, Write: 100000},
			}),
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	// Cost must be positive and precise for a large known-model session
	if out.EstimatedCostUSD.LessThanOrEqual(decimal.Zero) {
		t.Errorf("EstimatedCostUSD = %s, want positive for large known-model session", out.EstimatedCostUSD)
	}
}
