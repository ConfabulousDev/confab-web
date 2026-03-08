package models

import "time"

// UserStatus represents the status of a user account
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
)

// User represents a confab user (OAuth-based)
type User struct {
	ID        int64      `json:"id"`
	Email     string     `json:"email"`
	Name      *string    `json:"name,omitempty"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	Status    UserStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// AdminUserStats extends User with admin-visible statistics
type AdminUserStats struct {
	User
	SessionCount   int        `json:"session_count"`
	LastAPIKeyUsed *time.Time `json:"last_api_key_used,omitempty"`
	LastLoggedIn   *time.Time `json:"last_logged_in,omitempty"`
}

// OAuthProvider represents supported OAuth providers
type OAuthProvider string

const (
	ProviderGitHub OAuthProvider = "github"
	ProviderGoogle OAuthProvider = "google"
	ProviderOIDC   OAuthProvider = "oidc"
)

// OAuthUserInfo contains user info fetched from an OAuth provider
type OAuthUserInfo struct {
	Provider         OAuthProvider
	ProviderID       string
	ProviderUsername string
	Email            string
	Name             string
	AvatarURL        string
}

// WebSession represents a browser session (for OAuth)
type WebSession struct {
	ID         string     `json:"id"`
	UserID     int64      `json:"user_id"`
	UserEmail  string     `json:"-"` // For tracing, not serialized
	UserStatus UserStatus `json:"-"` // For auth check, not serialized
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

// APIKey represents an API key for authentication
type APIKey struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	KeyHash    string     `json:"-"` // Never expose the hash
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// SaveSessionRequest is the API request for saving a session
type SaveSessionRequest struct {
	ExternalID       string       `json:"external_id"`   // External system's session identifier
	TranscriptPath   string       `json:"transcript_path"`
	CWD              string       `json:"cwd"`
	Reason           string       `json:"reason"`
	GitInfo          interface{}  `json:"git_info,omitempty"`
	Files            []FileUpload `json:"files"`
	Summary          string       `json:"summary,omitempty"`
	FirstUserMessage string       `json:"first_user_message,omitempty"`
	SessionType      string       `json:"session_type,omitempty"`
	LastActivity     time.Time    `json:"last_activity"` // Required field, always provided by CLI
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
	Success    bool   `json:"success"`
	ID         string `json:"id"`          // UUID primary key
	ExternalID string `json:"external_id"` // External system's session identifier (echo back)
	RunID      int64  `json:"run_id"`
	SessionURL string `json:"session_url"`
	Message    string `json:"message,omitempty"`
}

// GitHubLinkType represents the type of GitHub artifact
type GitHubLinkType string

const (
	GitHubLinkTypeCommit      GitHubLinkType = "commit"
	GitHubLinkTypePullRequest GitHubLinkType = "pull_request"
)

// GitHubLinkSource represents how the link was created
type GitHubLinkSource string

const (
	GitHubLinkSourceCLIHook    GitHubLinkSource = "cli_hook"
	GitHubLinkSourceManual     GitHubLinkSource = "manual"
	GitHubLinkSourceTranscript GitHubLinkSource = "transcript"
)

// GitHubLink represents a link between a session and a GitHub artifact
type GitHubLink struct {
	ID        int64            `json:"id"`
	SessionID string           `json:"session_id"`
	LinkType  GitHubLinkType   `json:"link_type"`
	URL       string           `json:"url"`
	Owner     string           `json:"owner"`
	Repo      string           `json:"repo"`
	Ref       string           `json:"ref"`
	Title     *string          `json:"title,omitempty"`
	Source    GitHubLinkSource `json:"source"`
	CreatedAt time.Time        `json:"created_at"`
}

// CreateGitHubLinkRequest is the API request for creating a GitHub link
type CreateGitHubLinkRequest struct {
	LinkType GitHubLinkType   `json:"link_type"`
	URL      string           `json:"url"`
	Title    *string          `json:"title,omitempty"`
	Source   GitHubLinkSource `json:"source"`
}
