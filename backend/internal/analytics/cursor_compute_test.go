package analytics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// noBounds is the zero-signal session window: no created_at/first_seen and no
// last_message_at/last_sync_at. Compute must leave DurationMs nil for it.
var noBounds = CursorSessionBounds{}

// mainOnly wraps a single main-thread message slice in the [main, ...subagents]
// rollout-slice convention ComputeFromCursorRollout consumes — the common case
// in the structure tests, which exercise the main thread alone.
func mainOnly(messages []*CursorMessage) [][]*CursorMessage {
	return [][]*CursorMessage{messages}
}

// loadCursorSubagentMessages parses the committed subagent fixture
// (testdata/cursor/subagent.jsonl) — a sanitized Cursor subagent transcript
// with the identical envelope as the main thread plus the subagent-only
// UpdateCurrentStep tool. It fails the test on any unexpected parse error.
func loadCursorSubagentMessages(t *testing.T) []*CursorMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cursor", "subagent.jsonl"))
	if err != nil {
		t.Fatalf("read cursor subagent fixture: %v", err)
	}
	messages, lineErrors := parseCursorJSONL(context.Background(), raw, "subagent.jsonl")
	if len(messages) == 0 {
		t.Fatal("expected subagent fixture to parse into >0 messages")
	}
	for _, le := range lineErrors {
		if le.MessageType == "turn_ended" {
			continue
		}
		t.Errorf("unexpected parse error on subagent fixture line %d: %v", le.Line, le.Errors)
	}
	return messages
}

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
	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), noBounds)

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
	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), noBounds)

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
	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), noBounds)

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
	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), noBounds)

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
	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), noBounds)

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

	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), CursorSessionBounds{
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

// TestComputeFromCursorRolloutSessionModelsUsedNotNull is the y0kc regression:
// Cursor's session card must marshal models_used as a JSON array ([]), never
// null. computeCursorSession never populates per-line models (Cursor JSONL has
// none in v1), but leaving ModelsUsed nil marshals to JSON null, which the
// frontend's required SessionCardDataSchema.models_used (z.array(z.string()))
// rejects — breaking the whole Summary analytics load for every Cursor session.
//
// This drives the full production wire path the cd3z HTTP test missed:
// ComputeFromCursorRollout -> ToCards -> ToResponse -> json.Marshal, then
// inspects the raw JSON to assert the array shape (not a Go-level nil check,
// which Go-side []string{} vs nil both satisfy — only the wire shape differs).
func TestComputeFromCursorRolloutSessionModelsUsedNotNull(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), noBounds)

	cards := result.ToCards("test-session", int64(len(messages)))
	response := cards.ToResponse()

	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal analytics response: %v", err)
	}

	var decoded struct {
		Cards struct {
			Session map[string]json.RawMessage `json:"session"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal analytics response: %v", err)
	}

	got, ok := decoded.Cards.Session["models_used"]
	if !ok {
		t.Fatal("cards.session.models_used key missing from response")
	}
	if string(got) != "[]" {
		t.Errorf("cards.session.models_used = %s, want [] (must be a JSON array, never null — frontend SessionCardDataSchema rejects null)", string(got))
	}
}

// TestComputeFromCursorRolloutModelPopulatesModelsUsed is the zsr6 contract:
// when the per-session model recovered from the cursor_session_meta sidecar is
// threaded into bounds, the session card's models_used must surface exactly that
// single model. Cursor JSONL has no per-line model, so this is the only source.
func TestComputeFromCursorRolloutModelPopulatesModelsUsed(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	bounds := CursorSessionBounds{Model: "composer-2.5"}

	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), bounds)

	if got := result.ModelsUsed; len(got) != 1 || got[0] != "composer-2.5" {
		t.Errorf("ModelsUsed = %v, want [composer-2.5]", got)
	}
}

// TestComputeFromCursorRolloutNoModelLeavesModelsEmpty is the absent-metadata
// regression: with no sidecar model, models_used stays an empty (non-nil) slice
// — never an invented model, never null (y0kc).
func TestComputeFromCursorRolloutNoModelLeavesModelsEmpty(t *testing.T) {
	messages := loadCursorFixtureMessages(t)

	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), noBounds)

	if result.ModelsUsed == nil {
		t.Fatal("ModelsUsed = nil, want non-nil empty slice (must marshal as [], not null)")
	}
	if len(result.ModelsUsed) != 0 {
		t.Errorf("ModelsUsed = %v, want [] (no model in bounds → no invented model)", result.ModelsUsed)
	}
}

// TestComputeFromCursorRolloutEmptyModelLeavesModelsEmpty guards the empty-string
// edge: a blank model (should never reach here — the handler skips empty) still
// yields [], never [""].
func TestComputeFromCursorRolloutEmptyModelLeavesModelsEmpty(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	bounds := CursorSessionBounds{Model: ""}

	result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), bounds)

	if len(result.ModelsUsed) != 0 {
		t.Errorf("ModelsUsed = %v, want [] for an empty model", result.ModelsUsed)
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
			result := ComputeFromCursorRollout(context.Background(), mainOnly(messages), tc.bounds)
			if result.DurationMs != nil {
				t.Errorf("DurationMs = %d, want nil (%s must degrade, not invent a non-positive span)", *result.DurationMs, tc.name)
			}
		})
	}
}

// =============================================================================
// Subagent rollouts (wc9t) — analytics aggregation across [main, ...subagents]
// =============================================================================

// TestComputeFromCursorRolloutMergesSubagentTools is the wc9t core contract:
// tool stats merge across main + subagent rollouts. The subagent fixture adds
// Glob×2, Read×2, Grep×1 on top of the main fixture's tools; the merged
// ToolStats must equal the sum. (UpdateCurrentStep is NOT a tool — see the
// dedicated test below.)
func TestComputeFromCursorRolloutMergesSubagentTools(t *testing.T) {
	main := loadCursorFixtureMessages(t)
	sub := loadCursorSubagentMessages(t)

	mainOnlyResult := ComputeFromCursorRollout(context.Background(), mainOnly(main), noBounds)
	merged := ComputeFromCursorRollout(context.Background(), [][]*CursorMessage{main, sub}, noBounds)

	// The subagent contributes Glob×2, Read×2, Grep×1.
	wantDelta := map[string]int{"Glob": 2, "Read": 2, "Grep": 1}
	for name, delta := range wantDelta {
		gotMain := toolCount(mainOnlyResult, name)
		gotMerged := toolCount(merged, name)
		if gotMerged != gotMain+delta {
			t.Errorf("tool %q merged count = %d, want %d (main %d + subagent %d)", name, gotMerged, gotMain+delta, gotMain, delta)
		}
	}

	// TotalToolCalls must grow by exactly the subagent's real tool count (5),
	// never by the two UpdateCurrentStep markers.
	if merged.TotalToolCalls != mainOnlyResult.TotalToolCalls+5 {
		t.Errorf("TotalToolCalls = %d, want %d (main %d + 5 subagent tools)", merged.TotalToolCalls, mainOnlyResult.TotalToolCalls+5, mainOnlyResult.TotalToolCalls)
	}
}

// TestComputeFromCursorRolloutIgnoresUpdateCurrentStep is decision D2: the
// subagent-only UpdateCurrentStep progress marker is neither counted as a tool
// nor surfaced in ToolStats. The subagent fixture has two of them.
func TestComputeFromCursorRolloutIgnoresUpdateCurrentStep(t *testing.T) {
	sub := loadCursorSubagentMessages(t)
	result := ComputeFromCursorRollout(context.Background(), [][]*CursorMessage{sub}, noBounds)

	if stats := result.ToolStats["UpdateCurrentStep"]; stats != nil {
		t.Errorf("UpdateCurrentStep must not appear in ToolStats, got %+v", stats)
	}
	// The subagent has 5 real tools (Glob×2, Read×2, Grep×1); the two
	// UpdateCurrentStep markers must not inflate the count.
	if result.TotalToolCalls != 5 {
		t.Errorf("TotalToolCalls = %d, want 5 (UpdateCurrentStep excluded)", result.TotalToolCalls)
	}
}

// TestComputeFromCursorRolloutMergesSubagentCodeActivity verifies file/search
// activity merges across rollouts. The subagent reads two files (Read×2) and
// runs three searches (Glob×2 + Grep×1).
func TestComputeFromCursorRolloutMergesSubagentCodeActivity(t *testing.T) {
	main := loadCursorFixtureMessages(t)
	sub := loadCursorSubagentMessages(t)

	mainOnlyResult := ComputeFromCursorRollout(context.Background(), mainOnly(main), noBounds)
	merged := ComputeFromCursorRollout(context.Background(), [][]*CursorMessage{main, sub}, noBounds)

	if merged.FilesRead != mainOnlyResult.FilesRead+2 {
		t.Errorf("FilesRead = %d, want %d (main + 2 subagent reads)", merged.FilesRead, mainOnlyResult.FilesRead+2)
	}
	if merged.SearchCount != mainOnlyResult.SearchCount+3 {
		t.Errorf("SearchCount = %d, want %d (main + Glob×2 + Grep×1)", merged.SearchCount, mainOnlyResult.SearchCount+3)
	}
}

// TestComputeFromCursorRolloutConversationMainOnly locks the asymmetric merge:
// the conversation card (turn counts) reflects the MAIN thread only — subagent
// turns do not widen the user-perceived conversation (D3 / OpenCode parity).
func TestComputeFromCursorRolloutConversationMainOnly(t *testing.T) {
	main := loadCursorFixtureMessages(t)
	sub := loadCursorSubagentMessages(t)

	mainOnlyResult := ComputeFromCursorRollout(context.Background(), mainOnly(main), noBounds)
	merged := ComputeFromCursorRollout(context.Background(), [][]*CursorMessage{main, sub}, noBounds)

	if merged.UserTurns != mainOnlyResult.UserTurns {
		t.Errorf("UserTurns = %d, want %d (main-only; subagents must not widen the conversation)", merged.UserTurns, mainOnlyResult.UserTurns)
	}
	if merged.AssistantTurns != mainOnlyResult.AssistantTurns {
		t.Errorf("AssistantTurns = %d, want %d (main-only)", merged.AssistantTurns, mainOnlyResult.AssistantTurns)
	}
	// The session card's message counts ARE main-only too (the session window
	// and turn structure mirror the main thread; subagents nest within it).
	if merged.UserMessages != mainOnlyResult.UserMessages {
		t.Errorf("UserMessages = %d, want %d (main-only session counts)", merged.UserMessages, mainOnlyResult.UserMessages)
	}
}

// TestComputeFromCursorRolloutBoundsMainOnly verifies DurationMs is derived from
// the main-thread session window only; subagents do not affect it (D3).
func TestComputeFromCursorRolloutBoundsMainOnly(t *testing.T) {
	main := loadCursorFixtureMessages(t)
	sub := loadCursorSubagentMessages(t)
	t0 := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	bounds := CursorSessionBounds{FirstSeen: ptrTime(t0), LastMessageAt: ptrTime(t1)}

	merged := ComputeFromCursorRollout(context.Background(), [][]*CursorMessage{main, sub}, bounds)

	if merged.DurationMs == nil {
		t.Fatal("DurationMs = nil, want the main-thread window span")
	}
	if *merged.DurationMs != int64(30*60*1000) {
		t.Errorf("DurationMs = %d, want %d (main-thread window only)", *merged.DurationMs, int64(30*60*1000))
	}
}

// TestComputeFromCursorRolloutMergesSubagentAgents verifies Task-derived agent
// invocations merge across rollouts. The main fixture has one Task (explore);
// the subagent fixture has none, so the merged total stays the main's.
func TestComputeFromCursorRolloutMergesSubagentAgents(t *testing.T) {
	main := loadCursorFixtureMessages(t)
	sub := loadCursorSubagentMessages(t)

	merged := ComputeFromCursorRollout(context.Background(), [][]*CursorMessage{main, sub}, noBounds)
	if merged.TotalAgentInvocations != 1 {
		t.Errorf("TotalAgentInvocations = %d, want 1 (main's Task; subagent has none)", merged.TotalAgentInvocations)
	}
}

// TestComputeFromCursorRolloutEmptyMain guards the empty-main path: no main
// thread means an empty result.
func TestComputeFromCursorRolloutEmptyMain(t *testing.T) {
	result := ComputeFromCursorRollout(context.Background(), [][]*CursorMessage{nil}, noBounds)
	if result == nil {
		t.Fatal("must return a non-nil result")
	}
	if result.TotalToolCalls != 0 {
		t.Errorf("TotalToolCalls = %d, want 0 for an empty main rollout", result.TotalToolCalls)
	}
}

// toolCount returns the total (success + error) calls recorded for a tool name,
// or 0 when the tool is absent.
func toolCount(r *ComputeResult, name string) int {
	if r == nil {
		return 0
	}
	stats := r.ToolStats[name]
	if stats == nil {
		return 0
	}
	return stats.Success + stats.Errors
}

// TestExtractCursorSearchTextIncludesSubagents is decision D1: subagent text
// feeds the global search index (recall), so a phrase that appears ONLY in the
// subagent transcript must be present in the extracted search text.
func TestExtractCursorSearchTextIncludesSubagents(t *testing.T) {
	main := loadCursorFixtureMessages(t)
	sub := loadCursorSubagentMessages(t)

	text := extractCursorSearchText([][]*CursorMessage{main, sub})

	const subagentOnlyPhrase = "router.go dispatch does not guard against an empty id"
	if !strings.Contains(text, subagentOnlyPhrase) {
		t.Errorf("search text missing subagent-only phrase %q", subagentOnlyPhrase)
	}
}
