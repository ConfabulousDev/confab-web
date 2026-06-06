package api

import (
	"testing"
	"time"
)

func TestExtractTextFromMessage(t *testing.T) {
	tests := []struct {
		name  string
		entry map[string]interface{}
		want  string
	}{
		{
			name: "string content",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": "Hello world",
				},
			},
			want: "Hello world",
		},
		{
			name: "array content with text block",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": "Array text"},
					},
				},
			},
			want: "Array text",
		},
		{
			name: "array content with image then text",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "image", "source": map[string]interface{}{}},
						map[string]interface{}{"type": "text", "text": "After image"},
					},
				},
			},
			want: "After image",
		},
		{
			name: "array content with only image",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "image", "source": map[string]interface{}{}},
					},
				},
			},
			want: "",
		},
		{
			name: "no message field",
			entry: map[string]interface{}{
				"type": "user",
			},
			want: "",
		},
		{
			name: "nil content",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": nil,
				},
			},
			want: "",
		},
		{
			name: "empty string content",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": "",
				},
			},
			want: "",
		},
		{
			name: "empty array content",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextFromMessage(tt.entry)
			if got != tt.want {
				t.Errorf("extractTextFromMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractSessionTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "summary takes priority",
			content: `{"type":"user","message":{"role":"user","content":"First question"}}` + "\n" + `{"type":"summary","summary":"Session about questions"}`,
			want:    "Session about questions",
		},
		{
			name:    "falls back to first user text",
			content: `{"type":"user","message":{"role":"user","content":"What is Go?"}}` + "\n" + `{"type":"assistant","message":{"role":"assistant","content":"Go is a language"}}`,
			want:    "What is Go?",
		},
		{
			name:    "skips image-only message finds later text",
			content: `{"type":"user","message":{"role":"user","content":[{"type":"image","source":{}}]}}` + "\n" + `{"type":"user","message":{"role":"user","content":"Here is my question"}}`,
			want:    "Here is my question",
		},
		{
			name:    "multimodal message with text after image",
			content: `{"type":"user","message":{"role":"user","content":[{"type":"image","source":{}},{"type":"text","text":"Describe this image"}]}}`,
			want:    "Describe this image",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "only image messages",
			content: `{"type":"user","message":{"role":"user","content":[{"type":"image","source":{}}]}}`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSessionTitle([]byte(tt.content))
			if got != tt.want {
				t.Errorf("extractSessionTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExtractTimestampFromLine pins the helper's schema-agnostic behavior.
// Both Claude Code transcript lines and Codex rollout lines carry a top-level
// ISO-8601 "timestamp" field; the helper must parse either without provider
// knowledge. CF-355: regression guard so a future "tighten with a type check"
// refactor cannot silently break Codex last-message-at tracking.
func TestExtractTimestampFromLine(t *testing.T) {
	// want is RFC3339Nano; empty string means "expect nil".
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "claude code transcript line",
			line: `{"type":"assistant","timestamp":"2025-01-15T11:30:00.000Z","message":{"role":"assistant","content":"hi"}}`,
			want: "2025-01-15T11:30:00.000Z",
		},
		{
			name: "codex session_meta line",
			line: `{"timestamp":"2026-05-13T01:00:00.000Z","type":"session_meta","payload":{"id":"abc","cwd":"/p"}}`,
			want: "2026-05-13T01:00:00.000Z",
		},
		{
			name: "codex response_item line",
			line: `{"timestamp":"2026-05-13T01:00:00.300Z","type":"response_item","payload":{"type":"message","role":"developer","content":[]}}`,
			want: "2026-05-13T01:00:00.300Z",
		},
		{
			name: "missing timestamp field",
			line: `{"type":"summary","summary":"hello"}`,
		},
		{
			name: "empty timestamp value",
			line: `{"type":"user","timestamp":""}`,
		},
		{
			name: "malformed timestamp value",
			line: `{"type":"user","timestamp":"not-a-date"}`,
		},
		{
			name: "malformed json",
			line: `{"timestamp":"2026-05-13T01:00:00.000Z","type":`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTimestampFromLine(tt.line)
			if tt.want == "" {
				if got != nil {
					t.Errorf("extractTimestampFromLine() = %v, want nil", got)
				}
				return
			}
			want, err := time.Parse(time.RFC3339Nano, tt.want)
			if err != nil {
				t.Fatalf("test fixture: bad RFC3339Nano %q: %v", tt.want, err)
			}
			if got == nil {
				t.Fatalf("extractTimestampFromLine() = nil, want %v", want)
			}
			if !got.Equal(want) {
				t.Errorf("extractTimestampFromLine() = %v, want %v", got, want)
			}
		})
	}
}

func TestExtractOpenCodeTimestampFromLine(t *testing.T) {
	// wantMs is the expected epoch-millisecond value; 0 means "expect nil".
	tests := []struct {
		name   string
		line   string
		wantMs int64
	}{
		{
			name:   "opencode assistant message",
			line:   `{"info":{"id":"msg_01","role":"assistant","time":{"created":1717689600000,"completed":1717689605000}},"parts":[]}`,
			wantMs: 1717689600000,
		},
		{
			name:   "opencode user message (created only)",
			line:   `{"info":{"id":"msg_00","role":"user","time":{"created":1717689500000}},"parts":[]}`,
			wantMs: 1717689500000,
		},
		{
			name: "no created field",
			line: `{"info":{"id":"msg_01","role":"assistant","time":{}},"parts":[]}`,
		},
		{
			name: "zero created value",
			line: `{"info":{"time":{"created":0}}}`,
		},
		{
			name: "absurd future epoch rejected",
			line: `{"info":{"time":{"created":999999999999999999}}}`,
		},
		{
			name: "claude-shaped line has no info.time.created",
			line: `{"type":"assistant","timestamp":"2025-01-15T11:30:00.000Z"}`,
		},
		{
			name: "malformed json",
			line: `{"info":{"time":{"created":1717689600000`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOpenCodeTimestampFromLine(tt.line)
			if tt.wantMs == 0 {
				if got != nil {
					t.Errorf("extractOpenCodeTimestampFromLine() = %v, want nil", got)
				}
				return
			}
			want := time.UnixMilli(tt.wantMs).UTC()
			if got == nil {
				t.Fatalf("extractOpenCodeTimestampFromLine() = nil, want %v", want)
			}
			if !got.Equal(want) {
				t.Errorf("extractOpenCodeTimestampFromLine() = %v, want %v", got, want)
			}
		})
	}
}
