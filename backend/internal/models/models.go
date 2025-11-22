package models

import "time"

// User represents a confab user (OAuth-based)
type User struct {
	ID             int64     `json:"id"`
	Email          string    `json:"email"`
	Name           *string   `json:"name,omitempty"`
	AvatarURL      *string   `json:"avatar_url,omitempty"`
	GitHubID       *string   `json:"github_id,omitempty"`
	GitHubUsername *string   `json:"github_username,omitempty"`
	GoogleID       *string   `json:"google_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WebSession represents a browser session (for OAuth)
type WebSession struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// APIKey represents an API key for authentication
type APIKey struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	KeyHash   string    `json:"-"` // Never expose the hash
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Session represents a Claude Code session
type Session struct {
	SessionID   string    `json:"session_id"`
	UserID      int64     `json:"user_id"`
	FirstSeen   time.Time `json:"first_seen"`
	Title       *string   `json:"title,omitempty"`
	SessionType string    `json:"session_type"`
}

// Run represents a single execution/resumption of a session
type Run struct {
	ID             int64     `json:"id"`
	SessionID      string    `json:"session_id"`
	UserID         int64     `json:"user_id"`
	TranscriptPath string    `json:"transcript_path"`
	CWD            string    `json:"cwd"`
	Reason         string    `json:"reason"`
	EndTimestamp   time.Time `json:"end_timestamp"`
	S3Uploaded     bool      `json:"s3_uploaded"`
}

// File represents a session file (transcript, agent sidechain, or todos)
type File struct {
	ID           int64      `json:"id"`
	RunID        int64      `json:"run_id"`
	FilePath     string     `json:"file_path"`
	FileType     string     `json:"file_type"` // "transcript", "agent", or "todo"
	SizeBytes    int64      `json:"size_bytes"`
	S3Key        *string    `json:"s3_key,omitempty"`
	S3UploadedAt *time.Time `json:"s3_uploaded_at,omitempty"`
}

// SaveSessionRequest is the API request for saving a session
type SaveSessionRequest struct {
	SessionID      string       `json:"session_id"`
	TranscriptPath string       `json:"transcript_path"`
	CWD            string       `json:"cwd"`
	Reason         string       `json:"reason"`
	GitInfo        interface{}  `json:"git_info,omitempty"`
	Files          []FileUpload `json:"files"`
	Title          string       `json:"title,omitempty"`
	SessionType    string       `json:"session_type,omitempty"`
	LastActivity   time.Time    `json:"last_activity"` // Required field, always provided by CLI
}

// FileUpload represents a file to be uploaded
type FileUpload struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	SizeBytes int64  `json:"size_bytes"`
	Content   []byte `json:"content"` // Base64 encoded in JSON
}

// SaveSessionResponse is the API response
type SaveSessionResponse struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	RunID     int64  `json:"run_id"`
	Message   string `json:"message,omitempty"`
}
