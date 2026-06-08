package analytics

import (
	"context"
	"fmt"
	"testing"
)

// opencodeAssistantMessage builds a minimal assistant message with explicit
// tokens, model/provider, and timestamp. Used by the multi-rollout merge
// tests to make per-rollout token attribution unambiguous.
func opencodeAssistantMessage(id, sessionID, providerID, modelID string, tokens OpenCodeTokens, createdMs int64) *OpenCodeMessage {
	finish := "stop"
	return &OpenCodeMessage{
		Info: OpenCodeMessageInfo{
			ID:         id,
			SessionID:  sessionID,
			Role:       "assistant",
			ModelID:    modelID,
			ProviderID: providerID,
			Finish:     &finish,
			Tokens:     tokens,
			Time:       OpenCodeTime{Created: createdMs},
		},
	}
}

func opencodeUserMessage(id, sessionID string, createdMs int64) *OpenCodeMessage {
	return &OpenCodeMessage{
		Info: OpenCodeMessageInfo{
			ID:        id,
			SessionID: sessionID,
			Role:      "user",
			Time:      OpenCodeTime{Created: createdMs},
		},
	}
}

// TestComputeFromOpenCodeRollout_WithSubagents proves the merge: tokens sum
// across all rollouts; UserMessages includes subagent user msgs (Codex
// parity, accepted inflation); Conversation card stays main-only;
// TokensV2.ByProvider lists every (providerID, modelID) pair encountered.
func TestComputeFromOpenCodeRollout_WithSubagents(t *testing.T) {
	main := []*OpenCodeMessage{
		opencodeUserMessage("msg_main_u", "ses_main", 1717689600000),
		opencodeAssistantMessage("msg_main_a", "ses_main", "anthropic", "claude-sonnet-4-20250514",
			OpenCodeTokens{Input: 2000, Output: 400}, 1717689601000),
	}
	sub1 := []*OpenCodeMessage{
		opencodeUserMessage("msg_sub1_u", "ses_sub1", 1717689610000),
		opencodeAssistantMessage("msg_sub1_a", "ses_sub1", "anthropic", "claude-sonnet-4-20250514",
			OpenCodeTokens{Input: 500, Output: 100}, 1717689611000),
	}
	sub2 := []*OpenCodeMessage{
		opencodeUserMessage("msg_sub2_u", "ses_sub2", 1717689620000),
		opencodeAssistantMessage("msg_sub2_a", "ses_sub2", "anthropic", "claude-sonnet-4-20250514",
			OpenCodeTokens{Input: 300, Output: 50}, 1717689621000),
	}

	out := ComputeFromOpenCodeRollout(context.Background(), [][]*OpenCodeMessage{main, sub1, sub2})
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}

	// Tokens sum across all rollouts.
	if out.InputTokens != 2800 {
		t.Errorf("InputTokens = %d, want 2800 (main 2000 + sub1 500 + sub2 300)", out.InputTokens)
	}
	if out.OutputTokens != 550 {
		t.Errorf("OutputTokens = %d, want 550 (main 400 + sub1 100 + sub2 50)", out.OutputTokens)
	}

	// Session UserMessages includes subagent user msgs (Codex parity).
	if out.UserMessages != 3 {
		t.Errorf("UserMessages = %d, want 3 (1 main + 2 subagent)", out.UserMessages)
	}
	if out.AssistantMessages != 3 {
		t.Errorf("AssistantMessages = %d, want 3 (1 main + 2 subagent)", out.AssistantMessages)
	}

	// Conversation card stays main-only.
	if out.UserTurns != 1 {
		t.Errorf("UserTurns = %d, want 1 (Conversation card excludes subagent turns)", out.UserTurns)
	}
	if out.AssistantTurns != 1 {
		t.Errorf("AssistantTurns = %d, want 1 (Conversation card excludes subagent turns)", out.AssistantTurns)
	}

	// TokensV2 lists the merged providers.
	if out.TokensV2 == nil {
		t.Fatal("TokensV2 not populated")
	}
	prov, ok := out.TokensV2.ByProvider["anthropic"]
	if !ok {
		t.Fatalf("ByProvider missing anthropic: %v", out.TokensV2.ByProvider)
	}
	model, ok := prov.Models["claude-sonnet-4-20250514"]
	if !ok {
		t.Fatalf("anthropic models missing claude-sonnet-4-20250514: %v", prov.Models)
	}
	if model.Input != 2800 || model.Output != 550 {
		t.Errorf("merged model tokens = {input=%d output=%d}, want {input=2800 output=550}", model.Input, model.Output)
	}
}

// TestComputeFromOpenCodeRollout_SubagentDifferentModel proves the merged
// tokens_v2 tree correctly bucks subagents that use a different provider/model
// than the root, and that per-message OpenAI cache-subset normalization still
// applies when the providers are mixed in one session.
func TestComputeFromOpenCodeRollout_SubagentDifferentModel(t *testing.T) {
	main := []*OpenCodeMessage{
		opencodeAssistantMessage("msg_main_a", "ses_main", "openai", "gpt-4o",
			OpenCodeTokens{Input: 8000, Output: 2000, Cache: OpenCodeCache{Read: 3000}}, 1717689600000),
	}
	sub := []*OpenCodeMessage{
		opencodeAssistantMessage("msg_sub_a", "ses_sub", "anthropic", "claude-sonnet-4-20250514",
			OpenCodeTokens{Input: 1000, Output: 500, Cache: OpenCodeCache{Read: 200, Write: 100}}, 1717689610000),
	}

	out := ComputeFromOpenCodeRollout(context.Background(), [][]*OpenCodeMessage{main, sub})
	if out == nil || out.TokensV2 == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil or no TokensV2")
	}
	if len(out.TokensV2.ByProvider) != 2 {
		t.Fatalf("ByProvider has %d providers, want 2 (openai + anthropic): %+v",
			len(out.TokensV2.ByProvider), out.TokensV2.ByProvider)
	}

	// OpenAI per-message normalization: input adjusted to uncached (8000-3000=5000); cache_write zeroed.
	openai := out.TokensV2.ByProvider["openai"].Models["gpt-4o"]
	if openai.Input != 5000 {
		t.Errorf("openai.input = %d, want 5000 (8000 raw - 3000 cached)", openai.Input)
	}
	if openai.CacheWrite != 0 {
		t.Errorf("openai.cache_write = %d, want 0 (OpenAI doesn't bill writes)", openai.CacheWrite)
	}

	// Anthropic per-message normalization: input passes through; cache_write billed.
	anthropic := out.TokensV2.ByProvider["anthropic"].Models["claude-sonnet-4-20250514"]
	if anthropic.Input != 1000 {
		t.Errorf("anthropic.input = %d, want 1000 (no subtraction)", anthropic.Input)
	}
	if anthropic.CacheWrite != 100 {
		t.Errorf("anthropic.cache_write = %d, want 100 (Anthropic bills writes)", anthropic.CacheWrite)
	}
}

// TestComputeFromOpenCodeRollout_DurationSpansAllRollouts proves the Session
// card's DurationMs envelope extends to subagent timestamps that fall outside
// the main rollout's min/max.
func TestComputeFromOpenCodeRollout_DurationSpansAllRollouts(t *testing.T) {
	main := []*OpenCodeMessage{
		opencodeAssistantMessage("msg_main_a", "ses_main", "anthropic", "claude-sonnet-4-20250514",
			OpenCodeTokens{Input: 100, Output: 50}, 1000),
	}
	// Subagent starts later than main's only timestamp.
	sub := []*OpenCodeMessage{
		opencodeAssistantMessage("msg_sub_a", "ses_sub", "anthropic", "claude-sonnet-4-20250514",
			OpenCodeTokens{Input: 100, Output: 50}, 5000),
	}

	out := ComputeFromOpenCodeRollout(context.Background(), [][]*OpenCodeMessage{main, sub})
	if out == nil || out.DurationMs == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil or no DurationMs")
	}
	if *out.DurationMs != 4000 {
		t.Errorf("DurationMs = %d, want 4000 (5000 sub - 1000 main, wall-clock envelope)", *out.DurationMs)
	}
}

// TestMaterializeOpenCodeRollout_DownloadFailureSurfacesValidationError
// proves that when a subagent file fails to download, the rollout's
// validationErrors slice picks up one entry and ComputeCards surfaces it as
// ValidationErrorCount. Remaining rollouts still compute correctly.
func TestMaterializeOpenCodeRollout_DownloadFailureSurfacesValidationError(t *testing.T) {
	main := []*OpenCodeMessage{
		opencodeAssistantMessage("msg_main_a", "ses_main", "anthropic", "claude-sonnet-4-20250514",
			OpenCodeTokens{Input: 100, Output: 50}, 1000),
	}
	// One subagent download that succeeds, one that fails.
	goodFile := `{"info":{"id":"msg_sub_a","sessionID":"ses_sub","role":"assistant","modelID":"claude-sonnet-4-20250514","providerID":"anthropic","cost":0,"tokens":{"input":200,"output":100,"cache":{"read":0,"write":0}},"time":{"created":2000},"finish":"stop"},"parts":[]}` + "\n"
	downloader := func(_ context.Context, fileName string) ([]byte, error) {
		switch fileName {
		case "good.jsonl":
			return []byte(goodFile), nil
		case "bad.jsonl":
			return nil, fmt.Errorf("simulated S3 failure")
		}
		return nil, fmt.Errorf("unexpected file: %s", fileName)
	}
	r := &opencodeRollout{
		main: main,
		agentFileInfo: []opencodeAgentFileInfo{
			{FileName: "good.jsonl"},
			{FileName: "bad.jsonl"},
		},
		downloader: downloader,
	}

	p := &opencodeProvider{}
	result := p.ComputeCards(context.Background(), r)
	if result == nil {
		t.Fatal("ComputeCards returned nil")
	}

	if result.ValidationErrorCount != 1 {
		t.Errorf("ValidationErrorCount = %d, want 1 (one subagent file failed)", result.ValidationErrorCount)
	}
	// Remaining rollouts still compute: main 100 + good sub 200 = 300 input.
	if result.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300 (main 100 + good subagent 200; bad file dropped)", result.InputTokens)
	}
}
