package codex

import (
	"encoding/json"
	"strings"
	"testing"
)

// wireUsage mirrors one total_token_usage / last_token_usage object on the
// rollout wire.
type wireUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	CacheWriteInputTokens int64 `json:"cache_write_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

// tokenCountLine builds one event_msg/token_count rollout line. A nil last
// omits last_token_usage entirely (the total-only wire shape).
func tokenCountLine(t *testing.T, total wireUsage, last *wireUsage) string {
	t.Helper()
	info := map[string]any{"total_token_usage": total, "model_context_window": 258400}
	if last != nil {
		info["last_token_usage"] = *last
	}
	line, err := json.Marshal(map[string]any{
		"timestamp": "2026-11-18T01:00:00.000Z",
		"type":      "event_msg",
		"payload":   map[string]any{"type": "token_count", "info": info},
	})
	if err != nil {
		t.Fatalf("marshal token_count line: %v", err)
	}
	return string(line)
}

// parseUsage parses the given rollout lines and returns the reported usage.
func parseUsage(t *testing.T, lines ...string) TokenUsage {
	t.Helper()
	rollout, err := ParseRollout([]byte(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	return rollout.TokenUsage
}

func assertUsage(t *testing.T, got, want TokenUsage, why string) {
	t.Helper()
	if got != want {
		t.Errorf("TokenUsage = %+v, want %+v (%s)", got, want, why)
	}
}

// Real values from a Codex 0.170.0 `codex exec` + `codex exec fork` pair
// (23h5). The fork's running total is seeded with the parent's 17,270 tokens.
const forkSessionMeta = `{"timestamp":"2026-11-18T01:00:00.000Z","type":"session_meta","payload":{"id":"01a1f111-7789-7931-8002-b457733e7ab4","forked_from_id":"01a1f111-3621-7d30-8616-8a4d07adbc02","parent_thread_id":null,"history_mode":"paginated","history_base":{"thread_id":"01a1f111-3621-7d30-8616-8a4d07adbc02","end_ordinal_exclusive":18,"end_byte_offset":131096},"source":"exec","thread_source":"user","model":"gpt-5"}}`

var (
	forkFirstTotal = wireUsage{InputTokens: 39841, CachedInputTokens: 21504, OutputTokens: 10, TotalTokens: 39851}
	forkFirstLast  = wireUsage{InputTokens: 22576, CachedInputTokens: 10752, OutputTokens: 5, TotalTokens: 22581}
)

func TestParseRollout_ForkReportsOnlyItsOwnUsage(t *testing.T) {
	got := parseUsage(t, forkSessionMeta, tokenCountLine(t, forkFirstTotal, &forkFirstLast))
	assertUsage(t, got, TokenUsage{
		InputTokens:       22576,
		CachedInputTokens: 10752,
		OutputTokens:      5,
		TotalTokens:       22581,
	}, "a fork must report only its own usage, not the parent's inherited 17,270")
}

func TestParseRollout_MultiRequestForkSumsItsOwnDeltas(t *testing.T) {
	second := wireUsage{InputTokens: 30000, CachedInputTokens: 22000, OutputTokens: 40, ReasoningOutputTokens: 12, TotalTokens: 30040}
	secondTotal := wireUsage{
		InputTokens:           forkFirstTotal.InputTokens + second.InputTokens,
		CachedInputTokens:     forkFirstTotal.CachedInputTokens + second.CachedInputTokens,
		OutputTokens:          forkFirstTotal.OutputTokens + second.OutputTokens,
		ReasoningOutputTokens: second.ReasoningOutputTokens,
		TotalTokens:           forkFirstTotal.TotalTokens + second.TotalTokens,
	}
	got := parseUsage(t, forkSessionMeta,
		tokenCountLine(t, forkFirstTotal, &forkFirstLast),
		tokenCountLine(t, secondTotal, &second),
	)
	assertUsage(t, got, TokenUsage{
		InputTokens:           22576 + 30000,
		CachedInputTokens:     10752 + 22000,
		OutputTokens:          5 + 40,
		ReasoningOutputTokens: 12,
		TotalTokens:           22581 + 30040,
	}, "a multi-request fork must report the sum of its own per-request deltas")
}

func TestParseRollout_NonForkReportsUnchangedTotal(t *testing.T) {
	first := wireUsage{InputTokens: 1000, CachedInputTokens: 200, OutputTokens: 150, ReasoningOutputTokens: 50, TotalTokens: 1150}
	second := wireUsage{InputTokens: 2000, CachedInputTokens: 900, OutputTokens: 30, TotalTokens: 2030}
	finalTotal := wireUsage{InputTokens: 3000, CachedInputTokens: 1100, OutputTokens: 180, ReasoningOutputTokens: 50, TotalTokens: 3180}
	got := parseUsage(t,
		tokenCountLine(t, first, &first),
		tokenCountLine(t, finalTotal, &second),
	)
	assertUsage(t, got, TokenUsage{
		InputTokens:           3000,
		CachedInputTokens:     1100,
		OutputTokens:          180,
		ReasoningOutputTokens: 50,
		TotalTokens:           3180,
	}, "a non-fork rollout with no restart must report its final running total")
}

func TestParseRollout_RunningTotalRestartCountsBothSides(t *testing.T) {
	before := wireUsage{InputTokens: 5000, CachedInputTokens: 3000, OutputTokens: 200, ReasoningOutputTokens: 20, TotalTokens: 5200}
	// Resume: the running total restarts below the previous one.
	after := wireUsage{InputTokens: 900, CachedInputTokens: 100, OutputTokens: 80, TotalTokens: 980}
	got := parseUsage(t,
		tokenCountLine(t, before, &before),
		tokenCountLine(t, after, &after),
	)
	assertUsage(t, got, TokenUsage{
		InputTokens:           5900,
		CachedInputTokens:     3100,
		OutputTokens:          280,
		ReasoningOutputTokens: 20,
		TotalTokens:           6180,
	}, "usage before and after a running-total restart must both be counted")
}

// Shape seen on Codex 0.130.0: at a compaction the running total becomes
// all-zero categories with total_tokens = the context window and an all-zero
// last; the same total is then re-emitted with a non-zero last.total_tokens.
func TestParseRollout_CompactionResetNeitherOverNorUnderCounts(t *testing.T) {
	before := wireUsage{InputTokens: 1000, CachedInputTokens: 400, OutputTokens: 50, ReasoningOutputTokens: 10, TotalTokens: 1050}
	resetTotal := wireUsage{TotalTokens: 258400}
	zeroLast := wireUsage{}
	repeatLast := wireUsage{TotalTokens: 8581}
	after := wireUsage{InputTokens: 9000, CachedInputTokens: 2000, OutputTokens: 20, TotalTokens: 9020}
	got := parseUsage(t,
		tokenCountLine(t, before, &before),
		tokenCountLine(t, resetTotal, &zeroLast),
		tokenCountLine(t, resetTotal, &repeatLast),
		tokenCountLine(t, resetTotal, &repeatLast),
		tokenCountLine(t, after, &after),
	)
	assertUsage(t, got, TokenUsage{
		InputTokens:           10000,
		CachedInputTokens:     2400,
		OutputTokens:          70,
		ReasoningOutputTokens: 10,
		TotalTokens:           10070,
	}, "a compaction reset must not drop earlier usage nor count the re-emitted totals")
}

func TestParseRollout_RepeatedIdenticalTokenCountCountedOnce(t *testing.T) {
	first := wireUsage{InputTokens: 100, CachedInputTokens: 40, OutputTokens: 10, TotalTokens: 110}
	second := wireUsage{InputTokens: 50, OutputTokens: 5, TotalTokens: 55}
	secondTotal := wireUsage{InputTokens: 150, CachedInputTokens: 40, OutputTokens: 15, TotalTokens: 165}
	got := parseUsage(t,
		tokenCountLine(t, first, &first),
		tokenCountLine(t, first, &first),
		tokenCountLine(t, secondTotal, &second),
		tokenCountLine(t, secondTotal, &second),
		tokenCountLine(t, secondTotal, &second),
	)
	assertUsage(t, got, TokenUsage{
		InputTokens:       150,
		CachedInputTokens: 40,
		OutputTokens:      15,
		TotalTokens:       165,
	}, "a re-emitted token_count with an unchanged total must be counted once")
}

func TestParseRollout_FirstEventEqualToZeroTotalIsNotSkipped(t *testing.T) {
	// No previous total exists, so even an all-zero first event is not a
	// repeat; the following real request must still be counted.
	zero := wireUsage{}
	first := wireUsage{InputTokens: 70, OutputTokens: 7, TotalTokens: 77}
	got := parseUsage(t,
		tokenCountLine(t, zero, &zero),
		tokenCountLine(t, first, &first),
	)
	assertUsage(t, got, TokenUsage{InputTokens: 70, OutputTokens: 7, TotalTokens: 77},
		"the first token_count is never a repeat")
}

func TestParseRollout_TotalOnlyEventsFallBackToTotalDifference(t *testing.T) {
	first := wireUsage{InputTokens: 100, CachedInputTokens: 20, OutputTokens: 10, ReasoningOutputTokens: 2, TotalTokens: 110}
	grown := wireUsage{InputTokens: 250, CachedInputTokens: 120, OutputTokens: 30, ReasoningOutputTokens: 6, TotalTokens: 280}
	// input_tokens went down: a restart, so the whole total is added.
	restarted := wireUsage{InputTokens: 40, CachedInputTokens: 130, OutputTokens: 35, ReasoningOutputTokens: 7, TotalTokens: 75}

	cases := []struct {
		name  string
		lines []string
		want  TokenUsage
		why   string
	}{
		{
			name:  "first event adds the whole total",
			lines: []string{tokenCountLine(t, first, nil)},
			want:  TokenUsage{InputTokens: 100, CachedInputTokens: 20, OutputTokens: 10, ReasoningOutputTokens: 2, TotalTokens: 110},
			why:   "a first event without last_token_usage must add its whole total",
		},
		{
			name:  "increasing total adds the difference",
			lines: []string{tokenCountLine(t, first, nil), tokenCountLine(t, grown, nil)},
			want:  TokenUsage{InputTokens: 250, CachedInputTokens: 120, OutputTokens: 30, ReasoningOutputTokens: 6, TotalTokens: 280},
			why:   "an increasing total without last_token_usage must add (this total - previous total)",
		},
		{
			name:  "decreasing total adds the whole total",
			lines: []string{tokenCountLine(t, first, nil), tokenCountLine(t, grown, nil), tokenCountLine(t, restarted, nil)},
			want:  TokenUsage{InputTokens: 290, CachedInputTokens: 250, OutputTokens: 65, ReasoningOutputTokens: 13, TotalTokens: 355},
			why:   "a total without last_token_usage where any category went down must add the whole total",
		},
		{
			name:  "total-only event after a fork's seeded event adds the difference",
			lines: []string{tokenCountLine(t, forkFirstTotal, &forkFirstLast), tokenCountLine(t, wireUsage{InputTokens: 40000, CachedInputTokens: 21600, OutputTokens: 30, TotalTokens: 40030}, nil)},
			want:  TokenUsage{InputTokens: 22576 + 159, CachedInputTokens: 10752 + 96, OutputTokens: 5 + 20, TotalTokens: 22581 + 179},
			why:   "the fallback difference is taken against the previous total, not the accumulated usage",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertUsage(t, parseUsage(t, tc.lines...), tc.want, tc.why)
		})
	}
}

func TestParseRollout_CarriesCacheWriteInputTokens(t *testing.T) {
	first := wireUsage{InputTokens: 1000, CachedInputTokens: 200, CacheWriteInputTokens: 300, OutputTokens: 10, TotalTokens: 1010}
	second := wireUsage{InputTokens: 500, CacheWriteInputTokens: 120, OutputTokens: 5, TotalTokens: 505}
	secondTotal := wireUsage{InputTokens: 1500, CachedInputTokens: 200, CacheWriteInputTokens: 420, OutputTokens: 15, TotalTokens: 1515}
	// Total-only third event: cache-write goes through the fallback too.
	thirdTotal := wireUsage{InputTokens: 1600, CachedInputTokens: 200, CacheWriteInputTokens: 450, OutputTokens: 16, TotalTokens: 1616}
	got := parseUsage(t,
		tokenCountLine(t, first, &first),
		tokenCountLine(t, secondTotal, &second),
		tokenCountLine(t, thirdTotal, nil),
	)
	if got.CacheWriteInputTokens != 450 {
		t.Errorf("TokenUsage.CacheWriteInputTokens = %d, want 450 (cache_write_input_tokens must be carried through the parser)", got.CacheWriteInputTokens)
	}
}

func TestParseRollout_TokenCountWithoutTotalIsIgnored(t *testing.T) {
	first := wireUsage{InputTokens: 100, OutputTokens: 10, TotalTokens: 110}
	second := wireUsage{InputTokens: 60, OutputTokens: 6, TotalTokens: 66}
	secondTotal := wireUsage{InputTokens: 160, OutputTokens: 16, TotalTokens: 176}
	got := parseUsage(t,
		tokenCountLine(t, first, &first),
		`{"timestamp":"2026-11-18T01:00:01.000Z","type":"event_msg","payload":{"type":"token_count","info":null}}`,
		`{"timestamp":"2026-11-18T01:00:02.000Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":999,"output_tokens":9,"total_tokens":1008}}}}`,
		tokenCountLine(t, secondTotal, &second),
	)
	assertUsage(t, got, TokenUsage{InputTokens: 160, OutputTokens: 16, TotalTokens: 176},
		"token_count events with null info or no total_token_usage must be ignored")
}
