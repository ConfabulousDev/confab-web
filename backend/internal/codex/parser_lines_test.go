package codex

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// Line-splitting and oversized-line contract for ParseRollout (e9qd): one
// line over the cap must never abort the whole rollout.

const (
	linesSessionMeta  = `{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"model":"gpt-5","model_provider":"openai"}}`
	linesTaskStarted  = `{"timestamp":"2026-05-13T01:00:00.100Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1,"model":"gpt-5"}}`
	linesToolCall     = `{"timestamp":"2026-05-13T01:00:00.200Z","type":"response_item","payload":{"type":"custom_tool_call","status":"completed","call_id":"c1","name":"apply_patch","input":"*** Begin Patch\n*** Add File: x.txt\n+hi\n*** End Patch"}}`
	linesTaskComplete = `{"timestamp":"2026-05-13T01:00:01.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","completed_at":2,"duration_ms":900}}`
	linesSecondTurn   = `{"timestamp":"2026-05-13T01:00:02.000Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t2","started_at":3,"model":"gpt-5"}}`
)

// toolOutputLine builds a custom_tool_call_output line for call c1 whose
// output is `size` bytes of 'a'.
func toolOutputLine(size int) string {
	return `{"timestamp":"2026-05-13T01:00:00.300Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"c1","output":"` +
		strings.Repeat("a", size) + `"}}`
}

// toolOutputLineOfLen builds a custom_tool_call_output line exactly n bytes long.
func toolOutputLineOfLen(t *testing.T, n int) string {
	t.Helper()
	overhead := len(toolOutputLine(0))
	if n < overhead {
		t.Fatalf("line length %d below envelope overhead %d", n, overhead)
	}
	line := toolOutputLine(n - overhead)
	if len(line) != n {
		t.Fatalf("built line of %d bytes, want %d", len(line), n)
	}
	return line
}

// rolloutWithOutputLine is a one-turn rollout whose 4th line (the tool
// output) is outputLine, followed by task_complete and a second turn.
func rolloutWithOutputLine(outputLine string) []byte {
	return []byte(strings.Join([]string{
		linesSessionMeta,
		linesTaskStarted,
		linesToolCall,
		outputLine,
		linesTaskComplete,
		linesSecondTurn,
	}, "\n") + "\n")
}

func mustParse(t *testing.T, raw []byte) *ParsedRollout {
	t.Helper()
	rollout, err := ParseRollout(raw)
	if err != nil {
		t.Fatalf("ParseRollout returned error %v; one line must never abort the whole rollout", err)
	}
	return rollout
}

func TestParseRollout_ToolOutputBeyondOldScannerCap_ParsesFully(t *testing.T) {
	const bigOutput = 5 * 1024 * 1024 // over the old 4 MiB scanner cap
	big := mustParse(t, rolloutWithOutputLine(toolOutputLine(bigOutput)))
	small := mustParse(t, rolloutWithOutputLine(toolOutputLine(2)))

	if len(big.ValidationErrors) != 0 {
		t.Errorf("ValidationErrors = %+v, want none for a 5 MiB line under the cap", big.ValidationErrors)
	}
	if len(big.Turns) != len(small.Turns) || len(big.Turns) != 2 {
		t.Fatalf("Turns = %d (small rollout: %d), want 2 for both", len(big.Turns), len(small.Turns))
	}
	if len(big.Turns[0].ToolCalls) != len(small.Turns[0].ToolCalls) || len(big.Turns[0].ToolCalls) != 1 {
		t.Fatalf("turn 1 ToolCalls = %d (small rollout: %d), want 1", len(big.Turns[0].ToolCalls), len(small.Turns[0].ToolCalls))
	}
	if big.TotalLines != small.TotalLines {
		t.Errorf("TotalLines = %d, want %d (same as the small-output rollout)", big.TotalLines, small.TotalLines)
	}
	tc := big.Turns[0].ToolCalls[0]
	if len(tc.Output) != bigOutput {
		t.Errorf("tool Output length = %d, want %d (large output must be kept)", len(tc.Output), bigOutput)
	}
	if tc.Status != "completed" {
		t.Errorf("tool Status = %q, want completed", tc.Status)
	}
}

func TestParseRollout_LineOverCap_SkippedWithValidationError(t *testing.T) {
	over := toolOutputLineOfLen(t, maxLineBytes+1)
	rollout := mustParse(t, rolloutWithOutputLine(over))

	if len(rollout.ValidationErrors) != 1 {
		t.Fatalf("ValidationErrors = %d (%+v), want exactly 1 for the oversized line", len(rollout.ValidationErrors), rollout.ValidationErrors)
	}
	ve := rollout.ValidationErrors[0]
	if ve.Line != 4 {
		t.Errorf("ValidationError.Line = %d, want 4", ve.Line)
	}
	if ve.Type != "" {
		t.Errorf("ValidationError.Type = %q, want empty (oversized line is not decoded)", ve.Type)
	}
	wantReason := "line exceeds 32 MiB cap (" + strconv.Itoa(maxLineBytes+1) + " bytes); skipped"
	if ve.Reason != wantReason {
		t.Errorf("ValidationError.Reason = %q, want %q", ve.Reason, wantReason)
	}

	// Every other line is still parsed: both turns, the tool call (left
	// pending without its output), and the closed first turn.
	if len(rollout.Turns) != 2 {
		t.Fatalf("Turns = %d, want 2 (lines after the oversized one must still parse)", len(rollout.Turns))
	}
	if rollout.Turns[0].CompletedAt == nil {
		t.Error("turn 1 CompletedAt = nil, want set by the task_complete after the oversized line")
	}
	calls := rollout.Turns[0].ToolCalls
	if len(calls) != 1 || calls[0].Output != "" {
		t.Errorf("turn 1 ToolCalls = %+v, want one call with no output", calls)
	}
	if rollout.TotalLines != 6 {
		t.Errorf("TotalLines = %d, want 6 (the skipped line still counts)", rollout.TotalLines)
	}
}

func TestParseRollout_LineExactlyAtCap_Kept(t *testing.T) {
	rollout := mustParse(t, rolloutWithOutputLine(toolOutputLineOfLen(t, maxLineBytes)))
	if len(rollout.ValidationErrors) != 0 {
		t.Errorf("ValidationErrors = %+v, want none for a line exactly at the cap", rollout.ValidationErrors)
	}
	if got := len(rollout.Turns[0].ToolCalls[0].Output); got == 0 {
		t.Error("tool Output is empty, want the at-cap line's output kept")
	}
}

func TestParseRollout_OversizedFinalLineNoNewline_Skipped(t *testing.T) {
	raw := []byte(strings.Join([]string{
		linesSessionMeta,
		linesTaskStarted,
		linesToolCall,
		toolOutputLineOfLen(t, maxLineBytes+1),
	}, "\n"))
	rollout := mustParse(t, raw)
	if len(rollout.ValidationErrors) != 1 || rollout.ValidationErrors[0].Line != 4 {
		t.Fatalf("ValidationErrors = %+v, want one at line 4", rollout.ValidationErrors)
	}
	if rollout.Model != "gpt-5" || len(rollout.Turns) != 1 {
		t.Errorf("Model=%q Turns=%d, want gpt-5 and 1 turn from the lines before it", rollout.Model, len(rollout.Turns))
	}
}

func TestParseRollout_LineSplittingParity(t *testing.T) {
	base := []string{linesSessionMeta, linesTaskStarted, linesToolCall, toolOutputLine(2), linesTaskComplete}
	lf := mustParse(t, []byte(strings.Join(base, "\n")+"\n"))

	cases := []struct {
		name string
		raw  string
	}{
		{"CRLF", strings.Join(base, "\r\n") + "\r\n"},
		{"no trailing newline", strings.Join(base, "\n")},
		{"blank lines in the middle", strings.Join(base[:2], "\n") + "\n\n   \n" + strings.Join(base[2:], "\n") + "\n"},
		{"trailing blank lines", strings.Join(base, "\n") + "\n\n\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mustParse(t, []byte(c.raw))
			if got.TotalLines != lf.TotalLines {
				t.Errorf("TotalLines = %d, want %d (blank lines never count)", got.TotalLines, lf.TotalLines)
			}
			if len(got.ValidationErrors) != 0 {
				t.Errorf("ValidationErrors = %+v, want none", got.ValidationErrors)
			}
			if len(got.Turns) != 1 || got.Turns[0].CompletedAt == nil || len(got.Turns[0].ToolCalls) != 1 {
				t.Fatalf("Turns = %+v, want one completed turn with one tool call", got.Turns)
			}
			if out := got.Turns[0].ToolCalls[0].Output; out != "aa" {
				t.Errorf("tool Output = %q, want %q (no stray \\r)", out, "aa")
			}
		})
	}
}

func TestParseRollout_MalformedLineNumberCountsBlankLines(t *testing.T) {
	raw := []byte(linesSessionMeta + "\n\n" + "not json\r\n" + linesTaskStarted)
	rollout := mustParse(t, raw)
	if len(rollout.ValidationErrors) != 1 {
		t.Fatalf("ValidationErrors = %+v, want 1", rollout.ValidationErrors)
	}
	if got := rollout.ValidationErrors[0].Line; got != 3 {
		t.Errorf("ValidationError.Line = %d, want 3 (blank lines are counted in line numbers)", got)
	}
	if rollout.TotalLines != 3 {
		t.Errorf("TotalLines = %d, want 3", rollout.TotalLines)
	}
	if len(rollout.Turns) != 1 {
		t.Errorf("Turns = %d, want 1 from the line after the malformed one", len(rollout.Turns))
	}
}

func TestParseRollout_EmptyAndWhitespaceInput_NoLines(t *testing.T) {
	for name, raw := range map[string][]byte{"nil": nil, "empty": {}, "newlines only": []byte("\n\r\n\n")} {
		t.Run(name, func(t *testing.T) {
			rollout := mustParse(t, raw)
			if rollout.TotalLines != 0 || len(rollout.Turns) != 0 || len(rollout.ValidationErrors) != 0 {
				t.Errorf("TotalLines=%d Turns=%d ValidationErrors=%d, want all 0",
					rollout.TotalLines, len(rollout.Turns), len(rollout.ValidationErrors))
			}
		})
	}
}

// ParseRollout must not retain or mutate the caller's buffer contents.
func TestParseRollout_DoesNotMutateInput(t *testing.T) {
	raw := rolloutWithOutputLine(toolOutputLine(8))
	orig := bytes.Clone(raw)
	mustParse(t, raw)
	if !bytes.Equal(raw, orig) {
		t.Error("ParseRollout mutated its input")
	}
}
