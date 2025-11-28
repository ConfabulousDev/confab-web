package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewState(t *testing.T) {
	state := NewState("ext-123", "/path/to/transcript.jsonl", "/work/dir", 0)

	if state.ExternalID != "ext-123" {
		t.Errorf("expected ExternalID 'ext-123', got %q", state.ExternalID)
	}
	if state.TranscriptPath != "/path/to/transcript.jsonl" {
		t.Errorf("expected TranscriptPath '/path/to/transcript.jsonl', got %q", state.TranscriptPath)
	}
	if state.CWD != "/work/dir" {
		t.Errorf("expected CWD '/work/dir', got %q", state.CWD)
	}
	if state.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), state.PID)
	}
	if state.ParentPID != 0 {
		t.Errorf("expected ParentPID 0, got %d", state.ParentPID)
	}
	if time.Since(state.StartedAt) > time.Second {
		t.Error("expected StartedAt to be recent")
	}
}

func TestNewState_WithParentPID(t *testing.T) {
	state := NewState("ext-456", "/path/to/transcript.jsonl", "/work/dir", 12345)

	if state.ParentPID != 12345 {
		t.Errorf("expected ParentPID 12345, got %d", state.ParentPID)
	}
}

func TestState_SaveAndLoad(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create and save state
	state := NewState("test-external-id", "/path/to/transcript.jsonl", "/work/dir", 0)

	if err := state.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Verify file was created
	statePath := filepath.Join(tmpDir, ".confab", "sync", "test-external-id.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Load state
	loaded, err := LoadState("test-external-id")
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded state is nil")
	}

	// Verify loaded state
	if loaded.ExternalID != "test-external-id" {
		t.Errorf("expected ExternalID 'test-external-id', got %q", loaded.ExternalID)
	}
	if loaded.TranscriptPath != "/path/to/transcript.jsonl" {
		t.Errorf("expected TranscriptPath '/path/to/transcript.jsonl', got %q", loaded.TranscriptPath)
	}
}

func TestState_LoadNonExistent(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Load non-existent state
	state, err := LoadState("non-existent-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for non-existent file")
	}
}

func TestState_Delete(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create and save state
	state := NewState("delete-test-id", "/path", "/cwd", 0)
	if err := state.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Verify file exists
	statePath := filepath.Join(tmpDir, ".confab", "sync", "delete-test-id.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Delete state
	if err := state.Delete(); err != nil {
		t.Fatalf("failed to delete state: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("expected state file to be deleted")
	}
}

func TestState_IsDaemonRunning(t *testing.T) {
	state := NewState("ext-id", "/path", "/cwd", 0)

	// Current process should be running
	if !state.IsDaemonRunning() {
		t.Error("expected daemon to be running (current process)")
	}

	// Non-existent PID should not be running
	state.PID = 999999999 // Very unlikely to exist
	if state.IsDaemonRunning() {
		t.Error("expected daemon to not be running (non-existent PID)")
	}

	// Invalid PID should not be running
	state.PID = 0
	if state.IsDaemonRunning() {
		t.Error("expected daemon to not be running (zero PID)")
	}

	state.PID = -1
	if state.IsDaemonRunning() {
		t.Error("expected daemon to not be running (negative PID)")
	}
}

func TestState_IsParentRunning(t *testing.T) {
	// With parent PID set to current process, should be running
	state := NewState("ext-id", "/path", "/cwd", os.Getpid())
	if !state.IsParentRunning() {
		t.Error("expected parent to be running (current process)")
	}

	// Non-existent PID should not be running
	state.ParentPID = 999999999
	if state.IsParentRunning() {
		t.Error("expected parent to not be running (non-existent PID)")
	}

	// Zero PID (no parent monitoring) should return false
	state.ParentPID = 0
	if state.IsParentRunning() {
		t.Error("expected parent to not be running (zero PID)")
	}

	// Negative PID should return false
	state.ParentPID = -1
	if state.IsParentRunning() {
		t.Error("expected parent to not be running (negative PID)")
	}
}

func TestListAllStates(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create a few states
	state1 := NewState("list-test-1", "/path1", "/cwd1", 0)
	state1.Save()

	state2 := NewState("list-test-2", "/path2", "/cwd2", 0)
	state2.Save()

	state3 := NewState("list-test-3", "/path3", "/cwd3", 0)
	state3.Save()

	// List all states
	states, err := ListAllStates()
	if err != nil {
		t.Fatalf("failed to list states: %v", err)
	}

	if len(states) != 3 {
		t.Errorf("expected 3 states, got %d", len(states))
	}

	// Verify all states are present
	found := make(map[string]bool)
	for _, s := range states {
		found[s.ExternalID] = true
	}

	for _, id := range []string{"list-test-1", "list-test-2", "list-test-3"} {
		if !found[id] {
			t.Errorf("expected to find state with ID %q", id)
		}
	}
}

func TestListAllStates_EmptyDir(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// List states when sync dir doesn't exist
	states, err := ListAllStates()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if states != nil && len(states) != 0 {
		t.Errorf("expected empty states list, got %d", len(states))
	}
}

func TestGetInboxPath(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	path, err := GetInboxPath("test-session-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tmpDir, ".confab", "sync", "test-session-id.inbox.jsonl")
	if path != expected {
		t.Errorf("expected path %q, got %q", expected, path)
	}
}

func TestNewState_InboxPath(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	state := NewState("inbox-test-id", "/path", "/cwd", 0)

	expected := filepath.Join(tmpDir, ".confab", "sync", "inbox-test-id.inbox.jsonl")
	if state.InboxPath != expected {
		t.Errorf("expected InboxPath %q, got %q", expected, state.InboxPath)
	}
}
