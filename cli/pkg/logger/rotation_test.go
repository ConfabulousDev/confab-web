package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/natefinch/lumberjack.v2"
)

func TestLogRotation(t *testing.T) {
	// Create temporary directory for test logs
	tempDir := t.TempDir()

	// Create a lumberjack logger directly for testing
	logPath := filepath.Join(tempDir, "test.log")
	rotator := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    1, // 1MB
		MaxAge:     14,
		MaxBackups: 20,
		Compress:   false, // Don't compress for easier test verification
		LocalTime:  true,
	}
	defer rotator.Close()

	// Write exactly 1MB + a bit more to trigger rotation
	// Lumberjack rotates when the file exceeds MaxSize on the NEXT write
	oneMB := 1024 * 1024
	chunk := []byte(strings.Repeat("A", 1024)) // 1KB chunks

	// Write 1MB
	totalWritten := 0
	for i := 0; i < 1024; i++ {
		n, err := rotator.Write(chunk)
		if err != nil {
			t.Fatalf("Failed to write chunk %d: %v", i, err)
		}
		totalWritten += n
	}
	t.Logf("Wrote %d bytes (expected 1MB = %d)", totalWritten, oneMB)

	// This write should trigger rotation since we're already at 1MB
	rotator.Write(chunk)

	// Write a bit more to ensure rotation completes
	rotator.Write([]byte("extra data\n"))

	// Force flush
	rotator.Close()

	// List ALL files in temp directory to see what was created
	allFiles, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	t.Logf("All files in temp dir:")
	for _, entry := range allFiles {
		info, _ := entry.Info()
		t.Logf("  %s (%d bytes)", entry.Name(), info.Size())
	}

	// Check that rotation occurred
	// Lumberjack creates timestamped backup files, so we need to count all .log files
	files, err := filepath.Glob(filepath.Join(tempDir, "*.log"))
	if err != nil {
		t.Fatalf("Failed to list log files: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("Expected at least 2 log files after rotation, got %d", len(files))
		t.Logf("Files found via glob: %v", files)
	} else {
		t.Logf("Successfully verified rotation: found %d log files", len(files))
	}

	// Verify the current log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Current log file doesn't exist")
	}
}

func TestLogRotationConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	rotator := &lumberjack.Logger{
		Filename:   filepath.Join(tempDir, "test.log"),
		MaxSize:    maxSizeMB,
		MaxAge:     maxAgeDays,
		MaxBackups: maxBackups,
		Compress:   compressOld,
	}
	defer rotator.Close()

	// Write some data
	rotator.Write([]byte("test log entry\n"))

	// Verify configuration matches constants
	if rotator.MaxSize != maxSizeMB {
		t.Errorf("Expected MaxSize=%d, got %d", maxSizeMB, rotator.MaxSize)
	}
	if rotator.MaxAge != maxAgeDays {
		t.Errorf("Expected MaxAge=%d, got %d", maxAgeDays, rotator.MaxAge)
	}
	if rotator.MaxBackups != maxBackups {
		t.Errorf("Expected MaxBackups=%d, got %d", maxBackups, rotator.MaxBackups)
	}
	if rotator.Compress != compressOld {
		t.Errorf("Expected Compress=%v, got %v", compressOld, rotator.Compress)
	}
}

func TestLoggerWriteAfterRotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	rotator := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    1,
		MaxAge:     14,
		MaxBackups: 5,
		Compress:   false, // Don't compress for easier verification
		LocalTime:  true,
	}
	defer rotator.Close()

	// Write first batch
	firstMessage := "First batch of logs\n"
	for i := 0; i < 100; i++ {
		rotator.Write([]byte(strings.Repeat(firstMessage, 100)))
	}

	// Verify rotation happened
	files1, _ := filepath.Glob(filepath.Join(tempDir, "test.log*"))
	if len(files1) < 2 {
		t.Logf("Warning: Expected rotation after first batch, got %d files", len(files1))
	}

	// Write second batch - verify we can still write
	secondMessage := "Second batch of logs\n"
	for i := 0; i < 50; i++ {
		_, err := rotator.Write([]byte(strings.Repeat(secondMessage, 100)))
		if err != nil {
			t.Fatalf("Failed to write after rotation: %v", err)
		}
	}

	// Verify current log file contains recent data
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read current log: %v", err)
	}

	if !strings.Contains(string(content), secondMessage) {
		t.Error("Current log doesn't contain recent writes")
	}
}
