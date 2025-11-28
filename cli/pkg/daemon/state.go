package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// FileState tracks sync progress for a single file
type FileState struct {
	LastSyncedLine int `json:"last_synced_line"`
}

// State represents the daemon's persistent state
type State struct {
	ExternalID     string    `json:"external_id"`
	TranscriptPath string    `json:"transcript_path"`
	CWD            string    `json:"cwd"`
	PID            int       `json:"pid"`
	ParentPID      int       `json:"parent_pid,omitempty"` // Claude Code process ID
	InboxPath      string    `json:"inbox_path"`           // Path to event inbox (JSONL)
	StartedAt      time.Time `json:"started_at"`
}

// NewState creates a new daemon state.
// parentPID is the Claude Code process ID to monitor (0 to disable monitoring).
func NewState(externalID, transcriptPath, cwd string, parentPID int) *State {
	// InboxPath is deterministic but stored in state as source of truth
	inboxPath, _ := GetInboxPath(externalID)

	return &State{
		ExternalID:     externalID,
		TranscriptPath: transcriptPath,
		CWD:            cwd,
		PID:            os.Getpid(),
		ParentPID:      parentPID,
		InboxPath:      inboxPath,
		StartedAt:      time.Now(),
	}
}

// GetStatePath returns the path to the state file for a given external ID
func GetStatePath(externalID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".confab", "sync", externalID+".json"), nil
}

// GetInboxPath returns the path to the event inbox file for a given external ID
func GetInboxPath(externalID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".confab", "sync", externalID+".inbox.jsonl"), nil
}

// GetSyncDir returns the path to the sync state directory
func GetSyncDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".confab", "sync"), nil
}

// LoadState reads the state from disk for a given external ID
// Returns nil if the state file doesn't exist
func LoadState(externalID string) (*State, error) {
	path, err := GetStatePath(externalID)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// Save writes the state to disk
func (s *State) Save() error {
	path, err := GetStatePath(s.ExternalID)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create sync directory: %w", err)
	}

	// Marshal state to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// Delete removes the state file from disk
func (s *State) Delete() error {
	path, err := GetStatePath(s.ExternalID)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}

	return nil
}

// IsDaemonRunning checks if the daemon process is still alive
func (s *State) IsDaemonRunning() bool {
	return isProcessRunning(s.PID)
}

// IsParentRunning checks if the parent Claude Code process is still alive
func (s *State) IsParentRunning() bool {
	return isProcessRunning(s.ParentPID)
}

// isProcessRunning checks if a process with the given PID is still alive
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// ListAllStates returns all active sync states
func ListAllStates() ([]*State, error) {
	syncDir, err := GetSyncDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(syncDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sync directory: %w", err)
	}

	var states []*State
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Extract external ID from filename
		externalID := entry.Name()[:len(entry.Name())-5] // Remove .json

		state, err := LoadState(externalID)
		if err != nil {
			continue // Skip invalid state files
		}
		if state != nil {
			states = append(states, state)
		}
	}

	return states, nil
}
