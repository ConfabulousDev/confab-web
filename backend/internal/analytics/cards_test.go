package analytics

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestTokensV2Data_CacheScalarsRoundTrip is the pjnz contract: TokensV2Data must
// serialize the new top-level cache-count scalars under the snake_case keys
// total_cache_creation / total_cache_read and read them back unchanged. These
// scalars reproduce the retired flat v1 card's cache columns for the Trends
// daily time-series reader.
func TestTokensV2Data_CacheScalarsRoundTrip(t *testing.T) {
	in := TokensV2Data{
		TotalCostUSD:       "1.25",
		TotalInput:         100,
		TotalOutput:        50,
		TotalCacheCreation: 40,
		TotalCacheRead:     60,
		ByProvider:         map[string]TokensV2Provider{},
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	js := string(b)
	if !strings.Contains(js, `"total_cache_creation":40`) {
		t.Errorf("marshaled JSON missing total_cache_creation key: %s", js)
	}
	if !strings.Contains(js, `"total_cache_read":60`) {
		t.Errorf("marshaled JSON missing total_cache_read key: %s", js)
	}

	var out TokensV2Data
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.TotalCacheCreation != 40 {
		t.Errorf("TotalCacheCreation = %d, want 40", out.TotalCacheCreation)
	}
	if out.TotalCacheRead != 60 {
		t.Errorf("TotalCacheRead = %d, want 60", out.TotalCacheRead)
	}
}

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

func TestRegularCardRecord_IsValid(t *testing.T) {
	const upTo = int64(100)

	tests := []struct {
		name    string
		card    CardValidator
		current int64
		want    bool
	}{
		{"tokens_v2 nil", (*TokensV2CardRecord)(nil), upTo, false},
		{"tokens_v2 valid", &TokensV2CardRecord{Version: TokensV2CardVersion, UpToLine: upTo}, upTo, true},
		{"tokens_v2 wrong version", &TokensV2CardRecord{Version: TokensV2CardVersion - 1, UpToLine: upTo}, upTo, false},
		{"tokens_v2 stale line count", &TokensV2CardRecord{Version: TokensV2CardVersion, UpToLine: upTo - 1}, upTo, false},
		{"tokens_v2 future line count", &TokensV2CardRecord{Version: TokensV2CardVersion, UpToLine: upTo + 1}, upTo, false},

		{"session nil", (*SessionCardRecord)(nil), upTo, false},
		{"session valid", &SessionCardRecord{Version: SessionCardVersion, UpToLine: upTo}, upTo, true},
		{"session wrong version", &SessionCardRecord{Version: SessionCardVersion - 1, UpToLine: upTo}, upTo, false},
		{"session stale line count", &SessionCardRecord{Version: SessionCardVersion, UpToLine: upTo - 1}, upTo, false},

		{"tools nil", (*ToolsCardRecord)(nil), upTo, false},
		{"tools valid", &ToolsCardRecord{Version: ToolsCardVersion, UpToLine: upTo}, upTo, true},
		{"tools wrong version", &ToolsCardRecord{Version: ToolsCardVersion - 1, UpToLine: upTo}, upTo, false},

		{"code activity nil", (*CodeActivityCardRecord)(nil), upTo, false},
		{"code activity valid", &CodeActivityCardRecord{Version: CodeActivityCardVersion, UpToLine: upTo}, upTo, true},
		{"code activity wrong version", &CodeActivityCardRecord{Version: CodeActivityCardVersion - 1, UpToLine: upTo}, upTo, false},

		{"conversation nil", (*ConversationCardRecord)(nil), upTo, false},
		{"conversation valid", &ConversationCardRecord{Version: ConversationCardVersion, UpToLine: upTo}, upTo, true},
		{"conversation wrong version", &ConversationCardRecord{Version: ConversationCardVersion - 1, UpToLine: upTo}, upTo, false},

		{"agents+skills nil", (*AgentsAndSkillsCardRecord)(nil), upTo, false},
		{"agents+skills valid", &AgentsAndSkillsCardRecord{Version: AgentsAndSkillsCardVersion, UpToLine: upTo}, upTo, true},
		{"agents+skills wrong version", &AgentsAndSkillsCardRecord{Version: AgentsAndSkillsCardVersion + 1, UpToLine: upTo}, upTo, false},

		{"redactions nil", (*RedactionsCardRecord)(nil), upTo, false},
		{"redactions valid", &RedactionsCardRecord{Version: RedactionsCardVersion, UpToLine: upTo}, upTo, true},
		{"redactions wrong version", &RedactionsCardRecord{Version: RedactionsCardVersion - 1, UpToLine: upTo}, upTo, false},

		{"workflows nil", (*WorkflowsCardRecord)(nil), upTo, false},
		{"workflows valid", &WorkflowsCardRecord{Version: WorkflowsCardVersion, UpToLine: upTo}, upTo, true},
		{"workflows wrong version", &WorkflowsCardRecord{Version: WorkflowsCardVersion + 1, UpToLine: upTo}, upTo, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.card.IsValid(tt.current)
			if got != tt.want {
				t.Errorf("IsValid(%d) = %v, want %v", tt.current, got, tt.want)
			}
		})
	}
}

func TestCards_AllValid(t *testing.T) {
	const upTo = int64(50)

	allFresh := &Cards{
		TokensV2:        &TokensV2CardRecord{Version: TokensV2CardVersion, UpToLine: upTo},
		Session:         &SessionCardRecord{Version: SessionCardVersion, UpToLine: upTo},
		Tools:           &ToolsCardRecord{Version: ToolsCardVersion, UpToLine: upTo},
		CodeActivity:    &CodeActivityCardRecord{Version: CodeActivityCardVersion, UpToLine: upTo},
		Conversation:    &ConversationCardRecord{Version: ConversationCardVersion, UpToLine: upTo},
		AgentsAndSkills: &AgentsAndSkillsCardRecord{Version: AgentsAndSkillsCardVersion, UpToLine: upTo},
		Redactions:      &RedactionsCardRecord{Version: RedactionsCardVersion, UpToLine: upTo},
		Workflows:       &WorkflowsCardRecord{Version: WorkflowsCardVersion, UpToLine: upTo},
	}

	if !allFresh.AllValid(upTo) {
		t.Fatalf("AllValid(%d) returned false for fully fresh card set", upTo)
	}

	if (*Cards)(nil).AllValid(upTo) {
		t.Errorf("nil Cards.AllValid should return false")
	}

	t.Run("any nil card invalidates the set", func(t *testing.T) {
		c := *allFresh
		c.Tools = nil
		if c.AllValid(upTo) {
			t.Errorf("AllValid should be false when Tools is nil")
		}
	})

	t.Run("any stale card invalidates the set", func(t *testing.T) {
		c := *allFresh
		c.Redactions = &RedactionsCardRecord{Version: RedactionsCardVersion, UpToLine: upTo - 1}
		if c.AllValid(upTo) {
			t.Errorf("AllValid should be false when one card is behind on UpToLine")
		}
	})

	t.Run("any wrong-version card invalidates the set", func(t *testing.T) {
		c := *allFresh
		c.Conversation = &ConversationCardRecord{Version: ConversationCardVersion - 1, UpToLine: upTo}
		if c.AllValid(upTo) {
			t.Errorf("AllValid should be false when one card has stale Version")
		}
	})
}
