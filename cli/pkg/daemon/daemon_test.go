package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/santaclaude2025/confab/pkg/types"
)

// These tests verify daemon lifecycle and shutdown behavior via context
// cancellation and Stop(). They do NOT test OS signal handling (SIGTERM/SIGINT)
// because sending real signals affects the entire test process.
//
// OS signal handler placement is verified by code review - see Run() in daemon.go
// where signal.Notify is called early to catch signals during initialization.

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

func TestWriteInboxEvent(t *testing.T) {
	tmpDir := t.TempDir()
	inboxPath := filepath.Join(tmpDir, "test.inbox.jsonl")

	hookInput := &types.HookInput{
		SessionID:      "test-session-123",
		TranscriptPath: "/path/to/transcript.jsonl",
		CWD:            "/work/dir",
		Reason:         "test_reason",
		HookEventName:  "SessionEnd",
	}

	// Write event
	err := writeInboxEvent(inboxPath, "session_end", hookInput)
	if err != nil {
		t.Fatalf("failed to write inbox event: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(inboxPath)
	if err != nil {
		t.Fatalf("failed to read inbox file: %v", err)
	}

	// Parse and verify
	var event types.InboxEvent
	if err := json.Unmarshal(data[:len(data)-1], &event); err != nil { // -1 to remove newline
		t.Fatalf("failed to parse inbox event: %v", err)
	}

	if event.Type != "session_end" {
		t.Errorf("expected Type 'session_end', got %q", event.Type)
	}
	if event.HookInput == nil {
		t.Fatal("expected HookInput to be set")
	}
	if event.HookInput.SessionID != "test-session-123" {
		t.Errorf("expected SessionID 'test-session-123', got %q", event.HookInput.SessionID)
	}
	if event.HookInput.Reason != "test_reason" {
		t.Errorf("expected Reason 'test_reason', got %q", event.HookInput.Reason)
	}
}

func TestWriteInboxEvent_MultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()
	inboxPath := filepath.Join(tmpDir, "test.inbox.jsonl")

	// Write multiple events
	for i := 0; i < 3; i++ {
		hookInput := &types.HookInput{
			SessionID: "session-" + string(rune('A'+i)),
			Reason:    "reason-" + string(rune('1'+i)),
		}
		if err := writeInboxEvent(inboxPath, "session_end", hookInput); err != nil {
			t.Fatalf("failed to write event %d: %v", i, err)
		}
	}

	// Read and count lines
	data, _ := os.ReadFile(inboxPath)
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("expected 3 lines, got %d", lines)
	}
}

func TestDaemon_ReadInboxEvents(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create sync dir
	syncDir := filepath.Join(tmpDir, ".confab", "sync")
	os.MkdirAll(syncDir, 0755)

	// Create a daemon with state
	d := &Daemon{
		externalID: "inbox-read-test",
		state:      NewState("inbox-read-test", "/path", "/cwd", 0),
	}

	// Write some events to inbox
	hookInput1 := &types.HookInput{SessionID: "session-1", Reason: "reason1"}
	hookInput2 := &types.HookInput{SessionID: "session-2", Reason: "reason2"}
	writeInboxEvent(d.state.InboxPath, "session_end", hookInput1)
	writeInboxEvent(d.state.InboxPath, "other_event", hookInput2)

	// Read events
	events := d.readInboxEvents()

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != "session_end" {
		t.Errorf("expected first event type 'session_end', got %q", events[0].Type)
	}
	if events[0].HookInput.Reason != "reason1" {
		t.Errorf("expected first event reason 'reason1', got %q", events[0].HookInput.Reason)
	}

	if events[1].Type != "other_event" {
		t.Errorf("expected second event type 'other_event', got %q", events[1].Type)
	}
}

func TestDaemon_ReadInboxEvents_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	d := &Daemon{
		externalID: "no-inbox-test",
		state:      NewState("no-inbox-test", "/path", "/cwd", 0),
	}

	// Should return nil when inbox doesn't exist
	events := d.readInboxEvents()
	if events != nil {
		t.Errorf("expected nil events when inbox doesn't exist, got %v", events)
	}
}

func TestDaemon_CleanupInbox(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create sync dir
	syncDir := filepath.Join(tmpDir, ".confab", "sync")
	os.MkdirAll(syncDir, 0755)

	d := &Daemon{
		externalID: "cleanup-test",
		state:      NewState("cleanup-test", "/path", "/cwd", 0),
	}

	// Create inbox file
	hookInput := &types.HookInput{SessionID: "test"}
	writeInboxEvent(d.state.InboxPath, "session_end", hookInput)

	// Verify it exists
	if _, err := os.Stat(d.state.InboxPath); err != nil {
		t.Fatalf("inbox file not created: %v", err)
	}

	// Cleanup
	d.cleanupInbox()

	// Verify it's deleted
	if _, err := os.Stat(d.state.InboxPath); !os.IsNotExist(err) {
		t.Error("expected inbox file to be deleted")
	}
}

