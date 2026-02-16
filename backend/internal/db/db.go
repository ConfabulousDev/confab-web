package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("confab/db")

// DB wraps a PostgreSQL database connection
type DB struct {
	conn *sql.DB

	// ShareAllSessions makes all sessions visible to all authenticated users
	// as system shares (no database rows needed). For on-prem deployments.
	ShareAllSessions bool
}

// Connect establishes a connection to PostgreSQL
func Connect(dsn string) (*DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	// MaxOpenConns: Limit total connections to avoid overwhelming the database
	conn.SetMaxOpenConns(500)
	// MaxIdleConns: Keep some connections ready for reuse, but not too many
	conn.SetMaxIdleConns(100)
	// ConnMaxLifetime: Recycle connections periodically to avoid stale connections
	conn.SetConnMaxLifetime(20 * time.Minute)

	return &DB{conn: conn}, nil
}

// ConnectWithRetry attempts to connect to the database with exponential backoff.
// It retries until the context is cancelled or a connection is established.
// Backoff schedule: 1s, 2s, 4s, 8s, 16s, 32s, then capped at 60s.
func ConnectWithRetry(ctx context.Context, dsn string) (*DB, error) {
	delay := 1 * time.Second
	const maxDelay = 60 * time.Second

	for {
		database, err := Connect(dsn)
		if err == nil {
			return database, nil
		}

		// Check if context is already done before sleeping
		if ctx.Err() != nil {
			return nil, fmt.Errorf("giving up connecting to database: %w (last error: %v)", ctx.Err(), err)
		}

		logger.Warn("database connection failed, retrying", "error", err, "retry_in", delay)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("giving up connecting to database: %w (last error: %v)", ctx.Err(), err)
		case <-time.After(delay):
		}

		// Exponential backoff with cap
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Exec executes a query without returning rows (for testing/migrations)
func (db *DB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row (for testing)
func (db *DB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}

// Conn returns the underlying *sql.DB connection.
// Used by testutil to run migrations in tests.
func (db *DB) Conn() *sql.DB {
	return db.conn
}
