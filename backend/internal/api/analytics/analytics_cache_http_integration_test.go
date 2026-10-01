package analytics_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// Analytics cache behavior (5m68)
//
// GET /api/v1/sessions/{id}/analytics must persist computed cards no matter how
// long the compute takes or whether the client stays, dedupe concurrent
// computes per session, and serve current-version stale cards immediately
// while refreshing at most once per cooldown.
// =============================================================================

const (
	cacheTestLine1 = `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}`
	cacheTestLine2 = `{"type":"assistant","message":{"id":"msg_2","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Goodbye!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":200,"output_tokens":100}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}`
)

type cacheTestSession struct {
	env          *testutil.TestEnvironment
	userID       int64
	sessionID    string
	sessionToken string
	ts           *testutil.TestServer
	client       *testutil.TestClient
}

// newCacheTestSession seeds a one-line Claude session and starts a server.
func newCacheTestSession(t *testing.T, env *testutil.TestEnvironment) *cacheTestSession {
	t.Helper()
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "cache@example.com", "Cache User")
	token := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
	sessionID := testutil.CreateTestSession(t, env, user.ID, "cache-session")
	testutil.UploadTestChunk(t, env, user.ID, models.ProviderClaudeCode, "cache-session", "transcript.jsonl", 1, 1, []byte(cacheTestLine1+"\n"))
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 1)
	ts := setupTestServerWithEnv(t, env)
	return &cacheTestSession{
		env: env, userID: user.ID, sessionID: sessionID, sessionToken: token, ts: ts,
		client: testutil.NewTestClient(t, ts).WithSession(token),
	}
}

// growToTwoLines simulates the CLI syncing a second transcript line.
func (s *cacheTestSession) growToTwoLines(t *testing.T) {
	t.Helper()
	testutil.UploadTestChunk(t, s.env, s.userID, models.ProviderClaudeCode, "cache-session", "transcript.jsonl", 1, 2, []byte(cacheTestLine1+"\n"+cacheTestLine2+"\n"))
	testutil.CreateTestSyncFile(t, s.env, s.sessionID, "transcript.jsonl", "transcript", 2)
}

func (s *cacheTestSession) getAnalytics(t *testing.T) analytics.AnalyticsResponse {
	t.Helper()
	resp, err := s.client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", s.sessionID))
	if err != nil {
		t.Fatalf("GET analytics: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)
	var result analytics.AnalyticsResponse
	testutil.ParseJSON(t, resp, &result)
	return result
}

// waitForCachedCards polls the DB until every card is valid at lineCount.
func (s *cacheTestSession) waitForCachedCards(t *testing.T, lineCount int64) {
	t.Helper()
	store := analytics.NewStore(s.env.DB.Conn())
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		cards, err := store.GetCards(context.Background(), s.sessionID)
		if err == nil && cards.AllValid(lineCount) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("cards for session %s were never persisted at up_to_line=%d", s.sessionID, lineCount)
}

// computeCounter counts analytics computes; entered receives one signal per
// compute (buffered, non-blocking).
type computeCounter struct {
	count   atomic.Int64
	entered chan struct{}
}

// installComputeCounter installs a compute hook that counts computes and, when
// gate is non-nil, blocks each compute until gate is closed.
func installComputeCounter(t *testing.T, gate <-chan struct{}) *computeCounter {
	t.Helper()
	c := &computeCounter{entered: make(chan struct{}, 16)}
	restore := api.SetAnalyticsComputeHookForTest(func() {
		c.count.Add(1)
		select {
		case c.entered <- struct{}{}:
		default:
		}
		if gate != nil {
			<-gate
		}
	})
	t.Cleanup(restore)
	return c
}

func (c *computeCounter) waitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-c.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("compute never started")
	}
}

func TestGetSessionAnalytics_Cache_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	env := testutil.SetupTestEnvironment(t)

	t.Run("compute slower than DatabaseTimeout still persists cards and the next GET is a cache hit", func(t *testing.T) {
		s := newCacheTestSession(t, env)
		var computes atomic.Int64
		t.Cleanup(api.SetAnalyticsComputeHookForTest(func() {
			computes.Add(1)
			time.Sleep(api.DatabaseTimeout + 500*time.Millisecond)
		}))

		first := s.getAnalytics(t)
		if first.ComputedLines != 1 || first.Tokens.Input != 100 {
			t.Fatalf("first GET: computed_lines=%d input=%d, want 1/100", first.ComputedLines, first.Tokens.Input)
		}
		s.waitForCachedCards(t, 1)

		second := s.getAnalytics(t)
		if second.ComputedLines != 1 {
			t.Errorf("second GET computed_lines = %d, want 1", second.ComputedLines)
		}
		if got := computes.Load(); got != 1 {
			t.Errorf("computes = %d, want 1 (second GET must be a cache hit)", got)
		}
	})

	t.Run("concurrent GETs for an uncached session run exactly one compute", func(t *testing.T) {
		s := newCacheTestSession(t, env)
		gate := make(chan struct{})
		counter := installComputeCounter(t, gate)

		const n = 5
		var wg sync.WaitGroup
		results := make(chan string, n)
		for range n {
			wg.Go(func() {
				resp, err := s.client.Get(fmt.Sprintf("/api/v1/sessions/%s/analytics", s.sessionID))
				if err != nil {
					results <- "error: " + err.Error()
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					results <- fmt.Sprintf("status %d", resp.StatusCode)
					return
				}
				var r analytics.AnalyticsResponse
				testutil.ParseJSON(t, resp, &r)
				results <- fmt.Sprintf("lines=%d input=%d", r.ComputedLines, r.Tokens.Input)
			})
		}

		counter.waitEntered(t)
		time.Sleep(300 * time.Millisecond) // let the other requests join the in-flight compute
		close(gate)
		wg.Wait()
		close(results)

		for r := range results {
			if r != "lines=1 input=100" {
				t.Errorf("concurrent GET result = %q, want %q", r, "lines=1 input=100")
			}
		}
		if got := counter.count.Load(); got != 1 {
			t.Errorf("computes = %d, want exactly 1 for %d concurrent GETs", got, n)
		}
	})

	t.Run("client disconnecting mid-compute does not abandon the compute", func(t *testing.T) {
		s := newCacheTestSession(t, env)
		gate := make(chan struct{})
		counter := installComputeCounter(t, gate)

		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			fmt.Sprintf("%s/api/v1/sessions/%s/analytics", s.ts.URL, s.sessionID), nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: s.sessionToken})
		done := make(chan error, 1)
		go func() {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
			done <- err
		}()

		counter.waitEntered(t)
		cancel()
		if err := <-done; err == nil {
			t.Fatal("expected the cancelled request to fail client-side")
		}
		close(gate)

		s.waitForCachedCards(t, 1)
		if got := s.getAnalytics(t); got.ComputedLines != 1 || got.Tokens.Input != 100 {
			t.Errorf("follow-up GET: computed_lines=%d input=%d, want 1/100", got.ComputedLines, got.Tokens.Input)
		}
		if got := counter.count.Load(); got != 1 {
			t.Errorf("computes = %d, want 1 (follow-up GET must hit the cache the abandoned request filled)", got)
		}
	})

	t.Run("stale current-version cards are served without a refresh inside the cooldown", func(t *testing.T) {
		s := newCacheTestSession(t, env)
		counter := installComputeCounter(t, nil)

		s.getAnalytics(t) // compute + cache at line 1
		s.growToTwoLines(t)

		stale := s.getAnalytics(t)
		if stale.ComputedLines != 1 || stale.Tokens.Input != 100 {
			t.Errorf("stale GET: computed_lines=%d input=%d, want the cached 1/100", stale.ComputedLines, stale.Tokens.Input)
		}
		time.Sleep(300 * time.Millisecond)
		if got := counter.count.Load(); got != 1 {
			t.Errorf("computes = %d, want 1 (cards younger than the cooldown must not be refreshed)", got)
		}
	})

	t.Run("stale cards older than the cooldown are served immediately and refreshed in the background", func(t *testing.T) {
		s := newCacheTestSession(t, env)
		gate := make(chan struct{})
		var gateOpen atomic.Bool
		counter := &computeCounter{entered: make(chan struct{}, 16)}
		t.Cleanup(api.SetAnalyticsComputeHookForTest(func() {
			counter.count.Add(1)
			if gateOpen.Load() {
				return
			}
			counter.entered <- struct{}{}
			<-gate
		}))
		t.Cleanup(api.SetAnalyticsRefreshCooldownForTest(0))

		gateOpen.Store(true)
		s.getAnalytics(t) // initial compute, not gated
		gateOpen.Store(false)
		s.growToTwoLines(t)

		// The refresh compute is blocked on the gate, yet the GET returns
		// the stale cards right away.
		stale := s.getAnalytics(t)
		if stale.ComputedLines != 1 || stale.Tokens.Input != 100 {
			t.Errorf("stale GET: computed_lines=%d input=%d, want the cached 1/100", stale.ComputedLines, stale.Tokens.Input)
		}
		counter.waitEntered(t)
		close(gate)

		s.waitForCachedCards(t, 2)
		fresh := s.getAnalytics(t)
		if fresh.ComputedLines != 2 || fresh.Tokens.Input != 300 {
			t.Errorf("post-refresh GET: computed_lines=%d input=%d, want 2/300", fresh.ComputedLines, fresh.Tokens.Input)
		}
		if got := counter.count.Load(); got != 2 {
			t.Errorf("computes = %d, want 2 (initial + one background refresh)", got)
		}
	})

	for _, tc := range []struct {
		name   string
		mutate string
	}{
		{"version-mismatched cards are not served stale", fmt.Sprintf(`UPDATE session_card_redactions SET version = %d WHERE session_id = $1`, analytics.RedactionsCardVersion-1)},
		{"a missing card is not served stale", `DELETE FROM session_card_workflows WHERE session_id = $1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newCacheTestSession(t, env)
			counter := installComputeCounter(t, nil)

			s.getAnalytics(t)
			s.growToTwoLines(t)
			if _, err := env.DB.Conn().ExecContext(context.Background(), tc.mutate, s.sessionID); err != nil {
				t.Fatalf("mutate cards: %v", err)
			}

			got := s.getAnalytics(t)
			if got.ComputedLines != 2 || got.Tokens.Input != 300 {
				t.Errorf("GET: computed_lines=%d input=%d, want freshly computed 2/300", got.ComputedLines, got.Tokens.Input)
			}
			if n := counter.count.Load(); n != 2 {
				t.Errorf("computes = %d, want 2 (the request must block on a fresh compute)", n)
			}
		})
	}
}
