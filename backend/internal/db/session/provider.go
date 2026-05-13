package session

// Canonical and legacy provider values for sessions.session_type. The
// canonical lowercase forms ("claude-code", "codex") are what new code
// writes; the legacy display form is what older binaries wrote before
// CF-347 and may still appear during the deploy gap.
const (
	providerClaudeCode       = "claude-code"
	providerClaudeCodeLegacy = "Claude Code"
)

// normalizeProvider maps the legacy display value 'Claude Code' to the
// canonical lowercase form 'claude-code'. New code stores the canonical
// values ('claude-code', 'codex') directly, but rows created by older
// binaries — or by new code during the deploy gap when an older binary
// is still serving — may still hold 'Claude Code'. Apply this at every
// Scan site that reads sessions.session_type so the application layer
// and API surface always see canonical values.
//
// TODO(post-Codex-rollout): once the deploy gap is no longer a concern,
// run a one-time backfill (`UPDATE sessions SET session_type='claude-code'
// WHERE session_type='Claude Code'`) and then delete this helper and the
// dual-value IN (...) clauses in sync.go and analytics/precompute.go.
func normalizeProvider(p string) string {
	if p == providerClaudeCodeLegacy {
		return providerClaudeCode
	}
	return p
}
