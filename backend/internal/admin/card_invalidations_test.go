package admin_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// seedSessionWithTokens creates a session with a single session_card_tokens row.
// lastMsg is stored in both sessions.last_message_at and first_seen for simplicity.
func seedSessionWithTokens(t *testing.T, env *testutil.TestEnvironment, userID int64, lastMsg time.Time) string {
	t.Helper()
	sid := uuid.NewString()
	_, err := env.DB.Exec(env.Ctx, `
		INSERT INTO sessions (id, user_id, external_id, first_seen, last_message_at)
		VALUES ($1, $2, $3, $4, $5)
	`, sid, userID, "ext-"+sid[:8], lastMsg, lastMsg)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	_, err = env.DB.Exec(env.Ctx, `
		INSERT INTO session_card_tokens (
			session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, estimated_cost_usd
		) VALUES ($1, 1, NOW(), 100, 0, 0, 0, 0, '0.00')
	`, sid)
	if err != nil {
		t.Fatalf("insert tokens card: %v", err)
	}
	return sid
}

func TestInvalidateCards_AuthEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("unauthenticated gets 401", func(t *testing.T) {
		env.CleanDB(t)
		ts := setupTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Post("/api/v1/admin/cards/invalidate", admin.InvalidateCardsRequest{
			StartDate: "2026-04-01T00:00:00Z",
			CardTypes: []string{"session_card_tokens"},
			Reason:    "test",
		})
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("non-admin gets 403", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "user@example.com", "User")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, user.ID)

		resp, err := client.Post("/api/v1/admin/cards/invalidate", admin.InvalidateCardsRequest{
			StartDate: "2026-04-01T00:00:00Z",
			CardTypes: []string{"session_card_tokens"},
			Reason:    "test",
		})
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
	})
}

func TestInvalidateCards_ValidationErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	cases := []struct {
		name string
		body admin.InvalidateCardsRequest
	}{
		{
			name: "missing start_date",
			body: admin.InvalidateCardsRequest{
				CardTypes: []string{"session_card_tokens"},
				Reason:    "r",
			},
		},
		{
			name: "empty card_types",
			body: admin.InvalidateCardsRequest{
				StartDate: "2026-04-01T00:00:00Z",
				CardTypes: []string{},
				Reason:    "r",
			},
		},
		{
			name: "unknown card_type",
			body: admin.InvalidateCardsRequest{
				StartDate: "2026-04-01T00:00:00Z",
				CardTypes: []string{"session_card_bogus"},
				Reason:    "r",
			},
		},
		{
			name: "empty reason",
			body: admin.InvalidateCardsRequest{
				StartDate: "2026-04-01T00:00:00Z",
				CardTypes: []string{"session_card_tokens"},
				Reason:    "",
			},
		},
		{
			name: "missing timezone on start_date",
			body: admin.InvalidateCardsRequest{
				StartDate: "2026-04-01T00:00:00",
				CardTypes: []string{"session_card_tokens"},
				Reason:    "r",
			},
		},
		{
			name: "end before start",
			body: admin.InvalidateCardsRequest{
				StartDate: "2026-04-20T00:00:00Z",
				EndDate:   "2026-04-01T00:00:00Z",
				CardTypes: []string{"session_card_tokens"},
				Reason:    "r",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env.CleanDB(t)
			adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
			testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")
			ts := setupTestServer(t, env)
			client := adminClient(t, env, ts, adminUser.ID)

			resp, err := client.Post("/api/v1/admin/cards/invalidate", tc.body)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestInvalidateCards_DryRunReturnsCountsNoWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	now := time.Now().UTC()
	seedSessionWithTokens(t, env, user.ID, now.Add(-1*time.Hour))
	seedSessionWithTokens(t, env, user.ID, now.Add(-2*time.Hour))
	seedSessionWithTokens(t, env, user.ID, now.Add(-3*time.Hour))

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	dryRun := true
	resp, err := client.Post("/api/v1/admin/cards/invalidate", admin.InvalidateCardsRequest{
		StartDate: now.Add(-4 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		CardTypes: []string{"session_card_tokens"},
		Reason:    "dry run test",
		DryRun:    &dryRun,
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)

	var body admin.InvalidateCardsResponse
	testutil.ParseJSON(t, resp, &body)
	if body.AffectedSessions != 3 {
		t.Errorf("AffectedSessions = %d, want 3", body.AffectedSessions)
	}
	if body.Executed {
		t.Errorf("Executed should be false for dry run")
	}

	// No writes: audit table and card table unchanged
	var auditCount int
	_ = env.DB.QueryRow(env.Ctx, `SELECT COUNT(*) FROM admin_card_invalidations`).Scan(&auditCount)
	if auditCount != 0 {
		t.Errorf("expected 0 audit rows after dry run, got %d", auditCount)
	}
	var tokensCount int
	_ = env.DB.QueryRow(env.Ctx, `SELECT COUNT(*) FROM session_card_tokens`).Scan(&tokensCount)
	if tokensCount != 3 {
		t.Errorf("expected 3 tokens cards (unchanged), got %d", tokensCount)
	}
}

func TestInvalidateCards_DryRunDefaultWhenOmitted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	now := time.Now().UTC()
	seedSessionWithTokens(t, env, user.ID, now.Add(-1*time.Hour))

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	// No DryRun field set — should default to true (no writes)
	resp, err := client.Post("/api/v1/admin/cards/invalidate", admin.InvalidateCardsRequest{
		StartDate: now.Add(-4 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		CardTypes: []string{"session_card_tokens"},
		Reason:    "default dry run",
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)

	var body admin.InvalidateCardsResponse
	testutil.ParseJSON(t, resp, &body)
	if body.Executed {
		t.Errorf("Executed should default to false when dry_run field is omitted")
	}

	var tokensCount int
	_ = env.DB.QueryRow(env.Ctx, `SELECT COUNT(*) FROM session_card_tokens`).Scan(&tokensCount)
	if tokensCount != 1 {
		t.Errorf("expected 1 tokens card (unchanged), got %d", tokensCount)
	}
}

func TestInvalidateCards_ExecuteDeletesAndAudits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	now := time.Now().UTC()
	seedSessionWithTokens(t, env, user.ID, now.Add(-1*time.Hour))
	seedSessionWithTokens(t, env, user.ID, now.Add(-2*time.Hour))

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	dryRun := false
	resp, err := client.Post("/api/v1/admin/cards/invalidate", admin.InvalidateCardsRequest{
		StartDate: now.Add(-4 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		CardTypes: []string{"session_card_tokens"},
		Reason:    "execute test",
		DryRun:    &dryRun,
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)

	var body admin.InvalidateCardsResponse
	testutil.ParseJSON(t, resp, &body)
	if !body.Executed {
		t.Errorf("Executed should be true for execute")
	}
	if body.AffectedSessions != 2 {
		t.Errorf("AffectedSessions = %d, want 2", body.AffectedSessions)
	}
	if body.CorrelationID == "" {
		t.Errorf("CorrelationID should be non-empty")
	}

	// Tokens cards deleted
	var tokensCount int
	_ = env.DB.QueryRow(env.Ctx, `SELECT COUNT(*) FROM session_card_tokens`).Scan(&tokensCount)
	if tokensCount != 0 {
		t.Errorf("expected 0 tokens cards after execute, got %d", tokensCount)
	}

	// Audit rows written
	var auditCount int
	_ = env.DB.QueryRow(env.Ctx, `SELECT COUNT(*) FROM admin_card_invalidations WHERE correlation_id = $1`, body.CorrelationID).Scan(&auditCount)
	if auditCount != 2 {
		t.Errorf("expected 2 audit rows, got %d", auditCount)
	}
}

func TestListCardInvalidations_ReturnsRecentWithEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	now := time.Now().UTC()
	seedSessionWithTokens(t, env, user.ID, now.Add(-1*time.Hour))

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	dryRun := false
	_, err := client.Post("/api/v1/admin/cards/invalidate", admin.InvalidateCardsRequest{
		StartDate: now.Add(-4 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		CardTypes: []string{"session_card_tokens"},
		Reason:    "listing test",
		DryRun:    &dryRun,
	})
	if err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	resp, err := client.Get("/api/v1/admin/cards/invalidations")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)

	var body admin.CardInvalidationsListResponse
	testutil.ParseJSON(t, resp, &body)
	if len(body.Rows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(body.Rows))
	}
	row := body.Rows[0]
	if row.AdminEmail != "admin@example.com" {
		t.Errorf("AdminEmail = %q, want admin@example.com", row.AdminEmail)
	}
	if row.Reason != "listing test" {
		t.Errorf("Reason = %q, want listing test", row.Reason)
	}
	if len(row.CardTypes) != 1 || row.CardTypes[0] != "session_card_tokens" {
		t.Errorf("CardTypes = %v, want [session_card_tokens]", row.CardTypes)
	}
}

func TestListCardInvalidations_FilterByCorrelationID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	now := time.Now().UTC()
	seedSessionWithTokens(t, env, user.ID, now.Add(-1*time.Hour))
	seedSessionWithTokens(t, env, user.ID, now.Add(-2*time.Hour))

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	dryRun := false
	// First invalidation
	resp1, err := client.Post("/api/v1/admin/cards/invalidate", admin.InvalidateCardsRequest{
		StartDate: now.Add(-4 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		CardTypes: []string{"session_card_tokens"},
		Reason:    "first",
		DryRun:    &dryRun,
	})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	var body1 admin.InvalidateCardsResponse
	testutil.ParseJSON(t, resp1, &body1)

	// Reseed + second invalidation
	seedSessionWithTokens(t, env, user.ID, now.Add(-30*time.Minute))
	resp2, err := client.Post("/api/v1/admin/cards/invalidate", admin.InvalidateCardsRequest{
		StartDate: now.Add(-4 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		CardTypes: []string{"session_card_tokens"},
		Reason:    "second",
		DryRun:    &dryRun,
	})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	var body2 admin.InvalidateCardsResponse
	testutil.ParseJSON(t, resp2, &body2)

	// Filter to second correlation_id
	resp, err := client.Get("/api/v1/admin/cards/invalidations?correlation_id=" + body2.CorrelationID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)

	var list admin.CardInvalidationsListResponse
	testutil.ParseJSON(t, resp, &list)
	for _, row := range list.Rows {
		if row.CorrelationID != body2.CorrelationID {
			t.Errorf("row correlation = %s, want %s", row.CorrelationID, body2.CorrelationID)
		}
		if row.Reason != "second" {
			t.Errorf("row reason = %q, want second", row.Reason)
		}
	}
}
