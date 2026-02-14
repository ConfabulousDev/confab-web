package validation

import (
	"testing"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		// Valid emails
		{
			name:  "valid simple email",
			email: "user@example.com",
			want:  true,
		},
		{
			name:  "valid email with subdomain",
			email: "user@mail.example.com",
			want:  true,
		},
		{
			name:  "valid email with plus",
			email: "user+tag@example.com",
			want:  true,
		},
		{
			name:  "valid email with dots",
			email: "first.last@example.com",
			want:  true,
		},
		{
			name:  "valid email with numbers",
			email: "user123@example456.com",
			want:  true,
		},
		{
			name:  "valid with whitespace (should be trimmed)",
			email: "  user@example.com  ",
			want:  true,
		},
		// Invalid emails
		{
			name:  "invalid - no @",
			email: "userexample.com",
			want:  false,
		},
		{
			name:  "invalid - empty",
			email: "",
			want:  false,
		},
		{
			name:  "invalid - just @",
			email: "@",
			want:  false,
		},
		{
			name:  "invalid - multiple @",
			email: "@@@",
			want:  false,
		},
		{
			name:  "invalid - no local part",
			email: "@example.com",
			want:  false,
		},
		{
			name:  "invalid - no domain",
			email: "user@",
			want:  false,
		},
		{
			name:  "invalid - missing domain extension",
			email: "user@domain",
			want:  false,
		},
		{
			name:  "invalid - spaces in email",
			email: "user name@example.com",
			want:  false,
		},
		{
			name:  "invalid - double dots",
			email: "user..name@example.com",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidEmail(tt.email)
			if got != tt.want {
				t.Errorf("IsValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsAllowedEmailDomain(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		allowedDomains []string
		want           bool
	}{
		{
			name:           "empty list allows all",
			email:          "user@anything.com",
			allowedDomains: nil,
			want:           true,
		},
		{
			name:           "empty slice allows all",
			email:          "user@anything.com",
			allowedDomains: []string{},
			want:           true,
		},
		{
			name:           "single domain match",
			email:          "user@company.com",
			allowedDomains: []string{"company.com"},
			want:           true,
		},
		{
			name:           "single domain no match",
			email:          "user@other.com",
			allowedDomains: []string{"company.com"},
			want:           false,
		},
		{
			name:           "multiple domains - first matches",
			email:          "user@company.com",
			allowedDomains: []string{"company.com", "corp.net"},
			want:           true,
		},
		{
			name:           "multiple domains - second matches",
			email:          "user@corp.net",
			allowedDomains: []string{"company.com", "corp.net"},
			want:           true,
		},
		{
			name:           "multiple domains - none match",
			email:          "user@other.com",
			allowedDomains: []string{"company.com", "corp.net"},
			want:           false,
		},
		{
			name:           "case insensitive - uppercase email domain",
			email:          "user@COMPANY.COM",
			allowedDomains: []string{"company.com"},
			want:           true,
		},
		{
			name:           "case insensitive - mixed case email domain",
			email:          "user@Company.Com",
			allowedDomains: []string{"company.com"},
			want:           true,
		},
		{
			name:           "subdomain does NOT match parent",
			email:          "user@eng.company.com",
			allowedDomains: []string{"company.com"},
			want:           false,
		},
		{
			name:           "parent does NOT match subdomain",
			email:          "user@company.com",
			allowedDomains: []string{"eng.company.com"},
			want:           false,
		},
		{
			name:           "invalid email - no @",
			email:          "usercompany.com",
			allowedDomains: []string{"company.com"},
			want:           false,
		},
		{
			name:           "empty email",
			email:          "",
			allowedDomains: []string{"company.com"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAllowedEmailDomain(tt.email, tt.allowedDomains)
			if got != tt.want {
				t.Errorf("IsAllowedEmailDomain(%q, %v) = %v, want %v", tt.email, tt.allowedDomains, got, tt.want)
			}
		})
	}
}

func TestValidateDomainList(t *testing.T) {
	tests := []struct {
		name    string
		domains []string
		wantErr bool
	}{
		{
			name:    "valid single domain",
			domains: []string{"company.com"},
			wantErr: false,
		},
		{
			name:    "valid multiple domains",
			domains: []string{"company.com", "corp.net", "org.example.com"},
			wantErr: false,
		},
		{
			name:    "empty list is valid",
			domains: []string{},
			wantErr: false,
		},
		{
			name:    "invalid - no TLD",
			domains: []string{"localhost"},
			wantErr: true,
		},
		{
			name:    "invalid - spaces in domain",
			domains: []string{"company .com"},
			wantErr: true,
		},
		{
			name:    "invalid - empty string entry",
			domains: []string{""},
			wantErr: true,
		},
		{
			name:    "invalid - leading dot",
			domains: []string{".company.com"},
			wantErr: true,
		},
		{
			name:    "invalid - one bad entry among valid ones",
			domains: []string{"company.com", "invalid", "corp.net"},
			wantErr: true,
		},
		{
			name:    "valid - subdomain format",
			domains: []string{"sub.company.com"},
			wantErr: false,
		},
		{
			name:    "valid - hyphenated domain",
			domains: []string{"my-company.com"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomainList(tt.domains)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDomainList(%v) error = %v, wantErr %v", tt.domains, err, tt.wantErr)
			}
		})
	}
}
