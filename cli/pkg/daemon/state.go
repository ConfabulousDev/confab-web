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
	ExternalID     string               `json:"external_id"`
	SessionID      string               `json:"session_id"`
	TranscriptPath string               `json:"transcript_path"`
	CWD            string               `json:"cwd"`
	PID            int                  `json:"pid"`
	StartedAt      time.Time            `json:"started_at"`
	Files          map[string]FileState `json:"files"`
}

// NewState creates a new daemon state
func NewState(externalID, sessionID, transcriptPath, cwd string) *State {
	return &State{
		ExternalID:     externalID,
		SessionID:      sessionID,
		TranscriptPath: transcriptPath,
		CWD:            cwd,
		PID:            os.Getpid(),
		StartedAt:      time.Now(),
		Files:          make(map[string]FileState),
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

// GetFileState returns the sync state for a file, or zero state if not tracked
func (s *State) GetFileState(fileName string) FileState {
	if state, ok := s.Files[fileName]; ok {
		return state
	}
	return FileState{LastSyncedLine: 0}
}

// UpdateFileState updates the sync state for a file
func (s *State) UpdateFileState(fileName string, lastSyncedLine int) {
	s.Files[fileName] = FileState{LastSyncedLine: lastSyncedLine}
}

// IsDaemonRunning checks if the daemon process is still alive
func (s *State) IsDaemonRunning() bool {
	if s.PID <= 0 {
		return false
	}

	process, err := os.FindProcess(s.PID)
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
