package api

import "net/http"

// capabilitiesResponse advertises optional backend features so a newer CLI can
// gate behavior on what this (possibly older, self-hosted) backend supports.
// The body IS the capabilities map — there is no outer wrapper. Old backends
// lack this endpoint entirely, so the CLI treats a 404/absent signal as "off".
type capabilitiesResponse struct {
	// WorkflowFiles reports support for path-encoded workflow subagent file
	// names (subagents/workflows/<runId>/agent-<id>.jsonl).
	WorkflowFiles bool `json:"workflow_files"`
	// WorkflowJournal reports support for the workflow_journal file_type
	// (subagents/workflows/<runId>/journal.jsonl).
	WorkflowJournal bool `json:"workflow_journal"`
	// OpencodeSubagents reports support for ingesting OpenCode subagent JSONL
	// files uploaded as file_type='agent' under the root session (CF-539).
	// A CF-538 CLI gates upload of subagent files on this flag.
	OpencodeSubagents bool `json:"opencode_subagents"`
}

// handleCapabilities reports the workflow-file capabilities of this build.
// Public, no auth, and dependency-free (no DB/network) so the CLI can probe it
// before holding any session context. The signal is static: a build that ships
// this endpoint supports both capabilities. Older backends lack the route
// entirely, so the CLI reads a 404/absent signal as "unsupported".
func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, capabilitiesResponse{
		WorkflowFiles:     true,
		WorkflowJournal:   true,
		OpencodeSubagents: true,
	})
}
