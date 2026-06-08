package analytics

import (
	"encoding/json"
	"strings"
	"testing"
)

// parseRaw is a tiny test helper: parse a JSONL line into map form, as
// loadOpenCodeMessages does before calling ValidateOpenCodeLine.
func parseRaw(t *testing.T, line string) map[string]interface{} {
	t.Helper()
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		t.Fatalf("test fixture is not valid JSON: %v\n---\n%s", err, line)
	}
	return raw
}

// findError returns the ValidationError whose Path contains needle, or fails
// the test. Used so assertions describe the issue by path rather than index
// position in the slice.
func findError(t *testing.T, errors []ValidationError, needle string) ValidationError {
	t.Helper()
	for _, e := range errors {
		if strings.Contains(e.Path, needle) {
			return e
		}
	}
	t.Fatalf("no validation error matched path containing %q; got: %+v", needle, errors)
	return ValidationError{}
}

func TestValidateOpenCodeLine_HappyPath_UserMessage(t *testing.T) {
	line := `{"info":{"id":"msg_u1","sessionID":"ses_1","role":"user","time":{"created":1717689600000}},"parts":[{"id":"prt_u1","type":"text","text":"hello"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	if len(errors) != 0 {
		t.Errorf("happy-path user line produced errors: %+v", errors)
	}
}

func TestValidateOpenCodeLine_HappyPath_AssistantMessage(t *testing.T) {
	line := `{"info":{"id":"msg_a1","sessionID":"ses_1","role":"assistant","modelID":"claude-sonnet-4-20250514","providerID":"anthropic","cost":0.001,"tokens":{"input":100,"output":50,"cache":{"read":0,"write":0}},"time":{"created":1717689601000},"finish":"stop"},"parts":[{"id":"prt_a1","type":"text","text":"world"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	if len(errors) != 0 {
		t.Errorf("happy-path assistant line produced errors: %+v", errors)
	}
}

func TestValidateOpenCodeLine_MissingInfo(t *testing.T) {
	line := `{"parts":[]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	e := findError(t, errors, "info")
	if e.Expected != "object" {
		t.Errorf("info error expected=%q, want %q", e.Expected, "object")
	}
}

func TestValidateOpenCodeLine_MissingRole(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","time":{"created":1}},"parts":[]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "info.role")
}

func TestValidateOpenCodeLine_InvalidRole(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"agent","time":{"created":1}},"parts":[]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	e := findError(t, errors, "info.role")
	if !strings.Contains(e.Received, "agent") {
		t.Errorf("role error received=%q, want it to mention 'agent'", e.Received)
	}
}

func TestValidateOpenCodeLine_AssistantMissingTokens(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"assistant","modelID":"m","providerID":"p","time":{"created":1}},"parts":[]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "info.tokens")
}

func TestValidateOpenCodeLine_AssistantMissingModelID(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"assistant","providerID":"p","tokens":{"input":1,"output":1},"time":{"created":1}},"parts":[]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "info.modelID")
}

func TestValidateOpenCodeLine_TokensWrongType(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"assistant","modelID":"m","providerID":"p","tokens":{"input":"not-a-number","output":50},"time":{"created":1}},"parts":[]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "info.tokens.input")
}

func TestValidateOpenCodeLine_PartsNotArray(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"user","time":{"created":1}},"parts":"oops"}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	e := findError(t, errors, "parts")
	if e.Expected != "array" {
		t.Errorf("parts error expected=%q, want %q", e.Expected, "array")
	}
}

func TestValidateOpenCodeLine_PartMissingType(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"user","time":{"created":1}},"parts":[{"id":"prt_0"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "parts[0].type")
}

func TestValidateOpenCodeLine_TextPartMissingText(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"user","time":{"created":1}},"parts":[{"id":"prt_0","type":"text"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "parts[0].text")
}

func TestValidateOpenCodeLine_ToolPartMissingTool(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"assistant","modelID":"m","providerID":"p","tokens":{"input":1,"output":1},"time":{"created":1}},"parts":[{"id":"prt_0","type":"tool"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "parts[0].tool")
}

func TestValidateOpenCodeLine_ToolStateInvalidStatus(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"assistant","modelID":"m","providerID":"p","tokens":{"input":1,"output":1},"time":{"created":1}},"parts":[{"id":"prt_0","type":"tool","tool":"Bash","state":{"status":"weird"}}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "parts[0].state.status")
}

func TestValidateOpenCodeLine_SubtaskMissingName(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"assistant","modelID":"m","providerID":"p","tokens":{"input":1,"output":1},"time":{"created":1}},"parts":[{"id":"prt_0","type":"subtask","prompt":"do thing"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "parts[0].name")
}

func TestValidateOpenCodeLine_CompactionMissingAuto(t *testing.T) {
	line := `{"info":{"id":"x","sessionID":"y","role":"assistant","modelID":"m","providerID":"p","tokens":{"input":1,"output":1},"time":{"created":1}},"parts":[{"id":"prt_0","type":"compaction"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	findError(t, errors, "parts[0].auto")
}

func TestValidateOpenCodeLine_UnknownPartTypeAccepted(t *testing.T) {
	// Forward-compat: future OpenCode releases may add new part types. Don't
	// reject them; only validate types we know about.
	line := `{"info":{"id":"x","sessionID":"y","role":"assistant","modelID":"m","providerID":"p","tokens":{"input":1,"output":1},"time":{"created":1}},"parts":[{"id":"prt_0","type":"future-thing","weird":"value"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	if len(errors) != 0 {
		t.Errorf("unknown part type should pass validation (forward-compat): %+v", errors)
	}
}

func TestValidateOpenCodeLine_UnknownTopLevelFieldAccepted(t *testing.T) {
	// Forward-compat: future OpenCode releases may add top-level fields. Don't
	// reject them.
	line := `{"info":{"id":"x","sessionID":"y","role":"user","time":{"created":1}},"parts":[],"futureField":"value"}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	if len(errors) != 0 {
		t.Errorf("unknown top-level field should pass validation (forward-compat): %+v", errors)
	}
}

func TestValidateOpenCodeLine_RealCapture_UserMessage(t *testing.T) {
	// Sanity-check the schema against real captured data (from
	// /tmp/galois-opencode/opencode.db, parent session
	// ses_1652a0e78ffehrQYrdlZceiaC6). The fields below match a real user
	// message line, minus stable IDs that don't matter for validation.
	line := `{"info":{"id":"msg_real_user","sessionID":"ses_1652a0e78ffehrQYrdlZceiaC6","role":"user","time":{"created":1718000000000}},"parts":[{"id":"prt_real_u1","type":"text","text":"explore this codebase"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	if len(errors) != 0 {
		t.Errorf("real-capture user line should validate cleanly: %+v", errors)
	}
}

func TestValidateOpenCodeLine_RealCapture_AssistantMessage(t *testing.T) {
	// Real-capture shape: assistant with agent, parentID, full tokens including
	// reasoning + cache, cost. From the same galois subagent session.
	line := `{"info":{"id":"msg_real_asst","sessionID":"ses_16529ea8cffevgT59thxSGNu8V","role":"assistant","parentID":"msg_real_parent","agent":"explore","mode":"build","modelID":"qwen3.7-max","providerID":"opencode-go","cost":0.341651125,"tokens":{"input":54,"output":5838,"reasoning":0,"cache":{"read":257356,"write":0}},"time":{"created":1718000010000},"finish":"stop"},"parts":[{"id":"prt_real_a1","type":"text","text":"Done"}]}`
	errors := ValidateOpenCodeLine(parseRaw(t, line))
	if len(errors) != 0 {
		t.Errorf("real-capture assistant line should validate cleanly: %+v", errors)
	}
}
