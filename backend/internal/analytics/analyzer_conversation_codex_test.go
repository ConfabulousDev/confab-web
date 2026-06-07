package analytics

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// Spec tests for CF-441: Conversation card timing for Codex sessions.
//
// These tests drive computeCodexConversation through the public
// ComputeFromCodexRollout orchestrator, so they survive the per-card file
// reorg (Phase 4a) without modification.

func ts(year int, month time.Month, day, hour, min, sec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, 0, time.UTC)
}

// TestComputeCodexConversation_HappyPath
//
// Two user prompts at t=0s and t=60s. Assistant replies at t=10s, t=20s
// (window 1 closes), and t=70s (trailing window 2).
//
// Expected windows:
//   - window 1: user@0  → lastAsst@20  → duration 20s
//   - window 2: user@60 → lastAsst@70  → duration 10s (trailing)
//
// userThinking windows:
//   - between prompt 1 close (asst@20) and prompt 2 (user@60) → 40s
//
// utilization = 30000 / (30000 + 40000) * 100 ≈ 42.857
func TestComputeCodexConversation_HappyPath(t *testing.T) {
	base := ts(2026, 5, 17, 0, 0, 0)
	rollout := &codex.ParsedRollout{
		Turns: []codex.Turn{{
			TurnID: "t1",
			UserMessages: []codex.Message{
				{Role: "user", Text: "hi", Timestamp: base},
				{Role: "user", Text: "more", Timestamp: base.Add(60 * time.Second)},
			},
			AssistantMessages: []codex.Message{
				{Role: "assistant", Phase: "final", Text: "ok1", Timestamp: base.Add(10 * time.Second)},
				{Role: "assistant", Phase: "final", Text: "ok2", Timestamp: base.Add(20 * time.Second)},
				{Role: "assistant", Phase: "final", Text: "ok3", Timestamp: base.Add(70 * time.Second)},
			},
		}},
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	assertI64Ptr(t, "TotalAssistantDurationMs", out.TotalAssistantDurationMs, 30000)
	assertI64Ptr(t, "AvgAssistantTurnMs", out.AvgAssistantTurnMs, 15000)
	assertI64Ptr(t, "TotalUserDurationMs", out.TotalUserDurationMs, 40000)
	assertI64Ptr(t, "AvgUserThinkingMs", out.AvgUserThinkingMs, 40000)

	if out.AssistantUtilizationPct == nil {
		t.Fatalf("AssistantUtilizationPct nil; want ≈42.857")
	}
	if math.Abs(*out.AssistantUtilizationPct-42.857142857142854) > 0.001 {
		t.Errorf("AssistantUtilizationPct = %v, want ≈42.857", *out.AssistantUtilizationPct)
	}
}

// TestComputeCodexConversation_NoAssistant
//
// One user prompt, zero assistant messages. No window ever closes → all five
// timing fields must remain nil.
func TestComputeCodexConversation_NoAssistant(t *testing.T) {
	rollout := &codex.ParsedRollout{
		Turns: []codex.Turn{{
			TurnID: "t1",
			UserMessages: []codex.Message{
				{Role: "user", Text: "hi", Timestamp: ts(2026, 5, 17, 0, 0, 0)},
			},
		}},
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	assertNilI64(t, "TotalAssistantDurationMs", out.TotalAssistantDurationMs)
	assertNilI64(t, "AvgAssistantTurnMs", out.AvgAssistantTurnMs)
	assertNilI64(t, "TotalUserDurationMs", out.TotalUserDurationMs)
	assertNilI64(t, "AvgUserThinkingMs", out.AvgUserThinkingMs)
	if out.AssistantUtilizationPct != nil {
		t.Errorf("AssistantUtilizationPct = %v, want nil", *out.AssistantUtilizationPct)
	}
}

// TestComputeCodexConversation_TrailingAssistant
//
// One user prompt + one assistant reply. No subsequent user prompt → the
// trailing assistant window closes, but user thinking has no samples (only
// the first prompt exists; user thinking starts on the second prompt).
//
// Expected: assistant total/avg populated; user nil; utilization nil.
func TestComputeCodexConversation_TrailingAssistant(t *testing.T) {
	base := ts(2026, 5, 17, 0, 0, 0)
	rollout := &codex.ParsedRollout{
		Turns: []codex.Turn{{
			TurnID: "t1",
			UserMessages: []codex.Message{
				{Role: "user", Text: "hi", Timestamp: base},
			},
			AssistantMessages: []codex.Message{
				{Role: "assistant", Phase: "final", Text: "ok", Timestamp: base.Add(15 * time.Second)},
			},
		}},
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	assertI64Ptr(t, "TotalAssistantDurationMs", out.TotalAssistantDurationMs, 15000)
	assertI64Ptr(t, "AvgAssistantTurnMs", out.AvgAssistantTurnMs, 15000)
	assertNilI64(t, "TotalUserDurationMs", out.TotalUserDurationMs)
	assertNilI64(t, "AvgUserThinkingMs", out.AvgUserThinkingMs)
	if out.AssistantUtilizationPct != nil {
		t.Errorf("AssistantUtilizationPct = %v, want nil (user side missing)", *out.AssistantUtilizationPct)
	}
}

// TestComputeCodexConversation_MidStreamPromptsAcrossTurns
//
// Two Codex Turns. First turn has a user prompt and an assistant reply.
// Second turn has another user prompt mid-stream and another assistant reply.
// The walk is timestamp-driven, so the window math straddles turn boundaries
// cleanly.
func TestComputeCodexConversation_MidStreamPromptsAcrossTurns(t *testing.T) {
	base := ts(2026, 5, 17, 0, 0, 0)
	rollout := &codex.ParsedRollout{
		Turns: []codex.Turn{
			{
				TurnID: "t1",
				UserMessages: []codex.Message{
					{Role: "user", Text: "first", Timestamp: base},
				},
				AssistantMessages: []codex.Message{
					{Role: "assistant", Phase: "final", Text: "r1", Timestamp: base.Add(5 * time.Second)},
				},
			},
			{
				TurnID: "t2",
				UserMessages: []codex.Message{
					{Role: "user", Text: "second", Timestamp: base.Add(30 * time.Second)},
				},
				AssistantMessages: []codex.Message{
					{Role: "assistant", Phase: "final", Text: "r2", Timestamp: base.Add(40 * time.Second)},
				},
			},
		},
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	// Window 1: user@0 → asst@5  → duration 5s
	// Window 2 (trailing): user@30 → asst@40 → duration 10s
	// asstDurs = [5s, 10s] → total 15000, avg 7500
	assertI64Ptr(t, "TotalAssistantDurationMs", out.TotalAssistantDurationMs, 15000)
	assertI64Ptr(t, "AvgAssistantTurnMs", out.AvgAssistantTurnMs, 7500)

	// userThinking: between asst@5 (last asst before user@30) and user@30 → 25s.
	// Only one sample → total 25000, avg 25000.
	assertI64Ptr(t, "TotalUserDurationMs", out.TotalUserDurationMs, 25000)
	assertI64Ptr(t, "AvgUserThinkingMs", out.AvgUserThinkingMs, 25000)
}

// TestComputeCodexConversation_ZeroTimestampSkipped
//
// Codex zero-ts messages (parse failure on the timestamp field) are skipped
// entirely — they contribute neither to timing samples nor to state resets.
// This diverges from Claude's reset semantic (see implementation comment in
// analyzer_conversation_codex.go for rationale).
//
// Sequence:
//   - user@0, asst@10 (valid window 1)
//   - user{zero ts}, asst{zero ts}    (both skipped)
//   - user@30, asst@40 (valid window 2)
//
// Effective walk (zero-ts dropped): user@0, asst@10, user@30, asst@40
//   - window 1 closes when user@30 arrives → asstDurs += 10s
//   - userThinking: 30 - 10 = 20s → userDurs += 20s
//   - trailing window 2: asst@40 - user@30 = 10s → asstDurs += 10s
//
// Expected: asstDurs=[10s, 10s] → total 20s, avg 10s. userDurs=[20s].
func TestComputeCodexConversation_ZeroTimestampSkipped(t *testing.T) {
	base := ts(2026, 5, 17, 0, 0, 0)
	rollout := &codex.ParsedRollout{
		Turns: []codex.Turn{{
			TurnID: "t1",
			UserMessages: []codex.Message{
				{Role: "user", Text: "first", Timestamp: base},
				{Role: "user", Text: "bad-ts", Timestamp: time.Time{}},
				{Role: "user", Text: "second", Timestamp: base.Add(30 * time.Second)},
			},
			AssistantMessages: []codex.Message{
				{Role: "assistant", Phase: "final", Text: "r1", Timestamp: base.Add(10 * time.Second)},
				{Role: "assistant", Phase: "final", Text: "r-skip", Timestamp: time.Time{}},
				{Role: "assistant", Phase: "final", Text: "r2", Timestamp: base.Add(40 * time.Second)},
			},
		}},
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	assertI64Ptr(t, "TotalAssistantDurationMs", out.TotalAssistantDurationMs, 20000)
	assertI64Ptr(t, "AvgAssistantTurnMs", out.AvgAssistantTurnMs, 10000)
	assertI64Ptr(t, "TotalUserDurationMs", out.TotalUserDurationMs, 20000)
	assertI64Ptr(t, "AvgUserThinkingMs", out.AvgUserThinkingMs, 20000)
}

// TestComputeCodexConversation_ReasoningExtendsAssistant
//
// User@0, assistant text at t=10s, ReasoningCount=3, CompletedAt=t=30s.
// Next user at t=60s.
//
// The synthetic assistant event at CompletedAt=30s extends lastAsstTs past
// the visible assistant message. The assistant window for user@0 should
// close at t=30s, not t=10s.
//
// Expected:
//   - asst duration = 30000 (30s - 0s)
//   - user thinking = 30000 (60s - 30s)
func TestComputeCodexConversation_ReasoningExtendsAssistant(t *testing.T) {
	base := ts(2026, 5, 17, 0, 0, 0)
	completed := base.Add(30 * time.Second)
	rollout := &codex.ParsedRollout{
		Turns: []codex.Turn{
			{
				TurnID:         "t1",
				CompletedAt:    &completed,
				ReasoningCount: 3,
				UserMessages: []codex.Message{
					{Role: "user", Text: "hi", Timestamp: base},
				},
				AssistantMessages: []codex.Message{
					{Role: "assistant", Phase: "final", Text: "ok", Timestamp: base.Add(10 * time.Second)},
				},
			},
			{
				TurnID: "t2",
				UserMessages: []codex.Message{
					{Role: "user", Text: "next", Timestamp: base.Add(60 * time.Second)},
				},
			},
		},
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	assertI64Ptr(t, "TotalAssistantDurationMs (reasoning extends to CompletedAt)", out.TotalAssistantDurationMs, 30000)
	assertI64Ptr(t, "AvgAssistantTurnMs", out.AvgAssistantTurnMs, 30000)
	assertI64Ptr(t, "TotalUserDurationMs (user@60 - asst@30)", out.TotalUserDurationMs, 30000)
	assertI64Ptr(t, "AvgUserThinkingMs", out.AvgUserThinkingMs, 30000)
}

// TestComputeCodexConversation_ReasoningWithoutCompletedAt
//
// Same shape as the previous test but Turn.CompletedAt = nil (e.g. session
// ended mid-turn). No synthetic event is emitted; reasoning does not extend
// the assistant window.
//
// Expected: assistant duration = 10s (visible asst only).
func TestComputeCodexConversation_ReasoningWithoutCompletedAt(t *testing.T) {
	base := ts(2026, 5, 17, 0, 0, 0)
	rollout := &codex.ParsedRollout{
		Turns: []codex.Turn{
			{
				TurnID:         "t1",
				CompletedAt:    nil, // explicit
				ReasoningCount: 3,
				UserMessages: []codex.Message{
					{Role: "user", Text: "hi", Timestamp: base},
				},
				AssistantMessages: []codex.Message{
					{Role: "assistant", Phase: "final", Text: "ok", Timestamp: base.Add(10 * time.Second)},
				},
			},
			{
				TurnID: "t2",
				UserMessages: []codex.Message{
					{Role: "user", Text: "next", Timestamp: base.Add(60 * time.Second)},
				},
			},
		},
	}
	out := ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{rollout})

	assertI64Ptr(t, "TotalAssistantDurationMs (no synthetic, asst stays at 10s)", out.TotalAssistantDurationMs, 10000)
	assertI64Ptr(t, "AvgAssistantTurnMs", out.AvgAssistantTurnMs, 10000)
	// user thinking = user@60 - asst@10 = 50s.
	assertI64Ptr(t, "TotalUserDurationMs", out.TotalUserDurationMs, 50000)
	assertI64Ptr(t, "AvgUserThinkingMs", out.AvgUserThinkingMs, 50000)
}

// ----------------------------------------------------------------------------
// Local helpers (kept in this file to make the spec self-contained).
// ----------------------------------------------------------------------------

func assertI64Ptr(t *testing.T, name string, got *int64, want int64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: got nil, want %d", name, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %d, want %d", name, *got, want)
	}
}

func assertNilI64(t *testing.T, name string, got *int64) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %d, want nil", name, *got)
	}
}
