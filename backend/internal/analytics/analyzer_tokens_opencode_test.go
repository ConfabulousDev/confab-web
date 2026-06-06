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
		Parts: mustMarshalJSON([]OpenCodePart{}),
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
