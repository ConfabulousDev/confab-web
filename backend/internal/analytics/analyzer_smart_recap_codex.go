package analytics

import (
	"fmt"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// PrepareCodexTranscript builds an XML transcript for the smart recap LLM,
// reusing the same envelope as Claude's PrepareTranscript so the prompt
// accepts it without changes. Truncation honors DefaultFormatConfig() for
// parity with Claude.
//
// Message-ID anchoring (intentional limitation, CF-447):
// Codex rollout JSONL has no stable per-message identifier the frontend can
// anchor a deep link on. The idMap entries returned here are synthetic
// placeholders ("codex-msg-N", "codex-tool-N", "codex-tool-result-N",
// "codex-compaction-N") that only serve the LLM's internal cross-references
// within a single recap generation. To keep these synthetic ids from leaking
// into the rendered card, codexProvider.ClearMessageIDs() returns true; the
// generator calls SmartRecapGenerator.GenerateWithMessageIDClearing, which
// zeroes every AnnotatedItem.MessageID after the LLM returns. The frontend
// SmartRecapCard.tsx MessageLink component short-circuits on an empty
// message_id and renders the item as plain text. This is correct behavior,
// not a bug — Claude transcripts get clickable deep links because Claude
// has real per-message UUIDs; Codex transcripts do not.
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
