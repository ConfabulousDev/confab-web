package api

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
)


// =============================================================================
// extractPRLinkFromLine Unit Tests
// =============================================================================

func TestExtractPRLinkFromLine(t *testing.T) {
	t.Run("extracts valid pr-link", func(t *testing.T) {
		line := `{"type":"pr-link","prNumber":44,"prUrl":"https://github.com/ConfabulousDev/confab-web/pull/44","prRepository":"ConfabulousDev/confab-web","sessionId":"abc","timestamp":"2025-01-01T00:00:00Z"}`
		link := extractPRLinkFromLine(line)
		if link == nil {
			t.Fatal("expected non-nil link")
		}
		if link.Owner != "ConfabulousDev" {
			t.Errorf("expected owner 'ConfabulousDev', got %s", link.Owner)
		}
		if link.Repo != "confab-web" {
			t.Errorf("expected repo 'confab-web', got %s", link.Repo)
		}
		if link.Ref != "44" {
			t.Errorf("expected ref '44', got %s", link.Ref)
		}
		if link.LinkType != models.GitHubLinkTypePullRequest {
			t.Errorf("expected link_type 'pull_request', got %s", link.LinkType)
		}
		if link.Source != models.GitHubLinkSourceTranscript {
			t.Errorf("expected source 'transcript', got %s", link.Source)
		}
		expectedTitle := "ConfabulousDev/confab-web#44"
		if link.Title == nil || *link.Title != expectedTitle {
			t.Errorf("expected title %q, got %v", expectedTitle, link.Title)
		}
		if link.URL != "https://github.com/ConfabulousDev/confab-web/pull/44" {
			t.Errorf("expected URL preserved, got %s", link.URL)
		}
	})

	t.Run("returns nil for non-pr-link type", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":"hello"}}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for non-pr-link type")
		}
	})

	t.Run("returns nil for malformed JSON", func(t *testing.T) {
		if link := extractPRLinkFromLine(`{not valid json`); link != nil {
			t.Error("expected nil for malformed JSON")
		}
	})

	t.Run("returns nil for missing prUrl", func(t *testing.T) {
		line := `{"type":"pr-link","prNumber":44,"prRepository":"owner/repo"}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for missing prUrl")
		}
	})

	t.Run("returns nil for missing prRepository", func(t *testing.T) {
		line := `{"type":"pr-link","prNumber":44,"prUrl":"https://github.com/owner/repo/pull/44"}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for missing prRepository")
		}
	})

	t.Run("returns nil for missing prNumber", func(t *testing.T) {
		line := `{"type":"pr-link","prUrl":"https://github.com/owner/repo/pull/44","prRepository":"owner/repo"}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for missing prNumber")
		}
	})

	t.Run("returns nil for invalid prUrl format", func(t *testing.T) {
		line := `{"type":"pr-link","prNumber":44,"prUrl":"https://gitlab.com/owner/repo/pull/44","prRepository":"owner/repo"}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for non-GitHub prUrl")
		}
	})

	t.Run("returns nil for inconsistent fields", func(t *testing.T) {
		// prUrl says owner-a/repo-b#5, but prRepository says owner-x/repo-y and prNumber says 10
		line := `{"type":"pr-link","prNumber":10,"prUrl":"https://github.com/owner-a/repo-b/pull/5","prRepository":"owner-x/repo-y"}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for inconsistent fields")
		}
	})

	t.Run("returns nil for invalid prRepository format", func(t *testing.T) {
		line := `{"type":"pr-link","prNumber":44,"prUrl":"https://github.com/owner/repo/pull/44","prRepository":"noslash"}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for invalid prRepository")
		}
	})

	t.Run("returns nil for zero prNumber", func(t *testing.T) {
		line := `{"type":"pr-link","prNumber":0,"prUrl":"https://github.com/owner/repo/pull/0","prRepository":"owner/repo"}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for zero prNumber")
		}
	})

	t.Run("returns nil for line without pr-link string", func(t *testing.T) {
		// This line doesn't contain "pr-link" at all — quick check should skip it
		line := `{"type":"human","message":{"content":"hello world"}}`
		if link := extractPRLinkFromLine(line); link != nil {
			t.Error("expected nil for unrelated line")
		}
	})
}
