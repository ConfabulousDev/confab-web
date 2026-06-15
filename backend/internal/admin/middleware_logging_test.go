package admin_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// decodeAdminLogLines parses the captured buffer into one map per JSON record.
func decodeAdminLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
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
	return lines
}

// requestWithUser builds a request whose context carries a JSON capture logger
// and the given user ID (as the session middleware would have set it).
func requestWithUser(t *testing.T, method, path string, userID int64) (*http.Request, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	req := httptest.NewRequest(method, path, nil)
	ctx := auth.SetUserIDForTest(req.Context(), userID)
	ctx = logger.WithLogger(ctx, log)
	return req.WithContext(ctx), buf
}

// TestAdminMiddlewareDenialLogsStructuredLine verifies the previously silent 403
// for a non-admin user now emits one WARN line with the decision and the actor's
// identity (reason, user_id, email, client_ip, method, path).
func TestAdminMiddlewareDenialLogsStructuredLine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	user := testutil.CreateTestUser(t, env, "regular@example.com", "Regular User")

	handler := admin.Middleware(env.DB)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run for a denied non-admin")
	}))

	req, buf := requestWithUser(t, "GET", "/admin/users", user.ID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	lines := decodeAdminLogLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 log line, got %d: %v", len(lines), lines)
	}
	line := lines[0]
	if line["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", line["level"])
	}
	if line["reason"] == nil || line["reason"] == "" {
		t.Errorf("log line missing reason field: %v", line)
	}
	if line["email"] != "regular@example.com" {
		t.Errorf("email = %v, want regular@example.com", line["email"])
	}
	// user_id is a JSON number for the actor.
	if _, ok := line["user_id"].(float64); !ok {
		t.Errorf("user_id missing or not numeric: %v", line["user_id"])
	}
	if line["method"] != "GET" {
		t.Errorf("method = %v, want GET", line["method"])
	}
	if line["path"] != "/admin/users" {
		t.Errorf("path = %v, want /admin/users", line["path"])
	}
}

// TestAdminMiddlewareGrantLogsInfoLine verifies a granted admin access emits one
// INFO line carrying the actor identity, so admin actions are auditable.
func TestAdminMiddlewareGrantLogsInfoLine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	user := testutil.CreateTestUser(t, env, "admin@example.com", "Admin User")

	var nextRan bool
	handler := admin.Middleware(env.DB)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextRan = true
		w.WriteHeader(http.StatusOK)
	}))

	req, buf := requestWithUser(t, "POST", "/admin/users/deactivate", user.ID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !nextRan {
		t.Fatal("next handler must run for a granted admin")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	lines := decodeAdminLogLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 log line, got %d: %v", len(lines), lines)
	}
	line := lines[0]
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", line["level"])
	}
	if line["email"] != "admin@example.com" {
		t.Errorf("email = %v, want admin@example.com", line["email"])
	}
	if line["path"] != "/admin/users/deactivate" {
		t.Errorf("path = %v, want /admin/users/deactivate", line["path"])
	}
}
