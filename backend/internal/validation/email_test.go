package validation

import "testing"

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
