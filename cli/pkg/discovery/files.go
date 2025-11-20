package discovery

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santaclaude2025/confab/pkg/types"
)

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
// Properly parses JSONL and looks for toolUseResult.agentId fields
func findAgentReferences(transcriptPath string) ([]string, error) {
	file, err := os.Open(transcriptPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	seen := make(map[string]bool)
	var agentIDs []string

	// Parse JSONL line by line
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long transcript lines (default is 64KB, use 10MB)
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)
	for scanner.Scan() {
		var message map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			// Skip malformed lines
			continue
		}

		// Check if this is a user message (tool results)
		msgType, ok := message["type"].(string)
		if !ok || msgType != "user" {
			continue
		}

		// Check for toolUseResult.agentId at ROOT level (not inside message)
		if toolUseResult, ok := message["toolUseResult"].(map[string]interface{}); ok {
			if agentID, ok := toolUseResult["agentId"].(string); ok {
				// agentId is just the hex value (e.g., "96f3c489")
				// Validate it's 8 hex characters
				if len(agentID) == 8 && isHexString(agentID) {
					if !seen[agentID] {
						seen[agentID] = true
						agentIDs = append(agentIDs, agentID)
					}
				}
			}
		}

		// Also check in content blocks inside the nested message object
		if nestedMessage, ok := message["message"].(map[string]interface{}); ok {
			if content, ok := nestedMessage["content"].([]interface{}); ok {
				for _, block := range content {
					if blockMap, ok := block.(map[string]interface{}); ok {
						if blockMap["type"] == "tool_result" {
							// Check content for toolUseResult.agentId
							if resultContent, ok := blockMap["content"].(map[string]interface{}); ok {
								if toolUseResult, ok := resultContent["toolUseResult"].(map[string]interface{}); ok {
									if agentID, ok := toolUseResult["agentId"].(string); ok {
										if len(agentID) == 8 && isHexString(agentID) {
											if !seen[agentID] {
												seen[agentID] = true
												agentIDs = append(agentIDs, agentID)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return agentIDs, nil
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
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
