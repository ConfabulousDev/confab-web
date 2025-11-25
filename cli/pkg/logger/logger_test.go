package logger

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	// Save current instance and reset for test
	oldInstance := instance
	oldOnce := once
	defer func() {
		instance = oldInstance
		once = oldOnce
	}()

	// Reset singleton
	instance = nil
	once = sync.Once{}

	// Set CONFAB_LOG_DIR to temp dir for test isolation
	tmpDir := t.TempDir()
	t.Setenv(LogDirEnv, tmpDir)

	// Init logger
	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify log directory exists (it's the tmpDir itself now)
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Errorf("Log directory not created: %s", tmpDir)
	}

	// Note: lumberjack creates the file lazily on first write
	// So we need to write something first
	if instance != nil {
		instance.Info("Test message")
	}

	// Verify log file was created after writing
	logFile := filepath.Join(tmpDir, logFileName)
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("Log file not created: %s", logFile)
	}

	// Verify instance is set
	if instance == nil {
		t.Error("Logger instance is nil after Init()")
	}

	// Clean up
	if instance != nil && instance.file != nil {
		instance.file.Close()
	}
}

func TestLogLevels(t *testing.T) {
	// Create temp file for logger
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := &Logger{
		file:       tmpFile,
		logger:     log.New(tmpFile, "", 0),
		level:      INFO,
		alsoStderr: false,
	}

	// Test logging at different levels
	tests := []struct {
		name      string
		logLevel  Level
		setLevel  Level
		logFunc   func(*Logger, string, ...interface{})
		shouldLog bool
	}{
		{"DEBUG below INFO", DEBUG, INFO, (*Logger).Debug, false},
		{"INFO at INFO", INFO, INFO, (*Logger).Info, true},
		{"WARN at INFO", WARN, INFO, (*Logger).Warn, true},
		{"ERROR at INFO", ERROR, INFO, (*Logger).Error, true},
		{"INFO below WARN", INFO, WARN, (*Logger).Info, false},
		{"WARN at WARN", WARN, WARN, (*Logger).Warn, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set logger level
			logger.SetLevel(tt.setLevel)

			// Clear file
			tmpFile.Truncate(0)
			tmpFile.Seek(0, 0)

			// Log message
			tt.logFunc(logger, "test message")

			// Read file content
			tmpFile.Sync()
			content, err := os.ReadFile(tmpFile.Name())
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			logged := len(content) > 0

			if logged != tt.shouldLog {
				t.Errorf("Expected shouldLog=%v, but logged=%v. Content: %s", tt.shouldLog, logged, string(content))
			}
		})
	}
}

func TestLogFormat(t *testing.T) {
	// Create temp file for logger
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := &Logger{
		file:       tmpFile,
		logger:     log.New(tmpFile, "", 0),
		level:      INFO,
		alsoStderr: false,
	}

	// Log a message
	logger.Info("test message with %s", "args")

	// Read log file
	tmpFile.Sync()
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logLine := string(content)

	// Verify format: [timestamp] LEVEL: message
	if !strings.Contains(logLine, "INFO:") {
		t.Errorf("Log line missing level. Got: %s", logLine)
	}

	if !strings.Contains(logLine, "test message with args") {
		t.Errorf("Log line missing message. Got: %s", logLine)
	}

	// Verify timestamp format (YYYY-MM-DD HH:MM:SS)
	if !strings.Contains(logLine, time.Now().Format("2006-01-02")) {
		t.Errorf("Log line missing valid timestamp. Got: %s", logLine)
	}
}

func TestSetAlsoStderr(t *testing.T) {
	// Create temp file for logger
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := &Logger{
		file:       tmpFile,
		logger:     log.New(tmpFile, "", 0),
		level:      INFO,
		alsoStderr: false,
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Log without alsoStderr
	logger.Info("message 1")
	w.Close()

	stderrContent := make([]byte, 1024)
	n, _ := r.Read(stderrContent)
	r.Close()
	os.Stderr = oldStderr

	if n > 0 {
		t.Errorf("Expected no stderr output with alsoStderr=false, got: %s", string(stderrContent[:n]))
	}

	// Now enable alsoStderr
	logger.SetAlsoStderr(true)

	// Capture stderr again
	r2, w2, _ := os.Pipe()
	os.Stderr = w2

	logger.Info("message 2")
	w2.Close()

	stderrContent2 := make([]byte, 1024)
	n2, _ := r2.Read(stderrContent2)
	r2.Close()
	os.Stderr = oldStderr

	if n2 == 0 {
		t.Error("Expected stderr output with alsoStderr=true, got nothing")
	}

	if !bytes.Contains(stderrContent2[:n2], []byte("message 2")) {
		t.Errorf("Stderr output missing expected message. Got: %s", string(stderrContent2[:n2]))
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.want {
				t.Errorf("Level.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConcurrentLogging(t *testing.T) {
	// Create temp file for logger
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := &Logger{
		file:       tmpFile,
		logger:     log.New(tmpFile, "", 0),
		level:      INFO,
		alsoStderr: false,
	}

	// Log from multiple goroutines concurrently
	const numGoroutines = 10
	const messagesPerGoroutine = 5

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info("goroutine %d message %d", id, j)
			}
		}(i)
	}

	wg.Wait()

	// Read log file and count lines
	tmpFile.Sync()
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := bytes.Count(content, []byte("\n"))
	expectedLines := numGoroutines * messagesPerGoroutine

	if lines != expectedLines {
		t.Errorf("Expected %d log lines, got %d", expectedLines, lines)
	}
}

func TestGet(t *testing.T) {
	// Save current instance
	oldInstance := instance
	oldOnce := once
	defer func() {
		instance = oldInstance
		once = oldOnce
	}()

	// Reset singleton
	instance = nil
	once = sync.Once{}

	// Set CONFAB_LOG_DIR to temp dir for test isolation
	tmpDir := t.TempDir()
	t.Setenv(LogDirEnv, tmpDir)

	// Get should initialize if needed
	logger := Get()
	if logger == nil {
		t.Error("Get() returned nil")
	}

	// Calling Get() again should return same instance
	logger2 := Get()
	if logger != logger2 {
		t.Error("Get() returned different instances")
	}

	// Clean up the logger file
	if instance != nil && instance.file != nil {
		instance.file.Close()
	}
}

func TestClose(t *testing.T) {
	// Create temp file for logger
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Save current instance
	oldInstance := instance
	defer func() {
		instance = oldInstance
	}()

	instance = &Logger{
		file:   tmpFile,
		logger: log.New(tmpFile, "", 0),
	}

	// Close should not error
	err = Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Closing nil instance should not error
	instance = nil
	err = Close()
	if err != nil {
		t.Errorf("Close() with nil instance error = %v", err)
	}
}
