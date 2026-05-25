package org_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/shopspring/decimal"
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

// =============================================================================
// CF-495: ?owner= filter, filter_options surfacing, demo viewer
//
// Wire contract for the new owner narrowing. Privacy invariant test does a
// full deep-equal against a zeroed-out response fixture so a "leak through
// a single card" regression class is caught — not just a session_count
// mismatch.
// =============================================================================

// decodeTrends decodes a /api/v1/trends 200 response into the canonical type.
func decodeTrends(t *testing.T, resp *http.Response) *analytics.TrendsResponse {
	t.Helper()
	var out analytics.TrendsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode trends response: %v", err)
	}
	return &out
}

// assertTrendsResponseZeroed asserts every observable signal in the response
// is at zero — session_count, all card totals, all per_provider maps, all
// top sessions, providers_present. Per-day arrays may have anchor rows from
// the date_range LEFT JOIN (one row per day in range with empty per_provider
// + zero numerics); those anchors are NOT a leak — we walk them and assert
// each inner bucket is empty.
//
// disallowedOwner is the owner email the caller asked for; if it leaks into
// filter_options.owners the test fails (privacy invariant). filter_options
// is the visible-set view, so it CAN contain the viewer's own email — pass
// the *other* owner here.
func assertTrendsResponseZeroed(t *testing.T, got *analytics.TrendsResponse, disallowedOwner string) {
	t.Helper()

	if got.SessionCount != 0 {
		t.Errorf("session_count = %d, want 0", got.SessionCount)
	}
	if len(got.ProvidersPresent) != 0 {
		t.Errorf("providers_present = %v, want []", got.ProvidersPresent)
	}
	for _, o := range got.FilterOptions.Owners {
		if o == disallowedOwner {
			t.Errorf("filter_options.owners leaked %q (caller has no access)", disallowedOwner)
		}
	}

	if c := got.Cards.Overview; c == nil {
		t.Error("Overview card is nil")
	} else if c.SessionCount != 0 || c.TotalDurationMs != 0 || c.DaysCovered != 0 || c.TotalAssistantDurationMs != 0 {
		t.Errorf("Overview leaked non-zero values: %+v", c)
	}

	if c := got.Cards.Tokens; c == nil {
		t.Error("Tokens card is nil")
	} else {
		if c.TotalInputTokens != 0 || c.TotalOutputTokens != 0 || c.TotalCacheCreationTokens != 0 || c.TotalCacheReadTokens != 0 {
			t.Errorf("Tokens totals leaked: %+v", c)
		}
		if c.TotalCostUSD != "0" {
			t.Errorf("Tokens.TotalCostUSD = %q, want \"0\"", c.TotalCostUSD)
		}
		if len(c.PerProvider) != 0 {
			t.Errorf("Tokens.PerProvider leaked: %+v", c.PerProvider)
		}
		for _, p := range c.DailyCosts {
			if p.CostUSD != "0" || len(p.PerProvider) != 0 {
				t.Errorf("DailyCosts[%s] leaked: cost=%q per_provider=%v", p.Date, p.CostUSD, p.PerProvider)
			}
		}
	}

	if c := got.Cards.Activity; c == nil {
		t.Error("Activity card is nil")
	} else {
		if c.TotalFilesRead != 0 || c.TotalFilesModified != 0 || c.TotalLinesAdded != 0 || c.TotalLinesRemoved != 0 {
			t.Errorf("Activity totals leaked: %+v", c)
		}
		for _, p := range c.DailySessionCounts {
			if p.SessionCount != 0 || len(p.PerProvider) != 0 {
				t.Errorf("DailySessionCounts[%s] leaked: count=%d per_provider=%v", p.Date, p.SessionCount, p.PerProvider)
			}
		}
	}

	if c := got.Cards.Tools; c == nil {
		t.Error("Tools card is nil")
	} else if c.TotalCalls != 0 || c.TotalErrors != 0 || len(c.ToolStats) != 0 {
		t.Errorf("Tools leaked: %+v", c)
	}

	if c := got.Cards.Utilization; c == nil {
		t.Error("Utilization card is nil")
	} else {
		for _, p := range c.DailyUtilization {
			if p.UtilizationPct != nil {
				t.Errorf("DailyUtilization[%s] leaked non-nil utilization", p.Date)
			}
		}
	}

	if c := got.Cards.AgentsAndSkills; c == nil {
		t.Error("AgentsAndSkills card is nil")
	} else if c.TotalAgentInvocations != 0 || c.TotalSkillInvocations != 0 || len(c.AgentStats) != 0 || len(c.SkillStats) != 0 {
		t.Errorf("AgentsAndSkills leaked: %+v", c)
	}

	if c := got.Cards.TopSessions; c == nil {
		t.Error("TopSessions card is nil")
	} else if len(c.Sessions) != 0 {
		t.Errorf("TopSessions leaked %d entries", len(c.Sessions))
	}
}

// TestHandleGetTrends_OwnerPrivacyInvariant — load-bearing.
// Bob has no access to alice's session (no share, share-all OFF). With
// ?owner=alice@vis.test, every card must be zeroed and providers_present
// + filter_options.owners must omit alice. Full-shape comparison catches
// any single-card leak.
func TestHandleGetTrends_OwnerPrivacyInvariant(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice-priv@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob-priv@vis.test", "Bob")

	// Alice has a session. Bob has none. No shares. Share-all is off.
	_ = testutil.CreateTestSessionFull(t, env, alice.ID, "alice-priv-sess", testutil.TestSessionFullOpts{Summary: "x"})

	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, bob.ID)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	resp, err := client.Get("/api/v1/trends?owner=" + alice.Email)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	got := decodeTrends(t, resp)
	assertTrendsResponseZeroed(t, got, alice.Email)
}

// TestHandleGetTrends_OwnerWithPrivateShare — bob granted private share by
// alice can see alice's sessions when filtering ?owner=alice.
func TestHandleGetTrends_OwnerWithPrivateShare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice-pshare@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob-pshare@vis.test", "Bob")

	aliceSess := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-pshare-sess", testutil.TestSessionFullOpts{Summary: "x"})
	testutil.CreateTestShare(t, env, aliceSess, false, nil, []string{bob.Email})

	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, bob.ID)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	resp, err := client.Get("/api/v1/trends?owner=" + alice.Email)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	got := decodeTrends(t, resp)
	if got.SessionCount != 1 {
		t.Errorf("session_count = %d, want 1", got.SessionCount)
	}
}

// TestHandleGetTrends_OwnerWithSystemShare — same as above via system share.
func TestHandleGetTrends_OwnerWithSystemShare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice-sshare@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob-sshare@vis.test", "Bob")

	aliceSess := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-sshare-sess", testutil.TestSessionFullOpts{Summary: "x"})
	testutil.CreateTestSystemShare(t, env, aliceSess, nil)

	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, bob.ID)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	resp, err := client.Get("/api/v1/trends?owner=" + alice.Email)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	got := decodeTrends(t, resp)
	if got.SessionCount != 1 {
		t.Errorf("session_count = %d, want 1", got.SessionCount)
	}
}

// TestHandleGetTrends_ShareAllNoOwnerReturnsAggregate — under share-all,
// bare /api/v1/trends (no owner filter) returns the aggregate across all
// org sessions.
func TestHandleGetTrends_ShareAllNoOwnerReturnsAggregate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	alice := testutil.CreateTestUser(t, env, "alice-sa@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob-sa@vis.test", "Bob")
	charlie := testutil.CreateTestUser(t, env, "charlie-sa@vis.test", "Charlie")

	_ = testutil.CreateTestSessionFull(t, env, alice.ID, "sa-alice", testutil.TestSessionFullOpts{Summary: "x"})
	_ = testutil.CreateTestSessionFull(t, env, bob.ID, "sa-bob", testutil.TestSessionFullOpts{Summary: "x"})
	_ = testutil.CreateTestSessionFull(t, env, charlie.ID, "sa-charlie", testutil.TestSessionFullOpts{Summary: "x"})

	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, bob.ID)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	resp, err := client.Get("/api/v1/trends")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	got := decodeTrends(t, resp)
	if got.SessionCount != 3 {
		t.Errorf("share-all aggregate session_count = %d, want 3", got.SessionCount)
	}
	gotSorted := append([]string{}, got.FilterOptions.Owners...)
	sort.Strings(gotSorted)
	want := []string{"alice-sa@vis.test", "bob-sa@vis.test", "charlie-sa@vis.test"}
	if len(gotSorted) != 3 || gotSorted[0] != want[0] || gotSorted[1] != want[1] || gotSorted[2] != want[2] {
		t.Errorf("filter_options.owners = %v, want %v", got.FilterOptions.Owners, want)
	}
}

// TestHandleGetTrends_DemoViewerBareReturnsAggregate — demo viewer with
// share-all on, no ?owner=, gets the org-wide aggregate.
func TestHandleGetTrends_DemoViewerBareReturnsAggregate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	const demoEmailLocal = "demo-trends@confabulous.dev"
	const csrfSecret = "test-csrf-secret-key-32-bytes!!"
	testutil.SetEnvForTest(t, "DEMO_IDENTITY_EMAIL", demoEmailLocal)
	testutil.SetEnvForTest(t, "CSRF_SECRET_KEY", csrfSecret)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	// Bootstrap the demo identity (creates the read_only user + shared cookie row).
	if err := auth.BootstrapDemoIdentity(context.Background(), env.DB, demoEmailLocal, csrfSecret); err != nil {
		t.Fatalf("BootstrapDemoIdentity: %v", err)
	}

	alice := testutil.CreateTestUser(t, env, "alice-demo@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob-demo@vis.test", "Bob")
	_ = testutil.CreateTestSessionFull(t, env, alice.ID, "demo-alice", testutil.TestSessionFullOpts{Summary: "x"})
	_ = testutil.CreateTestSessionFull(t, env, bob.ID, "demo-bob", testutil.TestSessionFullOpts{Summary: "x"})

	ts := setupTestServerWithEnv(t, env)
	// No session token → auto-impersonate via the demo cookie.
	cookieID := auth.DemoSessionCookieID(csrfSecret, demoEmailLocal)
	client := testutil.NewTestClient(t, ts).WithSession(cookieID)

	resp, err := client.Get("/api/v1/trends")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	got := decodeTrends(t, resp)
	if got.SessionCount != 2 {
		t.Errorf("demo viewer bare /trends session_count = %d, want 2 (alice + bob)", got.SessionCount)
	}
}

// TestHandleGetTrends_RevokedShareNoLongerAccessible — viewer who used to
// have access via a share that has since expired gets zero rows for
// ?owner=<that-owner>. Catches a TOCTOU regression class.
func TestHandleGetTrends_RevokedShareNoLongerAccessible(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice-rev@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob-rev@vis.test", "Bob")

	aliceSess := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-rev-sess", testutil.TestSessionFullOpts{Summary: "x"})
	shareID := testutil.CreateTestShare(t, env, aliceSess, false, nil, []string{bob.Email})
	// Move expires_at into the past.
	if _, err := env.DB.Exec(env.Ctx, `UPDATE session_shares SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, shareID); err != nil {
		t.Fatalf("expire share: %v", err)
	}

	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, bob.ID)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	resp, err := client.Get("/api/v1/trends?owner=" + alice.Email)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	got := decodeTrends(t, resp)
	if got.SessionCount != 0 {
		t.Errorf("revoked share: session_count = %d, want 0", got.SessionCount)
	}
}

// TestHandleGetTrends_EmptyReposEqualsAllRepos (CF-506) pins the new wire
// contract: GET /api/v1/trends with no `repos` param returns the same
// aggregates as GET ...&repos=<every-available-repo>. With this contract the
// frontend can stop auto-stuffing every repo into the URL on mount.
func TestHandleGetTrends_EmptyReposEqualsAllRepos(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "cf506-trends@vis.test", "CF-506 Trends")
	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

	store := analytics.NewStore(env.DB.Conn())
	now := time.Now().UTC()
	// seed creates a session (with or without git_info, per repoURL) and the
	// tokens + conversation cards it needs to qualify for the trends aggregate.
	seed := func(externalID, repoURL string, inputTokens int64, cost float64) {
		t.Helper()
		var sid string
		if repoURL == "" {
			sid = testutil.CreateTestSession(t, env, user.ID, externalID)
		} else {
			sid = testutil.CreateTestSessionWithGitInfo(t, env, user.ID, externalID, repoURL)
		}
		assistantMs := int64(10000)
		userMs := int64(20000)
		err := store.UpsertCards(env.Ctx, &analytics.Cards{
			Tokens: &analytics.TokensCardRecord{
				SessionID:        sid,
				Version:          analytics.TokensCardVersion,
				ComputedAt:       now,
				UpToLine:         100,
				InputTokens:      inputTokens,
				EstimatedCostUSD: decimal.NewFromFloat(cost),
			},
			Conversation: &analytics.ConversationCardRecord{
				SessionID:                sid,
				Version:                  analytics.ConversationCardVersion,
				ComputedAt:               now,
				UpToLine:                 100,
				TotalAssistantDurationMs: &assistantMs,
				TotalUserDurationMs:      &userMs,
			},
		})
		if err != nil {
			t.Fatalf("UpsertCards (%s): %v", sid, err)
		}
	}
	seed("cf506-tr-a", "https://github.com/ConfabulousDev/confab-web.git", 1000, 2.00)
	seed("cf506-tr-b", "git@github.com:ConfabulousDev/cli.git", 2000, 3.00)
	seed("cf506-tr-norepo", "", 3000, 5.00)

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	trendsURL := func(extra url.Values) string {
		v := url.Values{}
		v.Set("start_ts", fmt.Sprintf("%d", now.Add(-7*24*time.Hour).Unix()))
		v.Set("end_ts", fmt.Sprintf("%d", now.Add(24*time.Hour).Unix()))
		v.Set("tz_offset", "0")
		v.Set("include_no_repo", "true")
		for k, vals := range extra {
			for _, vv := range vals {
				v.Add(k, vv)
			}
		}
		return "/api/v1/trends?" + v.Encode()
	}

	get := func(extra url.Values) *analytics.TrendsResponse {
		t.Helper()
		resp, err := client.Get(trendsURL(extra))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)
		return decodeTrends(t, resp)
	}

	empty := get(url.Values{})
	explicit := get(url.Values{"repos": {"ConfabulousDev/confab-web,ConfabulousDev/cli"}})

	if empty.SessionCount != explicit.SessionCount {
		t.Errorf("session_count empty=%d explicit=%d, want equal", empty.SessionCount, explicit.SessionCount)
	}
	if empty.Cards.Tokens == nil || explicit.Cards.Tokens == nil {
		t.Fatalf("Tokens card nil — empty=%v explicit=%v", empty.Cards.Tokens, explicit.Cards.Tokens)
	}
	if empty.Cards.Tokens.TotalCostUSD != explicit.Cards.Tokens.TotalCostUSD {
		t.Errorf("tokens.total_cost_usd empty=%s explicit=%s, want equal",
			empty.Cards.Tokens.TotalCostUSD, explicit.Cards.Tokens.TotalCostUSD)
	}
	if len(empty.ProvidersPresent) != len(explicit.ProvidersPresent) {
		t.Errorf("providers_present empty=%v explicit=%v, want equal length",
			empty.ProvidersPresent, explicit.ProvidersPresent)
	} else {
		for i := range empty.ProvidersPresent {
			if empty.ProvidersPresent[i] != explicit.ProvidersPresent[i] {
				t.Errorf("providers_present[%d] empty=%q explicit=%q",
					i, empty.ProvidersPresent[i], explicit.ProvidersPresent[i])
			}
		}
	}
	// Sanity: 3 sessions seeded, all should be aggregated.
	if empty.SessionCount != 3 {
		t.Errorf("session_count = %d, want 3 (the seed has 3 sessions)", empty.SessionCount)
	}
	if empty.Cards.Tokens.TotalInputTokens != 6000 {
		t.Errorf("tokens.total_input_tokens = %d, want 6000 (1000+2000+3000)",
			empty.Cards.Tokens.TotalInputTokens)
	}
}

// TestHandleGetTrends_OwnerValidationBounds — ?owner= filter goes through
// ValidateFilterValues so the 50-value cap is enforced.
func TestHandleGetTrends_OwnerValidationBounds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "valbounds@vis.test", "ValBounds")
	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	// 51 owners — exceeds MaxFilterCount (50).
	tooMany := ""
	for i := 0; i < 51; i++ {
		if i > 0 {
			tooMany += ","
		}
		tooMany += "u" + strconv.Itoa(i) + "@x.test"
	}
	resp, err := client.Get("/api/v1/trends?owner=" + tooMany)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusBadRequest)
}
