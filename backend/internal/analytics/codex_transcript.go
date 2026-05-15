package analytics

import (
	"fmt"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// PrepareCodexTranscript builds an XML transcript suitable for the smart
// recap LLM. Reuses the same <transcript>/<user>/<assistant>/<tool>/<tool_result>
// envelope as Claude's PrepareTranscript so the existing prompt accepts it
// without changes (per CF-350 interview decision: simple path, no prompt variants).
//
// The returned idMap maps sequential integer ids to synthetic
// "codex-<seq>" placeholders. These ids are NOT used by the frontend for
// deep-linking — Codex messages have no stable id. The Codex precompute
// path sets GenerateInput.ClearMessageIDs=true so the resulting card has
// empty message_ids, and SmartRecapCard.tsx's `if (!item.message_id)`
// short-circuit renders items as plain text.
//
// Truncation honors DefaultFormatConfig() for parity with the Claude path.
func PrepareCodexTranscript(rollout *codex.ParsedRollout) (string, map[int]string) {
	cfg := DefaultFormatConfig()
	var b strings.Builder
	idMap := make(map[int]string)
	counter := 0

	b.WriteString("<transcript>\n")
	if rollout != nil {
		for _, turn := range rollout.Turns {
			emitCodexTurn(&b, &counter, idMap, turn, cfg)
		}
		// Compactions: only the marker matters to the recap prompt; timestamps
		// are intentionally not surfaced.
		for range rollout.Compactions {
			counter++
			idMap[counter] = fmt.Sprintf("codex-compaction-%d", counter)
			fmt.Fprintf(&b, "<compaction id=\"%d\" />\n", counter)
		}
	}
	b.WriteString("</transcript>")
	return b.String(), idMap
}

// emitCodexTurn writes one Codex turn's items in JSONL order: user messages,
// assistant messages (commentary + final), and tool calls with their outputs.
func emitCodexTurn(b *strings.Builder, counter *int, idMap map[int]string, turn codex.Turn, cfg FormatConfig) {
	for _, m := range turn.UserMessages {
		*counter++
		idMap[*counter] = fmt.Sprintf("codex-msg-%d", *counter)
		fmt.Fprintf(b, "<user id=\"%d\">%s</user>\n",
			*counter, xmlEscape(cfg.truncate(m.Text, cfg.MaxUserChars)))
	}
	for _, m := range turn.AssistantMessages {
		*counter++
		idMap[*counter] = fmt.Sprintf("codex-msg-%d", *counter)
		phase := m.Phase
		if phase == "" {
			phase = "final"
		}
		fmt.Fprintf(b, "<assistant id=\"%d\" phase=\"%s\">%s</assistant>\n",
			*counter, phase, xmlEscape(cfg.truncate(m.Text, cfg.MaxAssistantChars)))
	}
	for _, tc := range turn.ToolCalls {
		*counter++
		idMap[*counter] = fmt.Sprintf("codex-tool-%d", *counter)
		toolID := *counter
		fmt.Fprintf(b, "<tool id=\"%d\" name=\"%s\">%s</tool>\n",
			toolID, xmlEscape(tc.Name),
			xmlEscape(cfg.truncate(tc.Arguments, cfg.MaxAssistantChars)))
		if tc.Output != "" {
			*counter++
			idMap[*counter] = fmt.Sprintf("codex-tool-result-%d", *counter)
			fmt.Fprintf(b, "<tool_result id=\"%d\" tool_id=\"%d\">%s</tool_result>\n",
				*counter, toolID,
				xmlEscape(cfg.truncate(tc.Output, cfg.MaxAssistantChars)))
		}
	}
}

// xmlEscape escapes the five XML-reserved chars. Lightweight (no encoding/xml
// dependency) — sufficient because the smart recap prompt treats content as
// opaque text, not a structured DOM.
var xmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\"", "&quot;",
	"'", "&apos;",
)

func xmlEscape(s string) string { return xmlReplacer.Replace(s) }
