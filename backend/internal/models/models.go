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
	UserEmail  string     `json:"-"` // Used for request tracing — never serialized to prevent leaking session-to-email mapping
	UserStatus UserStatus `json:"-"` // Used for auth middleware status check — never serialized to prevent leaking account state
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

// APIKey represents an API key for authentication
type APIKey struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	KeyHash    string     `json:"-"` // Used for key verification — never serialized to prevent exposing credential material
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
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

// TIL represents a "Today I Learned" note linked to a session transcript position
type TIL struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	SessionID   string    `json:"session_id"`
	MessageUUID *string   `json:"message_uuid,omitempty"`
	OwnerID     int64     `json:"-"` // Used for ownership authorization checks — never serialized to prevent leaking internal user IDs
	CreatedAt   time.Time `json:"created_at"`
}
