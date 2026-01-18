package db

import (
	"encoding/json"
	"time"
)

// MaxAPIKeysPerUser is the maximum number of API keys a user can have
const MaxAPIKeysPerUser = 500

// MaxCustomTitleLength is the maximum length of a custom session title
const MaxCustomTitleLength = 255

// SessionListView specifies which sessions to query
type SessionListView string

const (
	SessionListViewOwned        SessionListView = "owned"
	SessionListViewSharedWithMe SessionListView = "shared-with-me"
)

// SessionListItem represents a session in the list view
type SessionListItem struct {
	ID                    string     `json:"id"`                                // UUID primary key for URL routing
	ExternalID            string     `json:"external_id"`                       // External system's session ID (e.g., Claude Code's ID)
	FirstSeen             time.Time  `json:"first_seen"`
	FileCount             int        `json:"file_count"`                        // Number of sync files
	LastSyncTime          *time.Time `json:"last_sync_time,omitempty"`          // Last sync timestamp
	CustomTitle           *string    `json:"custom_title,omitempty"`            // User-set title override
	SuggestedSessionTitle *string    `json:"suggested_session_title,omitempty"` // AI-suggested title from Smart Recap
	Summary               *string    `json:"summary,omitempty"`                 // First summary from transcript
	FirstUserMessage      *string    `json:"first_user_message,omitempty"`      // First user message
	SessionType           string     `json:"session_type"`
	TotalLines            int64      `json:"total_lines"`                       // Sum of last_synced_line across all files
	// TODO: Remove git_repo field and only return git_repo_url, let frontend parse the org/repo
	GitRepo          *string    `json:"git_repo,omitempty"`           // Git repository (e.g., "org/repo") - extracted from git_info JSONB
	GitRepoURL       *string    `json:"git_repo_url,omitempty"`       // Full git repository URL (e.g., "https://github.com/org/repo")
	GitBranch        *string    `json:"git_branch,omitempty"`         // Git branch - extracted from git_info JSONB
	GitHubPRs        []string   `json:"github_prs,omitempty"`         // Linked GitHub PR refs (e.g., ["123", "456"])
	GitHubCommits    []string   `json:"github_commits,omitempty"`     // Linked GitHub commit SHAs (latest first)
	IsOwner          bool       `json:"is_owner"`                     // true if user owns this session
	AccessType       string     `json:"access_type"`                  // "owner" | "private_share" | "public_share" | "system_share"
	SharedByEmail    *string    `json:"shared_by_email,omitempty"`    // email of user who shared (if not owner)
	Hostname         *string    `json:"hostname,omitempty"`           // Client machine hostname (owner-only, null for shared sessions)
	Username         *string    `json:"username,omitempty"`           // OS username (owner-only, null for shared sessions)
}

// SessionDetail represents detailed session information (sync-based model)
type SessionDetail struct {
	ID                    string           `json:"id"`                                // UUID primary key for URL routing
	ExternalID            string           `json:"external_id"`                       // External system's session ID
	CustomTitle           *string          `json:"custom_title,omitempty"`            // User-set title override
	SuggestedSessionTitle *string          `json:"suggested_session_title,omitempty"` // AI-suggested title from Smart Recap
	Summary               *string          `json:"summary,omitempty"`                 // First summary from transcript
	FirstUserMessage      *string          `json:"first_user_message,omitempty"`      // First user message
	FirstSeen             time.Time        `json:"first_seen"`
	CWD              *string          `json:"cwd,omitempty"`                // Working directory
	TranscriptPath   *string          `json:"transcript_path,omitempty"`    // Original transcript path
	GitInfo          interface{}      `json:"git_info,omitempty"`           // Git metadata
	LastSyncAt       *time.Time       `json:"last_sync_at,omitempty"`       // Last sync timestamp
	Files            []SyncFileDetail `json:"files"`                        // Sync files
	Hostname         *string          `json:"hostname,omitempty"`           // Client machine hostname (owner-only)
	Username         *string          `json:"username,omitempty"`           // OS username (owner-only)
	IsOwner          *bool            `json:"is_owner,omitempty"`           // True if viewer is session owner (shared sessions only)
}

// SyncFileDetail represents a synced file
type SyncFileDetail struct {
	FileName       string    `json:"file_name"`
	FileType       string    `json:"file_type"`
	LastSyncedLine int       `json:"last_synced_line"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SessionShare represents a share link
type SessionShare struct {
	ID             int64      `json:"id"`
	SessionID      string     `json:"session_id"`      // UUID references sessions.id
	ExternalID     string     `json:"external_id"`     // External system's session ID (for display)
	IsPublic       bool       `json:"is_public"`       // true if in session_share_public table
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	Recipients     []string   `json:"recipients,omitempty"` // email addresses of recipients
}

// ShareWithSessionInfo includes both share and session details
type ShareWithSessionInfo struct {
	SessionShare
	SessionSummary          *string `json:"session_summary,omitempty"`
	SessionFirstUserMessage *string `json:"session_first_user_message,omitempty"`
}

// DeviceCode represents a pending device authorization
type DeviceCode struct {
	ID           int64      `json:"id"`
	DeviceCode   string     `json:"device_code"`
	UserCode     string     `json:"user_code"`
	KeyName      string     `json:"key_name"`
	UserID       *int64     `json:"user_id,omitempty"`
	ExpiresAt    time.Time  `json:"expires_at"`
	AuthorizedAt *time.Time `json:"authorized_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// SyncFileState represents the sync state for a single file
type SyncFileState struct {
	FileName       string `json:"file_name"`
	FileType       string `json:"file_type"`
	LastSyncedLine int    `json:"last_synced_line"`
	// ChunkCount is an estimate of the number of S3 chunks for this file.
	// nil means unknown (legacy), 0 means no chunks yet.
	// NOTE: This is an estimate and may drift due to races or failed uploads.
	// Do NOT use this to truncate key lists on read - always list actual S3 objects.
	// The read path self-heals this value by comparing against actual S3 chunk count.
	ChunkCount *int `json:"chunk_count"`
}

// SyncSessionParams contains parameters for creating/updating a sync session
type SyncSessionParams struct {
	ExternalID     string
	TranscriptPath string
	CWD            string
	GitInfo        json.RawMessage // Optional: JSONB for git metadata
	Hostname       string          // Optional: client machine hostname
	Username       string          // Optional: OS username of the client
}

// SessionEventParams contains parameters for inserting a session event
type SessionEventParams struct {
	SessionID      string
	EventType      string
	EventTimestamp time.Time
	Payload        json.RawMessage
}

// SessionAccessType represents how a user can access a session
type SessionAccessType string

const (
	SessionAccessNone      SessionAccessType = "none"
	SessionAccessOwner     SessionAccessType = "owner"
	SessionAccessPublic    SessionAccessType = "public"
	SessionAccessSystem    SessionAccessType = "system"
	SessionAccessRecipient SessionAccessType = "recipient"
)

// SessionAccessInfo contains information about how a user can access a session
type SessionAccessInfo struct {
	AccessType  SessionAccessType
	ShareID     *int64 // The share ID that granted access (for updating last_accessed_at)
	AuthMayHelp bool   // True if session has non-public shares that require authentication
}
