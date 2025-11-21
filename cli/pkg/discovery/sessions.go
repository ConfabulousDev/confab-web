package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
)

// SessionInfo holds metadata about a discovered session
type SessionInfo struct {
	SessionID      string
	TranscriptPath string
	ProjectPath    string // Relative path from projects dir
	ModTime        time.Time
	SizeBytes      int64
}

// GetProjectsDir returns the path to the Claude projects directory
// (defaults to ~/.claude/projects, can be overridden with CONFAB_CLAUDE_DIR)
func GetProjectsDir() (string, error) {
	// Use the centralized helper from config package
	return config.GetProjectsDir()
}

// ScanAllSessions finds all session transcript files in ~/.claude/projects/
// Returns sessions sorted by modification time (oldest first)
func ScanAllSessions() ([]SessionInfo, error) {
	projectsDir, err := GetProjectsDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get projects directory: %w", err)
	}

	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var sessions []SessionInfo
	var skippedPaths []string

	err = filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Log detailed error for debugging
			logger.Warn("Failed to access path during scan: %s: %v", path, err)

			// Track for user-friendly summary
			skippedPaths = append(skippedPaths, path)

			return nil // Continue walking
		}

		session := parseSessionFromPath(path, d, projectsDir)
		if session != nil {
			sessions = append(sessions, *session)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk projects directory: %w", err)
	}

	// Show user-friendly summary if there were errors
	reportSkippedPaths(skippedPaths, "scan")

	// Sort by mod time (oldest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.Before(sessions[j].ModTime)
	})

	return sessions, nil
}

// FindSessionByID finds a session transcript by full or partial ID
// Returns the full session ID and transcript path
func FindSessionByID(partialID string) (fullID string, transcriptPath string, err error) {
	projectsDir, err := GetProjectsDir()
	if err != nil {
		return "", "", err
	}

	var matches []SessionInfo
	var skippedPaths []string

	filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Log detailed error for debugging
			logger.Warn("Failed to access path during search: %s: %v", path, walkErr)

			// Track for user-friendly summary
			skippedPaths = append(skippedPaths, path)

			return nil // Continue walking
		}

		session := parseSessionFromPath(path, d, projectsDir)
		if session == nil {
			return nil
		}

		// Match full ID or prefix
		if session.SessionID == partialID || strings.HasPrefix(session.SessionID, partialID) {
			matches = append(matches, *session)
		}
		return nil
	})

	// Show user-friendly summary if there were errors
	reportSkippedPaths(skippedPaths, "search")

	if len(matches) == 0 {
		return "", "", fmt.Errorf("session not found: %s", partialID)
	}

	if len(matches) > 1 {
		return "", "", fmt.Errorf("ambiguous session ID '%s' matches %d sessions", partialID, len(matches))
	}

	return matches[0].SessionID, matches[0].TranscriptPath, nil
}

// reportSkippedPaths prints a user-friendly warning about paths that couldn't be accessed
func reportSkippedPaths(skippedPaths []string, operation string) {
	if len(skippedPaths) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "\n⚠ Warning: Could not access %d path(s) during %s:\n", len(skippedPaths), operation)
	for _, p := range skippedPaths {
		fmt.Fprintf(os.Stderr, "  - %s\n", p)
	}
	fmt.Fprintf(os.Stderr, "Check permissions or see logs at ~/.confab/logs/confab.log\n\n")
}

// parseSessionFromPath checks if a path is a valid session transcript and returns SessionInfo
func parseSessionFromPath(path string, d os.DirEntry, projectsDir string) *SessionInfo {
	if d.IsDir() {
		return nil
	}

	if !strings.HasSuffix(path, ".jsonl") {
		return nil
	}

	name := d.Name()

	// Skip agent files
	if strings.HasPrefix(name, "agent-") {
		return nil
	}

	// Session ID is the filename without extension
	sessionID := strings.TrimSuffix(name, ".jsonl")

	// Validate it looks like a UUID (36 chars with hyphens)
	if len(sessionID) != UUIDLength {
		return nil
	}

	info, err := d.Info()
	if err != nil {
		return nil
	}

	// Get project path (parent directory relative to projects/)
	relPath, _ := filepath.Rel(projectsDir, filepath.Dir(path))

	return &SessionInfo{
		SessionID:      sessionID,
		TranscriptPath: path,
		ProjectPath:    relPath,
		ModTime:        info.ModTime(),
		SizeBytes:      info.Size(),
	}
}
