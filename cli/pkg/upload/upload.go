package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/git"
	"github.com/santaclaude2025/confab/pkg/redactor"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/santaclaude2025/confab/pkg/utils"
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

	return UploadToCloudWithConfig(cfg, hookInput, files)
}

// UploadToCloudWithConfig uploads session data using the provided config
// Use this when you already have the config (e.g., in backfill to avoid repeated loading)
func UploadToCloudWithConfig(cfg *config.UploadConfig, hookInput *types.HookInput, files []types.SessionFile) error {
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
	fileUploads, err := ReadFilesForUpload(files)
	if err != nil {
		return fmt.Errorf("failed to read files for upload: %w", err)
	}

	// Extract last activity timestamp from transcript
	var lastActivity *time.Time
	for _, f := range files {
		if f.Type == "transcript" {
			ts, err := extractLastActivity(f.Path)
			if err != nil {
				// Log warning but don't fail upload
				fmt.Fprintf(os.Stderr, "Warning: Failed to extract last activity: %v\n", err)
			} else {
				lastActivity = ts
			}
			break
		}
	}

	// Create request payload
	request := SaveSessionRequest{
		SessionID:      hookInput.SessionID,
		TranscriptPath: hookInput.TranscriptPath,
		CWD:            hookInput.CWD,
		Reason:         hookInput.Reason,
		GitInfo:        gitInfo,
		Files:          fileUploads,
		LastActivity:   lastActivity,
	}

	return SendSessionRequest(cfg, &request)
}

// ReadFilesForUpload reads file contents and creates FileUpload entries
func ReadFilesForUpload(files []types.SessionFile) ([]FileUpload, error) {
	fileUploads := make([]FileUpload, 0, len(files))

	// Check if redaction is enabled and load config if so
	var r *redactor.Redactor
	if redactor.IsEnabled() {
		cfg, err := redactor.LoadConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load redaction config: %w", err)
		}

		r, err = redactor.NewRedactor(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create redactor: %w", err)
		}
	}

	for _, f := range files {
		content, err := os.ReadFile(f.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", f.Path, err)
		}

		// Redact content if redaction is enabled
		if r != nil {
			content = r.RedactBytes(content)
		}

		fileUploads = append(fileUploads, FileUpload{
			Path:      f.Path,
			Type:      f.Type,
			SizeBytes: int64(len(content)), // Use actual content size after redaction
			Content:   content,
		})
	}
	return fileUploads, nil
}

// extractLastActivity parses a transcript JSONL file and extracts the most recent timestamp
// from ALL message types. Returns nil if no valid timestamps are found.
func extractLastActivity(transcriptPath string) (*time.Time, error) {
	// Open the transcript file
	file, err := os.Open(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open transcript: %w", err)
	}
	defer file.Close()

	var maxTimestamp *time.Time
	scanner := types.NewJSONLScanner(file)

	// Read line by line (JSONL format)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse JSON
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Log warning but continue parsing other lines
			fmt.Fprintf(os.Stderr, "Warning: Failed to parse transcript line %d: %v\n", lineNum, err)
			continue
		}

		// Extract timestamp field (present in all message types)
		if tsStr, ok := entry["timestamp"].(string); ok && tsStr != "" {
			// Parse RFC3339 timestamp
			ts, err := time.Parse(time.RFC3339Nano, tsStr)
			if err != nil {
				// Try RFC3339 without nano precision
				ts, err = time.Parse(time.RFC3339, tsStr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to parse timestamp on line %d: %v\n", lineNum, err)
					continue
				}
			}

			// Update max timestamp
			if maxTimestamp == nil || ts.After(*maxTimestamp) {
				maxTimestamp = &ts
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading transcript: %w", err)
	}

	return maxTimestamp, nil
}

// SendSessionRequest sends a session save request to the backend with zstd compression
func SendSessionRequest(cfg *config.UploadConfig, request *SaveSessionRequest) error {
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

	client := &http.Client{Timeout: utils.UploadHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
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
	Source         string       `json:"source,omitempty"`
	GitInfo        *git.GitInfo `json:"git_info,omitempty"`
	Files          []FileUpload `json:"files"`
	LastActivity   *time.Time   `json:"last_activity,omitempty"`
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
