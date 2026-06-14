package main

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
)

// fatalCalled carries the arguments captured when logFatal is intercepted.
type fatalCalled struct {
	msg  string
	args []any
}

// withFatalRecover swaps logFatal for a panicking stub for the duration of fn.
// Returns the captured call if fn triggered logFatal, otherwise nil. Re-panics
// on any non-sentinel value to surface real test bugs.
func withFatalRecover(t *testing.T, fn func()) *fatalCalled {
	t.Helper()

	original := logFatal
	logFatal = func(msg string, args ...any) {
		panic(fatalCalled{msg: msg, args: args})
	}
	t.Cleanup(func() { logFatal = original })

	var captured *fatalCalled
	func() {
		defer func() {
			if r := recover(); r != nil {
				if fc, ok := r.(fatalCalled); ok {
					captured = &fc
					return
				}
				panic(r)
			}
		}()
		fn()
	}()
	return captured
}

// serverEnvKeys is every env var read by main.go and worker.go. clearServerEnv
// blanks each one so tests start from a known clean slate.
var serverEnvKeys = []string{
	"PORT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT",
	"AUTH_PASSWORD_ENABLED",
	"GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "GITHUB_REDIRECT_URL",
	"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL",
	"OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET",
	"OIDC_REDIRECT_URL", "OIDC_DISPLAY_NAME",
	"OAUTH_AUTO_LINK_EMAIL", "DEMO_IDENTITY_EMAIL", "SUPER_ADMIN_EMAILS",
	"ALLOWED_EMAIL_DOMAINS", "CSRF_SECRET_KEY", "DATABASE_URL",
	"FRONTEND_URL", "ALLOWED_ORIGINS", "INSECURE_DEV_MODE",
	"ADMIN_BOOTSTRAP_EMAIL", "ADMIN_BOOTSTRAP_PASSWORD",
	"RESEND_API_KEY", "EMAIL_FROM_ADDRESS", "EMAIL_FROM_NAME",
	"EMAIL_RATE_LIMIT_PER_HOUR",
	"ENABLE_PPROF", "SHARE_ALL_SESSIONS_TO_AUTHENTICATED",
	"ENABLE_SHARE_CREATION", "ENABLE_SAAS_FOOTER", "ENABLE_SAAS_TERMLY",
	"WORKER_POLL_INTERVAL", "WORKER_MAX_SESSIONS",
	"WORKER_MAX_SEARCH_INDEX_SESSIONS", "WORKER_DRY_RUN",
	"S3_ENDPOINT", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
	"BUCKET_NAME", "S3_USE_SSL",
	"SMART_RECAP_ENABLED", "ANTHROPIC_API_KEY", "SMART_RECAP_MODEL",
	"SMART_RECAP_QUOTA_LIMIT", "SMART_RECAP_MAX_OUTPUT_TOKENS",
	"SMART_RECAP_MAX_TRANSCRIPT_TOKENS",
	"WORKER_REGULAR_THRESHOLD_PCT", "WORKER_REGULAR_BASE_MIN_LINES",
	"WORKER_REGULAR_BASE_MIN_TIME", "WORKER_REGULAR_MIN_INITIAL_LINES",
	"WORKER_REGULAR_MIN_SESSION_AGE",
	"WORKER_RECAP_THRESHOLD_PCT", "WORKER_RECAP_BASE_MIN_LINES",
	"WORKER_RECAP_BASE_MIN_TIME", "WORKER_RECAP_MIN_INITIAL_LINES",
	"WORKER_RECAP_MIN_SESSION_AGE",
}

// clearServerEnv sets every server-related env var to "" via t.Setenv (which
// also schedules restoration on cleanup). All production reads go through
// os.Getenv, which treats "" and unset identically.
func clearServerEnv(t *testing.T) {
	t.Helper()
	for _, k := range serverEnvKeys {
		t.Setenv(k, "")
	}
}

// setRequiredServerEnv populates the minimum env for loadConfig() to succeed.
// loadConfig calls loadS3Config internally, so S3 vars are seeded too.
func setRequiredServerEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CSRF_SECRET_KEY", "this-is-a-32-character-long-secret-key")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	t.Setenv("AUTH_PASSWORD_ENABLED", "true")
	t.Setenv("S3_ENDPOINT", "s3.example.com")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	t.Setenv("BUCKET_NAME", "test-bucket")
}

// fakePrecomputer satisfies precomputerAPI with per-test function-field overrides.
// Unset Fn fields return (zero, nil). The recorded *Calls slices let tests assert
// what the Worker invoked, in order. Worker.Run is single-goroutine and tests run
// serially, so no synchronization is needed.
type fakePrecomputer struct {
	findStaleFn       func(context.Context, int) ([]analytics.StaleSession, error)
	findSmartRecapFn  func(context.Context, int) ([]analytics.StaleSession, error)
	findSearchIndexFn func(context.Context, int) ([]analytics.StaleSession, error)
	precomputeRegFn   func(context.Context, analytics.StaleSession) error
	precomputeRecapFn func(context.Context, analytics.StaleSession) error
	buildSearchIdxFn  func(context.Context, analytics.StaleSession) error

	findStaleCalls       int
	findSmartRecapCalls  int
	findSearchIndexCalls int

	regularCalls   []analytics.StaleSession
	recapCalls     []analytics.StaleSession
	searchIdxCalls []analytics.StaleSession
}

func (f *fakePrecomputer) FindStaleSessions(ctx context.Context, limit int) ([]analytics.StaleSession, error) {
	f.findStaleCalls++
	if f.findStaleFn != nil {
		return f.findStaleFn(ctx, limit)
	}
	return nil, nil
}

func (f *fakePrecomputer) FindStaleSmartRecapSessions(ctx context.Context, limit int) ([]analytics.StaleSession, error) {
	f.findSmartRecapCalls++
	if f.findSmartRecapFn != nil {
		return f.findSmartRecapFn(ctx, limit)
	}
	return nil, nil
}

func (f *fakePrecomputer) FindStaleSearchIndexSessions(ctx context.Context, limit int) ([]analytics.StaleSession, error) {
	f.findSearchIndexCalls++
	if f.findSearchIndexFn != nil {
		return f.findSearchIndexFn(ctx, limit)
	}
	return nil, nil
}

func (f *fakePrecomputer) PrecomputeRegularCards(ctx context.Context, session analytics.StaleSession) error {
	f.regularCalls = append(f.regularCalls, session)
	if f.precomputeRegFn != nil {
		return f.precomputeRegFn(ctx, session)
	}
	return nil
}

func (f *fakePrecomputer) PrecomputeSmartRecapOnly(ctx context.Context, session analytics.StaleSession) error {
	f.recapCalls = append(f.recapCalls, session)
	if f.precomputeRecapFn != nil {
		return f.precomputeRecapFn(ctx, session)
	}
	return nil
}

func (f *fakePrecomputer) BuildSearchIndexOnly(ctx context.Context, session analytics.StaleSession) error {
	f.searchIdxCalls = append(f.searchIdxCalls, session)
	if f.buildSearchIdxFn != nil {
		return f.buildSearchIdxFn(ctx, session)
	}
	return nil
}
