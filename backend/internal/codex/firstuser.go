package codex

import (
	"encoding/json"
	"strings"
)

// User-message extraction from the event_msg stream (tgxn).
//
// Codex emits the text a human typed twice: once as an `event_msg` the CLI
// renders, and once as a user-role `response_item` sent to the model. Only the
// event_msg stream is clean — across every local rollout inspected, the first
// user-role response_item was an `<environment_context>` block or an
// `# AGENTS.md instructions for …` injection, never the human prompt. So these
// helpers read event_msg and nothing else.
//
// The event_msg user-message shape changed across CLI versions and both are
// still uploaded (old rollouts live on disk forever), so both are handled here:
//
//	<=0.130.0  {"type":"event_msg","payload":{"type":"user_message","message":"…"}}
//	>=0.149.1  {"type":"event_msg","payload":{"type":"item_completed",
//	            "item":{"type":"UserMessage","content":[{"type":"text","text":"…"}]}}}
//
// This file is the single home for that shape knowledge: callers that need the
// human prompt (today, the sync ingest path) route through it rather than
// decoding the payload a second time.

// eventUserMessagePayload is the <=0.130.0 event_msg.user_message payload.
type eventUserMessagePayload struct {
	Message string `json:"message"`
}

// eventItemCompletedPayload is the >=0.149.1 event_msg.item_completed payload.
// Only the fields needed to recognize and read a UserMessage item are decoded;
// AgentMessage, CommandExecution and friends fall out as an item type mismatch.
type eventItemCompletedPayload struct {
	Item *struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"item"`
}

// EventMsgUserText returns the human-typed text carried by an `event_msg`
// payload, or "" when the payload is any other event (including an
// item_completed wrapping something that is not a UserMessage) or fails to
// decode. Surrounding whitespace is trimmed, so a whitespace-only message
// returns "" — callers use the empty string to mean "nothing derivable" and
// must not write it anywhere a real message is expected.
//
// Takes the payload rather than the whole line so a caller that has already
// peeled the envelope (the rollout parser) can reuse it directly.
func EventMsgUserText(payload json.RawMessage) string {
	var pt payloadType
	if err := json.Unmarshal(payload, &pt); err != nil {
		return ""
	}

	switch pt.Type {
	case "user_message":
		var p eventUserMessagePayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return ""
		}
		return strings.TrimSpace(p.Message)

	case "item_completed":
		var p eventItemCompletedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return ""
		}
		if p.Item == nil || p.Item.Type != "UserMessage" {
			return ""
		}
		// content[] mixes typed parts — real rollouts carry `skill` parts
		// alongside `text`. Keep the text, joined in order; the case-insensitive
		// match tolerates the capitalized "Text" sibling item types already use.
		var parts []string
		for _, c := range p.Item.Content {
			if !strings.EqualFold(c.Type, "text") {
				continue
			}
			if text := strings.TrimSpace(c.Text); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}

	return ""
}

// UserMessageFromLine returns the human-typed text carried by one rollout JSONL
// line, or "" when the line is not an `event_msg` user message — including
// response_item lines, malformed JSON, and blank lines. It never returns
// injected context.
func UserMessageFromLine(line string) string {
	var l rawLine
	if err := json.Unmarshal([]byte(line), &l); err != nil {
		return ""
	}
	if l.Type != "event_msg" {
		return ""
	}
	return EventMsgUserText(l.Payload)
}
