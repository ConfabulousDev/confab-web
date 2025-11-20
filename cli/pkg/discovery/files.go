package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/santaclaude2025/confab/pkg/types"
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

	// 4. Add todo files for session and agents
	// Main session todo: {sessionID}-agent-{sessionID}.json
	// Agent todos: {sessionID}-agent-{agentID}.json
	todoFiles := findTodoFiles(hookInput.SessionID, agentIDs)
	files = append(files, todoFiles...)

	return files, nil
}

// findAgentReferences parses a transcript file for agent ID references
// Only matches agent IDs in toolUseResult.agentId fields to avoid false positives
func findAgentReferences(transcriptPath string) ([]string, error) {
	content, err := os.ReadFile(transcriptPath)
	if err != nil {
		return nil, err
	}

	// More precise pattern: look for "agentId":"agent-XXXXXXXX" in toolUseResult
	// This ensures we only match agents that were actually spawned, not just mentioned
	agentRefPattern := regexp.MustCompile(`"agentId"\s*:\s*"agent-([a-f0-9]{8})"`)
	matches := agentRefPattern.FindAllStringSubmatch(string(content), -1)

	// Use map to deduplicate agent IDs
	seen := make(map[string]bool)
	var agentIDs []string

	for _, match := range matches {
		if len(match) > 1 {
			agentID := match[1] // Capture group (just the hex part)
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

// findTodoFiles finds todo files for the session and its agents
// Todos are stored in ~/.claude/todos/ as {sessionID}-agent-{agentID}.json
func findTodoFiles(sessionID string, agentIDs []string) []types.SessionFile {
	var files []types.SessionFile

	home, err := os.UserHomeDir()
	if err != nil {
		// Can't find home directory, skip todos
		return files
	}

	todoDir := filepath.Join(home, ".claude", "todos")

	// Check if todos directory exists
	if _, err := os.Stat(todoDir); os.IsNotExist(err) {
		return files
	}

	// All agent IDs to check (including main session which uses sessionID as agentID)
	allAgentIDs := append([]string{sessionID}, agentIDs...)

	for _, agentID := range allAgentIDs {
		todoFileName := fmt.Sprintf("%s-agent-%s.json", sessionID, agentID)
		todoPath := filepath.Join(todoDir, todoFileName)

		if todoInfo, err := os.Stat(todoPath); err == nil {
			files = append(files, types.SessionFile{
				Path:      todoPath,
				Type:      "todo",
				SizeBytes: todoInfo.Size(),
			})
		}
	}

	return files
}
