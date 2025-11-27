package sync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/git"
	"github.com/santaclaude2025/confab/pkg/redactor"
	"github.com/santaclaude2025/confab/pkg/types"
)

// Uploader handles uploading complete session files using the sync API.
// This provides a unified upload path for both daemon (incremental) and
// manual save/backfill (complete file) workflows.
type Uploader struct {
	client   *Client
	redactor *redactor.Redactor
}

// NewUploader creates a new uploader with the given config
func NewUploader(cfg *config.UploadConfig) (*Uploader, error) {
	client := NewClient(cfg)

	// Initialize redactor if enabled
	var r *redactor.Redactor
	if redactor.IsEnabled() {
		redactCfg, err := redactor.LoadConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load redaction config: %w", err)
		}
		r, err = redactor.NewRedactor(redactCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create redactor: %w", err)
		}
	}

	return &Uploader{
		client:   client,
		redactor: r,
	}, nil
}

// UploadSession uploads all files for a session using the sync API.
// It initializes (or resumes) a sync session, then uploads each file's
// content as chunks starting from where the backend left off.
//
// Returns the internal session ID on success.
func (u *Uploader) UploadSession(externalID, transcriptPath, cwd string, files []types.SessionFile) (string, error) {
	// Extract git info from transcript (source of truth), fall back to detecting from cwd
	var gitInfoJSON json.RawMessage
	if gitInfo, _ := git.ExtractGitInfoFromTranscript(transcriptPath); gitInfo != nil {
		gitInfoJSON, _ = json.Marshal(gitInfo)
	} else if gitInfo, _ := git.DetectGitInfo(cwd); gitInfo != nil {
		// Fallback: detect from directory if transcript doesn't have git info
		gitInfoJSON, _ = json.Marshal(gitInfo)
	}

	// Initialize sync session
	initResp, err := u.client.Init(externalID, transcriptPath, cwd, gitInfoJSON)
	if err != nil {
		return "", fmt.Errorf("failed to init sync session: %w", err)
	}

	// Upload each file
	for _, file := range files {
		fileName := filepath.Base(file.Path)

		// Get last synced line for this file (0 if new)
		lastSynced := 0
		if state, ok := initResp.Files[fileName]; ok {
			lastSynced = state.LastSyncedLine
		}

		// Upload new lines
		if err := u.uploadFile(initResp.SessionID, file.Path, fileName, file.Type, lastSynced); err != nil {
			return "", fmt.Errorf("failed to upload %s: %w", fileName, err)
		}
	}

	return initResp.SessionID, nil
}

// uploadFile reads a file and uploads lines starting from lastSynced+1
func (u *Uploader) uploadFile(sessionID, filePath, fileName, fileType string, lastSynced int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines (transcripts can have big tool results)
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxLineSize)
	scanner.Buffer(buf, maxLineSize)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= lastSynced {
			continue // Skip already synced lines
		}

		line := scanner.Text()

		// Apply redaction if enabled (JSON-aware to preserve structure)
		if u.redactor != nil {
			line = u.redactor.RedactJSONLine(line)
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if len(lines) == 0 {
		return nil // Nothing new to upload
	}

	// Upload as a single chunk (or could break into smaller chunks if needed)
	firstLine := lastSynced + 1
	_, err = u.client.UploadChunk(sessionID, fileName, fileType, firstLine, lines)
	if err != nil {
		return fmt.Errorf("failed to upload chunk: %w", err)
	}

	return nil
}
