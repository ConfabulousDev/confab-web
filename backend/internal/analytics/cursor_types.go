package analytics

import "encoding/json"

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
