package ratelimit

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter defines the interface for rate limiting
// This abstraction allows swapping between in-memory and distributed (Redis) implementations
type RateLimiter interface {
	// Allow checks if a request from the given key (IP, user ID, etc.) is allowed
	// Returns true if allowed, false if rate limit exceeded
	Allow(ctx context.Context, key string) bool

	// AllowN checks if N requests from the given key are allowed
	AllowN(ctx context.Context, key string, n int) bool
}

// InMemoryRateLimiter implements rate limiting using in-memory token buckets
// This is suitable for single-instance deployments
// For multi-instance deployments, use RedisRateLimiter instead
type InMemoryRateLimiter struct {
	// Rate is requests per second
	rate rate.Limit

	// Burst is the maximum burst size
	burst int

	// limiters stores per-key rate limiters
	limiters sync.Map // map[string]*rate.Limiter

	// cleanupInterval is how often to clean up old limiters
	cleanupInterval time.Duration

	// maxAge is how long to keep inactive limiters
	maxAge time.Duration

	// lastAccess tracks when each limiter was last used
	lastAccess sync.Map // map[string]time.Time

	// maxBuckets caps the number of live per-key limiters. When the cap is
	// reached, the oldest entries are evicted to admit a new key. This bounds
	// memory against a fast-rotating-IP attacker who would otherwise grow the
	// bucket map unboundedly within the cleanup window.
	maxBuckets int

	// bucketCount is a soft (advisory) count of live limiters. It may briefly
	// overshoot maxBuckets under concurrent inserts or drift slightly from the
	// true map size; the cap is a bound, not a hard invariant.
	bucketCount atomic.Int64

	// stopCleanup signals cleanup goroutine to stop
	stopCleanup chan struct{}
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
// rps: requests per second (e.g., 10 for 10 req/sec)
// burst: maximum burst size (e.g., 20 to allow bursts up to 20 requests)
// maxBuckets: cap on the number of live per-key limiters; once reached, the
// oldest entries are evicted to admit new keys (bounds memory under a
// rotating-key attack). A value <= 0 disables the cap.
func NewInMemoryRateLimiter(rps float64, burst int, maxBuckets int) *InMemoryRateLimiter {
	limiter := &InMemoryRateLimiter{
		rate:            rate.Limit(rps),
		burst:           burst,
		maxBuckets:      maxBuckets,
		cleanupInterval: 5 * time.Minute,
		maxAge:          10 * time.Minute,
		stopCleanup:     make(chan struct{}),
	}

	go limiter.cleanup()

	return limiter
}

// Allow checks if a single request is allowed
func (l *InMemoryRateLimiter) Allow(ctx context.Context, key string) bool {
	return l.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (l *InMemoryRateLimiter) AllowN(ctx context.Context, key string, n int) bool {
	limiter := l.getLimiter(key)
	now := time.Now().UTC()
	l.lastAccess.Store(key, now)
	return limiter.AllowN(now, n)
}

// getLimiter gets or creates a rate limiter for the given key
func (l *InMemoryRateLimiter) getLimiter(key string) *rate.Limiter {
	// Fast path: limiter already exists
	if v, ok := l.limiters.Load(key); ok {
		return v.(*rate.Limiter)
	}

	// At capacity: evict the oldest entries before admitting a new key so the
	// bucket map stays bounded. The check is best-effort (the count is a soft
	// bound), which is sufficient to prevent unbounded growth.
	if l.maxBuckets > 0 && l.bucketCount.Load() >= int64(l.maxBuckets) {
		l.evictOldest()
	}

	// Slow path: create and race to store
	limiter := rate.NewLimiter(l.rate, l.burst)
	actual, loaded := l.limiters.LoadOrStore(key, limiter)
	if loaded {
		return actual.(*rate.Limiter)
	}
	l.lastAccess.Store(key, time.Now().UTC())
	l.bucketCount.Add(1)
	return limiter
}

// evictOldest removes the least-recently-accessed limiters to make room when
// the bucket cap is reached. It evicts a small batch (~1% of the cap, at least
// one) so a steady stream of new keys doesn't trigger a full scan on every
// insert. Attacker-rotated stale keys are evicted before active ones.
func (l *InMemoryRateLimiter) evictOldest() {
	type entry struct {
		key  string
		when time.Time
	}
	var entries []entry
	l.lastAccess.Range(func(key, value interface{}) bool {
		entries = append(entries, entry{key.(string), value.(time.Time)})
		return true
	})
	if len(entries) == 0 {
		return
	}

	batch := l.maxBuckets / 100
	if batch < 1 {
		batch = 1
	}
	if batch > len(entries) {
		batch = len(entries)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].when.Before(entries[j].when)
	})

	for _, e := range entries[:batch] {
		// Decrement only when this call actually removed the bucket, so a key
		// concurrently evicted by cleanupOldLimiters isn't counted twice (which
		// would drift bucketCount negative and weaken the cap over time).
		if _, removed := l.limiters.LoadAndDelete(e.key); removed {
			l.bucketCount.Add(-1)
		}
		l.lastAccess.Delete(e.key)
	}

	slog.Warn("rate limiter bucket cap reached; evicted oldest entries",
		"evicted", batch, "max_buckets", l.maxBuckets)
}

// cleanup periodically removes old limiters to prevent memory leaks
func (l *InMemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanupOldLimiters()
		case <-l.stopCleanup:
			return
		}
	}
}

// cleanupOldLimiters removes limiters that haven't been used recently
func (l *InMemoryRateLimiter) cleanupOldLimiters() {
	cutoff := time.Now().UTC().Add(-l.maxAge)
	var keysToDelete []string

	l.lastAccess.Range(func(key, value interface{}) bool {
		if value.(time.Time).Before(cutoff) {
			keysToDelete = append(keysToDelete, key.(string))
		}
		return true
	})

	for _, key := range keysToDelete {
		// Decrement only on an actual removal (see evictOldest) to keep the
		// soft bucket count from drifting under concurrent eviction.
		if _, removed := l.limiters.LoadAndDelete(key); removed {
			l.bucketCount.Add(-1)
		}
		l.lastAccess.Delete(key)
	}
}

// Stop stops the cleanup goroutine
func (l *InMemoryRateLimiter) Stop() {
	close(l.stopCleanup)
}
