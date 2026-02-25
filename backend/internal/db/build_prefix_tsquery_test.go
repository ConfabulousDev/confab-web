package db

import "testing"

func TestBuildPrefixTsquery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single word", "auth", "auth:*"},
		{"two words", "auth flow", "auth:* & flow:*"},
		{"three words", "fix login bug", "fix:* & login:* & bug:*"},
		{"extra whitespace", "  auth   flow  ", "auth:* & flow:*"},
		{"special chars stripped", "auth&flow", "authflow:*"},
		{"pipe and parens stripped", "auth|flow()", "authflow:*"},
		{"colons and quotes stripped", "auth:'test'", "authtest:*"},
		{"backslash stripped", `auth\flow`, "authflow:*"},
		{"only special chars", "&|!()", ""},
		{"mixed normal and special", "auth &fix", "auth:* & fix:*"},
		{"numbers preserved", "cf280", "cf280:*"},
		{"hyphen preserved", "auth-flow", "auth-flow:*"},
		{"dots preserved", "file.go", "file.go:*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPrefixTsquery(tt.input)
			if result != tt.expected {
				t.Errorf("BuildPrefixTsquery(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
