package analytics

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCardsAllValid(t *testing.T) {
	// Create sample cached card records
	makeCards := func(version int, upToLine int64) *Cards {
		now := time.Now().UTC()
		return &Cards{
			Tokens: &TokensCardRecord{
				SessionID:   "test-session",
				Version:     version,
				ComputedAt:  now,
				UpToLine:    upToLine,
				InputTokens: 1000,
			},
			Cost: &CostCardRecord{
				SessionID:        "test-session",
				Version:          version,
				ComputedAt:       now,
				UpToLine:         upToLine,
				EstimatedCostUSD: decimal.NewFromFloat(1.50),
			},
			Compaction: &CompactionCardRecord{
				SessionID:  "test-session",
				Version:    version,
				ComputedAt: now,
				UpToLine:   upToLine,
				AutoCount:  2,
			},
		}
	}

	t.Run("returns false when cards is nil", func(t *testing.T) {
		var cards *Cards
		if cards.AllValid(100) {
			t.Error("expected false for nil cards")
		}
	})

	t.Run("returns false when tokens card is nil", func(t *testing.T) {
		cards := makeCards(1, 100)
		cards.Tokens = nil
		if cards.AllValid(100) {
			t.Error("expected false when tokens card is nil")
		}
	})

	t.Run("returns false when version mismatch", func(t *testing.T) {
		cards := makeCards(999, 100) // version 999 != TokensCardVersion (1)
		if cards.AllValid(100) {
			t.Error("expected false for version mismatch")
		}
	})

	t.Run("returns false when line count mismatch (new data synced)", func(t *testing.T) {
		cards := makeCards(TokensCardVersion, 100)
		currentLineCount := int64(150) // 50 new lines synced
		if cards.AllValid(currentLineCount) {
			t.Error("expected false for line count mismatch")
		}
	})

	t.Run("returns true when version and line count match", func(t *testing.T) {
		cards := makeCards(TokensCardVersion, 100)
		currentLineCount := int64(100)
		if !cards.AllValid(currentLineCount) {
			t.Error("expected true when both match")
		}
	})

	t.Run("returns true for zero line count when both match", func(t *testing.T) {
		cards := makeCards(TokensCardVersion, 0)
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
