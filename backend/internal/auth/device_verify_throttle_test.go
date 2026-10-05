package auth

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestAttemptLimiter_LocksAtMaxFailures(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newAttemptLimiter(3, 15*time.Minute)
	l.now = func() time.Time { return now }

	const key = "user-42"
	if l.Locked(key) {
		t.Fatal("a fresh key must not be locked")
	}

	// Two failures: still under the threshold.
	l.RecordFailure(key)
	l.RecordFailure(key)
	if l.Locked(key) {
		t.Fatal("must not lock before reaching max failures")
	}

	// Third failure reaches the threshold → locked.
	l.RecordFailure(key)
	if !l.Locked(key) {
		t.Fatal("must lock once max failures is reached")
	}

	// An unrelated key is unaffected (per-verifier isolation).
	if l.Locked("other-user") {
		t.Fatal("an unrelated key must not be locked")
	}
}

func TestAttemptLimiter_ResetClearsFailureCount(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newAttemptLimiter(3, 15*time.Minute)
	l.now = func() time.Time { return now }

	const key = "user-7"
	l.RecordFailure(key)
	l.RecordFailure(key)
	l.Reset(key) // e.g. a successful authorize

	// After reset it must take a full fresh run of failures to lock again.
	l.RecordFailure(key)
	l.RecordFailure(key)
	if l.Locked(key) {
		t.Fatal("reset must clear the failure count")
	}
	l.RecordFailure(key)
	if !l.Locked(key) {
		t.Fatal("must lock after a fresh run of max failures post-reset")
	}
}

func TestAttemptLimiter_LockExpiresAfterWindow(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	l := newAttemptLimiter(2, 15*time.Minute)
	l.now = func() time.Time { return now }

	const key = "user-9"
	l.RecordFailure(key)
	l.RecordFailure(key)
	if !l.Locked(key) {
		t.Fatal("must be locked after max failures")
	}

	// Within the window: still locked.
	now = base.Add(14 * time.Minute)
	if !l.Locked(key) {
		t.Fatal("must stay locked within the lockout window")
	}

	// Past the window: lock clears.
	now = base.Add(16 * time.Minute)
	if l.Locked(key) {
		t.Fatal("lock must expire after the lockout window")
	}
}

// newCapacityTestLimiter returns a limiter with the production thresholds and
// an injected clock the caller controls through *now.
func newCapacityTestLimiter(now *time.Time) *attemptLimiter {
	l := newAttemptLimiter(deviceVerifyMaxFailures, deviceVerifyLockout)
	l.now = func() time.Time { return *now }
	return l
}

func capacityKey(i int) string { return "user-" + strconv.Itoa(i) }

// lockKey records enough failures to put key into a live lockout.
func lockKey(l *attemptLimiter, key string) {
	for range deviceVerifyMaxFailures {
		l.RecordFailure(key)
	}
}

func TestAttemptLimiter_LiveLockoutsAreNeverEvictedAtCapacity(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newCapacityTestLimiter(&now)

	for i := range maxAttemptKeys {
		lockKey(l, capacityKey(i))
	}
	if got := len(l.states); got != maxAttemptKeys {
		t.Fatalf("setup: tracked keys = %d, want %d", got, maxAttemptKeys)
	}

	// A new key at capacity: the sweep finds nothing evictable (every entry
	// is a live lockout), so the new key is tracked beyond the soft bound.
	overflow := capacityKey(maxAttemptKeys)
	l.RecordFailure(overflow)
	if got := len(l.states); got != maxAttemptKeys+1 {
		t.Fatalf("tracked keys after one new key at capacity = %d, want %d (live lockouts must not be evicted)", got, maxAttemptKeys+1)
	}
	for i := range maxAttemptKeys {
		if !l.Locked(capacityKey(i)) {
			t.Fatalf("key %q lost its live lockout when a new key arrived at capacity", capacityKey(i))
		}
	}

	// The overflow key still gets the full brute-force defense.
	for range deviceVerifyMaxFailures - 1 {
		l.RecordFailure(overflow)
	}
	if !l.Locked(overflow) {
		t.Fatal("a key tracked beyond the soft bound must still lock at max failures")
	}

	// The map exceeds the bound only by the number of simultaneously locked keys.
	l.RecordFailure(capacityKey(maxAttemptKeys + 1))
	if got := len(l.states); got != maxAttemptKeys+2 {
		t.Fatalf("tracked keys = %d, want %d", got, maxAttemptKeys+2)
	}
}

func TestAttemptLimiter_ExpiredLockoutsAreReclaimedAtCapacity(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	l := newCapacityTestLimiter(&now)

	for i := range maxAttemptKeys {
		lockKey(l, capacityKey(i))
	}

	// Exactly at lockedUntil the lock has expired (the window is half-open).
	now = base.Add(deviceVerifyLockout)
	newKey := capacityKey(maxAttemptKeys)
	l.RecordFailure(newKey)
	if got := len(l.states); got != 1 {
		t.Fatalf("tracked keys after expiry sweep = %d, want 1 (only the new key)", got)
	}
	if _, ok := l.states[newKey]; !ok {
		t.Fatal("the new key must be tracked after the sweep")
	}
	for i := range maxAttemptKeys {
		if l.Locked(capacityKey(i)) {
			t.Fatalf("key %q must not be locked after its window expired", capacityKey(i))
		}
	}
}

func TestAttemptLimiter_UnlockedEntriesReclaimedButLiveLocksKeptAtCapacity(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newCapacityTestLimiter(&now)

	// Even keys are live-locked; odd keys have a pending, below-threshold count.
	locked := 0
	for i := range maxAttemptKeys {
		if i%2 == 0 {
			lockKey(l, capacityKey(i))
			locked++
		} else {
			l.RecordFailure(capacityKey(i))
		}
	}

	l.RecordFailure(capacityKey(maxAttemptKeys))
	if got := len(l.states); got != locked+1 {
		t.Fatalf("tracked keys after sweep = %d, want %d (live locks + new key)", got, locked+1)
	}
	for i := 0; i < maxAttemptKeys; i += 2 {
		if !l.Locked(capacityKey(i)) {
			t.Fatalf("live-locked key %q was evicted by the sweep", capacityKey(i))
		}
	}
	// A reclaimed key with pending failures restarts its count from zero.
	reclaimed := capacityKey(1)
	if _, ok := l.states[reclaimed]; ok {
		t.Fatalf("below-threshold key %q must be reclaimed at capacity", reclaimed)
	}
	for range deviceVerifyMaxFailures - 1 {
		l.RecordFailure(reclaimed)
	}
	if l.Locked(reclaimed) {
		t.Fatal("a reclaimed key must restart its failure count")
	}
}

func TestAttemptLimiter_ConcurrentUse(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newCapacityTestLimiter(&now)

	const workers = 16
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			own := "worker-" + strconv.Itoa(w)
			for range deviceVerifyMaxFailures {
				l.RecordFailure("shared")
				l.RecordFailure(own)
				_ = l.Locked("shared")
				_ = l.Locked(own)
			}
		})
	}
	wg.Wait()

	if !l.Locked("shared") {
		t.Fatal("a key failed concurrently by every worker must be locked")
	}
	for w := range workers {
		if !l.Locked("worker-" + strconv.Itoa(w)) {
			t.Fatalf("worker-%d must be locked after max failures under concurrency", w)
		}
	}
	if got := len(l.states); got != workers+1 {
		t.Fatalf("tracked keys = %d, want %d", got, workers+1)
	}
}
