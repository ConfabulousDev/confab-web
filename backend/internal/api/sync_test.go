package api

import (
	"testing"
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
