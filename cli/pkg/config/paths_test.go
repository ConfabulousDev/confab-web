package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetClaudeStateDir(t *testing.T) {
	// Save original env
	originalEnv := os.Getenv(ClaudeStateDirEnv)
	defer os.Setenv(ClaudeStateDirEnv, originalEnv)

	home, _ := os.UserHomeDir()

	tests := []struct {
		name    string
		envVal  string
		want    string
		wantErr bool
	}{
		{
			name:    "default to ~/.claude",
			envVal:  "",
			want:    filepath.Join(home, ".claude"),
			wantErr: false,
		},
		{
			name:    "override with env var",
			envVal:  "/tmp/custom-claude",
			want:    "/tmp/custom-claude",
			wantErr: false,
		},
		{
			name:    "override with relative path",
			envVal:  "my-claude-dir",
			want:    "my-claude-dir",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env var
			if tt.envVal == "" {
				os.Unsetenv(ClaudeStateDirEnv)
			} else {
				os.Setenv(ClaudeStateDirEnv, tt.envVal)
			}

			got, err := GetClaudeStateDir()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClaudeStateDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetClaudeStateDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProjectsDir(t *testing.T) {
	originalEnv := os.Getenv(ClaudeStateDirEnv)
	defer os.Setenv(ClaudeStateDirEnv, originalEnv)

	// Test with env var set
	os.Setenv(ClaudeStateDirEnv, "/tmp/test-claude")
	got, err := GetProjectsDir()
	if err != nil {
		t.Fatalf("GetProjectsDir() error = %v", err)
	}

	want := "/tmp/test-claude/projects"
	if got != want {
		t.Errorf("GetProjectsDir() = %v, want %v", got, want)
	}
}

func TestGetTodosDir(t *testing.T) {
	originalEnv := os.Getenv(ClaudeStateDirEnv)
	defer os.Setenv(ClaudeStateDirEnv, originalEnv)

	// Test with env var set
	os.Setenv(ClaudeStateDirEnv, "/tmp/test-claude")
	got, err := GetTodosDir()
	if err != nil {
		t.Fatalf("GetTodosDir() error = %v", err)
	}

	want := "/tmp/test-claude/todos"
	if got != want {
		t.Errorf("GetTodosDir() = %v, want %v", got, want)
	}
}

func TestGetClaudeSettingsPath(t *testing.T) {
	originalEnv := os.Getenv(ClaudeStateDirEnv)
	defer os.Setenv(ClaudeStateDirEnv, originalEnv)

	// Test with env var set
	os.Setenv(ClaudeStateDirEnv, "/tmp/test-claude")
	got, err := GetClaudeSettingsPath()
	if err != nil {
		t.Fatalf("GetClaudeSettingsPath() error = %v", err)
	}

	want := "/tmp/test-claude/settings.json"
	if got != want {
		t.Errorf("GetClaudeSettingsPath() = %v, want %v", got, want)
	}
}

// TestEndToEndWithCustomDir demonstrates how to use the env var for testing
func TestEndToEndWithCustomDir(t *testing.T) {
	originalEnv := os.Getenv(ClaudeStateDirEnv)
	defer os.Setenv(ClaudeStateDirEnv, originalEnv)

	// Create a temp directory for testing
	tmpDir := t.TempDir()
	os.Setenv(ClaudeStateDirEnv, tmpDir)

	// Create test structure
	projectsDir := filepath.Join(tmpDir, "projects")
	todosDir := filepath.Join(tmpDir, "todos")
	os.MkdirAll(projectsDir, 0755)
	os.MkdirAll(todosDir, 0755)

	// Verify paths resolve correctly
	gotProjects, _ := GetProjectsDir()
	if gotProjects != projectsDir {
		t.Errorf("Expected projects dir %s, got %s", projectsDir, gotProjects)
	}

	gotTodos, _ := GetTodosDir()
	if gotTodos != todosDir {
		t.Errorf("Expected todos dir %s, got %s", todosDir, gotTodos)
	}

	// Settings file should be at tmpDir/settings.json
	gotSettings, _ := GetClaudeSettingsPath()
	wantSettings := filepath.Join(tmpDir, "settings.json")
	if gotSettings != wantSettings {
		t.Errorf("Expected settings path %s, got %s", wantSettings, gotSettings)
	}
}
