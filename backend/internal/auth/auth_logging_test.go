package auth

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// captureLogs returns a request whose context carries a JSON slog logger that
// writes into buf, so a test can assert on the structured fields a handler logs.
func captureLogs(r *http.Request) (*http.Request, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return r.WithContext(logger.WithLogger(r.Context(), log)), buf
}

// decodeLogLines parses the captured buffer into one map per emitted JSON log
// record. Empty lines are skipped.
func decodeLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
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

// TestRequireSessionRejectionLogsStructuredLine verifies that the previously
// silent 401 at RequireSession's final reject now emits exactly one WARN line
// carrying the decision (reason) plus request context (client_ip, method, path).
// Reachable with a nil DB: no session cookie and an empty demo config means
// TrySessionAuth and AutoImpersonateIfDemo both return nil before any DB access.
func TestRequireSessionRejectionLogsStructuredLine(t *testing.T) {
	handler := RequireSession(nil, &OAuthConfig{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run on a rejected request")
	}))

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req, buf := captureLogs(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	lines := decodeLogLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 log line, got %d: %v", len(lines), lines)
	}
	line := lines[0]

	if line["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", line["level"])
	}
	if _, ok := line["reason"]; !ok {
		t.Errorf("log line missing reason field: %v", line)
	}
	if line["method"] != "GET" {
		t.Errorf("method = %v, want GET", line["method"])
	}
	if line["path"] != "/api/protected" {
		t.Errorf("path = %v, want /api/protected", line["path"])
	}
	if _, ok := line["client_ip"]; !ok {
		t.Errorf("log line missing client_ip field: %v", line)
	}
}

// TestSessionAuthRejectionLogsNoSecrets guards the discipline that auth-decision
// logs never carry the session cookie value. The reject-path log line is keyed
// on request context (reason/method/path/client_ip), never on the cookie, so the
// cookie value must not appear anywhere in the emitted log output.
func TestSessionAuthRejectionLogsNoSecrets(t *testing.T) {
	const secret = "super-secret-session-token-value"

	handler := RequireSession(nil, &OAuthConfig{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run on a rejected request")
	}))

	// A request whose cookie *header* carries the secret but which still rejects
	// at the no-session 401 (nil DB + empty demo config). The middleware logs the
	// decision without ever reading the cookie value into the log.
	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Cookie", SessionCookieName+"-bogus="+secret)
	req, buf := captureLogs(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if strings.Contains(buf.String(), secret) {
		t.Errorf("session token leaked into logs: %q", buf.String())
	}
}
