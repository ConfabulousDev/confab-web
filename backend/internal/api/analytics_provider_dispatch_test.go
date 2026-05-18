package api

import (
	"os"
	"strings"
	"testing"
)

// TestAnalyticsGoHasNoProviderLiterals enforces CF-403's done-state: dispatch
// in api/analytics.go goes through analytics.ProviderFor exclusively. After
// the refactor, the file must not reference codex-specific helpers,
// provider literals, or the isCodexSession guard.
//
// Mirrors TestPrecomputeGoHasNoProviderSwitchOrLiterals (CF-402) in spirit:
// a source-scan test that survives implementation churn and fails loudly if
// future changes accidentally reintroduce provider-aware branching at this
// boundary.
func TestAnalyticsGoHasNoProviderLiterals(t *testing.T) {
	source, err := os.ReadFile("analytics.go")
	if err != nil {
		t.Fatalf("read analytics.go: %v", err)
	}
	text := string(source)
	forbidden := []string{
		"isCodexSession",
		"handleCodexCacheMiss",
		"downloadCodexTranscriptForRecap",
		"downloadAndBuildTranscript",
		"models.ProviderCodex",
		"models.ProviderClaudeCode",
		"models.ProviderClaudeCodeLegacy",
		"analytics.LoadCodexRollout",
		"analytics.ComputeFromCodexRollout",
		"analytics.PrepareCodexTranscript",
		"analytics.ExtractCodexUserMessagesText",
	}
	for _, s := range forbidden {
		if strings.Contains(text, s) {
			t.Errorf("api/analytics.go should dispatch through analytics.ProviderFor, but still contains %q", s)
		}
	}
}
