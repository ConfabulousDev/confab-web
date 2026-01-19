package analytics

import (
	"testing"
	"time"
)

func TestSmartRecapCardRecord_HasValidVersion(t *testing.T) {
	tests := []struct {
		name string
		card *SmartRecapCardRecord
		want bool
	}{
		{
			name: "nil card has no valid version",
			card: nil,
			want: false,
		},
		{
			name: "card with correct version",
			card: &SmartRecapCardRecord{
				Version: SmartRecapCardVersion,
			},
			want: true,
		},
		{
			name: "card with wrong version",
			card: &SmartRecapCardRecord{
				Version: SmartRecapCardVersion - 1,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.card.HasValidVersion()
			if got != tt.want {
				t.Errorf("HasValidVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSmartRecapCardRecord_IsUpToDate(t *testing.T) {
	tests := []struct {
		name             string
		card             *SmartRecapCardRecord
		currentLineCount int64
		want             bool
	}{
		{
			name:             "nil card is not up-to-date",
			card:             nil,
			currentLineCount: 100,
			want:             false,
		},
		{
			name: "card with matching line count is up-to-date",
			card: &SmartRecapCardRecord{
				Version:  SmartRecapCardVersion,
				UpToLine: 100,
			},
			currentLineCount: 100,
			want:             true,
		},
		{
			name: "card with higher line count is up-to-date",
			card: &SmartRecapCardRecord{
				Version:  SmartRecapCardVersion,
				UpToLine: 150,
			},
			currentLineCount: 100,
			want:             true,
		},
		{
			name: "card with lower line count is not up-to-date (has new lines)",
			card: &SmartRecapCardRecord{
				Version:  SmartRecapCardVersion,
				UpToLine: 50,
			},
			currentLineCount: 100,
			want:             false,
		},
		{
			name: "wrong version is not up-to-date",
			card: &SmartRecapCardRecord{
				Version:  SmartRecapCardVersion - 1,
				UpToLine: 100,
			},
			currentLineCount: 100,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.card.IsUpToDate(tt.currentLineCount)
			if got != tt.want {
				t.Errorf("IsUpToDate() = %v, want %v", got, tt.want)
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
