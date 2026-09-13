package dbadmincardinvalidations_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db/dbadmincardinvalidations"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// seedSessionsAndCards creates N sessions for a user, each with last_message_at
// set to lastMsg. Optionally inserts token + smart_recap card rows for each session.
// Returns the created session IDs in creation order.
func seedSessionsAndCards(t *testing.T, env *testutil.TestEnvironment, userID int64, n int, lastMsg time.Time, withTokens, withRecap bool) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for range n {
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

// seedProviderSession creates one in-window session with an explicit session_type
// and first_user_message (nil = NULL). withTokens adds a session_card_tokens row.
func seedProviderSession(t *testing.T, env *testutil.TestEnvironment, userID int64, sessionType string, firstUserMessage *string, lastMsg time.Time, withTokens bool) string {
	t.Helper()
	sid := uuid.NewString()
	if _, err := env.DB.Exec(env.Ctx, `
		INSERT INTO sessions (id, user_id, external_id, first_seen, last_message_at, session_type, first_user_message)
		VALUES ($1, $2, $3, $4, $4, $5, $6)
	`, sid, userID, "ext-"+sid[:8], lastMsg, sessionType, firstUserMessage); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if withTokens {
		if _, err := env.DB.Exec(env.Ctx, `
			INSERT INTO session_card_tokens (
				session_id, version, computed_at, up_to_line,
				input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, estimated_cost_usd
			) VALUES ($1, 1, NOW(), 100, 0, 0, 0, 0, '0.00')
		`, sid); err != nil {
			t.Fatalf("insert tokens card: %v", err)
		}
	}
	return sid
}

func strPtr(s string) *string { return &s }

// titleRecomputeRequested reports whether the session's title_recompute_requested_at marker is set.
func titleRecomputeRequested(t *testing.T, env *testutil.TestEnvironment, sessionID string) bool {
	t.Helper()
	var marked bool
	if err := env.DB.QueryRow(env.Ctx,
		`SELECT title_recompute_requested_at IS NOT NULL FROM sessions WHERE id = $1`, sessionID,
	).Scan(&marked); err != nil {
		t.Fatalf("read marker: %v", err)
	}
	return marked
}

func TestCountAffected_ProviderFilterNarrowsCardCountsIncludingLegacyAlias(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)
	seedProviderSession(t, env, user.ID, models.ProviderClaudeCode, strPtr("hi"), inWindow, true)
	seedProviderSession(t, env, user.ID, models.ProviderClaudeCodeLegacy, strPtr("hi"), inWindow, true)
	seedProviderSession(t, env, user.ID, models.ProviderCodex, strPtr("hi"), inWindow, true)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-4 * time.Hour)

	result, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: start,
		CardTypes: []string{"session_card_tokens"},
		Providers: models.ExpandWithAliases([]string{models.ProviderClaudeCode}),
	})
	if err != nil {
		t.Fatalf("CountAffected: %v", err)
	}
	if result.AffectedSessions != 2 {
		t.Errorf("AffectedSessions = %d, want 2 (claude-code + legacy 'Claude Code', not codex)", result.AffectedSessions)
	}
	if result.AffectedCards["session_card_tokens"] != 2 {
		t.Errorf("AffectedCards[tokens] = %d, want 2", result.AffectedCards["session_card_tokens"])
	}

	// No provider filter → all three.
	all, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: start,
		CardTypes: []string{"session_card_tokens"},
	})
	if err != nil {
		t.Fatalf("CountAffected (no filter): %v", err)
	}
	if all.AffectedSessions != 3 || all.AffectedCards["session_card_tokens"] != 3 {
		t.Errorf("unfiltered = %d sessions / %d cards, want 3 / 3", all.AffectedSessions, all.AffectedCards["session_card_tokens"])
	}
}

func TestExecute_ProviderFilterScopesCardDeletes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)
	codexID := seedProviderSession(t, env, user.ID, models.ProviderCodex, strPtr("hi"), inWindow, true)
	claudeID := seedProviderSession(t, env, user.ID, models.ProviderClaudeCode, strPtr("hi"), inWindow, true)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	res, err := store.Execute(context.Background(), dbadmincardinvalidations.ExecuteRequest{
		CountRequest: dbadmincardinvalidations.CountRequest{
			StartDate: time.Now().UTC().Add(-4 * time.Hour),
			CardTypes: []string{"session_card_tokens"},
			Providers: models.ExpandWithAliases([]string{models.ProviderCodex}),
		},
		AdminUserID: admin.ID,
		Reason:      "codex only",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Result.AffectedSessions != 1 {
		t.Errorf("AffectedSessions = %d, want 1", res.Result.AffectedSessions)
	}

	countTokens := func(sid string) int {
		var n int
		if err := env.DB.QueryRow(env.Ctx, `SELECT COUNT(*) FROM session_card_tokens WHERE session_id = $1`, sid).Scan(&n); err != nil {
			t.Fatalf("count tokens: %v", err)
		}
		return n
	}
	if n := countTokens(codexID); n != 0 {
		t.Errorf("codex tokens cards = %d, want 0 (deleted)", n)
	}
	if n := countTokens(claudeID); n != 1 {
		t.Errorf("claude tokens cards = %d, want 1 (outside provider filter)", n)
	}
}

// TestCountAffected_SessionTitleCandidates pins the title candidate predicate:
// Codex with NULL first_user_message, or Cursor whose first_user_message still
// carries the <user_query> envelope. Nothing else qualifies.
func TestCountAffected_SessionTitleCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)
	outOfWindow := time.Now().UTC().Add(-48 * time.Hour)

	seedProviderSession(t, env, user.ID, models.ProviderCodex, nil, inWindow, false)                                            // ✓
	seedProviderSession(t, env, user.ID, models.ProviderCodex, strPtr("already titled"), inWindow, false)                       // ✗
	seedProviderSession(t, env, user.ID, models.ProviderCursor, strPtr("<user_query>\nfix it\n</user_query>"), inWindow, false) // ✓
	seedProviderSession(t, env, user.ID, models.ProviderCursor, strPtr("fix it"), inWindow, false)                              // ✗
	seedProviderSession(t, env, user.ID, models.ProviderClaudeCode, nil, inWindow, false)                                       // ✗
	seedProviderSession(t, env, user.ID, models.ProviderOpencode, nil, inWindow, false)                                         // ✗
	seedProviderSession(t, env, user.ID, models.ProviderCodex, nil, outOfWindow, false)                                         // ✗ (window)

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	start := time.Now().UTC().Add(-4 * time.Hour)

	result, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: start,
		CardTypes: []string{analytics.SessionTitleInvalidationTarget},
	})
	if err != nil {
		t.Fatalf("CountAffected with session_title only must not reach table interpolation: %v", err)
	}
	if result.AffectedSessions != 2 {
		t.Errorf("AffectedSessions = %d, want 2", result.AffectedSessions)
	}
	if got := result.AffectedCards[analytics.SessionTitleInvalidationTarget]; got != 2 {
		t.Errorf("AffectedCards[session_title] = %d, want 2", got)
	}

	// Provider filter applies to title candidates too.
	cursorOnly, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: start,
		CardTypes: []string{analytics.SessionTitleInvalidationTarget},
		Providers: []string{models.ProviderCursor},
	})
	if err != nil {
		t.Fatalf("CountAffected (cursor): %v", err)
	}
	if cursorOnly.AffectedSessions != 1 || cursorOnly.AffectedCards[analytics.SessionTitleInvalidationTarget] != 1 {
		t.Errorf("cursor-only = %d sessions / %d titles, want 1 / 1",
			cursorOnly.AffectedSessions, cursorOnly.AffectedCards[analytics.SessionTitleInvalidationTarget])
	}
}

func TestCountAffected_UnionOfCardAndTitleSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)
	seedProviderSession(t, env, user.ID, models.ProviderCodex, nil, inWindow, true)               // card + title
	seedProviderSession(t, env, user.ID, models.ProviderCodex, nil, inWindow, false)              // title only
	seedProviderSession(t, env, user.ID, models.ProviderClaudeCode, strPtr("x"), inWindow, true)  // card only
	seedProviderSession(t, env, user.ID, models.ProviderClaudeCode, strPtr("x"), inWindow, false) // neither

	store := &dbadmincardinvalidations.Store{DB: env.DB}
	result, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: time.Now().UTC().Add(-4 * time.Hour),
		CardTypes: []string{"session_card_tokens", analytics.SessionTitleInvalidationTarget},
	})
	if err != nil {
		t.Fatalf("CountAffected: %v", err)
	}
	if result.AffectedSessions != 3 {
		t.Errorf("AffectedSessions = %d, want 3 (distinct union)", result.AffectedSessions)
	}
	if result.AffectedCards["session_card_tokens"] != 2 {
		t.Errorf("AffectedCards[tokens] = %d, want 2", result.AffectedCards["session_card_tokens"])
	}
	if result.AffectedCards[analytics.SessionTitleInvalidationTarget] != 2 {
		t.Errorf("AffectedCards[session_title] = %d, want 2", result.AffectedCards[analytics.SessionTitleInvalidationTarget])
	}
}

func TestExecute_SessionTitleMarksOnlyCandidatesAndAudits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	admin := testutil.CreateTestUser(t, env, "admin@test.com", "Admin")
	user := testutil.CreateTestUser(t, env, "user@test.com", "User")
	inWindow := time.Now().UTC().Add(-2 * time.Hour)
	codexNull := seedProviderSession(t, env, user.ID, models.ProviderCodex, nil, inWindow, true)
	claudeWithCard := seedProviderSession(t, env, user.ID, models.ProviderClaudeCode, strPtr("x"), inWindow, true)
	codexTitled := seedProviderSession(t, env, user.ID, models.ProviderCodex, strPtr("titled"), inWindow, false)

	// Batch size 1 exercises the per-batch mark UPDATE across several commits.
	store := &dbadmincardinvalidations.Store{DB: env.DB, BatchSize: 1}
	res, err := store.Execute(context.Background(), dbadmincardinvalidations.ExecuteRequest{
		CountRequest: dbadmincardinvalidations.CountRequest{
			StartDate: time.Now().UTC().Add(-4 * time.Hour),
			CardTypes: []string{"session_card_tokens", analytics.SessionTitleInvalidationTarget},
		},
		AdminUserID: admin.ID,
		Reason:      "codex title repair",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Result.AffectedSessions != 2 {
		t.Errorf("AffectedSessions = %d, want 2", res.Result.AffectedSessions)
	}
	if got := res.Result.AffectedCards[analytics.SessionTitleInvalidationTarget]; got != 1 {
		t.Errorf("AffectedCards[session_title] = %d, want 1", got)
	}
	if got := res.Result.AffectedCards["session_card_tokens"]; got != 2 {
		t.Errorf("AffectedCards[tokens] = %d, want 2", got)
	}

	if !titleRecomputeRequested(t, env, codexNull) {
		t.Error("codex NULL-title session should be marked for title recompute")
	}
	if titleRecomputeRequested(t, env, claudeWithCard) {
		t.Error("claude session (card-affected only) must not be marked for title recompute")
	}
	if titleRecomputeRequested(t, env, codexTitled) {
		t.Error("codex session with an existing title must not be marked")
	}

	rows, err := store.ListByCorrelationID(context.Background(), res.CorrelationID)
	if err != nil {
		t.Fatalf("ListByCorrelationID: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("audit rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if len(r.CardTypes) != 2 || r.CardTypes[0] != "session_card_tokens" || r.CardTypes[1] != analytics.SessionTitleInvalidationTarget {
			t.Errorf("audit card_types = %v, want exactly as submitted", r.CardTypes)
		}
	}
}

func TestCountAffected_RejectsUnknownTarget(t *testing.T) {
	store := &dbadmincardinvalidations.Store{}
	_, err := store.CountAffected(context.Background(), dbadmincardinvalidations.CountRequest{
		StartDate: time.Now(),
		CardTypes: []string{"sessions; DROP TABLE sessions"},
	})
	if err == nil {
		t.Fatal("expected unknown card_type error before any SQL runs")
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
