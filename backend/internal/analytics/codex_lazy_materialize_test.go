package analytics

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// minimalCodexJSONL produces a parseable Codex JSONL byte stream with one
// user message + one assistant final. Each line follows the on-disk schema
// documented in internal/codex/types.go.
func minimalCodexJSONL(turnID string) []byte {
	const tpl = `{"timestamp":"2026-05-16T10:00:00Z","type":"session_meta","payload":{"id":"sess","model":"gpt-5","model_provider":"openai"}}
{"timestamp":"2026-05-16T10:00:01Z","type":"event_msg","payload":{"type":"task_started","task_id":"%s","started_at":"2026-05-16T10:00:01Z","model":"gpt-5"}}
{"timestamp":"2026-05-16T10:00:02Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}
{"timestamp":"2026-05-16T10:00:03Z","type":"response_item","payload":{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}}
{"timestamp":"2026-05-16T10:00:04Z","type":"event_msg","payload":{"type":"task_complete","task_id":"%s","completed_at":"2026-05-16T10:00:04Z","duration_ms":3000}}
`
	return []byte(strings.ReplaceAll(tpl, "%s", turnID))
}

// TestCodexRollout_SubagentsCachedAcrossComputeAndPrepareTranscript pins the
// lazy-materialize contract on codexProvider. After Phase 4 wires subagent
// discovery + caching, each subagent file's bytes are fetched once across all
// codexProvider analytics methods on the same rollout instance.
//
// Today (Phase 3b stub) codexProvider methods do not touch the agentFileInfo
// or downloader fields on codexRollout at all — the test counter stays at 0
// while the assertion expects agentCount.
func TestCodexRollout_SubagentsCachedAcrossComputeAndPrepareTranscript(t *testing.T) {
	mainBytes := minimalCodexJSONL("t-main")
	main, err := codex.ParseRollout(bytes.NewReader(mainBytes))
	if err != nil {
		t.Fatalf("parse main: %v", err)
	}

	agentBytes := minimalCodexJSONL("t-agent")
	const agentCount = 2
	var downloadCount int64
	downloader := func(_ context.Context, fileName string) ([]byte, error) {
		atomic.AddInt64(&downloadCount, 1)
		return agentBytes, nil
	}

	agentFileInfo := make([]codexAgentFileInfo, agentCount)
	for i := range agentFileInfo {
		agentFileInfo[i] = codexAgentFileInfo{FileName: "subagent-" + string(rune('a'+i)) + ".jsonl"}
	}

	rollout := &codexRollout{
		main:          main,
		agentFileInfo: agentFileInfo,
		downloader:    downloader,
	}

	sp := &codexProvider{}
	ctx := context.Background()

	_ = sp.ComputeCards(ctx, rollout)
	_, _, _ = sp.PrepareTranscript(ctx, rollout)

	if got := atomic.LoadInt64(&downloadCount); got != agentCount {
		t.Errorf("subagent downloader invocations = %d, want %d (subagents should be downloaded once and cached for later methods)", got, agentCount)
	}
}

// TestCodexRollout_AggregatesSubagentsIntoSlice ensures the parsed subagents
// reach the public ComputeFromCodexRollout slice. Phase 3b stub: codexRollout
// holds main only, so the slice ends up length 1 and the subagent's user
// message never contributes to the analytics result.
func TestCodexRollout_AggregatesSubagentsIntoSlice(t *testing.T) {
	mainBytes := minimalCodexJSONL("t-main")
	main, err := codex.ParseRollout(bytes.NewReader(mainBytes))
	if err != nil {
		t.Fatalf("parse main: %v", err)
	}

	// Subagent has its own token totals so aggregation is observable.
	agentBytes := codexJSONLWithTokens("gpt-5", codex.TokenUsage{
		InputTokens: 500, OutputTokens: 100, TotalTokens: 600,
	})

	downloader := func(_ context.Context, fileName string) ([]byte, error) {
		return agentBytes, nil
	}

	rollout := &codexRollout{
		main:          main,
		agentFileInfo: []codexAgentFileInfo{{FileName: "sub.jsonl"}},
		downloader:    downloader,
	}

	sp := &codexProvider{}
	ctx := context.Background()
	out := sp.ComputeCards(ctx, rollout)

	if out == nil {
		t.Fatal("ComputeCards returned nil")
	}
	// Main has one user message; subagent adds 500 input tokens. Until Phase 4
	// aggregation lands, InputTokens stays at the main rollout's value only.
	if out.InputTokens < 500 {
		t.Errorf("InputTokens = %d, want subagent tokens to contribute (>=500)", out.InputTokens)
	}
}

// codexJSONLWithTokens emits a parseable Codex JSONL stream whose token_count
// event makes the parser surface the supplied TokenUsage. Used by mocked
// subagent downloaders so the aggregation assertion is meaningful: even if
// the rest of the rollout content is empty, the token totals will roll up
// into ComputeResult.InputTokens / OutputTokens.
func codexJSONLWithTokens(model string, usage codex.TokenUsage) []byte {
	tokenInfo := struct {
		InputTokens           int64 `json:"input_tokens"`
		CachedInputTokens     int64 `json:"cached_input_tokens"`
		OutputTokens          int64 `json:"output_tokens"`
		ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
		TotalTokens           int64 `json:"total_tokens"`
	}{usage.InputTokens, usage.CachedInputTokens, usage.OutputTokens, usage.ReasoningOutputTokens, usage.TotalTokens}
	_ = tokenInfo
	// Construct two lines: session_meta + token_count event_msg. The parser
	// reads top-level token_count.info.total_token_usage into ParsedRollout.TokenUsage.
	const tpl = `{"timestamp":"2026-05-16T10:00:00Z","type":"session_meta","payload":{"id":"sess","model":"%MODEL%","model_provider":"openai"}}
{"timestamp":"2026-05-16T10:05:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":%IN%,"cached_input_tokens":%CACHED%,"output_tokens":%OUT%,"reasoning_output_tokens":%REASON%,"total_tokens":%TOTAL%}}}}
`
	s := strings.ReplaceAll(tpl, "%MODEL%", model)
	s = strings.ReplaceAll(s, "%IN%", itoa(usage.InputTokens))
	s = strings.ReplaceAll(s, "%CACHED%", itoa(usage.CachedInputTokens))
	s = strings.ReplaceAll(s, "%OUT%", itoa(usage.OutputTokens))
	s = strings.ReplaceAll(s, "%REASON%", itoa(usage.ReasoningOutputTokens))
	s = strings.ReplaceAll(s, "%TOTAL%", itoa(usage.TotalTokens))
	return []byte(s)
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
