package analytics

import (
	"fmt"
	"strings"
)

// PrepareCursorTranscript renders a parsed Cursor session as the XML transcript
// the smart-recap LLM consumes. Cursor messages have no stable ids (the
// provider's ClearMessageIDs returns true), so the returned idMap is always
// empty — the smart-recap layer drops the sequential ids rather than anchoring
// them to frontend message IDs. The sequential id attributes are still emitted
// so the LLM can cross-reference elements within the transcript.
//
// Cursor records tool INPUTS only (no tool_result blocks), so each tool call is
// rendered as a single <tool> element with a compact input summary and no
// <tool_result>.
//
// Layout:
//
//	<transcript>
//	<user id="1">prompt text</user>
//	<assistant id="2">assistant text</assistant>
//	<tool id="3" name="StrReplace">internal/api/session_handler.go</tool>
//	</transcript>
func PrepareCursorTranscript(messages []*CursorMessage) (string, map[int]string) {
	cfg := DefaultFormatConfig()
	idMap := make(map[int]string)
	counter := 0
	var b strings.Builder

	b.WriteString("<transcript>\n")
	for _, msg := range messages {
		text := joinCursorText(msg.Content)
		switch msg.Role {
		case "user":
			if text == "" {
				continue
			}
			counter++
			fmt.Fprintf(&b, "<user id=\"%d\">%s</user>\n",
				counter, xmlEscape(cfg.truncate(text, cfg.MaxUserChars)))

		case "assistant":
			if text != "" {
				counter++
				fmt.Fprintf(&b, "<assistant id=\"%d\">%s</assistant>\n",
					counter, xmlEscape(cfg.truncate(text, cfg.MaxAssistantChars)))
			}
			for _, block := range msg.Content {
				if block.Type != "tool_use" || block.Name == "" {
					continue
				}
				counter++
				fmt.Fprintf(&b, "<tool id=\"%d\" name=\"%s\">%s</tool>\n",
					counter, xmlEscape(block.Name),
					xmlEscape(cfg.truncate(cursorToolInputSummary(block), cfg.MaxAssistantChars)))
			}
		}
	}
	b.WriteString("</transcript>")
	return b.String(), idMap
}

// joinCursorText concatenates the text of every text block in a message, then
// strips Cursor's native bare `[REDACTED]` placeholder (fa3h). When only the
// placeholder was present the result is "" — PrepareCursorTranscript then omits
// the empty assistant element while still rendering the line's tool calls.
func joinCursorText(content []CursorBlock) string {
	var segments []string
	for _, block := range content {
		if block.Type == "text" && block.Text != "" {
			segments = append(segments, block.Text)
		}
	}
	return cleanCursorAssistantText(strings.Join(segments, "\n"))
}

// cursorToolInputSummary renders a tool call's most salient input as a compact
// string for the transcript, preferring the file path, then a shell command,
// then a search pattern/query.
func cursorToolInputSummary(block CursorBlock) string {
	for _, key := range []string{"path", "command", "pattern", "glob_pattern", "query", "search_term", "description"} {
		if v := block.stringInput(key); v != "" {
			return v
		}
	}
	return ""
}
