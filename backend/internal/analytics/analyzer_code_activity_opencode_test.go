package analytics

import (
	"testing"
)

func TestComputeOpenCodeCodeActivity_ReadTool(t *testing.T) {
	finish := "stop"
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_01", SessionID: "ses_01", Role: "assistant",
					ModelID: "claude-sonnet-4-20250514", ProviderID: "anthropic",
					Finish: &finish,
					Tokens: OpenCodeTokens{Input: 1000, Output: 500},
					Time:   OpenCodeTime{Created: 1717689600000},
				},
				Parts: []OpenCodePart{
					{ID: "prt_01", Type: "tool", CallID: "call_0", Tool: "Read",
						State: &OpenCodeToolState{Status: "completed", Input: map[string]interface{}{"file_path": "src/main.go"}}},
					{ID: "prt_02", Type: "tool", CallID: "call_1", Tool: "Read",
						State: &OpenCodeToolState{Status: "completed", Input: map[string]interface{}{"file_path": "src/utils.py"}}},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.FilesRead != 2 {
		t.Errorf("FilesRead = %d, want 2", out.FilesRead)
	}
	if out.LanguageBreakdown["go"] != 1 {
		t.Errorf("LanguageBreakdown[go] = %d, want 1", out.LanguageBreakdown["go"])
	}
	if out.LanguageBreakdown["python"] != 1 {
		t.Errorf("LanguageBreakdown[python] = %d, want 1", out.LanguageBreakdown["python"])
	}
}

func TestComputeOpenCodeCodeActivity_WriteTool(t *testing.T) {
	finish := "stop"
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_01", SessionID: "ses_01", Role: "assistant",
					ModelID: "claude-sonnet-4-20250514", ProviderID: "anthropic",
					Finish: &finish,
					Tokens: OpenCodeTokens{Input: 1000, Output: 500},
					Time:   OpenCodeTime{Created: 1717689600000},
				},
				Parts: []OpenCodePart{
					{ID: "prt_01", Type: "tool", CallID: "call_0", Tool: "Write",
						State: &OpenCodeToolState{Status: "completed",
							Input: map[string]interface{}{
								"file_path": "src/main.go",
								"content":   "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
							}}},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.FilesModified != 1 {
		t.Errorf("FilesModified = %d, want 1", out.FilesModified)
	}
	if out.LinesAdded != 5 {
		t.Errorf("LinesAdded = %d, want 5", out.LinesAdded)
	}
}

func TestComputeOpenCodeCodeActivity_EditTool(t *testing.T) {
	finish := "stop"
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_01", SessionID: "ses_01", Role: "assistant",
					ModelID: "claude-sonnet-4-20250514", ProviderID: "anthropic",
					Finish: &finish,
					Tokens: OpenCodeTokens{Input: 1000, Output: 500},
					Time:   OpenCodeTime{Created: 1717689600000},
				},
				Parts: []OpenCodePart{
					{ID: "prt_01", Type: "tool", CallID: "call_0", Tool: "Edit",
						State: &OpenCodeToolState{Status: "completed",
							Input: map[string]interface{}{
								"file_path":  "src/main.go",
								"old_string": "old line\nanother old",
								"new_string": "new line\nanother new\nextra line",
							}}},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.FilesModified != 1 {
		t.Errorf("FilesModified = %d, want 1", out.FilesModified)
	}
	if out.LinesAdded != 3 {
		t.Errorf("LinesAdded = %d, want 3", out.LinesAdded)
	}
	if out.LinesRemoved != 2 {
		t.Errorf("LinesRemoved = %d, want 2", out.LinesRemoved)
	}
}

func TestComputeOpenCodeCodeActivity_SearchTools(t *testing.T) {
	finish := "stop"
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_01", SessionID: "ses_01", Role: "assistant",
					ModelID: "claude-sonnet-4-20250514", ProviderID: "anthropic",
					Finish: &finish,
					Tokens: OpenCodeTokens{Input: 1000, Output: 500},
					Time:   OpenCodeTime{Created: 1717689600000},
				},
				Parts: []OpenCodePart{
					{ID: "prt_01", Type: "tool", CallID: "call_0", Tool: "Grep",
						State: &OpenCodeToolState{Status: "completed", Input: map[string]interface{}{"pattern": "TODO"}}},
					{ID: "prt_02", Type: "tool", CallID: "call_1", Tool: "Glob",
						State: &OpenCodeToolState{Status: "completed", Input: map[string]interface{}{"pattern": "**/*.go"}}},
					{ID: "prt_03", Type: "tool", CallID: "call_2", Tool: "Grep",
						State: &OpenCodeToolState{Status: "completed", Input: map[string]interface{}{"pattern": "FIXME"}}},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.SearchCount != 3 {
		t.Errorf("SearchCount = %d, want 3 (2 Grep + 1 Glob)", out.SearchCount)
	}
}

func TestComputeOpenCodeCodeActivity_PendingToolsExcluded(t *testing.T) {
	finish := "stop"
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_01", SessionID: "ses_01", Role: "assistant",
					ModelID: "claude-sonnet-4-20250514", ProviderID: "anthropic",
					Finish: &finish,
					Tokens: OpenCodeTokens{Input: 1000, Output: 500},
					Time:   OpenCodeTime{Created: 1717689600000},
				},
				Parts: []OpenCodePart{
					{ID: "prt_01", Type: "tool", CallID: "call_0", Tool: "Read",
						State: &OpenCodeToolState{Status: "pending", Input: map[string]interface{}{"file_path": "src/main.go"}}},
					{ID: "prt_02", Type: "tool", CallID: "call_1", Tool: "Read",
						State: &OpenCodeToolState{Status: "completed", Input: map[string]interface{}{"file_path": "src/utils.go"}}},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.FilesRead != 1 {
		t.Errorf("FilesRead = %d, want 1 (pending tool excluded)", out.FilesRead)
	}
}

func TestComputeOpenCodeConversation_TurnWindows(t *testing.T) {
	finish := "stop"
	r := &opencodeRollout{
		Messages: []*OpenCodeMessage{
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_01", SessionID: "ses_01", Role: "user",
					Time: OpenCodeTime{Created: 1717689500000},
				},
				Parts: []OpenCodePart{
					{ID: "prt_01", Type: "text", Text: "first prompt"},
				},
			},
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_02", SessionID: "ses_01", Role: "assistant",
					ModelID: "claude-sonnet-4-20250514", ProviderID: "anthropic",
					Finish: &finish,
					Tokens: OpenCodeTokens{Input: 1000, Output: 500},
					Time:   OpenCodeTime{Created: 1717689510000, Completed: ptrInt64(1717689515000)},
				},
				Parts: []OpenCodePart{
					{ID: "prt_02", Type: "text", Text: "reply 1"},
				},
			},
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_03", SessionID: "ses_01", Role: "user",
					Time: OpenCodeTime{Created: 1717689530000},
				},
				Parts: []OpenCodePart{
					{ID: "prt_03", Type: "text", Text: "second prompt"},
				},
			},
			{
				Info: OpenCodeMessageInfo{
					ID: "msg_04", SessionID: "ses_01", Role: "assistant",
					ModelID: "claude-sonnet-4-20250514", ProviderID: "anthropic",
					Finish: &finish,
					Tokens: OpenCodeTokens{Input: 2000, Output: 1000},
					Time:   OpenCodeTime{Created: 1717689540000, Completed: ptrInt64(1717689550000)},
				},
				Parts: []OpenCodePart{
					{ID: "prt_04", Type: "text", Text: "reply 2"},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2", out.UserTurns)
	}
	if out.AssistantTurns != 2 {
		t.Errorf("AssistantTurns = %d, want 2", out.AssistantTurns)
	}
}
