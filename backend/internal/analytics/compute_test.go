package analytics

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

func TestComputeFromJSONL_TokenStats(t *testing.T) {
	// Sample JSONL with two assistant messages
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessageFull("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, 20, 30, []map[string]interface{}{
			makeTextBlock("Hi"),
		}) + "\n" +
		makeAssistantMessageFull("a2", "2025-01-01T00:00:02Z", "claude-sonnet-4-20241022", 200, 100, 0, 50, []map[string]interface{}{
			makeTextBlock("Hello"),
		}) + "\n"

	result, err := ComputeFromJSONL(context.Background(), []byte(jsonl))
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
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessage("a1", "2025-01-01T00:00:10Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("Hi")}) + "\n" +
		makeCompactBoundaryMessageWithParent("c1", "2025-01-01T00:00:15Z", "auto", 50000, "a1") + "\n" +
		makeUserMessage("u2", "2025-01-01T00:01:00Z", "continue") + "\n" +
		makeAssistantMessage("a2", "2025-01-01T00:01:10Z", "claude-sonnet-4", 80, 40, []map[string]interface{}{makeTextBlock("Continuing")}) + "\n" +
		makeCompactBoundaryMessageWithParent("c2", "2025-01-01T00:02:00Z", "manual", 60000, "a2") + "\n" +
		makeAssistantMessage("a3", "2025-01-01T00:02:20Z", "claude-sonnet-4", 90, 45, []map[string]interface{}{makeTextBlock("More")}) + "\n" +
		makeCompactBoundaryMessageWithParent("c3", "2025-01-01T00:02:30Z", "auto", 70000, "a3") + "\n"

	result, err := ComputeFromJSONL(context.Background(), []byte(jsonl))
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
	result, err := ComputeFromJSONL(context.Background(), []byte{})
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
	jsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("Hi")}) + "\n" +
		"not valid json\n" +
		makeAssistantMessage("a2", "2025-01-01T00:00:02Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("Hello")}) + "\n"

	result, err := ComputeFromJSONL(context.Background(), []byte(jsonl))
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	// Should have processed the two valid lines
	if result.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200", result.InputTokens)
	}
}

func TestToCards(t *testing.T) {
	result := &ComputeResult{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 100,
		CacheReadTokens:     200,
		EstimatedCostUSD:    decimal.NewFromFloat(1.50),
		UserTurns:           5,
		AssistantTurns:      4,
		ModelsUsed:          []string{"claude-sonnet-4"},
		CompactionAuto:      2,
		CompactionManual:    1,
	}

	cards := result.ToCards("session-123", 500)

	// Check tokens card (now includes cost)
	if cards.Tokens == nil {
		t.Fatal("Tokens card should not be nil")
	}
	if cards.Tokens.SessionID != "session-123" {
		t.Errorf("Tokens.SessionID = %s, want session-123", cards.Tokens.SessionID)
	}
	if cards.Tokens.Version != TokensCardVersion {
		t.Errorf("Tokens.Version = %d, want %d", cards.Tokens.Version, TokensCardVersion)
	}
	if cards.Tokens.UpToLine != 500 {
		t.Errorf("Tokens.UpToLine = %d, want 500", cards.Tokens.UpToLine)
	}
	if cards.Tokens.InputTokens != 1000 {
		t.Errorf("Tokens.InputTokens = %d, want 1000", cards.Tokens.InputTokens)
	}
	if !cards.Tokens.EstimatedCostUSD.Equal(decimal.NewFromFloat(1.50)) {
		t.Errorf("Tokens.EstimatedCostUSD = %s, want 1.50", cards.Tokens.EstimatedCostUSD)
	}

	// Verify ComputedAt is in UTC (catches timezone bugs)
	if cards.Tokens.ComputedAt.Location().String() != "UTC" {
		t.Errorf("Tokens.ComputedAt should be UTC, got %s", cards.Tokens.ComputedAt.Location())
	}
	if cards.Session.ComputedAt.Location().String() != "UTC" {
		t.Errorf("Session.ComputedAt should be UTC, got %s", cards.Session.ComputedAt.Location())
	}

	// Check session card (now includes compaction)
	if cards.Session == nil {
		t.Fatal("Session card should not be nil")
	}
	if cards.Session.CompactionAuto != 2 {
		t.Errorf("Session.CompactionAuto = %d, want 2", cards.Session.CompactionAuto)
	}
	if cards.Session.CompactionManual != 1 {
		t.Errorf("Session.CompactionManual = %d, want 1", cards.Session.CompactionManual)
	}
}

func TestCardsToResponse(t *testing.T) {
	avgTime := 5000
	cards := &Cards{
		Tokens: &TokensCardRecord{
			UpToLine:            1500,
			InputTokens:         1000,
			OutputTokens:        500,
			CacheCreationTokens: 100,
			CacheReadTokens:     200,
			EstimatedCostUSD:    decimal.NewFromFloat(1.50),
		},
		Session: &SessionCardRecord{
			ModelsUsed:          []string{"claude-sonnet-4"},
			CompactionAuto:      2,
			CompactionManual:    1,
			CompactionAvgTimeMs: &avgTime,
		},
		Tools: &ToolsCardRecord{
			TotalCalls: 10,
			ToolStats: map[string]*ToolStats{
				"Read": {Success: 5, Errors: 0},
			},
			ErrorCount: 0,
		},
	}

	response := cards.ToResponse()

	// Check legacy flat format
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

	// Check new cards format
	if response.Cards == nil {
		t.Fatal("Cards should not be nil")
	}
	if len(response.Cards) != 3 {
		t.Errorf("Cards length = %d, want 3", len(response.Cards))
	}

	// Verify tokens card (now includes cost)
	tokens, ok := response.Cards["tokens"].(TokensCardData)
	if !ok {
		t.Fatal("tokens card not found or wrong type")
	}
	if tokens.Input != 1000 {
		t.Errorf("cards.tokens.Input = %d, want 1000", tokens.Input)
	}
	if tokens.EstimatedUSD != "1.5" {
		t.Errorf("cards.tokens.EstimatedUSD = %s, want 1.5", tokens.EstimatedUSD)
	}

	// Verify session card (now includes compaction)
	session, ok := response.Cards["session"].(SessionCardData)
	if !ok {
		t.Fatal("session card not found or wrong type")
	}
	if session.CompactionAuto != 2 {
		t.Errorf("cards.session.CompactionAuto = %d, want 2", session.CompactionAuto)
	}

	// Verify tools card
	tools, ok := response.Cards["tools"].(ToolsCardData)
	if !ok {
		t.Fatal("tools card not found or wrong type")
	}
	if tools.TotalCalls != 10 {
		t.Errorf("cards.tools.TotalCalls = %d, want 10", tools.TotalCalls)
	}
}
