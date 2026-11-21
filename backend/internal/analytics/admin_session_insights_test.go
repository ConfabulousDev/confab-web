package analytics_test

import (
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// setFirstSeen pins a session's first_seen so bucketing tests are deterministic.
func setFirstSeen(t *testing.T, env *testutil.TestEnvironment, sessionID string, ts time.Time) {
	t.Helper()
	if _, err := env.DB.Exec(env.Ctx, `UPDATE sessions SET first_seen = $2 WHERE id = $1`, sessionID, ts); err != nil {
		t.Fatalf("set first_seen: %v", err)
	}
}

// listableSession creates a listable session (summary + synced lines) of the given
// provider at the given first_seen.
func listableSession(t *testing.T, env *testutil.TestEnvironment, userID int64, ext, provider string, firstSeen time.Time) string {
	t.Helper()
	id := testutil.CreateTestSessionWithProvider(t, env, userID, ext, provider)
	testutil.MakeSessionListable(t, env, id)
	setFirstSeen(t, env, id, firstSeen)
	return id
}

func TestSessionInsightsBucketSize(t *testing.T) {
	day := 24 * time.Hour
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		span time.Duration
		want string
	}{
		{"7 days buckets by day", 7 * day, analytics.BucketDay},
		{"31 days still buckets by day", 31 * day, analytics.BucketDay},
		{"32 days buckets by week", 32 * day, analytics.BucketWeek},
		{"one year buckets by week", 366 * day, analytics.BucketWeek},
		{"beyond a year buckets by month", 367 * day, analytics.BucketMonth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := analytics.SessionInsightsBucketSize(base, base.Add(tc.span)); got != tc.want {
				t.Errorf("span %v: bucket = %q, want %q", tc.span, got, tc.want)
			}
		})
	}
}

func TestAdminProviderShare(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	store := analytics.NewStore(env.DB.Conn())

	t.Run("buckets by day and folds legacy Claude Code into claude-code", func(t *testing.T) {
		env.CleanDB(t)
		u := testutil.CreateTestUser(t, env, "a@test.com", "A")
		d1 := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
		d2 := time.Date(2026, 3, 3, 23, 59, 0, 0, time.UTC)
		listableSession(t, env, u.ID, "s1", models.ProviderClaudeCode, d1)
		legacy := testutil.CreateTestSessionLegacyClaudeCode(t, env, u.ID, "s2")
		testutil.MakeSessionListable(t, env, legacy)
		setFirstSeen(t, env, legacy, d1.Add(time.Hour))
		listableSession(t, env, u.ID, "s3", models.ProviderCodex, d2)

		from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
		got, err := store.AdminProviderShare(env.Ctx, analytics.SessionInsightsFilter{From: from, To: to})
		if err != nil {
			t.Fatalf("AdminProviderShare: %v", err)
		}
		if got.BucketSize != analytics.BucketDay {
			t.Errorf("bucket size = %q, want day", got.BucketSize)
		}
		if len(got.Buckets) != 2 {
			t.Fatalf("want 2 non-empty buckets, got %d: %+v", len(got.Buckets), got.Buckets)
		}
		if !got.Buckets[0].Start.Equal(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("first bucket start = %v, want 2026-03-02", got.Buckets[0].Start)
		}
		if c := got.Buckets[0].Counts[models.ProviderClaudeCode]; c != 2 {
			t.Errorf("legacy row must fold into claude-code: count = %d, want 2", c)
		}
		if _, ok := got.Buckets[0].Counts[models.ProviderClaudeCodeLegacy]; ok {
			t.Errorf("legacy provider key must not appear on the wire")
		}
		if c := got.Buckets[1].Counts[models.ProviderCodex]; c != 1 {
			t.Errorf("codex count on 2026-03-03 = %d, want 1", c)
		}
		if got.Totals[models.ProviderClaudeCode] != 2 || got.Totals[models.ProviderCodex] != 1 {
			t.Errorf("totals = %v, want claude-code:2 codex:1", got.Totals)
		}
	})

	t.Run("week and month bucketing truncate to Monday and the 1st", func(t *testing.T) {
		env.CleanDB(t)
		u := testutil.CreateTestUser(t, env, "a@test.com", "A")
		// 2026-03-04 is a Wednesday; its ISO week starts Monday 2026-03-02.
		listableSession(t, env, u.ID, "w", models.ProviderCodex, time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC))

		weekly, err := store.AdminProviderShare(env.Ctx, analytics.SessionInsightsFilter{
			From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("weekly: %v", err)
		}
		if weekly.BucketSize != analytics.BucketWeek || len(weekly.Buckets) != 1 ||
			!weekly.Buckets[0].Start.Equal(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("weekly = %+v, want one week bucket starting 2026-03-02", weekly)
		}

		// An explicit range buckets by its own span, even when the data is narrower.
		monthly, err := store.AdminProviderShare(env.Ctx, analytics.SessionInsightsFilter{
			From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("monthly: %v", err)
		}
		if monthly.BucketSize != analytics.BucketMonth || len(monthly.Buckets) != 1 ||
			!monthly.Buckets[0].Start.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("monthly = %+v, want one month bucket starting 2026-03-01", monthly)
		}
	})

	t.Run("an all-time request (zero From) clamps to the earliest matching session", func(t *testing.T) {
		env.CleanDB(t)
		u := testutil.CreateTestUser(t, env, "a@test.com", "A")
		earliest := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
		listableSession(t, env, u.ID, "e", models.ProviderCodex, earliest)
		to := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

		got, err := store.AdminProviderShare(env.Ctx, analytics.SessionInsightsFilter{To: to})
		if err != nil {
			t.Fatalf("AdminProviderShare: %v", err)
		}
		if !got.From.Equal(earliest) {
			t.Errorf("effective from = %v, want earliest session %v", got.From, earliest)
		}
		if got.BucketSize != analytics.BucketDay {
			t.Errorf("bucket size over a 6-day effective span = %q, want day", got.BucketSize)
		}
	})

	t.Run("listable gate, range bounds, provider and owner filters", func(t *testing.T) {
		env.CleanDB(t)
		u1 := testutil.CreateTestUser(t, env, "a@test.com", "A")
		u2 := testutil.CreateTestUser(t, env, "b@test.com", "B")
		in := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
		listableSession(t, env, u1.ID, "c1", models.ProviderClaudeCode, in)
		listableSession(t, env, u2.ID, "x2", models.ProviderCodex, in)
		listableSession(t, env, u2.ID, "c2", models.ProviderCursor, in)
		// Not listable: no summary, no synced lines.
		empty := testutil.CreateTestSessionWithProvider(t, env, u1.ID, "empty", models.ProviderCodex)
		setFirstSeen(t, env, empty, in)
		// Outside the range: `to` is exclusive.
		listableSession(t, env, u1.ID, "late", models.ProviderCodex, time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC))

		from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)

		all, err := store.AdminProviderShare(env.Ctx, analytics.SessionInsightsFilter{From: from, To: to})
		if err != nil {
			t.Fatalf("all: %v", err)
		}
		if all.Totals[models.ProviderCodex] != 1 {
			t.Errorf("unlistable and out-of-range sessions must be excluded: codex total = %d, want 1", all.Totals[models.ProviderCodex])
		}

		byProvider, err := store.AdminProviderShare(env.Ctx, analytics.SessionInsightsFilter{
			From: from, To: to, Providers: []string{models.ProviderCodex, models.ProviderCursor},
		})
		if err != nil {
			t.Fatalf("provider filter: %v", err)
		}
		if byProvider.Totals[models.ProviderClaudeCode] != 0 || byProvider.Totals[models.ProviderCodex] != 1 || byProvider.Totals[models.ProviderCursor] != 1 {
			t.Errorf("provider filter totals = %v, want codex:1 cursor:1 only", byProvider.Totals)
		}

		byOwner, err := store.AdminProviderShare(env.Ctx, analytics.SessionInsightsFilter{From: from, To: to, OwnerID: &u1.ID})
		if err != nil {
			t.Fatalf("owner filter: %v", err)
		}
		if len(byOwner.Totals) != 1 || byOwner.Totals[models.ProviderClaudeCode] != 1 {
			t.Errorf("owner filter totals = %v, want claude-code:1 only", byOwner.Totals)
		}
	})

	t.Run("an empty range returns empty buckets and totals, not nil", func(t *testing.T) {
		env.CleanDB(t)
		got, err := store.AdminProviderShare(env.Ctx, analytics.SessionInsightsFilter{
			From: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("AdminProviderShare: %v", err)
		}
		if got.Buckets == nil || got.Totals == nil || len(got.Buckets) != 0 || len(got.Totals) != 0 {
			t.Errorf("empty range = %+v, want non-nil empty buckets/totals", got)
		}
	})
}

func TestAdminSessionPoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	store := analytics.NewStore(env.DB.Conn())
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	t.Run("carries every metric, nulls for missing cards, and a sanitized repo", func(t *testing.T) {
		env.CleanDB(t)
		u := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		rich := testutil.CreateTestSessionFull(t, env, u.ID, "rich", testutil.TestSessionFullOpts{
			RepoURL: "https://user:secret-token@github.com/acme/widgets.git", Branch: "main",
			Summary: "Fix the widgets", SyncLines: 250,
		})
		setFirstSeen(t, env, rich, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC))
		if _, err := env.DB.Exec(env.Ctx, `UPDATE sessions SET custom_title = 'Custom title' WHERE id = $1`, rich); err != nil {
			t.Fatal(err)
		}
		testutil.SeedTokensV2Card(t, env, rich, analytics.TokensV2Data{TotalInput: 1000, TotalOutput: 500, TotalCostUSD: "1.25"})
		if _, err := env.DB.Exec(env.Ctx, `INSERT INTO session_card_session (session_id, computed_at, up_to_line, duration_ms) VALUES ($1, now(), 250, 60000)`, rich); err != nil {
			t.Fatal(err)
		}
		if _, err := env.DB.Exec(env.Ctx, `INSERT INTO session_card_conversation (session_id, computed_at, up_to_line, user_turns) VALUES ($1, now(), 250, 7)`, rich); err != nil {
			t.Fatal(err)
		}
		bare := listableSession(t, env, u.ID, "bare", models.ProviderCodex, time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC))

		got, err := store.AdminSessionPoints(env.Ctx, analytics.SessionInsightsFilter{From: from, To: to}, 100)
		if err != nil {
			t.Fatalf("AdminSessionPoints: %v", err)
		}
		if got.Total != 2 || got.Returned != 2 || len(got.Points) != 2 {
			t.Fatalf("total/returned = %d/%d (%d points), want 2/2", got.Total, got.Returned, len(got.Points))
		}
		byID := map[string]analytics.SessionPoint{}
		for _, p := range got.Points {
			byID[p.ID] = p
		}
		r := byID[rich]
		if r.Title != "Custom title" || r.OwnerEmail != "owner@test.com" || r.Provider != models.ProviderClaudeCode {
			t.Errorf("rich point identity = %+v", r)
		}
		if r.Repo == nil || *r.Repo != "acme/widgets" {
			t.Errorf("repo must be the sanitized owner/repo, got %v", r.Repo)
		}
		if r.Branch == nil || *r.Branch != "main" {
			t.Errorf("branch = %v, want main", r.Branch)
		}
		if r.Lines != 250 {
			t.Errorf("lines = %d, want 250", r.Lines)
		}
		if r.Tokens == nil || *r.Tokens != 1500 {
			t.Errorf("tokens (input+output) = %v, want 1500", r.Tokens)
		}
		if r.CostUSD == nil || *r.CostUSD != 1.25 {
			t.Errorf("cost = %v, want 1.25", r.CostUSD)
		}
		if r.UserTurns == nil || *r.UserTurns != 7 {
			t.Errorf("user_turns = %v, want 7", r.UserTurns)
		}
		if r.DurationMs == nil || *r.DurationMs != 60000 {
			t.Errorf("duration_ms = %v, want 60000", r.DurationMs)
		}

		b := byID[bare]
		if b.Tokens != nil || b.CostUSD != nil || b.UserTurns != nil || b.DurationMs != nil {
			t.Errorf("missing cards must come back as null metrics, got %+v", b)
		}
		if b.Repo != nil || b.Branch != nil {
			t.Errorf("no git_info must give null repo/branch, got %v/%v", b.Repo, b.Branch)
		}
		if b.Title != "listable" {
			t.Errorf("title falls back to summary: got %q", b.Title)
		}
	})

	t.Run("the cap returns exactly limit with the full total, and the sample is stable", func(t *testing.T) {
		env.CleanDB(t)
		u := testutil.CreateTestUser(t, env, "a@test.com", "A")
		for i := range 12 {
			listableSession(t, env, u.ID, "cap"+string(rune('a'+i)), models.ProviderCodex, time.Date(2026, 3, 1+i, 0, 0, 0, 0, time.UTC))
		}
		f := analytics.SessionInsightsFilter{From: from, To: to}
		first, err := store.AdminSessionPoints(env.Ctx, f, 5)
		if err != nil {
			t.Fatalf("first: %v", err)
		}
		if first.Total != 12 || first.Returned != 5 || len(first.Points) != 5 {
			t.Fatalf("capped total/returned = %d/%d, want 12/5", first.Total, first.Returned)
		}
		second, err := store.AdminSessionPoints(env.Ctx, f, 5)
		if err != nil {
			t.Fatalf("second: %v", err)
		}
		for i := range first.Points {
			if first.Points[i].ID != second.Points[i].ID {
				t.Fatalf("sample must be stable across calls: %v vs %v", first.Points[i].ID, second.Points[i].ID)
			}
		}
	})

	t.Run("filters apply and an empty match returns a non-nil empty list", func(t *testing.T) {
		env.CleanDB(t)
		u := testutil.CreateTestUser(t, env, "a@test.com", "A")
		listableSession(t, env, u.ID, "x", models.ProviderCodex, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC))
		got, err := store.AdminSessionPoints(env.Ctx, analytics.SessionInsightsFilter{
			From: from, To: to, Providers: []string{models.ProviderCursor},
		}, 100)
		if err != nil {
			t.Fatalf("AdminSessionPoints: %v", err)
		}
		if got.Total != 0 || got.Points == nil || len(got.Points) != 0 {
			t.Errorf("no match = %+v, want total 0 and non-nil empty points", got)
		}
	})
}
