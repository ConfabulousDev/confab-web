package auth_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestTrySessionAuthInactiveLogsStructuredLine verifies the security-relevant
// case where a valid session cookie resolves to a deactivated account now emits
// one WARN line (reason=user_inactive, user_id, request context) — and that the
// session token itself never appears in the log output.
func TestTrySessionAuthInactiveLogsStructuredLine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	userStore := &dbuser.Store{DB: env.DB}
	user := testutil.CreateTestUser(t, env, "inactive-log@example.com", "Inactive Log User")
	if err := userStore.UpdateUserStatus(t.Context(), user.ID, models.UserStatusInactive); err != nil {
		t.Fatalf("UpdateUserStatus failed: %v", err)
	}

	const sessionID = "test-session-inactive-log"
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().Add(24*time.Hour))

	buf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionID})
	req.RemoteAddr = "198.51.100.4:5555"
	req = req.WithContext(logger.WithLogger(req.Context(), log))

	if result := auth.TrySessionAuth(req, env.DB); result != nil {
		t.Fatal("TrySessionAuth must return nil for an inactive user")
	}

	if strings.Contains(buf.String(), sessionID) {
		t.Errorf("session token leaked into logs: %q", buf.String())
	}

	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("log line is not valid JSON: %q: %v", raw, err)
		}
		lines = append(lines, m)
	}
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 log line, got %d: %v", len(lines), lines)
	}
	line := lines[0]
	if line["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", line["level"])
	}
	if line["reason"] != "user_inactive" {
		t.Errorf("reason = %v, want user_inactive", line["reason"])
	}
	if _, ok := line["user_id"].(float64); !ok {
		t.Errorf("user_id missing or not numeric: %v", line["user_id"])
	}
	if line["method"] != "GET" {
		t.Errorf("method = %v, want GET", line["method"])
	}
	if line["path"] != "/api/protected" {
		t.Errorf("path = %v, want /api/protected", line["path"])
	}
}
