package analytics

import "testing"

// agentFile parses a single workflow agent transcript for the analyzer tests.
func agentFile(t *testing.T, agentID, jsonl string) *TranscriptFile {
	t.Helper()
	tf, err := parseTranscriptFile([]byte(jsonl), agentID)
	if err != nil {
		t.Fatalf("parseTranscriptFile(%s): %v", agentID, err)
	}
	return tf
}

// TestWorkflowsAnalyzer_GroupsByRunAndAggregates locks the core contract:
// agent files group by runId, tokens/cost sum per run, runs sort by start time.
func TestWorkflowsAnalyzer_GroupsByRunAndAggregates(t *testing.T) {
	// Run wf-1: two agents. Run wf-2: one agent, later start.
	agentA := makeAssistantMessageFull("aa1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, 10, 20, []map[string]interface{}{makeTextBlock("a")}) + "\n"
	agentB := makeAssistantMessage("ab1", "2025-01-01T00:00:03Z", "claude-sonnet-4-20241022", 200, 100, []map[string]interface{}{makeTextBlock("b")}) + "\n"
	agentC := makeAssistantMessage("ac1", "2025-01-01T00:01:00Z", "claude-sonnet-4-20241022", 300, 150, []map[string]interface{}{makeTextBlock("c")}) + "\n"

	wa := &WorkflowsAnalyzer{}
	wa.ProcessAgent(agentFile(t, "a", agentA), "wf-1")
	wa.ProcessAgent(agentFile(t, "b", agentB), "wf-1")
	wa.ProcessAgent(agentFile(t, "c", agentC), "wf-2")

	runs := wa.Result()
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}

	// Ordered by start time: wf-1 (00:00:01) before wf-2 (00:01:00).
	r1 := runs[0]
	if r1.RunID != "wf-1" {
		t.Errorf("runs[0].RunID = %q, want wf-1", r1.RunID)
	}
	if r1.AgentCount != 2 {
		t.Errorf("wf-1 AgentCount = %d, want 2", r1.AgentCount)
	}
	if r1.InputTokens != 300 {
		t.Errorf("wf-1 InputTokens = %d, want 300", r1.InputTokens)
	}
	if r1.OutputTokens != 150 {
		t.Errorf("wf-1 OutputTokens = %d, want 150", r1.OutputTokens)
	}
	if r1.CacheCreation != 10 {
		t.Errorf("wf-1 CacheCreation = %d, want 10", r1.CacheCreation)
	}
	if r1.CacheRead != 20 {
		t.Errorf("wf-1 CacheRead = %d, want 20", r1.CacheRead)
	}
	if r1.EstimatedUSD == "" || r1.EstimatedUSD == "0" {
		t.Errorf("wf-1 EstimatedUSD = %q, want a non-zero cost", r1.EstimatedUSD)
	}
	// Span across the run's agent line timestamps: 00:00:01 -> 00:00:03 = 2000ms.
	if r1.DurationMs != 2000 {
		t.Errorf("wf-1 DurationMs = %d, want 2000", r1.DurationMs)
	}
	// No journal processed.
	if r1.HasJournal {
		t.Errorf("wf-1 HasJournal = true, want false")
	}

	if runs[1].RunID != "wf-2" || runs[1].AgentCount != 1 || runs[1].InputTokens != 300 {
		t.Errorf("runs[1] = %+v, want wf-2 with 1 agent / 300 input", runs[1])
	}
}

// TestWorkflowsAnalyzer_IgnoresNonWorkflowAgents: agents with an empty runId
// (ordinary Task-tool subagents) form no workflow runs.
func TestWorkflowsAnalyzer_IgnoresNonWorkflowAgents(t *testing.T) {
	jsonl := makeAssistantMessage("x1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{makeTextBlock("x")}) + "\n"
	wa := &WorkflowsAnalyzer{}
	wa.ProcessAgent(agentFile(t, "x", jsonl), "")
	if got := wa.Result(); len(got) != 0 {
		t.Errorf("len(runs) = %d, want 0 for empty runId", len(got))
	}
}

// TestWorkflowsAnalyzer_JournalStatus: succeeded = agents with a "result" line;
// only-"started" agents are not counted; HasJournal flips true.
func TestWorkflowsAnalyzer_JournalStatus(t *testing.T) {
	agentA := makeAssistantMessage("aa1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{makeTextBlock("a")}) + "\n"
	agentB := makeAssistantMessage("ab1", "2025-01-01T00:00:02Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{makeTextBlock("b")}) + "\n"

	wa := &WorkflowsAnalyzer{}
	wa.ProcessAgent(agentFile(t, "a", agentA), "wf-1")
	wa.ProcessAgent(agentFile(t, "b", agentB), "wf-1")

	// Agent "a" has a result line; agent "b" only started.
	journal := `{"type":"started","key":"k1","agentId":"a"}` + "\n" +
		`{"type":"result","key":"k1","agentId":"a","result":{"ok":true}}` + "\n" +
		`{"type":"started","key":"k2","agentId":"b"}` + "\n"
	wa.ProcessJournal("wf-1", []byte(journal))

	runs := wa.Result()
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if !runs[0].HasJournal {
		t.Errorf("HasJournal = false, want true")
	}
	if runs[0].SucceededAgents != 1 {
		t.Errorf("SucceededAgents = %d, want 1", runs[0].SucceededAgents)
	}
	if runs[0].AgentCount != 2 {
		t.Errorf("AgentCount = %d, want 2", runs[0].AgentCount)
	}
}

// TestWorkflowsAnalyzer_JournalResultForFilelessAgentNotCounted: a journal
// result for an agent that has no transcript file (capped/failed download) is
// not counted, so SucceededAgents never exceeds AgentCount.
func TestWorkflowsAnalyzer_JournalResultForFilelessAgentNotCounted(t *testing.T) {
	agentA := makeAssistantMessage("aa1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{makeTextBlock("a")}) + "\n"

	wa := &WorkflowsAnalyzer{}
	wa.ProcessAgent(agentFile(t, "a", agentA), "wf-1") // only agent "a" has a file

	// Journal claims both "a" and "b" succeeded, but "b" has no file.
	journal := `{"type":"result","key":"k1","agentId":"a"}` + "\n" +
		`{"type":"result","key":"k2","agentId":"b"}` + "\n"
	wa.ProcessJournal("wf-1", []byte(journal))

	runs := wa.Result()
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].AgentCount != 1 {
		t.Errorf("AgentCount = %d, want 1", runs[0].AgentCount)
	}
	if runs[0].SucceededAgents != 1 {
		t.Errorf("SucceededAgents = %d, want 1 (the fileless agent 'b' must not count)", runs[0].SucceededAgents)
	}
}

// TestWorkflowsAnalyzer_JournalForUnknownRunIgnored: a journal for a run with no
// agent files does not fabricate a phantom run.
func TestWorkflowsAnalyzer_JournalForUnknownRunIgnored(t *testing.T) {
	wa := &WorkflowsAnalyzer{}
	wa.ProcessJournal("ghost", []byte(`{"type":"result","key":"k","agentId":"a"}`+"\n"))
	if got := wa.Result(); len(got) != 0 {
		t.Errorf("len(runs) = %d, want 0", len(got))
	}
}
