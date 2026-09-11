package analytics

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// loadCodexRollout parses one of internal/codex's captured-shape fixtures.
func loadCodexRollout(t *testing.T, name string) *codex.ParsedRollout {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "codex", "testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	r, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout(%s): %v", name, err)
	}
	return r
}

func computeCodex(t *testing.T, r *codex.ParsedRollout) *ComputeResult {
	t.Helper()
	return ComputeFromCodexRollout(context.Background(), []*codex.ParsedRollout{r})
}

// The headline regression: a >=0.149.1 rollout with real file edits reported
// all zeros, because apply_patch no longer exists in that era.
func TestCodexCodeActivity_ModernRolloutReportsFileEdits(t *testing.T) {
	out := computeCodex(t, loadCodexRollout(t, "sample_rollout_modern.jsonl"))

	if out.FilesModified != 2 {
		t.Errorf("FilesModified = %d, want 2", out.FilesModified)
	}
	// app.ts: +2 / -1 from the unified diff; README.md: +3, the whole new file.
	if out.LinesAdded != 5 {
		t.Errorf("LinesAdded = %d, want 5", out.LinesAdded)
	}
	if out.LinesRemoved != 1 {
		t.Errorf("LinesRemoved = %d, want 1", out.LinesRemoved)
	}
	wantLangs := map[string]int{"typescript": 1, "markdown": 1}
	for lang, want := range wantLangs {
		if out.LanguageBreakdown[lang] != want {
			t.Errorf("LanguageBreakdown[%s] = %d, want %d", lang, out.LanguageBreakdown[lang], want)
		}
	}
	if len(out.LanguageBreakdown) != len(wantLangs) {
		t.Errorf("LanguageBreakdown = %v, want exactly %v", out.LanguageBreakdown, wantLangs)
	}
}

// M2: parsed_cmd states what the shell line did, so modern rollouts can report
// reads and searches that the old format never recorded.
func TestCodexCodeActivity_ModernRolloutReportsReadsAndSearches(t *testing.T) {
	out := computeCodex(t, loadCodexRollout(t, "sample_rollout_modern.jsonl"))

	if out.FilesRead != 2 {
		t.Errorf("FilesRead = %d, want 2", out.FilesRead)
	}
	if out.SearchCount != 1 {
		t.Errorf("SearchCount = %d, want 1", out.SearchCount)
	}
}

// "unknown" is the majority of real parsed_cmd entries; bucketing it anywhere
// would inflate the card. "list_files" is neither a read nor a search.
func TestCodexCodeActivity_UnclassifiedCommandsAreNotCounted(t *testing.T) {
	r := &codex.ParsedRollout{Turns: []codex.Turn{{
		ParsedCommandKinds: []string{"unknown", "list_files", "webfetch", ""},
	}}}
	out := computeCodex(t, r)

	if out.FilesRead != 0 {
		t.Errorf("FilesRead = %d, want 0", out.FilesRead)
	}
	if out.SearchCount != 0 {
		t.Errorf("SearchCount = %d, want 0", out.SearchCount)
	}
}

// The critical guard: it is entirely possible to fix modern by breaking old.
func TestCodexCodeActivity_OldEraNumbersAreUnchanged(t *testing.T) {
	out := computeCodex(t, loadCodexRollout(t, "sample_rollout.jsonl"))

	if out.FilesModified != 2 {
		t.Errorf("FilesModified = %d, want 2", out.FilesModified)
	}
	if out.LinesAdded != 4 {
		t.Errorf("LinesAdded = %d, want 4", out.LinesAdded)
	}
	if out.LinesRemoved != 1 {
		t.Errorf("LinesRemoved = %d, want 1", out.LinesRemoved)
	}
	if got, want := out.LanguageBreakdown["markdown"], 1; got != want {
		t.Errorf("LanguageBreakdown[markdown] = %d, want %d", got, want)
	}
	// Documented as genuinely unavailable: exec_command carries no
	// classification of what the shell line did.
	if out.FilesRead != 0 {
		t.Errorf("FilesRead = %d, want 0 for the old era", out.FilesRead)
	}
	if out.SearchCount != 0 {
		t.Errorf("SearchCount = %d, want 0 for the old era", out.SearchCount)
	}
}

// A rollout spanning a CLI upgrade carries both shapes; the counts must sum.
func TestCodexCodeActivity_MixedEraRolloutSumsBothShapes(t *testing.T) {
	ts := time.Date(2026, 9, 10, 21, 44, 0, 0, time.UTC)
	r := &codex.ParsedRollout{Turns: []codex.Turn{{
		ToolCalls: []codex.ToolCall{{
			Name:      "apply_patch",
			Arguments: "*** Begin Patch\n*** Update File: /w/old.go\n-was\n+now\n*** End Patch\n",
			Status:    "completed",
		}},
		FileEdits: []codex.FileEdit{{
			Path:        "/w/new.go",
			ChangeType:  "update",
			UnifiedDiff: "@@ -1,2 +1,2 @@\n-was\n+now\n",
			Timestamp:   ts,
		}},
	}}}
	out := computeCodex(t, r)

	if out.FilesModified != 2 {
		t.Errorf("FilesModified = %d, want 2", out.FilesModified)
	}
	if out.LinesAdded != 2 {
		t.Errorf("LinesAdded = %d, want 2", out.LinesAdded)
	}
	if out.LinesRemoved != 2 {
		t.Errorf("LinesRemoved = %d, want 2", out.LinesRemoved)
	}
	if got := out.LanguageBreakdown["go"]; got != 2 {
		t.Errorf("LanguageBreakdown[go] = %d, want 2", got)
	}
}

// A unified diff's ---/+++ file headers are not content lines.
func TestCodexCodeActivity_DiffHeadersAreNotCountedAsLines(t *testing.T) {
	r := &codex.ParsedRollout{Turns: []codex.Turn{{
		FileEdits: []codex.FileEdit{{
			Path:        "/w/a.py",
			ChangeType:  "update",
			UnifiedDiff: "--- a/a.py\n+++ b/a.py\n@@ -1,2 +1,2 @@\n-x = 1\n+x = 2\n",
		}},
	}}}
	out := computeCodex(t, r)

	if out.LinesAdded != 1 {
		t.Errorf("LinesAdded = %d, want 1", out.LinesAdded)
	}
	if out.LinesRemoved != 1 {
		t.Errorf("LinesRemoved = %d, want 1", out.LinesRemoved)
	}
}

// An `add` carries the whole new file rather than a diff, including the case
// where it has no trailing newline, and the empty-file case.
func TestCodexCodeActivity_AddedFileCountsEveryContentLine(t *testing.T) {
	r := &codex.ParsedRollout{Turns: []codex.Turn{{
		FileEdits: []codex.FileEdit{
			{Path: "/w/a.go", ChangeType: "add", Content: "package a\n\nvar X = 1\n"},
			{Path: "/w/b.go", ChangeType: "add", Content: "package b"},
			{Path: "/w/empty.go", ChangeType: "add", Content: ""},
		},
	}}}
	out := computeCodex(t, r)

	if out.FilesModified != 3 {
		t.Errorf("FilesModified = %d, want 3", out.FilesModified)
	}
	if out.LinesAdded != 4 {
		t.Errorf("LinesAdded = %d, want 4", out.LinesAdded)
	}
	if out.LinesRemoved != 0 {
		t.Errorf("LinesRemoved = %d, want 0", out.LinesRemoved)
	}
}

// One sampled 0.149.1 rollout has no items and no tool calls at all.
func TestCodexCodeActivity_TrivialModernSessionProducesZerosNotErrors(t *testing.T) {
	lines := strings.Join([]string{
		`{"timestamp":"2026-08-25T17:59:12.000Z","type":"session_meta","payload":{"id":"01a03a13","cwd":"/w","cli_version":"0.149.1","model_provider":"openai","model":"gpt-5.6"}}`,
		`{"timestamp":"2026-08-25T17:59:12.200Z","type":"event_msg","payload":{"type":"task_started","turn_id":"01a03a13-t1","started_at":1787682000,"model":"gpt-5.6"}}`,
		`{"timestamp":"2026-08-25T17:59:14.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"01a03a13-t1","completed_at":1787682002,"duration_ms":2000}}`,
	}, "\n") + "\n"

	r, err := codex.ParseRollout(bytes.NewReader([]byte(lines)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	out := computeCodex(t, r)

	if out.FilesModified != 0 || out.LinesAdded != 0 || out.LinesRemoved != 0 ||
		out.FilesRead != 0 || out.SearchCount != 0 || out.TotalToolCalls != 0 {
		t.Errorf("trivial session produced non-zero activity: %+v", out)
	}
	if len(out.LanguageBreakdown) != 0 {
		t.Errorf("LanguageBreakdown = %v, want empty", out.LanguageBreakdown)
	}
}

// M6: modern web searches must still land on the Tools card, under the raw
// modern name. The `exec` custom_tool_call must not be double-counted against
// the CommandExecution items describing the same shell runs.
func TestCodexTools_ModernRolloutCountsWebSearchWithoutDoubleCounting(t *testing.T) {
	out := computeCodex(t, loadCodexRollout(t, "sample_rollout_modern.jsonl"))

	if out.TotalToolCalls != 2 {
		t.Errorf("TotalToolCalls = %d, want 2 (exec + web.search)", out.TotalToolCalls)
	}
	if out.ToolStats["web.search"] == nil || out.ToolStats["web.search"].Success != 1 {
		t.Errorf("ToolStats[web.search] = %+v, want 1 success", out.ToolStats["web.search"])
	}
	if out.ToolStats["exec"] == nil || out.ToolStats["exec"].Success != 1 {
		t.Errorf("ToolStats[exec] = %+v, want 1 success", out.ToolStats["exec"])
	}
	if out.ToolStats["web_search_call"] != nil {
		t.Error("modern rollout reported the retired web_search_call name")
	}
}

// Old-era edited paths reach the search index through the apply_patch envelope.
// Modern rollouts carry them only on FileChange items, so they must be indexed
// from there or era parity is lost.
func TestExtractCodexUserMessagesText_IndexesModernEditedPaths(t *testing.T) {
	r := loadCodexRollout(t, "sample_rollout_modern.jsonl")
	text := ExtractCodexUserMessagesText([]*codex.ParsedRollout{r})

	for _, want := range []string{"README.md", "src/app.ts", "typescript readme conventions"} {
		if !strings.Contains(text, want) {
			t.Errorf("search text missing %q", want)
		}
	}
}

// Redaction markers inside an edited file reach the card in the old era via the
// apply_patch envelope; the modern equivalent lives on the FileChange item.
func TestCodexRedactions_CountsMarkersInModernFileEdits(t *testing.T) {
	r := &codex.ParsedRollout{Turns: []codex.Turn{{
		FileEdits: []codex.FileEdit{
			{Path: "/w/a.env", ChangeType: "update", UnifiedDiff: "@@ -1 +1 @@\n+KEY=[REDACTED:API_KEY]\n"},
			{Path: "/w/b.env", ChangeType: "add", Content: "TOKEN=[REDACTED:TOKEN]\n"},
		},
	}}}
	out := computeCodex(t, r)

	if out.TotalRedactions != 2 {
		t.Errorf("TotalRedactions = %d, want 2", out.TotalRedactions)
	}
	if out.RedactionCounts["API_KEY"] != 1 || out.RedactionCounts["TOKEN"] != 1 {
		t.Errorf("RedactionCounts = %v, want one API_KEY and one TOKEN", out.RedactionCounts)
	}
}

// The audit the ticket asks for: every Codex analyzer, checked against a
// captured rollout from each era. Analyzers that key on nothing era-specific
// legitimately need no change — this test is the evidence that they behave, not
// the assertion that they were edited.
func TestComputeFromCodexRollout_EveryCardAcrossBothEras(t *testing.T) {
	for _, era := range []struct {
		name    string
		fixture string
		// Per-card expectations. Only the fields whose correctness is
		// era-sensitive are pinned.
		toolNames  []string
		filesRead  int
		searchCnt  int
		filesMod   int
		userMsgs   int
		assistants int
		validErrs  int
		redactions int
	}{
		{
			name:    "old era (0.130.0)",
			fixture: "sample_rollout.jsonl",
			// exec_command / apply_patch / web_search_call vocabulary.
			toolNames: []string{"apply_patch", "exec_command", "web_search_call"},
			filesRead: 0, searchCnt: 0, filesMod: 2, userMsgs: 2, assistants: 3,
			// The fixture deliberately carries one orphan function_call_output
			// (CF-438) and one [REDACTED:…] marker.
			validErrs: 1, redactions: 1,
		},
		{
			name:    "modern era (0.154.0)",
			fixture: "sample_rollout_modern.jsonl",
			// exec / web.search vocabulary — reported raw, not remapped.
			toolNames: []string{"exec", "web.search"},
			filesRead: 2, searchCnt: 1, filesMod: 2, userMsgs: 1, assistants: 1,
			validErrs: 0, redactions: 0,
		},
	} {
		t.Run(era.name, func(t *testing.T) {
			r := loadCodexRollout(t, era.fixture)
			out := computeCodex(t, r)

			// Parse health: no era may gain unexplained parse anomalies.
			if out.ValidationErrorCount != era.validErrs {
				t.Errorf("ValidationErrorCount = %d, want %d", out.ValidationErrorCount, era.validErrs)
			}
			// Tokens (analyzer_tokens_codex.go) — unchanged by this work;
			// token_count survived the format change in both eras.
			if out.TokensV2 == nil || out.TokensV2.TotalInput <= 0 {
				t.Errorf("TokensV2 = %+v, want non-zero input", out.TokensV2)
			}
			// Session (analyzer_session_codex.go).
			if out.UserMessages != era.userMsgs {
				t.Errorf("UserMessages = %d, want %d", out.UserMessages, era.userMsgs)
			}
			if out.AssistantMessages != era.assistants {
				t.Errorf("AssistantMessages = %d, want %d", out.AssistantMessages, era.assistants)
			}
			if out.HumanPrompts != out.UserMessages {
				t.Errorf("HumanPrompts = %d, want == UserMessages %d", out.HumanPrompts, out.UserMessages)
			}
			if out.DurationMs == nil || *out.DurationMs <= 0 {
				t.Errorf("DurationMs = %v, want positive", out.DurationMs)
			}
			if len(out.ModelsUsed) == 0 {
				t.Error("ModelsUsed is empty")
			}
			// Conversation (analyzer_conversation_codex.go).
			if out.UserTurns <= 0 || out.AssistantTurns <= 0 {
				t.Errorf("UserTurns/AssistantTurns = %d/%d, want both positive", out.UserTurns, out.AssistantTurns)
			}
			// Tools (analyzer_tools_codex.go) — raw per-era vocabulary (M3).
			var names []string
			for name := range out.ToolStats {
				names = append(names, name)
			}
			sort.Strings(names)
			if strings.Join(names, ",") != strings.Join(era.toolNames, ",") {
				t.Errorf("tool names = %v, want %v", names, era.toolNames)
			}
			if out.TotalToolCalls <= 0 {
				t.Errorf("TotalToolCalls = %d, want positive", out.TotalToolCalls)
			}
			// Code activity (analyzer_code_activity_codex.go) — the regression.
			if out.FilesModified != era.filesMod {
				t.Errorf("FilesModified = %d, want %d", out.FilesModified, era.filesMod)
			}
			if out.LinesAdded <= 0 {
				t.Errorf("LinesAdded = %d, want positive", out.LinesAdded)
			}
			if out.FilesRead != era.filesRead {
				t.Errorf("FilesRead = %d, want %d", out.FilesRead, era.filesRead)
			}
			if out.SearchCount != era.searchCnt {
				t.Errorf("SearchCount = %d, want %d", out.SearchCount, era.searchCnt)
			}
			if len(out.LanguageBreakdown) == 0 {
				t.Error("LanguageBreakdown is empty")
			}
			// Agents and skills (analyzer_agents_and_skills_codex.go): both
			// fixtures are plain sessions, so empty is the correct answer. The
			// populated cases have their own fixtures and tests.
			if out.AgentStats == nil || out.SkillStats == nil {
				t.Error("AgentStats/SkillStats must be non-nil maps, not null")
			}
			// Redactions (analyzer_redactions_codex.go).
			if out.TotalRedactions != era.redactions {
				t.Errorf("TotalRedactions = %d, want %d", out.TotalRedactions, era.redactions)
			}
			// Search index (codex_search.go) and smart recap
			// (analyzer_smart_recap_codex.go) must both carry the prompt.
			searchText := ExtractCodexUserMessagesText([]*codex.ParsedRollout{r})
			if searchText == "" {
				t.Error("search text is empty")
			}
			recap, _ := PrepareCodexTranscript([]*codex.ParsedRollout{r})
			if !strings.Contains(recap, "<user id=") {
				t.Errorf("recap transcript carries no user message: %s", recap)
			}
			for _, name := range era.toolNames {
				if !strings.Contains(recap, `name="`+name+`"`) {
					t.Errorf("recap transcript missing tool %q", name)
				}
			}
		})
	}
}
