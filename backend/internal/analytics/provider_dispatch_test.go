package analytics

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// TestDispatchCoversAllowedProviders is the compile-time-style guard that
// catches "added a string to models.AllowedProviders (so the SQL filter
// picks it up) but forgot to add a case to the precompute dispatch
// switches." Without this test, a forgotten case would silently route to
// the `default: return fmt.Errorf("unsupported provider...")` arm — but
// the SQL filter would still emit those sessions, so they would loop
// forever as "stale, can't process."
//
// Keep providerSupported() in lockstep with the switch arms in
// PrecomputeRegularCards, BuildSearchIndexOnly, and PrecomputeSmartRecapOnly.
// Phase 2 (CF-402) deletes this test along with providerSupported when the
// registry replaces both.
func TestDispatchCoversAllowedProviders(t *testing.T) {
	if len(models.AllowedProviders) == 0 {
		t.Fatal("models.AllowedProviders is empty; spec requires at least canonical providers + legacy aliases")
	}
	for _, p := range models.AllowedProviders {
		if !providerSupported(p) {
			t.Errorf("models.AllowedProviders contains %q but precompute dispatch has no case for it — add the case in precompute.go or remove from models.AllowedProviders", p)
		}
	}
}
