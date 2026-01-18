package analytics

import (
	"testing"
	"time"
)

func TestSmartRecapCardRecord_IsStale(t *testing.T) {
	tests := []struct {
		name             string
		card             *SmartRecapCardRecord
		currentLineCount int64
		stalenessMinutes int
		want             bool
	}{
		{
			name:             "nil card is stale",
			card:             nil,
			currentLineCount: 100,
			stalenessMinutes: 10,
			want:             true,
		},
		{
			name: "same line count is never stale",
			card: &SmartRecapCardRecord{
				UpToLine:   100,
				ComputedAt: time.Now().Add(-1 * time.Hour), // computed 1 hour ago
			},
			currentLineCount: 100,
			stalenessMinutes: 10,
			want:             false,
		},
		{
			name: "same line count is never stale even if computed long ago",
			card: &SmartRecapCardRecord{
				UpToLine:   100,
				ComputedAt: time.Now().Add(-24 * time.Hour), // computed 24 hours ago
			},
			currentLineCount: 100,
			stalenessMinutes: 10,
			want:             false,
		},
		{
			name: "new lines but not enough time passed is not stale",
			card: &SmartRecapCardRecord{
				UpToLine:   100,
				ComputedAt: time.Now().Add(-5 * time.Minute), // computed 5 minutes ago
			},
			currentLineCount: 150,
			stalenessMinutes: 10,
			want:             false,
		},
		{
			name: "new lines and enough time passed is stale",
			card: &SmartRecapCardRecord{
				UpToLine:   100,
				ComputedAt: time.Now().Add(-15 * time.Minute), // computed 15 minutes ago
			},
			currentLineCount: 150,
			stalenessMinutes: 10,
			want:             true,
		},
		{
			name: "new lines at exactly staleness threshold is stale",
			card: &SmartRecapCardRecord{
				UpToLine:   100,
				ComputedAt: time.Now().Add(-10 * time.Minute), // computed exactly 10 minutes ago
			},
			currentLineCount: 150,
			stalenessMinutes: 10,
			want:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.card.IsStale(tt.currentLineCount, tt.stalenessMinutes)
			if got != tt.want {
				t.Errorf("IsStale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSmartRecapCardRecord_CanAcquireLock(t *testing.T) {
	tests := []struct {
		name               string
		card               *SmartRecapCardRecord
		lockTimeoutSeconds int
		want               bool
	}{
		{
			name:               "nil card can acquire lock",
			card:               nil,
			lockTimeoutSeconds: 60,
			want:               true,
		},
		{
			name: "no lock held can acquire",
			card: &SmartRecapCardRecord{
				ComputingStartedAt: nil,
			},
			lockTimeoutSeconds: 60,
			want:               true,
		},
		{
			name: "fresh lock cannot acquire",
			card: &SmartRecapCardRecord{
				ComputingStartedAt: timePtr(time.Now().Add(-10 * time.Second)), // started 10 seconds ago
			},
			lockTimeoutSeconds: 60,
			want:               false,
		},
		{
			name: "stale lock can acquire",
			card: &SmartRecapCardRecord{
				ComputingStartedAt: timePtr(time.Now().Add(-120 * time.Second)), // started 2 minutes ago
			},
			lockTimeoutSeconds: 60,
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.card.CanAcquireLock(tt.lockTimeoutSeconds)
			if got != tt.want {
				t.Errorf("CanAcquireLock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
