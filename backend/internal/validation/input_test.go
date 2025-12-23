package validation

import (
	"strings"
	"testing"
)

func TestValidateExternalID(t *testing.T) {
	tests := []struct {
		name       string
		externalID string
		wantErr    bool
	}{
		{
			name:       "valid external ID",
			externalID: "session-123-abc",
			wantErr:    false,
		},
		{
			name:       "empty external ID",
			externalID: "",
			wantErr:    true,
		},
		{
			name:       "external ID too long",
			externalID: strings.Repeat("a", MaxExternalIDLength+1),
			wantErr:    true,
		},
		{
			name:       "external ID at max length",
			externalID: strings.Repeat("a", MaxExternalIDLength),
			wantErr:    false,
		},
		{
			name:       "external ID with spaces",
			externalID: "session 123",
			wantErr:    false, // Spaces are valid UTF-8
		},
		{
			name:       "external ID with special chars",
			externalID: "session-123_abc.xyz",
			wantErr:    false,
		},
		{
			name:       "invalid UTF-8",
			externalID: string([]byte{0xff, 0xfe, 0xfd}),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExternalID(tt.externalID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExternalID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{
			name:     "valid hostname",
			hostname: "macbook.local",
			wantErr:  false,
		},
		{
			name:     "empty hostname",
			hostname: "",
			wantErr:  false, // Empty is allowed (optional field)
		},
		{
			name:     "hostname at max length",
			hostname: strings.Repeat("a", MaxHostnameLength),
			wantErr:  false,
		},
		{
			name:     "hostname too long",
			hostname: strings.Repeat("a", MaxHostnameLength+1),
			wantErr:  true,
		},
		{
			name:     "hostname with special chars",
			hostname: "my-laptop.home.local",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.hostname)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostname() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "valid username",
			username: "jackie",
			wantErr:  false,
		},
		{
			name:     "empty username",
			username: "",
			wantErr:  false, // Empty is allowed (optional field)
		},
		{
			name:     "username at max length",
			username: strings.Repeat("a", MaxUsernameLength),
			wantErr:  false,
		},
		{
			name:     "username too long",
			username: strings.Repeat("a", MaxUsernameLength+1),
			wantErr:  true,
		},
		{
			name:     "username with special chars",
			username: "user_name-123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TODO(2026-Q2): Remove truncation tests when grace period ends
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "string under limit unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "string at limit unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "string over limit truncated",
			input:    "hello world",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "empty string unchanged",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "multi-byte UTF-8 not split",
			input:    "hello 世界",
			maxLen:   8, // Would cut '世' in half at byte 8
			expected: "hello ",
		},
		{
			name:     "multi-byte UTF-8 preserved when fits",
			input:    "hello 世界",
			maxLen:   9, // Exactly "hello 世" (6 + 3 bytes)
			expected: "hello 世",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestTruncateSyncFileName(t *testing.T) {
	t.Run("under limit unchanged", func(t *testing.T) {
		input := "file.txt"
		result := TruncateSyncFileName(input)
		if result != input {
			t.Errorf("expected %q, got %q", input, result)
		}
	})

	t.Run("over limit truncated", func(t *testing.T) {
		input := strings.Repeat("a", MaxSyncFileNameLength+100)
		result := TruncateSyncFileName(input)
		if len(result) != MaxSyncFileNameLength {
			t.Errorf("expected length %d, got %d", MaxSyncFileNameLength, len(result))
		}
	})
}

func TestTruncateSummary(t *testing.T) {
	t.Run("under limit unchanged", func(t *testing.T) {
		input := "A short summary"
		result := TruncateSummary(input)
		if result != input {
			t.Errorf("expected %q, got %q", input, result)
		}
	})

	t.Run("over limit truncated", func(t *testing.T) {
		input := strings.Repeat("a", MaxSummaryLength+100)
		result := TruncateSummary(input)
		if len(result) != MaxSummaryLength {
			t.Errorf("expected length %d, got %d", MaxSummaryLength, len(result))
		}
	})
}

func TestTruncateFirstUserMessage(t *testing.T) {
	t.Run("under limit unchanged", func(t *testing.T) {
		input := "Hello, how can I help?"
		result := TruncateFirstUserMessage(input)
		if result != input {
			t.Errorf("expected %q, got %q", input, result)
		}
	})

	t.Run("over limit truncated", func(t *testing.T) {
		input := strings.Repeat("a", MaxFirstUserMessageLength+100)
		result := TruncateFirstUserMessage(input)
		if len(result) != MaxFirstUserMessageLength {
			t.Errorf("expected length %d, got %d", MaxFirstUserMessageLength, len(result))
		}
	})
}
