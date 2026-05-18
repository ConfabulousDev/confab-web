package analytics

import "testing"

// makeCommandExpansionMessage creates a user message with command-expansion skill content.
func makeCommandExpansionMessage(uuid, timestamp, skillName string) string {
	content := "<command-message>" + skillName + "</command-message>\n<command-name>/" + skillName + "</command-name>\nExpanded skill content here"
	return makeUserMessage(uuid, timestamp, content)
}

// makeSkillToolUseMessage creates an assistant message that invokes the Skill tool.
func makeSkillToolUseMessage(uuid, timestamp, toolUseID, skillName string) string {
	return makeAssistantMessage(uuid, timestamp, "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock(toolUseID, "Skill", map[string]interface{}{"skill": skillName}),
	})
}

// makeSkillToolResultMessage creates a user message with tool_result for a Skill invocation.
func makeSkillToolResultMessage(uuid, timestamp, toolUseID string, isError bool) string {
	return makeUserMessageWithToolResults(uuid, timestamp, []map[string]interface{}{
		makeToolResultBlock(toolUseID, "skill expanded", isError),
	})
}

func TestIsCommandExpansionMessage(t *testing.T) {
	tests := []struct {
		name     string
		jsonl    string
		wantBool bool
	}{
		{
			name:     "command expansion message",
			jsonl:    makeCommandExpansionMessage("u1", "2025-01-01T00:00:00Z", "interview"),
			wantBool: true,
		},
		{
			name:     "regular user message",
			jsonl:    makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello world"),
			wantBool: false,
		},
		{
			name:     "user message with tool results",
			jsonl:    makeUserMessageWithToolResults("u1", "2025-01-01T00:00:00Z", []map[string]interface{}{makeToolResultBlock("t1", "result", false)}),
			wantBool: false,
		},
		{
			name:     "assistant message",
			jsonl:    makeAssistantMessage("a1", "2025-01-01T00:00:00Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("hello")}),
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, err := ParseLine([]byte(tt.jsonl))
			if err != nil {
				t.Fatalf("ParseLine failed: %v", err)
			}
			if got := line.IsCommandExpansionMessage(); got != tt.wantBool {
				t.Errorf("IsCommandExpansionMessage() = %v, want %v", got, tt.wantBool)
			}
		})
	}
}

func TestGetCommandExpansionSkillName(t *testing.T) {
	tests := []struct {
		name     string
		jsonl    string
		wantName string
	}{
		{
			name:     "interview skill",
			jsonl:    makeCommandExpansionMessage("u1", "2025-01-01T00:00:00Z", "interview"),
			wantName: "interview",
		},
		{
			name:     "commit skill",
			jsonl:    makeCommandExpansionMessage("u1", "2025-01-01T00:00:00Z", "commit"),
			wantName: "commit",
		},
		{
			name:     "regular user message returns empty",
			jsonl:    makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello"),
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, err := ParseLine([]byte(tt.jsonl))
			if err != nil {
				t.Fatalf("ParseLine failed: %v", err)
			}
			if got := line.GetCommandExpansionSkillName(); got != tt.wantName {
				t.Errorf("GetCommandExpansionSkillName() = %q, want %q", got, tt.wantName)
			}
		})
	}
}

func TestSkillsAnalyzer_CommandExpansion(t *testing.T) {
	// Transcript with both Skill tool invocations and command-expansion invocations
	jsonl := // Skill tool invocation (pattern #1)
		makeSkillToolUseMessage("a1", "2025-01-01T00:00:00Z", "tu1", "commit") + "\n" +
		makeSkillToolResultMessage("u1", "2025-01-01T00:00:01Z", "tu1", false) + "\n" +
		// Command-expansion invocation (pattern #2)
		makeCommandExpansionMessage("u2", "2025-01-01T00:00:02Z", "interview") + "\n" +
		makeAssistantMessage("a2", "2025-01-01T00:00:03Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("Starting interview")}) + "\n" +
		// Another command-expansion
		makeCommandExpansionMessage("u3", "2025-01-01T00:00:04Z", "commit") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&SkillsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 3 {
		t.Errorf("TotalInvocations = %d, want 3", result.TotalInvocations)
	}

	// commit: 1 from Skill tool + 1 from command-expansion
	commitStats := result.SkillStats["commit"]
	if commitStats == nil {
		t.Fatal("expected commit stats to exist")
	}
	if commitStats.Success != 2 {
		t.Errorf("commit.Success = %d, want 2", commitStats.Success)
	}

	// interview: 1 from command-expansion
	interviewStats := result.SkillStats["interview"]
	if interviewStats == nil {
		t.Fatal("expected interview stats to exist")
	}
	if interviewStats.Success != 1 {
		t.Errorf("interview.Success = %d, want 1", interviewStats.Success)
	}
}

func TestSkillsAnalyzer_OnlyToolInvocations(t *testing.T) {
	// Only Skill tool pattern - no command-expansion
	jsonl := makeSkillToolUseMessage("a1", "2025-01-01T00:00:00Z", "tu1", "commit") + "\n" +
		makeSkillToolResultMessage("u1", "2025-01-01T00:00:01Z", "tu1", false) + "\n" +
		makeSkillToolUseMessage("a2", "2025-01-01T00:00:02Z", "tu2", "commit") + "\n" +
		makeSkillToolResultMessage("u2", "2025-01-01T00:00:03Z", "tu2", true) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&SkillsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 2 {
		t.Errorf("TotalInvocations = %d, want 2", result.TotalInvocations)
	}
	commitStats := result.SkillStats["commit"]
	if commitStats == nil {
		t.Fatal("expected commit stats to exist")
	}
	if commitStats.Success != 1 {
		t.Errorf("commit.Success = %d, want 1", commitStats.Success)
	}
	if commitStats.Errors != 1 {
		t.Errorf("commit.Errors = %d, want 1", commitStats.Errors)
	}
}

func TestSkillsAnalyzer_OnlyCommandExpansions(t *testing.T) {
	// Only command-expansion pattern - no Skill tool
	jsonl := makeCommandExpansionMessage("u1", "2025-01-01T00:00:00Z", "interview") + "\n" +
		makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("response")}) + "\n" +
		makeCommandExpansionMessage("u2", "2025-01-01T00:00:02Z", "bugfix") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&SkillsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 2 {
		t.Errorf("TotalInvocations = %d, want 2", result.TotalInvocations)
	}
	if result.SkillStats["interview"] == nil || result.SkillStats["interview"].Success != 1 {
		t.Errorf("interview.Success = %v, want 1", result.SkillStats["interview"])
	}
	if result.SkillStats["bugfix"] == nil || result.SkillStats["bugfix"].Success != 1 {
		t.Errorf("bugfix.Success = %v, want 1", result.SkillStats["bugfix"])
	}
}

func TestSkillsAnalyzer_NoSkills(t *testing.T) {
	// No skills at all
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("hi")}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&SkillsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 0 {
		t.Errorf("TotalInvocations = %d, want 0", result.TotalInvocations)
	}
	if len(result.SkillStats) != 0 {
		t.Errorf("SkillStats length = %d, want 0", len(result.SkillStats))
	}
}
