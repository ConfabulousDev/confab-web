package main

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig_DefaultsWhenOnlyRequiredEnvSet(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)

	cfg := loadConfig()

	if cfg.Port != 8080 {
		t.Errorf("Port: want 8080, got %d", cfg.Port)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout: want 30s, got %s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout: want 30s, got %s", cfg.WriteTimeout)
	}
	if cfg.DatabaseURL != "postgres://test" {
		t.Errorf("DatabaseURL: want postgres://test, got %q", cfg.DatabaseURL)
	}
	if !cfg.OAuthConfig.PasswordEnabled {
		t.Error("PasswordEnabled: want true")
	}
	if cfg.OAuthConfig.GitHubEnabled || cfg.OAuthConfig.GoogleEnabled || cfg.OAuthConfig.OIDCEnabled {
		t.Errorf("only password should be enabled; oauth=%+v", cfg.OAuthConfig)
	}
	if cfg.EmailConfig.Enabled {
		t.Error("EmailConfig.Enabled: want false")
	}
	if cfg.EmailConfig.FromName != "Confab" {
		t.Errorf("EmailConfig.FromName default: want Confab, got %q", cfg.EmailConfig.FromName)
	}
	if cfg.EmailConfig.RateLimitPerHour != 100 {
		t.Errorf("EmailConfig.RateLimitPerHour default: want 100, got %d", cfg.EmailConfig.RateLimitPerHour)
	}
}

func TestLoadConfig_ParsesCustomPortAndTimeouts(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("HTTP_READ_TIMEOUT", "15s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "45s")

	cfg := loadConfig()

	if cfg.Port != 9090 {
		t.Errorf("Port: want 9090, got %d", cfg.Port)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout: want 15s, got %s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 45*time.Second {
		t.Errorf("WriteTimeout: want 45s, got %s", cfg.WriteTimeout)
	}
}

func TestLoadConfig_EnablesGitHubOAuthWhenAllEnvSet(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("GITHUB_CLIENT_ID", "gh-client")
	t.Setenv("GITHUB_CLIENT_SECRET", "gh-secret")
	t.Setenv("GITHUB_REDIRECT_URL", "http://localhost/gh/cb")

	cfg := loadConfig()

	if !cfg.OAuthConfig.GitHubEnabled {
		t.Fatal("GitHubEnabled: want true")
	}
	if cfg.OAuthConfig.GitHubClientID != "gh-client" {
		t.Errorf("GitHubClientID: want gh-client, got %q", cfg.OAuthConfig.GitHubClientID)
	}
	if cfg.OAuthConfig.GitHubClientSecret != "gh-secret" {
		t.Errorf("GitHubClientSecret: want gh-secret, got %q", cfg.OAuthConfig.GitHubClientSecret)
	}
	if cfg.OAuthConfig.GitHubRedirectURL != "http://localhost/gh/cb" {
		t.Errorf("GitHubRedirectURL: got %q", cfg.OAuthConfig.GitHubRedirectURL)
	}
}

func TestLoadConfig_EnablesGoogleOAuthWhenAllEnvSet(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("GOOGLE_CLIENT_ID", "g-client")
	t.Setenv("GOOGLE_CLIENT_SECRET", "g-secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "http://localhost/google/cb")

	cfg := loadConfig()

	if !cfg.OAuthConfig.GoogleEnabled {
		t.Fatal("GoogleEnabled: want true")
	}
	if cfg.OAuthConfig.GoogleClientID != "g-client" ||
		cfg.OAuthConfig.GoogleClientSecret != "g-secret" ||
		cfg.OAuthConfig.GoogleRedirectURL != "http://localhost/google/cb" {
		t.Errorf("Google fields not populated correctly: %+v", cfg.OAuthConfig)
	}
}

func TestLoadConfig_EnablesOIDCWhenAllEnvSet(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "oidc-client")
	t.Setenv("OIDC_CLIENT_SECRET", "oidc-secret")
	t.Setenv("OIDC_REDIRECT_URL", "http://localhost/oidc/cb")

	cfg := loadConfig()

	if !cfg.OAuthConfig.OIDCEnabled {
		t.Fatal("OIDCEnabled: want true")
	}
	if cfg.OAuthConfig.OIDCDisplayName != "SSO" {
		t.Errorf("OIDCDisplayName default: want SSO, got %q", cfg.OAuthConfig.OIDCDisplayName)
	}
}

func TestLoadConfig_PreservesCustomOIDCDisplayName(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "oidc-client")
	t.Setenv("OIDC_CLIENT_SECRET", "oidc-secret")
	t.Setenv("OIDC_REDIRECT_URL", "http://localhost/oidc/cb")
	t.Setenv("OIDC_DISPLAY_NAME", "Okta")

	cfg := loadConfig()

	if cfg.OAuthConfig.OIDCDisplayName != "Okta" {
		t.Errorf("OIDCDisplayName: want Okta, got %q", cfg.OAuthConfig.OIDCDisplayName)
	}
}

func TestLoadConfig_ParsesAllowedEmailDomainsWithWhitespaceAndCase(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("ALLOWED_EMAIL_DOMAINS", "  Example.com , ACME.co.uk ,, foo.IO ")

	cfg := loadConfig()

	want := []string{"example.com", "acme.co.uk", "foo.io"}
	if !reflect.DeepEqual(cfg.OAuthConfig.AllowedEmailDomains, want) {
		t.Errorf("AllowedEmailDomains: want %v, got %v", want, cfg.OAuthConfig.AllowedEmailDomains)
	}
}

func TestLoadConfig_EnablesEmailWhenAPIKeyAndFromAddressSet(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("RESEND_API_KEY", "re_test_key")
	t.Setenv("EMAIL_FROM_ADDRESS", "noreply@example.com")
	t.Setenv("EMAIL_FROM_NAME", "Custom Sender")

	cfg := loadConfig()

	if !cfg.EmailConfig.Enabled {
		t.Fatal("EmailConfig.Enabled: want true")
	}
	if cfg.EmailConfig.APIKey != "re_test_key" {
		t.Errorf("APIKey: got %q", cfg.EmailConfig.APIKey)
	}
	if cfg.EmailConfig.FromAddress != "noreply@example.com" {
		t.Errorf("FromAddress: got %q", cfg.EmailConfig.FromAddress)
	}
	if cfg.EmailConfig.FromName != "Custom Sender" {
		t.Errorf("FromName: got %q", cfg.EmailConfig.FromName)
	}
}

func TestLoadConfig_DoesNotEnableEmailWithOnlyAPIKey(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("RESEND_API_KEY", "re_test_key")
	// EMAIL_FROM_ADDRESS not set
	cfg := loadConfig()
	if cfg.EmailConfig.Enabled {
		t.Error("EmailConfig.Enabled: want false when from-address missing")
	}
}

func TestLoadConfig_DoesNotEnableEmailWithOnlyFromAddress(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("EMAIL_FROM_ADDRESS", "noreply@example.com")
	// RESEND_API_KEY not set
	cfg := loadConfig()
	if cfg.EmailConfig.Enabled {
		t.Error("EmailConfig.Enabled: want false when API key missing")
	}
}

func TestLoadConfig_ParsesEmailRateLimitPerHour(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("EMAIL_RATE_LIMIT_PER_HOUR", "250")

	cfg := loadConfig()

	if cfg.EmailConfig.RateLimitPerHour != 250 {
		t.Errorf("RateLimitPerHour: want 250, got %d", cfg.EmailConfig.RateLimitPerHour)
	}
}

func TestLoadConfig_FatalsWhenNoAuthMethodConfigured(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("CSRF_SECRET_KEY", "this-is-a-32-character-long-secret-key")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	// no auth method enabled

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected logFatal to be called")
	}
	if !strings.Contains(got.msg, "no authentication method configured") {
		t.Errorf("fatal msg: got %q", got.msg)
	}
}

func TestLoadConfig_FatalsWhenCSRFSecretKeyMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("AUTH_PASSWORD_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	// CSRF_SECRET_KEY missing

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
	if !strings.Contains(got.msg, "missing required env var") {
		t.Errorf("fatal msg: got %q", got.msg)
	}
}

func TestLoadConfig_FatalsWhenCSRFSecretKeyTooShort(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("AUTH_PASSWORD_ENABLED", "true")
	t.Setenv("CSRF_SECRET_KEY", "short-key-only-thirty-one-chars") // 31 chars
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
	if !strings.Contains(got.msg, "invalid env var") {
		t.Errorf("fatal msg: got %q", got.msg)
	}
}

func TestLoadConfig_FatalsWhenDatabaseURLMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("AUTH_PASSWORD_ENABLED", "true")
	t.Setenv("CSRF_SECRET_KEY", "this-is-a-32-character-long-secret-key")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	// DATABASE_URL missing

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

func TestLoadConfig_FatalsWhenFrontendURLMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("AUTH_PASSWORD_ENABLED", "true")
	t.Setenv("CSRF_SECRET_KEY", "this-is-a-32-character-long-secret-key")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	// FRONTEND_URL missing

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

func TestLoadConfig_FatalsWhenAllowedOriginsMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("AUTH_PASSWORD_ENABLED", "true")
	t.Setenv("CSRF_SECRET_KEY", "this-is-a-32-character-long-secret-key")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")
	// ALLOWED_ORIGINS missing

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

func TestLoadConfig_FatalsWhenAllowedEmailDomainsInvalid(t *testing.T) {
	clearServerEnv(t)
	setRequiredServerEnv(t)
	t.Setenv("ALLOWED_EMAIL_DOMAINS", "nodot") // fails ValidateDomainList (no TLD)

	got := withFatalRecover(t, func() { loadConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
	if !strings.Contains(got.msg, "invalid ALLOWED_EMAIL_DOMAINS") {
		t.Errorf("fatal msg: got %q", got.msg)
	}
}

func TestBuildPprofMux_RoutesEachRegisteredEndpoint(t *testing.T) {
	// /debug/pprof/profile (30s CPU profile) and /debug/pprof/trace are
	// intentionally excluded: they are long-running, and their registration
	// pattern is structurally identical to the other handlers below.
	paths := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/symbol",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/allocs",
		"/debug/pprof/block",
		"/debug/pprof/mutex",
		"/debug/pprof/threadcreate",
	}

	mux := buildPprofMux()

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest("GET", p, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != 200 {
				t.Errorf("path %s: status = %d, want 200; body=%s", p, rec.Code, rec.Body.String())
			}
		})
	}
}
