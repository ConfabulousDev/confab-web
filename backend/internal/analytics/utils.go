package analytics

import (
	"path"
	"strings"
)

// workflowPrefix is the load-bearing path convention under which the CLI writes
// workflow subagent transcripts and the run journal:
// "subagents/workflows/<runId>/agent-<id>.jsonl" and ".../journal.jsonl".
const workflowPrefix = "subagents/workflows/"

// ExtractAgentID extracts the agent ID from a filename like "agent-{id}.jsonl".
// Returns empty string if the filename doesn't match the expected pattern.
//
// The match is applied to the path basename, so both flat ("agent-{id}.jsonl")
// and path-encoded ("subagents/workflows/{runId}/agent-{id}.jsonl") workflow
// subagent file names resolve to the same {id}.
func ExtractAgentID(fileName string) string {
	base := path.Base(fileName)
	if !strings.HasPrefix(base, "agent-") || !strings.HasSuffix(base, ".jsonl") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(base, "agent-"), ".jsonl")
}

// ExtractWorkflowRunID returns the <runId> segment from a workflow-nested file
// name of the form "subagents/workflows/<runId>/...". Returns "" for flat names
// or any path that does not match that shape. Works for both the agent file and
// the run journal, since both live directly under the <runId> directory.
func ExtractWorkflowRunID(fileName string) string {
	dir := path.Dir(fileName)
	if !strings.HasPrefix(dir, workflowPrefix) {
		return ""
	}
	runID := strings.TrimPrefix(dir, workflowPrefix)
	// runID must be exactly one path segment directly under workflows/.
	if runID == "" || strings.Contains(runID, "/") {
		return ""
	}
	return runID
}
