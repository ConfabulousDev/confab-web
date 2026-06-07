package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// Spec / regression tests for CF-437. Backend semantics for the Session card
// don't change in this ticket — the audit's claimed HumanPrompts bug turned
// out to be a false alarm (the Codex parser separates tool outputs from
// user messages at the wire format, so out.HumanPrompts is already correct).
// These tests pin the invariant so a future parser change can't quietly
// break it.

// TestComputeCodexSession_HumanPromptsExcludesToolOutputs verifies that
// Codex tool outputs (function_call_output / custom_tool_call_output) flow
// through turn.ToolCalls and NEVER into turn.UserMessages. This is the
// invariant that lets out.HumanPrompts = len(turn.UserMessages) be correct
// without a Claude-style IsHumanMessage filter.
func TestComputeCodexSession_HumanPromptsExcludesToolOutputs(t *testing.T) {
	base := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	rollout := &codex.ParsedRollout{
		Model: "gpt-5-codex",
		Turns: []codex.Turn{{
			TurnID:    "t1",
			StartedAt: ptrTime(base),
			Model:     "gpt-5-codex",
			UserMessages: []codex.Message{
				{Role: "user", Text: "first prompt", Timestamp: base},
				{Role: "user", Text: "second prompt", Timestamp: base.Add(30 * time.Second)},
			},
			// 5 tool calls, each with output. In a buggy parser these might
			// have leaked into UserMessages — they must not.
			ToolCalls: []codex.ToolCall{
				{CallID: "c1", Name: "exec_command", Arguments: `{"cmd":"ls"}`, Output: "file1\nfile2", Status: "completed"},
				{CallID: "c2", Name: "apply_patch", Arguments: "*** Begin Patch", Output: "ok", Status: "completed"},
				{CallID: "c3", Name: "exec_command", Arguments: `{"cmd":"pwd"}`, Output: "/tmp", Status: "completed"},
				{CallID: "c4", Name: "web_search_call", Arguments: `{}`, Output: "results", Status: "completed"},
				{CallID: "c5", Name: "exec_command", Arguments: `{"cmd":"echo hi"}`, Output: "hi", Status: "completed"},
			},
		}},
	}

	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	if out.HumanPrompts != 2 {
		t.Errorf("HumanPrompts = %d, want 2 (only genuine user prompts; tool outputs must not inflate)", out.HumanPrompts)
	}
	if out.UserMessages != 2 {
		t.Errorf("UserMessages = %d, want 2 (Codex user-role messages exclude tool outputs)", out.UserMessages)
	}
	if out.ToolCalls != 5 {
		t.Errorf("ToolCalls = %d, want 5", out.ToolCalls)
	}
	if out.ToolResults != 5 {
		t.Errorf("ToolResults = %d, want 5 (all tool calls produced output)", out.ToolResults)
	}
}

// TestComputeCodexSession_ThinkingBlocksMirrorsReasoningCount documents the
// backend contract: ThinkingBlocks reflects the raw ReasoningCount. The
// semantic relabeling for the UI ("Reasoning steps" rather than "Thinking
// blocks") happens on the frontend in SessionCard.tsx; the backend stays
// provider-neutral.
func TestComputeCodexSession_ThinkingBlocksMirrorsReasoningCount(t *testing.T) {
	rollout := &codex.ParsedRollout{
		Turns: []codex.Turn{
			{TurnID: "t1", ReasoningCount: 3},
			{TurnID: "t2", ReasoningCount: 2},
		},
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	if out.ThinkingBlocks != 5 {
		t.Errorf("ThinkingBlocks = %d, want 5 (sum of per-turn ReasoningCount)", out.ThinkingBlocks)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
