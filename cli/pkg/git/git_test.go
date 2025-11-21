package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectGitInfo_NotGitRepo(t *testing.T) {
	// Create temp directory (not a git repo)
	tmpDir := t.TempDir()

	info, err := DetectGitInfo(tmpDir)
	if err != nil {
		t.Fatalf("DetectGitInfo() unexpected error: %v", err)
	}

	if info != nil {
		t.Errorf("Expected nil info for non-git directory, got %+v", info)
	}
}

func TestDetectGitInfo_GitRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	// Create temp directory and init git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	// Create a commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)
	runGit(t, tmpDir, "add", "test.txt")
	runGit(t, tmpDir, "commit", "-m", "Initial commit")

	// Add remote
	runGit(t, tmpDir, "remote", "add", "origin", "https://github.com/test/repo.git")

	// Detect git info
	info, err := DetectGitInfo(tmpDir)
	if err != nil {
		t.Fatalf("DetectGitInfo() error: %v", err)
	}

	if info == nil {
		t.Fatal("Expected non-nil info for git repo")
	}

	// Verify fields
	if info.RepoURL != "https://github.com/test/repo.git" {
		t.Errorf("RepoURL = %q, want %q", info.RepoURL, "https://github.com/test/repo.git")
	}

	if info.Branch == "" {
		t.Error("Branch should not be empty")
	}

	if info.CommitSHA == "" {
		t.Error("CommitSHA should not be empty")
	}

	if info.CommitMessage != "Initial commit" {
		t.Errorf("CommitMessage = %q, want %q", info.CommitMessage, "Initial commit")
	}

	if info.Author != "Test User <test@example.com>" {
		t.Errorf("Author = %q, want %q", info.Author, "Test User <test@example.com>")
	}

	// Repo should be clean (no uncommitted changes)
	if info.IsDirty {
		t.Error("IsDirty should be false for clean repo")
	}
}

func TestDetectGitInfo_DirtyRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	// Create temp directory and init git repo
	tmpDir := t.TempDir()

	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	// Create a commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)
	runGit(t, tmpDir, "add", "test.txt")
	runGit(t, tmpDir, "commit", "-m", "Initial commit")

	// Make uncommitted changes
	os.WriteFile(testFile, []byte("modified content"), 0644)

	// Detect git info
	info, err := DetectGitInfo(tmpDir)
	if err != nil {
		t.Fatalf("DetectGitInfo() error: %v", err)
	}

	if info == nil {
		t.Fatal("Expected non-nil info for git repo")
	}

	// Repo should be dirty
	if !info.IsDirty {
		t.Error("IsDirty should be true for repo with uncommitted changes")
	}
}

func TestDetectGitInfo_NoRemote(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	// Create temp directory and init git repo (no remote)
	tmpDir := t.TempDir()

	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	// Create a commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)
	runGit(t, tmpDir, "add", "test.txt")
	runGit(t, tmpDir, "commit", "-m", "Initial commit")

	// Detect git info
	info, err := DetectGitInfo(tmpDir)
	if err != nil {
		t.Fatalf("DetectGitInfo() error: %v", err)
	}

	if info == nil {
		t.Fatal("Expected non-nil info for git repo")
	}

	// RepoURL should be empty (no remote configured)
	if info.RepoURL != "" {
		t.Errorf("RepoURL should be empty for repo without remote, got %q", info.RepoURL)
	}

	// Other fields should still be populated
	if info.CommitSHA == "" {
		t.Error("CommitSHA should not be empty")
	}
}

func TestIsGitRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	// Not a git repo
	tmpDir := t.TempDir()
	if isGitRepo(tmpDir) {
		t.Error("isGitRepo() returned true for non-git directory")
	}

	// Is a git repo
	runGit(t, tmpDir, "init")
	if !isGitRepo(tmpDir) {
		t.Error("isGitRepo() returned false for git directory")
	}
}

// Helper to run git commands in tests
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(output))
	}
}
