package auth

import (
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
