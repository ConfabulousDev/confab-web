package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/email"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

func main() {
	// Load configuration from environment
	config := loadConfig()

	// Initialize database connection
	// Note: Migrations are run separately via CLI before starting the server
	// See: migrate -database "$DATABASE_URL" -path internal/db/migrations up
	database, err := db.Connect(config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Initialize S3/MinIO storage
	store, err := storage.NewS3Storage(config.S3Config)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize email service
	resendService := email.NewResendService(
		config.EmailConfig.APIKey,
		config.EmailConfig.FromAddress,
		config.EmailConfig.FromName,
	)
	emailService := email.NewRateLimitedService(resendService, config.EmailConfig.RateLimitPerHour)
	log.Printf("Email service configured with Resend (rate limit: %d/hour)", config.EmailConfig.RateLimitPerHour)

	// Create API server
	server := api.NewServer(database, store, config.OAuthConfig, emailService)
	router := server.SetupRoutes()

	// HTTP server configuration
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      router,
		ReadTimeout:  config.ReadTimeout,  // Configurable via HTTP_READ_TIMEOUT (default: 30s)
		WriteTimeout: config.WriteTimeout, // Configurable via HTTP_WRITE_TIMEOUT (default: 30s)
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting Confab backend server on port %d", config.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
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
		log.Fatal("GITHUB_CLIENT_ID is required")
	}

	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if githubClientSecret == "" {
		log.Fatal("GITHUB_CLIENT_SECRET is required")
	}

	githubRedirectURL := os.Getenv("GITHUB_REDIRECT_URL")
	if githubRedirectURL == "" {
		log.Fatal("GITHUB_REDIRECT_URL is required")
	}

	// Google OAuth configuration
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		log.Fatal("GOOGLE_CLIENT_ID is required")
	}

	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientSecret == "" {
		log.Fatal("GOOGLE_CLIENT_SECRET is required")
	}

	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if googleRedirectURL == "" {
		log.Fatal("GOOGLE_REDIRECT_URL is required")
	}

	// Validate required security configuration
	csrfSecretKey := os.Getenv("CSRF_SECRET_KEY")
	if csrfSecretKey == "" {
		log.Fatal("CSRF_SECRET_KEY is required (must be at least 32 characters)")
	}
	if len(csrfSecretKey) < 32 {
		log.Fatal("CSRF_SECRET_KEY must be at least 32 characters")
	}

	// Validate required S3/storage configuration
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		log.Fatal("S3_ENDPOINT is required")
	}

	awsAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	if awsAccessKeyID == "" {
		log.Fatal("AWS_ACCESS_KEY_ID is required")
	}

	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if awsSecretAccessKey == "" {
		log.Fatal("AWS_SECRET_ACCESS_KEY is required")
	}

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("BUCKET_NAME is required")
	}

	// Validate required access control configuration
	allowedEmails := os.Getenv("ALLOWED_EMAILS")
	if allowedEmails == "" {
		log.Fatal("ALLOWED_EMAILS is required (comma-separated list of allowed email addresses)")
	}
	allowInvitedAfterTS := os.Getenv("ALLOW_INVITED_EMAILS_AFTER_TS")
	if allowInvitedAfterTS == "" {
		log.Fatal("ALLOW_INVITED_EMAILS_AFTER_TS is required (unix timestamp; use large value like 2000000000 to disable)")
	}

	// Validate required database configuration
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// Validate required frontend configuration
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		log.Fatal("FRONTEND_URL is required")
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		log.Fatal("ALLOWED_ORIGINS is required (comma-separated list of allowed origins)")
	}

	// Validate required email configuration
	resendAPIKey := os.Getenv("RESEND_API_KEY")
	if resendAPIKey == "" {
		log.Fatal("RESEND_API_KEY is required")
	}

	emailFromAddress := os.Getenv("EMAIL_FROM_ADDRESS")
	if emailFromAddress == "" {
		log.Fatal("EMAIL_FROM_ADDRESS is required")
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
