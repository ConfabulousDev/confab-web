package analytics

import (
	"strings"
	"unicode/utf8"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// codexToolArgPreviewBytes is how many bytes of a Codex tool-call arg string
// we surface to the search index. Short enough to keep the index lean, long
// enough to catch file paths and short flags.
const codexToolArgPreviewBytes = 200

// ExtractCodexUserMessagesText builds the Weight C search-index content for
// a Codex session. Per CF-350 §2e the index includes:
//
//   - User messages (the parser already stripped <environment_context>)
//   - Assistant `final`-phase text (commentary is filler — excluded)
//   - Tool call summaries: "<tool_name> <args truncated to 200 chars>"
//
// Honors maxUserMessagesBytes (500 KB) so a long rollout doesn't blow the
// index up disproportionately. Mirrors UserMessagesBuilder's byte-cap
// truncation semantics, including UTF-8-safe boundary alignment.
func ExtractCodexUserMessagesText(rollout *codex.ParsedRollout) string {
	if rollout == nil {
		return ""
	}
	var b strings.Builder
	totalBytes := 0
	full := false

	add := func(text string) {
		if full || text == "" {
			return
		}
		if totalBytes+len(text)+1 > maxUserMessagesBytes {
			remaining := maxUserMessagesBytes - totalBytes
			if remaining > 1 && b.Len() > 0 {
				b.WriteByte('\n')
				remaining--
			}
			if remaining > 0 && remaining < len(text) {
				// Step back to the nearest UTF-8 rune boundary.
				for remaining > 0 && !utf8.RuneStart(text[remaining]) {
					remaining--
				}
				b.WriteString(text[:remaining])
			} else if remaining > 0 {
				b.WriteString(text)
			}
			full = true
			return
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
			totalBytes++
		}
		b.WriteString(text)
		totalBytes += len(text)
	}

	for _, turn := range rollout.Turns {
		for _, m := range turn.UserMessages {
			add(m.Text)
		}
		for _, m := range turn.AssistantMessages {
			if m.Phase == "final" && m.Text != "" {
				add(m.Text)
			}
		}
		for _, tc := range turn.ToolCalls {
			args := tc.Arguments
			if len(args) > codexToolArgPreviewBytes {
				args = args[:codexToolArgPreviewBytes]
			}
			add(tc.Name + " " + args)
		}
	}
	return b.String()
}
