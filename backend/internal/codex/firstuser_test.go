package codex

import "testing"

// Fixture lines below reproduce the exact wire shapes captured from real Codex
// rollouts on this machine (26 × 0.130.0, 6 × 0.149.1–0.154.0). Field names,
// nesting, ordering and the sibling fields the ticket's sketch omitted
// (`images`, `local_images`, `text_elements`, `ordinal`, `client_id`) are
// verbatim; only the human prompt text, ids and paths are substituted so no
// personal transcript content is committed.
const (
	// ≤0.130.0: event_msg → payload.type "user_message", flat `message` string.
	oldEraUserMessageLine = `{"timestamp":"2026-05-18T15:26:28.570Z","type":"event_msg","payload":{"type":"user_message","message":"add a retry to the uploader","images":[],"local_images":[],"text_elements":[]}}`

	// ≥0.149.1: event_msg → payload.type "item_completed" wrapping a UserMessage
	// item whose content[] carries typed parts.
	modernUserMessageLine = `{"timestamp":"2026-09-10T21:44:11.768Z","ordinal":9,"type":"event_msg","payload":{"type":"item_completed","thread_id":"01a08d46-8735-7c00-b558-f23eb1ce6cbc","turn_id":"01a08d47-3d8b-7442-9479-15fa190372fc","item":{"type":"UserMessage","id":"01a08d47-3ef8-75a0-9e73-cb16a9ec747d","client_id":"62bbfe8b-4ce1-48a5-b3bb-60bffd0cbffe","content":[{"type":"text","text":"explore this project and review my draft post","text_elements":[]}],"started_at_ms":1789076651768,"completed_at_ms":1789076651768}}}`

	// Real modern rollouts mix non-text parts (a `skill` element) into content[].
	modernUserMessageWithSkillPartLine = `{"timestamp":"2026-09-10T21:44:11.768Z","ordinal":9,"type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","id":"01a08d47-3ef8-75a0-9e73-cb16a9ec747d","content":[{"type":"text","text":"$retro bcc1cefd-3116-4303-ae5b-2516ba463","text_elements":[]},{"type":"skill","name":"retro","path":"/home/u/.claude/skills/retro/SKILL.md"}]}}}`

	// The stream we deliberately do NOT read (T1): in every real modern rollout
	// the first user-role response_item is injected context, not human input.
	agentsMDResponseItemLine       = `{"timestamp":"2026-08-25T18:00:53.109Z","ordinal":5,"type":"response_item","payload":{"type":"message","id":"msg_01a03a15","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions for /home/u/dev/example\n\n<INSTRUCTIONS>\n## Development\n</INSTRUCTIONS>"}]}}`
	environmentContextResponseItem = `{"timestamp":"2026-05-18T01:17:49.127Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\n  <cwd>/home/u/dev/example</cwd>\n</environment_context>"}]}}`
	developerRoleResponseItemLine  = `{"timestamp":"2026-05-18T01:17:49.127Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"<permissions instructions>\nFilesystem sandboxing..."}]}}`
	taskStartedEventLine           = `{"timestamp":"2026-05-18T01:17:49.127Z","type":"event_msg","payload":{"type":"task_started","turn_id":"019e38a9","started_at":1779067066,"model_context_window":258400}}`
	tokenCountEventLine            = `{"timestamp":"2026-05-18T01:17:49.320Z","type":"event_msg","payload":{"type":"token_count","info":null}}`
	agentMessageItemCompletedLine  = `{"timestamp":"2026-08-25T18:00:56.147Z","ordinal":10,"type":"event_msg","payload":{"type":"item_completed","item":{"type":"AgentMessage","id":"msg_01e9d0","content":[{"type":"Text","text":"I'll map the project structure first."}],"phase":"commentary"}}}`
	commandExecItemCompletedLine   = `{"timestamp":"2026-08-25T18:00:59.446Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"CommandExecution","id":"exec-9496","command":["/bin/zsh","-lc","pwd"],"status":"completed","stdout":"/home/u"}}}`
	sessionMetaLine                = `{"timestamp":"2026-09-10T21:44:00.000Z","type":"session_meta","payload":{"id":"01a08d46","cwd":"/home/u/dev/example","cli_version":"0.154.0"}}`
)

func TestUserMessageFromLine_ModernEra(t *testing.T) {
	got := UserMessageFromLine(modernUserMessageLine)
	want := "explore this project and review my draft post"
	if got != want {
		t.Errorf("UserMessageFromLine() = %q, want %q", got, want)
	}
}

func TestUserMessageFromLine_OldEra(t *testing.T) {
	got := UserMessageFromLine(oldEraUserMessageLine)
	want := "add a retry to the uploader"
	if got != want {
		t.Errorf("UserMessageFromLine() = %q, want %q", got, want)
	}
}

func TestUserMessageFromLine_IgnoresNonTextContentParts(t *testing.T) {
	got := UserMessageFromLine(modernUserMessageWithSkillPartLine)
	want := "$retro bcc1cefd-3116-4303-ae5b-2516ba463"
	if got != want {
		t.Errorf("UserMessageFromLine() = %q, want %q", got, want)
	}
}

func TestUserMessageFromLine_JoinsMultipleTextParts(t *testing.T) {
	line := `{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"text","text":"first part"},{"type":"text","text":"second part"}]}}}`
	got := UserMessageFromLine(line)
	want := "first part\nsecond part"
	if got != want {
		t.Errorf("UserMessageFromLine() = %q, want %q", got, want)
	}
}

// T1: the response_item stream is contaminated with injected context in every
// real rollout, so the extractor must never read it.
func TestUserMessageFromLine_NeverReadsResponseItems(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"AGENTS.md injection as user-role response_item", agentsMDResponseItemLine},
		{"environment_context as user-role response_item", environmentContextResponseItem},
		{"developer-role response_item", developerRoleResponseItemLine},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UserMessageFromLine(tt.line); got != "" {
				t.Errorf("UserMessageFromLine() = %q, want \"\"", got)
			}
		})
	}
}

func TestUserMessageFromLine_ReturnsEmptyForNonUserMessageLines(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"task_started event", taskStartedEventLine},
		{"token_count event", tokenCountEventLine},
		{"item_completed carrying an AgentMessage", agentMessageItemCompletedLine},
		{"item_completed carrying a CommandExecution", commandExecItemCompletedLine},
		{"session_meta", sessionMetaLine},
		{"empty line", ""},
		{"whitespace line", "   "},
		{"malformed JSON", `{"type":"event_msg","payload":{`},
		{"JSON array", `[1,2,3]`},
		{"JSON null", `null`},
		{"event_msg with no payload", `{"type":"event_msg"}`},
		{"user_message with empty message", `{"type":"event_msg","payload":{"type":"user_message","message":""}}`},
		{"user_message with whitespace-only message", `{"type":"event_msg","payload":{"type":"user_message","message":"   \n\t "}}`},
		{"UserMessage with empty content", `{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[]}}}`},
		{"UserMessage with only non-text parts", `{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"skill","name":"retro"}]}}}`},
		{"UserMessage with whitespace-only text", `{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"text","text":"  "}]}}}`},
		{"item_completed with no item", `{"type":"event_msg","payload":{"type":"item_completed"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UserMessageFromLine(tt.line); got != "" {
				t.Errorf("UserMessageFromLine() = %q, want \"\"", got)
			}
		})
	}
}

func TestUserMessageFromLine_TrimsSurroundingWhitespace(t *testing.T) {
	line := `{"type":"event_msg","payload":{"type":"user_message","message":"  fix the flaky test \n"}}`
	got := UserMessageFromLine(line)
	want := "fix the flaky test"
	if got != want {
		t.Errorf("UserMessageFromLine() = %q, want %q", got, want)
	}
}

// EventMsgUserText is the shape authority a caller holding an already-decoded
// event_msg payload (the rollout parser) can reuse, so it is exercised on its own.
func TestEventMsgUserText(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{"old era user_message", `{"type":"user_message","message":"add a retry"}`, "add a retry"},
		{"modern item_completed UserMessage", `{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"text","text":"add a retry"}]}}`, "add a retry"},
		{"agent message", `{"type":"item_completed","item":{"type":"AgentMessage","content":[{"type":"Text","text":"sure"}]}}`, ""},
		{"task_started", `{"type":"task_started","turn_id":"x"}`, ""},
		{"malformed payload", `{`, ""},
		{"empty payload", ``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EventMsgUserText([]byte(tt.payload)); got != tt.want {
				t.Errorf("EventMsgUserText() = %q, want %q", got, tt.want)
			}
		})
	}
}
