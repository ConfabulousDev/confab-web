package analytics

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCardsAllValid(t *testing.T) {
	// Create sample cached card records with correct version constants
	makeCards := func(upToLine int64) *Cards {
		now := time.Now().UTC()
		return &Cards{
			Tokens: &TokensCardRecord{
				SessionID:   "test-session",
				Version:     TokensCardVersion,
				ComputedAt:  now,
				UpToLine:    upToLine,
				InputTokens: 1000,
			},
			Cost: &CostCardRecord{
				SessionID:        "test-session",
				Version:          CostCardVersion,
				ComputedAt:       now,
				UpToLine:         upToLine,
				EstimatedCostUSD: decimal.NewFromFloat(1.50),
			},
			Compaction: &CompactionCardRecord{
				SessionID:  "test-session",
				Version:    CompactionCardVersion,
				ComputedAt: now,
				UpToLine:   upToLine,
				AutoCount:  2,
			},
			Session: &SessionCardRecord{
				SessionID:      "test-session",
				Version:        SessionCardVersion,
				ComputedAt:     now,
				UpToLine:       upToLine,
				UserTurns:      5,
				AssistantTurns: 5,
				ModelsUsed:     []string{"claude-sonnet-4"},
			},
			Tools: &ToolsCardRecord{
				SessionID:  "test-session",
				Version:    ToolsCardVersion,
				ComputedAt: now,
				UpToLine:   upToLine,
				TotalCalls: 10,
				ToolStats: map[string]*ToolStats{
					"Read":  {Success: 5, Errors: 0},
					"Write": {Success: 3, Errors: 0},
					"Bash":  {Success: 1, Errors: 1},
				},
				ErrorCount: 1,
			},
		}
	}

	// Helper to create cards with a specific version mismatch
	makeCardsWithVersion := func(version int, upToLine int64) *Cards {
		cards := makeCards(upToLine)
		// Override all versions with the specified version (for testing version mismatch)
		cards.Tokens.Version = version
		cards.Cost.Version = version
		cards.Compaction.Version = version
		cards.Session.Version = version
		cards.Tools.Version = version
		return cards
	}

	t.Run("returns false when cards is nil", func(t *testing.T) {
		var cards *Cards
		if cards.AllValid(100) {
			t.Error("expected false for nil cards")
		}
	})

	t.Run("returns false when tokens card is nil", func(t *testing.T) {
		cards := makeCards(100)
		cards.Tokens = nil
		if cards.AllValid(100) {
			t.Error("expected false when tokens card is nil")
		}
	})

	t.Run("returns false when version mismatch", func(t *testing.T) {
		cards := makeCardsWithVersion(999, 100) // version 999 != any card version
		if cards.AllValid(100) {
			t.Error("expected false for version mismatch")
		}
	})

	t.Run("returns false when line count mismatch (new data synced)", func(t *testing.T) {
		cards := makeCards(100)
		currentLineCount := int64(150) // 50 new lines synced
		if cards.AllValid(currentLineCount) {
			t.Error("expected false for line count mismatch")
		}
	})

	t.Run("returns true when version and line count match", func(t *testing.T) {
		cards := makeCards(100)
		currentLineCount := int64(100)
		if !cards.AllValid(currentLineCount) {
			t.Error("expected true when both match")
		}
	})

	t.Run("returns true for zero line count when both match", func(t *testing.T) {
		cards := makeCards(0)
		currentLineCount := int64(0)
		if !cards.AllValid(currentLineCount) {
			t.Error("expected true for zero line count when matched")
		}
	})
}

func TestTokensCardRecordIsValid(t *testing.T) {
	t.Run("returns false when nil", func(t *testing.T) {
		var card *TokensCardRecord
		if card.IsValid(100) {
			t.Error("expected false for nil card")
		}
	})

	t.Run("returns false when version mismatch", func(t *testing.T) {
		card := &TokensCardRecord{Version: 999, UpToLine: 100}
		if card.IsValid(100) {
			t.Error("expected false for version mismatch")
		}
	})

	t.Run("returns false when line count mismatch", func(t *testing.T) {
		card := &TokensCardRecord{Version: TokensCardVersion, UpToLine: 100}
		if card.IsValid(150) {
			t.Error("expected false for line count mismatch")
		}
	})

	t.Run("returns true when valid", func(t *testing.T) {
		card := &TokensCardRecord{Version: TokensCardVersion, UpToLine: 100}
		if !card.IsValid(100) {
			t.Error("expected true when valid")
		}
	})
}
