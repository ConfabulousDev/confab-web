package git

import (
	"testing"
)

func TestExtractGitInfoFromTranscript(t *testing.T) {
	// Test with a real transcript
	transcriptPath := "/Users/santaclaude/.claude/projects/-Users-santaclaude-dev-beta-confab/9fc6a017-1d02-4b3c-8405-754cd80d4677.jsonl"

	gitInfo, err := ExtractGitInfoFromTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("Failed to extract git info: %v", err)
	}

	if gitInfo == nil {
		t.Fatal("Expected git info, got nil")
	}

	if gitInfo.Branch == "" {
		t.Error("Expected branch to be set")
	}

	t.Logf("Extracted git info: Branch=%s, RepoURL=%s", gitInfo.Branch, gitInfo.RepoURL)
}
