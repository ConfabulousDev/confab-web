package analytics

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// newCaptureLogger returns a JSON slog logger writing to an in-memory buffer at
// the given level, plus the buffer for assertions.
func newCaptureLogger(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level})), &buf
}

// TestPricingForModel_UnknownNonEmptyModelWarnsWithContext is the AC3 contract:
// a genuinely-unknown NON-EMPTY model warns, and the warning carries the
// session_id + provider (from the enriched logger), the model, and its family.
func TestPricingForModel_UnknownNonEmptyModelWarnsWithContext(t *testing.T) {
	log, buf := newCaptureLogger(slog.LevelDebug)
	log = log.With("session_id", "sess-123", "provider", "codex")

	pricing := pricingForModel(log, "totally-made-up-model")
	if !pricing.Input.IsZero() {
		t.Errorf("pricingForModel(unknown).Input = %s, want 0", pricing.Input)
	}

	out := buf.String()
	for _, want := range []string{
		`"level":"WARN"`,
		`unknown model for pricing`,
		`"model":"totally-made-up-model"`,
		`"session_id":"sess-123"`,
		`"provider":"codex"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("warn output missing %q\ngot: %s", want, out)
		}
	}
}

// TestPricingForModel_EmptyModelDebugNoWarn is the AC1/AC2 contract: an empty
// model is an expected sentinel — it must NOT warn (only a DEBUG line) and must
// resolve to zero pricing.
func TestPricingForModel_EmptyModelDebugNoWarn(t *testing.T) {
	log, buf := newCaptureLogger(slog.LevelDebug)

	pricing := pricingForModel(log, "")
	if !pricing.Input.IsZero() {
		t.Errorf("pricingForModel(\"\").Input = %s, want 0", pricing.Input)
	}

	out := buf.String()
	if strings.Contains(out, "unknown model for pricing") {
		t.Errorf("empty model must not emit the unknown-model WARN\ngot: %s", out)
	}
	if strings.Contains(out, `"level":"WARN"`) {
		t.Errorf("empty model must not warn\ngot: %s", out)
	}
}

// TestPricingForModel_NilLoggerSafe guards the test/Analyze path where no logger
// was threaded onto the analyzer: pricingForModel must fall back to the default
// logger instead of panicking.
func TestPricingForModel_NilLoggerSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("pricingForModel(nil, ...) panicked: %v", r)
		}
	}()
	if p := pricingForModel(nil, "made-up"); !p.Input.IsZero() {
		t.Errorf("pricingForModel(nil, unknown).Input = %s, want 0", p.Input)
	}
}

// TestCodexComputeWarnCarriesSessionID is the end-to-end wiring proof: a codex
// rollout with a genuinely-unknown model, computed under a session-enriched ctx
// logger, must surface session_id in the resulting WARN.
func TestCodexComputeWarnCarriesSessionID(t *testing.T) {
	log, buf := newCaptureLogger(slog.LevelDebug)
	ctx := logger.WithLogger(context.Background(), log.With("session_id", "sess-xyz", "provider", "codex"))

	r := &codex.ParsedRollout{
		Model:      "totally-made-up-model",
		Turns:      []codex.Turn{{Model: "totally-made-up-model"}},
		TokenUsage: codex.TokenUsage{InputTokens: 100, OutputTokens: 50},
	}
	_ = ComputeFromCodexRollout(ctx, []*codex.ParsedRollout{r})

	out := buf.String()
	for _, want := range []string{"unknown model for pricing", "sess-xyz", "totally-made-up-model"} {
		if !strings.Contains(out, want) {
			t.Errorf("codex compute warn missing %q\ngot: %s", want, out)
		}
	}
}

// TestCodexComputeEmptyModelNoWarn is the AC4 codex audit: a rollout with no
// model name must not emit the unknown-model WARN (no model to price against is
// expected, not an anomaly).
func TestCodexComputeEmptyModelNoWarn(t *testing.T) {
	log, buf := newCaptureLogger(slog.LevelDebug)
	ctx := logger.WithLogger(context.Background(), log)

	r := &codex.ParsedRollout{
		Model:      "",
		Turns:      []codex.Turn{{}},
		TokenUsage: codex.TokenUsage{InputTokens: 100, OutputTokens: 50},
	}
	_ = ComputeFromCodexRollout(ctx, []*codex.ParsedRollout{r})

	if out := buf.String(); strings.Contains(out, "unknown model for pricing") {
		t.Errorf("empty codex model must not warn\ngot: %s", out)
	}
}
