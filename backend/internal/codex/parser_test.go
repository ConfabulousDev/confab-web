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

	// Two completed turns plus a third implicit turn for the trailing orphan
	// function_call_output (orphan outputs need a turn to land in, and the
	// rollout closed after the second task_complete).
	if len(rollout.Turns) != 3 {
		t.Fatalf("Turns count = %d, want 3 (2 completed + 1 implicit for orphan)", len(rollout.Turns))
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
	// it implicitly creates a third turn? No — implicit-turn fires only when
	// there's NO active turn AND we see a response_item. After turn 2
	// completes, the active turn is closed; a third turn is implicitly opened
	// for the orphan output.
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
