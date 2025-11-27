package testutil

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/models"
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

// AssertErrorResponse checks error response format and message
func AssertErrorResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, expectedMessage string) {
	t.Helper()

	AssertStatus(t, w, expectedStatus)

	var resp map[string]string
	ParseJSONResponse(t, w, &resp)

	if resp["error"] != expectedMessage {
		t.Errorf("expected error message %q, got %q", expectedMessage, resp["error"])
	}
}

// CreateTestUser creates a user in the database for testing
func CreateTestUser(t *testing.T, env *TestEnvironment, email, name string) *models.User {
	t.Helper()

	// Create user
	userQuery := `
		INSERT INTO users (email, name, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, email, name, avatar_url, created_at, updated_at
	`

	avatarURL := "https://github.com/avatar.png"

	var user models.User
	row := env.DB.QueryRow(env.Ctx, userQuery, email, name, avatarURL)
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
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
func CreateTestShare(t *testing.T, env *TestEnvironment, sessionID string, shareToken, visibility string, expiresAt *time.Time, invitedEmails []string) int64 {
	t.Helper()

	// Insert share
	query := `
		INSERT INTO session_shares (session_id, share_token, visibility, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id
	`

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, sessionID, shareToken, visibility, expiresAt)
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test share: %v", err)
	}

	// Add invited emails if private
	if visibility == "private" && len(invitedEmails) > 0 {
		for _, email := range invitedEmails {
			_, err := env.DB.Exec(env.Ctx,
				"INSERT INTO session_share_invites (share_id, email) VALUES ($1, $2)",
				id, email)
			if err != nil {
				t.Fatalf("failed to add invited email: %v", err)
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

// GenerateShareToken generates a random share token for testing (32 hex chars)
func GenerateShareToken() string {
	bytes := make([]byte, 16) // 16 bytes = 32 hex chars
	if _, err := rand.Read(bytes); err != nil {
		panic(err) // In tests, panic is acceptable for crypto failures
	}
	return hex.EncodeToString(bytes)
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
