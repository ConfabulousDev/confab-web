package dbadmincardinvalidations_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/ConfabulousDev/confab-web/internal/db/dbadmincardinvalidations"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// seedSessionsAndCards creates N sessions for a user, each with last_message_at
// set to lastMsg. Optionally inserts token + smart_recap card rows for each session.
// Returns the created session IDs in creation order.
func seedSessionsAndCards(t *testing.T, env *testutil.TestEnvironment, userID int64, n int, lastMsg time.Time, withTokens, withRecap bool) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		sid := uuid.NewString()
		_, err := env.DB.Exec(env.Ctx, `
			INSERT INTO sessions (id, user_id, external_id, first_seen, last_message_at)
			VALUES ($1, $2, $3, $4, $5)
		`, sid, userID, "ext-"+sid[:8], lastMsg, lastMsg)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
		if withTokens {
			_, err = env.DB.Exec(env.Ctx, `
				INSERT INTO session_card_tokens (
					session_id, version, computed_at, up_to_line,
					input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, estimated_cost_usd
				) VALUES ($1, 1, NOW(), 100, 0, 0, 0, 0, '0.00')
			`, sid)
			if err != nil {
				t.Fatalf("insert tokens card: %v", err)
			}
		}
		if withRecap {
			_, err = env.DB.Exec(env.Ctx, `
				INSERT INTO session_card_smart_recap (
					session_id, version, computed_at, up_to_line,
					recap, went_well, went_bad, human_suggestions, environment_suggestions, default_context_suggestions,
					model_used, input_tokens, output_tokens, generation_time_ms
				) VALUES ($1, 1, NOW(), 100, '', '[]', '[]', '[]', '[]', '[]', 'test-model', 0, 0, 0)
			`, sid)
			if err != nil {
				t.Fatalf("insert smart recap card: %v", err)
			}
		}
		ids = append(ids, sid)
	}
	return ids
}

func TestCountAffected_EmptyWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-10 * time.Hour)
	end := time.Now().UTC()

	result, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: start,
		EndDate:   &end,
		CardTypes: []string{"session_card_tokens"},
	})
	if err != nil {
		t.Fatalf("CountAffected failed: %v", err)
	}
	if result.AffectedSessions != 0 {
		t.Errorf("AffectedSessions = %d, want 0", result.AffectedSessions)
	}
	if result.AffectedCards["session_card_tokens"] != 0 {
		t.Errorf("AffectedCards[tokens] = %d, want 0", result.AffectedCards["session_card_tokens"])
	}
}

func TestCountAffected_SessionsInWindowWithMatchingCards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)
	outOfWindow := time.Now().UTC().Add(-48 * time.Hour)

	// 3 sessions in window, all with tokens cards.
	seedSessionsAndCards(t, env, user.ID, 3, inWindow, true, false)
	// 2 sessions outside window, also with tokens cards.
	seedSessionsAndCards(t, env, user.ID, 2, outOfWindow, true, false)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-4 * time.Hour)
	end := time.Now().UTC()

	result, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: start,
		EndDate:   &end,
		CardTypes: []string{"session_card_tokens"},
	})
	if err != nil {
		t.Fatalf("CountAffected failed: %v", err)
	}
	if result.AffectedSessions != 3 {
		t.Errorf("AffectedSessions = %d, want 3", result.AffectedSessions)
	}
	if result.AffectedCards["session_card_tokens"] != 3 {
		t.Errorf("AffectedCards[tokens] = %d, want 3", result.AffectedCards["session_card_tokens"])
	}
}

func TestCountAffected_IntersectionSemantic_SessionWithoutSelectedCardExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)

	// 2 sessions with tokens + smart_recap.
	seedSessionsAndCards(t, env, user.ID, 2, inWindow, true, true)
	// 3 sessions with only tokens (no smart_recap).
	seedSessionsAndCards(t, env, user.ID, 3, inWindow, true, false)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-4 * time.Hour)
	end := time.Now().UTC()

	result, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: start,
		EndDate:   &end,
		CardTypes: []string{"session_card_smart_recap"},
	})
	if err != nil {
		t.Fatalf("CountAffected failed: %v", err)
	}
	// Intersection semantic: only sessions with at least one selected card are counted.
	if result.AffectedSessions != 2 {
		t.Errorf("AffectedSessions = %d, want 2 (intersection semantic)", result.AffectedSessions)
	}
	if result.AffectedCards["session_card_smart_recap"] != 2 {
		t.Errorf("AffectedCards[smart_recap] = %d, want 2", result.AffectedCards["session_card_smart_recap"])
	}
}

func TestCountAffected_OpenEndedWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	recent := time.Now().UTC().Add(-1 * time.Hour)
	older := time.Now().UTC().Add(-48 * time.Hour)

	seedSessionsAndCards(t, env, user.ID, 2, recent, true, false)
	seedSessionsAndCards(t, env, user.ID, 1, older, true, false)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-7 * 24 * time.Hour)

	// No end_date — open-ended
	result, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: start,
		EndDate:   nil,
		CardTypes: []string{"session_card_tokens"},
	})
	if err != nil {
		t.Fatalf("CountAffected failed: %v", err)
	}
	if result.AffectedSessions != 3 {
		t.Errorf("AffectedSessions = %d, want 3 (open-ended)", result.AffectedSessions)
	}
}

func TestExecute_DeletesCardsAndWritesAudit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)
	ids := seedSessionsAndCards(t, env, user.ID, 3, inWindow, true, true)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-4 * time.Hour)
	end := time.Now().UTC()

	res, err := store.Execute(context.Background(), dbadmincardinvalidations.ExecuteRequest{
		CountRequest: dbadmincardinvalidations.CountRequest{
			StartDate: start,
			EndDate:   &end,
			CardTypes: []string{"session_card_tokens"},
		},
		AdminUserID: admin.ID,
		Reason:      "pricing fix",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if res.Result.AffectedSessions != 3 {
		t.Errorf("AffectedSessions = %d, want 3", res.Result.AffectedSessions)
	}
	if res.Result.AffectedCards["session_card_tokens"] != 3 {
		t.Errorf("AffectedCards[tokens] = %d, want 3", res.Result.AffectedCards["session_card_tokens"])
	}
	if res.CorrelationID == uuid.Nil {
		t.Errorf("expected non-zero correlation_id")
	}

	// Tokens cards should be deleted
	var tokensRemaining int
	if err := env.DB.QueryRow(env.Ctx,
		`SELECT COUNT(*) FROM session_card_tokens WHERE session_id = ANY($1)`,
		pq.Array(ids),
	).Scan(&tokensRemaining); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if tokensRemaining != 0 {
		t.Errorf("expected 0 tokens cards after Execute, got %d", tokensRemaining)
	}

	// Smart recap cards (not selected) should be untouched
	var recapRemaining int
	if err := env.DB.QueryRow(env.Ctx,
		`SELECT COUNT(*) FROM session_card_smart_recap WHERE session_id = ANY($1)`,
		pq.Array(ids),
	).Scan(&recapRemaining); err != nil {
		t.Fatalf("count recap: %v", err)
	}
	if recapRemaining != 3 {
		t.Errorf("expected 3 smart_recap cards (untouched), got %d", recapRemaining)
	}

	// Audit rows should exist for each affected session
	var auditCount int
	if err := env.DB.QueryRow(env.Ctx,
		`SELECT COUNT(*) FROM admin_card_invalidations WHERE correlation_id = $1`,
		res.CorrelationID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 3 {
		t.Errorf("expected 3 audit rows for correlation, got %d", auditCount)
	}

	// All audit rows have card_types = requested set and admin_user_id = admin.ID
	rows, err := env.DB.Conn().QueryContext(env.Ctx, `
		SELECT admin_user_id, card_types, reason FROM admin_card_invalidations
		WHERE correlation_id = $1
	`, res.CorrelationID)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var auditAdmin int64
		var cardTypes pq.StringArray
		var reason string
		if err := rows.Scan(&auditAdmin, &cardTypes, &reason); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if auditAdmin != admin.ID {
			t.Errorf("admin_user_id = %d, want %d", auditAdmin, admin.ID)
		}
		if len(cardTypes) != 1 || cardTypes[0] != "session_card_tokens" {
			t.Errorf("card_types = %v, want [session_card_tokens]", cardTypes)
		}
		if reason != "pricing fix" {
			t.Errorf("reason = %q, want %q", reason, "pricing fix")
		}
	}
}

func TestExecute_Chunked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)
	_ = seedSessionsAndCards(t, env, user.ID, 5, inWindow, true, false)

	// Force batch size of 2 so 5 sessions → 3 commits
	store := &dbadmincardinvalidations.Store{DB: env.DB, BatchSize: 2}

	start := time.Now().UTC().Add(-4 * time.Hour)
	end := time.Now().UTC()

	res, err := store.Execute(context.Background(), dbadmincardinvalidations.ExecuteRequest{
		CountRequest: dbadmincardinvalidations.CountRequest{
			StartDate: start,
			EndDate:   &end,
			CardTypes: []string{"session_card_tokens"},
		},
		AdminUserID: admin.ID,
		Reason:      "chunked test",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if res.Result.AffectedSessions != 5 {
		t.Errorf("AffectedSessions = %d, want 5", res.Result.AffectedSessions)
	}
	if res.CompletedBatches != 3 {
		t.Errorf("CompletedBatches = %d, want 3 (ceil(5/2))", res.CompletedBatches)
	}
}

func TestListRecent_OrdersByInvalidatedAtDesc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")

	inWindow := time.Now().UTC().Add(-30 * time.Minute)
	_ = seedSessionsAndCards(t, env, user.ID, 2, inWindow, true, false)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-2 * time.Hour)
	end := time.Now().UTC()

	// First execute
	_, err := store.Execute(context.Background(), dbadmincardinvalidations.ExecuteRequest{
		CountRequest: dbadmincardinvalidations.CountRequest{
			StartDate: start,
			EndDate:   &end,
			CardTypes: []string{"session_card_tokens"},
		},
		AdminUserID: admin.ID,
		Reason:      "first",
	})
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}

	// Second execute (should find 0 cards left, but produce a separate audit with 0 rows)
	// To have something meaningful, seed more sessions first.
	_ = seedSessionsAndCards(t, env, user.ID, 1, inWindow, true, false)
	second, err := store.Execute(context.Background(), dbadmincardinvalidations.ExecuteRequest{
		CountRequest: dbadmincardinvalidations.CountRequest{
			StartDate: start,
			EndDate:   &end,
			CardTypes: []string{"session_card_tokens"},
		},
		AdminUserID: admin.ID,
		Reason:      "second",
	})
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}

	rows, err := store.ListRecent(context.Background(), 100)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least 1 row from ListRecent")
	}

	// Newest row should come first.
	if rows[0].CorrelationID != second.CorrelationID {
		t.Errorf("newest row correlation = %s, want second execute %s", rows[0].CorrelationID, second.CorrelationID)
	}
	if rows[0].AdminEmail != "admin@test.com" {
		t.Errorf("AdminEmail = %q, want admin@test.com", rows[0].AdminEmail)
	}
}

func TestListByCorrelationID_FiltersToOneRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	inWindow := time.Now().UTC().Add(-30 * time.Minute)
	_ = seedSessionsAndCards(t, env, user.ID, 3, inWindow, true, false)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-2 * time.Hour)
	end := time.Now().UTC()

	res, err := store.Execute(context.Background(), dbadmincardinvalidations.ExecuteRequest{
		CountRequest: dbadmincardinvalidations.CountRequest{
			StartDate: start,
			EndDate:   &end,
			CardTypes: []string{"session_card_tokens"},
		},
		AdminUserID: admin.ID,
		Reason:      "filter test",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	rows, err := store.ListByCorrelationID(context.Background(), res.CorrelationID)
	if err != nil {
		t.Fatalf("ListByCorrelationID: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 rows for correlation, got %d", len(rows))
	}
	for _, row := range rows {
		if row.CorrelationID != res.CorrelationID {
			t.Errorf("row correlation_id = %s, want %s", row.CorrelationID, res.CorrelationID)
		}
	}
}
