package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestOrgAnalytics_HTTP_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns 401 without session", func(t *testing.T) {
		env.CleanDB(t)

		// Enable org analytics
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		resp, err := client.Get("/api/v1/org/analytics")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestOrgAnalytics_HTTP_InvalidParams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns 400 for invalid start_ts", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/org/analytics?start_ts=notanumber")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 400 when range exceeds 90 days", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		now := time.Now().UTC()
		startTS := now.Add(-100 * 24 * time.Hour).Unix()
		endTS := now.Unix()

		resp, err := client.Get(fmt.Sprintf("/api/v1/org/analytics?start_ts=%d&end_ts=%d", startTS, endTS))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})
}

func TestOrgAnalytics_HTTP_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns org analytics for authenticated user", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/org/analytics")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.OrgAnalyticsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// User should appear in results
		if len(result.Users) != 1 {
			t.Fatalf("Users length = %d, want 1", len(result.Users))
		}
		if result.Users[0].User.Email != "test@example.com" {
			t.Errorf("User email = %s, want test@example.com", result.Users[0].User.Email)
		}
	})
}

func TestOrgAnalytics_HTTP_DisabledRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns 404 when feature is disabled", func(t *testing.T) {
		env.CleanDB(t)
		// Do NOT set ENABLE_ORG_ANALYTICS

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/org/analytics")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Route not registered → 404 or 405
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 404 or 405 when feature disabled", resp.StatusCode)
		}
	})
}
