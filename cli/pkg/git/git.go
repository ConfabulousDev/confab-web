package git

import (
	"os/exec"
	"strings"
)

// GitInfo contains git repository information
type GitInfo struct {
	RepoURL       string `json:"repo_url,omitempty"`
	Branch        string `json:"branch,omitempty"`
	CommitSHA     string `json:"commit_sha,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	Author        string `json:"author,omitempty"`
	IsDirty       bool   `json:"is_dirty"`
}

// DetectGitInfo detects git information from the given directory
// Returns nil if not in a git repository (this is not an error)
func DetectGitInfo(cwd string) (*GitInfo, error) {
	// Check if we're in a git repository
	if !isGitRepo(cwd) {
		return nil, nil // Not a git repo - not an error
	}

	info := &GitInfo{}

	// Get remote URL
	if url, err := gitCommand(cwd, "config", "--get", "remote.origin.url"); err == nil {
		info.RepoURL = strings.TrimSpace(url)
	}

	// Get current branch
	if branch, err := gitCommand(cwd, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		info.Branch = strings.TrimSpace(branch)
	}

	// Get commit SHA
	if sha, err := gitCommand(cwd, "rev-parse", "HEAD"); err == nil {
		info.CommitSHA = strings.TrimSpace(sha)
	}

	// Get commit message
	if msg, err := gitCommand(cwd, "log", "-1", "--format=%s"); err == nil {
		info.CommitMessage = strings.TrimSpace(msg)
	}

	// Get author
	if author, err := gitCommand(cwd, "log", "-1", "--format=%an <%ae>"); err == nil {
		info.Author = strings.TrimSpace(author)
	}

	// Check if repo is dirty (has uncommitted changes)
	if status, err := gitCommand(cwd, "status", "--porcelain"); err == nil {
		info.IsDirty = strings.TrimSpace(status) != ""
	}

	return info, nil
}

// isGitRepo checks if the directory is inside a git repository
func isGitRepo(cwd string) bool {
	_, err := gitCommand(cwd, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// gitCommand runs a git command in the specified directory
func gitCommand(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}
