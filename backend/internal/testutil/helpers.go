package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	query := `
		INSERT INTO users (email, name, github_id, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, email, name, avatar_url, github_id, created_at, updated_at
	`

	// Generate unique github_id based on email to avoid collisions
	githubID := "test-github-" + email
	avatarURL := "https://github.com/avatar.png"

	var user models.User
	row := env.DB.QueryRow(env.Ctx, query, email, name, githubID, avatarURL)
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.GitHubID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return &user
}

// CreateTestSession creates a session in the database for testing
func CreateTestSession(t *testing.T, env *TestEnvironment, userID int64, sessionID string) {
	t.Helper()

	query := `
		INSERT INTO sessions (session_id, user_id, first_seen)
		VALUES ($1, $2, NOW())
	`

	_, err := env.DB.Exec(env.Ctx, query, sessionID, userID)
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
}

// CreateTestRun creates a run in the database for testing
func CreateTestRun(t *testing.T, env *TestEnvironment, sessionID string, userID int64, reason, cwd, transcriptPath string) int64 {
	t.Helper()

	query := `
		INSERT INTO runs (session_id, user_id, transcript_path, cwd, reason, source, end_timestamp)
		VALUES ($1, $2, $3, $4, $5, 'hook', NOW())
		RETURNING id
	`

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, sessionID, userID, transcriptPath, cwd, reason)
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test run: %v", err)
	}

	return id
}

// CreateTestFile creates a file in the database for testing
func CreateTestFile(t *testing.T, env *TestEnvironment, runID int64, filePath, fileType, s3Key string, sizeBytes int64) int64 {
	t.Helper()

	query := `
		INSERT INTO files (run_id, file_path, file_type, s3_key, size_bytes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, runID, filePath, fileType, s3Key, sizeBytes)
	err := row.Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	return id
}

// CreateTestShare creates a share in the database for testing
func CreateTestShare(t *testing.T, env *TestEnvironment, sessionID string, userID int64, shareToken, visibility string, expiresAt *time.Time, invitedEmails []string) int64 {
	t.Helper()

	// Insert share
	query := `
		INSERT INTO session_shares (session_id, user_id, share_token, visibility, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id
	`

	var id int64
	row := env.DB.QueryRow(env.Ctx, query, sessionID, userID, shareToken, visibility, expiresAt)
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

// BackdateRun updates the created_at timestamp of a run to a specific time
// This is useful for testing time-based features like rate limiting
func BackdateRun(t *testing.T, env *TestEnvironment, runID int64, timestamp time.Time) {
	t.Helper()

	query := `UPDATE runs SET created_at = $1 WHERE id = $2`
	_, err := env.DB.Exec(env.Ctx, query, timestamp, runID)
	if err != nil {
		t.Fatalf("failed to backdate run: %v", err)
	}
}
