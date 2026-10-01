package analytics

import (
	"context"
	"strings"
	"testing"
)

// redactionParityMain / redactionParityAgents form a fixture that exercises
// every redaction-counting rule: markers nested in tool_use inputs and
// tool_result content, the literal [REDACTED:TYPE] placeholder (skipped), a
// marker used as a JSON key (keys are not counted), a \u-escaped marker
// (counted: it decodes to a marker), and markers on validation-failing and
// invalid-JSON lines (not counted). Expected counts were captured from the
// pre-5m68 implementation, which walked TranscriptLine.RawData.
func redactionParityMain(t *testing.T) []byte {
	t.Helper()
	lines := []string{
		makeUserMessage("u1", "2025-01-01T00:00:00Z", "token [REDACTED:GITHUB_TOKEN] and [REDACTED:TYPE]"),
		makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 10, 5, []map[string]any{
			makeToolUseBlock("toolu_1", "Bash", map[string]any{
				"cmd":                 "export K=[REDACTED:AWS_KEY]",
				"nested":              map[string]any{"list": []any{"[REDACTED:AWS_KEY]", "x"}},
				"[REDACTED:KEY_ONLY]": "plain",
			}),
		}),
		makeUserMessageWithToolResults("u2", "2025-01-01T00:00:02Z", []map[string]any{
			makeToolResultBlock("toolu_1", "out: [REDACTED:PASSWORD][REDACTED:PASSWORD]", false),
		}),
		// \u-escaped marker: decodes to [REDACTED:ESCAPED] and is counted.
		strings.Replace(
			makeAssistantMessage("a2", "2025-01-01T00:00:03Z", "claude-sonnet-4", 10, 5, []map[string]any{
				makeTextBlock("[REDACTED:ESCAPED]"),
			}),
			`[REDACTED:ESCAPED]`, `\u005bREDACTED:ESCAPED]`, 1),
		// Validation-failing line (assistant without a message object).
		`{"type":"assistant","message":"[REDACTED:INVALID_LINE]"}`,
		// Invalid JSON.
		`{"broken [REDACTED:BROKEN_JSON]`,
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func redactionParityAgents() map[string][]byte {
	return map[string][]byte{
		"agent-a": []byte(makeAssistantMessage("aa1", "2025-01-01T00:00:04Z", "claude-haiku-3", 10, 5, []map[string]any{
			makeTextBlock("Agent: [REDACTED:AGENT_SECRET]"),
		}) + "\n" + makeUserMessage("au1", "2025-01-01T00:00:05Z", "More: [REDACTED:GITHUB_TOKEN]") + "\n"),
		"agent-b": []byte(makeUserMessageWithToolResults("bu1", "2025-01-01T00:00:06Z", []map[string]any{
			makeToolResultBlock("toolu_9", "[REDACTED:AGENT_SECRET]", false),
		}) + "\n"),
	}
}

var redactionParityWant = map[string]int{
	"GITHUB_TOKEN": 2,
	"AWS_KEY":      2,
	"PASSWORD":     2,
	"ESCAPED":      1,
	"AGENT_SECRET": 2,
}

const redactionParityWantTotal = 9

func assertRedactionParity(t *testing.T, total int, counts map[string]int) {
	t.Helper()
	if total != redactionParityWantTotal {
		t.Errorf("TotalRedactions = %d, want %d (counts=%v)", total, redactionParityWantTotal, counts)
	}
	if len(counts) != len(redactionParityWant) {
		t.Errorf("RedactionCounts = %v, want %v", counts, redactionParityWant)
	}
	for k, want := range redactionParityWant {
		if got := counts[k]; got != want {
			t.Errorf("RedactionCounts[%q] = %d, want %d", k, got, want)
		}
	}
	for _, excluded := range []string{"TYPE", "KEY_ONLY", "INVALID_LINE", "BROKEN_JSON"} {
		if n, ok := counts[excluded]; ok {
			t.Errorf("RedactionCounts[%q] = %d, must not be counted", excluded, n)
		}
	}
}

// TestRedactionCountsMatchPreChangeImplementation_FileCollection pins the
// redaction counts produced through the in-memory FileCollection path.
func TestRedactionCountsMatchPreChangeImplementation_FileCollection(t *testing.T) {
	fc, err := NewFileCollectionWithAgents(redactionParityMain(t), redactionParityAgents())
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents: %v", err)
	}
	if got := len(fc.Main.ValidationErrors); got != 2 {
		t.Fatalf("main ValidationErrors = %d, want 2 (fixture must include one schema-invalid and one invalid-JSON line)", got)
	}
	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	assertRedactionParity(t, result.TotalRedactions, result.RedactionCounts)
}

// TestRedactionCountsMatchPreChangeImplementation_StreamingRollout pins the
// same counts through the production path: claudeProvider.ComputeCards over a
// rollout that streams agent files from a downloader.
func TestRedactionCountsMatchPreChangeImplementation_StreamingRollout(t *testing.T) {
	main, err := parseTranscriptFile(redactionParityMain(t), "")
	if err != nil {
		t.Fatalf("parse main: %v", err)
	}
	agents := redactionParityAgents()
	rollout := &claudeRollout{
		main: main,
		agentInfo: []AgentFileInfo{
			{FileName: "agent-a.jsonl", AgentID: "agent-a"},
			{FileName: "agent-b.jsonl", AgentID: "agent-b"},
		},
		downloader: func(_ context.Context, fileName string) ([]byte, error) {
			return agents[strings.TrimSuffix(fileName, ".jsonl")], nil
		},
	}
	computed := (&claudeProvider{}).ComputeCards(context.Background(), rollout)
	assertRedactionParity(t, computed.TotalRedactions, computed.RedactionCounts)
}
