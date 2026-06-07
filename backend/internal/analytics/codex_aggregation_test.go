package analytics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// Spec tests for CF-403 Codex subagent aggregation. The public codex
// orchestrators (ComputeFromCodexRollout, PrepareCodexTranscript,
// ExtractCodexUserMessagesText) take []*codex.ParsedRollout. Phase 4 makes
// each analyzer aggregate across the full slice (except Conversation, which
// stays main-only). Phase 3b stub uses rollouts[0] only — these tests fail
// until the per-analyzer aggregation is wired.

// rolloutWithUserMessage returns a minimally-populated ParsedRollout with
// one user message, one final assistant message, and the supplied tool call.
// Each rollout uses its own model name so ModelsUsed aggregation can be
// verified.
func rolloutWithUserMessage(model string, userText string, toolName string, started time.Time) *codex.ParsedRollout {
	completed := started.Add(5 * time.Second)
	dur := int64(5000)
	return &codex.ParsedRollout{
		Model:         model,
		ModelProvider: "openai",
		Turns: []codex.Turn{
			{
				TurnID:      "t-" + model,
				StartedAt:   &started,
				CompletedAt: &completed,
				DurationMs:  &dur,
				Model:       model,
				UserMessages: []codex.Message{
					{Role: "user", Text: userText, Timestamp: started},
				},
				AssistantMessages: []codex.Message{
					{Role: "assistant", Text: "ok", Phase: "final", Timestamp: started.Add(time.Second)},
				},
				ToolCalls: []codex.ToolCall{
					{CallID: "c-" + model, Name: toolName, Arguments: `{"x":1}`, Output: "ok", Status: "completed", Timestamp: started.Add(2 * time.Second)},
				},
			},
		},
		TokenUsage: codex.TokenUsage{
			InputTokens:  1000,
			OutputTokens: 200,
			TotalTokens:  1200,
		},
	}
}

// TestCodexAggregation_TokensAcrossSubagents asserts that token totals sum
// across all rollouts in the slice. Phase 3b uses rollouts[0] only so input
// tokens = 1000, not 3000.
func TestCodexAggregation_TokensAcrossSubagents(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	rollouts := []*codex.ParsedRollout{
		rolloutWithUserMessage("gpt-5", "main", "exec_command", t0),
		rolloutWithUserMessage("gpt-5", "sub1", "exec_command", t0.Add(10*time.Second)),
		rolloutWithUserMessage("gpt-5", "sub2", "exec_command", t0.Add(20*time.Second)),
	}
	out := ComputeFromCodexRollout(context.Background(), rollouts)
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.InputTokens != 3000 {
		t.Errorf("InputTokens = %d, want 3000 (1000 main + 1000 sub1 + 1000 sub2)", out.InputTokens)
	}
	if out.OutputTokens != 600 {
		t.Errorf("OutputTokens = %d, want 600 (200 per rollout * 3)", out.OutputTokens)
	}
}

// TestCodexAggregation_ToolsAcrossSubagents asserts tool counts merge.
func TestCodexAggregation_ToolsAcrossSubagents(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	rollouts := []*codex.ParsedRollout{
		rolloutWithUserMessage("gpt-5", "main", "exec_command", t0),
		rolloutWithUserMessage("gpt-5", "sub", "apply_patch", t0.Add(10*time.Second)),
	}
	out := ComputeFromCodexRollout(context.Background(), rollouts)
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.TotalToolCalls != 2 {
		t.Errorf("TotalToolCalls = %d, want 2 (one per rollout)", out.TotalToolCalls)
	}
	if out.ToolStats["exec_command"] == nil || out.ToolStats["exec_command"].Success != 1 {
		t.Errorf("ToolStats[exec_command].Success missing or wrong: %v", out.ToolStats["exec_command"])
	}
	if out.ToolStats["apply_patch"] == nil || out.ToolStats["apply_patch"].Success != 1 {
		t.Errorf("ToolStats[apply_patch].Success missing or wrong: %v", out.ToolStats["apply_patch"])
	}
}

// TestCodexAggregation_SessionModelsUsedUnion asserts each distinct model
// from any rollout appears exactly once in ModelsUsed.
func TestCodexAggregation_SessionModelsUsedUnion(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	rollouts := []*codex.ParsedRollout{
		rolloutWithUserMessage("gpt-5", "main", "exec_command", t0),
		rolloutWithUserMessage("gpt-5-mini", "sub", "exec_command", t0.Add(10*time.Second)),
	}
	out := ComputeFromCodexRollout(context.Background(), rollouts)
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	models := map[string]bool{}
	for _, m := range out.ModelsUsed {
		models[m] = true
	}
	if !models["gpt-5"] || !models["gpt-5-mini"] {
		t.Errorf("ModelsUsed = %v, want both gpt-5 and gpt-5-mini", out.ModelsUsed)
	}
}

// TestCodexAggregation_CodeActivityAcrossSubagents asserts apply_patch
// outcomes aggregate across rollouts (lines added, files modified).
func TestCodexAggregation_CodeActivityAcrossSubagents(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	rollouts := []*codex.ParsedRollout{
		{
			Turns: []codex.Turn{{
				TurnID: "t-main",
				ToolCalls: []codex.ToolCall{
					{Name: "apply_patch", Arguments: "*** Begin Patch\n*** Add File: a.go\n+package a\n*** End Patch", Status: "completed", Timestamp: t0},
				},
			}},
		},
		{
			Turns: []codex.Turn{{
				TurnID: "t-sub",
				ToolCalls: []codex.ToolCall{
					{Name: "apply_patch", Arguments: "*** Begin Patch\n*** Add File: b.py\n+import os\n*** End Patch", Status: "completed", Timestamp: t0.Add(10 * time.Second)},
				},
			}},
		},
	}
	out := ComputeFromCodexRollout(context.Background(), rollouts)
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.FilesModified != 2 {
		t.Errorf("FilesModified = %d, want 2 (main + sub each added one file)", out.FilesModified)
	}
	if out.LinesAdded != 2 {
		t.Errorf("LinesAdded = %d, want 2 (one line per rollout)", out.LinesAdded)
	}
	if out.LanguageBreakdown["go"] != 1 {
		t.Errorf("LanguageBreakdown[go] = %d, want 1", out.LanguageBreakdown["go"])
	}
	if out.LanguageBreakdown["python"] != 1 && out.LanguageBreakdown["py"] != 1 {
		t.Errorf("LanguageBreakdown missing python: %v", out.LanguageBreakdown)
	}
}

// TestCodexAggregation_RedactionsAcrossSubagents asserts redaction counts
// sum across rollouts and per-category counts merge.
func TestCodexAggregation_RedactionsAcrossSubagents(t *testing.T) {
	rollouts := []*codex.ParsedRollout{
		{
			Turns: []codex.Turn{{
				AssistantMessages: []codex.Message{{Role: "assistant", Text: "email [REDACTED:EMAIL]"}},
			}},
		},
		{
			Turns: []codex.Turn{{
				ToolCalls: []codex.ToolCall{
					{Name: "exec_command", Output: "found [REDACTED:API_KEY] in env"},
				},
			}},
		},
	}
	out := ComputeFromCodexRollout(context.Background(), rollouts)
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.TotalRedactions != 2 {
		t.Errorf("TotalRedactions = %d, want 2 (one per rollout)", out.TotalRedactions)
	}
	if out.RedactionCounts["EMAIL"] != 1 || out.RedactionCounts["API_KEY"] != 1 {
		t.Errorf("RedactionCounts = %v, want EMAIL=1 and API_KEY=1", out.RedactionCounts)
	}
}

// TestCodexAggregation_ConversationCardExcludesSubagents pins the explicit
// per-card asymmetry: Conversation timing + turn counts reflect only the
// main rollout. Even when subagents add user/assistant turns, the
// Conversation card stays anchored to the main rollout's structure.
func TestCodexAggregation_ConversationCardExcludesSubagents(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	rollouts := []*codex.ParsedRollout{
		rolloutWithUserMessage("gpt-5", "main user", "exec_command", t0),
		// Subagent adds another user/assistant pair — must NOT count.
		rolloutWithUserMessage("gpt-5", "sub user", "exec_command", t0.Add(10*time.Second)),
	}
	out := ComputeFromCodexRollout(context.Background(), rollouts)
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.UserTurns != 1 {
		t.Errorf("UserTurns = %d, want 1 (subagent excluded from Conversation card)", out.UserTurns)
	}
	if out.AssistantTurns != 1 {
		t.Errorf("AssistantTurns = %d, want 1 (subagent excluded from Conversation card)", out.AssistantTurns)
	}
}

// TestCodexAggregation_TranscriptInlinesSubagents asserts the smart recap
// transcript emits subagent turns inline after main turns (mirrors Claude's
// TranscriptBuilder.ProcessFile per-file appending).
func TestCodexAggregation_TranscriptInlinesSubagents(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	rollouts := []*codex.ParsedRollout{
		rolloutWithUserMessage("gpt-5", "MAIN_USER_MARKER", "exec_command", t0),
		rolloutWithUserMessage("gpt-5", "SUBAGENT_USER_MARKER", "exec_command", t0.Add(10*time.Second)),
	}
	xml, _ := PrepareCodexTranscript(rollouts)

	mainIdx := strings.Index(xml, "MAIN_USER_MARKER")
	subIdx := strings.Index(xml, "SUBAGENT_USER_MARKER")
	if mainIdx == -1 {
		t.Errorf("transcript missing MAIN_USER_MARKER: %s", xml)
	}
	if subIdx == -1 {
		t.Errorf("transcript missing SUBAGENT_USER_MARKER (subagent not appended): %s", xml)
	}
	if subIdx <= mainIdx {
		t.Errorf("subagent marker (%d) must appear AFTER main marker (%d) in transcript", subIdx, mainIdx)
	}
	if strings.Contains(xml, "<subagent") {
		t.Errorf("transcript must NOT wrap subagent turns in <subagent> tags; expected inline shape. Got: %s", xml)
	}
}

// TestCodexAggregation_SearchIndexIncludesSubagents asserts the Weight C
// search-index content includes subagent user messages, assistant final text,
// and tool call summaries.
func TestCodexAggregation_SearchIndexIncludesSubagents(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	rollouts := []*codex.ParsedRollout{
		rolloutWithUserMessage("gpt-5", "MAIN_USER_MARKER", "exec_command", t0),
		rolloutWithUserMessage("gpt-5", "SUBAGENT_USER_MARKER", "exec_command", t0.Add(10*time.Second)),
	}
	text := ExtractCodexUserMessagesText(rollouts)
	if !strings.Contains(text, "MAIN_USER_MARKER") {
		t.Errorf("search index missing main user marker: %q", text)
	}
	if !strings.Contains(text, "SUBAGENT_USER_MARKER") {
		t.Errorf("search index missing subagent user marker (subagent not included): %q", text)
	}
}

// TestCodexAggregation_ValidationErrorCountSums asserts the validation
// error count surfaces the union across main + subagent rollouts so
// operators see all parse anomalies in the frontend counter.
func TestCodexAggregation_ValidationErrorCountSums(t *testing.T) {
	rollouts := []*codex.ParsedRollout{
		{ValidationErrors: []codex.ValidationError{{Line: 1, Reason: "main bad"}}},
		{ValidationErrors: []codex.ValidationError{{Line: 1, Reason: "subagent A bad"}, {Line: 2, Reason: "subagent A bad #2"}}},
	}
	out := ComputeFromCodexRollout(context.Background(), rollouts)
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.ValidationErrorCount != 3 {
		t.Errorf("ValidationErrorCount = %d, want 3 (main 1 + subagent 2)", out.ValidationErrorCount)
	}
}

// TestCodexAggregation_EmptySliceProducesEmptyResult asserts a nil/empty
// slice yields a zero-valued ComputeResult, never a nil pointer.
func TestCodexAggregation_EmptySliceProducesEmptyResult(t *testing.T) {
	out := ComputeFromCodexRollout(context.Background(), nil)
	if out == nil {
		t.Fatal("ComputeFromCodexRollout(context.Background(), nil) returned nil pointer; expected empty ComputeResult")
	}
	if out.UserMessages != 0 || out.TotalToolCalls != 0 {
		t.Errorf("expected zero counts on empty input, got users=%d tools=%d", out.UserMessages, out.TotalToolCalls)
	}
	out = ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout(context.Background(), empty slice) returned nil pointer; expected empty ComputeResult")
	}
}
