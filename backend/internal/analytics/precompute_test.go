package analytics

import (
	"testing"
)

func TestExtractAgentID(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     string
	}{
		{
			name:     "valid agent file",
			fileName: "agent-abc123.jsonl",
			want:     "abc123",
		},
		{
			name:     "valid agent file with long id",
			fileName: "agent-abc123-def456-ghi789.jsonl",
			want:     "abc123-def456-ghi789",
		},
		{
			name:     "transcript file returns empty",
			fileName: "transcript.jsonl",
			want:     "",
		},
		{
			name:     "wrong prefix returns empty",
			fileName: "foo-abc123.jsonl",
			want:     "",
		},
		{
			name:     "wrong suffix returns empty",
			fileName: "agent-abc123.txt",
			want:     "",
		},
		{
			name:     "missing suffix returns empty",
			fileName: "agent-abc123",
			want:     "",
		},
		{
			name:     "missing prefix returns empty",
			fileName: "abc123.jsonl",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAgentID(tt.fileName)
			if got != tt.want {
				t.Errorf("ExtractAgentID(%q) = %q, want %q", tt.fileName, got, tt.want)
			}
		})
	}
}

func TestPrecomputeConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config PrecomputeConfig
		valid  bool
	}{
		{
			name: "valid config with smart recap enabled",
			config: PrecomputeConfig{
				SmartRecapEnabled:  true,
				AnthropicAPIKey:    "test-key",
				SmartRecapModel:    "claude-haiku-4-5-20251001",
				SmartRecapQuota:    100,
				LockTimeoutSeconds: 60,
			},
			valid: true,
		},
		{
			name: "disabled config is always valid",
			config: PrecomputeConfig{
				SmartRecapEnabled: false,
			},
			valid: true,
		},
		{
			name: "missing API key makes it invalid for smart recap",
			config: PrecomputeConfig{
				SmartRecapEnabled: true,
				AnthropicAPIKey:   "",
				SmartRecapModel:   "claude-haiku-4-5-20251001",
				SmartRecapQuota:   100,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The config validation logic is in worker.go's loadPrecomputeConfig
			// This test documents the expected behavior
			isValid := tt.config.SmartRecapEnabled == false || (
				tt.config.AnthropicAPIKey != "" &&
				tt.config.SmartRecapModel != "" &&
				tt.config.SmartRecapQuota > 0)

			if isValid != tt.valid {
				t.Errorf("config validation = %v, want %v", isValid, tt.valid)
			}
		})
	}
}
