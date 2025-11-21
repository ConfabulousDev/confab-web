package logger

import (
	"log/slog"
	"os"
)

var log *slog.Logger

func init() {
	// Create JSON handler for production-ready structured logging
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, // Default to Info level
	})
	log = slog.New(handler)
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

// SetLevel sets the minimum log level
func SetLevel(level slog.Level) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	log = slog.New(handler)
}
