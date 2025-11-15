package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/santaclaude/confab/pkg/types"
)

var agentIDPattern = regexp.MustCompile(`agent-([a-f0-9]{8})`)

// DiscoverSessionFiles finds all files associated with a session
func DiscoverSessionFiles(hookInput *types.HookInput) ([]types.SessionFile, error) {
	var files []types.SessionFile

	// 1. Expand transcript path (handle ~ for home directory)
	transcriptPath := expandPath(hookInput.TranscriptPath)

	// Check if transcript exists
	transcriptInfo, err := os.Stat(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("transcript file not found: %w", err)
	}

	// Add transcript file
	files = append(files, types.SessionFile{
		Path:      transcriptPath,
		Type:      "transcript",
		SizeBytes: transcriptInfo.Size(),
	})

	// 2. Parse transcript for agent references
	agentIDs, err := findAgentReferences(transcriptPath)
	if err != nil {
		// Log warning but continue - transcript alone is still useful
		fmt.Fprintf(os.Stderr, "Warning: failed to parse transcript for agents: %v\n", err)
		return files, nil
	}

	// 3. Add agent files if they exist
	transcriptDir := filepath.Dir(transcriptPath)
	for _, agentID := range agentIDs {
		agentFileName := fmt.Sprintf("agent-%s.jsonl", agentID)
		agentPath := filepath.Join(transcriptDir, agentFileName)

		if agentInfo, err := os.Stat(agentPath); err == nil {
			files = append(files, types.SessionFile{
				Path:      agentPath,
				Type:      "agent",
				SizeBytes: agentInfo.Size(),
			})
		}
	}

	return files, nil
}

// findAgentReferences parses a transcript file for agent ID references
func findAgentReferences(transcriptPath string) ([]string, error) {
	content, err := os.ReadFile(transcriptPath)
	if err != nil {
		return nil, err
	}

	matches := agentIDPattern.FindAllStringSubmatch(string(content), -1)

	// Use map to deduplicate agent IDs
	seen := make(map[string]bool)
	var agentIDs []string

	for _, match := range matches {
		if len(match) > 1 {
			agentID := match[1] // Capture group
			if !seen[agentID] {
				seen[agentID] = true
				agentIDs = append(agentIDs, agentID)
			}
		}
	}

	return agentIDs, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
