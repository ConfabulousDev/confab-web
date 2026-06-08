package analytics

import (
	"fmt"
	"strings"
)

// PrepareOpenCodeTranscript renders the session as the XML transcript the smart
// recap LLM consumes. Each emitted element gets a sequential id; idMap maps that
// id back to the real OpenCode message ULID so the recap can deep-link to a
// message in the frontend (OpenCode message IDs are stable, so the provider
// keeps them — see opencodeProvider.ClearMessageIDs).
//
// Multi-rollout layout (CF-539): iterates [main, ...subagents], emitting each
// rollout's turns inline in the same XML envelope, no <subagent> wrapper.
// Matches Claude's TranscriptBuilder.ProcessFile per-file pattern and Codex's
// PrepareCodexTranscript per-rollout loop. The counter is shared across
// rollouts so the LLM's cross-references resolve uniquely.
//
// Layout:
//
//	<transcript>
//	<user id="1">prompt text</user>
//	<assistant id="2">
//	<thinking>reasoning text</thinking>
//	assistant text
//	</assistant>
//	<tool id="3" name="Bash">{"command":"ls"}</tool>
//	<tool_result id="4" tool_id="3">file1\nfile2</tool_result>
//	<compaction id="5" />
//	</transcript>
func PrepareOpenCodeTranscript(rollouts [][]*OpenCodeMessage) (string, map[int]string) {
	cfg := DefaultFormatConfig()
	idMap := make(map[int]string)
	counter := 0
	var b strings.Builder

	anchor := func(id int, messageID string) {
		if messageID != "" {
			idMap[id] = messageID
		}
	}

	b.WriteString("<transcript>\n")
	for _, messages := range rollouts {
		for _, msg := range messages {
			parts := msg.Parts
			switch msg.Info.Role {
			case "user":
				text := joinOpenCodeText(parts, "text")
				if text == "" {
					continue
				}
				counter++
				anchor(counter, msg.Info.ID)
				fmt.Fprintf(&b, "<user id=\"%d\">%s</user>\n",
					counter, xmlEscape(cfg.truncate(text, cfg.MaxUserChars)))

			case "assistant":
				thinking := joinOpenCodeText(parts, "reasoning")
				text := joinOpenCodeText(parts, "text")
				if thinking != "" || text != "" {
					counter++
					anchor(counter, msg.Info.ID)
					fmt.Fprintf(&b, "<assistant id=\"%d\">\n", counter)
					if thinking != "" {
						fmt.Fprintf(&b, "<thinking>%s</thinking>\n",
							xmlEscape(cfg.truncate(thinking, cfg.MaxThinkingChars)))
					}
					if text != "" {
						b.WriteString(xmlEscape(cfg.truncate(text, cfg.MaxAssistantChars)))
						b.WriteByte('\n')
					}
					b.WriteString("</assistant>\n")
				}

				for _, p := range parts {
					switch p.Type {
					case "tool":
						state := p.State
						if state == nil || (state.Status != "completed" && state.Status != "error") {
							continue
						}
						counter++
						anchor(counter, msg.Info.ID)
						toolID := counter
						fmt.Fprintf(&b, "<tool id=\"%d\" name=\"%s\">%s</tool>\n",
							toolID, xmlEscape(p.Tool),
							xmlEscape(cfg.truncate(toolInputSummary(state), cfg.MaxAssistantChars)))
						if state.Output != "" {
							counter++
							anchor(counter, msg.Info.ID)
							fmt.Fprintf(&b, "<tool_result id=\"%d\" tool_id=\"%d\" status=\"%s\">%s</tool_result>\n",
								counter, toolID, state.Status,
								xmlEscape(cfg.truncate(state.Output, cfg.MaxAssistantChars)))
						}
					case "compaction":
						counter++
						anchor(counter, msg.Info.ID)
						fmt.Fprintf(&b, "<compaction id=\"%d\" />\n", counter)
					}
				}
			}
		}
	}
	b.WriteString("</transcript>")
	return b.String(), idMap
}

// joinOpenCodeText concatenates the text of every part of the given type.
func joinOpenCodeText(parts []OpenCodePart, partType string) string {
	var segments []string
	for _, p := range parts {
		if p.Type == partType && p.Text != "" {
			segments = append(segments, p.Text)
		}
	}
	return strings.Join(segments, "\n")
}

// toolInputSummary renders a tool call's input map as a compact string for the
// transcript. Falls back to the tool's title when there is no input.
func toolInputSummary(state *OpenCodeToolState) string {
	if state == nil {
		return ""
	}
	if cmd := getStringInput(state, "command"); cmd != "" {
		return cmd
	}
	if fp := getStringInput(state, "file_path"); fp != "" {
		return fp
	}
	return state.Title
}
