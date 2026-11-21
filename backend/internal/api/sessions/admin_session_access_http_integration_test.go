package sessions_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// xtrf: admins can view and list any session (read-only, redacted)
//
// Covers both admin sources: an env SUPER_ADMIN_EMAILS user and a
// users.is_admin user. Every read endpoint routes through the canonical
// access check, so one admin path unlocks detail, sync/file, analytics and
// github-links; writes keep their own owner checks.
// =============================================================================

const envAdminEmail = "envadmin@example.com"

type adminFixture struct {
	owner      *models.User
	envAdmin   *models.User
	dbAdmin    *models.User
	plainUser  *models.User
	sessionID  string
	externalID string
}

func setupAdminFixture(t *testing.T, env *testutil.TestEnvironment) adminFixture {
	t.Helper()
	env.CleanDB(t)
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", envAdminEmail)

	f := adminFixture{externalID: "owner-session"}
	f.owner = testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	f.envAdmin = testutil.CreateTestUser(t, env, envAdminEmail, "Env Admin")
	f.dbAdmin = testutil.CreateTestUser(t, env, "dbadmin@example.com", "DB Admin")
	f.plainUser = testutil.CreateTestUser(t, env, "plain@example.com", "Plain")
	if _, err := env.DB.Exec(env.Ctx, `UPDATE users SET is_admin = true WHERE id = $1`, f.dbAdmin.ID); err != nil {
		t.Fatalf("failed to grant is_admin: %v", err)
	}
	f.sessionID = testutil.CreateTestSessionFull(t, env, f.owner.ID, f.externalID, testutil.TestSessionFullOpts{
		Summary: "Owner session",
	})
	testutil.UploadTestTranscript(t, env, f.owner.ID, "claude-code", f.externalID, "transcript.jsonl", testutil.MinimalTranscript())
	return f
}

func TestAdminCanReadAnotherUsersSession_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)

	for _, source := range []string{"env SUPER_ADMIN_EMAILS", "users.is_admin"} {
		t.Run(source, func(t *testing.T) {
			f := setupAdminFixture(t, env)
			adminUser := f.envAdmin
			if source == "users.is_admin" {
				adminUser = f.dbAdmin
			}
			ts := setupTestServerWithEnv(t, env)
			client := testutil.NewTestClient(t, ts).WithSession(testutil.CreateTestWebSessionWithToken(t, env, adminUser.ID))

			// GET /sessions/{id}: wire JSON carries access_type admin, is_owner false.
			resp, err := client.Get("/api/v1/sessions/" + f.sessionID)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			testutil.RequireStatus(t, resp, http.StatusOK)
			var wire map[string]any
			testutil.ParseJSON(t, resp, &wire)
			resp.Body.Close()
			if wire["access_type"] != "admin" {
				t.Errorf("access_type = %v, want admin", wire["access_type"])
			}
			if wire["is_owner"] != false {
				t.Errorf("is_owner = %v, want false", wire["is_owner"])
			}
			if wire["owner_email"] != f.owner.Email {
				t.Errorf("owner_email = %v, want %s", wire["owner_email"], f.owner.Email)
			}
			for _, k := range []string{"cwd", "hostname", "username", "transcript_path"} {
				if _, has := wire[k]; has {
					t.Errorf("admin view must redact %s, got %v", k, wire[k])
				}
			}

			for _, path := range []string{
				"/api/v1/sessions/" + f.sessionID + "/sync/file?file_name=transcript.jsonl",
				"/api/v1/sessions/" + f.sessionID + "/analytics",
				"/api/v1/sessions/" + f.sessionID + "/github-links",
			} {
				resp, err := client.Get(path)
				if err != nil {
					t.Fatalf("GET %s failed: %v", path, err)
				}
				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("GET %s as admin: status %d, want 200: %s", path, resp.StatusCode, body)
				}
				resp.Body.Close()
			}
		})
	}
}

func TestNonAdminStillGets404OnAnotherUsersSession_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)
	f := setupAdminFixture(t, env)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(testutil.CreateTestWebSessionWithToken(t, env, f.plainUser.ID))

	for _, path := range []string{
		"/api/v1/sessions/" + f.sessionID,
		"/api/v1/sessions/" + f.sessionID + "/sync/file?file_name=transcript.jsonl",
		"/api/v1/sessions/" + f.sessionID + "/github-links",
	} {
		resp, err := client.Get(path)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s as non-admin: status %d, want 404", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestAdminCanReadInactiveOwnersSession_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)
	f := setupAdminFixture(t, env)
	if _, err := env.DB.Exec(env.Ctx, `UPDATE users SET status = 'inactive' WHERE id = $1`, f.owner.ID); err != nil {
		t.Fatalf("failed to deactivate owner: %v", err)
	}
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(testutil.CreateTestWebSessionWithToken(t, env, f.dbAdmin.ID))

	resp, err := client.Get("/api/v1/sessions/" + f.sessionID)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)
}

func TestAdminCannotWriteAnotherUsersSession_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)
	f := setupAdminFixture(t, env)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(testutil.CreateTestWebSessionWithToken(t, env, f.envAdmin.ID))

	resp, err := client.Patch("/api/v1/sessions/"+f.sessionID+"/title", map[string]any{"custom_title": "hijacked"})
	if err != nil {
		t.Fatalf("PATCH failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("admin PATCH title on another user's session: status %d, want 403/404", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = client.Delete("/api/v1/sessions/" + f.sessionID)
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("admin DELETE on another user's session: status %d, want 403/404", resp.StatusCode)
	}
	resp.Body.Close()

	var count int
	var title *string
	if err := env.DB.QueryRow(env.Ctx, `SELECT COUNT(*), MAX(custom_title) FROM sessions WHERE id = $1`, f.sessionID).Scan(&count, &title); err != nil {
		t.Fatalf("failed to read session: %v", err)
	}
	if count != 1 {
		t.Errorf("session must still exist after admin DELETE, count = %d", count)
	}
	if title != nil {
		t.Errorf("custom_title must be unchanged after admin PATCH, got %q", *title)
	}
}

func TestListSessionsScopeAll_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)

	listAs := func(t *testing.T, f adminFixture, user *models.User, query string) (int, *db.SessionListResult) {
		t.Helper()
		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(testutil.CreateTestWebSessionWithToken(t, env, user.ID))
		resp, err := client.Get("/api/v1/sessions" + query)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return resp.StatusCode, nil
		}
		var result db.SessionListResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp.StatusCode, &result
	}

	t.Run("admin scope=all returns other users' sessions and filter options", func(t *testing.T) {
		f := setupAdminFixture(t, env)
		testutil.CreateTestSessionFull(t, env, f.dbAdmin.ID, "admin-own", testutil.TestSessionFullOpts{Summary: "mine"})

		status, result := listAs(t, f, f.dbAdmin, "?scope=all")
		if status != http.StatusOK {
			t.Fatalf("admin scope=all: status %d, want 200", status)
		}
		var sawOwner, sawOwn bool
		for _, s := range result.Sessions {
			switch s.ExternalID {
			case f.externalID:
				sawOwner = true
				if s.IsOwner {
					t.Error("other user's session must have is_owner=false")
				}
				if s.OwnerEmail != f.owner.Email {
					t.Errorf("owner_email = %q, want %q", s.OwnerEmail, f.owner.Email)
				}
			case "admin-own":
				sawOwn = true
				if s.AccessType != "owner" {
					t.Errorf("admin's own session access_type = %q, want owner", s.AccessType)
				}
			}
		}
		if !sawOwner || !sawOwn {
			t.Errorf("scope=all must include other users' and own sessions (other=%v own=%v)", sawOwner, sawOwn)
		}
		var ownerInOptions bool
		for _, o := range result.FilterOptions.Owners {
			if o == f.owner.Email {
				ownerInOptions = true
			}
		}
		if !ownerInOptions {
			t.Errorf("scope=all filter options must include other owners, got %v", result.FilterOptions.Owners)
		}

		// Env admin too.
		status, _ = listAs(t, f, f.envAdmin, "?scope=all")
		if status != http.StatusOK {
			t.Errorf("env admin scope=all: status %d, want 200", status)
		}
	})

	t.Run("default scope is unchanged for admins", func(t *testing.T) {
		f := setupAdminFixture(t, env)
		_, result := listAs(t, f, f.dbAdmin, "")
		for _, s := range result.Sessions {
			if s.ExternalID == f.externalID {
				t.Error("default scope must not include other users' unshared sessions")
			}
		}
		for _, o := range result.FilterOptions.Owners {
			if o == f.owner.Email {
				t.Error("default scope filter options must not include other owners")
			}
		}
	})

	t.Run("non-admin scope=all is 403", func(t *testing.T) {
		f := setupAdminFixture(t, env)
		status, _ := listAs(t, f, f.plainUser, "?scope=all")
		if status != http.StatusForbidden {
			t.Errorf("non-admin scope=all: status %d, want 403", status)
		}
	})

	t.Run("unknown scope is 400", func(t *testing.T) {
		f := setupAdminFixture(t, env)
		status, _ := listAs(t, f, f.dbAdmin, "?scope=everything")
		if status != http.StatusBadRequest {
			t.Errorf("scope=everything: status %d, want 400", status)
		}
	})
}
