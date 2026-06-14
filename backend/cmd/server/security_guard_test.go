package main

import (
	"strings"
	"testing"
)

// The startup guard must refuse to run with a public default secret whenever the
// config signals production intent (INSECURE_DEV_MODE != "true"), while leaving
// local eval (INSECURE_DEV_MODE=true) free to use the shipped defaults.
func TestInsecureDefaultReason(t *testing.T) {
	const defaultCSRF = "local-dev-csrf-secret-change-me-32chars"
	const envExampleCSRF = "your-csrf-secret-key-at-least-32-chars"
	const defaultAdminPw = "localdevpassword"
	const customCSRF = "a-unique-production-csrf-secret-key-32+"
	const customAdminPw = "a-strong-unique-password"

	const localURL = "http://localhost:8080"
	const httpsURL = "https://confab.example.com"

	tests := []struct {
		name            string
		insecureDevMode bool
		frontendURL     string
		csrf            string
		adminPassword   string
		wantBlocked     bool
	}{
		{"local eval allows default csrf", true, localURL, defaultCSRF, "", false},
		{"local eval allows default admin password", true, localURL, customCSRF, defaultAdminPw, false},
		{"prod (dev mode off) blocks compose default csrf", false, localURL, defaultCSRF, "", true},
		{"prod blocks env-example default csrf", false, localURL, envExampleCSRF, "", true},
		{"prod blocks default admin password", false, localURL, customCSRF, defaultAdminPw, true},
		{"prod allows unique secrets", false, httpsURL, customCSRF, customAdminPw, false},
		{"prod allows unique csrf with no bootstrap password", false, httpsURL, customCSRF, "", false},
		// https FRONTEND_URL signals production even if INSECURE_DEV_MODE is left on.
		{"https url blocks default csrf despite dev mode", true, httpsURL, defaultCSRF, "", true},
		{"https url blocks default admin password despite dev mode", true, httpsURL, customCSRF, defaultAdminPw, true},
		{"https url allows unique secrets", true, httpsURL, customCSRF, customAdminPw, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason := insecureDefaultReason(tc.insecureDevMode, tc.frontendURL, tc.csrf, tc.adminPassword)
			if tc.wantBlocked && reason == "" {
				t.Fatalf("expected startup to be blocked, got empty reason")
			}
			if !tc.wantBlocked && reason != "" {
				t.Fatalf("expected startup to be allowed, got reason %q", reason)
			}
		})
	}
}

// loadConfig must fatal when production-intent meets a known default CSRF key.
func TestLoadConfig_RefusesDefaultCSRFInProduction(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("CSRF_SECRET_KEY", "local-dev-csrf-secret-change-me-32chars")
	// INSECURE_DEV_MODE cleared by clearServerEnv => production intent.

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected loadConfig to fatal on default CSRF key in production mode")
	}
	if !strings.Contains(strings.ToLower(got.msg), "refusing to start") {
		t.Errorf("unexpected fatal message: %q", got.msg)
	}
}

// The same default CSRF key is allowed in local eval (INSECURE_DEV_MODE=true).
func TestLoadConfig_AllowsDefaultCSRFInDevMode(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("CSRF_SECRET_KEY", "local-dev-csrf-secret-change-me-32chars")
	t.Setenv("INSECURE_DEV_MODE", "true")

	if got := withFatalRecover(t, func() { loadConfig() }); got != nil {
		t.Fatalf("expected no fatal in dev mode, got %q", got.msg)
	}
}

// loadConfig must fatal when production-intent meets the default admin password.
func TestLoadConfig_RefusesDefaultAdminPasswordInProduction(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t) // sets a unique CSRF key
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "localdevpassword")

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected loadConfig to fatal on default admin password in production mode")
	}
}
