package sync

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/http"
)

// Client handles communication with the sync API endpoints
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new sync API client
func NewClient(cfg *config.UploadConfig) *Client {
	return &Client{
		httpClient: http.NewClient(cfg, 30*time.Second),
	}
}

// SyncInitRequest is the request body for POST /api/v1/sync/init
type SyncInitRequest struct {
	ExternalID     string          `json:"external_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	GitInfo        json.RawMessage `json:"git_info,omitempty"`
}

// SyncInitResponse is the response for POST /api/v1/sync/init
type SyncInitResponse struct {
	SessionID string                   `json:"session_id"`
	Files     map[string]SyncFileState `json:"files"`
}

// SyncFileState represents the sync state for a single file
type SyncFileState struct {
	LastSyncedLine int `json:"last_synced_line"`
}

// SyncChunkRequest is the request body for POST /api/v1/sync/chunk
type SyncChunkRequest struct {
	SessionID string   `json:"session_id"`
	FileName  string   `json:"file_name"`
	FileType  string   `json:"file_type"`
	FirstLine int      `json:"first_line"`
	Lines     []string `json:"lines"`
}

// SyncChunkResponse is the response for POST /api/v1/sync/chunk
type SyncChunkResponse struct {
	LastSyncedLine int `json:"last_synced_line"`
}

// SyncEventRequest is the request body for POST /api/v1/sync/event
type SyncEventRequest struct {
	SessionID string          `json:"session_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// SyncEventResponse is the response for POST /api/v1/sync/event
type SyncEventResponse struct {
	Success bool `json:"success"`
}

// Init initializes or resumes a sync session
// Returns the session ID and current sync state for all files
func (c *Client) Init(externalID, transcriptPath, cwd string, gitInfo json.RawMessage) (*SyncInitResponse, error) {
	req := SyncInitRequest{
		ExternalID:     externalID,
		TranscriptPath: transcriptPath,
		CWD:            cwd,
		GitInfo:        gitInfo,
	}

	var resp SyncInitResponse
	if err := c.httpClient.Post("/api/v1/sync/init", req, &resp); err != nil {
		return nil, fmt.Errorf("sync init failed: %w", err)
	}

	return &resp, nil
}

// UploadChunk uploads a chunk of lines for a file
// Returns the new last synced line number
func (c *Client) UploadChunk(sessionID, fileName, fileType string, firstLine int, lines []string) (int, error) {
	req := SyncChunkRequest{
		SessionID: sessionID,
		FileName:  fileName,
		FileType:  fileType,
		FirstLine: firstLine,
		Lines:     lines,
	}

	var resp SyncChunkResponse
	if err := c.httpClient.Post("/api/v1/sync/chunk", req, &resp); err != nil {
		return 0, fmt.Errorf("chunk upload failed: %w", err)
	}

	return resp.LastSyncedLine, nil
}

// SendEvent sends a session lifecycle event to the backend
func (c *Client) SendEvent(sessionID, eventType string, timestamp time.Time, payload json.RawMessage) error {
	req := SyncEventRequest{
		SessionID: sessionID,
		EventType: eventType,
		Timestamp: timestamp,
		Payload:   payload,
	}

	var resp SyncEventResponse
	if err := c.httpClient.Post("/api/v1/sync/event", req, &resp); err != nil {
		return fmt.Errorf("send event failed: %w", err)
	}

	return nil
}
