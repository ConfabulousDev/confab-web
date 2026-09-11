package analytics

import (
	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// codexToolArgPreviewBytes caps how many bytes of a Codex tool-call argument
// string we surface to the search index.
const codexToolArgPreviewBytes = 200

// ExtractCodexUserMessagesText builds the Weight C search-index content for
// a Codex session: user messages, assistant final-phase text, and tool-call
// summaries ("<tool_name> <args>") across all rollouts. Honors
// maxUserMessagesBytes with UTF-8-safe truncation via searchTextBuilder
// (shared with OpenCode).
func ExtractCodexUserMessagesText(rollouts []*codex.ParsedRollout) string {
	if len(rollouts) == 0 {
		return ""
	}
	b := newSearchTextBuilder(maxUserMessagesBytes)
	for _, rollout := range rollouts {
		if rollout == nil {
			continue
		}
		for _, turn := range rollout.Turns {
			for _, m := range turn.UserMessages {
				b.Add(m.Text)
			}
			for _, m := range turn.AssistantMessages {
				if m.Phase == "final" && m.Text != "" {
					b.Add(m.Text)
				}
			}
			for _, tc := range turn.ToolCalls {
				args := tc.Arguments
				if len(args) > codexToolArgPreviewBytes {
					args = args[:codexToolArgPreviewBytes]
				}
				b.Add(tc.Name + " " + args)
			}
			// <=0.130.0 rollouts reach edited paths through the apply_patch
			// envelope in Arguments above. >=0.149.1 rollouts carry them only
			// on FileChange items, so index them here or the two eras are not
			// searchable alike.
			for _, edit := range turn.FileEdits {
				b.Add(edit.Path)
			}
		}
	}
	return b.String()
}
