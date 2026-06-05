package analytics

import "testing"

// TestExtractWorkflowRunID locks the CF-532 contract: the run grouping is
// recoverable from a workflow-nested file name, and only from that shape.
func TestExtractWorkflowRunID(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     string
	}{
		{
			name:     "agent file under a workflow run",
			fileName: "subagents/workflows/run-123/agent-abc.jsonl",
			want:     "run-123",
		},
		{
			name:     "journal file under a workflow run",
			fileName: "subagents/workflows/run-123/journal.jsonl",
			want:     "run-123",
		},
		{
			name:     "runId may itself contain underscores and hyphens",
			fileName: "subagents/workflows/wf_2026-06-05_abc/agent-x.jsonl",
			want:     "wf_2026-06-05_abc",
		},
		{
			name:     "flat agent name has no run id",
			fileName: "agent-abc.jsonl",
			want:     "",
		},
		{
			name:     "plain transcript has no run id",
			fileName: "transcript.jsonl",
			want:     "",
		},
		{
			name:     "non-workflow nested path has no run id",
			fileName: "subagents/agent-abc.jsonl",
			want:     "",
		},
		{
			name:     "workflows prefix without a run segment has no run id",
			fileName: "subagents/workflows/agent-abc.jsonl",
			want:     "",
		},
		{
			name:     "empty string has no run id",
			fileName: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractWorkflowRunID(tt.fileName)
			if got != tt.want {
				t.Errorf("ExtractWorkflowRunID(%q) = %q, want %q", tt.fileName, got, tt.want)
			}
		})
	}
}
