package analytics

import (
	"context"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// newCountingClaudeRollout builds a claudeRollout over agentCount identical
// agent files whose downloader counts invocations. Each agent carries a
// distinctive user prompt so transcript/search output can be checked for it.
func newCountingClaudeRollout(t *testing.T, agentCount int) (*claudeRollout, *int64) {
	t.Helper()
	mainJsonl := makeAssistantMessage("u1", "2026-05-16T10:00:00Z", "claude-sonnet-4-6", 50, 25, []map[string]any{
		makeTextBlock("main response"),
	}) + "\n"
	main, err := parseTranscriptFile([]byte(mainJsonl), "")
	if err != nil {
		t.Fatalf("parse main: %v", err)
	}

	agentJsonl := makeUserMessage("au1", "2026-05-16T10:00:01Z", "agent-side prompt") + "\n" +
		makeAssistantMessage("a1", "2026-05-16T10:00:02Z", "claude-sonnet-4-6", 30, 15, []map[string]any{
			makeTextBlock("agent response"),
		}) + "\n"

	var downloadCount int64
	agentInfo := make([]AgentFileInfo, agentCount)
	for i := range agentInfo {
		id := string(rune('a' + i))
		agentInfo[i] = AgentFileInfo{FileName: "agent-" + id + ".jsonl", AgentID: id}
	}
	return &claudeRollout{
		main:      main,
		agentInfo: agentInfo,
		downloader: func(_ context.Context, _ string) ([]byte, error) {
			atomic.AddInt64(&downloadCount, 1)
			return []byte(agentJsonl), nil
		},
	}, &downloadCount
}

// TestClaudeRollout_EachTraversalStreamsEachAgentOnce pins the 5m68 streaming
// contract: a rollout does not memoize parsed agent files, so every traversal
// (ComputeCards, PrepareTranscript, SearchText) streams each agent exactly once
// and nothing is retained between traversals.
func TestClaudeRollout_EachTraversalStreamsEachAgentOnce(t *testing.T) {
	const agentCount = 3
	rollout, downloads := newCountingClaudeRollout(t, agentCount)
	sp := &claudeProvider{}
	ctx := context.Background()

	_ = sp.ComputeCards(ctx, rollout)
	if got := atomic.LoadInt64(downloads); got != agentCount {
		t.Fatalf("after ComputeCards: agent downloads = %d, want %d (one per agent)", got, agentCount)
	}

	_, _, _ = sp.PrepareTranscript(ctx, rollout)
	if got := atomic.LoadInt64(downloads); got != 2*agentCount {
		t.Errorf("after PrepareTranscript: agent downloads = %d, want %d (a second traversal must re-stream, not replay a memo)", got, 2*agentCount)
	}

	_ = sp.SearchText(ctx, rollout)
	if got := atomic.LoadInt64(downloads); got != 3*agentCount {
		t.Errorf("after SearchText: agent downloads = %d, want %d", got, 3*agentCount)
	}
}

// TestClaudeRollout_TraversalsIncludeAgentContent verifies that each reader
// sees agent content when it is the first (and only) traversal.
func TestClaudeRollout_TraversalsIncludeAgentContent(t *testing.T) {
	sp := &claudeProvider{}
	ctx := context.Background()

	rollout, _ := newCountingClaudeRollout(t, 2)
	transcript, _, err := sp.PrepareTranscript(ctx, rollout)
	if err != nil {
		t.Fatalf("PrepareTranscript: %v", err)
	}
	if !strings.Contains(transcript, "agent response") {
		t.Errorf("PrepareTranscript output must include streamed agent content; got %q", transcript)
	}

	rollout, _ = newCountingClaudeRollout(t, 2)
	if text := sp.SearchText(ctx, rollout); !strings.Contains(text, "agent-side prompt") {
		t.Errorf("SearchText output must include streamed agent user prompts; got %q", text)
	}
}

// TestClaudeRollout_RepeatedComputeCardsResultsUnchanged verifies that
// streaming (no memo) yields identical card results on every traversal.
func TestClaudeRollout_RepeatedComputeCardsResultsUnchanged(t *testing.T) {
	rollout, _ := newCountingClaudeRollout(t, 3)
	sp := &claudeProvider{}
	ctx := context.Background()

	first := sp.ComputeCards(ctx, rollout)
	second := sp.ComputeCards(ctx, rollout)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("ComputeCards results differ across traversals of the same rollout:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if first.TotalMessages == 0 {
		t.Error("ComputeCards produced no messages; fixture did not flow through the analyzers")
	}
}

// TestTranscriptLineRetainsNoRawJSON guards the 5m68 memory fix: parsed lines
// keep only the typed struct, never a generic map[string]any copy of the line.
func TestTranscriptLineRetainsNoRawJSON(t *testing.T) {
	if _, ok := reflect.TypeFor[TranscriptLine]().FieldByName("RawData"); ok {
		t.Error("TranscriptLine must not retain a RawData copy of the parsed JSON (5m68: ~6x heap amplification)")
	}
}

// TestParseTranscriptFileCountsRedactionsPerFile verifies that redaction
// markers are counted at parse time, per file, from valid lines only.
func TestParseTranscriptFileCountsRedactionsPerFile(t *testing.T) {
	tf, err := parseTranscriptFile(redactionParityMain(t), "")
	if err != nil {
		t.Fatalf("parseTranscriptFile: %v", err)
	}
	want := map[string]int{"GITHUB_TOKEN": 1, "AWS_KEY": 2, "PASSWORD": 2, "ESCAPED": 1}
	if !reflect.DeepEqual(tf.RedactionCounts, want) {
		t.Errorf("main file RedactionCounts = %v, want %v", tf.RedactionCounts, want)
	}

	empty, err := parseTranscriptFile([]byte(makeUserMessage("u1", "2025-01-01T00:00:00Z", "nothing secret")+"\n"), "")
	if err != nil {
		t.Fatalf("parseTranscriptFile: %v", err)
	}
	if len(empty.RedactionCounts) != 0 {
		t.Errorf("file without markers: RedactionCounts = %v, want empty", empty.RedactionCounts)
	}
}
