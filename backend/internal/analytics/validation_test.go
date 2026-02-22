package analytics

import (
	"testing"
)

func TestValidateLine_UserMessage(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantErrors int
		wantPaths  []string
	}{
		{
			name: "valid user message with string content",
			input: map[string]interface{}{
				"type":        "user",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"message": map[string]interface{}{
					"role":    "user",
					"content": "Hello, world!",
				},
			},
			wantErrors: 0,
		},
		{
			name: "valid user message with content blocks",
			input: map[string]interface{}{
				"type":        "user",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"message": map[string]interface{}{
					"role": "user",
					"content": []interface{}{
						map[string]interface{}{
							"type":        "tool_result",
							"tool_use_id": "tool-123",
							"content":     "result data",
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "missing uuid",
			input: map[string]interface{}{
				"type":        "user",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"message": map[string]interface{}{
					"role":    "user",
					"content": "Hello",
				},
			},
			wantErrors: 1,
			wantPaths:  []string{"uuid"},
		},
		{
			name: "missing message",
			input: map[string]interface{}{
				"type":        "user",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
			},
			wantErrors: 1,
			wantPaths:  []string{"message"},
		},
		{
			name: "wrong message role",
			input: map[string]interface{}{
				"type":        "user",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Hello",
				},
			},
			wantErrors: 1,
			wantPaths:  []string{"message.role"},
		},
		{
			name: "missing multiple base fields",
			input: map[string]interface{}{
				"type": "user",
				"message": map[string]interface{}{
					"role":    "user",
					"content": "Hello",
				},
			},
			wantErrors: 8, // uuid, timestamp, parentUuid, isSidechain, userType, cwd, sessionId, version
			wantPaths:  []string{"uuid", "timestamp", "parentUuid", "isSidechain", "userType", "cwd", "sessionId", "version"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateLine(tt.input)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateLine() got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
			if tt.wantPaths != nil {
				pathSet := make(map[string]bool)
				for _, e := range errors {
					pathSet[e.Path] = true
				}
				for _, p := range tt.wantPaths {
					if !pathSet[p] {
						t.Errorf("expected error at path %q, but not found", p)
					}
				}
			}
		})
	}
}

func TestValidateLine_AssistantMessage(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantErrors int
		wantPaths  []string
	}{
		{
			name: "valid assistant message",
			input: map[string]interface{}{
				"type":        "assistant",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"message": map[string]interface{}{
					"model":         "claude-3-5-sonnet",
					"id":            "msg-123",
					"type":          "message",
					"role":          "assistant",
					"stop_reason":   "end_turn",
					"stop_sequence": nil,
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Hello!",
						},
					},
					"usage": map[string]interface{}{
						"input_tokens":  float64(100),
						"output_tokens": float64(50),
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "missing model",
			input: map[string]interface{}{
				"type":        "assistant",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"message": map[string]interface{}{
					"id":            "msg-123",
					"type":          "message",
					"role":          "assistant",
					"stop_reason":   "end_turn",
					"stop_sequence": nil,
					"content":       []interface{}{},
					"usage": map[string]interface{}{
						"input_tokens":  float64(100),
						"output_tokens": float64(50),
					},
				},
			},
			wantErrors: 1,
			wantPaths:  []string{"message.model"},
		},
		{
			name: "missing usage",
			input: map[string]interface{}{
				"type":        "assistant",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"message": map[string]interface{}{
					"model":         "claude-3-5-sonnet",
					"id":            "msg-123",
					"type":          "message",
					"role":          "assistant",
					"stop_reason":   "end_turn",
					"stop_sequence": nil,
					"content":       []interface{}{},
				},
			},
			wantErrors: 1,
			wantPaths:  []string{"message.usage"},
		},
		{
			name: "invalid content block in array",
			input: map[string]interface{}{
				"type":        "assistant",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"message": map[string]interface{}{
					"model":         "claude-3-5-sonnet",
					"id":            "msg-123",
					"type":          "message",
					"role":          "assistant",
					"stop_reason":   "end_turn",
					"stop_sequence": nil,
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							// missing "text" field
						},
					},
					"usage": map[string]interface{}{
						"input_tokens":  float64(100),
						"output_tokens": float64(50),
					},
				},
			},
			wantErrors: 1,
			wantPaths:  []string{"message.content[0].text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateLine(tt.input)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateLine() got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
			if tt.wantPaths != nil {
				pathSet := make(map[string]bool)
				for _, e := range errors {
					pathSet[e.Path] = true
				}
				for _, p := range tt.wantPaths {
					if !pathSet[p] {
						t.Errorf("expected error at path %q, but not found", p)
					}
				}
			}
		})
	}
}

func TestValidateLine_SystemMessage(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantErrors int
		wantPaths  []string
	}{
		{
			name: "valid system message with content",
			input: map[string]interface{}{
				"type":        "system",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"subtype":     "info",
				"content":     "System message",
				"isMeta":      true,
				"level":       "info",
			},
			wantErrors: 0,
		},
		{
			name: "valid turn_duration system message (no content/level)",
			input: map[string]interface{}{
				"type":        "system",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"subtype":     "turn_duration",
				"isMeta":      true,
				"durationMs":  float64(1500),
				"slug":        "session-slug",
			},
			wantErrors: 0,
		},
		{
			name: "valid compact_boundary system message",
			input: map[string]interface{}{
				"type":        "system",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"subtype":     "compact_boundary",
				"isMeta":      true,
				"compactMetadata": map[string]interface{}{
					"trigger":   "auto",
					"preTokens": float64(50000),
				},
			},
			wantErrors: 0,
		},
		{
			name: "missing subtype",
			input: map[string]interface{}{
				"type":        "system",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"isMeta":      true,
			},
			wantErrors: 1,
			wantPaths:  []string{"subtype"},
		},
		{
			name: "invalid compactMetadata (missing trigger)",
			input: map[string]interface{}{
				"type":        "system",
				"uuid":        "test-uuid",
				"timestamp":   "2025-01-01T00:00:00Z",
				"parentUuid":  nil,
				"isSidechain": false,
				"userType":    "external",
				"cwd":         "/home/user",
				"sessionId":   "session-123",
				"version":     "1.0.0",
				"subtype":     "compact_boundary",
				"isMeta":      true,
				"compactMetadata": map[string]interface{}{
					"preTokens": float64(50000),
				},
			},
			wantErrors: 1,
			wantPaths:  []string{"compactMetadata.trigger"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateLine(tt.input)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateLine() got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
			if tt.wantPaths != nil {
				pathSet := make(map[string]bool)
				for _, e := range errors {
					pathSet[e.Path] = true
				}
				for _, p := range tt.wantPaths {
					if !pathSet[p] {
						t.Errorf("expected error at path %q, but not found", p)
					}
				}
			}
		})
	}
}

func TestValidateLine_FileHistorySnapshot(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid file history snapshot",
			input: map[string]interface{}{
				"type":             "file-history-snapshot",
				"messageId":        "msg-123",
				"isSnapshotUpdate": true,
				"snapshot": map[string]interface{}{
					"messageId":          "msg-123",
					"timestamp":          "2025-01-01T00:00:00Z",
					"trackedFileBackups": map[string]interface{}{},
				},
			},
			wantErrors: 0,
		},
		{
			name: "missing snapshot",
			input: map[string]interface{}{
				"type":             "file-history-snapshot",
				"messageId":        "msg-123",
				"isSnapshotUpdate": true,
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateLine(tt.input)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateLine() got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
		})
	}
}

func TestValidateLine_SummaryMessage(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid summary message",
			input: map[string]interface{}{
				"type":     "summary",
				"summary":  "This is a summary",
				"leafUuid": "leaf-123",
			},
			wantErrors: 0,
		},
		{
			name: "missing summary field",
			input: map[string]interface{}{
				"type":     "summary",
				"leafUuid": "leaf-123",
			},
			wantErrors: 1,
		},
		{
			name: "missing leafUuid",
			input: map[string]interface{}{
				"type":    "summary",
				"summary": "This is a summary",
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateLine(tt.input)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateLine() got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
		})
	}
}

func TestValidateLine_QueueOperationMessage(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid queue operation",
			input: map[string]interface{}{
				"type":      "queue-operation",
				"operation": "enqueue",
				"timestamp": "2025-01-01T00:00:00Z",
				"sessionId": "session-123",
			},
			wantErrors: 0,
		},
		{
			name: "missing operation",
			input: map[string]interface{}{
				"type":      "queue-operation",
				"timestamp": "2025-01-01T00:00:00Z",
				"sessionId": "session-123",
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateLine(tt.input)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateLine() got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
		})
	}
}

func TestValidateLine_PRLinkMessage(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantErrors int
		wantPaths  []string
	}{
		{
			name: "valid pr-link message",
			input: map[string]interface{}{
				"type":         "pr-link",
				"prNumber":     float64(22),
				"prRepository": "ConfabulousDev/confab-web",
				"prUrl":        "https://github.com/ConfabulousDev/confab-web/pull/22",
				"sessionId":    "session-123",
				"timestamp":    "2026-02-22T08:00:41.865Z",
			},
			wantErrors: 0,
		},
		{
			name: "missing prNumber",
			input: map[string]interface{}{
				"type":         "pr-link",
				"prRepository": "ConfabulousDev/confab-web",
				"prUrl":        "https://github.com/ConfabulousDev/confab-web/pull/22",
				"sessionId":    "session-123",
				"timestamp":    "2026-02-22T08:00:41.865Z",
			},
			wantErrors: 1,
			wantPaths:  []string{"prNumber"},
		},
		{
			name: "missing prRepository",
			input: map[string]interface{}{
				"type":      "pr-link",
				"prNumber":  float64(22),
				"prUrl":     "https://github.com/ConfabulousDev/confab-web/pull/22",
				"sessionId": "session-123",
				"timestamp": "2026-02-22T08:00:41.865Z",
			},
			wantErrors: 1,
			wantPaths:  []string{"prRepository"},
		},
		{
			name: "missing prUrl",
			input: map[string]interface{}{
				"type":         "pr-link",
				"prNumber":     float64(22),
				"prRepository": "ConfabulousDev/confab-web",
				"sessionId":    "session-123",
				"timestamp":    "2026-02-22T08:00:41.865Z",
			},
			wantErrors: 1,
			wantPaths:  []string{"prUrl"},
		},
		{
			name: "missing all required fields except type",
			input: map[string]interface{}{
				"type": "pr-link",
			},
			wantErrors: 5,
			wantPaths:  []string{"prNumber", "prRepository", "prUrl", "sessionId", "timestamp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateLine(tt.input)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateLine() got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
			if tt.wantPaths != nil {
				pathSet := make(map[string]bool)
				for _, e := range errors {
					pathSet[e.Path] = true
				}
				for _, p := range tt.wantPaths {
					if !pathSet[p] {
						t.Errorf("expected error at path %q, but not found", p)
					}
				}
			}
		})
	}
}

func TestValidateLine_UnknownType(t *testing.T) {
	// Unknown types should be allowed for forward compatibility
	input := map[string]interface{}{
		"type": "some-future-type",
		"data": "whatever",
	}
	errors := ValidateLine(input)
	if len(errors) != 0 {
		t.Errorf("unknown type should not cause errors for forward compatibility, got %d", len(errors))
	}
}

func TestValidateLine_MissingType(t *testing.T) {
	input := map[string]interface{}{
		"uuid": "test",
	}
	errors := ValidateLine(input)
	if len(errors) != 1 {
		t.Errorf("missing type should cause 1 error, got %d", len(errors))
	}
	if errors[0].Path != "type" {
		t.Errorf("error should be at path 'type', got %q", errors[0].Path)
	}
}

func TestValidateContentBlock_Text(t *testing.T) {
	tests := []struct {
		name       string
		block      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid text block",
			block: map[string]interface{}{
				"type": "text",
				"text": "Hello, world!",
			},
			wantErrors: 0,
		},
		{
			name: "missing text field",
			block: map[string]interface{}{
				"type": "text",
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateContentBlock(tt.block)
			if len(errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d", len(errors), tt.wantErrors)
			}
		})
	}
}

func TestValidateContentBlock_Thinking(t *testing.T) {
	tests := []struct {
		name       string
		block      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid thinking block",
			block: map[string]interface{}{
				"type":     "thinking",
				"thinking": "Let me think...",
			},
			wantErrors: 0,
		},
		{
			name: "valid thinking block with signature",
			block: map[string]interface{}{
				"type":      "thinking",
				"thinking":  "Let me think...",
				"signature": "abc123",
			},
			wantErrors: 0,
		},
		{
			name: "missing thinking field",
			block: map[string]interface{}{
				"type": "thinking",
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateContentBlock(tt.block)
			if len(errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d", len(errors), tt.wantErrors)
			}
		})
	}
}

func TestValidateContentBlock_ToolUse(t *testing.T) {
	tests := []struct {
		name       string
		block      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid tool_use block",
			block: map[string]interface{}{
				"type":  "tool_use",
				"id":    "tool-123",
				"name":  "Read",
				"input": map[string]interface{}{"file_path": "/tmp/test.txt"},
			},
			wantErrors: 0,
		},
		{
			name: "missing id",
			block: map[string]interface{}{
				"type":  "tool_use",
				"name":  "Read",
				"input": map[string]interface{}{},
			},
			wantErrors: 1,
		},
		{
			name: "missing name",
			block: map[string]interface{}{
				"type":  "tool_use",
				"id":    "tool-123",
				"input": map[string]interface{}{},
			},
			wantErrors: 1,
		},
		{
			name: "missing input",
			block: map[string]interface{}{
				"type": "tool_use",
				"id":   "tool-123",
				"name": "Read",
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateContentBlock(tt.block)
			if len(errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d", len(errors), tt.wantErrors)
			}
		})
	}
}

func TestValidateContentBlock_ToolResult(t *testing.T) {
	tests := []struct {
		name       string
		block      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid tool_result with string content",
			block: map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "tool-123",
				"content":     "result data",
			},
			wantErrors: 0,
		},
		{
			name: "valid tool_result with nested blocks",
			block: map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "tool-123",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "nested result",
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "valid tool_result with is_error",
			block: map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "tool-123",
				"content":     "error message",
				"is_error":    true,
			},
			wantErrors: 0,
		},
		{
			name: "missing tool_use_id",
			block: map[string]interface{}{
				"type":    "tool_result",
				"content": "result",
			},
			wantErrors: 1,
		},
		{
			name: "missing content",
			block: map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "tool-123",
			},
			wantErrors: 1,
		},
		{
			name: "invalid nested block",
			block: map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "tool-123",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						// missing "text" field
					},
				},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateContentBlock(tt.block)
			if len(errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
		})
	}
}

func TestValidateContentBlock_Image(t *testing.T) {
	tests := []struct {
		name       string
		block      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid image block with base64",
			block: map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "base64",
					"media_type": "image/png",
					"data":       "base64data...",
				},
			},
			wantErrors: 0,
		},
		{
			name: "valid image block with url",
			block: map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "url",
					"media_type": "image/jpeg",
					"url":        "https://example.com/image.jpg",
				},
			},
			wantErrors: 0,
		},
		{
			name: "missing source",
			block: map[string]interface{}{
				"type": "image",
			},
			wantErrors: 1,
		},
		{
			name: "missing source.type",
			block: map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"media_type": "image/png",
				},
			},
			wantErrors: 1,
		},
		{
			name: "missing source.media_type",
			block: map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type": "base64",
				},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateContentBlock(tt.block)
			if len(errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
		})
	}
}

func TestValidateContentBlock_UnknownType(t *testing.T) {
	// Unknown block types should be allowed for forward compatibility
	block := map[string]interface{}{
		"type": "some-future-block-type",
		"data": "whatever",
	}
	errors := validateContentBlock(block)
	if len(errors) != 0 {
		t.Errorf("unknown block type should not cause errors, got %d", len(errors))
	}
}

func TestValidateTokenUsage(t *testing.T) {
	tests := []struct {
		name       string
		usage      map[string]interface{}
		wantErrors int
	}{
		{
			name: "valid minimal usage",
			usage: map[string]interface{}{
				"input_tokens":  float64(100),
				"output_tokens": float64(50),
			},
			wantErrors: 0,
		},
		{
			name: "valid full usage",
			usage: map[string]interface{}{
				"input_tokens":                 float64(100),
				"output_tokens":                float64(50),
				"cache_creation_input_tokens":  float64(10),
				"cache_read_input_tokens":      float64(20),
				"service_tier":                 "default",
			},
			wantErrors: 0,
		},
		{
			name: "missing input_tokens",
			usage: map[string]interface{}{
				"output_tokens": float64(50),
			},
			wantErrors: 1,
		},
		{
			name: "missing output_tokens",
			usage: map[string]interface{}{
				"input_tokens": float64(100),
			},
			wantErrors: 1,
		},
		{
			name:       "missing both required fields",
			usage:      map[string]interface{}{},
			wantErrors: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateTokenUsage(tt.usage)
			if len(errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d", len(errors), tt.wantErrors)
				for _, e := range errors {
					t.Logf("  - %s: %s", e.Path, e.Message)
				}
			}
		})
	}
}

func TestTypeOf(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{nil, "undefined"},
		{"hello", "string"},
		{float64(42), "number"},
		{true, "boolean"},
		{[]interface{}{}, "array"},
		{map[string]interface{}{}, "object"},
	}

	for _, tt := range tests {
		got := typeOf(tt.input)
		if got != tt.want {
			t.Errorf("typeOf(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncateJSON(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
	}

	for _, tt := range tests {
		got := truncateJSON(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateJSON(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestFormatValidationErrors(t *testing.T) {
	errors := []LineValidationError{
		{
			Line:        1,
			MessageType: "user",
			Errors: []ValidationError{
				{Path: "uuid", Message: "required field missing", Expected: "string", Received: "undefined"},
			},
		},
		{
			Line:        5,
			MessageType: "assistant",
			Errors: []ValidationError{
				{Path: "message.usage", Message: "required field missing"},
			},
		},
	}

	output := FormatValidationErrors(errors, 10)
	if output == "" {
		t.Error("expected non-empty output")
	}
	if !contains(output, "Line 1") {
		t.Error("expected output to contain 'Line 1'")
	}
	if !contains(output, "Line 5") {
		t.Error("expected output to contain 'Line 5'")
	}
}

func TestFormatValidationErrors_Truncation(t *testing.T) {
	// Create more errors than maxErrors
	var errors []LineValidationError
	for i := 0; i < 15; i++ {
		errors = append(errors, LineValidationError{
			Line:        i + 1,
			MessageType: "user",
			Errors:      []ValidationError{{Path: "test", Message: "error"}},
		})
	}

	output := FormatValidationErrors(errors, 5)
	if !contains(output, "and 10 more errors") {
		t.Errorf("expected truncation message, got: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
