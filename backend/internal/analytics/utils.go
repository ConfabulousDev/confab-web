package analytics

import "strings"

// ExtractAgentID extracts the agent ID from a filename like "agent-{id}.jsonl".
// Returns empty string if the filename doesn't match the expected pattern.
func ExtractAgentID(fileName string) string {
	if !strings.HasPrefix(fileName, "agent-") || !strings.HasSuffix(fileName, ".jsonl") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(fileName, "agent-"), ".jsonl")
}
