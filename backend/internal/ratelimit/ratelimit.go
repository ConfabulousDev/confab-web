package ratelimit

import (
	"context"
	"sync"
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

	// stopCleanup signals cleanup goroutine to stop
	stopCleanup chan struct{}
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
// rate: requests per second (e.g., 10 for 10 req/sec)
// burst: maximum burst size (e.g., 20 to allow bursts up to 20 requests)
func NewInMemoryRateLimiter(rps float64, burst int) *InMemoryRateLimiter {
	limiter := &InMemoryRateLimiter{
		rate:            rate.Limit(rps),
		burst:           burst,
		cleanupInterval: 5 * time.Minute,
		maxAge:          10 * time.Minute,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine
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

	// Update last access time
	l.lastAccess.Store(key, time.Now().UTC())

	return limiter.AllowN(time.Now().UTC(), n)
}

// getLimiter gets or creates a rate limiter for the given key
func (l *InMemoryRateLimiter) getLimiter(key string) *rate.Limiter {
	// Try to load existing limiter
	if limiter, exists := l.limiters.Load(key); exists {
		return limiter.(*rate.Limiter)
	}

	// Create new limiter
	limiter := rate.NewLimiter(l.rate, l.burst)

	// Store it (may race with another goroutine, that's OK)
	actual, loaded := l.limiters.LoadOrStore(key, limiter)
	if loaded {
		// Another goroutine created it first, use that one
		return actual.(*rate.Limiter)
	}

	// We created it, store last access time
	l.lastAccess.Store(key, time.Now().UTC())

	return limiter
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

	// Find old limiters
	l.lastAccess.Range(func(key, value interface{}) bool {
		lastTime := value.(time.Time)
		if lastTime.Before(cutoff) {
			keysToDelete = append(keysToDelete, key.(string))
		}
		return true
	})

	// Delete them
	for _, key := range keysToDelete {
		l.limiters.Delete(key)
		l.lastAccess.Delete(key)
	}

	if len(keysToDelete) > 0 {
		// Optional: log cleanup
		// log.Printf("Cleaned up %d old rate limiters", len(keysToDelete))
	}
}

// Stop stops the cleanup goroutine
func (l *InMemoryRateLimiter) Stop() {
	close(l.stopCleanup)
}

// Stats returns statistics about the rate limiter
func (l *InMemoryRateLimiter) Stats() map[string]interface{} {
	var count int
	l.limiters.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	return map[string]interface{}{
		"type":            "in-memory",
		"active_limiters": count,
		"rate_per_second": float64(l.rate),
		"burst":           l.burst,
	}
}
