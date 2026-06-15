package admin

import "testing"

// TestVerifyConfirmation pins the typed-confirmation compare (kyrr): trim +
// case-insensitive, with an empty expected value never matching.
func TestVerifyConfirmation(t *testing.T) {
	cases := []struct {
		name     string
		expected string
		provided string
		want     bool
	}{
		{"exact match", "user@example.com", "user@example.com", true},
		{"case-insensitive", "User@Example.com", "user@example.COM", true},
		{"trims whitespace", "user@example.com", "  user@example.com  ", true},
		{"trims both sides", "  user@example.com  ", "user@example.com", true},
		{"numeric count match", "42", "42", true},
		{"mismatch", "user@example.com", "other@example.com", false},
		{"blank provided", "user@example.com", "", false},
		{"whitespace-only provided", "user@example.com", "   ", false},
		{"empty expected never matches", "", "", false},
		{"empty expected rejects any input", "", "anything", false},
		{"whitespace-only expected never matches", "   ", "anything", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := verifyConfirmation(c.expected, c.provided); got != c.want {
				t.Errorf("verifyConfirmation(%q, %q) = %v, want %v", c.expected, c.provided, got, c.want)
			}
		})
	}
}
