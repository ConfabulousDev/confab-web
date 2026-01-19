package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/email"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/honeycombio/otel-config-go/otelconfig"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	// Check for worker mode
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		runWorker()
		return
	}

	// Start pprof debug server if enabled (for memory/CPU profiling)
	// Access via: fly proxy 6060:6060 -a confab-backend
	if os.Getenv("ENABLE_PPROF") == "true" {
		go startPprofServer()
	}

	// Initialize OpenTelemetry (sends traces to Honeycomb)
	// Configured via env vars: OTEL_SERVICE_NAME, OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_HEADERS
	otelShutdown, err := otelconfig.ConfigureOpenTelemetry()
	if err != nil {
		logger.Warn("failed to configure OpenTelemetry", "error", err)
		// Non-fatal: continue without tracing if OTEL env vars not set
	} else {
		defer otelShutdown()
	}

	// Load configuration from environment
	config := loadConfig()

	// Initialize database connection
	// Note: Migrations are run separately via CLI before starting the server
	// See: migrate -database "$DATABASE_URL" -path internal/db/migrations up
	database, err := db.Connect(config.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", "error", err)
	}
	defer database.Close()

	// Initialize S3/MinIO storage
	store, err := storage.NewS3Storage(config.S3Config)
	if err != nil {
		logger.Fatal("failed to initialize storage", "error", err)
	}

	// Initialize email service
	resendService := email.NewResendService(
		config.EmailConfig.APIKey,
		config.EmailConfig.FromAddress,
		config.EmailConfig.FromName,
	)
	emailService := email.NewRateLimitedService(resendService, config.EmailConfig.RateLimitPerHour)
	logger.Info("email service configured", "provider", "resend", "rate_limit_per_hour", config.EmailConfig.RateLimitPerHour)

	// Create API server
	server := api.NewServer(database, store, config.OAuthConfig, emailService)
	router := server.SetupRoutes()

	// Wrap router with OpenTelemetry HTTP instrumentation
	// This automatically traces all incoming HTTP requests
	handler := otelhttp.NewHandler(router, "confabulous-backend-prod")

	// HTTP server configuration
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      handler,
		ReadTimeout:  config.ReadTimeout,  // Configurable via HTTP_READ_TIMEOUT (default: 30s)
		WriteTimeout: config.WriteTimeout, // Configurable via HTTP_WRITE_TIMEOUT (default: 30s)
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting server", "port", config.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", "error", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", "error", err)
	}

	logger.Info("server stopped")
}

type Config struct {
	Port         int
	DatabaseURL  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	S3Config     storage.S3Config
	OAuthConfig  auth.OAuthConfig
	EmailConfig  EmailConfig
}

type EmailConfig struct {
	APIKey           string
	FromAddress      string
	FromName         string
	RateLimitPerHour int
}

func loadConfig() Config {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	// HTTP timeout configuration (defaults to 30s)
	readTimeout := 30 * time.Second
	if rt := os.Getenv("HTTP_READ_TIMEOUT"); rt != "" {
		if parsed, err := time.ParseDuration(rt); err == nil {
			readTimeout = parsed
		}
	}

	writeTimeout := 30 * time.Second
	if wt := os.Getenv("HTTP_WRITE_TIMEOUT"); wt != "" {
		if parsed, err := time.ParseDuration(wt); err == nil {
			writeTimeout = parsed
		}
	}

	// Validate required OAuth configuration
	githubClientID := os.Getenv("GITHUB_CLIENT_ID")
	if githubClientID == "" {
		logger.Fatal("missing required env var", "var", "GITHUB_CLIENT_ID")
	}

	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if githubClientSecret == "" {
		logger.Fatal("missing required env var", "var", "GITHUB_CLIENT_SECRET")
	}

	githubRedirectURL := os.Getenv("GITHUB_REDIRECT_URL")
	if githubRedirectURL == "" {
		logger.Fatal("missing required env var", "var", "GITHUB_REDIRECT_URL")
	}

	// Google OAuth configuration
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		logger.Fatal("missing required env var", "var", "GOOGLE_CLIENT_ID")
	}

	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientSecret == "" {
		logger.Fatal("missing required env var", "var", "GOOGLE_CLIENT_SECRET")
	}

	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if googleRedirectURL == "" {
		logger.Fatal("missing required env var", "var", "GOOGLE_REDIRECT_URL")
	}

	// Validate required security configuration
	csrfSecretKey := os.Getenv("CSRF_SECRET_KEY")
	if csrfSecretKey == "" {
		logger.Fatal("missing required env var", "var", "CSRF_SECRET_KEY", "hint", "must be at least 32 characters")
	}
	if len(csrfSecretKey) < 32 {
		logger.Fatal("invalid env var", "var", "CSRF_SECRET_KEY", "error", "must be at least 32 characters")
	}

	// Validate required S3/storage configuration
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

	// Validate required database configuration
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Fatal("missing required env var", "var", "DATABASE_URL")
	}

	// Validate required frontend configuration
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		logger.Fatal("missing required env var", "var", "FRONTEND_URL")
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		logger.Fatal("missing required env var", "var", "ALLOWED_ORIGINS", "hint", "comma-separated list of allowed origins")
	}

	// Validate required email configuration
	resendAPIKey := os.Getenv("RESEND_API_KEY")
	if resendAPIKey == "" {
		logger.Fatal("missing required env var", "var", "RESEND_API_KEY")
	}

	emailFromAddress := os.Getenv("EMAIL_FROM_ADDRESS")
	if emailFromAddress == "" {
		logger.Fatal("missing required env var", "var", "EMAIL_FROM_ADDRESS")
	}

	emailFromName := os.Getenv("EMAIL_FROM_NAME")
	if emailFromName == "" {
		emailFromName = "Confab"
	}

	emailRateLimitPerHour := 100 // Default: 100 emails per hour per user
	if rateLimit := os.Getenv("EMAIL_RATE_LIMIT_PER_HOUR"); rateLimit != "" {
		fmt.Sscanf(rateLimit, "%d", &emailRateLimitPerHour)
	}

	return Config{
		Port:         port,
		DatabaseURL:  databaseURL,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		S3Config: storage.S3Config{
			Endpoint:        s3Endpoint,
			AccessKeyID:     awsAccessKeyID,
			SecretAccessKey: awsSecretAccessKey,
			BucketName:      bucketName,
			UseSSL:          os.Getenv("S3_USE_SSL") != "false", // Default true
		},
		OAuthConfig: auth.OAuthConfig{
			GitHubClientID:     githubClientID,
			GitHubClientSecret: githubClientSecret,
			GitHubRedirectURL:  githubRedirectURL,
			GoogleClientID:     googleClientID,
			GoogleClientSecret: googleClientSecret,
			GoogleRedirectURL:  googleRedirectURL,
		},
		EmailConfig: EmailConfig{
			APIKey:           resendAPIKey,
			FromAddress:      emailFromAddress,
			FromName:         emailFromName,
			RateLimitPerHour: emailRateLimitPerHour,
		},
	}
}

// startPprofServer starts a pprof debug server on localhost:6060.
// This server is only accessible locally (127.0.0.1) and is intended
// for use with `fly proxy 6060:6060` for remote debugging.
//
// Available endpoints:
//   - /debug/pprof/heap      - heap memory profile
//   - /debug/pprof/goroutine - goroutine stack traces
//   - /debug/pprof/allocs    - allocation profile
//   - /debug/pprof/profile   - CPU profile (30s default)
//   - /debug/pprof/trace     - execution trace
func startPprofServer() {
	mux := http.NewServeMux()

	// Register pprof handlers
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// Register specific profile handlers
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))

	addr := "127.0.0.1:6060"
	logger.Info("pprof debug server starting", "addr", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Warn("pprof server failed", "error", err)
	}
}
