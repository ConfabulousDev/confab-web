package analytics

import (
	"testing"
)

func TestRunCollectors_LineCount(t *testing.T) {
	jsonl := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":"world"},"uuid":"u2","timestamp":"2025-01-01T00:00:02Z"}
`

	ctx, err := RunCollectors([]byte(jsonl))
	if err != nil {
		t.Fatalf("RunCollectors failed: %v", err)
	}

	if ctx.LineCount != 3 {
		t.Errorf("LineCount = %d, want 3", ctx.LineCount)
	}
}

func TestRunCollectors_TimestampMap(t *testing.T) {
	jsonl := `{"type":"user","uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","uuid":"a1","timestamp":"2025-01-01T00:00:01Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
`

	ctx, err := RunCollectors([]byte(jsonl))
	if err != nil {
		t.Fatalf("RunCollectors failed: %v", err)
	}

	if _, ok := ctx.TimestampByUUID["u1"]; !ok {
		t.Error("TimestampByUUID should contain u1")
	}
	if _, ok := ctx.TimestampByUUID["a1"]; !ok {
		t.Error("TimestampByUUID should contain a1")
	}
}

func TestTokensCollector(t *testing.T) {
	tokens := NewTokensCollector()

	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":30}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":200,"output_tokens":100}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z"}
`

	_, err := RunCollectors([]byte(jsonl), tokens)
	if err != nil {
		t.Fatalf("RunCollectors failed: %v", err)
	}

	if tokens.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", tokens.InputTokens)
	}
	if tokens.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150", tokens.OutputTokens)
	}
	if tokens.CacheCreationTokens != 20 {
		t.Errorf("CacheCreationTokens = %d, want 20", tokens.CacheCreationTokens)
	}
	if tokens.CacheReadTokens != 30 {
		t.Errorf("CacheReadTokens = %d, want 30", tokens.CacheReadTokens)
	}
	if tokens.EstimatedCostUSD.IsZero() {
		t.Error("EstimatedCostUSD should not be zero")
	}
}

func TestCompactionCollector(t *testing.T) {
	compaction := NewCompactionCollector()

	jsonl := `{"type":"assistant","uuid":"a1","timestamp":"2025-01-01T00:00:10Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z"}
{"type":"assistant","uuid":"a2","timestamp":"2025-01-01T00:01:00Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":80,"output_tokens":40}}}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"manual","preTokens":60000},"logicalParentUuid":"a2","uuid":"c2","timestamp":"2025-01-01T00:02:00Z"}
`

	_, err := RunCollectors([]byte(jsonl), compaction)
	if err != nil {
		t.Fatalf("RunCollectors failed: %v", err)
	}

	if compaction.AutoCount != 1 {
		t.Errorf("AutoCount = %d, want 1", compaction.AutoCount)
	}
	if compaction.ManualCount != 1 {
		t.Errorf("ManualCount = %d, want 1", compaction.ManualCount)
	}
	if compaction.AvgTimeMs == nil {
		t.Fatal("AvgTimeMs should not be nil")
	}
	// a1 at 00:00:10, c1 at 00:00:15 = 5000ms
	if *compaction.AvgTimeMs != 5000 {
		t.Errorf("AvgTimeMs = %d, want 5000", *compaction.AvgTimeMs)
	}
}

func TestMultipleCollectors(t *testing.T) {
	tokens := NewTokensCollector()
	compaction := NewCompactionCollector()

	jsonl := `{"type":"assistant","uuid":"a1","timestamp":"2025-01-01T00:00:10Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z"}
`

	_, err := RunCollectors([]byte(jsonl), tokens, compaction)
	if err != nil {
		t.Fatalf("RunCollectors failed: %v", err)
	}

	// Verify both collectors received data
	if tokens.InputTokens != 100 {
		t.Errorf("tokens.InputTokens = %d, want 100", tokens.InputTokens)
	}
	if compaction.AutoCount != 1 {
		t.Errorf("compaction.AutoCount = %d, want 1", compaction.AutoCount)
	}
}
