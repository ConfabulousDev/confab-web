package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/storage"
)

// TestEnvironment holds test infrastructure (PostgreSQL + MinIO containers)
type TestEnvironment struct {
	DB                *db.DB
	Storage           *storage.S3Storage
	PostgresContainer *postgres.PostgresContainer
	MinioContainer    *minio.MinioContainer
	Ctx               context.Context
}

// SetupTestEnvironment starts PostgreSQL and MinIO containers for integration testing
// This function should be called once per test or test suite
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()
	ctx := context.Background()

	// Start PostgreSQL container
	t.Log("Starting PostgreSQL container...")
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("confab_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get postgres connection string: %v", err)
	}

	// Connect to database
	database, err := db.Connect(connStr)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Run migrations (using testutil's migrate function with embedded SQL files)
	t.Log("Running database migrations...")
	if err := RunMigrations(database.Conn()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Start MinIO container
	t.Log("Starting MinIO container...")
	minioContainer, err := minio.Run(ctx,
		"minio/minio:latest",
		minio.WithUsername("minioadmin"),
		minio.WithPassword("minioadmin"),
	)
	if err != nil {
		t.Fatalf("Failed to start minio container: %v", err)
	}

	// Get MinIO endpoint
	minioEndpoint, err := minioContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("Failed to get minio endpoint: %v", err)
	}

	// Create S3 storage client with retry (MinIO needs time to initialize)
	t.Log("Initializing S3 storage...")
	var s3Storage *storage.S3Storage
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		s3Storage, err = storage.NewS3Storage(storage.S3Config{
			Endpoint:        minioEndpoint,
			AccessKeyID:     "minioadmin",
			SecretAccessKey: "minioadmin",
			BucketName:      "confab-test",
			UseSSL:          false, // Local testing without SSL
		})
		if err == nil {
			break
		}
		if i == maxRetries-1 {
			t.Fatalf("Failed to create S3 storage after %d retries: %v", maxRetries, err)
		}
		t.Logf("MinIO not ready yet, retrying... (%d/%d)", i+1, maxRetries)
		time.Sleep(500 * time.Millisecond)
	}

	env := &TestEnvironment{
		DB:                database,
		Storage:           s3Storage,
		PostgresContainer: postgresContainer,
		MinioContainer:    minioContainer,
		Ctx:               ctx,
	}

	// Register cleanup to run when test finishes
	t.Cleanup(func() {
		env.Cleanup(t)
	})

	t.Log("Test environment ready!")
	return env
}

// Cleanup stops containers and closes connections
func (e *TestEnvironment) Cleanup(t *testing.T) {
	t.Helper()
	t.Log("Cleaning up test environment...")

	if e.DB != nil {
		if err := e.DB.Close(); err != nil {
			t.Logf("Warning: failed to close database: %v", err)
		}
	}

	if e.PostgresContainer != nil {
		if err := e.PostgresContainer.Terminate(e.Ctx); err != nil {
			t.Logf("Warning: failed to terminate postgres container: %v", err)
		}
	}

	if e.MinioContainer != nil {
		if err := e.MinioContainer.Terminate(e.Ctx); err != nil {
			t.Logf("Warning: failed to terminate minio container: %v", err)
		}
	}

	t.Log("Test environment cleaned up")
}

// CleanDB truncates all tables to provide clean state for each test
// Call this at the beginning of each test function for test isolation
func (e *TestEnvironment) CleanDB(t *testing.T) {
	t.Helper()

	// Truncate tables in reverse dependency order to avoid FK violations
	tables := []string{
		"session_share_invites",
		"session_shares",
		"files",
		"runs",
		"sessions",
		"api_keys",
		"device_codes",
		"web_sessions",
		"users",
	}

	ctx := context.Background()
	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
		if _, err := e.DB.Exec(ctx, query); err != nil {
			t.Fatalf("Failed to truncate table %s: %v", table, err)
		}
	}
}
