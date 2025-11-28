package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/types"
)

// TrackedFile represents a file being watched for changes
type TrackedFile struct {
	Path           string
	Type           string // "transcript" or "agent"
	LastSyncedLine int
	LastSize       int64
}

// Watcher tracks files and detects new content
type Watcher struct {
	transcriptPath      string
	transcriptDir       string
	files               map[string]*TrackedFile
	knownAgentIDs       map[string]bool
	lastScannedLine     int  // last line scanned for agent IDs
	initialScanComplete bool // whether we've done the initial full scan
}

// NewWatcher creates a new file watcher for a session
func NewWatcher(transcriptPath string) *Watcher {
	return &Watcher{
		transcriptPath: transcriptPath,
		transcriptDir:  filepath.Dir(transcriptPath),
		files:          make(map[string]*TrackedFile),
		knownAgentIDs:  make(map[string]bool),
	}
}

// InitFromState initializes the watcher with existing state from backend
func (w *Watcher) InitFromState(files map[string]FileState) {
	// Add transcript
	transcriptName := filepath.Base(w.transcriptPath)
	transcriptState := files[transcriptName]
	w.files[transcriptName] = &TrackedFile{
		Path:           w.transcriptPath,
		Type:           "transcript",
		LastSyncedLine: transcriptState.LastSyncedLine,
	}

	// Add any agent files from state
	for fileName, state := range files {
		if fileName == transcriptName {
			continue
		}
		// Assume other files are agents
		agentPath := filepath.Join(w.transcriptDir, fileName)
		w.files[fileName] = &TrackedFile{
			Path:           agentPath,
			Type:           "agent",
			LastSyncedLine: state.LastSyncedLine,
		}
	}
}

// GetTrackedFiles returns all currently tracked files
func (w *Watcher) GetTrackedFiles() []*TrackedFile {
	result := make([]*TrackedFile, 0, len(w.files))
	for _, f := range w.files {
		result = append(result, f)
	}
	return result
}

// CheckForNewFiles scans the transcript for new agent file references
// Returns any newly discovered files
func (w *Watcher) CheckForNewFiles() ([]*TrackedFile, error) {
	agentIDs, err := w.scanForAgentIDs()
	if err != nil {
		return nil, err
	}

	// Add newly discovered agent IDs to known set
	for _, agentID := range agentIDs {
		w.knownAgentIDs[agentID] = true
	}

	// Check all known agent IDs for files that now exist
	var newFiles []*TrackedFile
	for agentID := range w.knownAgentIDs {
		agentFileName := fmt.Sprintf("agent-%s.jsonl", agentID)

		// Skip if already being tracked
		if _, tracked := w.files[agentFileName]; tracked {
			continue
		}

		agentPath := filepath.Join(w.transcriptDir, agentFileName)

		// Check if file exists on disk
		if _, err := os.Stat(agentPath); err != nil {
			continue // Agent file doesn't exist yet
		}

		// Add to tracked files
		tracked := &TrackedFile{
			Path:           agentPath,
			Type:           "agent",
			LastSyncedLine: 0,
		}
		w.files[agentFileName] = tracked
		newFiles = append(newFiles, tracked)
	}

	return newFiles, nil
}

// ReadNewLines reads lines from a file starting after lastSyncedLine
// Returns the lines and their starting line number (1-based)
func (w *Watcher) ReadNewLines(filePath string, lastSyncedLine int) ([]string, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := types.NewJSONLScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= lastSyncedLine {
			continue
		}
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to scan file: %w", err)
	}

	if len(lines) == 0 {
		return nil, 0, nil
	}

	firstLine := lastSyncedLine + 1
	return lines, firstLine, nil
}

// UpdateLastSynced updates the last synced line for a file
func (w *Watcher) UpdateLastSynced(fileName string, lastLine int) {
	if f, ok := w.files[fileName]; ok {
		f.LastSyncedLine = lastLine
	}
}

// scanForAgentIDs parses the transcript for agent ID references.
// On first call, scans the entire file. On subsequent calls, only scans new lines.
func (w *Watcher) scanForAgentIDs() ([]string, error) {
	file, err := os.Open(w.transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open transcript: %w", err)
	}
	defer file.Close()

	var agentIDs []string
	seen := make(map[string]bool) // track duplicates within this scan
	lineNum := 0
	startLine := 0
	if w.initialScanComplete {
		startLine = w.lastScannedLine
	}

	scanner := types.NewJSONLScanner(file)
	for scanner.Scan() {
		lineNum++
		if lineNum <= startLine {
			continue // skip already-scanned lines
		}

		var message map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			logger.Debug("Skipping malformed JSON at line %d: %v", lineNum, err)
			continue
		}

		// Check for user messages with tool results
		msgType, ok := message["type"].(string)
		if !ok || msgType != "user" {
			continue
		}

		// Check toolUseResult.agentId at root level
		if toolUseResult, ok := message["toolUseResult"].(map[string]interface{}); ok {
			if agentID, ok := toolUseResult["agentId"].(string); ok {
				if isValidAgentID(agentID) && !seen[agentID] {
					seen[agentID] = true
					agentIDs = append(agentIDs, agentID)
				}
			}
		}

		// Also check in content blocks inside nested message object
		if nestedMessage, ok := message["message"].(map[string]interface{}); ok {
			if content, ok := nestedMessage["content"].([]interface{}); ok {
				for _, block := range content {
					if blockMap, ok := block.(map[string]interface{}); ok {
						if blockMap["type"] == "tool_result" {
							if resultContent, ok := blockMap["content"].(map[string]interface{}); ok {
								if toolUseResult, ok := resultContent["toolUseResult"].(map[string]interface{}); ok {
									if agentID, ok := toolUseResult["agentId"].(string); ok {
										if isValidAgentID(agentID) && !seen[agentID] {
											seen[agentID] = true
											agentIDs = append(agentIDs, agentID)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan transcript: %w", err)
	}

	// Update scan position
	w.lastScannedLine = lineNum
	w.initialScanComplete = true

	return agentIDs, nil
}

// isValidAgentID checks if a string is a valid 8-character hex agent ID
func isValidAgentID(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
