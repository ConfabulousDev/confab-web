package analytics

import (
	"strings"
	"unicode/utf8"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// codexToolArgPreviewBytes caps how many bytes of a Codex tool-call argument
// string we surface to the search index.
const codexToolArgPreviewBytes = 200

// ExtractCodexUserMessagesText builds the Weight C search-index content for
// a Codex session: user messages, assistant final-phase text, and tool-call
// summaries ("<tool_name> <args>") across all rollouts. Honors
// maxUserMessagesBytes with UTF-8-safe truncation, mirroring
// UserMessagesBuilder.
func ExtractCodexUserMessagesText(rollouts []*codex.ParsedRollout) string {
	if len(rollouts) == 0 {
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

	for _, rollout := range rollouts {
		if rollout == nil {
			continue
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
	}
	return b.String()
}
