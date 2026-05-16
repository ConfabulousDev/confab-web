package session

import (
	"reflect"
	"sort"
	"testing"
)

// TestExpandProviderLegacy locks the canonical → (canonical + legacy) expansion
// rule that lets `?provider=claude-code` match rows with `session_type='Claude Code'`.
// The result is fed into `s.session_type = ANY(...)`, so order does not matter;
// the test compares sorted slices.
//
// Sibling to db.NormalizeProvider — when the post-rollout backfill collapses
// 'Claude Code' rows to 'claude-code', this helper and its test go away together.
func TestExpandProviderLegacy(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty input returns empty",
			input: nil,
			want:  []string{},
		},
		{
			name:  "claude-code expands to canonical plus legacy",
			input: []string{"claude-code"},
			want:  []string{"claude-code", "Claude Code"},
		},
		{
			name:  "codex has no legacy form, passes through",
			input: []string{"codex"},
			want:  []string{"codex"},
		},
		{
			name:  "both providers includes both, plus claude-code legacy",
			input: []string{"claude-code", "codex"},
			want:  []string{"claude-code", "Claude Code", "codex"},
		},
		{
			name:  "ordering of inputs does not change the canonical/legacy expansion",
			input: []string{"codex", "claude-code"},
			want:  []string{"codex", "claude-code", "Claude Code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandProviderLegacy(tt.input)
			gotSorted := append([]string{}, got...)
			wantSorted := append([]string{}, tt.want...)
			sort.Strings(gotSorted)
			sort.Strings(wantSorted)
			if !reflect.DeepEqual(gotSorted, wantSorted) {
				t.Errorf("expandProviderLegacy(%v) = %v, want %v (order-insensitive)",
					tt.input, got, tt.want)
			}
		})
	}
}
