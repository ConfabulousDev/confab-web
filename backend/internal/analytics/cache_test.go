package analytics

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestIsCacheValid(t *testing.T) {
	// Create a sample cached analytics record
	makeCached := func(version int, upToLine int64) *SessionAnalytics {
		return &SessionAnalytics{
			SessionID:        "test-session",
			AnalyticsVersion: version,
			UpToLine:         upToLine,
			ComputedAt:       time.Now().UTC(),
			InputTokens:      1000,
			OutputTokens:     500,
			EstimatedCostUSD: decimal.NewFromFloat(1.50),
		}
	}

	t.Run("returns false when cached is nil", func(t *testing.T) {
		if IsCacheValid(nil, 1, 100) {
			t.Error("expected false for nil cached")
		}
	})

	t.Run("returns false when version mismatch", func(t *testing.T) {
		cached := makeCached(1, 100)
		currentVersion := 2
		currentLineCount := int64(100)

		if IsCacheValid(cached, currentVersion, currentLineCount) {
			t.Error("expected false for version mismatch")
		}
	})

	t.Run("returns false when line count mismatch (new data synced)", func(t *testing.T) {
		cached := makeCached(1, 100)
		currentVersion := 1
		currentLineCount := int64(150) // 50 new lines synced

		if IsCacheValid(cached, currentVersion, currentLineCount) {
			t.Error("expected false for line count mismatch")
		}
	})

	t.Run("returns false when both version and line count mismatch", func(t *testing.T) {
		cached := makeCached(1, 100)
		currentVersion := 2
		currentLineCount := int64(150)

		if IsCacheValid(cached, currentVersion, currentLineCount) {
			t.Error("expected false when both mismatch")
		}
	})

	t.Run("returns true when version and line count match", func(t *testing.T) {
		cached := makeCached(1, 100)
		currentVersion := 1
		currentLineCount := int64(100)

		if !IsCacheValid(cached, currentVersion, currentLineCount) {
			t.Error("expected true when both match")
		}
	})

	t.Run("returns true for zero line count when both match", func(t *testing.T) {
		cached := makeCached(1, 0)
		currentVersion := 1
		currentLineCount := int64(0)

		if !IsCacheValid(cached, currentVersion, currentLineCount) {
			t.Error("expected true for zero line count when matched")
		}
	})
}
