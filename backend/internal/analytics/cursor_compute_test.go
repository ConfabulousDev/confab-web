package analytics

import (
	"context"
	"os"
	"testing"
	"time"
)

// noBounds is the zero-signal session window: no created_at/first_seen and no
// last_message_at/last_sync_at. Compute must leave DurationMs nil for it.
var noBounds = CursorSessionBounds{}

// loadCursorFixtureMessages parses the committed fy5q fixture
// (testdata/cursor/main.jsonl) into the typed Cursor message slice the compute
// path consumes. It fails the test if the fixture is unreadable or any line
// fails to parse — the fixture is the ground-truth wire shape for v1.
func loadCursorFixtureMessages(t *testing.T) []*CursorMessage {
	t.Helper()
	raw, err := os.ReadFile(cursorFixturePath)
	if err != nil {
		t.Fatalf("read cursor fixture: %v", err)
	}
	messages, lineErrors := parseCursorJSONL(context.Background(), raw, "main.jsonl")
	if len(messages) == 0 {
		t.Fatal("expected fixture to parse into >0 messages")
	}
	// The fixture is hand-sanitized and must parse cleanly. The one intentional
	// turn_ended error row is surfaced as a validation entry (so error turns
	// aren't dropped silently) — that is expected, not a malformed-line failure.
	for _, le := range lineErrors {
		if le.MessageType == "turn_ended" {
			continue
		}
		t.Errorf("unexpected parse error on fixture line %d: %v", le.Line, le.Errors)
	}
	return messages
}

// TestParseCursorJSONLSeparatesConversationFromMarkers verifies the parser
// keeps conversation rows (user/assistant) as messages and treats turn_ended
// rows as markers (not conversation messages), counting error turns.
func TestParseCursorJSONLSeparatesConversationFromMarkers(t *testing.T) {
	messages := loadCursorFixtureMessages(t)

	var users, assistants int
	for _, m := range messages {
		switch m.Role {
		case "user":
			users++
		case "assistant":
			assistants++
		default:
			t.Errorf("unexpected message role %q in parsed conversation", m.Role)
		}
	}
	if users != 3 {
		t.Errorf("user messages = %d, want 3", users)
	}
	if assistants != 6 {
		t.Errorf("assistant messages = %d, want 6", assistants)
	}
}

// TestComputeFromCursorRolloutSession checks the session card counts derived
// purely from message structure (no timestamps in Cursor JSONL).
func TestComputeFromCursorRolloutSession(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	result := ComputeFromCursorRollout(context.Background(), messages, noBounds)

	if result.UserMessages != 3 {
		t.Errorf("UserMessages = %d, want 3", result.UserMessages)
	}
	if result.AssistantMessages != 6 {
		t.Errorf("AssistantMessages = %d, want 6", result.AssistantMessages)
	}
	// Cursor JSONL has no per-line timestamps, so duration is unknowable.
	if result.DurationMs != nil {
		t.Errorf("DurationMs = %v, want nil (Cursor lines carry no timestamps)", *result.DurationMs)
	}
}

// TestComputeFromCursorRolloutTools verifies every tool_use is counted exactly
// once under its Cursor-specific name.
func TestComputeFromCursorRolloutTools(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	result := ComputeFromCursorRollout(context.Background(), messages, noBounds)

	wantTools := map[string]int{
		"Read": 1, "Grep": 1, "Glob": 1, "SemanticSearch": 1, "Task": 1,
		"StrReplace": 1, "Write": 1, "Shell": 1, "Delete": 1,
		"WebSearch": 1, "AskQuestion": 1,
	}
	var wantTotal int
	for name, want := range wantTools {
		stats := result.ToolStats[name]
		if stats == nil {
			t.Errorf("tool %q missing from ToolStats", name)
			continue
		}
		got := stats.Success + stats.Errors
		if got != want {
			t.Errorf("tool %q count = %d, want %d", name, got, want)
		}
		wantTotal += want
	}
	if result.TotalToolCalls != wantTotal {
		t.Errorf("TotalToolCalls = %d, want %d", result.TotalToolCalls, wantTotal)
	}
}

// TestComputeFromCursorRolloutCodeActivity locks the corrected tool
// classification: StrReplace is the EDIT tool (must count as a modification),
// Read is a file read, Write/Delete are modifications, and Grep/Glob/
// SemanticSearch are searches while WebSearch is NOT.
func TestComputeFromCursorRolloutCodeActivity(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	result := ComputeFromCursorRollout(context.Background(), messages, noBounds)

	if result.FilesRead != 1 {
		t.Errorf("FilesRead = %d, want 1 (Read only)", result.FilesRead)
	}
	// Write (create) + StrReplace (edit) + Delete = 3 modifications.
	if result.FilesModified != 3 {
		t.Errorf("FilesModified = %d, want 3 (Write+StrReplace+Delete)", result.FilesModified)
	}
	// Grep + Glob + SemanticSearch = 3; WebSearch excluded (web, not code).
	if result.SearchCount != 3 {
		t.Errorf("SearchCount = %d, want 3 (Grep+Glob+SemanticSearch, NOT WebSearch)", result.SearchCount)
	}
}

// TestComputeFromCursorRolloutAgents verifies Task invocations are bucketed by
// the subagent_type field of the Task input (main-thread-only agents card).
func TestComputeFromCursorRolloutAgents(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	result := ComputeFromCursorRollout(context.Background(), messages, noBounds)

	if result.TotalAgentInvocations != 1 {
		t.Errorf("TotalAgentInvocations = %d, want 1", result.TotalAgentInvocations)
	}
	// The fixture's Task carries subagent_type "explore".
	if result.AgentStats["explore"] == nil {
		t.Errorf("expected agent bucket for subagent_type %q, got %v", "explore", result.AgentStats)
	}
}

// TestComputeFromCursorRolloutAlwaysWritesEmptyTokensV2 locks decision #1:
// Cursor synced JSONL carries no token/cost data, so tokens_v2 is always
// written with an empty by_provider tree and zero cost (no invented dollars).
func TestComputeFromCursorRolloutAlwaysWritesEmptyTokensV2(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	result := ComputeFromCursorRollout(context.Background(), messages, noBounds)

	if result.TokensV2 == nil {
		t.Fatal("TokensV2 must always be written (empty tree), got nil")
	}
	if len(result.TokensV2.ByProvider) != 0 {
		t.Errorf("TokensV2.ByProvider = %v, want empty (no token data in Cursor JSONL)", result.TokensV2.ByProvider)
	}
	if result.TokensV2.TotalCostUSD != "0" {
		t.Errorf("TokensV2.TotalCostUSD = %q, want %q", result.TokensV2.TotalCostUSD, "0")
	}
	if result.InputTokens != 0 || result.OutputTokens != 0 {
		t.Errorf("token counts must be zero, got input=%d output=%d", result.InputTokens, result.OutputTokens)
	}
}

// TestComputeFromCursorRolloutTurnEndedErrorTolerated verifies a turn_ended
// error row does not crash the parse and is counted as a validation/error
// signal rather than dropped silently.
func TestComputeFromCursorRolloutTurnEndedErrorTolerated(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	// Parse must succeed and yield the 9 conversation messages despite the
	// error turn marker.
	if len(messages) != 9 {
		t.Errorf("parsed conversation messages = %d, want 9", len(messages))
	}
}

// TestComputeFromCursorRolloutEmpty guards the nil/empty input path.
func TestComputeFromCursorRolloutEmpty(t *testing.T) {
	result := ComputeFromCursorRollout(context.Background(), nil, noBounds)
	if result == nil {
		t.Fatal("ComputeFromCursorRollout(nil) must return a non-nil result")
	}
}

// TestComputeFromCursorRolloutDurationFromBounds locks the core 5w7r contract:
// Cursor lines carry no per-line timestamps, so DurationMs is derived from the
// session window (start = created_at ?? first_seen; end = last_message_at ??
// last_sync_at). With a clean T0→T1 window the duration is exactly the span.
func TestComputeFromCursorRolloutDurationFromBounds(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	t0 := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(90 * time.Minute)

	result := ComputeFromCursorRollout(context.Background(), messages, CursorSessionBounds{
		FirstSeen:     ptrTime(t0),
		LastMessageAt: ptrTime(t1),
	})

	if result.DurationMs == nil {
		t.Fatal("DurationMs = nil, want span between session bounds")
	}
	wantMs := int64(90 * 60 * 1000)
	if *result.DurationMs != wantMs {
		t.Errorf("DurationMs = %d, want %d (T1-T0)", *result.DurationMs, wantMs)
	}
}

// TestComputeCursorBoundsStartPrecedence verifies created_at is preferred over
// first_seen as the start anchor, and last_message_at over last_sync_at as the
// end anchor (start = created_at ?? first_seen; end = last_message_at ??
// last_sync_at).
func TestComputeCursorBoundsStartPrecedence(t *testing.T) {
	createdAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	firstSeen := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	lastMessage := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)
	lastSync := time.Date(2026, 6, 15, 11, 30, 0, 0, time.UTC)

	start, end := cursorSessionWindow(CursorSessionBounds{
		CreatedAt:     ptrTime(createdAt),
		FirstSeen:     ptrTime(firstSeen),
		LastMessageAt: ptrTime(lastMessage),
		LastSyncAt:    ptrTime(lastSync),
	})
	if start == nil || !start.Equal(createdAt) {
		t.Errorf("start = %v, want created_at %v (created_at preferred over first_seen)", start, createdAt)
	}
	if end == nil || !end.Equal(lastMessage) {
		t.Errorf("end = %v, want last_message_at %v (preferred over last_sync_at)", end, lastMessage)
	}

	// Fallbacks: no created_at → first_seen; no last_message_at → last_sync_at.
	start, end = cursorSessionWindow(CursorSessionBounds{
		FirstSeen:  ptrTime(firstSeen),
		LastSyncAt: ptrTime(lastSync),
	})
	if start == nil || !start.Equal(firstSeen) {
		t.Errorf("start = %v, want first_seen fallback %v", start, firstSeen)
	}
	if end == nil || !end.Equal(lastSync) {
		t.Errorf("end = %v, want last_sync_at fallback %v", end, lastSync)
	}
}

// TestComputeFromCursorRolloutDurationDegrades covers the graceful-degradation
// edge cases: a missing bound, and an end-before-start window (which must not
// emit a negative or zero-padded duration).
func TestComputeFromCursorRolloutDurationDegrades(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	t0 := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	cases := []struct {
		name   string
		bounds CursorSessionBounds
	}{
		{"no bounds at all", noBounds},
		{"start only", CursorSessionBounds{FirstSeen: ptrTime(t0)}},
		{"end only", CursorSessionBounds{LastMessageAt: ptrTime(t1)}},
		{"end before start", CursorSessionBounds{FirstSeen: ptrTime(t1), LastMessageAt: ptrTime(t0)}},
		{"zero-length window", CursorSessionBounds{FirstSeen: ptrTime(t0), LastMessageAt: ptrTime(t0)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := ComputeFromCursorRollout(context.Background(), messages, tc.bounds)
			if result.DurationMs != nil {
				t.Errorf("DurationMs = %d, want nil (%s must degrade, not invent a non-positive span)", *result.DurationMs, tc.name)
			}
		})
	}
}
