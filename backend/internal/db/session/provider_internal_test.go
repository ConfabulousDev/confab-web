package session

import "testing"

// TestNormalizeProvider locks the legacy-to-canonical mapping for the
// sessions.session_type column. The transformation is intentionally
// minimal — only the one historical display value 'Claude Code' is
// rewritten — and is applied at every Scan path until a future
// one-time backfill PR removes it (see comment in provider.go).
func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "legacy display form maps to canonical claude-code",
			in:   "Claude Code",
			want: "claude-code",
		},
		{
			name: "canonical claude-code passes through unchanged",
			in:   "claude-code",
			want: "claude-code",
		},
		{
			name: "canonical codex passes through unchanged",
			in:   "codex",
			want: "codex",
		},
		{
			name: "empty string passes through unchanged",
			in:   "",
			want: "",
		},
		{
			name: "unknown value passes through unchanged (DB is source of truth)",
			in:   "gemini",
			want: "gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeProvider(tt.in)
			if got != tt.want {
				t.Errorf("normalizeProvider(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
