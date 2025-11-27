package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These tests verify daemon lifecycle and shutdown behavior via context
// cancellation and Stop(). They do NOT test OS signal handling (SIGTERM/SIGINT)
// because sending real signals affects the entire test process.
//
// OS signal handler placement is verified by code review - see Run() in daemon.go
// where signal.Notify is called before EnsureAuthenticated to catch signals
// during initialization.

// TestDaemonStopsOnContextCancel verifies the daemon exits cleanly on context cancel.
func TestDaemonStopsOnContextCancel(t *testing.T) {
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create transcript file so daemon doesn't wait
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(`{"type":"system"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to create transcript: %v", err)
	}

	// Create config so EnsureAuthenticated doesn't fail immediately
	// (it will fail on missing config, but that's OK - we want to test signal handling)
	confabDir := filepath.Join(tmpDir, ".confab")
	os.MkdirAll(confabDir, 0755)
	configPath := filepath.Join(confabDir, "config.json")
	os.WriteFile(configPath, []byte(`{"backend_url":"http://localhost:9999","api_key":"test-key-1234567890"}`), 0600)
	os.Setenv("CONFAB_CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFAB_CONFIG_PATH")

	d := New(Config{
		ExternalID:     "ctx-cancel-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Give daemon time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	select {
	case err := <-errCh:
		// Should exit cleanly (nil error from shutdown)
		if err != nil {
			t.Logf("daemon exited with error (expected for this test setup): %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not exit on context cancel")
	}
}

// TestDaemonStopsOnStopChannel verifies the daemon exits when Stop() is called.
func TestDaemonStopsOnStopChannel(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")
	os.WriteFile(transcriptPath, []byte(`{"type":"system"}`+"\n"), 0644)

	confabDir := filepath.Join(tmpDir, ".confab")
	os.MkdirAll(confabDir, 0755)
	configPath := filepath.Join(confabDir, "config.json")
	os.WriteFile(configPath, []byte(`{"backend_url":"http://localhost:9999","api_key":"test-key-1234567890"}`), 0600)
	os.Setenv("CONFAB_CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFAB_CONFIG_PATH")

	d := New(Config{
		ExternalID:     "stop-channel-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   100 * time.Millisecond,
	})

	ctx := context.Background()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Call Stop()
	d.Stop()

	select {
	case <-errCh:
		// Success - daemon exited
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not exit on Stop()")
	}
}

// TestWaitForTranscriptRespectsContext verifies waitForTranscript exits on context cancel.
// This is an internal test to ensure signals/context are checked during the wait loop.
func TestWaitForTranscriptRespectsContext(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// DON'T create transcript - we want it to wait
	transcriptPath := filepath.Join(tmpDir, "nonexistent", "transcript.jsonl")

	// Create config
	confabDir := filepath.Join(tmpDir, ".confab")
	os.MkdirAll(confabDir, 0755)
	configPath := filepath.Join(confabDir, "config.json")
	os.WriteFile(configPath, []byte(`{"backend_url":"http://localhost:9999","api_key":"test-key-1234567890"}`), 0600)
	os.Setenv("CONFAB_CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFAB_CONFIG_PATH")

	d := New(Config{
		ExternalID:     "wait-ctx-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Give daemon time to enter waitForTranscript
	time.Sleep(100 * time.Millisecond)

	// Cancel context while waiting for transcript
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected error when context cancelled during waitForTranscript")
		}
		// Should mention context or waiting
		t.Logf("daemon exited with: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not exit when context cancelled during waitForTranscript")
	}
}

// TestWaitForTranscriptRespectsStopChannel verifies waitForTranscript exits on Stop().
func TestWaitForTranscriptRespectsStopChannel(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// DON'T create transcript
	transcriptPath := filepath.Join(tmpDir, "nonexistent", "transcript.jsonl")

	confabDir := filepath.Join(tmpDir, ".confab")
	os.MkdirAll(confabDir, 0755)
	configPath := filepath.Join(confabDir, "config.json")
	os.WriteFile(configPath, []byte(`{"backend_url":"http://localhost:9999","api_key":"test-key-1234567890"}`), 0600)
	os.Setenv("CONFAB_CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFAB_CONFIG_PATH")

	d := New(Config{
		ExternalID:     "wait-stop-test",
		TranscriptPath: transcriptPath,
		CWD:            tmpDir,
		SyncInterval:   100 * time.Millisecond,
	})

	ctx := context.Background()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Give daemon time to enter waitForTranscript
	time.Sleep(100 * time.Millisecond)

	// Stop while waiting for transcript
	d.Stop()

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected error when stopped during waitForTranscript")
		}
		t.Logf("daemon exited with: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not exit when stopped during waitForTranscript")
	}
}

