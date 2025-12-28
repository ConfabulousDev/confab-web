package analytics

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestComputeFromJSONL_TokenStats(t *testing.T) {
	// Sample JSONL with two assistant messages
	jsonl := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":30}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":200,"output_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":50}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z"}
`

	result, err := ComputeFromJSONL([]byte(jsonl))
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	// Check token sums
	if result.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", result.InputTokens)
	}
	if result.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150", result.OutputTokens)
	}
	if result.CacheCreationTokens != 20 {
		t.Errorf("CacheCreationTokens = %d, want 20", result.CacheCreationTokens)
	}
	if result.CacheReadTokens != 80 {
		t.Errorf("CacheReadTokens = %d, want 80", result.CacheReadTokens)
	}

	// Check cost is computed
	if result.EstimatedCostUSD.IsZero() {
		t.Error("EstimatedCostUSD should not be zero")
	}
}

func TestComputeFromJSONL_CompactionStats(t *testing.T) {
	// Sample JSONL with compaction boundaries
	jsonl := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:10Z"}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z"}
{"type":"user","message":{"role":"user","content":"continue"},"uuid":"u2","timestamp":"2025-01-01T00:01:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":80,"output_tokens":40}},"uuid":"a2","timestamp":"2025-01-01T00:01:10Z"}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"manual","preTokens":60000},"logicalParentUuid":"a2","uuid":"c2","timestamp":"2025-01-01T00:02:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":90,"output_tokens":45}},"uuid":"a3","timestamp":"2025-01-01T00:02:20Z"}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":70000},"logicalParentUuid":"a3","uuid":"c3","timestamp":"2025-01-01T00:02:30Z"}
`

	result, err := ComputeFromJSONL([]byte(jsonl))
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	// Check compaction counts
	if result.CompactionAuto != 2 {
		t.Errorf("CompactionAuto = %d, want 2", result.CompactionAuto)
	}
	if result.CompactionManual != 1 {
		t.Errorf("CompactionManual = %d, want 1", result.CompactionManual)
	}

	// Check average compaction time (only for auto)
	// First auto: 00:00:15 - 00:00:10 = 5 seconds = 5000ms
	// Second auto: 00:02:30 - 00:02:20 = 10 seconds = 10000ms
	// Average = (5000 + 10000) / 2 = 7500ms
	if result.CompactionAvgTimeMs == nil {
		t.Fatal("CompactionAvgTimeMs should not be nil")
	}
	if *result.CompactionAvgTimeMs != 7500 {
		t.Errorf("CompactionAvgTimeMs = %d, want 7500", *result.CompactionAvgTimeMs)
	}
}

func TestComputeFromJSONL_EmptyContent(t *testing.T) {
	result, err := ComputeFromJSONL([]byte{})
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	if result.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", result.InputTokens)
	}
	if !result.EstimatedCostUSD.Equal(decimal.Zero) {
		t.Errorf("EstimatedCostUSD = %s, want 0", result.EstimatedCostUSD)
	}
	if result.CompactionAvgTimeMs != nil {
		t.Errorf("CompactionAvgTimeMs = %v, want nil", result.CompactionAvgTimeMs)
	}
}

func TestComputeFromJSONL_MalformedLines(t *testing.T) {
	// Should skip malformed lines without error
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
not valid json
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z"}
`

	result, err := ComputeFromJSONL([]byte(jsonl))
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	// Should have processed the two valid lines
	if result.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200", result.InputTokens)
	}
}

func TestToSessionAnalytics(t *testing.T) {
	result := &ComputeResult{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 100,
		CacheReadTokens:     200,
		EstimatedCostUSD:    decimal.NewFromFloat(1.50),
		CompactionAuto:      2,
		CompactionManual:    1,
	}

	analytics := result.ToSessionAnalytics("session-123", 1, 500)

	if analytics.SessionID != "session-123" {
		t.Errorf("SessionID = %s, want session-123", analytics.SessionID)
	}
	if analytics.AnalyticsVersion != 1 {
		t.Errorf("AnalyticsVersion = %d, want 1", analytics.AnalyticsVersion)
	}
	if analytics.UpToLine != 500 {
		t.Errorf("UpToLine = %d, want 500", analytics.UpToLine)
	}
	if analytics.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", analytics.InputTokens)
	}
	if analytics.CompactionAuto != 2 {
		t.Errorf("CompactionAuto = %d, want 2", analytics.CompactionAuto)
	}
}

func TestToResponse(t *testing.T) {
	avgTime := 5000
	analytics := &SessionAnalytics{
		UpToLine:            1500,
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 100,
		CacheReadTokens:     200,
		EstimatedCostUSD:    decimal.NewFromFloat(1.50),
		CompactionAuto:      2,
		CompactionManual:    1,
		CompactionAvgTimeMs: &avgTime,
	}

	response := analytics.ToResponse()

	if response.ComputedLines != 1500 {
		t.Errorf("ComputedLines = %d, want 1500", response.ComputedLines)
	}
	if response.Tokens.Input != 1000 {
		t.Errorf("Tokens.Input = %d, want 1000", response.Tokens.Input)
	}
	if response.Tokens.Output != 500 {
		t.Errorf("Tokens.Output = %d, want 500", response.Tokens.Output)
	}
	if !response.Cost.EstimatedUSD.Equal(decimal.NewFromFloat(1.50)) {
		t.Errorf("Cost.EstimatedUSD = %s, want 1.50", response.Cost.EstimatedUSD)
	}
	if response.Compaction.Auto != 2 {
		t.Errorf("Compaction.Auto = %d, want 2", response.Compaction.Auto)
	}
	if *response.Compaction.AvgTimeMs != 5000 {
		t.Errorf("Compaction.AvgTimeMs = %d, want 5000", *response.Compaction.AvgTimeMs)
	}
}
