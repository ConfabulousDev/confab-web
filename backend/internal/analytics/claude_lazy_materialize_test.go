package analytics

import (
	"context"
	"sync/atomic"
	"testing"
)

// TestClaudeRollout_AgentsCachedAcrossComputeAndPrepareTranscript verifies
// the lazy-materialize contract on the claudeProvider: agent files are
// streamed from storage exactly once across ComputeCards + PrepareTranscript
// calls on the same rollout instance, even when both methods iterate the
// agent set.
//
// Today (Phase 3b stub) each method invokes a fresh AgentProvider that
// re-downloads every agent file, so the counter reaches 2x the agent count.
// Phase 4 implements the cache; the test then sees N downloads total.
func TestClaudeRollout_AgentsCachedAcrossComputeAndPrepareTranscript(t *testing.T) {
	mainJsonl := makeAssistantMessage("u1", "2026-05-16T10:00:00Z", "claude-sonnet-4-6", 50, 25, []map[string]interface{}{
		makeTextBlock("main response"),
	}) + "\n"
	main, err := parseTranscriptFile([]byte(mainJsonl), "")
	if err != nil {
		t.Fatalf("parse main: %v", err)
	}

	agentJsonl := makeAssistantMessage("a1", "2026-05-16T10:00:01Z", "claude-sonnet-4-6", 30, 15, []map[string]interface{}{
		makeTextBlock("agent response"),
	}) + "\n"

	const agentCount = 3
	var downloadCount int64
	downloader := func(_ context.Context, fileName string) ([]byte, error) {
		atomic.AddInt64(&downloadCount, 1)
		return []byte(agentJsonl), nil
	}

	agentInfo := make([]AgentFileInfo, agentCount)
	for i := range agentInfo {
		agentInfo[i] = AgentFileInfo{
			FileName: "agent-" + string(rune('a'+i)) + ".jsonl",
			AgentID:  string(rune('a' + i)),
		}
	}

	rollout := &claudeRollout{
		main:       main,
		agentInfo:  agentInfo,
		downloader: downloader,
	}

	sp := &claudeProvider{}
	ctx := context.Background()

	_ = sp.ComputeCards(ctx, rollout)
	_, _, _ = sp.PrepareTranscript(ctx, rollout)

	if got := atomic.LoadInt64(&downloadCount); got != agentCount {
		t.Errorf("agent downloader invocations = %d, want %d (cache should prevent the second stream from re-downloading)", got, agentCount)
	}
}

// TestClaudeRollout_SearchTextReusesAgentCache extends the lazy-materialize
// contract to the third reader, SearchText. After ComputeCards primes the
// cache, a subsequent SearchText call must NOT trigger any additional
// downloads.
func TestClaudeRollout_SearchTextReusesAgentCache(t *testing.T) {
	mainJsonl := makeAssistantMessage("u1", "2026-05-16T10:00:00Z", "claude-sonnet-4-6", 50, 25, []map[string]interface{}{
		makeTextBlock("main response"),
	}) + "\n"
	main, err := parseTranscriptFile([]byte(mainJsonl), "")
	if err != nil {
		t.Fatalf("parse main: %v", err)
	}

	agentJsonl := makeAssistantMessage("a1", "2026-05-16T10:00:01Z", "claude-sonnet-4-6", 30, 15, []map[string]interface{}{
		makeTextBlock("agent response"),
	}) + "\n"

	var downloadCount int64
	downloader := func(_ context.Context, fileName string) ([]byte, error) {
		atomic.AddInt64(&downloadCount, 1)
		return []byte(agentJsonl), nil
	}

	rollout := &claudeRollout{
		main: main,
		agentInfo: []AgentFileInfo{
			{FileName: "agent-a.jsonl", AgentID: "a"},
			{FileName: "agent-b.jsonl", AgentID: "b"},
		},
		downloader: downloader,
	}

	sp := &claudeProvider{}
	ctx := context.Background()

	_ = sp.ComputeCards(ctx, rollout)
	primed := atomic.LoadInt64(&downloadCount)

	_ = sp.SearchText(ctx, rollout)

	if got := atomic.LoadInt64(&downloadCount) - primed; got != 0 {
		t.Errorf("SearchText after ComputeCards triggered %d extra agent downloads, want 0 (cache should serve all reads after first traversal)", got)
	}
}
