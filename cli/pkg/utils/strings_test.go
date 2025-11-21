package utils

import "testing"

func TestTruncateSecret(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		prefixLen int
		suffixLen int
		want      string
	}{
		{
			name:      "normal API key",
			input:     "sk_live_abcdefghijklmnopqrstuvwxyz123456",
			prefixLen: 8,
			suffixLen: 4,
			want:      "sk_live_...3456",
		},
		{
			name:      "longer prefix for status display",
			input:     "sk_live_abcdefghijklmnopqrstuvwxyz123456",
			prefixLen: 12,
			suffixLen: 4,
			want:      "sk_live_abcd...3456",
		},
		{
			name:      "exactly minimum length",
			input:     "abcdefghijkl",
			prefixLen: 8,
			suffixLen: 4,
			want:      "abcdefgh...ijkl",
		},
		{
			name:      "too short - masks",
			input:     "short",
			prefixLen: 8,
			suffixLen: 4,
			want:      "***",
		},
		{
			name:      "empty string",
			input:     "",
			prefixLen: 8,
			suffixLen: 4,
			want:      "(empty)",
		},
		{
			name:      "one character",
			input:     "x",
			prefixLen: 8,
			suffixLen: 4,
			want:      "***",
		},
		{
			name:      "just under minimum",
			input:     "abcdefghijk", // 11 chars, need 12
			prefixLen: 8,
			suffixLen: 4,
			want:      "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateSecret(tt.input, tt.prefixLen, tt.suffixLen)
			if got != tt.want {
				t.Errorf("TruncateSecret(%q, %d, %d) = %q, want %q",
					tt.input, tt.prefixLen, tt.suffixLen, got, tt.want)
			}
		})
	}
}

// TestTruncateSecretNoPanic ensures no panic on edge cases
func TestTruncateSecretNoPanic(t *testing.T) {
	// These should never panic
	inputs := []string{"", "a", "ab", "abc", "abcd", "abcde"}
	for _, input := range inputs {
		// Should not panic regardless of prefix/suffix lengths
		_ = TruncateSecret(input, 0, 0)
		_ = TruncateSecret(input, 1, 1)
		_ = TruncateSecret(input, 8, 4)
		_ = TruncateSecret(input, 12, 4)
		_ = TruncateSecret(input, 100, 100)
	}
}
