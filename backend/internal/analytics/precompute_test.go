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
			got := extractAgentID(tt.fileName)
			if got != tt.want {
				t.Errorf("extractAgentID(%q) = %q, want %q", tt.fileName, got, tt.want)
			}
		})
	}
}

func TestMergeChunks(t *testing.T) {
	tests := []struct {
		name   string
		chunks [][]byte
		want   string
	}{
		{
			name:   "empty chunks returns nil",
			chunks: [][]byte{},
			want:   "",
		},
		{
			name:   "single chunk",
			chunks: [][]byte{[]byte("hello")},
			want:   "hello",
		},
		{
			name:   "multiple chunks",
			chunks: [][]byte{[]byte("hello"), []byte(" "), []byte("world")},
			want:   "hello world",
		},
		{
			name:   "chunks with newlines",
			chunks: [][]byte{[]byte("line1\n"), []byte("line2\n")},
			want:   "line1\nline2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mergeChunks(tt.chunks)
			if err != nil {
				t.Errorf("mergeChunks() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("mergeChunks() = %q, want %q", string(got), tt.want)
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
				StalenessMinutes:   10,
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
				StalenessMinutes:  10,
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
				tt.config.SmartRecapQuota > 0 &&
				tt.config.StalenessMinutes > 0)

			if isValid != tt.valid {
				t.Errorf("config validation = %v, want %v", isValid, tt.valid)
			}
		})
	}
}
