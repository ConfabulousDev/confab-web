package cmd

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"", 0, false},
		{"5d", 5 * 24 * time.Hour, false},
		{"12h", 12 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"1d", 24 * time.Hour, false},
		{"invalid", 0, true},
		{"5x", 0, true},
		{"d", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("For input %q: expected %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}
