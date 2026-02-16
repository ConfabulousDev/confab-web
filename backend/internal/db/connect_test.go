package db

import (
	"context"
	"testing"
	"time"
)

func TestConnectWithRetry_InvalidDSN_RespectsContextCancellation(t *testing.T) {
	// Use a context that expires quickly — ConnectWithRetry should give up
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := ConnectWithRetry(ctx, "postgres://invalid:invalid@localhost:1/nonexistent")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from ConnectWithRetry with invalid DSN")
	}

	// Should have retried at least once (1s delay) but respected the 2s timeout
	if elapsed < 1*time.Second {
		t.Errorf("expected at least one retry (~1s), but returned in %v", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("expected to respect context timeout (~2s), but took %v", elapsed)
	}
}

func TestConnectWithRetry_AlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := ConnectWithRetry(ctx, "postgres://invalid:invalid@localhost:1/nonexistent")
	if err == nil {
		t.Fatal("expected error from ConnectWithRetry with cancelled context")
	}
}
