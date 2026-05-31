package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// TestParseAllowedOrigins covers the CORS allowlist parser, including the
// CF-425 guard that drops a wildcard "*" entry. Startup validation in
// cmd/server/main.go refuses ALLOWED_ORIGINS=* outright; this parser is the
// in-process defense-in-depth: even if a "*" somehow leaks into the env var,
// it must never reach the chi CORS middleware (which would otherwise pair
// AllowCredentials=true with Access-Control-Allow-Origin: *).
func TestParseAllowedOrigins(t *testing.T) {
	cases := []struct {
		name              string
		env               string
		wantCORS, wantCSRF []string
	}{
		{
			name:     "single origin",
			env:      "https://confab.example.com",
			wantCORS: []string{"https://confab.example.com"},
			wantCSRF: []string{"confab.example.com"},
		},
		{
			name:     "multiple with whitespace",
			env:      "https://a.example.com, https://b.example.com",
			wantCORS: []string{"https://a.example.com", "https://b.example.com"},
			wantCSRF: []string{"a.example.com", "b.example.com"},
		},
		{
			name:     "wildcard is dropped",
			env:      "*",
			wantCORS: nil,
			wantCSRF: nil,
		},
		{
			name:     "wildcard mixed with explicit origin is dropped",
			env:      "https://confab.example.com,*",
			wantCORS: []string{"https://confab.example.com"},
			wantCSRF: []string{"confab.example.com"},
		},
		{
			name:     "empty entries skipped",
			env:      "https://confab.example.com,,",
			wantCORS: []string{"https://confab.example.com"},
			wantCSRF: []string{"confab.example.com"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ALLOWED_ORIGINS", tc.env)
			gotCORS, gotCSRF := parseAllowedOrigins()
			if !reflect.DeepEqual(gotCORS, tc.wantCORS) {
				t.Errorf("CORS origins: got %v, want %v", gotCORS, tc.wantCORS)
			}
			if !reflect.DeepEqual(gotCSRF, tc.wantCSRF) {
				t.Errorf("CSRF trusted origins: got %v, want %v", gotCSRF, tc.wantCSRF)
			}
		})
	}
}

// TestDeleteAccountHelpPage_ProviderNeutral pins the CF-353 invariant that
// the public delete-account help page describes uploaded content in
// provider-neutral language. Recipients of this page may have uploaded
// Claude Code sessions, Codex sessions, or both; the bullet list must not
// claim it only deletes "Claude Code session transcripts".
func TestDeleteAccountHelpPage_ProviderNeutral(t *testing.T) {
	server := NewServer(&db.DB{}, &storage.S3Storage{}, &auth.OAuthConfig{}, nil, BuildInfo{})

	req := httptest.NewRequest(http.MethodGet, "/help/delete-account", nil)
	rr := httptest.NewRecorder()
	server.handleDeleteAccountHelp(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()

	if strings.Contains(body, "Claude Code session transcripts") {
		t.Errorf("privacy page leaks 'Claude Code session transcripts' — should be provider-neutral")
	}
	if !strings.Contains(body, "session transcripts you've uploaded") {
		t.Errorf("privacy page missing the neutral phrase 'session transcripts you've uploaded'")
	}
}
