package analytics

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestNormalizeV2ModelKey pins the provider-aware rule (2hh1): OpenCode raw
// vendor keys collapse to families; Claude/Codex keys (already families, with
// Claude's baked-in " · fast" suffix) pass through untouched; the empty key
// stays empty (rendered Unknown).
func TestNormalizeV2ModelKey(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		rawKey   string
		want     string
	}{
		// OpenCode: raw model ids keyed by vendor → normalized to family.
		{"opencode anthropic raw → family", models.ProviderOpencode, "claude-opus-4-5-20251101", "opus-4-5"},
		{"opencode openai raw → family", models.ProviderOpencode, "gpt-5-2026-05-01", "gpt-5"},
		{"opencode already-family idempotent", models.ProviderOpencode, "opus-4-5", "opus-4-5"},
		// Claude: keys are already families and MUST NOT be re-normalized; the
		// " · fast" suffix must survive intact (getModelFamily would mangle it).
		{"claude family pass-through", models.ProviderClaudeCode, "opus-4-5", "opus-4-5"},
		{"claude fast suffix preserved", models.ProviderClaudeCode, "opus-4-5 · fast", "opus-4-5 · fast"},
		// Codex: already a family, pass-through.
		{"codex family pass-through", models.ProviderCodex, "gpt-5", "gpt-5"},
		// Empty key → Unknown sentinel, for every provider.
		{"empty key claude", models.ProviderClaudeCode, "", ""},
		{"empty key opencode", models.ProviderOpencode, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeV2ModelKey(tt.provider, tt.rawKey)
			if got != tt.want {
				t.Errorf("normalizeV2ModelKey(%q, %q) = %q, want %q", tt.provider, tt.rawKey, got, tt.want)
			}
		})
	}
}

// TestIsTimeoutErr pins which errors degrade the cost-by-model card (graceful)
// versus surface as a hard failure. Context deadlines (possibly wrapped) and
// Postgres query_canceled (57014) degrade; everything else does not.
func TestIsTimeoutErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"context deadline exceeded", context.DeadlineExceeded, true},
		{"wrapped context deadline", fmt.Errorf("cost-by-model query: %w", context.DeadlineExceeded), true},
		{"postgres query_canceled 57014", &pgconn.PgError{Code: "57014"}, true},
		{"wrapped postgres 57014", fmt.Errorf("scan: %w", &pgconn.PgError{Code: "57014"}), true},
		{"nil", nil, false},
		{"unrelated error", errors.New("connection refused"), false},
		{"other pg error", &pgconn.PgError{Code: "23505"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTimeoutErr(tt.err); got != tt.want {
				t.Errorf("isTimeoutErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
