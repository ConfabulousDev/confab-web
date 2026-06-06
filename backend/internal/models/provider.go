package models

// Provider identity for Confab sessions.
//
// Confab is OSS self-hosted, so we cannot assume any operator's database
// has been migrated to a single canonical session_type form. Instead of a
// one-time backfill, this package owns the PERMANENT provider-aliasing
// layer:
//
//   - CanonicalProviders is the closed enum exposed at the wire and UI.
//   - LegacyAliases maps non-canonical session_type values to canonical
//     ones. To deprecate a provider name, add an entry here rather than
//     scattering dual-value handling across the codebase.
//   - AllowedProviders is the full set of DB session_type values accepted
//     by the analytics pipeline (canonical + every alias).
//   - NormalizeProvider, ExpandWithAliases are the helpers used at Scan
//     sites and in SQL filter parameters respectively.

// Canonical provider values stored in sessions.session_type for new rows.
const (
	ProviderClaudeCode = "claude-code"
	ProviderCodex      = "codex"
	ProviderOpencode   = "opencode"
)

// ProviderClaudeCodeLegacy is the pre-CF-347 display form that older
// binaries wrote to sessions.session_type. Permanent alias of
// ProviderClaudeCode — never backfilled away in OSS self-hosted installs.
const ProviderClaudeCodeLegacy = "Claude Code"

// CanonicalProviders is the closed enum exposed to API callers, filter
// UIs, and validation. Wire input is canonical-only; legacy values are a
// DB-side concern.
var CanonicalProviders = []string{
	ProviderClaudeCode,
	ProviderCodex,
	ProviderOpencode,
}

// LegacyAliases maps non-canonical session_type values to canonical form.
// Permanent provider-aliasing layer — add entries here to deprecate a
// provider name without scattering dual-value handling across the
// codebase.
var LegacyAliases = map[string]string{
	ProviderClaudeCodeLegacy: ProviderClaudeCode,
}

// AllowedProviders is the full set of session_type values the analytics
// pipeline accepts — canonical + every legacy alias. Hand-written in
// deterministic order; TestAllowedProvidersInSyncWithCanonicalAndLegacy
// asserts it stays in sync with CanonicalProviders + keys(LegacyAliases).
//
// Every SQL filter that wants "all provider rows" passes this slice as
// pq.Array(AllowedProviders) to `session_type = ANY($N)`. Forgetting to
// add a new value here is the silent-skip bug this package exists to
// prevent — TestDispatchCoversAllowedProviders in internal/analytics is
// the gate that catches it at test time.
var AllowedProviders = []string{
	ProviderClaudeCode,
	ProviderCodex,
	ProviderOpencode,
	ProviderClaudeCodeLegacy,
}

// NormalizeProvider maps a possibly-legacy session_type value to its
// canonical form. Unknown values pass through unchanged so future
// providers don't accidentally collapse to claude-code.
func NormalizeProvider(p string) string {
	if canonical, ok := LegacyAliases[p]; ok {
		return canonical
	}
	return p
}

// ExpandWithAliases expands a list of canonical provider values to
// include every registered legacy alias for each one. Used to build the
// parameter passed to `session_type = ANY($N)` so legacy rows match
// canonical-form requests.
//
// Example: ExpandWithAliases([]string{"claude-code"}) returns
// []string{"claude-code", "Claude Code"} (order of aliases not guaranteed).
// Returns a non-nil empty slice when canonical is empty.
func ExpandWithAliases(canonical []string) []string {
	out := make([]string, 0, len(canonical)+len(LegacyAliases))
	for _, v := range canonical {
		out = append(out, v)
		for legacy, target := range LegacyAliases {
			if target == v {
				out = append(out, legacy)
			}
		}
	}
	return out
}
