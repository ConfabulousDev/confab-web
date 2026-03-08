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

