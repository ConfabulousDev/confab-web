package models

import (
	"reflect"
	"sort"
	"testing"
)

// TestProviderCanonicalConstants locks the wire values of the canonical
// provider constants. These strings appear in sessions.session_type and
// in API responses; changing them is a wire break and must be intentional.
func TestProviderCanonicalConstants(t *testing.T) {
	if ProviderClaudeCode != "claude-code" {
		t.Errorf("ProviderClaudeCode = %q, want %q", ProviderClaudeCode, "claude-code")
	}
	if ProviderCodex != "codex" {
		t.Errorf("ProviderCodex = %q, want %q", ProviderCodex, "codex")
	}
	if ProviderClaudeCodeLegacy != "Claude Code" {
		t.Errorf("ProviderClaudeCodeLegacy = %q, want %q", ProviderClaudeCodeLegacy, "Claude Code")
	}
	if ProviderOpencode != "opencode" {
		t.Errorf("ProviderOpencode = %q, want %q", ProviderOpencode, "opencode")
	}
}

// TestCanonicalProvidersExcludesLegacy asserts that CanonicalProviders
// contains only canonical forms. CanonicalProviders is the wire-input
// allowlist (validation.ValidateProvider, filter UI enum); legacy values
// must NOT be acceptable inputs from clients.
func TestCanonicalProvidersExcludesLegacy(t *testing.T) {
	want := []string{ProviderClaudeCode, ProviderCodex, ProviderOpencode}
	if !reflect.DeepEqual(CanonicalProviders, want) {
		t.Errorf("CanonicalProviders = %v, want %v (deterministic order required)", CanonicalProviders, want)
	}
	for legacy := range LegacyAliases {
		for _, c := range CanonicalProviders {
			if c == legacy {
				t.Errorf("CanonicalProviders contains legacy alias %q; legacy values must not be accepted on the wire", legacy)
			}
		}
	}
}

// TestAllowedProvidersInSyncWithCanonicalAndLegacy guards the invariant
// that the SQL allowlist exactly equals CanonicalProviders ∪ keys(LegacyAliases).
// This is the compile-time check (well, test-time) that catches the case
// where someone adds a constant but forgets to extend AllowedProviders.
func TestAllowedProvidersInSyncWithCanonicalAndLegacy(t *testing.T) {
	got := append([]string{}, AllowedProviders...)
	sort.Strings(got)

	want := append([]string{}, CanonicalProviders...)
	for legacy := range LegacyAliases {
		want = append(want, legacy)
	}
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllowedProviders out of sync with CanonicalProviders ∪ keys(LegacyAliases)\n  got  (sorted): %v\n  want (sorted): %v", got, want)
	}
}

// TestNormalizeProvider locks the legacy → canonical mapping plus the
// passthrough rule for unknown values. Passthrough (not erroring) means a
// future provider whose name we don't yet know flows through analytics
// without accidentally collapsing to claude-code.
func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"claude-code", "claude-code"},
		{"Claude Code", "claude-code"},
		{"codex", "codex"},
		{"unknown-provider", "unknown-provider"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeProvider(tt.in); got != tt.want {
				t.Errorf("NormalizeProvider(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestExpandWithAliases — moved from
// backend/internal/db/session/expand_provider_legacy_test.go (CF-393).
// Locks the canonical → (canonical + legacy) expansion rule used by
// `session_type = ANY(...)` queries. Result fed into ANY; order does not
// matter, so the test compares sorted slices.
func TestExpandWithAliases(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty input returns empty",
			input: nil,
			want:  []string{},
		},
		{
			name:  "claude-code expands to canonical plus legacy",
			input: []string{"claude-code"},
			want:  []string{"claude-code", "Claude Code"},
		},
		{
			name:  "codex has no legacy form, passes through",
			input: []string{"codex"},
			want:  []string{"codex"},
		},
		{
			name:  "both providers includes both, plus claude-code legacy",
			input: []string{"claude-code", "codex"},
			want:  []string{"claude-code", "Claude Code", "codex"},
		},
		{
			name:  "ordering of inputs does not change the canonical/legacy expansion",
			input: []string{"codex", "claude-code"},
			want:  []string{"codex", "claude-code", "Claude Code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandWithAliases(tt.input)
			gotSorted := append([]string{}, got...)
			wantSorted := append([]string{}, tt.want...)
			sort.Strings(gotSorted)
			sort.Strings(wantSorted)
			if !reflect.DeepEqual(gotSorted, wantSorted) {
				t.Errorf("ExpandWithAliases(%v) = %v, want %v (order-insensitive)",
					tt.input, got, tt.want)
			}
		})
	}
}
