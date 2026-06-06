package analytics

import (
	"strings"
	"testing"
)

func opencodeMinimalRollout() *opencodeRollout {
	finish := "stop"
	return &opencodeRollout{
		Messages: []*OpenCodeMessage{
			{
				Info: OpenCodeMessageInfo{
					ID:         "msg_01JX0000000000000000000001",
					SessionID:  "ses_01JX0000000000000000000001",
					Role:       "user",
					Agent:      "build",
					ModelID:    "claude-sonnet-4-20250514",
					ProviderID: "anthropic",
					Time:       OpenCodeTime{Created: 1717689500000},
				},
				Parts: []OpenCodePart{
					{ID: "prt_01JX0000000000000000000001", Type: "text", Text: "Find all Go files"},
				},
			},
			{
				Info: OpenCodeMessageInfo{
					ID:         "msg_01JX0000000000000000000002",
					SessionID:  "ses_01JX0000000000000000000001",
					Role:       "assistant",
					ParentID:   "msg_01JX0000000000000000000001",
					ModelID:    "claude-sonnet-4-20250514",
					ProviderID: "anthropic",
					Mode:       "build",
					Finish:     &finish,
					Cost:       0.015,
					Tokens: OpenCodeTokens{
						Input: 10000, Output: 5000, Reasoning: 2000,
						Cache: OpenCodeCache{Read: 3000, Write: 2000},
					},
					Time: OpenCodeTime{Created: 1717689600000, Completed: ptrInt64(1717689605000)},
				},
				Parts: []OpenCodePart{
					{ID: "prt_01JX0000000000000000000002", Type: "step-start", Snapshot: "abc123"},
					{ID: "prt_01JX0000000000000000000003", Type: "reasoning", Text: "Let me check the files..."},
					{ID: "prt_01JX0000000000000000000004", Type: "tool", CallID: "call_0", Tool: "Bash",
						State: &OpenCodeToolState{Status: "completed", Input: map[string]interface{}{"command": "ls"}, Output: "file1\nfile2"}},
					{ID: "prt_01JX0000000000000000000005", Type: "text", Text: "I found 2 files."},
					{ID: "prt_01JX0000000000000000000006", Type: "step-finish", Reason: "tool-calls", Cost: 0.015,
						Tokens: &OpenCodeTokens{Input: 10000, Output: 5000, Reasoning: 2000, Cache: OpenCodeCache{Read: 3000, Write: 2000}}},
				},
			},
		},
	}
}

func TestComputeFromOpenCodeRollout_HappyPath(t *testing.T) {
	r := opencodeMinimalRollout()
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.UserMessages != 1 {
		t.Errorf("UserMessages = %d, want 1", out.UserMessages)
	}
	if out.AssistantMessages != 1 {
		t.Errorf("AssistantMessages = %d, want 1", out.AssistantMessages)
	}
	if out.HumanPrompts != 1 {
		t.Errorf("HumanPrompts = %d, want 1", out.HumanPrompts)
	}
	if out.ToolCalls != 1 {
		t.Errorf("ToolCalls = %d, want 1", out.ToolCalls)
	}
	if out.ThinkingBlocks != 1 {
		t.Errorf("ThinkingBlocks = %d, want 1", out.ThinkingBlocks)
	}
	if out.TextResponses != 1 {
		t.Errorf("TextResponses = %d, want 1", out.TextResponses)
	}
	if out.DurationMs == nil || *out.DurationMs != 100000 {
		t.Errorf("DurationMs = %v, want 100000", out.DurationMs)
	}
	if len(out.ModelsUsed) != 1 || out.ModelsUsed[0] != "claude-sonnet-4-20250514" {
		t.Errorf("ModelsUsed = %v, want [claude-sonnet-4-20250514]", out.ModelsUsed)
	}
	if out.TotalToolCalls != 1 {
		t.Errorf("TotalToolCalls = %d, want 1", out.TotalToolCalls)
	}
	if out.ToolStats == nil || out.ToolStats["Bash"] == nil {
		t.Fatalf("ToolStats[Bash] missing: %v", out.ToolStats)
	}
	if out.ToolStats["Bash"].Success != 1 {
		t.Errorf("ToolStats[Bash].Success = %d, want 1", out.ToolStats["Bash"].Success)
	}
	if out.CompactionAuto != 0 || out.CompactionManual != 0 {
		t.Errorf("Compaction Auto/Manual = %d/%d, want 0/0", out.CompactionAuto, out.CompactionManual)
	}
}

func TestComputeFromOpenCodeRollout_EmptyRollout(t *testing.T) {
	out := ComputeFromOpenCodeRollout(&opencodeRollout{})
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.UserMessages != 0 || out.AssistantMessages != 0 {
		t.Errorf("expected all zero counts, got users=%d asst=%d", out.UserMessages, out.AssistantMessages)
	}
	if out.TotalToolCalls != 0 {
		t.Errorf("TotalToolCalls = %d, want 0", out.TotalToolCalls)
	}
}

func TestComputeFromOpenCodeRollout_FailedTool(t *testing.T) {
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
					{ID: "prt_01", Type: "tool", CallID: "call_0", Tool: "Bash",
						State: &OpenCodeToolState{Status: "completed", Input: map[string]interface{}{"command": "ls"}, Output: "ok"}},
					{ID: "prt_02", Type: "tool", CallID: "call_1", Tool: "Bash",
						State: &OpenCodeToolState{Status: "error", Input: map[string]interface{}{"command": "rm -rf /"}, Error: "permission denied"}},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.TotalToolCalls != 2 {
		t.Errorf("TotalToolCalls = %d, want 2", out.TotalToolCalls)
	}
	if out.ToolErrorCount != 1 {
		t.Errorf("ToolErrorCount = %d, want 1", out.ToolErrorCount)
	}
	stats := out.ToolStats["Bash"]
	if stats == nil {
		t.Fatalf("ToolStats[Bash] missing: %v", out.ToolStats)
	}
	if stats.Success != 1 {
		t.Errorf("ToolStats[Bash].Success = %d, want 1", stats.Success)
	}
	if stats.Errors != 1 {
		t.Errorf("ToolStats[Bash].Errors = %d, want 1", stats.Errors)
	}
}

func TestComputeFromOpenCodeRollout_Compaction(t *testing.T) {
	autoTrue := true
	autoFalse := false
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
					{ID: "prt_01", Type: "compaction", Auto: &autoTrue},
					{ID: "prt_02", Type: "compaction", Auto: &autoFalse},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.CompactionAuto != 1 {
		t.Errorf("CompactionAuto = %d, want 1", out.CompactionAuto)
	}
	if out.CompactionManual != 1 {
		t.Errorf("CompactionManual = %d, want 1", out.CompactionManual)
	}
}

func TestComputeFromOpenCodeRollout_Redactions(t *testing.T) {
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
					{ID: "prt_01", Type: "text", Text: "user email is [REDACTED:EMAIL]"},
					{ID: "prt_02", Type: "tool", CallID: "call_0", Tool: "Bash",
						State: &OpenCodeToolState{Status: "completed", Output: "found token [REDACTED:API_KEY] in env"}},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.TotalRedactions != 2 {
		t.Errorf("TotalRedactions = %d, want 2", out.TotalRedactions)
	}
	if out.RedactionCounts["EMAIL"] != 1 {
		t.Errorf("RedactionCounts[EMAIL] = %d, want 1", out.RedactionCounts["EMAIL"])
	}
	if out.RedactionCounts["API_KEY"] != 1 {
		t.Errorf("RedactionCounts[API_KEY] = %d, want 1", out.RedactionCounts["API_KEY"])
	}
}

func TestComputeFromOpenCodeRollout_SubtaskAgents(t *testing.T) {
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
					{ID: "prt_01", Type: "subtask", Name: "explore", Prompt: "Search for X"},
					{ID: "prt_02", Type: "subtask", Name: "build", Prompt: "Implement Y"},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.TotalAgentInvocations != 2 {
		t.Errorf("TotalAgentInvocations = %d, want 2", out.TotalAgentInvocations)
	}
	if out.AgentStats["explore"] == nil {
		t.Errorf("AgentStats[explore] missing: %v", out.AgentStats)
	}
	if out.AgentStats["build"] == nil {
		t.Errorf("AgentStats[build] missing: %v", out.AgentStats)
	}
	if out.TotalSkillInvocations != 0 {
		t.Errorf("TotalSkillInvocations = %d, want 0 (OpenCode has no skills)", out.TotalSkillInvocations)
	}
}

func TestPrepareOpenCodeTranscript(t *testing.T) {
	r := opencodeMinimalRollout()
	transcript, idMap := PrepareOpenCodeTranscript(r)

	for _, want := range []string{
		"<transcript>",
		"</transcript>",
		"<user id=\"1\">Find all Go files</user>",
		"<assistant id=\"2\">",
		"<thinking>Let me check the files...</thinking>",
		"I found 2 files.",
		"<tool id=\"3\" name=\"Bash\">ls</tool>",
		"<tool_result id=\"4\" tool_id=\"3\" status=\"completed\">file1\nfile2</tool_result>",
	} {
		if !strings.Contains(transcript, want) {
			t.Errorf("transcript missing %q\n---\n%s", want, transcript)
		}
	}

	// IDs are kept (OpenCode ULIDs are stable anchors) and map back to the
	// containing message id.
	if idMap[1] != "msg_01JX0000000000000000000001" {
		t.Errorf("idMap[1] = %q, want user message ULID", idMap[1])
	}
	if idMap[2] != "msg_01JX0000000000000000000002" {
		t.Errorf("idMap[2] = %q, want assistant message ULID", idMap[2])
	}
}

func TestComputeFromOpenCodeRollout_AgentModeSwitchesNotCounted(t *testing.T) {
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
					{ID: "prt_01", Type: "agent", Name: "plan"},
					{ID: "prt_02", Type: "agent", Name: "build"},
				},
			},
		},
	}
	out := ComputeFromOpenCodeRollout(r)
	if out == nil {
		t.Fatal("ComputeFromOpenCodeRollout returned nil")
	}
	if out.TotalAgentInvocations != 0 {
		t.Errorf("TotalAgentInvocations = %d, want 0 (agent mode switches are not invocations)", out.TotalAgentInvocations)
	}
}
