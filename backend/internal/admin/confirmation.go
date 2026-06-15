package admin

import (
	"net/http"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/httputil"
)

// confirmation.go: the shared typed-confirmation echo used to gate destructive
// admin actions (kyrr / g0bq item #1). The admin must echo a target-bound value
// (the user's email, or the affected-session count) and the server verifies it
// before mutating — so a misclick on the wrong row, or a stale preview, can't fire
// the action. No token store, no migration: the expected value is derived from data
// the handler already has.

// verifyConfirmation reports whether the admin-provided confirmation matches the
// expected target value. The compare is trim + Unicode-case-insensitive so the
// admin can paste the displayed email without fighting whitespace/case. An empty
// expected value never matches (a missing target can't be "confirmed"), which also
// means a blank provided value is always rejected.
func verifyConfirmation(expected, provided string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	return strings.EqualFold(expected, strings.TrimSpace(provided))
}

// respondConfirmationMismatch writes the shared 400 used by every confirmation
// gate, so the wire contract (status + message) stays identical across endpoints.
func respondConfirmationMismatch(w http.ResponseWriter) {
	httputil.RespondError(w, http.StatusBadRequest, "Confirmation does not match")
}
