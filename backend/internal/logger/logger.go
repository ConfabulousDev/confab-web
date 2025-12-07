package logger

import (
	"log/slog"
	"os"
	"strings"
)

var log *slog.Logger
var logLevel slog.Level

func init() {
	// Parse log level from environment variable
	// Supports: debug, info, warn, error (case-insensitive)
	logLevel = slog.LevelInfo // Default
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		switch strings.ToLower(levelStr) {
		case "debug":
			logLevel = slog.LevelDebug
		case "info":
			logLevel = slog.LevelInfo
		case "warn", "warning":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		}
	}

	// Create JSON handler for production-ready structured logging
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	log = slog.New(handler)
}

// IsDebug returns true if debug logging is enabled
func IsDebug() bool {
	return logLevel == slog.LevelDebug
}

// SetDebugForTest enables or disables debug mode for testing purposes.
// Returns a cleanup function that restores the original state.
// This should only be used in tests.
func SetDebugForTest(enabled bool) func() {
	original := logLevel
	if enabled {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelInfo
	}
	return func() {
		logLevel = original
	}
}

// Info logs an informational message with structured fields
func Info(msg string, args ...any) {
	log.Info(msg, args...)
}

// Error logs an error message with structured fields
func Error(msg string, args ...any) {
	log.Error(msg, args...)
}

// Debug logs a debug message with structured fields
func Debug(msg string, args ...any) {
	log.Debug(msg, args...)
}

// Warn logs a warning message with structured fields
func Warn(msg string, args ...any) {
	log.Warn(msg, args...)
}
