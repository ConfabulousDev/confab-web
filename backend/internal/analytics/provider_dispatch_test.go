package analytics

import (
	"os"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// TestRegistryCoversAllowedProviders catches "added a string to
// models.AllowedProviders (so the SQL filter picks it up) but forgot to
// register an analytics provider for it." Without this test, those sessions
// would loop forever as stale-but-unprocessable.
func TestRegistryCoversAllowedProviders(t *testing.T) {
	if len(models.AllowedProviders) == 0 {
		t.Fatal("models.AllowedProviders is empty; spec requires at least canonical providers + legacy aliases")
	}
	for _, p := range models.AllowedProviders {
		if _, err := ProviderFor(p); err != nil {
			t.Errorf("models.AllowedProviders contains %q but analytics registry has no provider: %v", p, err)
		}
	}
}

func TestProviderRegistryHandlesClaudeLegacyAlias(t *testing.T) {
	canonical, err := ProviderFor(models.ProviderClaudeCode)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderClaudeCode, err)
	}
	legacy, err := ProviderFor(models.ProviderClaudeCodeLegacy)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderClaudeCodeLegacy, err)
	}
	if canonical != legacy {
		t.Fatal("legacy Claude Code alias should resolve to the same provider instance as claude-code")
	}
	if canonical.ClearMessageIDs() {
		t.Fatal("Claude provider should keep message ids for smart recap deep-linking")
	}
}

func TestProviderRegistryCodexClearsMessageIDs(t *testing.T) {
	provider, err := ProviderFor(models.ProviderCodex)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderCodex, err)
	}
	if !provider.ClearMessageIDs() {
		t.Fatal("Codex provider must clear smart recap message ids because Codex has no stable frontend anchors")
	}
}

func TestProviderForUnknownProviderReturnsLoudError(t *testing.T) {
	_, err := ProviderFor("future-provider")
	if err == nil {
		t.Fatal("expected unknown provider error, got nil")
	}
	if !strings.Contains(err.Error(), "future-provider") {
		t.Fatalf("expected error to name unknown provider, got %q", err.Error())
	}
}

func TestPrecomputeGoHasNoProviderSwitchOrLiterals(t *testing.T) {
	source, err := os.ReadFile("precompute.go")
	if err != nil {
		t.Fatalf("read precompute.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"switch session.Provider",
		"models.ProviderClaudeCode",
		"models.ProviderClaudeCodeLegacy",
		"models.ProviderCodex",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("precompute.go should dispatch through ProviderFor, but still contains %q", forbidden)
		}
	}
}
