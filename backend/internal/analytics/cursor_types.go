package analytics

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Cursor agent-transcript JSONL wire shapes (gevp / fy5q spec).
//
// Cursor's local transcripts live under
// ~/.cursor/projects/<path>/agent-transcripts/<session-uuid>.jsonl and are
// uploaded verbatim through the sync protocol. The format is NOT Claude Code's
// TranscriptLine: conversation rows carry no top-level type/uuid/timestamp/
// usage/model fields, and tool_use blocks have no id.
//
// Two line shapes exist (verified against the committed fixture and a full
// local scan):
//
//	{"role":"user"|"assistant","message":{"content":[ <block>... ]}}  (conversation)
//	{"type":"turn_ended","status":"success"}                          (turn marker)
//	{"type":"turn_ended","status":"error","error":"..."}              (turn marker, error)
//
// Block shapes inside message.content:
//
//	{"type":"text","text":"..."}
//	{"type":"tool_use","name":"Read","input":{...}}   (no id)
//
// There are NO tool_result blocks anywhere — Cursor records tool inputs only,
// never outputs. Token, cost, and model data are absent entirely.

// cursorRawLine is the permissive union used to classify a single JSONL line
// into a conversation message vs. a turn_ended marker.
type cursorRawLine struct {
	// Conversation rows.
	Role    string `json:"role,omitempty"`
	Message *struct {
		Content []CursorBlock `json:"content"`
	} `json:"message,omitempty"`

	// turn_ended marker rows.
	Type   string `json:"type,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

// CursorMessage is a parsed conversation row (user or assistant). turn_ended
// markers are not CursorMessages — they are tracked separately on the rollout.
type CursorMessage struct {
	Role    string
	Content []CursorBlock
}

// CursorBlock is one content block. text blocks carry Text; tool_use blocks
// carry Name + Input. Input is kept as raw JSON and decoded per-tool because
// the input schema varies by tool (and carries variable optional flags).
type CursorBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// trailingBareRedacted matches a trailing bare `[REDACTED]` placeholder with any
// surrounding whitespace. It deliberately matches only the bare token (no
// colon/type) so Confab CLI `[REDACTED:TYPE]` markers — a different contract for
// a real redacted secret — stay intact.
var trailingBareRedacted = regexp.MustCompile(`\s*\[REDACTED\]\s*$`)

// cleanCursorAssistantText strips Cursor's native bare `[REDACTED]` placeholder
// from assistant text. Cursor's on-disk JSONL appends a bare `[REDACTED]` to
// nearly every assistant turn (parent wkkd) — either as the entire text block on
// tool-only turns or as a trailing suffix after the narrative; it is scaffolding
// noise, not a counted secret. This removes a trailing bare `[REDACTED]` and
// returns "" when the block's whole content was the placeholder, so callers can
// omit an empty assistant element. Confab CLI `[REDACTED:TYPE]` markers are
// never touched. Mirrors the frontend cleanCursorAssistantText.
func cleanCursorAssistantText(raw string) string {
	cleaned := trailingBareRedacted.ReplaceAllString(raw, "")
	if strings.TrimSpace(cleaned) == "" {
		return ""
	}
	return cleaned
}

// userQueryBlock matches one `<user_query>…</user_query>` envelope block (the
// human prompt). Cursor wraps every user `text` block in this envelope, often
// alongside injected-context tags (rules, attached files, skills). The prompt
// is the only human-authored part; everything else is scaffolding. `(?s)` lets
// `.` span newlines (queries are multi-line).
var userQueryBlock = regexp.MustCompile(`(?s)<user_query>(.*?)</user_query>`)

// parseCursorUserPrompt extracts the human prompt from a Cursor user text block
// by pulling the content of every `<user_query>…</user_query>` envelope and
// joining them with newlines (trimmed). When NO well-formed `<user_query>` tag
// is present (plain text, or an unclosed tag), it falls back to the raw text
// (trimmed) so content is never dropped. The surrounding injected-context tags
// are discarded — only the prompt reaches search, smart-recap, and the synced
// first_user_message title. Mirrors the frontend parseCursorUserText (which
// also returns the discarded sections for the collapsible-context UI; the
// backend has no consumer for those, so it returns the prompt only).
func parseCursorUserPrompt(raw string) string {
	matches := userQueryBlock.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(raw)
	}
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		parts = append(parts, strings.TrimSpace(m[1]))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// ExtractCursorUserPrompt is the exported entry point for parseCursorUserPrompt,
// used by the sync intake (internal/api) to clean a Cursor-supplied
// first_user_message before it is validated and stored as the session-list
// title. Same semantics: unwrap `<user_query>`, fall back to the raw text when
// no well-formed envelope is present.
func ExtractCursorUserPrompt(raw string) string {
	return parseCursorUserPrompt(raw)
}

// stringInput decodes the block's tool input and returns the string value at
// key, or "" when the input is absent, not an object, or the key is missing/
// non-string. Used for path extraction (input.path) and similar scalar fields.
func (b CursorBlock) stringInput(key string) string {
	if len(b.Input) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b.Input, &m); err != nil {
		return ""
	}
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
