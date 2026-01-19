package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/honeycombio/otel-config-go/otelconfig"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var workerTracer = otel.Tracer("confab/worker")

// WorkerConfig holds configuration for the analytics precompute worker.
type WorkerConfig struct {
	PollInterval time.Duration
	MaxSessions  int  // Maximum sessions to query per cycle
	DryRun       bool // If true, log what would be done without actually precomputing
}

// Worker is the background analytics precompute worker.
type Worker struct {
	db          *db.DB
	store       *storage.S3Storage
	precomputer *analytics.Precomputer
	config      WorkerConfig
}

// runWorker is the entry point for the background worker process.
func runWorker() {
	logger.Info("starting analytics precompute worker")

	// Initialize OpenTelemetry (same as server)
	otelShutdown, err := otelconfig.ConfigureOpenTelemetry()
	if err != nil {
		logger.Warn("failed to configure OpenTelemetry for worker", "error", err)
	} else {
		defer otelShutdown()
	}

	// Load worker configuration
	workerConfig := loadWorkerConfig()
	logger.Info("worker configuration loaded",
		"poll_interval", workerConfig.PollInterval,
		"max_sessions", workerConfig.MaxSessions,
		"dry_run", workerConfig.DryRun,
	)

	if workerConfig.DryRun {
		logger.Info("DRY-RUN MODE ENABLED - no sessions will be precomputed")
	}

	// Load required database/storage configuration
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Fatal("missing required env var", "var", "DATABASE_URL")
	}

	// Initialize database connection
	database, err := db.Connect(databaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", "error", err)
	}
	defer database.Close()

	// Initialize S3 storage
	s3Config := loadS3Config()
	store, err := storage.NewS3Storage(s3Config)
	if err != nil {
		logger.Fatal("failed to initialize storage", "error", err)
	}

	// Load smart recap configuration
	precomputeConfig := loadPrecomputeConfig()
	logger.Info("smart recap configuration",
		"enabled", precomputeConfig.SmartRecapEnabled,
		"model", precomputeConfig.SmartRecapModel,
		"quota", precomputeConfig.SmartRecapQuota,
	)

	// Create analytics store and precomputer
	analyticsStore := analytics.NewStore(database.Conn())
	precomputer := analytics.NewPrecomputer(database.Conn(), store, analyticsStore, precomputeConfig)

	// Create and run worker
	worker := &Worker{
		db:          database,
		store:       store,
		precomputer: precomputer,
		config:      workerConfig,
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("shutdown signal received, stopping worker")
		cancel()
	}()

	// Run the worker
	worker.Run(ctx)
	logger.Info("worker stopped")
}

// Run executes the main worker loop.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	// Run immediately on startup
	w.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// runOnce executes a single precomputation cycle.
// It processes two buckets:
// 1. Sessions with stale regular cards (computes all cards including smart recap if stale)
// 2. Sessions with only stale smart recap (computes only smart recap)
func (w *Worker) runOnce(ctx context.Context) {
	ctx, span := workerTracer.Start(ctx, "worker.run_once")
	defer span.End()

	logger.Info("starting precomputation cycle")

	// Bucket 1: Find sessions with stale regular cards
	regularSessions, err := w.precomputer.FindStaleSessions(ctx, w.config.MaxSessions)
	if err != nil {
		logger.Error("failed to find stale sessions", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	// Bucket 2: Find sessions with only stale smart recap (regular cards up-to-date)
	smartRecapSessions, err := w.precomputer.FindStaleSmartRecapSessions(ctx, w.config.MaxSessions)
	if err != nil {
		logger.Error("failed to find stale smart recap sessions", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	totalFound := len(regularSessions) + len(smartRecapSessions)
	if totalFound == 0 {
		logger.Info("no stale sessions found")
		span.SetAttributes(
			attribute.Int("sessions.regular.found", 0),
			attribute.Int("sessions.smart_recap.found", 0),
		)
		return
	}

	logger.Info("found stale sessions",
		"regular_cards", len(regularSessions),
		"smart_recap_only", len(smartRecapSessions),
	)
	span.SetAttributes(
		attribute.Int("sessions.regular.found", len(regularSessions)),
		attribute.Int("sessions.smart_recap.found", len(smartRecapSessions)),
	)

	// In dry-run mode, just log what would be processed and return
	if w.config.DryRun {
		for _, session := range regularSessions {
			logger.Info("[DRY-RUN] would precompute session (regular cards)",
				"session_id", session.SessionID,
				"user_id", session.UserID,
				"external_id", session.ExternalID,
				"total_lines", session.TotalLines,
			)
		}
		for _, session := range smartRecapSessions {
			logger.Info("[DRY-RUN] would precompute session (smart recap only)",
				"session_id", session.SessionID,
				"user_id", session.UserID,
				"external_id", session.ExternalID,
				"total_lines", session.TotalLines,
			)
		}
		logger.Info("[DRY-RUN] precomputation cycle complete",
			"would_process_regular", len(regularSessions),
			"would_process_smart_recap", len(smartRecapSessions),
		)
		span.SetAttributes(
			attribute.Bool("dry_run", true),
			attribute.Int("sessions.regular.would_process", len(regularSessions)),
			attribute.Int("sessions.smart_recap.would_process", len(smartRecapSessions)),
		)
		return
	}

	// Process Bucket 1: Sessions with stale regular cards
	regularProcessed, regularErrors := w.processRegularSessions(ctx, regularSessions)

	// Process Bucket 2: Sessions with only stale smart recap
	smartRecapProcessed, smartRecapErrors := w.processSmartRecapSessions(ctx, smartRecapSessions)

	logger.Info("precomputation cycle complete",
		"regular_processed", regularProcessed,
		"regular_errors", regularErrors,
		"smart_recap_processed", smartRecapProcessed,
		"smart_recap_errors", smartRecapErrors,
	)
	span.SetAttributes(
		attribute.Int("sessions.regular.processed", regularProcessed),
		attribute.Int("sessions.regular.errors", regularErrors),
		attribute.Int("sessions.smart_recap.processed", smartRecapProcessed),
		attribute.Int("sessions.smart_recap.errors", smartRecapErrors),
	)
}

// processRegularSessions processes sessions with stale regular cards.
func (w *Worker) processRegularSessions(ctx context.Context, sessions []analytics.StaleSession) (processed, errors int) {
	for i, session := range sessions {
		select {
		case <-ctx.Done():
			logger.Info("stopping processing due to shutdown")
			return
		default:
		}

		err := w.precomputer.PrecomputeSession(ctx, session)
		if err != nil {
			logger.Error("failed to precompute session",
				"session_id", session.SessionID,
				"user_id", session.UserID,
				"external_id", session.ExternalID,
				"total_lines", session.TotalLines,
				"error", err,
			)
			errors++
		} else {
			logger.Info("precomputed session",
				"session_id", session.SessionID,
				"user_id", session.UserID,
				"external_id", session.ExternalID,
				"total_lines", session.TotalLines,
			)
			processed++
		}

		// Brief delay between sessions for steady pacing (skip after last)
		if i < len(sessions)-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	return
}

// processSmartRecapSessions processes sessions with only stale smart recap.
func (w *Worker) processSmartRecapSessions(ctx context.Context, sessions []analytics.StaleSession) (processed, errors int) {
	for i, session := range sessions {
		select {
		case <-ctx.Done():
			logger.Info("stopping processing due to shutdown")
			return
		default:
		}

		err := w.precomputer.PrecomputeSmartRecapOnly(ctx, session)
		if err != nil {
			logger.Error("failed to precompute smart recap",
				"session_id", session.SessionID,
				"user_id", session.UserID,
				"external_id", session.ExternalID,
				"total_lines", session.TotalLines,
				"error", err,
			)
			errors++
		} else {
			logger.Info("precomputed smart recap",
				"session_id", session.SessionID,
				"user_id", session.UserID,
				"external_id", session.ExternalID,
				"total_lines", session.TotalLines,
			)
			processed++
		}

		// Brief delay between sessions for steady pacing (skip after last)
		if i < len(sessions)-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	return
}

// loadWorkerConfig loads worker configuration from environment variables.
func loadWorkerConfig() WorkerConfig {
	config := WorkerConfig{
		PollInterval: 30 * time.Minute, // Default: 30 minutes
		DryRun:       false,            // Default: actually precompute
	}

	if interval := os.Getenv("WORKER_POLL_INTERVAL"); interval != "" {
		if parsed, err := time.ParseDuration(interval); err == nil && parsed > 0 {
			config.PollInterval = parsed
		}
	}

	// MaxSessions is mandatory
	maxSessions := os.Getenv("WORKER_MAX_SESSIONS")
	if maxSessions == "" {
		logger.Fatal("missing required env var", "var", "WORKER_MAX_SESSIONS")
	}
	parsed, err := strconv.Atoi(maxSessions)
	if err != nil || parsed <= 0 {
		logger.Fatal("invalid WORKER_MAX_SESSIONS", "value", maxSessions)
	}
	config.MaxSessions = parsed

	// Dry-run mode: log what would be done without actually precomputing
	if dryRun := os.Getenv("WORKER_DRY_RUN"); dryRun == "true" || dryRun == "1" {
		config.DryRun = true
	}

	return config
}

// loadS3Config loads S3 configuration from environment variables.
func loadS3Config() storage.S3Config {
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		logger.Fatal("missing required env var", "var", "S3_ENDPOINT")
	}

	awsAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	if awsAccessKeyID == "" {
		logger.Fatal("missing required env var", "var", "AWS_ACCESS_KEY_ID")
	}

	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if awsSecretAccessKey == "" {
		logger.Fatal("missing required env var", "var", "AWS_SECRET_ACCESS_KEY")
	}

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		logger.Fatal("missing required env var", "var", "BUCKET_NAME")
	}

	return storage.S3Config{
		Endpoint:        s3Endpoint,
		AccessKeyID:     awsAccessKeyID,
		SecretAccessKey: awsSecretAccessKey,
		BucketName:      bucketName,
		UseSSL:          os.Getenv("S3_USE_SSL") != "false",
	}
}

// loadPrecomputeConfig loads smart recap configuration from environment variables.
func loadPrecomputeConfig() analytics.PrecomputeConfig {
	config := analytics.PrecomputeConfig{
		SmartRecapEnabled:  os.Getenv("SMART_RECAP_ENABLED") == "true",
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		SmartRecapModel:    os.Getenv("SMART_RECAP_MODEL"),
		LockTimeoutSeconds: 60,
	}

	// Parse quota limit
	if quotaStr := os.Getenv("SMART_RECAP_QUOTA_LIMIT"); quotaStr != "" {
		if quota, err := strconv.Atoi(quotaStr); err == nil && quota > 0 {
			config.SmartRecapQuota = quota
		}
	}

	// Parse staleness minutes
	if stalenessStr := os.Getenv("SMART_RECAP_STALENESS_MINUTES"); stalenessStr != "" {
		if staleness, err := strconv.Atoi(stalenessStr); err == nil && staleness > 0 {
			config.StalenessMinutes = staleness
		}
	}

	// Disable if required config is missing
	if config.AnthropicAPIKey == "" || config.SmartRecapModel == "" || config.SmartRecapQuota == 0 || config.StalenessMinutes == 0 {
		config.SmartRecapEnabled = false
	}

	return config
}
