package validation

import (
	"strings"
	"testing"
)

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantErr   bool
	}{
		{
			name:      "valid session ID",
			sessionID: "session-123-abc",
			wantErr:   false,
		},
		{
			name:      "empty session ID",
			sessionID: "",
			wantErr:   true,
		},
		{
			name:      "session ID too long",
			sessionID: strings.Repeat("a", 257),
			wantErr:   true,
		},
		{
			name:      "session ID at max length",
			sessionID: strings.Repeat("a", 256),
			wantErr:   false,
		},
		{
			name:      "session ID with spaces",
			sessionID: "session 123",
			wantErr:   false, // Spaces are valid UTF-8
		},
		{
			name:      "session ID with special chars",
			sessionID: "session-123_abc.xyz",
			wantErr:   false,
		},
		{
			name:      "invalid UTF-8",
			sessionID: string([]byte{0xff, 0xfe, 0xfd}),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionID(tt.sessionID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSessionID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateShareToken(t *testing.T) {
	tests := []struct {
		name       string
		shareToken string
		wantErr    bool
	}{
		{
			name:       "valid share token",
			shareToken: "abcdef0123456789abcdef0123456789",
			wantErr:    false,
		},
		{
			name:       "valid share token uppercase",
			shareToken: "ABCDEF0123456789ABCDEF0123456789",
			wantErr:    false,
		},
		{
			name:       "valid share token mixed case",
			shareToken: "AbCdEf0123456789aBcDeF0123456789",
			wantErr:    false,
		},
		{
			name:       "empty share token",
			shareToken: "",
			wantErr:    true,
		},
		{
			name:       "share token too short",
			shareToken: "abcdef0123456789",
			wantErr:    true,
		},
		{
			name:       "share token too long",
			shareToken: "abcdef0123456789abcdef0123456789abc",
			wantErr:    true,
		},
		{
			name:       "share token with non-hex chars",
			shareToken: "abcdefg123456789abcdef0123456789",
			wantErr:    true,
		},
		{
			name:       "share token with spaces",
			shareToken: "abcdef 123456789abcdef0123456789",
			wantErr:    true,
		},
		{
			name:       "share token with dashes",
			shareToken: "abcdef-123456789-abcdef-123456789",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateShareToken(tt.shareToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateShareToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
