package db

// Canonical and legacy provider values for the sessions.session_type column.
//
// New code writes the canonical lowercase forms ("claude-code", "codex").
// Older binaries (pre-CF-347) wrote the display form "Claude Code"; those
// rows may still exist until the post-rollout backfill PR collapses them.
//
// validation.ProviderClaudeCode / validation.ProviderCodex are the
// matching constants on the input-validation side. Keep these two
// definitions in lockstep — they are intentionally duplicated to keep
// the db and validation layers free of cross-dependencies.
const (
	ProviderClaudeCode       = "claude-code"
	ProviderClaudeCodeLegacy = "Claude Code"
	ProviderCodex            = "codex"
)

// NormalizeProvider maps the legacy display value "Claude Code" to the
// canonical lowercase "claude-code". New code stores the canonical values
// directly, but rows created by older binaries — or by new code during the
// deploy gap when an older binary is still serving — may still hold the
// legacy form. Apply this at every Scan site that reads sessions.session_type
// so the application layer and API surface always see canonical values.
//
// TODO(post-Codex-rollout): once the deploy gap is no longer a concern,
// run a one-time backfill (UPDATE sessions SET session_type='claude-code'
// WHERE session_type='Claude Code') and then delete this helper and the
// dual-value IN-clauses in db/session/sync.go and analytics/precompute.go.
func NormalizeProvider(p string) string {
	if p == ProviderClaudeCodeLegacy {
		return ProviderClaudeCode
	}
	return p
}
