package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSessionFromPath(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	projectsDir := filepath.Join(tmpDir, "projects")
	projectDir := filepath.Join(projectsDir, "test-project")
	os.MkdirAll(projectDir, 0755)

	tests := []struct {
		name       string
		filename   string
		wantNil    bool
		wantID     string
		createFile bool
	}{
		{
			name:       "valid session file",
			filename:   "12345678-1234-1234-1234-123456789abc.jsonl",
			wantNil:    false,
			wantID:     "12345678-1234-1234-1234-123456789abc",
			createFile: true,
		},
		{
			name:       "agent file should be skipped",
			filename:   "agent-abcd1234.jsonl",
			wantNil:    true,
			createFile: true,
		},
		{
			name:       "non-jsonl file should be skipped",
			filename:   "readme.txt",
			wantNil:    true,
			createFile: true,
		},
		{
			name:       "short uuid should be skipped",
			filename:   "short-id.jsonl",
			wantNil:    true,
			createFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(projectDir, tt.filename)
			if tt.createFile {
				os.WriteFile(filePath, []byte("{}"), 0644)
			}

			info, _ := os.Stat(filePath)
			entry := mockDirEntry{
				name:  tt.filename,
				isDir: false,
				info:  info,
			}

			result := parseSessionFromPath(filePath, entry, projectsDir)

			if tt.wantNil && result != nil {
				t.Errorf("expected nil, got %+v", result)
			}
			if !tt.wantNil && result == nil {
				t.Error("expected result, got nil")
			}
			if !tt.wantNil && result != nil && result.SessionID != tt.wantID {
				t.Errorf("expected SessionID %q, got %q", tt.wantID, result.SessionID)
			}
		})
	}
}

func TestParseSessionFromPath_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	entry := mockDirEntry{name: "somedir", isDir: true}

	result := parseSessionFromPath(tmpDir, entry, tmpDir)
	if result != nil {
		t.Errorf("expected nil for directory, got %+v", result)
	}
}

// mockDirEntry implements os.DirEntry for testing
type mockDirEntry struct {
	name  string
	isDir bool
	info  os.FileInfo
}

func (m mockDirEntry) Name() string               { return m.name }
func (m mockDirEntry) IsDir() bool                { return m.isDir }
func (m mockDirEntry) Type() os.FileMode          { return 0 }
func (m mockDirEntry) Info() (os.FileInfo, error) { return m.info, nil }

func TestScanAllSessions(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Set env var to use temp directory
	oldEnv := os.Getenv("CONFAB_CLAUDE_DIR")
	os.Setenv("CONFAB_CLAUDE_DIR", tmpDir)
	defer os.Setenv("CONFAB_CLAUDE_DIR", oldEnv)

	// Create projects directory
	projectsDir := filepath.Join(tmpDir, "projects")
	project1 := filepath.Join(projectsDir, "project1")
	project2 := filepath.Join(projectsDir, "project2")
	os.MkdirAll(project1, 0755)
	os.MkdirAll(project2, 0755)

	// Create some session files
	session1 := "aaaaaaaa-1111-1111-1111-111111111111.jsonl"
	session2 := "bbbbbbbb-2222-2222-2222-222222222222.jsonl"
	session3 := "cccccccc-3333-3333-3333-333333333333.jsonl"

	os.WriteFile(filepath.Join(project1, session1), []byte("{}"), 0644)
	time.Sleep(10 * time.Millisecond) // Ensure different mod times
	os.WriteFile(filepath.Join(project2, session2), []byte("{}"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(project1, session3), []byte("{}"), 0644)

	// Create some files that should be ignored
	os.WriteFile(filepath.Join(project1, "agent-12345678.jsonl"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(project1, "readme.txt"), []byte("{}"), 0644)

	// Run scan
	sessions, err := ScanAllSessions()
	if err != nil {
		t.Fatalf("ScanAllSessions() error = %v", err)
	}

	// Should find exactly 3 sessions
	if len(sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sessions))
	}

	// Verify session IDs
	foundIDs := make(map[string]bool)
	for _, s := range sessions {
		foundIDs[s.SessionID] = true
	}

	expectedIDs := []string{
		"aaaaaaaa-1111-1111-1111-111111111111",
		"bbbbbbbb-2222-2222-2222-222222222222",
		"cccccccc-3333-3333-3333-333333333333",
	}

	for _, id := range expectedIDs {
		if !foundIDs[id] {
			t.Errorf("Expected to find session %s", id)
		}
	}

	// Verify sorting by mod time (oldest first)
	if len(sessions) >= 2 {
		if sessions[0].ModTime.After(sessions[1].ModTime) {
			t.Error("Sessions not sorted by mod time (oldest first)")
		}
	}
}

func TestScanAllSessions_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	oldEnv := os.Getenv("CONFAB_CLAUDE_DIR")
	os.Setenv("CONFAB_CLAUDE_DIR", tmpDir)
	defer os.Setenv("CONFAB_CLAUDE_DIR", oldEnv)

	// Create empty projects directory
	projectsDir := filepath.Join(tmpDir, "projects")
	os.MkdirAll(projectsDir, 0755)

	sessions, err := ScanAllSessions()
	if err != nil {
		t.Fatalf("ScanAllSessions() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions in empty directory, got %d", len(sessions))
	}
}

func TestScanAllSessions_NoProjectsDir(t *testing.T) {
	tmpDir := t.TempDir()

	oldEnv := os.Getenv("CONFAB_CLAUDE_DIR")
	os.Setenv("CONFAB_CLAUDE_DIR", tmpDir)
	defer os.Setenv("CONFAB_CLAUDE_DIR", oldEnv)

	// Don't create projects directory

	sessions, err := ScanAllSessions()
	if err != nil {
		t.Fatalf("ScanAllSessions() error = %v", err)
	}

	if sessions != nil {
		t.Errorf("Expected nil for non-existent directory, got %d sessions", len(sessions))
	}
}

func TestFindSessionByID(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	oldEnv := os.Getenv("CONFAB_CLAUDE_DIR")
	os.Setenv("CONFAB_CLAUDE_DIR", tmpDir)
	defer os.Setenv("CONFAB_CLAUDE_DIR", oldEnv)

	// Create projects directory
	projectsDir := filepath.Join(tmpDir, "projects")
	project1 := filepath.Join(projectsDir, "project1")
	os.MkdirAll(project1, 0755)

	// Create session files
	sessionID := "aaaaaaaa-1111-1111-1111-111111111111"
	sessionFile := sessionID + ".jsonl"
	sessionPath := filepath.Join(project1, sessionFile)
	os.WriteFile(sessionPath, []byte("{}"), 0644)

	tests := []struct {
		name      string
		searchID  string
		wantFound bool
		wantID    string
	}{
		{
			name:      "find by full ID",
			searchID:  "aaaaaaaa-1111-1111-1111-111111111111",
			wantFound: true,
			wantID:    "aaaaaaaa-1111-1111-1111-111111111111",
		},
		{
			name:      "find by 8-char prefix",
			searchID:  "aaaaaaaa",
			wantFound: true,
			wantID:    "aaaaaaaa-1111-1111-1111-111111111111",
		},
		{
			name:      "find by 4-char prefix",
			searchID:  "aaaa",
			wantFound: true,
			wantID:    "aaaaaaaa-1111-1111-1111-111111111111",
		},
		{
			name:      "not found",
			searchID:  "nonexistent",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullID, transcriptPath, err := FindSessionByID(tt.searchID)

			if tt.wantFound {
				if err != nil {
					t.Errorf("Expected to find session, got error: %v", err)
					return
				}
				if fullID != tt.wantID {
					t.Errorf("Expected ID %s, got %s", tt.wantID, fullID)
				}
				if transcriptPath != sessionPath {
					t.Errorf("Expected path %s, got %s", sessionPath, transcriptPath)
				}
			} else {
				if err == nil {
					t.Error("Expected error for non-existent session")
				}
			}
		})
	}
}

func TestFindSessionByID_AmbiguousID(t *testing.T) {
	tmpDir := t.TempDir()

	oldEnv := os.Getenv("CONFAB_CLAUDE_DIR")
	os.Setenv("CONFAB_CLAUDE_DIR", tmpDir)
	defer os.Setenv("CONFAB_CLAUDE_DIR", oldEnv)

	// Create projects directory
	projectsDir := filepath.Join(tmpDir, "projects")
	project1 := filepath.Join(projectsDir, "project1")
	os.MkdirAll(project1, 0755)

	// Create two sessions with same prefix
	session1 := "aaaa1111-1111-1111-1111-111111111111.jsonl"
	session2 := "aaaa2222-2222-2222-2222-222222222222.jsonl"
	os.WriteFile(filepath.Join(project1, session1), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(project1, session2), []byte("{}"), 0644)

	// Search with ambiguous prefix
	_, _, err := FindSessionByID("aaaa")
	if err == nil {
		t.Error("Expected error for ambiguous session ID")
	}
	if err != nil && !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("Expected 'ambiguous' error, got: %v", err)
	}
}
