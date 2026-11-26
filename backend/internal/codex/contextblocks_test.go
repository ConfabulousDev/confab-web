package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contextBlocksFixturePath locates the repo-root cross-language fixture from
// this package directory (backend/internal/codex → repo root is three up).
var contextBlocksFixturePath = filepath.Join("..", "..", "..", "testdata", "codex_contextual_user_blocks.json")

// TestIsContextBlock_Parity loads the shared fixture (also loaded by the
// frontend's codexContextBlocks.test.ts) and asserts isContextBlock agrees with
// every row, so the Go and TS classifiers can't silently drift (z63s).
func TestIsContextBlock_Parity(t *testing.T) {
	raw, err := os.ReadFile(contextBlocksFixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", contextBlocksFixturePath, err)
	}
	var fixture struct {
		Cases []struct {
			Name      string `json:"name"`
			Text      string `json:"text"`
			IsContext bool   `json:"isContext"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("fixture has no cases")
	}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			if got := isContextBlock(c.Text); got != c.IsContext {
				t.Errorf("isContextBlock(%q) = %v, want %v", c.Text, got, c.IsContext)
			}
		})
	}
}

// userMessageLine builds a user-role response_item.message whose content is
// one input_text block per text.
func userMessageLine(t *testing.T, ts string, texts ...string) string {
	t.Helper()
	blocks := make([]map[string]string, 0, len(texts))
	for _, text := range texts {
		blocks = append(blocks, map[string]string{"type": "input_text", "text": text})
	}
	line, err := json.Marshal(map[string]any{
		"timestamp": ts,
		"type":      "response_item",
		"payload":   map[string]any{"type": "message", "role": "user", "content": blocks},
	})
	if err != nil {
		t.Fatalf("marshal user message: %v", err)
	}
	return string(line)
}

// parseOneTurn wraps the given lines in a single task_started/task_complete
// turn and parses them.
func parseOneTurn(t *testing.T, lines ...string) *ParsedRollout {
	t.Helper()
	all := append([]string{
		`{"timestamp":"2026-11-25T10:00:00.000Z","type":"session_meta","payload":{"id":"s","cli_version":"0.172.0"}}`,
		`{"timestamp":"2026-11-25T10:00:00.100Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1","started_at":1,"model":"gpt-5"}}`,
	}, lines...)
	all = append(all, `{"timestamp":"2026-11-25T10:00:09.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","completed_at":9,"duration_ms":8900}}`)
	rollout, err := ParseRollout([]byte(strings.Join(all, "\n")))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	return rollout
}

func allUserTexts(rollout *ParsedRollout) []string {
	var out []string
	for _, turn := range rollout.Turns {
		for _, m := range turn.UserMessages {
			out = append(out, m.Text)
		}
	}
	return out
}

const (
	agentsMDBlock   = "# AGENTS.md instructions for /home/dev/example\n\n<INSTRUCTIONS>\n## Development\nRun make ci.\n</INSTRUCTIONS>"
	envContextBlock = "<environment_context>\n  <cwd>/home/dev/example</cwd>\n</environment_context>"
	skillBlock      = "<skill>\n<name>demo-skill</name>\n<path>/home/dev/.codex/skills/demo/SKILL.md</path>\n---\nbody text\n</skill>"
)

func TestHandleMessage_AgentsMDContextMessageIsNotAUserMessage(t *testing.T) {
	rollout := parseOneTurn(t,
		userMessageLine(t, "2026-11-25T10:00:01.000Z", agentsMDBlock, envContextBlock),
		userMessageLine(t, "2026-11-25T10:00:02.000Z", "fix the flaky test"),
	)
	got := allUserTexts(rollout)
	if len(got) != 1 || got[0] != "fix the flaky test" {
		t.Fatalf("user messages = %q, want only the human prompt [\"fix the flaky test\"]", got)
	}
}

func TestHandleMessage_DropsEveryContextBlockKind(t *testing.T) {
	contextOnly := []string{
		"<turn_aborted>\nThe user interrupted the previous turn on purpose.\n</turn_aborted>",
		"<user_shell_command>\n<command>ls</command>\n</user_shell_command>",
		"<codex_internal_context source=\"goal\">\nkeep going\n</codex_internal_context>",
		"<subagent_notification>\n{}\n</subagent_notification>",
		"<external_demo_open_page>{\"page_id\":42}</external_demo_open_page>",
	}
	for _, text := range contextOnly {
		rollout := parseOneTurn(t, userMessageLine(t, "2026-11-25T10:00:01.000Z", text))
		if got := allUserTexts(rollout); len(got) != 0 {
			t.Errorf("context-only message %q must produce no user message, got %q", text, got)
		}
	}
}

func TestHandleMessage_KeepsPromptBlockNextToSkillBlock(t *testing.T) {
	rollout := parseOneTurn(t,
		userMessageLine(t, "2026-11-25T10:00:01.000Z", "hey, run this:", skillBlock, "thanks"),
	)
	got := allUserTexts(rollout)
	if len(got) != 1 || got[0] != "hey, run this:\nthanks" {
		t.Fatalf("user messages = %q, want [\"hey, run this:\\nthanks\"]", got)
	}
	if len(rollout.SkillInvocations) != 1 {
		t.Fatalf("SkillInvocations = %d, want 1 (skill block must still be recorded)", len(rollout.SkillInvocations))
	}
	inv := rollout.SkillInvocations[0]
	if inv.Name != "demo-skill" || inv.Path != "/home/dev/.codex/skills/demo/SKILL.md" {
		t.Errorf("SkillInvocations[0] = {%q, %q}, want {demo-skill, /home/dev/.codex/skills/demo/SKILL.md}", inv.Name, inv.Path)
	}
}

func TestHandleMessage_SkillBlockMixedCaseStillRecordsInvocation(t *testing.T) {
	rollout := parseOneTurn(t,
		userMessageLine(t, "2026-11-25T10:00:01.000Z", "<SKILL>\n<name>loud</name>\n<path>/p</path>\n</Skill>"),
	)
	if len(rollout.SkillInvocations) != 1 || rollout.SkillInvocations[0].Name != "loud" {
		t.Fatalf("SkillInvocations = %+v, want one invocation named loud", rollout.SkillInvocations)
	}
	if got := allUserTexts(rollout); len(got) != 0 {
		t.Errorf("skill-only message must produce no user message, got %q", got)
	}
}

func TestHandleMessage_PromptMentioningSkillIsKeptVerbatim(t *testing.T) {
	prompt := "run this for me: <skill>x</skill>"
	rollout := parseOneTurn(t, userMessageLine(t, "2026-11-25T10:00:01.000Z", prompt))
	got := allUserTexts(rollout)
	if len(got) != 1 || got[0] != prompt {
		t.Fatalf("user messages = %q, want the prompt kept verbatim [%q]", got, prompt)
	}
	if len(rollout.SkillInvocations) != 0 {
		t.Errorf("a prompt that merely mentions <skill> must not record a SkillInvocation, got %+v", rollout.SkillInvocations)
	}
}

func TestHandleMessage_WhitespaceOnlyRemainderIsDropped(t *testing.T) {
	rollout := parseOneTurn(t,
		userMessageLine(t, "2026-11-25T10:00:01.000Z", agentsMDBlock, "  \n "),
	)
	if got := allUserTexts(rollout); len(got) != 0 {
		t.Errorf("message with only context + whitespace must produce no user message, got %q", got)
	}
}

func TestHandleMessage_AssistantContextLookalikeIsUntouched(t *testing.T) {
	text := "<turn_aborted>\nnot really\n</turn_aborted>"
	line := `{"timestamp":"2026-11-25T10:00:01.000Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":` + mustJSONString(t, text) + `}]}}`
	rollout := parseOneTurn(t, line)
	if len(rollout.Turns) != 1 || len(rollout.Turns[0].AssistantMessages) != 1 {
		t.Fatalf("want 1 assistant message, got turns=%+v", rollout.Turns)
	}
	if got := rollout.Turns[0].AssistantMessages[0].Text; got != text {
		t.Errorf("assistant text = %q, want %q (classification is user-role only)", got, text)
	}
}

func mustJSONString(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
