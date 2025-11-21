package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindAgentReferences(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		wantIDs  []string
		wantErr  bool
	}{
		{
			name:    "no agents in transcript",
			fixture: "testdata/transcript_no_agents.jsonl",
			wantIDs: nil,
			wantErr: false,
		},
		{
			name:    "one agent at root level",
			fixture: "testdata/transcript_one_agent_root.jsonl",
			wantIDs: []string{"96f3c489"},
			wantErr: false,
		},
		{
			name:    "one agent in nested format",
			fixture: "testdata/transcript_one_agent_nested.jsonl",
			wantIDs: []string{"abcd1234"},
			wantErr: false,
		},
		{
			name:    "multiple different agents",
			fixture: "testdata/transcript_multiple_agents.jsonl",
			wantIDs: []string{"11111111", "22222222", "33333333"},
			wantErr: false,
		},
		{
			name:    "duplicate agents are deduplicated",
			fixture: "testdata/transcript_duplicate_agents.jsonl",
			wantIDs: []string{"deadbeef"},
			wantErr: false,
		},
		{
			name:    "malformed lines are skipped",
			fixture: "testdata/transcript_malformed_lines.jsonl",
			wantIDs: []string{"aabb1122", "ccdd3344"},
			wantErr: false,
		},
		{
			name:    "mixed root and nested formats",
			fixture: "testdata/transcript_mixed_formats.jsonl",
			wantIDs: []string{"11112222", "33334444"},
			wantErr: false,
		},
		{
			name:    "invalid agent IDs are filtered",
			fixture: "testdata/transcript_invalid_agent_ids.jsonl",
			wantIDs: []string{"abcd5678", "ABCD1234"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIDs, err := findAgentReferences(tt.fixture)

			if (err != nil) != tt.wantErr {
				t.Errorf("findAgentReferences() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !sliceEqual(gotIDs, tt.wantIDs) {
				t.Errorf("findAgentReferences() = %v, want %v", gotIDs, tt.wantIDs)
			}
		})
	}
}

func TestFindAgentReferences_FileNotFound(t *testing.T) {
	_, err := findAgentReferences("testdata/nonexistent.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFindAgentReferences_EmptyFile(t *testing.T) {
	// Create temp empty file
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.jsonl")
	os.WriteFile(emptyFile, []byte{}, 0644)

	ids, err := findAgentReferences(emptyFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice, got %v", ids)
	}
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abcdef", true},
		{"ABCDEF", true},
		{"123456", true},
		{"abcd1234", true},
		{"ABCD1234", true},
		{"ghijkl", false},
		{"abcdefg", false},
		{"12345z", false},
		{"", true}, // empty string has no non-hex chars
		{"abc def", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isHexString(tt.input)
			if got != tt.want {
				t.Errorf("isHexString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tilde expansion",
			input: "~/foo/bar",
			want:  filepath.Join(home, "foo/bar"),
		},
		{
			name:  "no tilde",
			input: "/absolute/path",
			want:  "/absolute/path",
		},
		{
			name:  "relative path",
			input: "relative/path",
			want:  "relative/path",
		},
		{
			name:  "just tilde",
			input: "~",
			want:  home,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.input)
			if got != tt.want {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// sliceEqual compares two string slices for equality
func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
