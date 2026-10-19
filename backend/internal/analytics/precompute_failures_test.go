package analytics_test

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
)

func TestIsPrecomputeBucket_AcceptsOnlyTheFourWorkerBuckets(t *testing.T) {
	for _, b := range []string{
		analytics.BucketRegularCards,
		analytics.BucketSmartRecap,
		analytics.BucketSearchIndex,
		analytics.BucketSessionTitle,
	} {
		if !analytics.IsPrecomputeBucket(b) {
			t.Errorf("IsPrecomputeBucket(%q) = false, want true", b)
		}
	}
	for _, b := range []string{"", "REGULAR_CARDS", "regular_cards ", "tokens_v2", "'; DROP TABLE sessions; --"} {
		if analytics.IsPrecomputeBucket(b) {
			t.Errorf("IsPrecomputeBucket(%q) = true, want false", b)
		}
	}
}
