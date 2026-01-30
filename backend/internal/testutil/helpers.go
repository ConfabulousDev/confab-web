package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// AuthenticatedRequest creates an HTTP request with user authentication context
func AuthenticatedRequest(t *testing.T, method, url string, body interface{}, userID int64) *http.Request {
	t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyJSON)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req := httptest.NewRequest(method, url, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	// Add user ID to context (simulating auth middleware)
	ctx := context.WithValue(req.Context(), auth.GetUserIDContextKey(), userID)
	return req.WithContext(ctx)
}

// ParseJSONResponse decodes JSON response body into v
func ParseJSONResponse(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()

	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v. Body: %s", err, w.Body.String())
	}
}

// AssertStatus checks HTTP status code matches expected
func AssertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()

	if w.Code != expected {
		t.Errorf("expected status %d, got %d. Body: %s", expected, w.Code, w.Body.String())
	}
}

// CreateTestUser creates a user in the database for testing
func CreateTestUser(t *testing.T, env *TestEnvironment, email, name string) *models.User {
	t.Helper()

	// Create user
	userQuery := `
		INSERT INTO users (email, name, avatar_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
		RETURNING id, email, name, avatar_url, status, created_at, updated_at
	`

	avatarURL := "https://github.com/avatar.png"

	var user models.User
	row := env.DB.QueryRow(env.Ctx, userQuery, email, name, avatarURL)
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a GitHub identity for the user (for test compatibility)
	identityQuery := `
		INSERT INTO user_identities (user_id, provider, provider_id, created_at)
		VALUES ($1, 'github', $2, NOW())
	`
	githubID := "test-github-" + email
	_, err = env.DB.Exec(env.Ctx, identityQuery, user.ID, githubID)
	if err != nil {
		t.Fatalf("failed to create test user identity: %v", err)
	}

	return &user
}

// CreateTestSession creates a session in the database for testing
// Returns the session's UUID primary key (id)
func CreateTestSession(t *testing.T, env *TestEnvironment, userID int64, externalID string) string {
	t.Helper()

	sessionID := uuid.New().String()

	query := `
		INSERT INTO sessions (id, user_id, external_id, first_seen)
		VALUES ($1, $2, $3, NOW())
	`

	_, err := env.DB.Exec(env.Ctx, query, sessionID, userID, externalID)
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	return sessionID
}

// CreateTestSessionWithGitInfo creates a session with git_info in the database for testing
func CreateTestSessionWithGitInfo(t *testing.T, env *TestEnvironment, userID int64, externalID, repoURL string) string {
	t.Helper()

	sessionID := uuid.New().String()

	gitInfo := map[string]interface{}{
		"repo_url": repoURL,
	}
	gitInfoJSON, err := json.Marshal(gitInfo)
	if err != nil {
		t.Fatalf("failed to marshal git_info: %v", err)
	}

	query := `
		INSERT INTO sessions (id, user_id, external_id, first_seen, git_info)
		VALUES ($1, $2, $3, NOW(), $4)
	`

	_, err = env.DB.Exec(env.Ctx, query, sessionID, userID, externalID, gitInfoJSON)
	if err != nil {
		t.Fatalf("failed to create test session with git info: %v", err)
	}

	return sessionID
}

// CreateTestSyncFile creates a sync_file in the database for testing
func CreateTestSyncFile(t *testing.T, env *TestEnvironment, sessionID string, fileName, fileType string, lastSyncedLine int) {
	t.Helper()

	query := `
		INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (session_id, file_name) DO UPDATE SET
			last_synced_line = EXCLUDED.last_synced_line,
			updated_at = NOW()
	`

	_, err := env.DB.Exec(env.Ctx, query, sessionID, fileName, fileType, lastSyncedLine)
	if err != nil {
		t.Fatalf("failed to create test sync file: %v", err)
	}
}

// CreateTestShare creates a share in the database for testing
// sessionID is the UUID primary key of the session
// isPublic: true creates a public share (anyone with link), false creates a recipient-only share
func CreateTestShare(t *testing.T, env *TestEnvironment, sessionID string, isPublic bool, expiresAt *time.Time, recipients []string) int64 {
	t.Helper()

	// Insert share
	query := `
		INSERT INTO session_shares (session_id, expires_at, created_at)
		VALUES ($1, $2, NOW())
		RETURNING id
	`

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, sessionID, expiresAt)
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test share: %v", err)
	}

	// Add public flag if public share
	if isPublic {
		_, err := env.DB.Exec(env.Ctx,
			"INSERT INTO session_share_public (share_id) VALUES ($1)",
			id)
		if err != nil {
			t.Fatalf("failed to add public flag: %v", err)
		}
	}

	// Add recipients if non-public share
	if !isPublic && len(recipients) > 0 {
		for _, email := range recipients {
			// Try to resolve user_id from email
			var userID *int64
			row := env.DB.QueryRow(env.Ctx, "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", email)
			var uid int64
			if err := row.Scan(&uid); err == nil {
				userID = &uid
			}

			_, err := env.DB.Exec(env.Ctx,
				"INSERT INTO session_share_recipients (share_id, email, user_id) VALUES ($1, $2, $3)",
				id, email, userID)
			if err != nil {
				t.Fatalf("failed to add recipient: %v", err)
			}
		}
	}

	return id
}

// CreateTestAPIKey creates an API key in the database for testing
func CreateTestAPIKey(t *testing.T, env *TestEnvironment, userID int64, keyHash, name string) int64 {
	t.Helper()

	query := `
		INSERT INTO api_keys (user_id, key_hash, name, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id
	`

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, userID, keyHash, name)
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test API key: %v", err)
	}

	return id
}


// CreateTestDeviceCode creates a device code in the database for testing
// Note: expiresAt should be in UTC for consistent behavior with PostgreSQL NOW()
func CreateTestDeviceCode(t *testing.T, env *TestEnvironment, deviceCode, userCode, keyName string, expiresAt time.Time) int64 {
	t.Helper()

	// Ensure time is in UTC for consistency with PostgreSQL
	expiresAtUTC := expiresAt.UTC()

	query := `
		INSERT INTO device_codes (device_code, user_code, key_name, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id
	`

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, deviceCode, userCode, keyName, expiresAtUTC)
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test device code: %v", err)
	}

	return id
}

// AuthorizeTestDeviceCode marks a device code as authorized by a user
func AuthorizeTestDeviceCode(t *testing.T, env *TestEnvironment, userCode string, userID int64) {
	t.Helper()

	query := `UPDATE device_codes SET user_id = $1, authorized_at = NOW() WHERE user_code = $2`
	_, err := env.DB.Exec(env.Ctx, query, userID, userCode)
	if err != nil {
		t.Fatalf("failed to authorize test device code: %v", err)
	}
}

// CreateTestWebSession creates a web session in the database for testing
func CreateTestWebSession(t *testing.T, env *TestEnvironment, sessionID string, userID int64, expiresAt time.Time) {
	t.Helper()

	query := `
		INSERT INTO web_sessions (id, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, NOW())
	`

	_, err := env.DB.Exec(env.Ctx, query, sessionID, userID, expiresAt)
	if err != nil {
		t.Fatalf("failed to create test web session: %v", err)
	}
}

// CreateTestSystemShare creates a system share in the database for testing
// System shares are accessible to all authenticated users
func CreateTestSystemShare(t *testing.T, env *TestEnvironment, sessionID string, expiresAt *time.Time) int64 {
	t.Helper()

	// Insert share
	query := `
		INSERT INTO session_shares (session_id, expires_at, created_at)
		VALUES ($1, $2, NOW())
		RETURNING id
	`

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, sessionID, expiresAt)
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test share: %v", err)
	}

	// Add system share flag
	_, err = env.DB.Exec(env.Ctx,
		"INSERT INTO session_share_system (share_id) VALUES ($1)",
		id)
	if err != nil {
		t.Fatalf("failed to add system share flag: %v", err)
	}

	return id
}

// CreateTestGitHubLink creates a GitHub link in the database for testing
func CreateTestGitHubLink(t *testing.T, env *TestEnvironment, sessionID, linkType, ref string) int64 {
	t.Helper()

	query := `
		INSERT INTO session_github_links (session_id, link_type, url, owner, repo, ref, source, created_at)
		VALUES ($1, $2, $3, 'test-owner', 'test-repo', $4, 'manual', NOW())
		RETURNING id
	`

	url := "https://github.com/test-owner/test-repo/"
	if linkType == "pull_request" {
		url += "pull/" + ref
	} else {
		url += "commit/" + ref
	}

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, sessionID, linkType, url, ref)
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test github link: %v", err)
	}

	return id
}

// UploadTestChunk uploads a JSONL chunk to S3 storage for testing
func UploadTestChunk(t *testing.T, env *TestEnvironment, userID int64, externalID, fileName string, firstLine, lastLine int, data []byte) {
	t.Helper()

	_, err := env.Storage.UploadChunk(env.Ctx, userID, externalID, fileName, firstLine, lastLine, data)
	if err != nil {
		t.Fatalf("failed to upload test chunk: %v", err)
	}
}

// UploadTestTranscript uploads transcript bytes to S3 as a single chunk
func UploadTestTranscript(t *testing.T, env *TestEnvironment, userID int64, externalID, fileName string, data []byte) {
	t.Helper()

	// Count lines in data
	lineCount := 1
	for _, b := range data {
		if b == '\n' {
			lineCount++
		}
	}
	// Adjust for trailing newline
	if len(data) > 0 && data[len(data)-1] == '\n' {
		lineCount--
	}

	_, err := env.Storage.UploadChunk(env.Ctx, userID, externalID, fileName, 1, lineCount, data)
	if err != nil {
		t.Fatalf("failed to upload test transcript: %v", err)
	}
}

// MinimalTranscript returns a minimal valid JSONL transcript for testing
func MinimalTranscript() []byte {
	// Minimal transcript with 3 lines: init, user message, assistant response
	return []byte(`{"type":"init","timestamp":"2024-01-01T00:00:00Z","session_id":"test","model":"claude-sonnet-4-20250514"}
{"type":"human","timestamp":"2024-01-01T00:00:01Z","message":{"role":"user","content":"Hello"}}
{"type":"assistant","timestamp":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"Hi there!"},"usage":{"input_tokens":10,"output_tokens":5}}
`)
}

// TestGitHubUser creates an OAuthUserInfo for a GitHub user for testing
func TestGitHubUser(suffix string) models.OAuthUserInfo {
	return models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "github-" + suffix,
		Email:      suffix + "@github-test.com",
		Name:       "Test User " + suffix,
		AvatarURL:  "https://github.com/avatar/" + suffix + ".png",
	}
}

// TestGoogleUser creates an OAuthUserInfo for a Google user for testing
func TestGoogleUser(suffix string) models.OAuthUserInfo {
	return models.OAuthUserInfo{
		Provider:   models.ProviderGoogle,
		ProviderID: "google-" + suffix,
		Email:      suffix + "@google-test.com",
		Name:       "Test User " + suffix,
		AvatarURL:  "https://google.com/avatar/" + suffix + ".png",
	}
}
