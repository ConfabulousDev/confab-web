package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// TestDeleteAccountHelpPage_ProviderNeutral pins the CF-353 invariant that
// the public delete-account help page describes uploaded content in
// provider-neutral language. Recipients of this page may have uploaded
// Claude Code sessions, Codex sessions, or both; the bullet list must not
// claim it only deletes "Claude Code session transcripts".
func TestDeleteAccountHelpPage_ProviderNeutral(t *testing.T) {
	server := NewServer(&db.DB{}, &storage.S3Storage{}, &auth.OAuthConfig{}, nil, "")

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
