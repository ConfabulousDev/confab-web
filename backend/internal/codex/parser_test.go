package codex

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadFixture returns the bytes of testdata/sample_rollout.jsonl.
func loadFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("testdata", "sample_rollout.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// truncateBeforeLastTaskComplete strips off everything from the final task_complete
// event onward — used to simulate a mid-turn upload.
func truncateBeforeLastTaskComplete(t *testing.T, raw []byte) []byte {
	t.Helper()
	var keep [][]byte
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if bytes.Contains(line, []byte(`"task_complete"`)) {
			// drop this and everything after — the truncation point.
			continue
		}
		// Copy because scanner reuses its buffer.
		c := make([]byte, len(line))
		copy(c, line)
		keep = append(keep, c)
	}
	return bytes.Join(keep, []byte("\n"))
}

func TestParseRollout_HappyPath(t *testing.T) {
	rollout, err := ParseRollout(bytes.NewReader(loadFixture(t)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if rollout == nil {
		t.Fatal("rollout is nil")
	}

	// Session metadata.
	if rollout.Model != "gpt-5" {
		t.Errorf("Model = %q, want %q", rollout.Model, "gpt-5")
	}
	if rollout.ModelProvider != "openai" {
		t.Errorf("ModelProvider = %q, want %q", rollout.ModelProvider, "openai")
	}
	if rollout.CWD != "/Users/dev/example-project" {
		t.Errorf("CWD = %q", rollout.CWD)
	}

	// Three explicit turns (the third carrying an inline-failed apply_patch
	// covering CF-438) plus a fourth implicit turn for the trailing orphan
	// function_call_output. Orphan outputs need a turn to land in, and the
	// rollout closed after the third task_complete.
	if len(rollout.Turns) != 4 {
		t.Fatalf("Turns count = %d, want 4 (3 completed + 1 implicit for orphan)", len(rollout.Turns))
	}

	turn1 := rollout.Turns[0]
	if turn1.TurnID != "019e-turn-0001" {
		t.Errorf("turn1.TurnID = %q", turn1.TurnID)
	}
	if turn1.Model != "gpt-5" {
		t.Errorf("turn1.Model = %q", turn1.Model)
	}
	if turn1.DurationMs == nil || *turn1.DurationMs != 11000 {
		t.Errorf("turn1.DurationMs = %v, want 11000", turn1.DurationMs)
	}
	if turn1.TimeToFirstTokenMs == nil || *turn1.TimeToFirstTokenMs != 1704 {
		t.Errorf("turn1.TimeToFirstTokenMs = %v, want 1704", turn1.TimeToFirstTokenMs)
	}
	// Developer message dropped, env-context-only message dropped: one real user
	// message remains ("add the linear mcp to my codex config").
	if len(turn1.UserMessages) != 1 {
		t.Errorf("turn1.UserMessages count = %d, want 1", len(turn1.UserMessages))
	} else if !strings.Contains(turn1.UserMessages[0].Text, "linear mcp") {
		t.Errorf("turn1.UserMessages[0].Text = %q", turn1.UserMessages[0].Text)
	}
	// One commentary + one final assistant message.
	if len(turn1.AssistantMessages) != 2 {
		t.Errorf("turn1.AssistantMessages count = %d, want 2", len(turn1.AssistantMessages))
	}
	// Two tool calls (exec_command, apply_patch).
	if len(turn1.ToolCalls) != 2 {
		t.Errorf("turn1.ToolCalls count = %d, want 2", len(turn1.ToolCalls))
	}
	// Reasoning item present in turn 1.
	if turn1.ReasoningCount != 1 {
		t.Errorf("turn1.ReasoningCount = %d, want 1", turn1.ReasoningCount)
	}

	// exec_command preamble parsed.
	var execCall *ToolCall
	for i := range turn1.ToolCalls {
		if turn1.ToolCalls[i].Name == "exec_command" {
			execCall = &turn1.ToolCalls[i]
			break
		}
	}
	if execCall == nil {
		t.Fatal("exec_command tool call not found in turn 1")
	}
	if execCall.ExitCode == nil || *execCall.ExitCode != 0 {
		t.Errorf("exec_command ExitCode = %v, want 0", execCall.ExitCode)
	}
	if execCall.WallTimeMs == nil || *execCall.WallTimeMs != 50 {
		t.Errorf("exec_command WallTimeMs = %v, want 50", execCall.WallTimeMs)
	}
	if execCall.Status != "completed" {
		t.Errorf("exec_command Status = %q, want completed", execCall.Status)
	}
	if !strings.Contains(execCall.Output, "/Users/dev/example-project") {
		t.Errorf("exec_command Output stripped preamble incorrectly: %q", execCall.Output)
	}
	if strings.Contains(execCall.Output, "Chunk ID:") {
		t.Errorf("exec_command Output still contains preamble: %q", execCall.Output)
	}

	// Turn 2: one user message, one final assistant, one web_search_call tool.
	turn2 := rollout.Turns[1]
	if turn2.TurnID != "019e-turn-0002" {
		t.Errorf("turn2.TurnID = %q", turn2.TurnID)
	}
	if len(turn2.UserMessages) != 1 {
		t.Errorf("turn2.UserMessages count = %d, want 1", len(turn2.UserMessages))
	}

	// Turn 3 (CF-438): inline-failed apply_patch. The custom_tool_call payload
	// carries status="failed"; the parser must propagate that onto ToolCall.Status.
	turn3 := rollout.Turns[2]
	if turn3.TurnID != "019e-turn-0003" {
		t.Errorf("turn3.TurnID = %q, want 019e-turn-0003", turn3.TurnID)
	}
	if len(turn3.ToolCalls) != 1 {
		t.Fatalf("turn3.ToolCalls count = %d, want 1", len(turn3.ToolCalls))
	}
	if turn3.ToolCalls[0].Name != "apply_patch" {
		t.Errorf("turn3 tool name = %q, want apply_patch", turn3.ToolCalls[0].Name)
	}
	if turn3.ToolCalls[0].Status != "failed" {
		t.Errorf("turn3 tool Status = %q, want \"failed\" (CF-438: inline-failed custom_tool_call must propagate)", turn3.ToolCalls[0].Status)
	}

	// Token usage from last non-null token_count info.
	if rollout.TokenUsage.InputTokens != 1000 {
		t.Errorf("TokenUsage.InputTokens = %d, want 1000", rollout.TokenUsage.InputTokens)
	}
	if rollout.TokenUsage.CachedInputTokens != 200 {
		t.Errorf("TokenUsage.CachedInputTokens = %d, want 200", rollout.TokenUsage.CachedInputTokens)
	}
	if rollout.TokenUsage.OutputTokens != 150 {
		t.Errorf("TokenUsage.OutputTokens = %d, want 150", rollout.TokenUsage.OutputTokens)
	}
	if rollout.TokenUsage.ReasoningOutputTokens != 50 {
		t.Errorf("TokenUsage.ReasoningOutputTokens = %d, want 50", rollout.TokenUsage.ReasoningOutputTokens)
	}

	// One compaction event.
	if len(rollout.Compactions) != 1 {
		t.Errorf("Compactions count = %d, want 1", len(rollout.Compactions))
	} else if rollout.Compactions[0].ReplacementCount != 2 {
		t.Errorf("Compactions[0].ReplacementCount = %d, want 2", rollout.Compactions[0].ReplacementCount)
	}
}

func TestParseRollout_MidTurnTruncation(t *testing.T) {
	raw := truncateBeforeLastTaskComplete(t, loadFixture(t))
	rollout, err := ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if len(rollout.Turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	last := rollout.Turns[len(rollout.Turns)-1]
	if last.CompletedAt != nil {
		t.Errorf("last turn CompletedAt = %v, want nil", last.CompletedAt)
	}
	if last.DurationMs != nil {
		t.Errorf("last turn DurationMs = %v, want nil", last.DurationMs)
	}
}

func TestParseRollout_BadJSONLine(t *testing.T) {
	bad := []byte(`{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"model":"gpt-5"}}
this is not json
{"timestamp":"2026-05-13T01:00:01.000Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1}}`)

	rollout, err := ParseRollout(bytes.NewReader(bad))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if rollout.Model != "gpt-5" {
		t.Errorf("Model = %q, want gpt-5", rollout.Model)
	}
	if len(rollout.ValidationErrors) == 0 {
		t.Error("expected at least one ValidationError")
	}
	if len(rollout.Turns) != 1 {
		t.Errorf("Turns count = %d, want 1", len(rollout.Turns))
	}
}

func TestParseRollout_EmptyInput(t *testing.T) {
	rollout, err := ParseRollout(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if len(rollout.Turns) != 0 {
		t.Errorf("Turns count = %d, want 0", len(rollout.Turns))
	}
	if len(rollout.ValidationErrors) != 0 {
		t.Errorf("ValidationErrors count = %d, want 0", len(rollout.ValidationErrors))
	}
}

func TestParseRollout_NullTokenCountInfo(t *testing.T) {
	null := []byte(`{"timestamp":"2026-05-13T01:00:00.000Z","type":"event_msg","payload":{"type":"token_count","info":null}}`)
	rollout, err := ParseRollout(bytes.NewReader(null))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if rollout.TokenUsage.InputTokens != 0 {
		t.Errorf("TokenUsage.InputTokens = %d, want 0", rollout.TokenUsage.InputTokens)
	}
}

func TestParseRollout_ExecExitNonZero(t *testing.T) {
	failExec := []byte(`{"timestamp":"2026-05-13T01:00:03.000Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{}","call_id":"c1"}}
{"timestamp":"2026-05-13T01:00:03.100Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"Chunk ID: 1\nWall time: 0.1 seconds\nProcess exited with code 1\nOriginal token count: 1\nOutput:\nerror\n"}}`)
	rollout, err := ParseRollout(bytes.NewReader(failExec))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if len(rollout.Turns) == 0 {
		t.Fatal("expected at least one (implicit) turn")
	}
	calls := rollout.Turns[0].ToolCalls
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Status != "failed" {
		t.Errorf("Status = %q, want failed (exit code 1)", calls[0].Status)
	}
	if calls[0].ExitCode == nil || *calls[0].ExitCode != 1 {
		t.Errorf("ExitCode = %v, want 1", calls[0].ExitCode)
	}
}

func TestParseRollout_EnvironmentContextStripped(t *testing.T) {
	envOnly := []byte(`{"timestamp":"2026-05-13T01:00:00.500Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\n<cwd>/x</cwd>\n</environment_context>"}]}}`)
	rollout, err := ParseRollout(bytes.NewReader(envOnly))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	for _, turn := range rollout.Turns {
		if len(turn.UserMessages) != 0 {
			t.Errorf("env-context-only message should be dropped, got %d user messages", len(turn.UserMessages))
		}
	}
}

func TestParseRollout_CompactionPreservesPriorTurns(t *testing.T) {
	rollout, err := ParseRollout(bytes.NewReader(loadFixture(t)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	// The fixture has 2 explicit turns + a compaction + an orphan output (implicit 3rd turn).
	// Compaction must NOT collapse the prior 2 turns.
	if len(rollout.Turns) < 2 {
		t.Errorf("Turns = %d, want at least 2 (compaction shouldn't drop turns)", len(rollout.Turns))
	}
	if len(rollout.Compactions) != 1 {
		t.Errorf("Compactions = %d, want 1", len(rollout.Compactions))
	}
}

func TestParseRollout_OrphanFunctionCallOutput(t *testing.T) {
	rollout, err := ParseRollout(bytes.NewReader(loadFixture(t)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	// The fixture's trailing orphan output should be surfaced as a synthetic
	// "<unknown>"-named tool call. It lands after the last task_complete, so
	// implicit-turn fires (active turn is closed; orphan opens a new one).
	var found bool
	for _, turn := range rollout.Turns {
		for _, tc := range turn.ToolCalls {
			if tc.Name == "<unknown>" && strings.Contains(tc.Output, "orphan output") {
				found = true
			}
		}
	}
	if !found {
		t.Error("orphan function_call_output should produce a synthetic <unknown> tool call")
	}

	// CF-438: every orphan output must also append a ValidationError naming
	// the unmatched call_id so the anomaly is discoverable downstream.
	var orphanErrs int
	for _, ve := range rollout.ValidationErrors {
		if ve.Type == "function_call_output" && strings.Contains(ve.Reason, "orphan output") {
			orphanErrs++
		}
	}
	if orphanErrs != 1 {
		t.Errorf("orphan ValidationError count = %d, want 1 (one orphan in fixture)", orphanErrs)
	}
}

// TestParseRollout_CustomToolCallFailedStatus covers CF-438 acceptance #1:
// a custom_tool_call carrying status="failed" inline (e.g. an apply_patch that
// fails on the call rather than via a later patch_apply_end event) must
// produce ToolCall.Status="failed", not "pending".
func TestParseRollout_CustomToolCallFailedStatus(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"model":"gpt-5","model_provider":"openai"}}`,
		`{"timestamp":"2026-05-13T01:00:00.100Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1,"model":"gpt-5"}}`,
		`{"timestamp":"2026-05-13T01:00:00.200Z","type":"response_item","payload":{"type":"custom_tool_call","status":"failed","call_id":"c1","name":"apply_patch","input":"*** Begin Patch\n*** Add File: x.txt\n+hi\n*** End Patch"}}`,
		`{"timestamp":"2026-05-13T01:00:01.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","completed_at":2,"duration_ms":900}}`,
	}, "\n")
	rollout, err := ParseRollout(bytes.NewReader([]byte(jsonl)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if len(rollout.Turns) != 1 {
		t.Fatalf("Turns = %d, want 1", len(rollout.Turns))
	}
	calls := rollout.Turns[0].ToolCalls
	if len(calls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(calls))
	}
	tc := calls[0]
	if tc.Name != "apply_patch" {
		t.Errorf("Name = %q, want apply_patch", tc.Name)
	}
	if tc.Status != "failed" {
		t.Errorf("Status = %q, want \"failed\" (inline failure must propagate)", tc.Status)
	}
}

// TestParseRollout_CustomToolCallCompletedStatus is the matched-pair test for
// the failed-status case: completed payloads must also propagate cleanly
// without regressing the existing behavior.
func TestParseRollout_CustomToolCallCompletedStatus(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"model":"gpt-5","model_provider":"openai"}}`,
		`{"timestamp":"2026-05-13T01:00:00.100Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1,"model":"gpt-5"}}`,
		`{"timestamp":"2026-05-13T01:00:00.200Z","type":"response_item","payload":{"type":"custom_tool_call","status":"completed","call_id":"c1","name":"apply_patch","input":"*** Begin Patch\n*** Add File: x.txt\n+hi\n*** End Patch"}}`,
		`{"timestamp":"2026-05-13T01:00:01.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","completed_at":2,"duration_ms":900}}`,
	}, "\n")
	rollout, err := ParseRollout(bytes.NewReader([]byte(jsonl)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if len(rollout.Turns) != 1 || len(rollout.Turns[0].ToolCalls) != 1 {
		t.Fatalf("unexpected shape")
	}
	if got := rollout.Turns[0].ToolCalls[0].Status; got != "completed" {
		t.Errorf("Status = %q, want \"completed\"", got)
	}
}

func TestParseRollout_ImplicitTurn(t *testing.T) {
	// A response_item before any task_started should create an implicit turn.
	implicit := []byte(`{"timestamp":"2026-05-13T01:00:00.500Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`)
	rollout, err := ParseRollout(bytes.NewReader(implicit))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if len(rollout.Turns) != 1 {
		t.Errorf("Turns = %d, want 1 (implicit)", len(rollout.Turns))
	}
}

// TestParseRollout_ModelFromTurnContext covers the Codex CLI ~0.130+ layout
// where `model` is absent from session_meta and lives in the per-turn
// turn_context envelope instead. Without this, rollout.Model stays empty and
// GetPricing returns zero — surfacing as $0.00 in the cost card.
func TestParseRollout_ModelFromTurnContext(t *testing.T) {
	raw := []byte(`{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"id":"s1","model_provider":"openai","cwd":"/x"}}
{"timestamp":"2026-05-13T01:00:00.100Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1}}
{"timestamp":"2026-05-13T01:00:00.100Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5.5"}}
{"timestamp":"2026-05-13T01:00:01.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","completed_at":2,"duration_ms":900}}`)

	rollout, err := ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if rollout.Model != "gpt-5.5" {
		t.Errorf("Model = %q, want gpt-5.5 (from turn_context)", rollout.Model)
	}
	if rollout.ModelProvider != "openai" {
		t.Errorf("ModelProvider = %q, want openai", rollout.ModelProvider)
	}
	if len(rollout.Turns) != 1 {
		t.Fatalf("Turns = %d, want 1", len(rollout.Turns))
	}
	if rollout.Turns[0].Model != "gpt-5.5" {
		t.Errorf("Turns[0].Model = %q, want gpt-5.5", rollout.Turns[0].Model)
	}
}

// TestParseRollout_TaskStartedModelWinsOverTurnContext ensures task_started.model
// remains authoritative for Turn.Model when both task_started and turn_context
// carry a model. This preserves CF-350's per-turn model contract.
func TestParseRollout_TaskStartedModelWinsOverTurnContext(t *testing.T) {
	raw := []byte(`{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"id":"s1","model_provider":"openai"}}
{"timestamp":"2026-05-13T01:00:00.100Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1,"model":"gpt-5"}}
{"timestamp":"2026-05-13T01:00:00.100Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5.5"}}
{"timestamp":"2026-05-13T01:00:01.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","completed_at":2,"duration_ms":900}}`)

	rollout, err := ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if len(rollout.Turns) != 1 {
		t.Fatalf("Turns = %d, want 1", len(rollout.Turns))
	}
	if rollout.Turns[0].Model != "gpt-5" {
		t.Errorf("Turns[0].Model = %q, want gpt-5 (task_started wins)", rollout.Turns[0].Model)
	}
	// Session-level Model was empty until turn_context filled it.
	if rollout.Model != "gpt-5.5" {
		t.Errorf("Model = %q, want gpt-5.5 (turn_context filled empty session-level)", rollout.Model)
	}
}

// TestParseRollout_SessionMetaModelWinsOverTurnContext ensures back-compat
// with older rollouts: when session_meta carries `model`, it stays as the
// session-level Model even if a later turn_context advertises a different one.
// Per-turn switches still flow into Turn.Model.
func TestParseRollout_SessionMetaModelWinsOverTurnContext(t *testing.T) {
	raw := []byte(`{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"id":"s1","model":"gpt-5","model_provider":"openai"}}
{"timestamp":"2026-05-13T01:00:00.100Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1}}
{"timestamp":"2026-05-13T01:00:00.100Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5.5"}}
{"timestamp":"2026-05-13T01:00:01.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","completed_at":2,"duration_ms":900}}`)

	rollout, err := ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if rollout.Model != "gpt-5" {
		t.Errorf("Model = %q, want gpt-5 (session_meta wins)", rollout.Model)
	}
	if len(rollout.Turns) != 1 {
		t.Fatalf("Turns = %d, want 1", len(rollout.Turns))
	}
	// task_started carried no model, but turn_context did → Turn picks it up.
	if rollout.Turns[0].Model != "gpt-5.5" {
		t.Errorf("Turns[0].Model = %q, want gpt-5.5 (per-turn override)", rollout.Turns[0].Model)
	}
}
