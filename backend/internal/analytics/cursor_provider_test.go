package analytics

import (
	"context"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// TestCursorProviderRegistered confirms the cursor analytics provider is
// registered under the canonical models.ProviderCursor name. Without this,
// TestRegistryCoversAllowedProviders red-bars once cursor joins
// models.AllowedProviders.
func TestCursorProviderRegistered(t *testing.T) {
	p, err := ProviderFor(models.ProviderCursor)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderCursor, err)
	}
	if p == nil {
		t.Fatal("cursor provider resolved to nil")
	}
}

// TestCursorProviderClearsMessageIDs locks decision #6: Cursor JSONL has no
// stable message/tool ids, so smart recap must drop ids (Codex precedent).
func TestCursorProviderClearsMessageIDs(t *testing.T) {
	p, err := ProviderFor(models.ProviderCursor)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderCursor, err)
	}
	if !p.ClearMessageIDs() {
		t.Fatal("cursor provider must clear smart-recap message ids (no stable anchors)")
	}
}

// TestCursorProviderDisplayName locks the human-facing label.
func TestCursorProviderDisplayName(t *testing.T) {
	p, err := ProviderFor(models.ProviderCursor)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderCursor, err)
	}
	if p.DisplayName() != "Cursor" {
		t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), "Cursor")
	}
}

// TestExtractCursorSearchText verifies the search index folds in user and
// assistant text (Cursor records no tool outputs, so there is nothing else to
// index) and includes at least one distinctive phrase from the fixture.
func TestExtractCursorSearchText(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	text := extractCursorSearchText(messages)
	if text == "" {
		t.Fatal("search text is empty")
	}
	for _, want := range []string{
		"Add input validation to the session handler",
		"reading the handler",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("search text missing expected phrase %q", want)
		}
	}
}

// TestPrepareCursorTranscript verifies the smart-recap XML envelope wraps the
// conversation and emits user/assistant/tool elements derived from the
// fixture, with no idMap anchors (Cursor has no stable ids).
func TestPrepareCursorTranscript(t *testing.T) {
	messages := loadCursorFixtureMessages(t)
	xml, idMap := PrepareCursorTranscript(messages)

	if !strings.HasPrefix(xml, "<transcript>") || !strings.HasSuffix(xml, "</transcript>") {
		t.Errorf("transcript not wrapped in <transcript> envelope:\n%s", xml)
	}
	if !strings.Contains(xml, "<user") {
		t.Error("transcript missing <user> element")
	}
	if !strings.Contains(xml, "<assistant") {
		t.Error("transcript missing <assistant> element")
	}
	if !strings.Contains(xml, "StrReplace") {
		t.Error("transcript should reference the StrReplace tool call")
	}
	if len(idMap) != 0 {
		t.Errorf("idMap should be empty for Cursor (no stable anchors), got %d entries", len(idMap))
	}
}

// TestCursorProviderComputeCardsHandlesNilRollout guards the defensive path
// where Parse returned no rollout.
func TestCursorProviderComputeCardsHandlesNilRollout(t *testing.T) {
	p, err := ProviderFor(models.ProviderCursor)
	if err != nil {
		t.Fatalf("ProviderFor(%q): %v", models.ProviderCursor, err)
	}
	result := p.ComputeCards(context.Background(), nil)
	if result == nil {
		t.Fatal("ComputeCards(nil) must return a non-nil result")
	}
}
