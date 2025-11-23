package config

import "time"

// Application constants - centralized configuration values used across packages

// === Network Timeouts ===

// HTTP client timeouts for outbound requests
const (
	// DefaultHTTPTimeout is used for quick API calls like validation
	DefaultHTTPTimeout = 30 * time.Second

	// UploadHTTPTimeout is used for potentially large file uploads
	UploadHTTPTimeout = 5 * time.Minute
)

// HTTP server timeouts for inbound requests
const (
	// ServerReadTimeout is the maximum duration for reading the entire request
	ServerReadTimeout = 30 * time.Second

	// ServerWriteTimeout is the maximum duration before timing out writes of the response
	ServerWriteTimeout = 30 * time.Second

	// ServerIdleTimeout is the maximum duration to wait for the next request when keep-alives are enabled
	ServerIdleTimeout = 60 * time.Second
)

// User interaction timeouts
const (
	// OAuthFlowTimeout is the maximum time to wait for user to complete OAuth authentication
	OAuthFlowTimeout = 5 * time.Minute
)

// === File Processing ===

// Buffer and size limits
const (
	// MaxJSONLLineSize is the maximum size for a single JSONL line (10MB)
	// Default bufio.Scanner buffer is 64KB, but transcript lines with
	// thinking blocks and tool results can exceed 1MB
	MaxJSONLLineSize = 10 * 1024 * 1024

	// TranscriptScanBufferSize is the buffer size for scanning transcript files (1MB)
	// Used for extracting git info from transcripts
	TranscriptScanBufferSize = 1024 * 1024
)

// Byte size constants
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)

// === Retry Configuration ===

// Settings update retry parameters
const (
	// MaxSettingsUpdateRetries is the maximum number of retry attempts for atomic settings updates
	MaxSettingsUpdateRetries = 10

	// BaseSettingsRetryDelay is the initial delay before retrying a settings update
	BaseSettingsRetryDelay = 5 * time.Millisecond
)

// === Validation ===

// API key validation
const (
	// MinAPIKeyLength is the minimum acceptable length for API keys
	// Used to catch truncated or corrupted keys
	MinAPIKeyLength = 16
)

// ID format lengths
const (
	// AgentIDLength is the expected length of agent ID hex strings
	AgentIDLength = 8

	// UUIDLength is the expected length of UUID strings (with hyphens)
	UUIDLength = 36
)

// === File Paths ===

// Directory and file names (relative to home or confab directories)
const (
	// ConfabDir is the main confab configuration directory
	ConfabDir = ".confab"

	// LogDirName is the log directory within confab dir
	LogDirName = ".confab/logs"

	// LogFileName is the name of the log file
	LogFileName = "confab.log"

	// ConfigFileName is the main config file name
	ConfigFileName = "config.json"

	// RedactionConfigFileName is the redaction config file name (enabled state)
	RedactionConfigFileName = "redaction.json"

	// RedactionConfigDisabledFileName is the redaction config file name (disabled state)
	RedactionConfigDisabledFileName = "redaction.json.disabled"
)

// Claude Code directories
const (
	// ClaudeStateDir is the Claude Code state directory name
	ClaudeStateDir = ".claude"

	// ClaudeProjectsSubdir is the projects subdirectory within Claude state dir
	ClaudeProjectsSubdir = "projects"

	// ClaudeTodosSubdir is the todos subdirectory within Claude state dir
	ClaudeTodosSubdir = "todos"

	// ClaudeSettingsFile is the settings file name within Claude state dir
	ClaudeSettingsFile = "settings.json"
)

// === Environment Variables ===

// Environment variable names
const (
	// ClaudeStateDirEnv is the environment variable to override the default Claude state directory
	// Defaults to ~/.claude but can be overridden with this env var
	// Useful for testing and non-standard installations
	ClaudeStateDirEnv = "CONFAB_CLAUDE_DIR"
)
