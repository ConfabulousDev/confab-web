package analytics

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// TestSessionProvider_DisplayName_Claude pins the email-facing label for the
// Claude Code provider. The label is concatenated with " session" in
// email/email.go::humanProviderLabel; tests under email_test.go assert the
// composed phrase ("Claude Code session"), so DisplayName must return exactly
// "Claude Code".
func TestSessionProvider_DisplayName_Claude(t *testing.T) {
	sp, err := ProviderFor(models.ProviderClaudeCode)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderClaudeCode, err)
	}
	if got := sp.DisplayName(); got != "Claude Code" {
		t.Errorf("DisplayName() = %q, want %q", got, "Claude Code")
	}
}

// TestSessionProvider_DisplayName_ClaudeLegacyAlias asserts the legacy
// "Claude Code" alias resolves to the same provider and produces the same
// display string. This guards the alias-by-DisplayName contract for the
// permanent legacy session_type values described in CF-400.
func TestSessionProvider_DisplayName_ClaudeLegacyAlias(t *testing.T) {
	sp, err := ProviderFor(models.ProviderClaudeCodeLegacy)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderClaudeCodeLegacy, err)
	}
	if got := sp.DisplayName(); got != "Claude Code" {
		t.Errorf("legacy alias DisplayName() = %q, want %q", got, "Claude Code")
	}
}

// TestSessionProvider_DisplayName_Codex pins the email-facing label for the
// Codex provider. email_test.go asserts the composed phrase "Codex session".
func TestSessionProvider_DisplayName_Codex(t *testing.T) {
	sp, err := ProviderFor(models.ProviderCodex)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderCodex, err)
	}
	if got := sp.DisplayName(); got != "Codex" {
		t.Errorf("DisplayName() = %q, want %q", got, "Codex")
	}
}
