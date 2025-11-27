package daemon

import (
	"fmt"
	"path/filepath"

	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/sync"
)

// Syncer handles syncing files to the backend
type Syncer struct {
	client    *sync.Client
	sessionID string
	watcher   *Watcher
}

// NewSyncer creates a new syncer
func NewSyncer(client *sync.Client, sessionID string, watcher *Watcher) *Syncer {
	return &Syncer{
		client:    client,
		sessionID: sessionID,
		watcher:   watcher,
	}
}

// SyncAll syncs all tracked files
// Returns the number of chunks uploaded and any error
func (s *Syncer) SyncAll() (int, error) {
	totalChunks := 0

	// First, check for any new agent files
	newFiles, err := s.watcher.CheckForNewFiles()
	if err != nil {
		logger.Warn("Failed to check for new agent files: %v", err)
		// Continue anyway - we can still sync known files
	}

	for _, f := range newFiles {
		logger.Info("Discovered new agent file: path=%s", f.Path)
	}

	// Sync all tracked files
	for _, file := range s.watcher.GetTrackedFiles() {
		chunks, err := s.syncFile(file)
		if err != nil {
			logger.Error("Failed to sync file: file=%s error=%v", file.Path, err)
			// Continue with other files
			continue
		}
		totalChunks += chunks
	}

	return totalChunks, nil
}

// syncFile syncs a single file
// Returns the number of chunks uploaded
func (s *Syncer) syncFile(file *TrackedFile) (int, error) {
	fileName := filepath.Base(file.Path)

	// Read new lines since last sync
	lines, firstLine, err := s.watcher.ReadNewLines(file.Path, file.LastSyncedLine)
	if err != nil {
		return 0, fmt.Errorf("failed to read new lines: %w", err)
	}

	if len(lines) == 0 {
		return 0, nil // Nothing new to sync
	}

	// Upload chunk
	lastLine, err := s.client.UploadChunk(s.sessionID, fileName, file.Type, firstLine, lines)
	if err != nil {
		return 0, fmt.Errorf("failed to upload chunk: %w", err)
	}

	// Update watcher state
	s.watcher.UpdateLastSynced(fileName, lastLine)

	logger.Debug("Synced file: file=%s first_line=%d last_line=%d lines=%d",
		fileName, firstLine, lastLine, len(lines))

	return 1, nil
}

// GetSyncStats returns current sync statistics
func (s *Syncer) GetSyncStats() map[string]int {
	stats := make(map[string]int)
	for _, file := range s.watcher.GetTrackedFiles() {
		fileName := filepath.Base(file.Path)
		stats[fileName] = file.LastSyncedLine
	}
	return stats
}
