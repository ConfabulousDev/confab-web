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

	"github.com/santaclaude2025/confab/backend/internal/api"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/storage"
)

func main() {
	// Load configuration from environment
	config := loadConfig()

	// Initialize database connection
	database, err := db.Connect(config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize S3/MinIO storage
	store, err := storage.NewS3Storage(config.S3Config)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create API server
	server := api.NewServer(database, store, config.OAuthConfig)
	router := server.SetupRoutes()

	// HTTP server configuration
	httpServer := &http.Server{
		Addr:        fmt.Sprintf(":%d", config.Port),
		Handler:     router,
		ReadTimeout: 15 * time.Second,
		// TODO: is 15s enough for uploads?
		WriteTimeout: 15 * time.Second,
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
	Port        int
	DatabaseURL string
	S3Config    storage.S3Config
	OAuthConfig auth.OAuthConfig
}

func loadConfig() Config {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	return Config{
		Port:        port,
		DatabaseURL: getEnvOrDefault("DATABASE_URL", "postgres://confab:confab@localhost:5432/confab?sslmode=disable"),
		S3Config: storage.S3Config{
			Endpoint:        getEnvOrDefault("S3_ENDPOINT", "localhost:9000"),
			AccessKeyID:     getEnvOrDefault("AWS_ACCESS_KEY_ID", getEnvOrDefault("S3_ACCESS_KEY", "minioadmin")),
			SecretAccessKey: getEnvOrDefault("AWS_SECRET_ACCESS_KEY", getEnvOrDefault("S3_SECRET_KEY", "minioadmin")),
			BucketName:      getEnvOrDefault("BUCKET_NAME", getEnvOrDefault("S3_BUCKET", "confab")),
			UseSSL:          os.Getenv("S3_USE_SSL") == "true",
		},
		OAuthConfig: auth.OAuthConfig{
			GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			GitHubRedirectURL:  getEnvOrDefault("GITHUB_REDIRECT_URL", "http://localhost:8080/auth/github/callback"),
		},
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
