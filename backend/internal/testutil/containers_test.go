package testutil

import (
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/wait"
)

// These tests pin the container wait-strategy timeouts used by
// SetupTestEnvironment. They exist because previous "speed up tests" commits
// (e.g. 85f7760) tightened these to values that flake under cross-package
// parallel load. If you genuinely need to change them, update the test and
// the production code together — and document why in the commit message.

func TestMinioWaitStrategy_Timeout(t *testing.T) {
	t.Parallel()

	strategy := MinioWaitStrategy()
	httpStrategy, ok := strategy.(*wait.HTTPStrategy)
	if !ok {
		t.Fatalf("MinioWaitStrategy() returned %T, want *wait.HTTPStrategy", strategy)
	}

	got := httpStrategy.Timeout()
	if got == nil {
		t.Fatal("MinioWaitStrategy() HTTP strategy has nil timeout; expected explicit 90s")
	}
	if want := 90 * time.Second; *got != want {
		t.Errorf("MinioWaitStrategy() timeout = %v, want %v", *got, want)
	}
}

func TestPostgresWaitStrategy_Timeouts(t *testing.T) {
	t.Parallel()

	strategy := PostgresWaitStrategy()
	multi, ok := strategy.(*wait.MultiStrategy)
	if !ok {
		t.Fatalf("PostgresWaitStrategy() returned %T, want *wait.MultiStrategy", strategy)
	}

	var logTimeout, portTimeout *time.Duration
	for _, s := range multi.Strategies {
		switch ss := s.(type) {
		case *wait.LogStrategy:
			logTimeout = ss.Timeout()
		case *wait.HostPortStrategy:
			portTimeout = ss.Timeout()
		}
	}

	if logTimeout == nil {
		t.Fatal("PostgresWaitStrategy() missing LogStrategy with explicit timeout")
	}
	if want := 60 * time.Second; *logTimeout != want {
		t.Errorf("Postgres log-wait timeout = %v, want %v", *logTimeout, want)
	}

	if portTimeout == nil {
		t.Fatal("PostgresWaitStrategy() missing HostPortStrategy with explicit timeout")
	}
	if want := 30 * time.Second; *portTimeout != want {
		t.Errorf("Postgres port-wait timeout = %v, want %v", *portTimeout, want)
	}
}
