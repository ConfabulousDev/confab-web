package analytics

import (
	"strings"
	"testing"
)

// nfbe: Cursor user `text` blocks arrive wrapped in an envelope. The human
// prompt lives in `<user_query>…</user_query>`; injected context arrives as
// sibling top-level tagged blocks. The search index, the smart-recap
// transcript, and the synced first_user_message must carry ONLY the prompt —
// the envelope tags must never leak. Mirrors the frontend parseCursorUserText.

func TestParseCursorUserPrompt(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"extracts and trims user_query", "<user_query>\ndoes gh repo have alerts?\n</user_query>", "does gh repo have alerts?"},
		{"strips the envelope tags", "<user_query>hello</user_query>", "hello"},
		{"falls back to raw when no tag (trimmed)", "  plain prompt  ", "plain prompt"},
		{"falls back to raw on unclosed tag", "<user_query>unterminated body", "<user_query>unterminated body"},
		{"concatenates multiple user_query blocks", "<user_query>a</user_query>\n<user_query>b</user_query>", "a\nb"},
		{"keeps [Image #N] literal inside the prompt", "<user_query>fix this [Image #1] please</user_query>", "fix this [Image #1] please"},
		{"strips leading context block, keeps prompt", "<manually_attached_skills>\nbody\n</manually_attached_skills>\n<user_query>\ngo\n</user_query>", "go"},
		{"empty query yields empty prompt", "<user_query></user_query>", ""},
		{"whitespace-only query yields empty prompt", "<user_query>   \n  </user_query>", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseCursorUserPrompt(tc.in); got != tc.want {
				t.Errorf("parseCursorUserPrompt(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestExtractCursorSearchTextStripsUserQueryEnvelope verifies the <user_query>
// envelope (and sibling context tags) never lands in the search index, while
// the human prompt inside it still does.
func TestExtractCursorSearchTextStripsUserQueryEnvelope(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	text := extractCursorSearchText(mainOnly(messages))

	if strings.Contains(text, "<user_query>") || strings.Contains(text, "</user_query>") {
		t.Errorf("search text must not contain the <user_query> envelope tags:\n%s", text)
	}
	if strings.Contains(text, "manually_attached_skills") {
		t.Errorf("search text must not contain injected-context tags:\n%s", text)
	}
	if !strings.Contains(text, "Add input validation to the session handler") {
		t.Error("search text must keep the human prompt extracted from <user_query>")
	}
}

// TestPrepareCursorTranscriptStripsUserQueryEnvelope verifies the smart-recap
// XML carries the extracted prompt, not the raw envelope tags.
func TestPrepareCursorTranscriptStripsUserQueryEnvelope(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	xml, _ := PrepareCursorTranscript(messages)

	if strings.Contains(xml, "<user_query>") || strings.Contains(xml, "&lt;user_query&gt;") {
		t.Errorf("transcript must not contain the <user_query> envelope (raw or escaped):\n%s", xml)
	}
	if strings.Contains(xml, "manually_attached_skills") {
		t.Errorf("transcript must not contain injected-context tags:\n%s", xml)
	}
	if !strings.Contains(xml, "Add input validation to the session handler") {
		t.Error("transcript must keep the human prompt extracted from <user_query>")
	}
}
