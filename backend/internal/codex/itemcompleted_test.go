package codex

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Fixture lines reproduce wire shapes captured verbatim from real local Codex
// rollouts (26 × 0.130.0, 6 × 0.149.1–0.154.0). Field names, nesting and the
// sibling fields the ticket's sketches omit (`move_path`, `text_elements`,
// `source`, `aggregated_output`, `formatted_output`) are as observed; only
// prompt text, ids and paths are substituted so no personal transcript
// content is committed.
const (
	// >=0.149.1: a FileChange item carries a per-path map. `update` entries
	// carry a unified_diff, `add` entries carry the full new `content`.
	modernFileChangeLine = `{"timestamp":"2026-09-10T21:44:25.770Z","ordinal":16,"type":"event_msg","payload":{"type":"item_completed","item":{"type":"FileChange","id":"exec-1aea9924","changes":{"/w/src/app.ts":{"type":"update","unified_diff":"@@ -1,2 +1,2 @@\n-const a = 1;\n+const a = 2;\n","move_path":null},"/w/README.md":{"type":"add","content":"# Title\nBody\n"}},"status":"completed","stdout":"","stderr":""}}}`

	// >=0.149.1: CommandExecution, with Codex's own parsed_cmd classification.
	modernCommandExecutionLine = `{"timestamp":"2026-09-10T21:44:15.366Z","ordinal":12,"type":"event_msg","payload":{"type":"item_completed","item":{"type":"CommandExecution","id":"exec-58f0815f","process_id":"53645","command":["/bin/zsh","-lc","sed -n '1,200p' src/app.ts; rg -n 'TODO' src; pwd"],"cwd":"file:///w","parsed_cmd":[{"type":"read","cmd":"sed -n '1,200p' src/app.ts","name":"app.ts","path":"src/app.ts"},{"type":"search","cmd":"rg -n 'TODO' src","path":"src","query":"TODO"},{"type":"unknown","cmd":"pwd"}],"source":"unified_exec_startup","status":"completed","stdout":"","stderr":"","aggregated_output":"","exit_code":0,"duration":{"secs":0,"nanos":8000000},"formatted_output":""}}}`

	// >=0.149.1: web search moved into an Extension item keyed by `kind`.
	modernWebSearchExtensionLine = `{"timestamp":"2026-09-10T21:44:20.659Z","ordinal":15,"type":"event_msg","payload":{"type":"item_completed","item":{"type":"Extension","kind":"web.search","id":"exec-ea70f327","query":"typescript readme conventions","action":{"type":"search","url":"https://example.com/search"},"results":[{"type":"text_result","domain":"example.com","ref_id":"turn0search0","title":"Conventions","url":"https://example.com/conventions"}]}}}`

	// An Extension kind we have never observed: must be ignored, not counted.
	modernUnknownExtensionLine = `{"timestamp":"2026-09-10T21:44:21.000Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"Extension","kind":"canvas.render","id":"exec-cccc","query":"x"}}}`

	// An item type no analyzer consumes: must be skipped for forward-compat.
	modernImageViewLine = `{"timestamp":"2026-09-10T21:44:22.000Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"ImageView","id":"exec-dddd","path":"/w/shot.png"}}}`

	// <=0.130.0 pair: the apply_patch custom_tool_call plus the patch_apply_end
	// event that reports its terminal status.
	oldApplyPatchCallLine      = `{"timestamp":"2026-05-13T01:00:10.000Z","type":"response_item","payload":{"type":"custom_tool_call","call_id":"call_old1","name":"apply_patch","input":"*** Begin Patch\n*** Update File: /w/notes.md\n-old line\n+new line\n*** End Patch\n","status":"completed"}}`
	oldPatchApplyEndFailedLine = `{"timestamp":"2026-05-13T01:00:11.000Z","type":"event_msg","payload":{"type":"patch_apply_end","call_id":"call_old1","success":false}}`

	modernTaskStartedLine  = `{"timestamp":"2026-09-10T21:44:00.200Z","type":"event_msg","payload":{"type":"task_started","turn_id":"01a08d47","started_at":1789076640,"model":"gpt-5.6"}}`
	modernTaskCompleteLine = `{"timestamp":"2026-09-10T21:44:28.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"01a08d47","completed_at":1789076668,"duration_ms":27800}}`
)

func parseLines(t *testing.T, lines ...string) *ParsedRollout {
	t.Helper()
	r, err := ParseRollout(bytes.NewReader([]byte(strings.Join(lines, "\n") + "\n")))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if len(r.ValidationErrors) != 0 {
		t.Fatalf("unexpected validation errors: %+v", r.ValidationErrors)
	}
	return r
}

func onlyTurn(t *testing.T, r *ParsedRollout) Turn {
	t.Helper()
	if len(r.Turns) != 1 {
		t.Fatalf("Turns = %d, want 1", len(r.Turns))
	}
	return r.Turns[0]
}

func loadModernFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "sample_rollout_modern.jsonl"))
	if err != nil {
		t.Fatalf("read modern fixture: %v", err)
	}
	return data
}

func TestItemCompleted_FileChangeSurfacesPerFileEdits(t *testing.T) {
	turn := onlyTurn(t, parseLines(t, modernTaskStartedLine, modernFileChangeLine, modernTaskCompleteLine))

	if len(turn.FileEdits) != 2 {
		t.Fatalf("FileEdits = %d, want 2", len(turn.FileEdits))
	}
	// changes is a JSON map: the parser must impose a deterministic order so
	// language-breakdown accumulation never depends on map iteration.
	if turn.FileEdits[0].Path != "/w/README.md" || turn.FileEdits[1].Path != "/w/src/app.ts" {
		t.Fatalf("FileEdits not sorted by path: %q, %q", turn.FileEdits[0].Path, turn.FileEdits[1].Path)
	}
	add := turn.FileEdits[0]
	if add.ChangeType != "add" {
		t.Errorf("add.ChangeType = %q, want %q", add.ChangeType, "add")
	}
	if add.Content != "# Title\nBody\n" {
		t.Errorf("add.Content = %q", add.Content)
	}
	if add.UnifiedDiff != "" {
		t.Errorf("add.UnifiedDiff = %q, want empty", add.UnifiedDiff)
	}
	upd := turn.FileEdits[1]
	if upd.ChangeType != "update" {
		t.Errorf("update.ChangeType = %q, want %q", upd.ChangeType, "update")
	}
	if !strings.HasPrefix(upd.UnifiedDiff, "@@ -1,2 +1,2 @@") {
		t.Errorf("update.UnifiedDiff = %q", upd.UnifiedDiff)
	}
	if upd.Content != "" {
		t.Errorf("update.Content = %q, want empty", upd.Content)
	}
}

func TestItemCompleted_FileChangeIsNotAToolCall(t *testing.T) {
	// Modern rollouts already surface `exec` via response_item.custom_tool_call;
	// the two streams share no ids, so items must never be counted as tools.
	turn := onlyTurn(t, parseLines(t, modernTaskStartedLine, modernFileChangeLine, modernCommandExecutionLine, modernTaskCompleteLine))
	if len(turn.ToolCalls) != 0 {
		t.Errorf("ToolCalls = %d, want 0", len(turn.ToolCalls))
	}
}

func TestItemCompleted_CommandExecutionSurfacesParsedCommandKinds(t *testing.T) {
	turn := onlyTurn(t, parseLines(t, modernTaskStartedLine, modernCommandExecutionLine, modernTaskCompleteLine))
	want := []string{"read", "search", "unknown"}
	if got := turn.ParsedCommandKinds; !slices.Equal(got, want) {
		t.Errorf("ParsedCommandKinds = %v, want %v", got, want)
	}
}

func TestItemCompleted_WebSearchExtensionBecomesToolCall(t *testing.T) {
	turn := onlyTurn(t, parseLines(t, modernTaskStartedLine, modernWebSearchExtensionLine, modernTaskCompleteLine))
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(turn.ToolCalls))
	}
	tc := turn.ToolCalls[0]
	// Raw modern name: the tool vocabulary genuinely changed, so we report what
	// the transcript says rather than remapping onto the retired name.
	if tc.Name != "web.search" {
		t.Errorf("Name = %q, want %q", tc.Name, "web.search")
	}
	if tc.Status != "completed" {
		t.Errorf("Status = %q, want %q", tc.Status, "completed")
	}
	if !strings.Contains(tc.Arguments, "typescript readme conventions") {
		t.Errorf("Arguments = %q, want it to carry the query", tc.Arguments)
	}
}

func TestItemCompleted_UnobservedExtensionKindIsIgnored(t *testing.T) {
	turn := onlyTurn(t, parseLines(t, modernTaskStartedLine, modernUnknownExtensionLine, modernTaskCompleteLine))
	if len(turn.ToolCalls) != 0 {
		t.Errorf("ToolCalls = %d, want 0", len(turn.ToolCalls))
	}
}

func TestItemCompleted_UnhandledItemTypesAreSkippedSilently(t *testing.T) {
	r := parseLines(t, modernTaskStartedLine, modernImageViewLine, modernTaskCompleteLine)
	turn := onlyTurn(t, r)
	if len(turn.ToolCalls) != 0 || len(turn.FileEdits) != 0 || len(turn.ParsedCommandKinds) != 0 {
		t.Errorf("ImageView produced output: %+v", turn)
	}
}

func TestItemCompleted_UserMessageIsNotDuplicatedFromTheEventStream(t *testing.T) {
	// Modern rollouts carry the human prompt in BOTH streams. The parser reads
	// response_item; recording the event copy too would double every count.
	r, err := ParseRollout(bytes.NewReader(loadModernFixture(t)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	var users, assistants int
	for _, turn := range r.Turns {
		users += len(turn.UserMessages)
		assistants += len(turn.AssistantMessages)
	}
	if users != 1 {
		t.Errorf("UserMessages = %d, want 1", users)
	}
	if assistants != 1 {
		t.Errorf("AssistantMessages = %d, want 1", assistants)
	}
}

func TestItemCompleted_MixedEraRolloutHandlesBothShapes(t *testing.T) {
	// A session started before a CLI upgrade and resumed after it carries both
	// shapes in one file; handling is per-event, so both must land.
	r := parseLines(t,
		modernTaskStartedLine,
		oldApplyPatchCallLine,
		oldPatchApplyEndFailedLine,
		modernFileChangeLine,
		modernTaskCompleteLine,
	)
	turn := onlyTurn(t, r)
	if len(turn.FileEdits) != 2 {
		t.Errorf("FileEdits = %d, want 2", len(turn.FileEdits))
	}
	if len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != "apply_patch" {
		t.Fatalf("ToolCalls = %+v, want one apply_patch", turn.ToolCalls)
	}
	// patch_apply_end must keep resolving old-era terminal status (CF-438).
	if turn.ToolCalls[0].Status != "failed" {
		t.Errorf("apply_patch Status = %q, want %q", turn.ToolCalls[0].Status, "failed")
	}
}

func TestItemCompleted_OldEraRolloutGainsNoItemState(t *testing.T) {
	r, err := ParseRollout(bytes.NewReader(loadFixture(t)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	for i, turn := range r.Turns {
		if len(turn.FileEdits) != 0 {
			t.Errorf("turn %d: FileEdits = %d, want 0", i, len(turn.FileEdits))
		}
		if len(turn.ParsedCommandKinds) != 0 {
			t.Errorf("turn %d: ParsedCommandKinds = %v, want none", i, turn.ParsedCommandKinds)
		}
	}
}

func TestItemCompleted_MalformedItemDoesNotPanicOrEmit(t *testing.T) {
	for name, line := range map[string]string{
		"no item":        `{"timestamp":"2026-09-10T21:44:22.000Z","type":"event_msg","payload":{"type":"item_completed"}}`,
		"null item":      `{"timestamp":"2026-09-10T21:44:22.000Z","type":"event_msg","payload":{"type":"item_completed","item":null}}`,
		"item not obj":   `{"timestamp":"2026-09-10T21:44:22.000Z","type":"event_msg","payload":{"type":"item_completed","item":"nope"}}`,
		"empty changes":  `{"timestamp":"2026-09-10T21:44:22.000Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"FileChange","changes":{}}}}`,
		"no parsed_cmd":  `{"timestamp":"2026-08-25T18:00:59.446Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"CommandExecution","id":"exec-9496","command":["/bin/zsh","-lc","pwd"],"status":"completed","stdout":"/w"}}}`,
		"blank cmd kind": `{"timestamp":"2026-09-10T21:44:22.000Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"CommandExecution","id":"exec-9497","parsed_cmd":[{"cmd":"pwd"}]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			r, err := ParseRollout(bytes.NewReader([]byte(modernTaskStartedLine + "\n" + line + "\n" + modernTaskCompleteLine + "\n")))
			if err != nil {
				t.Fatalf("ParseRollout: %v", err)
			}
			for _, turn := range r.Turns {
				if len(turn.FileEdits) != 0 || len(turn.ToolCalls) != 0 || len(turn.ParsedCommandKinds) != 0 {
					t.Errorf("malformed item produced output: %+v", turn)
				}
			}
		})
	}
}

func TestParseRollout_ModernFixtureShape(t *testing.T) {
	r, err := ParseRollout(bytes.NewReader(loadModernFixture(t)))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	if r.CLIVersion != "0.154.0" {
		t.Errorf("CLIVersion = %q, want %q", r.CLIVersion, "0.154.0")
	}
	if len(r.ValidationErrors) != 0 {
		t.Fatalf("ValidationErrors = %+v, want none", r.ValidationErrors)
	}
	turn := onlyTurn(t, r)
	if len(turn.FileEdits) != 2 {
		t.Errorf("FileEdits = %d, want 2", len(turn.FileEdits))
	}
	wantKinds := []string{"read", "list_files", "search", "unknown", "read"}
	if !slices.Equal(turn.ParsedCommandKinds, wantKinds) {
		t.Errorf("ParsedCommandKinds = %v, want %v", turn.ParsedCommandKinds, wantKinds)
	}
	var names []string
	for _, tc := range turn.ToolCalls {
		names = append(names, tc.Name)
	}
	if !slices.Equal(names, []string{"exec", "web.search"}) {
		t.Errorf("tool names = %v, want [exec web.search]", names)
	}
}
