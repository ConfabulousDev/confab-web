package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/types"
)

// UploadToCloud uploads session data to the backend
func UploadToCloud(hookInput *types.HookInput, files []types.SessionFile) error {
	// Get upload configuration
	cfg, err := config.GetUploadConfig()
	if err != nil {
		return fmt.Errorf("failed to get upload config: %w", err)
	}

	// Skip if API key not configured
	if cfg.APIKey == "" {
		return nil
	}

	// Validate backend URL
	if cfg.BackendURL == "" {
		return fmt.Errorf("backend URL not configured")
	}

	// Read file contents
	fileUploads := make([]FileUpload, 0, len(files))
	for _, f := range files {
		content, err := os.ReadFile(f.Path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", f.Path, err)
		}

		fileUploads = append(fileUploads, FileUpload{
			Path:      f.Path,
			Type:      f.Type,
			SizeBytes: f.SizeBytes,
			Content:   content,
		})
	}

	// Create request payload
	request := SaveSessionRequest{
		SessionID:      hookInput.SessionID,
		TranscriptPath: hookInput.TranscriptPath,
		CWD:            hookInput.CWD,
		Reason:         hookInput.Reason,
		Files:          fileUploads,
	}

	// Marshal to JSON
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// TODO: Add gzip compression for large uploads
	// Currently sends uncompressed JSON with base64-encoded content
	// Example 5MB transcript: ~6.65MB over wire (base64 overhead)
	// With gzip: could be ~0.5-1MB (80-90% reduction)
	// Add: Content-Encoding: gzip header + compress payload

	// Send HTTP request
	url := cfg.BackendURL + "/api/v1/sessions/save"
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(resp.Body)

	// Check status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response SaveSessionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("upload failed: %s", response.Message)
	}

	return nil
}

// SaveSessionRequest is the API request for saving a session
type SaveSessionRequest struct {
	SessionID      string       `json:"session_id"`
	TranscriptPath string       `json:"transcript_path"`
	CWD            string       `json:"cwd"`
	Reason         string       `json:"reason"`
	Files          []FileUpload `json:"files"`
}

// FileUpload represents a file to be uploaded
type FileUpload struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	SizeBytes int64  `json:"size_bytes"`
	Content   []byte `json:"content"`
}

// SaveSessionResponse is the API response
type SaveSessionResponse struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	RunID     int64  `json:"run_id"`
	Message   string `json:"message,omitempty"`
}
