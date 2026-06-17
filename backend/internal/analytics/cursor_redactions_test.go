package analytics

import (
	"strings"
	"testing"
)

// fa3h: Cursor's on-disk JSONL appends a bare `[REDACTED]` to nearly every
// assistant turn — either as a trailing suffix after narrative or as the entire
// text block on tool-only turns. It is scaffolding noise, not a counted secret,
// so it must be stripped before the text reaches the search index or the
// smart-recap transcript. Confab CLI `[REDACTED:TYPE]` markers are a different
// contract and must stay visible.

func TestCleanCursorAssistantText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"trailing double-newline suffix", "Checking the repo for open alerts.\n\n[REDACTED]", "Checking the repo for open alerts."},
		{"trailing single-newline suffix", "Fetching details.\n[REDACTED]", "Fetching details."},
		{"entire block is the placeholder", "[REDACTED]", ""},
		{"whitespace plus placeholder only", "  \n[REDACTED]\n  ", ""},
		{"carriage returns before placeholder", "Doing work.\r\n\r\n[REDACTED]", "Doing work."},
		{"no placeholder is untouched", "Just narrative.", "Just narrative."},
		{"typed marker preserved (embedded)", "See [REDACTED:GITHUB_TOKEN] in env.", "See [REDACTED:GITHUB_TOKEN] in env."},
		{"typed marker preserved (trailing)", "Token is [REDACTED:GITHUB_TOKEN]", "Token is [REDACTED:GITHUB_TOKEN]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanCursorAssistantText(tc.in); got != tc.want {
				t.Errorf("cleanCursorAssistantText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestExtractCursorSearchTextStripsRedacted verifies the bare [REDACTED]
// placeholder never lands in the search index, while the narrative around it
// still does.
func TestExtractCursorSearchTextStripsRedacted(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	text := extractCursorSearchText(mainOnly(messages))

	if strings.Contains(text, "[REDACTED]") {
		t.Errorf("search text must not contain the bare [REDACTED] placeholder:\n%s", text)
	}
	// The narrative that preceded a trailing [REDACTED] must survive.
	if !strings.Contains(text, "Checking the repo for open Dependabot alerts") {
		t.Error("search text must keep narrative that preceded a [REDACTED] suffix")
	}
}

// TestPrepareCursorTranscriptStripsRedacted verifies the smart-recap XML never
// carries the bare placeholder, and that a [REDACTED]-only assistant line emits
// no <assistant> element while its tool call still renders.
func TestPrepareCursorTranscriptStripsRedacted(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	xml, _ := PrepareCursorTranscript(messages)

	if strings.Contains(xml, "[REDACTED]") {
		t.Errorf("transcript must not contain the bare [REDACTED] placeholder:\n%s", xml)
	}
	// The [REDACTED]-only line's Shell tool call must still appear.
	if !strings.Contains(xml, "gh pr list --state open") {
		t.Error("tool call from a [REDACTED]-only line must still render")
	}
	// The narrative around a trailing [REDACTED] must survive.
	if !strings.Contains(xml, "Checking the repo for open Dependabot alerts") {
		t.Error("transcript must keep narrative that preceded a [REDACTED] suffix")
	}
}
