package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractGitInfoFromTranscript_WithGitInfo(t *testing.T) {
	// Create temporary transcript with git info
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"session-start","timestamp":"2024-01-01T00:00:00Z"}
{"type":"user","message":"test","cwd":"/tmp/nonexistent","gitBranch":"main","timestamp":"2024-01-01T00:01:00Z"}
{"type":"assistant","message":"response","timestamp":"2024-01-01T00:02:00Z"}`

	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitInfo, err := ExtractGitInfoFromTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if gitInfo == nil {
		t.Fatal("Expected git info, got nil")
	}

	if gitInfo.Branch != "main" {
		t.Errorf("Expected branch 'main', got '%s'", gitInfo.Branch)
	}
}

func TestExtractGitInfoFromTranscript_NoGitInfo(t *testing.T) {
	// Create temporary transcript without git info
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"session-start","timestamp":"2024-01-01T00:00:00Z"}
{"type":"user","message":"test","timestamp":"2024-01-01T00:01:00Z"}
{"type":"assistant","message":"response","timestamp":"2024-01-01T00:02:00Z"}`

	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitInfo, err := ExtractGitInfoFromTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if gitInfo != nil {
		t.Errorf("Expected nil git info, got: %+v", gitInfo)
	}
}

func TestExtractGitInfoFromTranscript_EmptyTranscript(t *testing.T) {
	// Create empty transcript
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(transcriptPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitInfo, err := ExtractGitInfoFromTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if gitInfo != nil {
		t.Errorf("Expected nil git info for empty transcript, got: %+v", gitInfo)
	}
}

func TestExtractGitInfoFromTranscript_FileNotExists(t *testing.T) {
	gitInfo, err := ExtractGitInfoFromTranscript("/nonexistent/path/transcript.jsonl")
	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}

	if gitInfo != nil {
		t.Errorf("Expected nil git info, got: %+v", gitInfo)
	}
}

func TestExtractGitInfoFromTranscript_MalformedJSON(t *testing.T) {
	// Create transcript with malformed JSON lines
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"session-start","timestamp":"2024-01-01T00:00:00Z"}
this is not valid json
{"type":"user","message":"test","cwd":"/tmp/test","gitBranch":"develop","timestamp":"2024-01-01T00:01:00Z"}
more invalid json
{"type":"assistant","message":"response","timestamp":"2024-01-01T00:02:00Z"}`

	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitInfo, err := ExtractGitInfoFromTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("Expected no error (should skip malformed lines), got: %v", err)
	}

	if gitInfo == nil {
		t.Fatal("Expected git info, got nil")
	}

	if gitInfo.Branch != "develop" {
		t.Errorf("Expected branch 'develop', got '%s'", gitInfo.Branch)
	}
}

func TestExtractGitInfoFromTranscript_EmptyGitBranch(t *testing.T) {
	// Create transcript with empty gitBranch field
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"session-start","timestamp":"2024-01-01T00:00:00Z"}
{"type":"user","message":"test","cwd":"/tmp/test","gitBranch":"","timestamp":"2024-01-01T00:01:00Z"}
{"type":"assistant","message":"response","timestamp":"2024-01-01T00:02:00Z"}`

	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitInfo, err := ExtractGitInfoFromTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Empty gitBranch should be treated as no git info
	if gitInfo != nil {
		t.Errorf("Expected nil git info for empty gitBranch, got: %+v", gitInfo)
	}
}

func TestExtractGitInfoFromTranscript_MultipleBranches(t *testing.T) {
	// Create transcript with multiple gitBranch entries (should use first one)
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"session-start","timestamp":"2024-01-01T00:00:00Z"}
{"type":"user","message":"first","cwd":"/tmp/test","gitBranch":"main","timestamp":"2024-01-01T00:01:00Z"}
{"type":"user","message":"second","cwd":"/tmp/test","gitBranch":"feature-branch","timestamp":"2024-01-01T00:02:00Z"}`

	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitInfo, err := ExtractGitInfoFromTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if gitInfo == nil {
		t.Fatal("Expected git info, got nil")
	}

	// Should extract the first occurrence
	if gitInfo.Branch != "main" {
		t.Errorf("Expected branch 'main' (first occurrence), got '%s'", gitInfo.Branch)
	}
}

func TestExtractGitInfoFromTranscript_LargeLine(t *testing.T) {
	// Create transcript with a very large line to test buffer handling
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	// Create a line with a large message field (simulating a large tool response)
	largeMessage := make([]byte, 500*1024) // 500KB
	for i := range largeMessage {
		largeMessage[i] = 'a'
	}

	content := `{"type":"session-start","timestamp":"2024-01-01T00:00:00Z"}
{"type":"assistant","message":"` + string(largeMessage) + `","timestamp":"2024-01-01T00:01:00Z"}
{"type":"user","message":"test","cwd":"/tmp/test","gitBranch":"main","timestamp":"2024-01-01T00:02:00Z"}`

	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gitInfo, err := ExtractGitInfoFromTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("Expected no error with large lines, got: %v", err)
	}

	if gitInfo == nil {
		t.Fatal("Expected git info, got nil")
	}

	if gitInfo.Branch != "main" {
		t.Errorf("Expected branch 'main', got '%s'", gitInfo.Branch)
	}
}
