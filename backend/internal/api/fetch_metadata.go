package api

import (
	"net/http"
	"net/url"

	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// crossOriginGuard rejects cross-origin browser requests to a state-changing
// auth route, using the Fetch Metadata (Sec-Fetch-Site) approach with an Origin
// fallback, and reusing the server's trustedOrigins allowlist (56mw).
//
// It exists because /auth/cli/authorize (a GET that mints an API key) and
// /auth/device/verify (a session-cookie form POST) are registered OUTSIDE the
// router's CSRF group. The project's CSRF library (filippo.io/csrf) — like the
// Fetch-Metadata standard — treats GET/HEAD/OPTIONS as always-safe and skips
// them, so it would not protect the state-changing GET even if those routes
// were moved under it. This guard applies the Sec-Fetch-Site/Origin check to
// ALL methods, closing that gap without converting cli/authorize to POST.
func crossOriginGuard(trustedOrigins []string, next http.HandlerFunc) http.HandlerFunc {
	trusted := make(map[string]struct{}, len(trustedOrigins))
	for _, o := range trustedOrigins {
		trusted[o] = struct{}{}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !crossOriginAllowed(r, trusted) {
			logger.Ctx(r.Context()).Warn("cross-origin request rejected on state-changing auth route",
				"method", r.Method,
				"path", r.URL.Path,
				"sec_fetch_site", r.Header.Get("Sec-Fetch-Site"),
				"origin", r.Header.Get("Origin"),
			)
			respondError(w, http.StatusForbidden, "Cross-origin request rejected")
			return
		}
		next(w, r)
	}
}

// crossOriginAllowed reports whether r is a same-origin request or a top-level
// navigation (or, when Sec-Fetch-Site is absent, a same-origin / trusted-origin
// request). Unlike a standard CSRF check it does NOT exempt safe methods, so a
// state-changing GET is still covered. It fails closed when neither
// Sec-Fetch-Site nor Origin is present — these routes are only ever reached by
// browsers, which send at least one since 2023.
func crossOriginAllowed(r *http.Request, trusted map[string]struct{}) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		// Same-origin request, or a direct top-level navigation (CLI-opened
		// browser, bookmark, typed URL).
		return true
	case "":
		// No Sec-Fetch-Site (pre-2023 browser / non-browser). Fall back to the
		// Origin header against same-origin + the trusted allowlist.
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false // fail closed
		}
		if o, err := url.Parse(origin); err == nil && o.Host == r.Host {
			return true // same-origin
		}
		_, ok := trusted[origin]
		return ok
	default:
		// cross-site / same-site → reject.
		return false
	}
}
