package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/pricingsource"
)

// ---------- loadWorkerConfig ----------

func TestLoadWorkerConfig_DefaultsWhenOnlyRequiredEnvSet(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_MAX_SESSIONS", "50")

	cfg := loadWorkerConfig()

	if cfg.PollInterval != 30*time.Minute {
		t.Errorf("PollInterval: want 30m, got %s", cfg.PollInterval)
	}
	if cfg.MaxSessions != 50 {
		t.Errorf("MaxSessions: want 50, got %d", cfg.MaxSessions)
	}
	if cfg.MaxSearchIndexSessions != 200 {
		t.Errorf("MaxSearchIndexSessions: want 200, got %d", cfg.MaxSearchIndexSessions)
	}
	if cfg.DryRun {
		t.Error("DryRun: want false")
	}
}

func TestLoadWorkerConfig_ParsesCustomPollInterval(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_MAX_SESSIONS", "50")
	t.Setenv("WORKER_POLL_INTERVAL", "5m")

	cfg := loadWorkerConfig()

	if cfg.PollInterval != 5*time.Minute {
		t.Errorf("PollInterval: want 5m, got %s", cfg.PollInterval)
	}
}

func TestLoadWorkerConfig_KeepsDefaultPollIntervalWhenGarbage(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_MAX_SESSIONS", "50")
	t.Setenv("WORKER_POLL_INTERVAL", "not-a-duration")

	cfg := loadWorkerConfig()

	if cfg.PollInterval != 30*time.Minute {
		t.Errorf("PollInterval: want 30m default, got %s", cfg.PollInterval)
	}
}

func TestLoadWorkerConfig_KeepsDefaultPollIntervalWhenZeroOrNegative(t *testing.T) {
	cases := []string{"0s", "-1m"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			clearServerEnv(t)
			t.Setenv("WORKER_MAX_SESSIONS", "50")
			t.Setenv("WORKER_POLL_INTERVAL", v)
			cfg := loadWorkerConfig()
			if cfg.PollInterval != 30*time.Minute {
				t.Errorf("PollInterval for %q: want 30m default, got %s", v, cfg.PollInterval)
			}
		})
	}
}

func TestLoadWorkerConfig_ParsesCustomMaxSearchIndexSessions(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_MAX_SESSIONS", "50")
	t.Setenv("WORKER_MAX_SEARCH_INDEX_SESSIONS", "777")

	cfg := loadWorkerConfig()

	if cfg.MaxSearchIndexSessions != 777 {
		t.Errorf("MaxSearchIndexSessions: want 777, got %d", cfg.MaxSearchIndexSessions)
	}
}

func TestLoadWorkerConfig_KeepsDefaultMaxSearchIndexSessionsWhenGarbageOrNonpositive(t *testing.T) {
	cases := []string{"abc", "0", "-3"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			clearServerEnv(t)
			t.Setenv("WORKER_MAX_SESSIONS", "50")
			t.Setenv("WORKER_MAX_SEARCH_INDEX_SESSIONS", v)
			cfg := loadWorkerConfig()
			if cfg.MaxSearchIndexSessions != 200 {
				t.Errorf("MaxSearchIndexSessions for %q: want 200 default, got %d", v, cfg.MaxSearchIndexSessions)
			}
		})
	}
}

func TestLoadWorkerConfig_EnablesDryRunForTrueAnd1(t *testing.T) {
	cases := []struct {
		val      string
		wantTrue bool
	}{
		{"true", true},
		{"1", true},
		{"TRUE", false},
		{"yes", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("val="+c.val, func(t *testing.T) {
			clearServerEnv(t)
			t.Setenv("WORKER_MAX_SESSIONS", "50")
			if c.val != "" {
				t.Setenv("WORKER_DRY_RUN", c.val)
			}
			cfg := loadWorkerConfig()
			if cfg.DryRun != c.wantTrue {
				t.Errorf("DryRun for %q: want %v, got %v", c.val, c.wantTrue, cfg.DryRun)
			}
		})
	}
}

func TestLoadWorkerConfig_FatalsWhenMaxSessionsMissing(t *testing.T) {
	clearServerEnv(t)

	got := withFatalRecover(t, func() { loadWorkerConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
	if !strings.Contains(got.msg, "missing required env var") {
		t.Errorf("fatal msg: %q", got.msg)
	}
}

func TestLoadWorkerConfig_FatalsWhenMaxSessionsZero(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_MAX_SESSIONS", "0")

	got := withFatalRecover(t, func() { loadWorkerConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
	if !strings.Contains(got.msg, "invalid WORKER_MAX_SESSIONS") {
		t.Errorf("fatal msg: %q", got.msg)
	}
}

func TestLoadWorkerConfig_FatalsWhenMaxSessionsNegative(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_MAX_SESSIONS", "-5")

	got := withFatalRecover(t, func() { loadWorkerConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

func TestLoadWorkerConfig_FatalsWhenMaxSessionsNotInteger(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_MAX_SESSIONS", "abc")

	got := withFatalRecover(t, func() { loadWorkerConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

// ---------- loadS3Config ----------

func TestLoadS3Config_LoadsAllFieldsWithSSLDefaultTrue(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("S3_ENDPOINT", "s3.example.com")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("BUCKET_NAME", "bucket")

	cfg := loadS3Config()

	if cfg.Endpoint != "s3.example.com" {
		t.Errorf("Endpoint: %q", cfg.Endpoint)
	}
	if cfg.AccessKeyID != "AKIA" {
		t.Errorf("AccessKeyID: %q", cfg.AccessKeyID)
	}
	if cfg.SecretAccessKey != "secret" {
		t.Errorf("SecretAccessKey: %q", cfg.SecretAccessKey)
	}
	if cfg.BucketName != "bucket" {
		t.Errorf("BucketName: %q", cfg.BucketName)
	}
	if !cfg.UseSSL {
		t.Error("UseSSL default: want true")
	}
}

func TestLoadS3Config_DisablesSSLWhenEnvIsFalse(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("S3_ENDPOINT", "s3.example.com")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("BUCKET_NAME", "bucket")
	t.Setenv("S3_USE_SSL", "false")

	cfg := loadS3Config()

	if cfg.UseSSL {
		t.Error("UseSSL: want false when S3_USE_SSL=false")
	}
}

func TestLoadS3Config_KeepsSSLEnabledForAnyOtherString(t *testing.T) {
	cases := []string{"no", "0", "False"} // case-sensitive: "False" != "false"
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			clearServerEnv(t)
			t.Setenv("S3_ENDPOINT", "s3.example.com")
			t.Setenv("AWS_ACCESS_KEY_ID", "AKIA")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
			t.Setenv("BUCKET_NAME", "bucket")
			t.Setenv("S3_USE_SSL", v)
			cfg := loadS3Config()
			if !cfg.UseSSL {
				t.Errorf("UseSSL for %q: want true (only literal 'false' disables)", v)
			}
		})
	}
}

func TestLoadS3Config_FatalsWhenS3EndpointMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("BUCKET_NAME", "bucket")

	got := withFatalRecover(t, func() { loadS3Config() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

func TestLoadS3Config_FatalsWhenAWSAccessKeyIDMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("S3_ENDPOINT", "s3.example.com")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("BUCKET_NAME", "bucket")

	got := withFatalRecover(t, func() { loadS3Config() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

func TestLoadS3Config_FatalsWhenAWSSecretAccessKeyMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("S3_ENDPOINT", "s3.example.com")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA")
	t.Setenv("BUCKET_NAME", "bucket")

	got := withFatalRecover(t, func() { loadS3Config() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

func TestLoadS3Config_FatalsWhenBucketNameMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("S3_ENDPOINT", "s3.example.com")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

	got := withFatalRecover(t, func() { loadS3Config() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

// ---------- loadPrecomputeConfig ----------

func TestLoadPrecomputeConfig_DefaultsWhenSmartRecapDisabled(t *testing.T) {
	clearServerEnv(t)

	cfg := loadPrecomputeConfig()

	if cfg.SmartRecapEnabled {
		t.Error("SmartRecapEnabled: want false by default")
	}
	if cfg.LockTimeoutSeconds != 60 {
		t.Errorf("LockTimeoutSeconds: want 60, got %d", cfg.LockTimeoutSeconds)
	}
	if cfg.RegularCardsThresholds != analytics.DefaultRegularCardsThresholds() {
		t.Errorf("RegularCardsThresholds: want defaults, got %+v", cfg.RegularCardsThresholds)
	}
	if cfg.SmartRecapThresholds != analytics.DefaultSmartRecapThresholds() {
		t.Errorf("SmartRecapThresholds: want defaults, got %+v", cfg.SmartRecapThresholds)
	}
}

func TestLoadPrecomputeConfig_DisablesWhenAPIKeyMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("SMART_RECAP_ENABLED", "true")
	t.Setenv("SMART_RECAP_MODEL", "claude-sonnet-4-6")
	// ANTHROPIC_API_KEY missing

	cfg := loadPrecomputeConfig()

	if cfg.SmartRecapEnabled {
		t.Error("SmartRecapEnabled: want false when ANTHROPIC_API_KEY missing")
	}
}

func TestLoadPrecomputeConfig_DisablesWhenModelMissing(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("SMART_RECAP_ENABLED", "true")
	t.Setenv("ANTHROPIC_API_KEY", "key")
	// SMART_RECAP_MODEL missing

	cfg := loadPrecomputeConfig()

	if cfg.SmartRecapEnabled {
		t.Error("SmartRecapEnabled: want false when SMART_RECAP_MODEL missing")
	}
}

func TestLoadPrecomputeConfig_EnablesWhenAllRequiredSet(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("SMART_RECAP_ENABLED", "true")
	t.Setenv("ANTHROPIC_API_KEY", "key")
	t.Setenv("SMART_RECAP_MODEL", "claude-sonnet-4-6")

	cfg := loadPrecomputeConfig()

	if !cfg.SmartRecapEnabled {
		t.Error("SmartRecapEnabled: want true")
	}
	if cfg.AnthropicAPIKey != "key" {
		t.Errorf("AnthropicAPIKey: %q", cfg.AnthropicAPIKey)
	}
	if cfg.SmartRecapModel != "claude-sonnet-4-6" {
		t.Errorf("SmartRecapModel: %q", cfg.SmartRecapModel)
	}
}

func TestLoadPrecomputeConfig_ParsesQuotaLimitPositive(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("SMART_RECAP_QUOTA_LIMIT", "500")

	cfg := loadPrecomputeConfig()

	if cfg.SmartRecapQuota != 500 {
		t.Errorf("SmartRecapQuota: want 500, got %d", cfg.SmartRecapQuota)
	}
}

func TestLoadPrecomputeConfig_ParsesQuotaLimitZeroAsUnlimited(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("SMART_RECAP_QUOTA_LIMIT", "0")

	cfg := loadPrecomputeConfig()

	if cfg.SmartRecapQuota != 0 {
		t.Errorf("SmartRecapQuota: want 0 (unlimited), got %d", cfg.SmartRecapQuota)
	}
}

func TestLoadPrecomputeConfig_ParsesMaxOutputTokensAndTranscriptTokens(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("SMART_RECAP_MAX_OUTPUT_TOKENS", "8192")
	t.Setenv("SMART_RECAP_MAX_TRANSCRIPT_TOKENS", "120000")

	cfg := loadPrecomputeConfig()

	if cfg.MaxOutputTokens != 8192 {
		t.Errorf("MaxOutputTokens: want 8192, got %d", cfg.MaxOutputTokens)
	}
	if cfg.MaxTranscriptTokens != 120000 {
		t.Errorf("MaxTranscriptTokens: want 120000, got %d", cfg.MaxTranscriptTokens)
	}
}

func TestLoadPrecomputeConfig_LoadsRegularAndSmartRecapThresholdsWithCorrectPrefixes(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_REGULAR_BASE_MIN_LINES", "42")
	t.Setenv("WORKER_RECAP_BASE_MIN_LINES", "999")

	cfg := loadPrecomputeConfig()

	if cfg.RegularCardsThresholds.BaseMinLines != 42 {
		t.Errorf("RegularCardsThresholds.BaseMinLines: want 42, got %d", cfg.RegularCardsThresholds.BaseMinLines)
	}
	if cfg.SmartRecapThresholds.BaseMinLines != 999 {
		t.Errorf("SmartRecapThresholds.BaseMinLines: want 999, got %d", cfg.SmartRecapThresholds.BaseMinLines)
	}
}

func TestLoadPrecomputeConfig_FatalsOnNegativeQuotaLimit(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("SMART_RECAP_QUOTA_LIMIT", "-1")

	got := withFatalRecover(t, func() { loadPrecomputeConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

func TestLoadPrecomputeConfig_FatalsOnNonIntegerQuotaLimit(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("SMART_RECAP_QUOTA_LIMIT", "abc")

	got := withFatalRecover(t, func() { loadPrecomputeConfig() })
	if got == nil {
		t.Fatal("expected logFatal")
	}
}

// ---------- loadStalenessThresholds ----------

func TestLoadStalenessThresholds_KeepsDefaultsWhenEnvUnset(t *testing.T) {
	clearServerEnv(t)
	defaults := analytics.DefaultRegularCardsThresholds()

	got := loadStalenessThresholds("WORKER_REGULAR", defaults)

	if got != defaults {
		t.Errorf("want defaults %+v, got %+v", defaults, got)
	}
}

func TestLoadStalenessThresholds_ParsesAllFiveFields(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("WORKER_REGULAR_THRESHOLD_PCT", "0.35")
	t.Setenv("WORKER_REGULAR_BASE_MIN_LINES", "7")
	t.Setenv("WORKER_REGULAR_BASE_MIN_TIME", "5m")
	t.Setenv("WORKER_REGULAR_MIN_INITIAL_LINES", "12")
	t.Setenv("WORKER_REGULAR_MIN_SESSION_AGE", "15m")

	got := loadStalenessThresholds("WORKER_REGULAR", analytics.DefaultRegularCardsThresholds())

	if got.ThresholdPct != 0.35 {
		t.Errorf("ThresholdPct: want 0.35, got %f", got.ThresholdPct)
	}
	if got.BaseMinLines != 7 {
		t.Errorf("BaseMinLines: want 7, got %d", got.BaseMinLines)
	}
	if got.BaseMinTime != 5*time.Minute {
		t.Errorf("BaseMinTime: want 5m, got %s", got.BaseMinTime)
	}
	if got.MinInitialLines != 12 {
		t.Errorf("MinInitialLines: want 12, got %d", got.MinInitialLines)
	}
	if got.MinSessionAge != 15*time.Minute {
		t.Errorf("MinSessionAge: want 15m, got %s", got.MinSessionAge)
	}
}

func TestLoadStalenessThresholds_SilentlyKeepsDefaultForOutOfRangePct(t *testing.T) {
	defaults := analytics.DefaultRegularCardsThresholds()
	for _, v := range []string{"1.5", "-0.2"} {
		t.Run("pct="+v, func(t *testing.T) {
			clearServerEnv(t)
			t.Setenv("WORKER_REGULAR_THRESHOLD_PCT", v)
			got := loadStalenessThresholds("WORKER_REGULAR", defaults)
			if got.ThresholdPct != defaults.ThresholdPct {
				t.Errorf("ThresholdPct for %q: want default %f, got %f", v, defaults.ThresholdPct, got.ThresholdPct)
			}
		})
	}
}

func TestLoadStalenessThresholds_SilentlyKeepsDefaultForNegativeIntOrDuration(t *testing.T) {
	defaults := analytics.DefaultRegularCardsThresholds()
	clearServerEnv(t)
	t.Setenv("WORKER_REGULAR_BASE_MIN_LINES", "-3")
	t.Setenv("WORKER_REGULAR_BASE_MIN_TIME", "-5m")
	t.Setenv("WORKER_REGULAR_MIN_INITIAL_LINES", "-1")
	t.Setenv("WORKER_REGULAR_MIN_SESSION_AGE", "-2m")

	got := loadStalenessThresholds("WORKER_REGULAR", defaults)

	if got.BaseMinLines != defaults.BaseMinLines {
		t.Errorf("BaseMinLines: want default %d, got %d", defaults.BaseMinLines, got.BaseMinLines)
	}
	if got.BaseMinTime != defaults.BaseMinTime {
		t.Errorf("BaseMinTime: want default %s, got %s", defaults.BaseMinTime, got.BaseMinTime)
	}
	if got.MinInitialLines != defaults.MinInitialLines {
		t.Errorf("MinInitialLines: want default %d, got %d", defaults.MinInitialLines, got.MinInitialLines)
	}
	if got.MinSessionAge != defaults.MinSessionAge {
		t.Errorf("MinSessionAge: want default %s, got %s", defaults.MinSessionAge, got.MinSessionAge)
	}
}

func TestLoadStalenessThresholds_SilentlyKeepsDefaultForGarbageString(t *testing.T) {
	defaults := analytics.DefaultRegularCardsThresholds()
	clearServerEnv(t)
	t.Setenv("WORKER_REGULAR_THRESHOLD_PCT", "abc")
	t.Setenv("WORKER_REGULAR_BASE_MIN_LINES", "xyz")
	t.Setenv("WORKER_REGULAR_BASE_MIN_TIME", "not-a-duration")
	t.Setenv("WORKER_REGULAR_MIN_INITIAL_LINES", "qqq")
	t.Setenv("WORKER_REGULAR_MIN_SESSION_AGE", "garbage")

	got := loadStalenessThresholds("WORKER_REGULAR", defaults)

	if got != defaults {
		t.Errorf("want defaults preserved, got %+v", got)
	}
}

// ---------- Worker.processSessions ----------

func newTestWorker(fp *fakePrecomputer, cfg WorkerConfig) *Worker {
	// Disabled source (empty URL) → runOnce primes from the embedded table, no network.
	return &Worker{
		precomputer:   fp,
		config:        cfg,
		pricingSource: pricingsource.NewSource(pricingsource.Embedded(), "", time.Hour),
	}
}

func sess(id string) analytics.StaleSession {
	return analytics.StaleSession{SessionID: id, UserID: 1, ExternalID: "ext-" + id, Provider: "claude-code"}
}

func TestWorkerProcessSessions_EmptyInputReturnsZeroCounts(t *testing.T) {
	w := newTestWorker(&fakePrecomputer{}, WorkerConfig{MaxSessions: 10})
	called := false
	processed, errs := w.processSessions(context.Background(), nil, "test",
		func(context.Context, analytics.StaleSession) error { called = true; return nil }, 0)
	if processed != 0 || errs != 0 {
		t.Errorf("counts: processed=%d errors=%d, want 0/0", processed, errs)
	}
	if called {
		t.Error("process fn must not be called for empty input")
	}
}

func TestWorkerProcessSessions_CountsSuccessesAndErrorsSeparately(t *testing.T) {
	w := newTestWorker(&fakePrecomputer{}, WorkerConfig{MaxSessions: 10})
	sessions := []analytics.StaleSession{sess("a"), sess("b"), sess("c"), sess("d")}

	processed, errs := w.processSessions(context.Background(), sessions, "test",
		func(_ context.Context, s analytics.StaleSession) error {
			if s.SessionID == "b" || s.SessionID == "d" {
				return errors.New("boom")
			}
			return nil
		}, 0)

	if processed != 2 {
		t.Errorf("processed: want 2, got %d", processed)
	}
	if errs != 2 {
		t.Errorf("errors: want 2, got %d", errs)
	}
}

func TestWorkerProcessSessions_TreatsQuotaExceededAsErrorCount(t *testing.T) {
	w := newTestWorker(&fakePrecomputer{}, WorkerConfig{MaxSessions: 10})
	sessions := []analytics.StaleSession{sess("a"), sess("b")}

	processed, errs := w.processSessions(context.Background(), sessions, "test",
		func(context.Context, analytics.StaleSession) error { return analytics.ErrQuotaExceeded }, 0)

	if processed != 0 {
		t.Errorf("processed: want 0, got %d", processed)
	}
	if errs != 2 {
		t.Errorf("errors: want 2 (quota counts as error), got %d", errs)
	}
}

func TestWorkerProcessSessions_ContextCancelBeforeFirstSessionReturnsImmediately(t *testing.T) {
	w := newTestWorker(&fakePrecomputer{}, WorkerConfig{MaxSessions: 10})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	processed, errs := w.processSessions(ctx, []analytics.StaleSession{sess("a"), sess("b")}, "test",
		func(context.Context, analytics.StaleSession) error { called = true; return nil }, 0)

	if called {
		t.Error("process fn called despite canceled ctx")
	}
	if processed != 0 || errs != 0 {
		t.Errorf("counts: processed=%d errors=%d, want 0/0", processed, errs)
	}
}

func TestWorkerProcessSessions_ContextCancelMidLoopStopsProcessing(t *testing.T) {
	w := newTestWorker(&fakePrecomputer{}, WorkerConfig{MaxSessions: 10})
	ctx, cancel := context.WithCancel(context.Background())

	var calls int32
	processed, _ := w.processSessions(ctx, []analytics.StaleSession{sess("a"), sess("b"), sess("c")}, "test",
		func(context.Context, analytics.StaleSession) error {
			atomic.AddInt32(&calls, 1)
			if atomic.LoadInt32(&calls) == 1 {
				cancel()
			}
			return nil
		}, 0)

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("process fn called %d times after cancel; want 1", calls)
	}
	if processed != 1 {
		t.Errorf("processed: want 1, got %d", processed)
	}
}

func TestWorkerProcessSessions_NoPacingAfterLastSession(t *testing.T) {
	// With a single session, no inter-session pacing fires at all, so any
	// elapsed time near `pacing` would mean a trailing pacing leaked. The
	// production loop guards this with `if i < len(sessions)-1`.
	w := newTestWorker(&fakePrecomputer{}, WorkerConfig{MaxSessions: 10})
	pacing := 2 * time.Second
	start := time.Now()
	processed, _ := w.processSessions(context.Background(),
		[]analytics.StaleSession{sess("a")}, "test",
		func(context.Context, analytics.StaleSession) error { return nil },
		pacing)
	elapsed := time.Since(start)
	if processed != 1 {
		t.Errorf("processed: want 1, got %d", processed)
	}
	if elapsed >= pacing {
		t.Errorf("elapsed %s — pacing should not apply after last session", elapsed)
	}
}

// ---------- Worker.process{Regular,SmartRecap,SearchIndex}Sessions ----------

func TestWorkerProcessRegularSessions_CallsPrecomputeRegularCards(t *testing.T) {
	fp := &fakePrecomputer{}
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10})
	processed, _ := w.processRegularSessions(context.Background(), []analytics.StaleSession{sess("a")})
	if processed != 1 {
		t.Errorf("processed: want 1, got %d", processed)
	}
	if len(fp.regularCalls) != 1 || fp.regularCalls[0].SessionID != "a" {
		t.Errorf("PrecomputeRegularCards calls: %+v", fp.regularCalls)
	}
	if len(fp.recapCalls) != 0 || len(fp.searchIdxCalls) != 0 {
		t.Error("other precomputer methods should not be called")
	}
}

func TestWorkerProcessSmartRecapSessions_CallsPrecomputeSmartRecapOnly(t *testing.T) {
	fp := &fakePrecomputer{}
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10})
	processed, _ := w.processSmartRecapSessions(context.Background(), []analytics.StaleSession{sess("a")})
	if processed != 1 {
		t.Errorf("processed: want 1, got %d", processed)
	}
	if len(fp.recapCalls) != 1 || fp.recapCalls[0].SessionID != "a" {
		t.Errorf("PrecomputeSmartRecapOnly calls: %+v", fp.recapCalls)
	}
	if len(fp.regularCalls) != 0 || len(fp.searchIdxCalls) != 0 {
		t.Error("other precomputer methods should not be called")
	}
}

func TestWorkerProcessSearchIndexSessions_CallsBuildSearchIndexOnly(t *testing.T) {
	fp := &fakePrecomputer{}
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10})
	processed, _ := w.processSearchIndexSessions(context.Background(), []analytics.StaleSession{sess("a")})
	if processed != 1 {
		t.Errorf("processed: want 1, got %d", processed)
	}
	if len(fp.searchIdxCalls) != 1 || fp.searchIdxCalls[0].SessionID != "a" {
		t.Errorf("BuildSearchIndexOnly calls: %+v", fp.searchIdxCalls)
	}
	if len(fp.regularCalls) != 0 || len(fp.recapCalls) != 0 {
		t.Error("other precomputer methods should not be called")
	}
}

// ---------- Worker.runOnce ----------

func TestWorkerRunOnce_NoStaleSessionsReturnsEarly(t *testing.T) {
	fp := &fakePrecomputer{} // all Find* default to empty
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10, MaxSearchIndexSessions: 10})
	w.runOnce(context.Background())

	if fp.findStaleCalls != 1 || fp.findSmartRecapCalls != 1 || fp.findSearchIndexCalls != 1 {
		t.Errorf("Find* calls: stale=%d recap=%d search=%d; want 1/1/1",
			fp.findStaleCalls, fp.findSmartRecapCalls, fp.findSearchIndexCalls)
	}
	if len(fp.regularCalls) != 0 || len(fp.recapCalls) != 0 || len(fp.searchIdxCalls) != 0 {
		t.Error("Precompute* must not be called when all buckets empty")
	}
}

func TestWorkerRunOnce_DryRunModeLogsButDoesNotProcess(t *testing.T) {
	fp := &fakePrecomputer{
		findStaleFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return []analytics.StaleSession{sess("a")}, nil
		},
		findSmartRecapFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return []analytics.StaleSession{sess("b")}, nil
		},
		findSearchIndexFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return []analytics.StaleSession{sess("c")}, nil
		},
	}
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10, MaxSearchIndexSessions: 10, DryRun: true})
	w.runOnce(context.Background())

	if len(fp.regularCalls) != 0 || len(fp.recapCalls) != 0 || len(fp.searchIdxCalls) != 0 {
		t.Errorf("dry-run must skip processing; regular=%d recap=%d search=%d",
			len(fp.regularCalls), len(fp.recapCalls), len(fp.searchIdxCalls))
	}
}

func TestWorkerRunOnce_ProcessesAllThreeBuckets(t *testing.T) {
	fp := &fakePrecomputer{
		findStaleFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return []analytics.StaleSession{sess("r1"), sess("r2")}, nil
		},
		findSmartRecapFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return []analytics.StaleSession{sess("sr1")}, nil
		},
		findSearchIndexFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return []analytics.StaleSession{sess("si1"), sess("si2"), sess("si3")}, nil
		},
	}
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10, MaxSearchIndexSessions: 10})
	w.runOnce(context.Background())

	if len(fp.regularCalls) != 2 {
		t.Errorf("regular bucket: want 2 calls, got %d", len(fp.regularCalls))
	}
	if len(fp.recapCalls) != 1 {
		t.Errorf("recap bucket: want 1 call, got %d", len(fp.recapCalls))
	}
	if len(fp.searchIdxCalls) != 3 {
		t.Errorf("search-index bucket: want 3 calls, got %d", len(fp.searchIdxCalls))
	}
}

func TestWorkerRunOnce_FindStaleSessionsErrorSkipsRemainingBuckets(t *testing.T) {
	// Pins current behavior: an error in bucket 1's Find* aborts the whole cycle.
	fp := &fakePrecomputer{
		findStaleFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return nil, errors.New("bucket1 find failed")
		},
	}
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10, MaxSearchIndexSessions: 10})
	w.runOnce(context.Background())

	if fp.findSmartRecapCalls != 0 {
		t.Errorf("FindStaleSmartRecapSessions should not be called when bucket1 fails; calls=%d", fp.findSmartRecapCalls)
	}
	if fp.findSearchIndexCalls != 0 {
		t.Errorf("FindStaleSearchIndexSessions should not be called when bucket1 fails; calls=%d", fp.findSearchIndexCalls)
	}
	if len(fp.regularCalls) != 0 {
		t.Error("PrecomputeRegularCards must not be called when bucket1 Find fails")
	}
}

func TestWorkerRunOnce_FindSmartRecapErrorSkipsSearchIndexAndProcessing(t *testing.T) {
	fp := &fakePrecomputer{
		findSmartRecapFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return nil, errors.New("bucket2 find failed")
		},
	}
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10, MaxSearchIndexSessions: 10})
	w.runOnce(context.Background())

	if fp.findSearchIndexCalls != 0 {
		t.Errorf("FindStaleSearchIndexSessions should not be called when bucket2 fails; calls=%d", fp.findSearchIndexCalls)
	}
	if len(fp.regularCalls) != 0 || len(fp.recapCalls) != 0 || len(fp.searchIdxCalls) != 0 {
		t.Error("no Precompute* must be called when a Find error short-circuits the cycle")
	}
}

func TestWorkerRunOnce_FindSearchIndexErrorSkipsProcessing(t *testing.T) {
	fp := &fakePrecomputer{
		findStaleFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return []analytics.StaleSession{sess("r1")}, nil
		},
		findSmartRecapFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return []analytics.StaleSession{sess("sr1")}, nil
		},
		findSearchIndexFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			return nil, errors.New("bucket3 find failed")
		},
	}
	w := newTestWorker(fp, WorkerConfig{MaxSessions: 10, MaxSearchIndexSessions: 10})
	w.runOnce(context.Background())

	// Even though buckets 1 and 2 returned sessions, the early return on bucket 3's
	// error happens before any processing starts.
	if len(fp.regularCalls) != 0 || len(fp.recapCalls) != 0 || len(fp.searchIdxCalls) != 0 {
		t.Errorf("no processing should happen when bucket3 Find fails; regular=%d recap=%d search=%d",
			len(fp.regularCalls), len(fp.recapCalls), len(fp.searchIdxCalls))
	}
}

// ---------- Worker.Run ----------

func TestWorkerRun_RunsOnceImmediatelyThenExitsOnContextCancel(t *testing.T) {
	var cycles int32
	fp := &fakePrecomputer{
		findStaleFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			atomic.AddInt32(&cycles, 1)
			return nil, nil
		},
	}
	w := newTestWorker(fp, WorkerConfig{
		MaxSessions: 10, MaxSearchIndexSessions: 10,
		PollInterval: 1 * time.Hour, // ticker won't fire within the test window
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	// Wait for the immediate-on-startup run to register.
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&cycles) < 1 {
		select {
		case <-deadline:
			t.Fatal("worker did not run immediately on startup")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit within 2s after context cancel")
	}
	if got := atomic.LoadInt32(&cycles); got != 1 {
		t.Errorf("cycles: want exactly 1 (immediate, no ticker fired), got %d", got)
	}
}

func TestWorkerRun_TickerFiresSubsequentCycles(t *testing.T) {
	var cycles int32
	fp := &fakePrecomputer{
		findStaleFn: func(context.Context, int) ([]analytics.StaleSession, error) {
			atomic.AddInt32(&cycles, 1)
			return nil, nil
		},
	}
	w := newTestWorker(fp, WorkerConfig{
		MaxSessions: 10, MaxSearchIndexSessions: 10,
		PollInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	w.Run(ctx)

	if got := atomic.LoadInt32(&cycles); got < 2 {
		t.Errorf("cycles: want >= 2 with 10ms ticker over 80ms, got %d", got)
	}
}
