package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/git"
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

	// Detect git information from current working directory
	gitInfo, err := git.DetectGitInfo(hookInput.CWD)
	if err != nil {
		// Log error but don't fail upload if git detection fails
		fmt.Fprintf(os.Stderr, "Warning: Failed to detect git info: %v\n", err)
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
		GitInfo:        gitInfo,
		Files:          fileUploads,
	}

	// Marshal to JSON
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Compress with zstd for faster uploads and lower bandwidth
	// Typical compression: 5MB transcript -> 6.65MB JSON (base64) -> ~0.8MB zstd (88% reduction)
	var compressedPayload bytes.Buffer
	encoder, err := zstd.NewWriter(&compressedPayload, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return fmt.Errorf("failed to create zstd encoder: %w", err)
	}

	_, err = encoder.Write(payload)
	if err != nil {
		encoder.Close()
		return fmt.Errorf("failed to compress payload: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return fmt.Errorf("failed to finalize compression: %w", err)
	}

	// Send HTTP request
	url := cfg.BackendURL + "/api/v1/sessions/save"
	req, err := http.NewRequest("POST", url, bytes.NewReader(compressedPayload.Bytes()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")
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
	GitInfo        *git.GitInfo `json:"git_info,omitempty"`
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
