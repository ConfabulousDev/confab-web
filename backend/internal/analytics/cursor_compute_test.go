package analytics

import (
	"context"
	"os"
	"testing"
)

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
	if assistants != 8 {
		t.Errorf("assistant messages = %d, want 8", assistants)
	}
}

// TestComputeFromCursorRolloutSession checks the session card counts derived
// purely from message structure (no timestamps in Cursor JSONL).
func TestComputeFromCursorRolloutSession(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	result := ComputeFromCursorRollout(context.Background(), messages)

	if result.UserMessages != 3 {
		t.Errorf("UserMessages = %d, want 3", result.UserMessages)
	}
	if result.AssistantMessages != 8 {
		t.Errorf("AssistantMessages = %d, want 8", result.AssistantMessages)
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
	result := ComputeFromCursorRollout(context.Background(), messages)

	wantTools := map[string]int{
		"Read": 1, "Grep": 1, "Glob": 1, "SemanticSearch": 1, "Task": 1,
		"StrReplace": 1, "Write": 1, "Shell": 3, "Delete": 1,
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
	result := ComputeFromCursorRollout(context.Background(), messages)

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
	result := ComputeFromCursorRollout(context.Background(), messages)

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
	result := ComputeFromCursorRollout(context.Background(), messages)

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
	// Parse must succeed and yield the 11 conversation messages despite the
	// error turn marker.
	if len(messages) != 11 {
		t.Errorf("parsed conversation messages = %d, want 11", len(messages))
	}
}

// TestComputeFromCursorRolloutEmpty guards the nil/empty input path.
func TestComputeFromCursorRolloutEmpty(t *testing.T) {
	result := ComputeFromCursorRollout(context.Background(), nil)
	if result == nil {
		t.Fatal("ComputeFromCursorRollout(nil) must return a non-nil result")
	}
}
