package types

import (
	"bufio"
	"io"
	"time"
)

// MaxJSONLLineSize is the maximum size for a single JSONL line
// Default bufio.Scanner buffer is 64KB, but transcript lines with
// thinking blocks and tool results can exceed 1MB
const MaxJSONLLineSize = 10 * 1024 * 1024 // 10MB

// NewJSONLScanner creates a bufio.Scanner configured for large JSONL files
// with a 10MB buffer to handle long transcript lines
func NewJSONLScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, MaxJSONLLineSize)
	scanner.Buffer(buf, MaxJSONLLineSize)
	return scanner
}

// HookInput represents the SessionEnd hook data from Claude Code
type HookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	PermissionMode string `json:"permission_mode"`
	HookEventName  string `json:"hook_event_name"`
	Reason         string `json:"reason"`
}

// NewHookInput creates a HookInput for manual session uploads
// (not from stdin hook). Sets the required fields and leaves
// hook-specific fields empty.
func NewHookInput(sessionID, transcriptPath, cwd, reason string) *HookInput {
	return &HookInput{
		SessionID:      sessionID,
		TranscriptPath: transcriptPath,
		CWD:            cwd,
		Reason:         reason,
		// PermissionMode and HookEventName are empty for manual uploads
	}
}

// HookResponse is the JSON response sent back to Claude Code
type HookResponse struct {
	Continue       bool   `json:"continue"`
	StopReason     string `json:"stopReason"`
	SuppressOutput bool   `json:"suppressOutput"`
}

// SessionFile represents a file discovered for a session
type SessionFile struct {
	Path      string
	Type      string // "transcript" | "agent" | "todo"
	SizeBytes int64
}

// Session represents a captured session in the database
type Session struct {
	SessionID      string
	TranscriptPath string
	CWD            string
	Reason         string
	Timestamp      time.Time
	FileCount      int
	TotalSizeBytes int64
}
