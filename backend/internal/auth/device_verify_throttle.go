package auth

import (
	"sync"
	"time"
)

const (
	// deviceVerifyMaxFailures / deviceVerifyLockout mirror the password-auth
	// lockout (dbauth.MaxFailedAttempts / LockoutDuration): after this many
	// failed /auth/device/verify submissions a verifier is locked out for the
	// window. Defense-in-depth against brute-forcing outstanding user_codes.
	deviceVerifyMaxFailures = 5
	deviceVerifyLockout     = 15 * time.Minute

	// maxAttemptKeys is a soft bound on the limiter's map, far above any
	// legitimate concurrent-verifier count. When a new key arrives with the map
	// at or above this size, unlocked and expired entries are reclaimed (see
	// sweepLocked). Live lockouts are never evicted, so the map can exceed the
	// bound by the number of simultaneously locked keys. Keys are authenticated
	// user IDs, so that overshoot is bounded by the account population.
	maxAttemptKeys = 10000
)

// attemptLimiter is an in-memory failed-attempt lockout keyed by an arbitrary
// string (the verifier's user ID for device-verify). It mirrors the password
// lockout semantics — count failures, lock for a window once the threshold is
// reached, reset on success or window expiry — without a DB column or a write
// per attempt. Safe for concurrent use.
type attemptLimiter struct {
	mu          sync.Mutex
	states      map[string]*attemptState
	maxFailures int
	lockout     time.Duration
	now         func() time.Time // injectable for tests
}

type attemptState struct {
	failures    int
	lockedUntil time.Time // zero until the failure threshold is reached
}

func newAttemptLimiter(maxFailures int, lockout time.Duration) *attemptLimiter {
	return &attemptLimiter{
		states:      make(map[string]*attemptState),
		maxFailures: maxFailures,
		lockout:     lockout,
		now:         time.Now,
	}
}

// Locked reports whether key is currently within a lockout window. An expired
// lock is cleared as a side effect.
func (l *attemptLimiter) Locked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	s := l.states[key]
	if s == nil || s.lockedUntil.IsZero() {
		return false
	}
	if l.now().Before(s.lockedUntil) {
		return true
	}
	delete(l.states, key) // lock expired
	return false
}

// RecordFailure records one failed attempt for key, locking it for the window
// once the failure threshold is reached.
func (l *attemptLimiter) RecordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	s := l.states[key]
	if s == nil {
		l.sweepLocked()
		s = &attemptState{}
		l.states[key] = s
	} else if !s.lockedUntil.IsZero() && !l.now().Before(s.lockedUntil) {
		// A prior lock has expired — start a fresh count.
		s.failures = 0
		s.lockedUntil = time.Time{}
	}

	s.failures++
	if s.failures >= l.maxFailures {
		s.lockedUntil = l.now().Add(l.lockout)
	}
}

// Reset clears all failure state for key, e.g. after a successful authorize.
func (l *attemptLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.states, key)
}

// sweepLocked enforces the maxAttemptKeys soft bound: once the map has reached
// it, every unlocked or expired entry is dropped. A dropped key with pending
// (below-threshold) failures simply restarts its count, which is acceptable
// under memory pressure. Live lockouts are never evicted: evicting one would
// let an attacker clear their own lockout by flooding new keys. While live
// locks alone hold the map at or above the bound, the sweep drops nothing, the
// caller still inserts its key, and each further new key rescans the map under
// the mutex until enough locks expire. Caller must hold l.mu.
func (l *attemptLimiter) sweepLocked() {
	if len(l.states) < maxAttemptKeys {
		return
	}
	now := l.now()
	for k, s := range l.states {
		if s.lockedUntil.IsZero() || !now.Before(s.lockedUntil) {
			delete(l.states, k)
		}
	}
}
