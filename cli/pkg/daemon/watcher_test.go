package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWatcher(t *testing.T) {
	w := NewWatcher("/path/to/transcript.jsonl")

	if w.transcriptPath != "/path/to/transcript.jsonl" {
		t.Errorf("expected transcriptPath '/path/to/transcript.jsonl', got %q", w.transcriptPath)
	}
	if w.transcriptDir != "/path/to" {
		t.Errorf("expected transcriptDir '/path/to', got %q", w.transcriptDir)
	}
	if w.files == nil {
		t.Error("expected files map to be initialized")
	}
	if w.knownAgentIDs == nil {
		t.Error("expected knownAgentIDs map to be initialized")
	}
}

func TestWatcher_InitFromState(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	w := NewWatcher(transcriptPath)

	state := map[string]FileState{
		"transcript.jsonl":      {LastSyncedLine: 100},
		"agent-abc12345.jsonl":  {LastSyncedLine: 50},
		"agent-def67890.jsonl":  {LastSyncedLine: 25},
	}

	w.InitFromState(state)

	files := w.GetTrackedFiles()
	if len(files) != 3 {
		t.Errorf("expected 3 tracked files, got %d", len(files))
	}

	// Check transcript
	found := false
	for _, f := range files {
		if filepath.Base(f.Path) == "transcript.jsonl" {
			found = true
			if f.Type != "transcript" {
				t.Errorf("expected transcript type, got %q", f.Type)
			}
			if f.LastSyncedLine != 100 {
				t.Errorf("expected LastSyncedLine 100, got %d", f.LastSyncedLine)
			}
		}
	}
	if !found {
		t.Error("transcript not found in tracked files")
	}
}

func TestWatcher_ReadNewLines(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Create test file with some lines
	content := `{"line": 1}
{"line": 2}
{"line": 3}
{"line": 4}
{"line": 5}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	w := NewWatcher(transcriptPath)

	// Read all lines (lastSyncedLine = 0)
	lines, firstLine, err := w.ReadNewLines(transcriptPath, 0)
	if err != nil {
		t.Fatalf("failed to read lines: %v", err)
	}
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}
	if firstLine != 1 {
		t.Errorf("expected firstLine 1, got %d", firstLine)
	}

	// Read from line 3 onwards (lastSyncedLine = 2)
	lines, firstLine, err = w.ReadNewLines(transcriptPath, 2)
	if err != nil {
		t.Fatalf("failed to read lines: %v", err)
	}
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if firstLine != 3 {
		t.Errorf("expected firstLine 3, got %d", firstLine)
	}
	if lines[0] != `{"line": 3}` {
		t.Errorf("expected first line to be '{\"line\": 3}', got %q", lines[0])
	}

	// Read when already fully synced
	lines, firstLine, err = w.ReadNewLines(transcriptPath, 5)
	if err != nil {
		t.Fatalf("failed to read lines: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines when fully synced, got %d", len(lines))
	}
}

func TestWatcher_UpdateLastSynced(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	w := NewWatcher(transcriptPath)
	w.InitFromState(map[string]FileState{
		"transcript.jsonl": {LastSyncedLine: 0},
	})

	// Update last synced
	w.UpdateLastSynced("transcript.jsonl", 100)

	files := w.GetTrackedFiles()
	for _, f := range files {
		if filepath.Base(f.Path) == "transcript.jsonl" {
			if f.LastSyncedLine != 100 {
				t.Errorf("expected LastSyncedLine 100, got %d", f.LastSyncedLine)
			}
			return
		}
	}
	t.Error("transcript not found")
}

func TestWatcher_CheckForNewFiles(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Create transcript with agent reference
	content := `{"type": "user", "toolUseResult": {"agentId": "abc12345"}}
{"type": "assistant", "message": "hello"}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create the agent file
	agentPath := filepath.Join(tmpDir, "agent-abc12345.jsonl")
	if err := os.WriteFile(agentPath, []byte(`{"line": 1}`), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	w := NewWatcher(transcriptPath)
	w.InitFromState(map[string]FileState{
		"transcript.jsonl": {LastSyncedLine: 0},
	})

	// Check for new files
	newFiles, err := w.CheckForNewFiles()
	if err != nil {
		t.Fatalf("failed to check for new files: %v", err)
	}

	if len(newFiles) != 1 {
		t.Errorf("expected 1 new file, got %d", len(newFiles))
	}

	if len(newFiles) > 0 {
		if filepath.Base(newFiles[0].Path) != "agent-abc12345.jsonl" {
			t.Errorf("expected agent-abc12345.jsonl, got %q", newFiles[0].Path)
		}
		if newFiles[0].Type != "agent" {
			t.Errorf("expected type 'agent', got %q", newFiles[0].Type)
		}
	}

	// Check again - should find no new files
	newFiles, err = w.CheckForNewFiles()
	if err != nil {
		t.Fatalf("failed to check for new files: %v", err)
	}
	if len(newFiles) != 0 {
		t.Errorf("expected 0 new files on second check, got %d", len(newFiles))
	}
}

func TestWatcher_CheckForNewFiles_AgentFileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Create transcript with agent reference but don't create the agent file
	content := `{"type": "user", "toolUseResult": {"agentId": "missing123"}}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	w := NewWatcher(transcriptPath)
	w.InitFromState(map[string]FileState{
		"transcript.jsonl": {LastSyncedLine: 0},
	})

	// Check for new files - should not error even though agent file doesn't exist
	newFiles, err := w.CheckForNewFiles()
	if err != nil {
		t.Fatalf("failed to check for new files: %v", err)
	}

	if len(newFiles) != 0 {
		t.Errorf("expected 0 new files (agent file doesn't exist), got %d", len(newFiles))
	}
}

func TestIsValidAgentID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc12345", true},
		{"ABCDEF12", true},
		{"12345678", true},
		{"abcdefgh", false}, // g and h are not hex
		{"abc1234", false},  // too short
		{"abc123456", false}, // too long
		{"", false},
		{"abc-1234", false}, // contains hyphen
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidAgentID(tt.input)
			if result != tt.expected {
				t.Errorf("isValidAgentID(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}
