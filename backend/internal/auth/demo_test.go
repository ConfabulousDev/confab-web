package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// CF-483: Demo identity spec tests (unit level).
// =============================================================================

// DemoSessionCookieID is deterministic and depends on BOTH inputs.
// Spec: HMAC-SHA256(csrfSecret, "demo:"+email) base64url-encoded.
func TestDemoSessionCookieID_Deterministic(t *testing.T) {
	a := DemoSessionCookieID("secret-key-32-bytes-of-entropy!!", "demo@confabulous.dev")
	b := DemoSessionCookieID("secret-key-32-bytes-of-entropy!!", "demo@confabulous.dev")
	if a == "" {
		t.Fatal("DemoSessionCookieID returned empty for non-empty email — Phase 4 must implement HMAC")
	}
	if a != b {
		t.Errorf("DemoSessionCookieID not deterministic across calls: %q vs %q", a, b)
	}
}

func TestDemoSessionCookieID_DependsOnSecret(t *testing.T) {
	a := DemoSessionCookieID("secret-A-32-bytes-of-entropy-aaa", "demo@confabulous.dev")
	b := DemoSessionCookieID("secret-B-32-bytes-of-entropy-bbb", "demo@confabulous.dev")
	if a == b {
		t.Errorf("DemoSessionCookieID must vary with secret; got identical %q", a)
	}
}

func TestDemoSessionCookieID_DependsOnEmail(t *testing.T) {
	a := DemoSessionCookieID("secret-key-32-bytes-of-entropy!!", "demo@confabulous.dev")
	b := DemoSessionCookieID("secret-key-32-bytes-of-entropy!!", "sandbox@confabulous.dev")
	if a == b {
		t.Errorf("DemoSessionCookieID must vary with email; got identical %q", a)
	}
}

// Empty email returns "" sentinel so HandleLogout can short-circuit
// without comparing to a real (deterministic but valid) cookie value.
func TestDemoSessionCookieID_EmptyEmailReturnsEmpty(t *testing.T) {
	if got := DemoSessionCookieID("secret-key-32-bytes-of-entropy!!", ""); got != "" {
		t.Errorf("DemoSessionCookieID(secret, \"\") = %q, want \"\" sentinel", got)
	}
}

// =============================================================================
// EnforceReadOnly method matrix (B1/D1 spec).
// =============================================================================

// Mutating methods on a read-only user must return the documented
// structured 403 body — exact shape, exact strings.
func TestEnforceReadOnly_BlocksMutatingMethods(t *testing.T) {
	mw := EnforceReadOnly(nil) // db arg unused once read_only is in ctx
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handlerCalled = true })
	wrapped := mw(next)

	mutating := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, m := range mutating {
		t.Run(m, func(t *testing.T) {
			handlerCalled = false
			req := httptest.NewRequest(m, "/api/v1/sessions/abc/share", nil)
			req = req.WithContext(WithReadOnly(req.Context(), true))
			rr := httptest.NewRecorder()
			wrapped.ServeHTTP(rr, req)

			if handlerCalled {
				t.Errorf("downstream handler called for %s, want 403 short-circuit", m)
			}
			if rr.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", rr.Code)
			}
			if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			var body ReadOnlyUserError
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v (raw=%s)", err, rr.Body.String())
			}
			if body.Error != "read_only_user" {
				t.Errorf("body.error = %q, want %q", body.Error, "read_only_user")
			}
			if body.Message == "" {
				t.Errorf("body.message empty; spec requires human-readable text")
			}
		})
	}
}

// Reads must pass through unchanged.
func TestEnforceReadOnly_AllowsReadMethods(t *testing.T) {
	mw := EnforceReadOnly(nil)
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(next)

	readMethods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, m := range readMethods {
		t.Run(m, func(t *testing.T) {
			handlerCalled = false
			req := httptest.NewRequest(m, "/api/v1/sessions", nil)
			req = req.WithContext(WithReadOnly(req.Context(), true))
			rr := httptest.NewRecorder()
			wrapped.ServeHTTP(rr, req)

			if !handlerCalled {
				t.Errorf("downstream handler not called for %s; reads must pass through", m)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 (pass-through)", rr.Code)
			}
		})
	}
}

// Regression: when no read-only flag is set in context (real user OR
// anonymous request that wasn't impersonated), middleware must NOT
// block any method. This is the D1 safe-by-default guarantee that lets
// us mount EnforceReadOnly at the /api/v1 root without breaking
// non-demo deployments.
func TestEnforceReadOnly_PassThroughWhenNoReadOnlyFlag(t *testing.T) {
	mw := EnforceReadOnly(nil)
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	})
	wrapped := mw(next)

	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(m, func(t *testing.T) {
			handlerCalled = false
			req := httptest.NewRequest(m, "/api/v1/whatever", nil)
			rr := httptest.NewRecorder()
			wrapped.ServeHTTP(rr, req)
			if !handlerCalled {
				t.Errorf("%s blocked despite no read-only flag in context", m)
			}
			if rr.Code != http.StatusCreated {
				t.Errorf("%s status = %d, want 201 (pass-through)", m, rr.Code)
			}
		})
	}
}

// Same regression but explicit ctx with read_only=false (real user).
func TestEnforceReadOnly_PassThroughForRealUsers(t *testing.T) {
	mw := EnforceReadOnly(nil)
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", nil)
	req = req.WithContext(WithReadOnly(req.Context(), false))
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("real user POST blocked by EnforceReadOnly; must pass through")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

// =============================================================================
// AutoImpersonateIfDemo guard
// =============================================================================

// Zero-behavior-change guarantee: when demoEmail is empty, AutoImpersonateIfDemo
// must return nil without touching the request, response, or database.
func TestAutoImpersonateIfDemo_NilWhenDemoEmailEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rr := httptest.NewRecorder()

	res := AutoImpersonateIfDemo(rr, req, nil, "", "secret")
	if res != nil {
		t.Errorf("AutoImpersonateIfDemo returned non-nil with empty demoEmail: %+v", res)
	}
	if len(rr.Header().Values("Set-Cookie")) > 0 {
		t.Errorf("Set-Cookie emitted despite empty demoEmail: %v", rr.Header().Values("Set-Cookie"))
	}
}

// =============================================================================
// Context helper sanity
// =============================================================================

func TestReadOnlyContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	if ReadOnlyFromContext(ctx) {
		t.Error("ReadOnlyFromContext default must be false")
	}
	ctx = WithReadOnly(ctx, true)
	if !ReadOnlyFromContext(ctx) {
		t.Error("ReadOnlyFromContext did not preserve true value")
	}
	ctx = WithReadOnly(ctx, false)
	if ReadOnlyFromContext(ctx) {
		t.Error("ReadOnlyFromContext did not overwrite to false")
	}
}
