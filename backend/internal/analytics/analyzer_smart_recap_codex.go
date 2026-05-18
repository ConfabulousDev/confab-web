package analytics

import (
	"fmt"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// PrepareCodexTranscript builds an XML transcript for the smart recap LLM,
// reusing the same envelope as Claude's PrepareTranscript so the prompt
// accepts it without changes. The idMap entries are synthetic placeholders;
// codexProvider reports ClearMessageIDs=true so the frontend treats items as
// plain text. Truncation honors DefaultFormatConfig() for parity with Claude.
func PrepareCodexTranscript(rollouts []*codex.ParsedRollout) (string, map[int]string) {
	cfg := DefaultFormatConfig()
	var b strings.Builder
	idMap := make(map[int]string)
	counter := 0

	b.WriteString("<transcript>\n")
	for _, rollout := range rollouts {
		if rollout == nil {
			continue
		}
		for _, turn := range rollout.Turns {
			emitCodexTurn(&b, &counter, idMap, turn, cfg)
		}
		// Compactions surface only as markers; timestamps are intentionally omitted.
		for range rollout.Compactions {
			counter++
			idMap[counter] = fmt.Sprintf("codex-compaction-%d", counter)
			fmt.Fprintf(&b, "<compaction id=\"%d\" />\n", counter)
		}
	}
	b.WriteString("</transcript>")
	return b.String(), idMap
}

// emitCodexTurn writes one turn's items in JSONL order.
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

// xmlEscape escapes the five XML-reserved chars. Lightweight — sufficient
// because the smart recap prompt treats content as opaque text.
var xmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\"", "&quot;",
	"'", "&apos;",
)

func xmlEscape(s string) string { return xmlReplacer.Replace(s) }
