package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/ConfabulousDev/confab-web/internal/clientip"
)

// allowAllLimiter is a test double that returns a fixed Allow result.
type allowAllLimiter struct{ allowed bool }

func (l *allowAllLimiter) Allow(_ context.Context, _ string) bool         { return l.allowed }
func (l *allowAllLimiter) AllowN(_ context.Context, _ string, _ int) bool { return l.allowed }

// captureKeyLimiter records the most recent key passed to Allow/AllowN.
type captureKeyLimiter struct {
	lastKey string
	allow   bool
}

func (l *captureKeyLimiter) Allow(_ context.Context, key string) bool {
	l.lastKey = key
	return l.allow
}
func (l *captureKeyLimiter) AllowN(_ context.Context, key string, _ int) bool {
	l.lastKey = key
	return l.allow
}

func TestAllow_AllowsWithinBurst(t *testing.T) {
	rl := NewInMemoryRateLimiter(1, 5, 1000) // 1 req/sec, burst 5
	defer rl.Stop()

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if !rl.Allow(ctx, "client-a") {
			t.Errorf("request %d should be allowed within burst", i+1)
		}
	}
}

func TestAllow_BlocksAfterBurstExhausted(t *testing.T) {
	rl := NewInMemoryRateLimiter(0.0001, 2, 1000) // effectively no refill in test window
	defer rl.Stop()

	ctx := context.Background()
	if !rl.Allow(ctx, "client-b") {
		t.Fatal("first request should pass")
	}
	if !rl.Allow(ctx, "client-b") {
		t.Fatal("second request should pass (burst=2)")
	}
	if rl.Allow(ctx, "client-b") {
		t.Error("third request should be denied — burst exhausted, no refill yet")
	}
}

func TestAllowN_RespectsBurst(t *testing.T) {
	rl := NewInMemoryRateLimiter(0.0001, 5, 1000)
	defer rl.Stop()

	ctx := context.Background()
	if !rl.AllowN(ctx, "client-c", 5) {
		t.Error("AllowN(5) with burst=5 should pass")
	}
	if rl.AllowN(ctx, "client-c", 1) {
		t.Error("AllowN(1) after exhausting burst should fail")
	}
}

func TestPerKeyIsolation(t *testing.T) {
	rl := NewInMemoryRateLimiter(0.0001, 1, 1000)
	defer rl.Stop()

	ctx := context.Background()
	if !rl.Allow(ctx, "alice") {
		t.Fatal("alice's first request should pass")
	}
	if !rl.Allow(ctx, "bob") {
		t.Error("bob's first request should pass — keys are independent")
	}
	if rl.Allow(ctx, "alice") {
		t.Error("alice's second request should be denied")
	}
}

func TestGetLimiter_ConcurrentLoadOrStore(t *testing.T) {
	// Verify the LoadOrStore race in getLimiter is handled — many goroutines
	// hitting the same key must all observe the same *rate.Limiter.
	rl := NewInMemoryRateLimiter(10, 10, 1000)
	defer rl.Stop()

	const n = 50
	results := make(chan *rate.Limiter, n)
	for i := 0; i < n; i++ {
		go func() {
			results <- rl.getLimiter("shared")
		}()
	}

	var first *rate.Limiter
	for i := 0; i < n; i++ {
		got := <-results
		if first == nil {
			first = got
			continue
		}
		if got != first {
			t.Errorf("getLimiter returned distinct *rate.Limiter values for same key")
		}
	}
}

func TestCleanupOldLimiters_EvictsStale(t *testing.T) {
	rl := NewInMemoryRateLimiter(1, 1, 1000)
	defer rl.Stop()
	rl.maxAge = 50 * time.Millisecond

	ctx := context.Background()
	rl.Allow(ctx, "stale")
	rl.Allow(ctx, "fresh")

	// Manually backdate stale entry so it qualifies for eviction.
	rl.lastAccess.Store("stale", time.Now().UTC().Add(-1*time.Hour))

	rl.cleanupOldLimiters()

	if _, ok := rl.limiters.Load("stale"); ok {
		t.Error("stale key should have been evicted from limiters")
	}
	if _, ok := rl.lastAccess.Load("stale"); ok {
		t.Error("stale key should have been evicted from lastAccess")
	}
	if _, ok := rl.limiters.Load("fresh"); !ok {
		t.Error("fresh key should not have been evicted")
	}
}

func TestCleanupOldLimiters_NoOpWhenAllFresh(t *testing.T) {
	rl := NewInMemoryRateLimiter(1, 1, 1000)
	defer rl.Stop()
	rl.maxAge = 1 * time.Hour

	ctx := context.Background()
	rl.Allow(ctx, "a")
	rl.Allow(ctx, "b")

	rl.cleanupOldLimiters()
	if _, ok := rl.limiters.Load("a"); !ok {
		t.Error("a should not be evicted")
	}
	if _, ok := rl.limiters.Load("b"); !ok {
		t.Error("b should not be evicted")
	}
}

// ---------- bucket cap (F3) ----------

// TestBucketCap_EvictsOldest fills the limiter to its maxBuckets cap, then
// inserts one more key and asserts the least-recently-used key is evicted.
func TestBucketCap_EvictsOldest(t *testing.T) {
	rl := NewInMemoryRateLimiter(1, 1, 5)
	defer rl.Stop()
	rl.maxAge = time.Hour // keep the background cleanup from interfering

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		rl.Allow(ctx, fmt.Sprintf("key-%d", i))
	}
	// Make key-0 the unambiguously oldest by last access.
	rl.lastAccess.Store("key-0", time.Now().UTC().Add(-1*time.Hour))

	// This insert exceeds the cap and must evict the oldest entry first.
	rl.Allow(ctx, "key-new")

	if _, ok := rl.limiters.Load("key-0"); ok {
		t.Error("oldest key (key-0) should have been evicted when cap exceeded")
	}
	if _, ok := rl.lastAccess.Load("key-0"); ok {
		t.Error("oldest key (key-0) should have been removed from lastAccess too")
	}
}

// TestBucketCap_NewKeyAdmittedAfterEviction verifies that a new key inserted at
// the cap boundary is admitted and gets a functioning limiter.
func TestBucketCap_NewKeyAdmittedAfterEviction(t *testing.T) {
	rl := NewInMemoryRateLimiter(1, 1, 5)
	defer rl.Stop()
	rl.maxAge = time.Hour

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		rl.Allow(ctx, fmt.Sprintf("key-%d", i))
	}

	if !rl.Allow(ctx, "key-new") {
		t.Error("new key admitted at cap boundary should be allowed (fresh burst)")
	}
	if _, ok := rl.limiters.Load("key-new"); !ok {
		t.Error("new key should have a limiter stored after admission")
	}
}

// TestBucketCounter_DecrementOnCleanup verifies the live bucket counter is
// decremented when cleanupOldLimiters evicts stale keys.
func TestBucketCounter_DecrementOnCleanup(t *testing.T) {
	rl := NewInMemoryRateLimiter(1, 1, 1000)
	defer rl.Stop()
	rl.maxAge = 50 * time.Millisecond

	ctx := context.Background()
	rl.Allow(ctx, "stale")
	rl.Allow(ctx, "fresh")
	if got := rl.bucketCount.Load(); got != 2 {
		t.Fatalf("bucketCount = %d, want 2 after two inserts", got)
	}

	rl.lastAccess.Store("stale", time.Now().UTC().Add(-1*time.Hour))
	rl.cleanupOldLimiters()

	if got := rl.bucketCount.Load(); got != 1 {
		t.Errorf("bucketCount = %d, want 1 after cleanup evicted one stale key", got)
	}
}

// TestBucketCap_SequentialOverflowStaysBounded inserts far more distinct keys
// than the cap, one at a time (the shape of a rotating-IP attacker on a single
// connection). The map must stay tightly bounded near the cap — this is the
// core anti-OOM guarantee.
func TestBucketCap_SequentialOverflowStaysBounded(t *testing.T) {
	const cap = 50
	rl := NewInMemoryRateLimiter(1, 1, cap)
	defer rl.Stop()
	rl.maxAge = time.Hour // isolate from background cleanup

	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		rl.Allow(ctx, fmt.Sprintf("attacker-%d", i))
	}

	count := 0
	rl.limiters.Range(func(_, _ any) bool { count++; return true })
	// Each over-cap insert evicts one and adds one, so the steady state hugs
	// the cap. Allow one slot of slack for the in-flight new key.
	if count < 1 || count > cap+1 {
		t.Errorf("map size = %d after 1000 sequential keys, want within [1, %d]", count, cap+1)
	}
}

// TestBucketCounter_Soft drives many unique keys concurrently against a small
// cap. The cap is a soft bound, so under a thundering herd the map may briefly
// overshoot; the guarantees we assert are thread-safety (no panic/race — run
// with -race), no phantom keys beyond what was inserted, and that the limiter
// remains usable afterward.
func TestBucketCounter_Soft(t *testing.T) {
	rl := NewInMemoryRateLimiter(10, 10, 10)
	defer rl.Stop()
	rl.maxAge = time.Hour

	ctx := context.Background()
	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rl.Allow(ctx, fmt.Sprintf("k-%d", i))
		}(i)
	}
	wg.Wait()

	count := 0
	rl.limiters.Range(func(_, _ any) bool { count++; return true })
	if count > n {
		t.Errorf("limiters map size = %d exceeds the %d distinct keys inserted (phantom entries)", count, n)
	}
	if !rl.Allow(ctx, "after-burst") {
		t.Error("limiter should remain usable after a concurrent burst")
	}
}

func TestStop_TerminatesCleanupGoroutine(t *testing.T) {
	// Smoke test that Stop returns and doesn't deadlock. Without proper
	// channel handling the cleanup goroutine could block forever.
	rl := NewInMemoryRateLimiter(1, 1, 1000)
	done := make(chan struct{})
	go func() {
		rl.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within 2s")
	}
}

// ---------- middleware ----------

func TestMiddleware_Allowed(t *testing.T) {
	mw := Middleware(&allowAllLimiter{allowed: true})
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	handler.ServeHTTP(rec, req)
	if !called {
		t.Error("inner handler should be called when limiter allows")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestMiddleware_Denied(t *testing.T) {
	mw := Middleware(&allowAllLimiter{allowed: false})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler must not be called when denied")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
}

func TestMiddlewareWithKey_UsesCustomKeyAndFallsBackToIP(t *testing.T) {
	keyFunc := func(r *http.Request) string {
		if v := r.Header.Get("X-User"); v != "" {
			return v
		}
		return ""
	}

	captureLimiter := &captureKeyLimiter{allow: true}
	mw := MiddlewareWithKey(captureLimiter, keyFunc)
	// clientip.NewMiddleware populates the context that MiddlewareWithKey reads
	// from for the IP fallback.
	handler := clientip.NewMiddleware(nil)(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	// Case 1: custom key present.
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.42:5555"
	req.Header.Set("X-User", "user-123")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if captureLimiter.lastKey != "user-123" {
		t.Errorf("expected custom key, got %q", captureLimiter.lastKey)
	}

	// Case 2: custom key empty → fall back to IP-based key.
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.0.0.42:5555"
	handler.ServeHTTP(httptest.NewRecorder(), req2)
	if captureLimiter.lastKey == "" {
		t.Error("fallback key should be non-empty (clientip.FromRequest)")
	}
	if captureLimiter.lastKey == "user-123" {
		t.Errorf("fallback key should differ from custom key, got %q", captureLimiter.lastKey)
	}
}

func TestHandlerFunc_AppliesLimit(t *testing.T) {
	captureLimiter := &captureKeyLimiter{allow: false}
	called := false
	wrapped := HandlerFunc(captureLimiter, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest("GET", "/x", nil))
	if called {
		t.Error("handler should not be called when limit denies")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}

	captureLimiter.allow = true
	rec = httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest("GET", "/x", nil))
	if !called {
		t.Error("handler should be called when limit allows")
	}
}

func TestUserKeyFunc(t *testing.T) {
	type userIDKey struct{}
	kf := UserKeyFunc(userIDKey{})

	cases := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "user ID present",
			ctx:  context.WithValue(context.Background(), userIDKey{}, int64(99)),
			want: "user:99",
		},
		{
			name: "user ID missing",
			ctx:  context.Background(),
			want: "",
		},
		{
			name: "wrong type",
			ctx:  context.WithValue(context.Background(), userIDKey{}, "not-int64"),
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil).WithContext(c.ctx)
			if got := kf(req); got != c.want {
				t.Errorf("kf() = %q, want %q", got, c.want)
			}
		})
	}
}
