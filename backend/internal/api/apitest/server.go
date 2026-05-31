// Package apitest provides a shared HTTP test server helper used by every
// internal/api/<feature>/ sub-package's integration tests. Centralizing the
// setup keeps per-feature test packages thin and avoids drift between the
// dozen near-identical setupXxxTestServer helpers that previously lived in
// the api package.
package apitest

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// DefaultCSRFSecret is the 32-byte CSRF key used by every integration test
// helper. Kept exported so tests that need to mint demo cookies (or otherwise
// compute HMACs over this key) can reuse the same value.
const DefaultCSRFSecret = "test-csrf-secret-key-32-bytes!!"

// Options tunes a test server beyond the common defaults. Zero value is the
// "sync-style" baseline: GitHub+Google OAuth client IDs filled in, demo mode
// off, share creation off, no email-domain restrictions.
type Options struct {
	// DemoEmail enables CF-483 demo mode. When set, DEMO_IDENTITY_EMAIL is
	// exported and OAuthConfig.DemoIdentityEmail / CSRFSecretKey are filled in.
	DemoEmail string

	// BootstrapDemo runs auth.BootstrapDemoIdentity before the server starts.
	// Only meaningful when DemoEmail is non-empty.
	BootstrapDemo bool

	// PasswordEnabled toggles OAuthConfig.PasswordEnabled.
	PasswordEnabled bool

	// EnableShareCreation sets ENABLE_SHARE_CREATION=true.
	EnableShareCreation bool

	// AllowedEmailDomains restricts the device-code email-domain whitelist.
	AllowedEmailDomains []string

	// SkipOAuthClientIDs leaves OAuth client ID/secret fields empty. The demo
	// integration tests use this because they exercise the password+demo path
	// only and want to assert behavior under a minimal OAuthConfig.
	SkipOAuthClientIDs bool
}

// NewServer brings up a real HTTP test server backed by the production
// router, returning the testutil.TestServer wrapper. The caller-provided
// TestEnvironment supplies the DB and storage handles.
func NewServer(t *testing.T, env *testutil.TestEnvironment, opts Options) *testutil.TestServer {
	t.Helper()

	testutil.SetEnvForTest(t, "CSRF_SECRET_KEY", DefaultCSRFSecret)
	testutil.SetEnvForTest(t, "ALLOWED_ORIGINS", "http://localhost:3000")
	testutil.SetEnvForTest(t, "FRONTEND_URL", "http://localhost:3000")
	testutil.SetEnvForTest(t, "INSECURE_DEV_MODE", "true")
	testutil.SetEnvForTest(t, "DEMO_IDENTITY_EMAIL", opts.DemoEmail)
	if opts.EnableShareCreation {
		testutil.SetEnvForTest(t, "ENABLE_SHARE_CREATION", "true")
	}

	cfg := auth.OAuthConfig{
		PasswordEnabled:     opts.PasswordEnabled,
		AllowedEmailDomains: opts.AllowedEmailDomains,
	}
	if !opts.SkipOAuthClientIDs {
		cfg.GitHubClientID = "test-github-client-id"
		cfg.GitHubClientSecret = "test-github-client-secret"
		cfg.GitHubRedirectURL = "http://localhost:3000/auth/github/callback"
		cfg.GoogleClientID = "test-google-client-id"
		cfg.GoogleClientSecret = "test-google-client-secret"
		cfg.GoogleRedirectURL = "http://localhost:3000/auth/google/callback"
	}
	if opts.DemoEmail != "" {
		cfg.DemoIdentityEmail = opts.DemoEmail
		cfg.CSRFSecretKey = DefaultCSRFSecret
	}
	if opts.BootstrapDemo {
		if err := auth.BootstrapDemoIdentity(context.Background(), env.DB, opts.DemoEmail, DefaultCSRFSecret); err != nil {
			t.Fatalf("BootstrapDemoIdentity: %v", err)
		}
	}

	srv := api.NewServer(env.DB, env.Storage, &cfg, nil, api.BuildInfo{})
	return testutil.StartTestServer(t, env, srv.SetupRoutes())
}
