package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logDirName  = ".confab/logs"
	logFileName = "confab.log"
	maxSizeMB   = 1     // 1MB per file
	maxAgeDays  = 14    // Keep 2 weeks
	maxBackups  = 20    // Max old log files (safety limit)
	compressOld = true  // Compress rotated logs
)

// Level represents the log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger manages logging to file and optionally stderr
type Logger struct {
	file       io.WriteCloser
	logger     *log.Logger
	logPath    string
	level      Level
	mu         sync.Mutex
	alsoStderr bool // Also write to stderr
}

var (
	instance *Logger
	once     sync.Once
)

// Init initializes the logger (creates log directory and file)
func Init() error {
	var err error
	once.Do(func() {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			err = fmt.Errorf("failed to get home directory: %w", homeErr)
			return
		}

		logDir := filepath.Join(home, logDirName)
		if mkdirErr := os.MkdirAll(logDir, 0755); mkdirErr != nil {
			err = fmt.Errorf("failed to create log directory: %w", mkdirErr)
			return
		}

		logPath := filepath.Join(logDir, logFileName)

		// Use lumberjack for automatic log rotation
		rotator := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    maxSizeMB,   // megabytes
			MaxAge:     maxAgeDays,  // days
			MaxBackups: maxBackups,  // number of old files
			Compress:   compressOld, // compress old files
			LocalTime:  true,        // use local time for filenames
		}

		instance = &Logger{
			file:       rotator,
			logger:     log.New(rotator, "", 0), // We'll format manually
			logPath:    logPath,
			level:      INFO,
			alsoStderr: false,
		}
	})
	return err
}

// Get returns the logger instance (initializes if needed)
func Get() *Logger {
	if instance == nil {
		if err := Init(); err != nil {
			// Fallback to stderr-only logger
			instance = &Logger{
				logger:     log.New(os.Stderr, "", 0),
				level:      INFO,
				alsoStderr: true,
			}
		}
	}
	return instance
}

// Close closes the log file
func Close() error {
	if instance != nil && instance.file != nil {
		return instance.file.Close()
	}
	return nil
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetAlsoStderr sets whether to also write to stderr
func (l *Logger) SetAlsoStderr(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.alsoStderr = enabled
}

// LogPath returns the path to the log file
func (l *Logger) LogPath() string {
	return l.logPath
}

// log writes a log message at the specified level
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)

	// Write to log file
	if l.logger != nil {
		l.logger.Print(logLine)
	}

	// Also write to stderr if enabled
	if l.alsoStderr {
		fmt.Fprint(os.Stderr, logLine)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Package-level convenience functions
func Debug(format string, args ...interface{}) {
	Get().Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	Get().Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	Get().Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	Get().Error(format, args...)
}
