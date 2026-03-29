package api

import (
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/validation"
)

func TestValidateFilterValues(t *testing.T) {
	tests := []struct {
		name    string
		param   string
		values  []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty values OK",
			param:   "repo",
			values:  nil,
			wantErr: false,
		},
		{
			name:    "normal values OK",
			param:   "repo",
			values:  []string{"org/repo1", "org/repo2"},
			wantErr: false,
		},
		{
			name:    "too many values",
			param:   "repo",
			values:  make([]string, validation.MaxFilterCount+1),
			wantErr: true,
			errMsg:  "repo filter exceeds maximum",
		},
		{
			name:    "exactly max values OK",
			param:   "repo",
			values:  make([]string, validation.MaxFilterCount),
			wantErr: false,
		},
		{
			name:    "value too long",
			param:   "branch",
			values:  []string{strings.Repeat("a", validation.FilterMaxLen+1)},
			wantErr: true,
			errMsg:  "branch filter value exceeds maximum length",
		},
		{
			name:    "value at limit OK",
			param:   "branch",
			values:  []string{strings.Repeat("a", validation.FilterMaxLen)},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateFilterValues(tt.param, tt.values)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %q", err.Error())
				}
			}
		})
	}
}

func TestValidateSearchQuery(t *testing.T) {
	// At limit: OK
	err := validation.ValidateSearchQuery(strings.Repeat("x", validation.MaxSearchQueryLen))
	if err != nil {
		t.Errorf("expected no error at limit, got %q", err.Error())
	}

	// Over limit: error
	err = validation.ValidateSearchQuery(strings.Repeat("x", validation.MaxSearchQueryLen+1))
	if err == nil {
		t.Error("expected error for oversized query, got nil")
	}
}
