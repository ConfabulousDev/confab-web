package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// GET /api/v1/trends - Provider filter (CF-424)
//
// Pins the wire contract for the new ?provider= query parameter, mirroring
// the session-listing endpoint shipped in CF-393. The canonical lowercase
// values are accepted (case-insensitive); the legacy DB form 'Claude Code'
// is rejected on the wire; unknown values 400.
// =============================================================================

func TestHandleGetTrends_ProviderParam(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	cases := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"canonical claude-code accepted", "provider=claude-code", http.StatusOK},
		{"canonical codex accepted", "provider=codex", http.StatusOK},
		{"mixed case normalized and accepted", "provider=Claude-Code", http.StatusOK},
		{"comma-separated multi accepted", "provider=claude-code,codex", http.StatusOK},
		{"upper-case codex normalized and accepted", "provider=CODEX", http.StatusOK},
		{"empty value treated as omitted", "provider=", http.StatusOK},
		{"omitted entirely", "", http.StatusOK},
		{"legacy 'Claude Code' rejected on the wire", "provider=Claude%20Code", http.StatusBadRequest},
		{"unknown provider rejected", "provider=windsurf", http.StatusBadRequest},
		{"partial-valid still rejected if any unknown", "provider=claude-code,windsurf", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env.CleanDB(t)

			user := testutil.CreateTestUser(t, env, "trends-prov-wire@test.com", "Trends Wire User")
			sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

			ts := setupTestServerWithEnv(t, env)
			client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

			path := "/api/v1/trends"
			if tc.query != "" {
				path += "?" + tc.query
			}

			resp, err := client.Get(path)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			testutil.RequireStatus(t, resp, tc.wantStatus)
		})
	}
}
