package models

import (
	"database/sql"
	"time"
)

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
	ReadOnly  bool       `json:"read_only"`
	// IsAdmin is the raw users.is_admin column — an internal authz input
	// (unioned with SUPER_ADMIN_EMAILS), never serialized (`json:"-"`, D-K1).
	// Clients learn admin status only via the /me union field and the admin
	// list's explicit flags (5k4v).
	IsAdmin   bool      `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	ReadOnly   bool       `json:"-"` // Used for EnforceReadOnly middleware (CF-483) — never serialized
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	// LastActivityAt drives the sliding idle-timeout gate (60j6). Nullable in the
	// DB (rollout-gap rows fall back to created_at via COALESCE); never serialized.
	LastActivityAt sql.NullTime `json:"-"`
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
